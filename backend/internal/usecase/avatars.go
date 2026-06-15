package usecase

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type AvatarsUseCase struct {
	usersRepo    interfaces.UsersRepo
	storage      storage.Storage
	avatarBucket string
}

func NewAvatarsUseCase(
	usersRepo interfaces.UsersRepo,
	storage storage.Storage,
	avatarBucket string,
) *AvatarsUseCase {
	return &AvatarsUseCase{
		usersRepo:    usersRepo,
		storage:      storage,
		avatarBucket: avatarBucket,
	}
}

type AvatarImage struct {
	readCloser io.ReadCloser
	etag       string
}

func (a AvatarImage) ReadCloser() io.ReadCloser {
	return a.readCloser
}

func (a AvatarImage) Etag() string {
	return a.etag
}

// UploadAvatar uploads a user's avatar to storage and updates the user record
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

	// Generate unique image ID
	imgID := uuid.New().String()
	key := imgID

	// Upload to storage
	err := uc.storage.UploadFile(ctx, uc.avatarBucket, key, fileReader, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload avatar: %w", err)
	}

	// Update user record with avatar ID (stored in avatar_url column)
	err = uc.usersRepo.UpdateUser(ctx, models.UpdateUserParams{
		Id:        userID,
		AvatarUrl: &imgID,
	})
	if err != nil {
		// Try to clean up uploaded file
		_ = uc.storage.DeleteFile(ctx, uc.avatarBucket, key)
		return "", fmt.Errorf("failed to update user avatar: %w", err)
	}

	return imgID, nil
}

// DeleteAvatar deletes a user's avatar from storage and updates the user record
func (uc *AvatarsUseCase) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	// Get user to find current avatar
	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.AvatarUrl == nil || *user.AvatarUrl == "" {
		return fmt.Errorf("user has no avatar")
	}

	// Delete the avatar file
	err = uc.storage.DeleteFile(ctx, uc.avatarBucket, *user.AvatarUrl)
	if err != nil {
		// Log error but continue to update user record
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

// GetAvatar retrieves a user's avatar from storage
func (uc *AvatarsUseCase) GetAvatar(ctx context.Context, userID uuid.UUID, ifNoneMatch *string) (AvatarImage, error) {
	// Get user to find current avatar key
	user, err := uc.usersRepo.GetUserById(ctx, userID)
	if err != nil {
		return AvatarImage{}, fmt.Errorf("failed to get user: %w", err)
	}

	if user.AvatarUrl == nil || *user.AvatarUrl == "" {
		return AvatarImage{}, storage.ErrNotFound
	}

	body, etag, err := uc.storage.DownloadFile(ctx, uc.avatarBucket, *user.AvatarUrl, ifNoneMatch)
	if err != nil {
		return AvatarImage{}, err
	}

	return AvatarImage{
		readCloser: body,
		etag:       etag,
	}, nil
}
