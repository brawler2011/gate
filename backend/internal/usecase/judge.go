package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

// JudgeUseCase orchestrates the judging process
type JudgeUseCase struct {
	submissionsRepo interfaces.SubmissionsRepo
	packagesRepo    interfaces.PackagesRepo
	packageLoader   *problemformat.PackageLoader
	sandboxClient   *sandbox.Client
	eventPublisher  *judge.EventPublisher
	componentCache  *judge.ComponentCache
	logger          *slog.Logger
}

// NewJudgeUseCase creates a new judge use case
func NewJudgeUseCase(
	submissionsRepo interfaces.SubmissionsRepo,
	packagesRepo interfaces.PackagesRepo,
	s3Client *pkg.S3Client,
	packageBucket string,
	tempDir string,
	sandboxClient *sandbox.Client,
	eventPublisher *judge.EventPublisher,
) *JudgeUseCase {
	return &JudgeUseCase{
		submissionsRepo: submissionsRepo,
		packagesRepo:    packagesRepo,
		packageLoader:   problemformat.NewPackageLoader(s3Client, packageBucket, tempDir),
		sandboxClient:   sandboxClient,
		eventPublisher:  eventPublisher,
		componentCache:  judge.NewComponentCache(sandboxClient),
		logger:          slog.Default().With("component", "judge_usecase"),
	}
}

// JudgeSubmission judges a submission
func (uc *JudgeUseCase) JudgeSubmission(ctx context.Context, submissionID uuid.UUID) error {
	// Get submission from database
	submission, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("failed to get submission: %w", err)
	}

	// Create event metadata
	meta := models.SubmissionEventMeta{
		UserId:       submission.CreatedBy,
		Username:     submission.Username,
		ContestId:    submission.ContestID,
		ContestTitle: submission.ContestTitle,
		ProblemId:    submission.ProblemID,
		ProblemTitle: submission.ProblemTitle,
		Position:     submission.Position,
		Language:     submission.Language,
		CreatedAt:    submission.CreatedAt,
	}

	// Publish queued event
	if err := uc.eventPublisher.PublishQueued(ctx, submissionID, meta); err != nil {
		uc.logger.Error("failed to publish queued event", "error", err)
	}

	// Get problem details
	if submission.ProblemID == nil {
		return fmt.Errorf("submission has no problem ID")
	}
	readyPackage, err := uc.packagesRepo.GetReadyPackage(ctx, *submission.ProblemID)
	if err != nil {
		return fmt.Errorf("problem has no published version")
	}

	// Load problem package
	pkg, err := uc.packageLoader.LoadPackage(ctx, submission.ProblemID.String(), readyPackage.PackageHash)
	if err != nil {
		return fmt.Errorf("failed to load problem package: %w", err)
	}
	defer pkg.Cleanup()

	// Publish compiling started event
	if err := uc.eventPublisher.PublishCompilingStarted(ctx, submissionID, meta); err != nil {
		uc.logger.Error("failed to publish compiling started event", "error", err)
	}

	// Compile components (checker, validator, interactor if needed)
	compiledComponents, err := uc.compileComponents(ctx, pkg, *submission.ProblemID)
	if err != nil {
		return fmt.Errorf("failed to compile components: %w", err)
	}

	// Determine judging strategy based on problem type
	var strategy judge.JudgingStrategy
	switch pkg.Manifest.ProblemType {
	case "scoring":
		strategy = judge.NewScoringStrategy(uc.sandboxClient, uc.eventPublisher, pkg, compiledComponents)
	case "interactive":
		strategy = judge.NewInteractiveStrategy(uc.sandboxClient, uc.eventPublisher, pkg, compiledComponents)
	default: // "pass-fail" or any other type
		strategy = judge.NewStandardStrategy(uc.sandboxClient, uc.eventPublisher, pkg, compiledComponents)
	}

	// Judge the submission
	verdict, err := strategy.Judge(ctx, submissionID, submission.Submission, submission.Language, meta)
	if err != nil {
		// Update submission with error state
		updateErr := uc.submissionsRepo.UpdateSubmission(ctx, submissionID, &models.SubmissionUpdate{
			State:      models.GotRE,
			Score:      0,
			TimeStat:   0,
			MemoryStat: 0,
		})
		if updateErr != nil {
			uc.logger.Error("failed to update submission with error", "error", updateErr)
		}

		// Publish completed event with error
		if pubErr := uc.eventPublisher.PublishCompleted(ctx, submissionID, models.GotRE, 0, 0, 0, submission.Penalty, meta); pubErr != nil {
			uc.logger.Error("failed to publish completed event", "error", pubErr)
		}

		return fmt.Errorf("judging failed: %w", err)
	}

	// Update submission in database
	err = uc.submissionsRepo.UpdateSubmission(ctx, submissionID, &models.SubmissionUpdate{
		State:      verdict.State,
		Score:      verdict.Score,
		TimeStat:   verdict.MaxTime,
		MemoryStat: verdict.MaxMemory,
	})
	if err != nil {
		return fmt.Errorf("failed to update submission: %w", err)
	}

	// Publish completed event
	if err := uc.eventPublisher.PublishCompleted(
		ctx,
		submissionID,
		verdict.State,
		verdict.Score,
		verdict.MaxTime,
		verdict.MaxMemory,
		submission.Penalty,
		meta,
	); err != nil {
		uc.logger.Error("failed to publish completed event", "error", err)
	}

	uc.logger.Info("judging completed",
		"submission_id", submissionID,
		"verdict", verdict.State,
		"score", verdict.Score,
		"time", verdict.MaxTime,
		"memory", verdict.MaxMemory,
	)

	return nil
}

// compileComponents compiles problem components and caches them
func (uc *JudgeUseCase) compileComponents(ctx context.Context, pkg *problemformat.ProblemPackage, problemID uuid.UUID) (map[string]string, error) {
	compiled := make(map[string]string) // component type -> fileID

	for _, meta := range pkg.Manifest.FilesMetadata {
		component, exists := pkg.Components[meta.Filename]
		if !exists {
			continue
		}

		// Try to get from cache
		cacheKey := fmt.Sprintf("%s:%s:%s", problemID.String(), meta.Type, *meta.BinarySha256)
		if cachedFileID, found := uc.componentCache.Get(cacheKey); found {
			compiled[meta.Type] = cachedFileID
			uc.logger.Debug("component cache hit", "type", meta.Type, "cache_key", cacheKey)
			continue
		}

		// Compile component
		orchestrator := sandbox.NewOrchestrator(uc.sandboxClient)
		result, err := orchestrator.CompileComponentFromSource(
			ctx,
			meta.Type,
			component.SourceCode,
			component.Language,
			component.Dependencies,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to compile %s: %w", meta.Type, err)
		}

		if !result.Success {
			return nil, fmt.Errorf("compilation failed for %s: %s", meta.Type, result.Error)
		}

		compiled[meta.Type] = result.FileID

		// Cache the compiled component
		uc.componentCache.Set(cacheKey, result.FileID)
		uc.logger.Debug("component compiled and cached", "type", meta.Type, "file_id", result.FileID)
	}

	return compiled, nil
}
