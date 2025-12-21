package usecase

import (
	"context"
	"encoding/json"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type SubmissionsUseCase struct {
	submissionsRepo interfaces.SubmissionsRepo
	contestsUC      interfaces.ContestsUC
	problemsUC      interfaces.ProblemsUC
	outboxRepo      interfaces.OutboxRepo
	natsPublisher   interfaces.NatsPublisher
}

func NewSubmissionsUseCase(
	submissionsRepo interfaces.SubmissionsRepo,
	contestsUC interfaces.ContestsUC,
	problemsUC interfaces.ProblemsUC,
	outboxRepo interfaces.OutboxRepo,
	natsPublisher interfaces.NatsPublisher,
) *SubmissionsUseCase {
	return &SubmissionsUseCase{
		submissionsRepo: submissionsRepo,
		contestsUC:      contestsUC,
		problemsUC:      problemsUC,
		outboxRepo:      outboxRepo,
		natsPublisher:   natsPublisher,
	}
}

func (uc *SubmissionsUseCase) GetSubmission(ctx context.Context, id uuid.UUID) (models.Submission, error) {
	return uc.submissionsRepo.GetSubmission(ctx, id)
}

func (uc *SubmissionsUseCase) CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error) {
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
	id, err := uc.submissionsRepo.CreateSubmission(ctx, creation)
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
		Language:     int32(creation.Language),
		CreatedBy:    creation.UserId,
	}

	testPayloadBytes, err := json.Marshal(testPayload)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to marshal test payload")
	}

	testEvent := &models.OutboxEvent{
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
func (uc *SubmissionsUseCase) publishSubmissionCreated(id uuid.UUID, creation *models.SubmissionCreation, contest models.Contest, problem models.Problem) {
	event := &models.SubmissionWebSocketEvent{
		MessageType: models.MessageTypeSubmissionCreated,
		Submission: &models.SubmissionListItem{
			ID:           id,
			UserID:       creation.UserId,
			State:        models.Saved, // Initial state
			Score:        0,
			Penalty:      creation.Penalty,
			TimeStat:     0,
			MemoryStat:   0,
			Language:     creation.Language,
			ProblemID:    creation.ProblemId,
			ProblemTitle: problem.Title,
			Position:     0, // Position is set when problem is added to contest
			ContestID:    creation.ContestId,
			ContestTitle: contest.Title,
		},
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return
	}

	if err := uc.natsPublisher.Publish("submissions.list", eventBytes); err != nil {
		return
	}
}

func (uc *SubmissionsUseCase) UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error {
	return uc.submissionsRepo.UpdateSubmission(ctx, id, update)
}

func (uc *SubmissionsUseCase) ListSubmissions(ctx context.Context, filter models.SubmissionsFilter) (*models.SubmissionsList, error) {
	submissions, total, err := uc.submissionsRepo.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &models.SubmissionsList{
		Submissions: submissions,
		Pagination:  models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}
