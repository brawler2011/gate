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

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/worker/judge"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/sandbox"
	"github.com/gate149/gate/backend/pkg/storage"
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
		return nil, err
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
		return nil, err
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		os.RemoveAll(pkgDir)
		return nil, err
	}
	f.Close()

	zipReader, err := zip.OpenReader(tmpZip)
	if err != nil {
		os.RemoveAll(pkgDir)
		return nil, err
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
	packageLoader   *PackageLoader
	sandbox         *sandbox.Sandbox
	eventPublisher  *judge.EventPublisher
	componentCache  *judge.ComponentCache
	logger          *slog.Logger
}

func NewJudgeUseCase(
	submissionsRepo interfaces.SubmissionsRepo,
	packagesRepo interfaces.PackagesRepo,
	storage storage.Storage,
	packageBucket string,
	tempDir string,
	sandbox *sandbox.Sandbox,
	eventPublisher *judge.EventPublisher,
) *JudgeUseCase {
	return &JudgeUseCase{
		submissionsRepo: submissionsRepo,
		packagesRepo:    packagesRepo,
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
	readyPackage, err := uc.packagesRepo.GetReadyPackage(ctx, *submission.ProblemID)
	if err != nil {
		return fmt.Errorf("problem has no published version")
	}

	pkg, err := uc.packageLoader.LoadPackage(ctx, submission.ProblemID.String(), readyPackage.PackageHash)
	if err != nil {
		return fmt.Errorf("failed to load problem package: %w", err)
	}
	defer pkg.Cleanup()

	if err := uc.eventPublisher.PublishCompilingStarted(ctx, submissionID, meta); err != nil {
		uc.logger.Error("failed to publish compiling started event", "error", err)
	}

	compiledComponents, err := uc.compileComponents(ctx, pkg.Format, *submission.ProblemID)
	if err != nil {
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
		updateErr := uc.submissionsRepo.UpdateSubmission(ctx, submissionID, &models.SubmissionUpdate{
			State:      models.GotRE,
			Score:      0,
			TimeStat:   0,
			MemoryStat: 0,
		})
		if updateErr != nil {
			uc.logger.Error("failed to update submission with error", "error", updateErr)
		}

		if pubErr := uc.eventPublisher.PublishCompleted(ctx, submissionID, models.GotRE, 0, 0, 0, submission.Penalty, meta); pubErr != nil {
			uc.logger.Error("failed to publish completed event", "error", pubErr)
		}

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

	uc.logger.Info("judging completed",
		"submission_id", submissionID,
		"verdict", verdict.State,
		"score", verdict.Score,
		"time", verdict.MaxTime,
		"memory", verdict.MaxMemory,
	)

	return nil
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
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			data, err := os.ReadFile(filepath.Join(libDir, entry.Name()))
			if err != nil {
				return nil, err
			}
			deps[entry.Name()] = data
		}
	}
	return deps, nil
}
