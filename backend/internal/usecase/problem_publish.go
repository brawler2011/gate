package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/packagegen"
	"github.com/gate149/gate/backend/pkg/problemformat"
	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/gate149/gate/backend/pkg/vcs"
	"github.com/google/uuid"
)

type PublishResult struct {
	Version   int32
	PackageID uuid.UUID
}

type ProblemPublishUseCase struct {
	problemsRepo  interfaces.ProblemsRepo
	packagesRepo  interfaces.PackagesRepo
	vcsService    vcs.Service
	storage       storage.Storage
	packageBucket string
}

func NewProblemPublishUseCase(
	problemsRepo interfaces.ProblemsRepo,
	packagesRepo interfaces.PackagesRepo,
	vcsService vcs.Service,
	storage storage.Storage,
	packageBucket string,
) *ProblemPublishUseCase {
	return &ProblemPublishUseCase{
		problemsRepo:  problemsRepo,
		packagesRepo:  packagesRepo,
		vcsService:    vcsService,
		storage:       storage,
		packageBucket: packageBucket,
	}
}

// PublishProblem publishes a problem by creating a package and uploading to S3
func (uc *ProblemPublishUseCase) PublishProblem(
	ctx context.Context,
	problemID uuid.UUID,
) (*PublishResult, error) {
	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get problem: %w", err)
	}

	manifestBytes, err := uc.problemsRepo.GetProblemManifest(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get problem manifest: %w", err)
	}
	if len(manifestBytes) == 0 {
		return nil, fmt.Errorf("problem manifest is not initialized")
	}

	var manifest problemformat.ProblemManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "problem-publish-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := os.WriteFile(filepath.Join(tempDir, "manifest.json"), manifestBytes, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write manifest.json: %w", err)
	}

	workspaceFiles, err := uc.vcsService.ListAllFiles(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace files: %w", err)
	}

	for _, filePath := range workspaceFiles {
		if filePath == "manifest.json" {
			continue
		}
		content, err := uc.vcsService.ReadFile(ctx, problemID, filePath)
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

	builtPkg, err := packagegen.BuildPackage(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to build package: %w", err)
	}

	if err := packagegen.ValidatePackage(builtPkg); err != nil {
		return nil, fmt.Errorf("package validation failed: %w", err)
	}

	var zipBuffer bytes.Buffer
	if err := packagegen.WritePackageToZip(builtPkg, &zipBuffer); err != nil {
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

	// Use a background context for status updates so they are never cancelled
	// by the incoming request context (e.g. client disconnect or deadline).
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

// ListPackages returns all packages for a problem ordered by creation date descending.
func (uc *ProblemPublishUseCase) ListPackages(
	ctx context.Context,
	problemID uuid.UUID,
) ([]models.ProblemPackage, error) {
	return uc.packagesRepo.ListPackages(ctx, problemID, 100, 0)
}

// GetPublishedPackageURL returns a presigned S3 URL for the given version.
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
