package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/packagegen"
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
	s3Client      *pkg.S3Client
	packageBucket string
}

func NewProblemPublishUseCase(
	problemsRepo interfaces.ProblemsRepo,
	packagesRepo interfaces.PackagesRepo,
	vcsService vcs.Service,
	s3Client *pkg.S3Client,
	packageBucket string,
) *ProblemPublishUseCase {
	return &ProblemPublishUseCase{
		problemsRepo:  problemsRepo,
		packagesRepo:  packagesRepo,
		vcsService:    vcsService,
		s3Client:      s3Client,
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

	gitHash, err := uc.vcsService.GetCurrentSHA(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get git commit hash: %w", err)
	}

	problemDir := uc.vcsService.GetRepoPath(problemID)

	builtPkg, err := packagegen.BuildPackage(problemDir)
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

	packageID := uuid.New()
	created, err := uc.packagesRepo.CreatePackage(ctx, &models.CreatePackageParams{
		ID:             packageID,
		ProblemID:      problemID,
		OrganizationID: problem.OrganizationID,
		GitCommitHash:  gitHash,
		PackageHash:    packageHash,
		Status:         "building",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create package record: %w", err)
	}

	versionStr := fmt.Sprintf("v%d", created.Version)
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), versionStr)
	uploadErr := uc.s3Client.UploadFile(
		ctx,
		uc.packageBucket,
		s3Key,
		bytes.NewReader(zipBytes),
		"application/zip",
	)

	// Use a background context for status updates so they are never cancelled
	// by the incoming request context (e.g. client disconnect or deadline).
	bgCtx := context.Background()

	if uploadErr != nil {
		buildLog := uploadErr.Error()
		updateErr := uc.packagesRepo.UpdatePackageStatus(bgCtx, &models.UpdatePackageStatusParams{
			ID:       packageID,
			Status:   "failed",
			BuildLog: &buildLog,
		})
		if updateErr != nil {
			return nil, fmt.Errorf("failed to upload package to S3 (%w) and failed to update package status: %v", uploadErr, updateErr)
		}
		return nil, fmt.Errorf("failed to upload package to S3: %w", uploadErr)
	}

	if err := uc.packagesRepo.UpdatePackageStatus(bgCtx, &models.UpdatePackageStatusParams{
		ID:     packageID,
		Status: "ready",
		URL:    nil,
	}); err != nil {
		return nil, fmt.Errorf("failed to update package status to ready: %w", err)
	}

	// Update the problems table so the judge knows which version to load.
	// problems.git_commit_hash stores the version key used as the S3 filename.
	if err := uc.problemsRepo.UpdateProblem(bgCtx, problemID, &models.ProblemUpdate{
		GitCommitHash: &versionStr,
	}); err != nil {
		return nil, fmt.Errorf("failed to update problem git commit hash: %w", err)
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
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), version)
	packageURL, err := uc.s3Client.GetPresignedURL(ctx, uc.packageBucket, s3Key, 1*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate package URL: %w", err)
	}
	return packageURL, nil
}
