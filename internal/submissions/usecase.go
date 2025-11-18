package submissions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Publisher interface {
	Publish(subject string, data []byte) error
}

type Repo interface {
	GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error)
}

type ProblemsUC interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
	DownloadTestsArchive(ctx context.Context, id uuid.UUID) (string, error)
	UnarchiveTestsArchive(ctx context.Context, zipPath, destDirPath string) (string, error)
}

type UseCase struct {
	solutionsRepo Repo
	problemsUC    ProblemsUC
	pub           Publisher
	redisClient   *redis.Client
	judgeQueue    string
}

func NewUseCase(
	solutionsRepo Repo,
	problemsUC ProblemsUC,
	pub Publisher,
	redisClient *redis.Client,
	judgeQueue string,
) *UseCase {
	return &UseCase{
		solutionsRepo: solutionsRepo,
		problemsUC:    problemsUC,
		pub:           pub,
		redisClient:   redisClient,
		judgeQueue:    judgeQueue,
	}
}

func (uc *UseCase) GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error) {
	return uc.solutionsRepo.GetSubmissions(ctx, id)
}

func (uc *UseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	solutionId, err := uc.solutionsRepo.CreateSubmission(ctx, creation)
	if err != nil {
		return uuid.Nil, err
	}

	// Enqueue solution for testing
	if err := uc.enqueueSolutionForTesting(ctx, solutionId); err != nil {
		// Log error but don't fail the request
		// The solution was created successfully, just failed to queue
		fmt.Printf("Failed to enqueue solution for testing: %v\n", err)
	}

	return solutionId, nil
}

// enqueueSolutionForTesting adds a solution to the judge queue
func (uc *UseCase) enqueueSolutionForTesting(ctx context.Context, solutionID uuid.UUID) error {
	job := map[string]interface{}{
		"solution_id": solutionID.String(),
		"priority":    0,
	}

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Add to queue (RPUSH for FIFO)
	if err := uc.redisClient.RPush(ctx, uc.judgeQueue, jobJSON).Err(); err != nil {
		return fmt.Errorf("failed to push to queue: %w", err)
	}

	return nil
}

func (uc *UseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	return uc.solutionsRepo.UpdateSubmission(ctx, id, update)
}

func (uc *UseCase) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error) {
	return uc.solutionsRepo.ListSolutions(ctx, filter)
}

const (
	MessageTypeCreate = "CREATE"
	MessageTypeUpdate = "UPDATE"
	MessageTypeDelete = "DELETE"
)

type SolutionsListItem struct {
	Id int32 `json:"id"`

	UserId   int32  `json:"user_id"`
	Username string `json:"username"`

	State      models.State        `json:"state"`
	Score      int32               `json:"score"`
	Penalty    int32               `json:"penalty"`
	TimeStat   int32               `json:"time_stat"`
	MemoryStat int32               `json:"memory_stat"`
	Language   models.LanguageName `json:"language"`

	ProblemId    int32  `json:"problem_id"`
	ProblemTitle string `json:"problem_title"`

	Position int32 `json:"position"`

	ContestId    int32  `json:"contest_id"`
	ContestTitle string `json:"contest_title"`

	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	MessageType string            `json:"message_type"`
	Message     *string           `json:"message,omitempty"`
	Solution    SolutionsListItem `json:"solution"`
}
