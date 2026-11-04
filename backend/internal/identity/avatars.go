package identity

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type AvatarsUseCase struct {
	usersRepo    UsersRepo
	transactions IdentityTransactions
	storage      storage.Storage
	avatarBucket string
}

func NewAvatarsUseCase(
	usersRepo UsersRepo,
	transactions IdentityTransactions,
	storage storage.Storage,
	avatarBucket string,
) *AvatarsUseCase {
	return &AvatarsUseCase{
		usersRepo:    usersRepo,
		transactions: transactions,
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
	username string,
	fileReader io.Reader,
	filename string,
	contentType string,
) (string, error) {
	username = strings.TrimPrefix(username, "@") // FIXME: зачем?? Как это вообще возможно?
	user, err := uc.usersRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	if err := accessmodel.RequireSelfOrAdmin(ctx, user.Id); err != nil {
		return "", err
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filename)) // FIXME: че? почему мы по filename определяем контент? что насчёт contentType???
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
	if !allowedExts[ext] {
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "invalid avatar file extension")
	}

	// Generate unique image ID
	imgID := uuid.New().String()
	key := imgID // FIXME: зачем просто так переменную копировать?

	// Upload to storage
	err = uc.storage.UploadFile(ctx, uc.avatarBucket, key, fileReader, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to upload avatar: %w", err)
	}

	var oldAvatar *string
	err = withinAccount(ctx, uc.transactions, user.Id, false, func(store IdentityTxStore) error {
		current, err := store.GetUserById(ctx, user.Id)
		if err != nil {
			return err
		}
		oldAvatar = current.AvatarUrl
		return store.UpdateUser(ctx, identitymodel.UpdateUserParams{Id: user.Id, AvatarUrl: &imgID})
	})

	if err != nil {
		// Try to clean up uploaded file
		uc.deleteAvatarFile(ctx, key)
		return "", fmt.Errorf("failed to update user avatar: %w", err)
	}

	if oldAvatar != nil && *oldAvatar != "" {
		uc.deleteAvatarFile(ctx, *oldAvatar)
	}
	return imgID, nil
}

// DeleteAvatar deletes a user's avatar from storage and updates the user record
func (uc *AvatarsUseCase) DeleteAvatar(ctx context.Context, username string) error {
	username = strings.TrimPrefix(username, "@") // FIXME: блядь что??
	user, err := uc.usersRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if err := accessmodel.RequireSelfOrAdmin(ctx, user.Id); err != nil {
		return err
	}

	var oldAvatar string
	err = withinAccount(ctx, uc.transactions, user.Id, false, func(store IdentityTxStore) error {
		current, err := store.GetUserById(ctx, user.Id)
		if err != nil {
			return err
		}
		if current.AvatarUrl == nil || *current.AvatarUrl == "" {
			return pkg.ErrNotFound
		}
		oldAvatar = *current.AvatarUrl
		emptyURL := ""
		return store.UpdateUser(ctx, identitymodel.UpdateUserParams{Id: user.Id, AvatarUrl: &emptyURL})
	})
	if err != nil {
		return err
	}
	uc.deleteAvatarFile(ctx, oldAvatar)

	return nil
}

// GetAvatar retrieves a user's avatar from storage
func (uc *AvatarsUseCase) GetAvatar(ctx context.Context, username string, ifNoneMatch *string) (AvatarImage, error) {
	username = strings.TrimPrefix(username, "@") // FIXME: бред!
	user, err := uc.usersRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return AvatarImage{}, fmt.Errorf("failed to get user: %w", err)
	}

	// FIXME: invariant: null eq ""
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

func (uc *AvatarsUseCase) deleteAvatarFile(ctx context.Context, key string) {
	if err := uc.storage.DeleteFile(ctx, uc.avatarBucket, key); err != nil {
		slog.Warn("failed to delete avatar file", "key", key, "error", err)
	}
}
