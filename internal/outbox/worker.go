package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gate149/core/internal/models"
	outboxsqlc "github.com/gate149/core/internal/outbox/sqlc"
	problemssqlc "github.com/gate149/core/internal/problems/sqlc"
	submissionssqlc "github.com/gate149/core/internal/submissions/sqlc"
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
	RetryInterval    = 5 * time.Minute // Interval to retry untested submissions
	RetryBatchSize   = 5               // Number of untested submissions to retry at once
)

type SubmissionsRepo interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (submissionssqlc.GetSubmissionRow, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	GetUntestedSubmissions(ctx context.Context, limit int32) ([]submissionssqlc.GetUntestedSubmissionsRow, error)
}

type ProblemsRepo interface {
	GetProblemById(ctx context.Context, id uuid.UUID) (problemssqlc.Problem, error)
	GetProblemTests(ctx context.Context, problemId uuid.UUID) ([]problemssqlc.ProblemTest, error)
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

	retryTicker := time.NewTicker(RetryInterval)
	defer retryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("outbox worker shutting down")
			return
		case <-ticker.C:
			w.processEvents(ctx)
		case <-cleanupTicker.C:
			w.cleanup(ctx)
		case <-retryTicker.C:
			w.retryUntestedSubmissions(ctx)
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

	for i := range events {
		event := &events[i]
		// Mark as processing
		if err := w.outboxRepo.MarkAsProcessing(ctx, event.ID); err != nil {
			w.logger.Error("failed to mark event as processing", slog.String("event_id", event.ID.String()), slog.Any("error", err))
			continue
		}

		// Process the event
		if err := w.processEvent(ctx, event); err != nil {
			w.logger.Error("failed to process event",
				slog.String("event_id", event.ID.String()),
				slog.String("event_type", string(event.EventType)),
				slog.Any("error", err))

			// Mark as failed
			if markErr := w.outboxRepo.MarkAsFailed(ctx, event.ID, err.Error()); markErr != nil {
				w.logger.Error("failed to mark event as failed", slog.String("event_id", event.ID.String()), slog.Any("error", markErr))
			}
			continue
		}

		// Mark as completed
		if err := w.outboxRepo.MarkAsCompleted(ctx, event.ID); err != nil {
			w.logger.Error("failed to mark event as completed", slog.String("event_id", event.ID.String()), slog.Any("error", err))
		}
	}
}

func (w *Worker) processEvent(ctx context.Context, event *outboxsqlc.OutboxEvent) error {
	switch event.EventType {
	case models.EventTypeSubmissionTest:
		return w.processSubmissionTest(ctx, event)
	case models.EventTypeSubmissionTested:
		return w.processSubmissionTested(ctx, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.EventType)
	}
}

