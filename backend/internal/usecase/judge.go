package usecase

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/brawler2011/gate/backend/internal/events"
	"github.com/brawler2011/gate/backend/internal/judging"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/worker/judge"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type LoadedPackage struct {
	Dir    string
	Format *gfmt.GateFormat
}

func (l *LoadedPackage) Cleanup() {
	_ = os.RemoveAll(l.Dir)
}

type PackageLoader struct {
	storage storage.Storage
	bucket  string
	tempDir string
}

func NewPackageLoader(storage storage.Storage, bucket string, tempDir string) *PackageLoader {
	return &PackageLoader{
		storage: storage,
		bucket:  bucket,
		tempDir: tempDir,
	}
}

func (l *PackageLoader) LoadPackage(ctx context.Context, problemID string, packageHash string) (*LoadedPackage, error) {
	pkgDir, err := os.MkdirTemp(l.tempDir, "package-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID, packageHash)
	rc, _, err := l.storage.DownloadFile(ctx, l.bucket, s3Key, nil)
	if err != nil {
		os.RemoveAll(pkgDir)
		return nil, err
	}
	defer rc.Close()

	tmpZip := filepath.Join(pkgDir, "package.zip")
	f, err := os.Create(tmpZip)
	if err != nil {
		os.RemoveAll(pkgDir)
		return nil, fmt.Errorf("failed to create temp zip file %s: %w", tmpZip, err)
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		os.RemoveAll(pkgDir)
		return nil, fmt.Errorf("failed to write package zip content: %w", err)
	}
	f.Close()

	zipReader, err := zip.OpenReader(tmpZip)
	if err != nil {
		os.RemoveAll(pkgDir)
		return nil, fmt.Errorf("failed to open package zip %s: %w", tmpZip, err)
	}
	defer zipReader.Close()

	if err := extractZip(&zipReader.Reader, pkgDir); err != nil {
		os.RemoveAll(pkgDir)
		return nil, err
	}
	_ = os.Remove(tmpZip)

	gfmtPkg, err := gfmt.OpenPackage(pkgDir)
	if err != nil {
		os.RemoveAll(pkgDir)
		return nil, err
	}

	return &LoadedPackage{
		Dir:    pkgDir,
		Format: gfmtPkg,
	}, nil
}

type JudgeUseCase struct {
	jobs            judging.Jobs
	transactions    JudgeTransactions
	submissionsRepo interfaces.SubmissionsRepo
	packageLoader   *PackageLoader
	sandbox         judging.Sandbox
	eventPublisher  judging.Progress
	componentCache  *judge.ComponentCache
	logger          *slog.Logger
}

func NewJudgeUseCase(
	jobs judging.Jobs,
	transactions JudgeTransactions,
	submissionsRepo interfaces.SubmissionsRepo,
	storage storage.Storage,
	packageBucket string,
	tempDir string,
	sandbox judging.Sandbox,
	eventPublisher judging.Progress,
) *JudgeUseCase {
	return &JudgeUseCase{jobs: jobs, transactions: transactions,
		submissionsRepo: submissionsRepo,
		packageLoader:   NewPackageLoader(storage, packageBucket, tempDir),
		sandbox:         sandbox,
		eventPublisher:  eventPublisher,
		componentCache:  judge.NewComponentCache(sandbox),
		logger:          slog.Default().With("component", "judge_usecase"),
	}
}

func (uc *JudgeUseCase) JudgeSubmission(ctx context.Context, submissionID uuid.UUID) error {
	job, err := uc.jobs.Claim(ctx, submissionID)
	if err != nil {
		return err
	}
	if job.ID == uuid.Nil {
		return nil
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				if err := uc.jobs.Renew(leaseCtx, job); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		close(done)
		<-stopped
		releaseCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if err := uc.jobs.Release(releaseCtx, job); err != nil {
			uc.logger.ErrorContext(releaseCtx, "release judge job", "error", err, "job_id", job.ID)
		}
	}()
	ctx = leaseCtx
	logger := uc.logger.With("submission_id", submissionID, "request_id", requestid.Get(ctx))
	if meta := queuecontext.Get(ctx); meta.EventID != uuid.Nil {
		logger = logger.With("event_id", meta.EventID)
	}
	submission, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("failed to get submission: %w", err)
	}

	meta := models.SubmissionEventMeta{
		Revision:     submission.Revision,
		UserId:       submission.CreatedBy,
		Username:     submission.Username,
		ContestId:    submission.ContestID,
		ContestTitle: submission.ContestTitle,
		ProblemId:    submission.ProblemID,
		ProblemTitle: submission.ProblemTitle,
		Position:     submission.Position,
		Language:     submission.Language,
		CreatedAt:    submission.CreatedAt,
		UpdatedAt:    submission.UpdatedAt,
	}

	if err := uc.eventPublisher.PublishQueued(ctx, submissionID, meta); err != nil {
		logger.Error("failed to publish queued event", "operation", "judge_submission", "error_type", fmt.Sprintf("%T", err))
	}

	if submission.ProblemID == nil {
		return fmt.Errorf("submission has no problem ID")
	}

	if job.PackageHash == "" {
		return uc.markInternalError(ctx, job, fmt.Errorf("problem has no published version"))
	}
	pkg, err := uc.packageLoader.LoadPackage(ctx, submission.ProblemID.String(), job.PackageHash)
	if err != nil {
		if judging.IsTemporary(err) {
			return err
		}
		return uc.markInternalError(ctx, job, fmt.Errorf("failed to load problem package: %w", err))
	}
	defer pkg.Cleanup()

	if err := uc.eventPublisher.PublishCompilingStarted(ctx, submissionID, meta); err != nil {
		logger.Error("failed to publish compiling started event", "operation", "judge_submission", "error_type", fmt.Sprintf("%T", err))
	}

	compiledComponents, err := uc.compileComponents(ctx, pkg.Format, *submission.ProblemID)
	if err != nil {
		if judging.IsTemporary(err) {
			return err
		}
		return uc.markInternalError(ctx, job, fmt.Errorf("failed to compile components: %w", err))
	}

	var strategy judge.JudgingStrategy
	switch pkg.Format.Problem.Type {
	case "scoring":
		strategy = judge.NewScoringStrategy(uc.sandbox, uc.eventPublisher, pkg.Format, compiledComponents)
	case "interactive":
		strategy = judge.NewInteractiveStrategy(uc.sandbox, uc.eventPublisher, pkg.Format, compiledComponents)
	default:
		strategy = judge.NewStandardStrategy(uc.sandbox, uc.eventPublisher, pkg.Format, compiledComponents)
	}

	verdict, err := strategy.Judge(ctx, submissionID, submission.Submission, submission.Language, meta)
	if err != nil {
		if judging.IsTemporary(err) {
			return err
		}
		return uc.markInternalError(ctx, job, fmt.Errorf("strategy judging failed: %w", err))
	}

	var failedTest *int32
	if verdict.FailedTest != nil {
		ft := safeInt32(*verdict.FailedTest)
		failedTest = &ft
	}

	if err := uc.applyJudgeResult(ctx, job, &models.SubmissionUpdate{State: verdict.State, Score: verdict.Score, TimeStat: verdict.MaxTime, MemoryStat: verdict.MaxMemory, FailedTest: failedTest, TestDetails: verdict.TestDetails}); err != nil {
		return err
	}

	logger.Info("submission_result", "event", "submission_result",
		"verdict", verdict.State,
		"score", verdict.Score,
		"time", verdict.MaxTime,
		"memory", verdict.MaxMemory,
		"failed_test", failedTest,
	)

	return nil
}

