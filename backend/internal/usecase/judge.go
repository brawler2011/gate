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

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/worker/judge"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/pkg/telemetry"
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
	submissionsRepo interfaces.SubmissionsRepo
	packagesRepo    interfaces.PackagesRepo
	contestsUC      interfaces.ContestsUC
	packageLoader   *PackageLoader
	sandbox         *sandbox.Sandbox
	eventPublisher  *judge.EventPublisher
	componentCache  *judge.ComponentCache
	logger          *slog.Logger
}

func NewJudgeUseCase(
	submissionsRepo interfaces.SubmissionsRepo,
	packagesRepo interfaces.PackagesRepo,
	contestsUC interfaces.ContestsUC,
	storage storage.Storage,
	packageBucket string,
	tempDir string,
	sandbox *sandbox.Sandbox,
	eventPublisher *judge.EventPublisher,
) *JudgeUseCase {
	return &JudgeUseCase{
		submissionsRepo: submissionsRepo,
		packagesRepo:    packagesRepo,
		contestsUC:      contestsUC,
		packageLoader:   NewPackageLoader(storage, packageBucket, tempDir),
		sandbox:         sandbox,
		eventPublisher:  eventPublisher,
		componentCache:  judge.NewComponentCache(sandbox),
		logger:          slog.Default().With("component", "judge_usecase"),
	}
}

func (uc *JudgeUseCase) JudgeSubmission(ctx context.Context, submissionID uuid.UUID) error {
	submission, err := uc.submissionsRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		return fmt.Errorf("failed to get submission: %w", err)
	}

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

	if err := uc.eventPublisher.PublishQueued(ctx, submissionID, meta); err != nil {
		uc.logger.Error("failed to publish queued event", "error", err)
	}

	if submission.ProblemID == nil {
		return fmt.Errorf("submission has no problem ID")
	}

	var targetPkg models.ProblemPackage
	if submission.ContestID != nil {
		contestProb, err := uc.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
			ContestId: *submission.ContestID,
			ProblemId: *submission.ProblemID,
		})
		if err == nil && contestProb.PackageID != uuid.Nil {
			targetPkg, err = uc.packagesRepo.GetPackageByID(ctx, contestProb.PackageID)
			if err != nil {
				uc.logger.Warn("failed to load contest problem package, falling back to ready package", "error", err)
			}
		}
	}

	if targetPkg.PackageHash == "" {
		readyPackage, err := uc.packagesRepo.GetReadyPackage(ctx, *submission.ProblemID)
		if err != nil {
			uc.markInternalError(ctx, submissionID, submission.Penalty, meta, fmt.Errorf("problem has no published version: %w", err))
			return fmt.Errorf("problem has no published version")
		}
		targetPkg = readyPackage
	}

	pkg, err := uc.packageLoader.LoadPackage(ctx, submission.ProblemID.String(), targetPkg.PackageHash)
	if err != nil {
		uc.markInternalError(ctx, submissionID, submission.Penalty, meta, fmt.Errorf("failed to load problem package: %w", err))
		return fmt.Errorf("failed to load problem package: %w", err)
	}
	defer pkg.Cleanup()

	if err := uc.eventPublisher.PublishCompilingStarted(ctx, submissionID, meta); err != nil {
		uc.logger.Error("failed to publish compiling started event", "error", err)
	}

	compiledComponents, err := uc.compileComponents(ctx, pkg.Format, *submission.ProblemID)
	if err != nil {
		uc.markInternalError(ctx, submissionID, submission.Penalty, meta, fmt.Errorf("failed to compile components: %w", err))
		return fmt.Errorf("failed to compile components: %w", err)
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
		uc.markInternalError(ctx, submissionID, submission.Penalty, meta, fmt.Errorf("strategy judging failed: %w", err))
		return fmt.Errorf("judging failed: %w", err)
	}

	err = uc.submissionsRepo.UpdateSubmission(ctx, submissionID, &models.SubmissionUpdate{
		State:      verdict.State,
		Score:      verdict.Score,
		TimeStat:   verdict.MaxTime,
		MemoryStat: verdict.MaxMemory,
	})
	if err != nil {
		return fmt.Errorf("failed to update submission: %w", err)
	}

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

	submission.State = verdict.State
	if uc.contestsUC != nil {
		if procErr := uc.contestsUC.ProcessSubmissionResult(ctx, &submission); procErr != nil {
			uc.logger.Error("failed to process contest submission result", "error", procErr)
		}
	}

	verdictStr := stateToVerdictString(verdict.State)
	langStr := languageToString(submission.Language)
	telemetry.RecordJudgeSubmission(ctx, submission.ContestID.String(), submission.ProblemID.String(), langStr, verdictStr)

	uc.logger.Info("judging completed",
		"submission_id", submissionID,
		"verdict", verdict.State,
		"score", verdict.Score,
		"time", verdict.MaxTime,
		"memory", verdict.MaxMemory,
	)

	return nil
}

func (uc *JudgeUseCase) markInternalError(ctx context.Context, submissionID uuid.UUID, penalty int32, meta models.SubmissionEventMeta, err error) {
	uc.logger.Error("judging failed with internal error", "submission_id", submissionID, "error", err)
	updateErr := uc.submissionsRepo.UpdateSubmission(ctx, submissionID, &models.SubmissionUpdate{
		State:      models.GotIE,
		Score:      0,
		TimeStat:   0,
		MemoryStat: 0,
	})
	if updateErr != nil {
		uc.logger.Error("failed to update submission with error", "error", updateErr)
	}

	if pubErr := uc.eventPublisher.PublishCompleted(ctx, submissionID, models.GotIE, 0, 0, 0, penalty, meta); pubErr != nil {
		uc.logger.Error("failed to publish completed event", "error", pubErr)
	}
}

func stateToVerdictString(s models.State) string {
	switch s {
	case models.Accepted:
		return "Accepted"
	case models.GotWA:
		return "Wrong Answer"
	case models.GotTL:
		return "Time Limit Exceeded"
	case models.GotML:
		return "Memory Limit Exceeded"
	case models.GotRE:
		return "Runtime Error"
	case models.GotPE:
		return "Presentation Error"
	case models.GotCE:
		return "Compilation Error"
	case models.GotIE:
		return "Internal Error"
	default:
		return "Unknown"
	}
}

func languageToString(l models.LanguageName) string {
	switch l {
	case models.Golang:
		return "Go"
	case models.Cpp:
		return "C++"
	case models.Python:
		return "Python"
	default:
		return fmt.Sprintf("Language(%d)", l)
	}
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