func (w *Worker) processSubmissionTest(ctx context.Context, event *outboxsqlc.OutboxEvent) error {
	// Parse payload
	var payload models.SubmissionTestPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	w.logger.Info("processing submission.test event",
		slog.String("submission_id", payload.SubmissionId.String()),
		slog.String("problem_id", payload.ProblemId.String()))

	// Fetch submission
	submission, err := w.submissionsRepo.GetSubmission(ctx, payload.SubmissionId)
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

	// Build submission context for progress events
	subCtx := w.buildSubmissionContext(&submission)

	// Test submission with Judge0
	results, err := w.testSubmission(ctx, submission, subCtx, problem, tests, judge0LanguageID)
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

	// Fetch updated submission to publish WebSocket event
	updatedSubmission, err := w.submissionsRepo.GetSubmission(ctx, payload.SubmissionId)
	if err != nil {
		w.logger.Error("failed to fetch updated submission for websocket event",
			slog.String("submission_id", payload.SubmissionId.String()),
			slog.Any("error", err))
	} else {
		// Publish submission.updated WebSocket event for real-time list updates
		w.publishSubmissionUpdated(&updatedSubmission)
	}

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

	testedEvent := &outboxsqlc.OutboxEvent{
		AggregateID:   payload.SubmissionId,
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

// SubmissionContext holds metadata needed for publishing test progress events
type SubmissionContext struct {
	SubmissionID      uuid.UUID
	ContestID         uuid.UUID
	UserID            uuid.UUID
	ProblemID         uuid.UUID
	Language          int64
	ContestVisibility string
}

// publishTestProgress publishes a test progress event to NATS
// Publishes to both the individual submission subject and the list subject
func (w *Worker) publishTestProgress(ctx *SubmissionContext, event *models.TestProgressEvent) {
	// Publish to individual submission subject (for backward compatibility)
	subject := fmt.Sprintf("submissions.%s.progress", ctx.SubmissionID.String())

	eventBytes, err := json.Marshal(event)
	if err != nil {
		w.logger.Error("failed to marshal test progress event",
			slog.String("submission_id", ctx.SubmissionID.String()),
			slog.Any("error", err))
		return
	}

	if err := w.natsPublisher.Publish(subject, eventBytes); err != nil {
		w.logger.Error("failed to publish test progress event",
			slog.String("submission_id", ctx.SubmissionID.String()),
			slog.String("subject", subject),
			slog.Any("error", err))
	}

	// Also publish to submissions.list for real-time list updates
	w.publishTestProgressToList(ctx, event)

	w.logger.Debug("published test progress event",
		slog.String("submission_id", ctx.SubmissionID.String()),
		slog.String("type", string(event.Type)))
}

// publishTestProgressToList publishes test progress to the submissions.list subject
// This allows WebSocket clients watching the submission list to see real-time test progress
func (w *Worker) publishTestProgressToList(ctx *SubmissionContext, event *models.TestProgressEvent) {
	// Map TestProgressEvent type to WebSocket message type
	var messageType models.WebSocketMessageType
	switch event.Type {
	case models.TestProgressEventTestingStarted:
		messageType = models.MessageTypeTestingStarted
	case models.TestProgressEventTestCompleted:
		messageType = models.MessageTypeTestCompleted
	case models.TestProgressEventTestingCompleted:
		messageType = models.MessageTypeTestingCompleted
	default:
		return
	}

	// Build WebSocket event with filter metadata
	passed := event.Passed
	state := event.State
	wsEvent := &models.SubmissionWebSocketEvent{
		MessageType:       messageType,
		SubmissionID:      &ctx.SubmissionID,
		TestNumber:        event.TestNumber,
		TotalTests:        event.TotalTests,
		Passed:            &passed,
		State:             &state,
		ContestID:         &ctx.ContestID,
		UserID:            &ctx.UserID,
		ProblemID:         &ctx.ProblemID,
		Language:          &ctx.Language,
		ContestVisibility: ctx.ContestVisibility,
	}

	w.publishSubmissionWebSocketEvent(wsEvent)

	w.logger.Info("published test progress list event",
		slog.String("submission_id", ctx.SubmissionID.String()),
		slog.String("type", string(messageType)),
		slog.String("user_id", ctx.UserID.String()))
}

// publishSubmissionCreated publishes a submission.created WebSocket event to NATS
// This is used to notify WebSocket clients about new submissions in real-time
func (w *Worker) publishSubmissionCreated(submission *submissionssqlc.GetSubmissionRow) {
	event := w.buildSubmissionWebSocketEvent(submission, models.MessageTypeSubmissionCreated)
	w.publishSubmissionWebSocketEvent(event)
}

// publishSubmissionUpdated publishes a submission.updated WebSocket event to NATS
// This is used to notify WebSocket clients when submission test results are ready
func (w *Worker) publishSubmissionUpdated(submission *submissionssqlc.GetSubmissionRow) {
	event := w.buildSubmissionWebSocketEvent(submission, models.MessageTypeSubmissionUpdated)
	w.publishSubmissionWebSocketEvent(event)
}

// buildSubmissionWebSocketEvent creates a WebSocket event from a submission row
func (w *Worker) buildSubmissionWebSocketEvent(submission *submissionssqlc.GetSubmissionRow, messageType models.WebSocketMessageType) *models.SubmissionWebSocketEvent {
	// Convert pgtype.UUID to uuid.UUID safely
	var userID, problemID, contestID uuid.UUID
	if submission.CreatedBy.Valid {
		userID = submission.CreatedBy.Bytes
	}
	if submission.ProblemID.Valid {
		problemID = submission.ProblemID.Bytes
	}
	if submission.ContestID.Valid {
		contestID = submission.ContestID.Bytes
	}

	// Handle nullable strings
	username := ""
	if submission.Username != nil {
		username = *submission.Username
	}
	problemTitle := ""
	if submission.ProblemTitle != nil {
		problemTitle = *submission.ProblemTitle
	}
	contestTitle := ""
	if submission.ContestTitle != nil {
		contestTitle = *submission.ContestTitle
	}
	position := int64(0)
	if submission.Position != nil {
		position = int64(*submission.Position)
	}

	// Get contest visibility for permission filtering
	contestVisibility := ""
	if submission.ContestVisibility.Valid {
		contestVisibility = string(submission.ContestVisibility.ContestVisibility)
	}

	return &models.SubmissionWebSocketEvent{
		MessageType: messageType,
		Submission: &models.SubmissionListItem{
			ID:                submission.ID,
			UserID:            userID,
			Username:          username,
			State:             submission.State,
			Score:             int64(submission.Score),
			Penalty:           int64(submission.Penalty),
			TimeStat:          int64(submission.TimeStat),
			MemoryStat:        int64(submission.MemoryStat),
			Language:          int64(submission.Language),
			ProblemID:         problemID,
			ProblemTitle:      problemTitle,
			Position:          position,
			ContestID:         contestID,
			ContestTitle:      contestTitle,
			UpdatedAt:         submission.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt:         submission.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			ContestVisibility: contestVisibility,
		},
	}
}

// buildSubmissionContext creates a SubmissionContext from a submission row
func (w *Worker) buildSubmissionContext(submission *submissionssqlc.GetSubmissionRow) *SubmissionContext {
	var userID, problemID, contestID uuid.UUID
	if submission.CreatedBy.Valid {
		userID = submission.CreatedBy.Bytes
	}
	if submission.ProblemID.Valid {
		problemID = submission.ProblemID.Bytes
	}
	if submission.ContestID.Valid {
		contestID = submission.ContestID.Bytes
	}

	contestVisibility := ""
	if submission.ContestVisibility.Valid {
		contestVisibility = string(submission.ContestVisibility.ContestVisibility)
	}

	return &SubmissionContext{
		SubmissionID:      submission.ID,
		ContestID:         contestID,
		UserID:            userID,
		ProblemID:         problemID,
		Language:          int64(submission.Language),
		ContestVisibility: contestVisibility,
	}
}

// publishSubmissionWebSocketEvent publishes a submission WebSocket event to NATS
func (w *Worker) publishSubmissionWebSocketEvent(event *models.SubmissionWebSocketEvent) {
	// Publish to submissions.list subject for real-time list updates
	subject := "submissions.list"

	eventBytes, err := json.Marshal(event)
	if err != nil {
		w.logger.Error("failed to marshal submission websocket event",
			slog.Any("error", err))
		return
	}

	if err := w.natsPublisher.Publish(subject, eventBytes); err != nil {
		w.logger.Error("failed to publish submission websocket event",
			slog.String("subject", subject),
			slog.Any("error", err))
		return
	}

	// Get submission ID for logging
	var subID string
	if event.Submission != nil {
		subID = event.Submission.ID.String()
	} else if event.SubmissionID != nil {
		subID = event.SubmissionID.String()
	}

	w.logger.Debug("published submission websocket event",
		slog.String("submission_id", subID),
		slog.String("message_type", string(event.MessageType)))
}

func (w *Worker) testSubmission(
	ctx context.Context,
	submission submissionssqlc.GetSubmissionRow,
	subCtx *SubmissionContext,
	problem problemssqlc.Problem,
	tests []problemssqlc.ProblemTest,
	languageID int,
) (*TestResults, error) {
	results := &TestResults{
		State:      models.Accepted,
		Score:      0,
		TimeStat:   0,
		MemoryStat: 0,
	}

	totalTests := len(tests)
	passedTests := 0
	maxTime := int64(0)
	maxMemory := int64(0)

	// Publish testing_started event
	w.publishTestProgress(subCtx, &models.TestProgressEvent{
		Type:         models.TestProgressEventTestingStarted,
		SubmissionId: submission.ID,
		TotalTests:   totalTests,
	})

	w.logger.Info("starting tests",
		slog.String("submission_id", submission.ID.String()),
		slog.Int("total_tests", totalTests))

	for i, test := range tests {
		testNumber := i + 1

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
		statusID := result.Status.Id
		testPassed := statusID == 3

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

		// Publish test_completed event
		w.publishTestProgress(subCtx, &models.TestProgressEvent{
			Type:         models.TestProgressEventTestCompleted,
			SubmissionId: submission.ID,
			TestNumber:   testNumber,
			TotalTests:   totalTests,
			Passed:       testPassed,
		})

		// If not accepted, stop testing
		if !testPassed {
			w.logger.Info("test failed",
				slog.String("submission_id", submission.ID.String()),
				slog.Int64("test_ordinal", int64(test.Ordinal)),
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
		results.Score = int64(passedTests * 100 / totalTests)
	}

	// If all tests passed, set state to Accepted
	if passedTests == totalTests {
		results.State = models.Accepted
	}

	results.TimeStat = maxTime
	results.MemoryStat = maxMemory

	// Publish testing_completed event
	w.publishTestProgress(subCtx, &models.TestProgressEvent{
		Type:         models.TestProgressEventTestingCompleted,
		SubmissionId: submission.ID,
		TotalTests:   totalTests,
		State:        results.State,
	})

	return results, nil
}

func (w *Worker) processSubmissionTested(ctx context.Context, event *outboxsqlc.OutboxEvent) error {
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

// retryUntestedSubmissions finds submissions that are still in "Saved" state
// and creates new outbox events for them to retry testing
func (w *Worker) retryUntestedSubmissions(ctx context.Context) {
	w.logger.Info("checking for untested submissions")

	// Get submissions that are still in "Saved" state
	submissions, err := w.submissionsRepo.GetUntestedSubmissions(ctx, RetryBatchSize)
	if err != nil {
		w.logger.Error("failed to get untested submissions", slog.Any("error", err))
		return
	}

	if len(submissions) == 0 {
		return
	}

	w.logger.Info("found untested submissions to retry", slog.Int("count", len(submissions)))

	// Create new outbox events for untested submissions
	for _, sub := range submissions {
		// Convert pgtype.UUID to uuid.UUID
		var problemID uuid.UUID
		var contestID uuid.UUID
		var createdBy uuid.UUID

		if sub.ProblemID.Valid {
			problemID = sub.ProblemID.Bytes
		}
		if sub.ContestID.Valid {
			contestID = sub.ContestID.Bytes
		}
		if sub.CreatedBy.Valid {
			createdBy = sub.CreatedBy.Bytes
		}

		payload := models.SubmissionCreatedPayload{
			SubmissionId: sub.ID,
			ProblemId:    problemID,
			ContestId:    contestID,
			Language:     int64(sub.Language),
			CreatedBy:    createdBy,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			w.logger.Error("failed to marshal retry payload",
				slog.String("submission_id", sub.ID.String()),
				slog.Any("error", err))
			continue
		}

		event := &outboxsqlc.OutboxEvent{
			AggregateID:   sub.ID,
			AggregateType: "submission",
			EventType:     models.EventTypeSubmissionCreated,
			Payload:       payloadBytes,
			Status:        models.OutboxEventStatusPending,
			RetryCount:    0,
		}

		if err := w.outboxRepo.InsertEvent(ctx, event); err != nil {
			w.logger.Error("failed to insert retry event",
				slog.String("submission_id", sub.ID.String()),
				slog.Any("error", err))
			continue
		}

		w.logger.Info("created retry event for untested submission",
			slog.String("submission_id", sub.ID.String()))
	}
}
