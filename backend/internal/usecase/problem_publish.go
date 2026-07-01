package usecase

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

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/formats/gfmt"
	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type PublishResult struct {
	Version   int32
	PackageID uuid.UUID
}

type ProblemPublishUseCase struct {
	problemsRepo     interfaces.ProblemsRepo
	packagesRepo     interfaces.PackagesRepo
	workspaceStorage *WorkspaceStorage
	storage          storage.Storage
	packageBucket    string
}

func NewProblemPublishUseCase(
	problemsRepo interfaces.ProblemsRepo,
	packagesRepo interfaces.PackagesRepo,
	workspaceStorage *WorkspaceStorage,
	storage storage.Storage,
	packageBucket string,
) *ProblemPublishUseCase {
	return &ProblemPublishUseCase{
		problemsRepo:     problemsRepo,
		packagesRepo:     packagesRepo,
		workspaceStorage: workspaceStorage,
		storage:          storage,
		packageBucket:    packageBucket,
	}
}

func (uc *ProblemPublishUseCase) PublishProblem(
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
		if err := os.WriteFile(fullPath, content, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write workspace file %s: %w", filePath, err)
		}
	}

	// Validate the package layout by loading problem.yaml
	_, err = gfmt.OpenPackage(tempDir)
	if err != nil {
		return nil, fmt.Errorf("invalid problem package: %w", err)
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
	created, err := uc.packagesRepo.CreatePackage(ctx, &models.CreatePackageParams{
		ID:             packageID,
		ProblemID:      problemID,
		OrganizationID: problem.OrganizationID,
		PackageHash:    packageHash,
		Status:         "building",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create package record: %w", err)
	}

	bgCtx := context.Background()
	if err := uc.packagesRepo.UpdatePackageStatus(bgCtx, &models.UpdatePackageStatusParams{
		ID:     packageID,
		Status: "ready",
		URL:    nil,
	}); err != nil {
		return nil, fmt.Errorf("failed to update package status to ready: %w", err)
	}

	return &PublishResult{
		Version:   created.Version,
		PackageID: packageID,
	}, nil
}

func (uc *ProblemPublishUseCase) ListPackages(
	ctx context.Context,
	problemID uuid.UUID,
) ([]models.ProblemPackage, error) {
	return uc.packagesRepo.ListPackages(ctx, problemID, 100, 0)
}

func (uc *ProblemPublishUseCase) GetPublishedPackageURL(
	ctx context.Context,
	problemID uuid.UUID,
	version string,
) (string, error) {
	version = strings.TrimPrefix(version, "v")
	versionNum, err := strconv.ParseInt(version, 10, 32)
	if err != nil {
		return "", fmt.Errorf("invalid version: %w", err)
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
) (models.ProblemPackage, error) {
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
	defer archive.Close()

	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		fWriter, err := archive.Create(rel)
		if err != nil {
			return err
		}

		_, err = io.Copy(fWriter, file)
		return err
	})

	return err
}
