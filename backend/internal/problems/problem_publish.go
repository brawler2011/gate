package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type PublishResult struct {
	Version   int32
	PackageID uuid.UUID
}

type ProblemPublishUseCase struct {
	transactions     ProblemTransactions
	problemsRepo     ProblemsRepo
	packagesRepo     PackagesRepo
	workspaceStorage *WorkspaceStorage
	storage          storage.Storage
	packageBucket    string
}

func NewProblemPublishUseCase(
	problemsRepo ProblemsRepo,
	packagesRepo PackagesRepo,
	workspaceStorage *WorkspaceStorage,
	storage storage.Storage,
	packageBucket string,
	transactions ProblemTransactions,
) *ProblemPublishUseCase {
	return &ProblemPublishUseCase{transactions: transactions,
		problemsRepo:     problemsRepo,
		packagesRepo:     packagesRepo,
		workspaceStorage: workspaceStorage,
		storage:          storage,
		packageBucket:    packageBucket,
	}
}

func (uc *ProblemPublishUseCase) publishProblemFiles(
	ctx context.Context,
	problemID uuid.UUID,
) (*PublishResult, error) {
	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get problem: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "problem-publish-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download draft files from workspace storage to local temp directory
	workspaceFiles, err := uc.workspaceStorage.ListAllFiles(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	for _, filePath := range workspaceFiles {
		content, err := uc.workspaceStorage.ReadFile(ctx, problemID, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read workspace file %s: %w", filePath, err)
		}

		fullPath := filepath.Join(tempDir, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create parent directory for %s: %w", filePath, err)
		}
		if err := os.WriteFile(fullPath, content, 0o600); err != nil {
			return nil, fmt.Errorf("failed to write workspace file %s: %w", filePath, err)
		}
	}

	// Validate the package layout by loading problem.yaml
	gfmtProb, err := gfmt.OpenPackage(tempDir)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid problem package")
	}

	// Validate required components
	switch gfmtProb.Problem.Type {
	case "interactive":
		if gfmtProb.Problem.Interactor == "" {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "interactive problem must have an interactor")
		}
		if _, err := gfmtProb.GetInteractor(gfmtProb.Problem.Interactor); err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, fmt.Sprintf("interactor file %s not found", gfmtProb.Problem.Interactor))
		}
	default: // "pass-fail", "scoring"
		if gfmtProb.Problem.Checker == "" {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "problem must have a checker")
		}
		if _, err := gfmtProb.GetChecker(gfmtProb.Problem.Checker); err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, fmt.Sprintf("checker file %s not found", gfmtProb.Problem.Checker))
		}
	}

	// Directly compress the temp directory into a ZIP archive
	var zipBuffer bytes.Buffer
	if err := ZipDirectory(tempDir, &zipBuffer); err != nil {
		return nil, fmt.Errorf("failed to create ZIP: %w", err)
	}

	zipBytes := zipBuffer.Bytes()
	hashBytes := sha256.Sum256(zipBytes)
	packageHash := fmt.Sprintf("%x", hashBytes)
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), packageHash)
	uploadErr := uc.storage.UploadFile(
		ctx,
		uc.packageBucket,
		s3Key,
		bytes.NewReader(zipBytes),
		"application/zip",
	)
	if uploadErr != nil {
		return nil, fmt.Errorf("failed to upload package to S3: %w", uploadErr)
	}

	packageID := uuid.New()

	var created problemsmodel.ProblemPackage
	err = withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionEditProblem, func(tx ProblemTxStore) error {
		var err error
		created, err = tx.CreatePackage(ctx, &problemsmodel.CreatePackageParams{ID: packageID, ProblemID: problemID, OrganizationID: problem.OrganizationID, PackageHash: packageHash, Status: "ready"})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create published package: %w", err)
	}

	return &PublishResult{
		Version:   created.Version,
		PackageID: packageID,
	}, nil
}

func (uc *ProblemPublishUseCase) ListPackages(
	ctx context.Context,
	problemID uuid.UUID,
) ([]problemsmodel.ProblemPackage, error) {
	if err := withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionViewProblem, func(ProblemTxStore) error { return nil }); err != nil {
		return nil, err
	}
	return uc.packagesRepo.ListPackages(ctx, problemID, 100, 0)
}

func (uc *ProblemPublishUseCase) GetPublishedPackageURL(
	ctx context.Context,
	problemID uuid.UUID,
	version string,
) (string, error) {
	if err := withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionViewProblem, func(ProblemTxStore) error { return nil }); err != nil {
		return "", err
	}

	version = strings.TrimPrefix(version, "v")
	versionNum, err := strconv.ParseInt(version, 10, 32)
	if err != nil {
		return "", pkg.Wrap(pkg.ErrBadInput, err, "invalid version")
	}

	pkgVersion, err := uc.packagesRepo.GetPackageByVersion(ctx, problemID, int32(versionNum))
	if err != nil {
		return "", fmt.Errorf("failed to resolve package version: %w", err)
	}

	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), pkgVersion.PackageHash)
	packageURL, err := uc.storage.GetPresignedURL(ctx, uc.packageBucket, s3Key, 1*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate package URL: %w", err)
	}
	return packageURL, nil
}

func (uc *ProblemPublishUseCase) GetReadyPackage(
	ctx context.Context,
	problemID uuid.UUID,
) (problemsmodel.ProblemPackage, error) {
	return uc.packagesRepo.GetReadyPackage(ctx, problemID)
}

func (uc *ProblemPublishUseCase) DownloadPackage(
	ctx context.Context,
	problemID uuid.UUID,
	packageHash string,
) (io.ReadCloser, error) {
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), packageHash)
	reader, _, err := uc.storage.DownloadFile(ctx, uc.packageBucket, s3Key, nil)
	return reader, err
}

func ZipDirectory(srcDir string, writer io.Writer) error {
	archive := zip.NewWriter(writer)

	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		fWriter, err := archive.Create(rel)
		if err != nil {
			return fmt.Errorf("failed to create zip entry for %s: %w", rel, err)
		}

		if _, err := io.Copy(fWriter, file); err != nil {
			return fmt.Errorf("failed to copy file %s to zip: %w", path, err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to zip directory %s: %w", srcDir, err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("close published archive: %w", err)
	}
	return nil
}

func (uc *ProblemPublishUseCase) PublishProblem(ctx context.Context, problemID uuid.UUID) (*PublishResult, error) {
	if err := withinProblem(ctx, uc.transactions, problemID, accessmodel.ActionEditProblem, func(ProblemTxStore) error { return nil }); err != nil {
		return nil, err
	}
	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*PublishResult, error) { return uc.publishProblemFiles(ctx, problemID) })
}
