package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

const (
	MaxRetries       = 3
	PollInterval     = 1 * time.Second
	BatchSize        = 10
	MaxWaitForJudge0 = 30 * time.Second
	CleanupInterval  = 1 * time.Hour
	RetentionDays    = 7
)

type SubmissionsRepo interface {
	GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
}

type ProblemsRepo interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
	GetProblemTests(ctx context.Context, problemId uuid.UUID) (models.ProblemTests, error)
}

type Worker struct {
	logger          *slog.Logger
	outboxRepo      *Repository
	submissionsRepo SubmissionsRepo
	problemsRepo    ProblemsRepo
	judge0Client    *pkg.Judge0Client
	natsPublisher   *pkg.NatsPublisher
}

func NewWorker(
	logger *slog.Logger,
	outboxRepo *Repository,
	submissionsRepo SubmissionsRepo,
	problemsRepo ProblemsRepo,
	judge0Client *pkg.Judge0Client,
	natsPublisher *pkg.NatsPublisher,
) *Worker {
	return &Worker{
		logger:          logger,
		outboxRepo:      outboxRepo,
		submissionsRepo: submissionsRepo,
		problemsRepo:    problemsRepo,
		judge0Client:    judge0Client,
		natsPublisher:   natsPublisher,
	}
}

// Start begins the worker loop
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("outbox worker starting")

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(CleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("outbox worker shutting down")
			return
		case <-ticker.C:
			w.processEvents(ctx)
		case <-cleanupTicker.C:
			w.cleanup(ctx)
		}
	}
}

func (w *Worker) processEvents(ctx context.Context) {
	// Reset failed events that can be retried
	if err := w.outboxRepo.ResetFailedToPending(ctx, MaxRetries); err != nil {
		w.logger.Error("failed to reset failed events", slog.Any("error", err))
	}

	// Get pending events
	events, err := w.outboxRepo.GetPendingEvents(ctx, BatchSize)
	if err != nil {
		w.logger.Error("failed to get pending events", slog.Any("error", err))
		return
	}

	for _, event := range events {
		// Mark as processing
		if err := w.outboxRepo.MarkAsProcessing(ctx, event.Id); err != nil {
			w.logger.Error("failed to mark event as processing", slog.String("event_id", event.Id.String()), slog.Any("error", err))
			continue
		}

		// Process the event
		if err := w.processEvent(ctx, event); err != nil {
			w.logger.Error("failed to process event",
				slog.String("event_id", event.Id.String()),
				slog.String("event_type", string(event.EventType)),
				slog.Any("error", err))

			// Mark as failed
			if markErr := w.outboxRepo.MarkAsFailed(ctx, event.Id, err.Error()); markErr != nil {
				w.logger.Error("failed to mark event as failed", slog.String("event_id", event.Id.String()), slog.Any("error", markErr))
			}
			continue
		}

		// Mark as completed
		if err := w.outboxRepo.MarkAsCompleted(ctx, event.Id); err != nil {
			w.logger.Error("failed to mark event as completed", slog.String("event_id", event.Id.String()), slog.Any("error", err))
		}
	}
}

