package usecase

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gate149/core/pkg"
	"github.com/gate149/core/pkg/packagegen"
	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/google/uuid"
)

type ProblemPublishUseCase struct {
	problemsRepo     interfaces.ProblemsRepo
	s3Client         *pkg.S3Client
	packageBucket    string
	workshopReposDir string
}

func NewProblemPublishUseCase(
	problemsRepo interfaces.ProblemsRepo,
	s3Client *pkg.S3Client,
	packageBucket string,
	workshopReposDir string,
) *ProblemPublishUseCase {
	return &ProblemPublishUseCase{
		problemsRepo:     problemsRepo,
		s3Client:         s3Client,
		packageBucket:    packageBucket,
		workshopReposDir: workshopReposDir,
	}
}

// PublishProblem publishes a problem by creating a package and uploading to S3
func (uc *ProblemPublishUseCase) PublishProblem(
	ctx context.Context,
	problemID uuid.UUID,
	version string,
	commitSHA string,
) error {
	// Get problem from database
	_, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return fmt.Errorf("failed to get problem: %w", err)
	}

	// Build problem directory path
	problemDir := fmt.Sprintf("%s/%s", uc.workshopReposDir, problemID.String())

	// Build package from repository
	pkg, err := packagegen.BuildPackage(problemDir)
	if err != nil {
		return fmt.Errorf("failed to build package: %w", err)
	}

	// Validate package structure
	if err := packagegen.ValidatePackage(pkg); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Generate ZIP file in memory
	var zipBuffer bytes.Buffer
	if err := packagegen.WritePackageToZip(pkg, &zipBuffer); err != nil {
		return fmt.Errorf("failed to create ZIP: %w", err)
	}

	// Upload to S3 with key: problems/{problem_id}/{version}.zip
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), version)
	err = uc.s3Client.UploadFile(
		ctx,
		uc.packageBucket,
		s3Key,
		bytes.NewReader(zipBuffer.Bytes()),
		"application/zip",
	)
	if err != nil {
		return fmt.Errorf("failed to upload package to S3: %w", err)
	}

	// Generate package URL (presigned URL valid for 1 year)
	_, err = uc.s3Client.GetPresignedURL(ctx, uc.packageBucket, s3Key, 365*24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate package URL: %w", err)
	}

	// Update problem record with publishing information
	// Note: This requires adding UpdateProblemPublishing method to ProblemsRepo
	// For now, we'll use a simplified approach
	
	// TODO: Add proper database update method
	// err = uc.problemsRepo.UpdateProblemPublishing(ctx, problemID, version, packageURL, commitSHA)
	// if err != nil {
	// 	return fmt.Errorf("failed to update problem record: %w", err)
	// }

	return nil
}

// GetPublishedPackageURL retrieves the package URL for a published problem
func (uc *ProblemPublishUseCase) GetPublishedPackageURL(
	ctx context.Context,
	problemID uuid.UUID,
	version string,
) (string, error) {
	s3Key := fmt.Sprintf("problems/%s/%s.zip", problemID.String(), version)
	
	// Generate presigned URL (valid for 1 hour)
	packageURL, err := uc.s3Client.GetPresignedURL(ctx, uc.packageBucket, s3Key, 1*time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate package URL: %w", err)
	}

	return packageURL, nil
}
