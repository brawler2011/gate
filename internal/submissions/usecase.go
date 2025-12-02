package submissions

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	outboxsqlc "github.com/gate149/core/internal/outbox/sqlc"
	submissionssqlc "github.com/gate149/core/internal/submissions/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type Repo interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionssqlc.GetSubmissionRow, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) ([]submissionssqlc.ListSubmissionsRow, int64, error)
	GetUntestedSubmissions(ctx context.Context, limit int32) ([]submissionssqlc.GetUntestedSubmissionsRow, error)
}

type ContestsUC interface {
	GetContest(ctx context.Context, id uuid.UUID) (domain.Contest, error)
}

type ProblemsUC interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (domain.Problem, error)
}

type OutboxRepo interface {
	InsertEvent(ctx context.Context, event *outboxsqlc.OutboxEvent) error
}

type NatsPublisher interface {
	Publish(subject string, data []byte) error
}

type UseCase struct {
	solutionsRepo Repo
	contestsUC    ContestsUC
	problemsUC    ProblemsUC
	outboxRepo    OutboxRepo
	natsPublisher NatsPublisher
	logger        *slog.Logger
}

func NewUseCase(
	solutionsRepo Repo,
	contestsUC ContestsUC,
	problemsUC ProblemsUC,
	outboxRepo OutboxRepo,
	natsPublisher NatsPublisher,
	logger *slog.Logger,
) *UseCase {
	return &UseCase{
		solutionsRepo: solutionsRepo,
		contestsUC:    contestsUC,
		problemsUC:    problemsUC,
		outboxRepo:    outboxRepo,
		natsPublisher: natsPublisher,
		logger:        logger,
	}
}

func (uc *UseCase) GetSubmissions(ctx context.Context, id uuid.UUID) (domain.Submission, error) {
	s, err := uc.solutionsRepo.GetSubmission(ctx, id)
	if err != nil {
		return domain.Submission{}, err
	}
	return domain.SubmissionFromSqlc(s), nil
}

func (uc *UseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
	// Validate contest exists
	contest, err := uc.contestsUC.GetContest(ctx, creation.ContestId)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "contest not found")
	}

	// Validate problem exists
	problem, err := uc.problemsUC.GetProblemById(ctx, creation.ProblemId)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "problem not found")
	}

	// Save submission to database (state will be Saved (1) by default)
	id, err := uc.solutionsRepo.CreateSubmission(ctx, creation)
	if err != nil {
		return uuid.Nil, err
	}

	// Publish submission.created event directly to NATS (immediate WebSocket notification)
	uc.publishSubmissionCreated(id, creation, contest, problem)

	// Create outbox event for submission.test (async testing via Judge0)
	testPayload := models.SubmissionTestPayload{
		SubmissionId: id,
		ProblemId:    creation.ProblemId,
		ContestId:    creation.ContestId,
		Language:     int64(creation.Language),
		CreatedBy:    creation.UserId,
	}

	testPayloadBytes, err := json.Marshal(testPayload)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to marshal test payload")
	}

	testEvent := &outboxsqlc.OutboxEvent{
		AggregateID:   id,
		AggregateType: "submission",
		EventType:     models.EventTypeSubmissionTest,
		Payload:       testPayloadBytes,
		Status:        models.OutboxEventStatusPending,
		RetryCount:    0,
	}

	if err := uc.outboxRepo.InsertEvent(ctx, testEvent); err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to insert submission.test event")
	}

	return id, nil
}

// publishSubmissionCreated publishes submission.created event directly to NATS
func (uc *UseCase) publishSubmissionCreated(id uuid.UUID, creation *models.SubmissionCreation, contest domain.Contest, problem domain.Problem) {
	event := &models.SubmissionWebSocketEvent{
		MessageType: models.MessageTypeSubmissionCreated,
		Submission: &models.SubmissionListItem{
			ID:                id,
			UserID:            creation.UserId,
			State:             models.Saved, // Initial state
			Score:             0,
			Penalty:           int64(creation.Penalty),
			TimeStat:          0,
			MemoryStat:        0,
			Language:          int64(creation.Language),
			ProblemID:         creation.ProblemId,
			ProblemTitle:      problem.Title,
			Position:          0, // Position is set when problem is added to contest
			ContestID:         creation.ContestId,
			ContestTitle:      contest.Title,
			ContestVisibility: string(contest.Visibility),
		},
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		uc.logger.Error("failed to marshal submission.created event",
			slog.String("submission_id", id.String()),
			slog.Any("error", err))
		return
	}

	if err := uc.natsPublisher.Publish("submissions.list", eventBytes); err != nil {
		uc.logger.Error("failed to publish submission.created to NATS",
			slog.String("submission_id", id.String()),
			slog.Any("error", err))
		return
	}

	uc.logger.Info("published submission.created to NATS",
		slog.String("submission_id", id.String()))
}

func (uc *UseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	return uc.solutionsRepo.UpdateSubmission(ctx, id, update)
}

func (uc *UseCase) ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*domain.SubmissionsList, error) {
	submissions, total, err := uc.solutionsRepo.ListSolutions(ctx, filter)
	if err != nil {
		return nil, err
	}

	domainSubmissions := make([]domain.Submission, len(submissions))
	for i, s := range submissions {
		domainSubmissions[i] = domain.SubmissionListRowFromSqlc(s)
	}

	return &domain.SubmissionsList{
		Submissions: domainSubmissions,
		Pagination:  domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}
