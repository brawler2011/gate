package usecase

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type AvatarsUseCase struct {
	usersRepo    interfaces.UsersRepo
	s3Client     *pkg.S3Client
	avatarBucket string
}

func NewAvatarsUseCase(
	usersRepo interfaces.UsersRepo,
	s3Client *pkg.S3Client,
	avatarBucket string,
) *AvatarsUseCase {
	return &AvatarsUseCase{
		usersRepo:    usersRepo,
		s3Client:     s3Client,
		avatarBucket: avatarBucket,
	}
}

// UploadAvatar uploads a user's avatar to S3 and updates the user record
func (uc *AvatarsUseCase) UploadAvatar(
	ctx context.Context,
	userID uuid.UUID,
	fileReader io.Reader,
	filename string,
	contentType string,
) (string, error) {
	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
	if !allowedExts[ext] {
		return "", fmt.Errorf("invalid file extension: %s (allowed: jpg, jpeg, png, gif, webp)", ext)
	}

	// Generate unique key: {user_id}/{timestamp}-{random}{ext}
	timestamp := time.Now().Unix()
	randomID := uuid.New().String()[:8]
	key := fmt.Sprintf("%s/%d-%s%s", userID.String(), timestamp, randomID, ext)

	// Upload to S3
	err := uc.s3Client.UploadFile(ctx, uc.avatarBucket, key, fileReader, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload avatar: %w", err)
	}

	// Generate presigned URL (valid for 7 days)
	avatarURL, err := uc.s3Client.GetPresignedURL(ctx, uc.avatarBucket, key, 7*24*time.Hour)
	if err != nil {
		// Try to clean up uploaded file
		_ = uc.s3Client.DeleteFile(ctx, uc.avatarBucket, key)
		return "", fmt.Errorf("failed to generate avatar URL: %w", err)
	}

	// Update user record with avatar URL
	err = uc.usersRepo.UpdateUser(ctx, models.UpdateUserParams{
		Id:        userID,
		AvatarUrl: &avatarURL,
	})
	if err != nil {
		// Try to clean up uploaded file
		_ = uc.s3Client.DeleteFile(ctx, uc.avatarBucket, key)
		return "", fmt.Errorf("failed to update user avatar: %w", err)
	}

	return avatarURL, nil
}

// DeleteAvatar deletes a user's avatar from S3 and updates the user record
func (uc *AvatarsUseCase) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	// Get user to find current avatar
	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.AvatarUrl == nil || *user.AvatarUrl == "" {
		return fmt.Errorf("user has no avatar")
	}

	// Extract key from URL (assuming format: https://endpoint/bucket/key)
	// For simplicity, we'll list all files for this user and delete them
	prefix := fmt.Sprintf("%s/", userID.String())
	keys, err := uc.s3Client.ListFiles(ctx, uc.avatarBucket, prefix)
	if err != nil {
		return fmt.Errorf("failed to list avatar files: %w", err)
	}

	// Delete all avatar files for this user
	for _, key := range keys {
		err := uc.s3Client.DeleteFile(ctx, uc.avatarBucket, key)
		if err != nil {
			// Log error but continue
			continue
		}
	}

	// Update user record to remove avatar URL
	emptyURL := ""
	err = uc.usersRepo.UpdateUser(ctx, models.UpdateUserParams{
		Id:        userID,
		AvatarUrl: &emptyURL,
	})
	if err != nil {
		return fmt.Errorf("failed to update user record: %w", err)
	}

	return nil
}