func (uc *JudgeUseCase) markInternalError(ctx context.Context, job judging.Job, cause error) error {
	uc.logger.ErrorContext(ctx, "judging terminal failure", "job_id", job.ID, "error", cause)
	return uc.applyJudgeResult(ctx, job, &models.SubmissionUpdate{State: models.GotIE})
}
func (uc *JudgeUseCase) applyJudgeResult(ctx context.Context, job judging.Job, update *models.SubmissionUpdate) error {
	return uc.transactions.Within(ctx, func(tx JudgeTxStore) error {
		current, err := tx.CompleteJob(ctx, job)
		if err != nil {
			return err
		}
		if !current {
			return nil
		}
		change, err := tx.UpdateSubmission(ctx, job.SubmissionID, update)
		if err != nil {
			return err
		}
		sub, err := tx.GetSubmission(ctx, job.SubmissionID)
		if err != nil {
			return err
		}
		if err := processSubmissionResult(ctx, tx, &sub); err != nil {
			return err
		}
		event := models.SubmissionCompletedEvent{Id: sub.ID, PreviousState: change.PreviousState, State: sub.State, Score: sub.Score, Penalty: sub.Penalty, TimeStat: sub.TimeStat, MemoryStat: sub.MemoryStat, FailedTest: sub.FailedTest,
			SubmissionEventMeta: models.SubmissionEventMeta{Revision: sub.Revision, UpdatedAt: sub.UpdatedAt, UserId: sub.CreatedBy, Username: sub.Username, ContestId: sub.ContestID, ContestTitle: sub.ContestTitle, ProblemId: sub.ProblemID, ProblemTitle: sub.ProblemTitle, Position: sub.Position, Language: sub.Language, CreatedAt: sub.CreatedAt}}
		return tx.Record(ctx, events.Event{AggregateID: sub.ID, Type: models.OutboxEventSubmissionCompleted, Payload: event})
	})
}