func (w *Worker) processEvent(ctx context.Context, event *models.OutboxEvent) error {
	switch event.EventType {
	case models.EventTypeSubmissionCreated:
		return w.processSubmissionCreated(ctx, event)
	case models.EventTypeSubmissionTested:
		return w.processSubmissionTested(ctx, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}

func (w *Worker) processSubmissionCreated(ctx context.Context, event *models.OutboxEvent) error {
	// Parse payload
	var payload models.SubmissionCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	w.logger.Info("processing submission.created event",
		slog.String("submission_id", payload.SubmissionId.String()),
		slog.String("problem_id", payload.ProblemId.String()))

	// Fetch submission
	submission, err := w.submissionsRepo.GetSubmissions(ctx, payload.SubmissionId)
	if err != nil {
		return fmt.Errorf("failed to get submission: %w", err)
	}

	// Fetch problem
	problem, err := w.problemsRepo.GetProblemById(ctx, payload.ProblemId)
	if err != nil {
		return fmt.Errorf("failed to get problem: %w", err)
	}

	// Fetch tests
	tests, err := w.problemsRepo.GetProblemTests(ctx, payload.ProblemId)
	if err != nil {
		return fmt.Errorf("failed to get problem tests: %w", err)
	}

	if len(tests) == 0 {
		w.logger.Warn("no tests found for problem", slog.String("problem_id", payload.ProblemId.String()))
		// Update submission state to indicate no tests
		update := &models.SubmissionUpdate{
			State:      models.Accepted, // Accept if no tests
			Score:      0,
			TimeStat:   0,
			MemoryStat: 0,
		}
		if err := w.submissionsRepo.UpdateSubmission(ctx, payload.SubmissionId, update); err != nil {
			return fmt.Errorf("failed to update submission: %w", err)
		}
		return nil
	}

	// Map language ID
	judge0LanguageID, err := pkg.MapLanguageID(int32(submission.Language))
	if err != nil {
		return fmt.Errorf("failed to map language ID: %w", err)
	}

	// Test submission with Judge0
	results, err := w.testSubmission(ctx, submission, problem, tests, judge0LanguageID)
	if err != nil {
		return fmt.Errorf("failed to test submission: %w", err)
	}

	// Update submission with results
	update := &models.SubmissionUpdate{
		State:      results.State,
		Score:      results.Score,
		TimeStat:   results.TimeStat,
		MemoryStat: results.MemoryStat,
	}

	if err := w.submissionsRepo.UpdateSubmission(ctx, payload.SubmissionId, update); err != nil {
		return fmt.Errorf("failed to update submission: %w", err)
	}

	w.logger.Info("submission tested successfully",
		slog.String("submission_id", payload.SubmissionId.String()),
		slog.Int64("state", int64(results.State)),
		slog.Int64("score", results.Score))

	// Create submission.tested event
	testedPayload := models.SubmissionTestedPayload{
		SubmissionId: payload.SubmissionId,
		ProblemId:    payload.ProblemId,
		ContestId:    payload.ContestId,
		State:        results.State,
		Score:        results.Score,
		TimeStat:     results.TimeStat,
		MemoryStat:   results.MemoryStat,
		Language:     payload.Language,
		CreatedBy:    payload.CreatedBy,
	}

	testedPayloadBytes, err := json.Marshal(testedPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal tested payload: %w", err)
	}

	testedEvent := &models.OutboxEvent{
		AggregateId:   payload.SubmissionId,
		AggregateType: "submission",
		EventType:     models.EventTypeSubmissionTested,
		Payload:       testedPayloadBytes,
		Status:        models.OutboxEventStatusPending,
		RetryCount:    0,
	}

	if err := w.outboxRepo.InsertEvent(ctx, testedEvent); err != nil {
		return fmt.Errorf("failed to insert tested event: %w", err)
	}

	return nil
}

type TestResults struct {
	State      models.State
	Score      int64
	TimeStat   int64
	MemoryStat int64
}

func (w *Worker) testSubmission(
	ctx context.Context,
	submission *models.Submission,
	problem *models.Problem,
	tests models.ProblemTests,
	languageID int,
) (*TestResults, error) {
	results := &TestResults{
		State:      models.Accepted,
		Score:      0,
		TimeStat:   0,
		MemoryStat: 0,
	}

	totalTests := int64(len(tests))
	passedTests := int64(0)
	maxTime := int64(0)
	maxMemory := int64(0)

	for _, test := range tests {
		// Create submission in Judge0
		timeLimit := float64(problem.TimeLimit) / 1000.0 // Convert ms to seconds
		memoryLimit := int(problem.MemoryLimit)          // Memory limit in MB

		result, err := w.judge0Client.CreateSubmission(
			ctx,
			submission.Submission,
			languageID,
			test.Input,
			test.Output,
			timeLimit,
			memoryLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create judge0 submission: %w", err)
		}

		// Wait for result
		result, err = w.judge0Client.WaitForSubmission(ctx, result.Token, MaxWaitForJudge0)
		if err != nil {
			return nil, fmt.Errorf("failed to wait for judge0 submission: %w", err)
		}

		// Check result status
		if result.Status == nil {
			return nil, fmt.Errorf("judge0 returned nil status")
		}

		// Map Judge0 status to our state
		// Judge0 status IDs:
		// 3 = Accepted
		// 4 = Wrong Answer
		// 5 = Time Limit Exceeded
		// 6 = Compilation Error
		// 7 = Runtime Error (SIGSEGV)
		// 8 = Runtime Error (SIGXFSZ)
		// 9 = Runtime Error (SIGFPE)
		// 10 = Runtime Error (SIGABRT)
		// 11 = Runtime Error (NZEC)
		// 12 = Runtime Error (Other)
		// 13 = Internal Error
		// 14 = Exec Format Error
		statusID := result.Status.Id

		switch statusID {
		case 3: // Accepted
			passedTests++
		case 4: // Wrong Answer
			results.State = models.GotWA
		case 5: // Time Limit Exceeded
			results.State = models.GotTL
		case 6: // Compilation Error
			results.State = models.GotCE
		case 7, 8, 9, 10, 11, 12: // Runtime Errors
			results.State = models.GotRE
		default:
			results.State = models.GotRE
		}

		// If not accepted, stop testing
		if statusID != 3 {
			w.logger.Info("test failed",
				slog.String("submission_id", submission.Id.String()),
				slog.Int64("test_ordinal", test.Ordinal),
				slog.Int("status_id", statusID))
			break
		}

		// Update max time and memory
		if result.Time != nil {
			// Time is in seconds (string), convert to milliseconds
			var timeSeconds float64
			fmt.Sscanf(*result.Time, "%f", &timeSeconds)
			timeMs := int64(timeSeconds * 1000)
			if timeMs > maxTime {
				maxTime = timeMs
			}
		}

		if result.Memory != nil {
			// Memory is in KB (float32), convert to MB
			memoryMB := int64(*result.Memory / 1024)
			if memoryMB > maxMemory {
				maxMemory = memoryMB
			}
		}
	}

	// Calculate score (percentage of passed tests)
	if totalTests > 0 {
		results.Score = (passedTests * 100) / totalTests
	}

	// If all tests passed, set state to Accepted
	if passedTests == totalTests {
		results.State = models.Accepted
	}

	results.TimeStat = maxTime
	results.MemoryStat = maxMemory

	return results, nil
}

func (w *Worker) processSubmissionTested(ctx context.Context, event *models.OutboxEvent) error {
	// Parse payload
	var payload models.SubmissionTestedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	w.logger.Info("processing submission.tested event",
		slog.String("submission_id", payload.SubmissionId.String()))

	// Publish to NATS
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload for NATS: %w", err)
	}

	if err := w.natsPublisher.Publish("submissions.tested", payloadBytes); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}

	w.logger.Info("published submission.tested event to NATS",
		slog.String("submission_id", payload.SubmissionId.String()))

	return nil
}

func (w *Worker) cleanup(ctx context.Context) {
	w.logger.Info("running cleanup of old outbox events")

	if err := w.outboxRepo.DeleteOldEvents(ctx, RetentionDays); err != nil {
		w.logger.Error("failed to delete old events", slog.Any("error", err))
	}
}