func (uc *JudgeUseCase) compileComponents(ctx context.Context, g *gfmt.GateFormat, problemID uuid.UUID) (map[string]sandbox.Executable, error) {
	compiled := make(map[string]sandbox.Executable)

	components := map[string]string{
		"checker":    g.Problem.Checker,
		"validator":  g.Problem.Validator,
		"interactor": g.Problem.Interactor,
	}

	for compType, relativePath := range components {
		if relativePath == "" {
			continue
		}

		filePath := filepath.Join(g.Path, relativePath)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read configured %s file %s: %w", compType, relativePath, err)
		}

		filename := filepath.Base(relativePath)
		hash := sha256.Sum256(data)
		cacheKey := fmt.Sprintf("%s:%s:%x", problemID.String(), compType, hash)
		if fileID, found := uc.componentCache.Get(cacheKey); found {
			compiled[compType] = sandbox.Executable{PrimaryFileID: fileID}
			uc.logger.Debug("component cache hit", "type", compType, "cache_key", cacheKey)
			continue
		}

		deps, err := loadLibDependencies(g.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to load library dependencies for %s: %w", compType, err)
		}

		lang := detectLanguage(filepath.Ext(filename))
		exec, err := uc.sandbox.Compile(ctx, data, lang, deps)
		if err != nil {
			return nil, fmt.Errorf("failed to compile component %s (%s): %w", compType, filename, err)
		}

		compiled[compType] = exec
		uc.componentCache.Set(cacheKey, exec.PrimaryFileID)
		uc.logger.Debug("component compiled and cached", "type", compType, "file_id", exec.PrimaryFileID)
	}

	return compiled, nil
}

func loadLibDependencies(pkgPath string) (map[string][]byte, error) {
	deps := make(map[string][]byte)
	libDir := filepath.Join(pkgPath, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		if os.IsNotExist(err) {
			return deps, nil
		}
		return nil, fmt.Errorf("failed to read lib directory %s: %w", libDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			data, err := os.ReadFile(filepath.Join(libDir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("failed to read lib file %s: %w", entry.Name(), err)
			}
			deps[entry.Name()] = data
		}
	}
	return deps, nil
}
