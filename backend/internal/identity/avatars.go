package identity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
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
) (uuid.UUID, error) {
	user, err := uc.usersRepo.GetUserByUsername(ctx, username)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := accessmodel.RequireSelfOrAdmin(ctx, user.Id); err != nil {
		return uuid.Nil, err
	}

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
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid avatar file extension")
	}

	// Decode image to validate content and normalize supported formats to PNG
	img, _, err := image.Decode(fileReader)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to decode avatar image")
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return uuid.Nil, fmt.Errorf("failed to encode avatar to PNG: %w", err)
	}

	// Generate unique image ID
	imgID := uuid.New()
	key := imgID.String()

	// Upload normalized PNG to storage
	err = uc.storage.UploadFile(ctx, uc.avatarBucket, key, &buf, "image/png")
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to upload avatar: %w", err)
	}

	var oldAvatar *string
	err = withinAccount(ctx, uc.transactions, user.Id, false, func(store IdentityTxStore) error {
		current, err := store.GetUserById(ctx, user.Id)
		if err != nil {
			return err
		}
		oldAvatar = current.AvatarUrl
		return store.UpdateUser(ctx, identitymodel.UpdateUserParams{Id: user.Id, AvatarUrl: &key})
	})

	if err != nil {
		// Try to clean up uploaded file
		uc.deleteAvatarFile(ctx, key)
		return uuid.Nil, fmt.Errorf("failed to update user avatar: %w", err)
	}

	if oldAvatar != nil && *oldAvatar != "" {
		uc.deleteAvatarFile(ctx, *oldAvatar)
	}
	return imgID, nil
}

// DeleteAvatar deletes a user's avatar from storage and updates the user record
func (uc *AvatarsUseCase) DeleteAvatar(ctx context.Context, username string) error {
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
	user, err := uc.usersRepo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			return AvatarImage{}, storage.ErrNotFound
		}
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

	// Read magic bytes to determine if the stored file is already a PNG.
	// PNG signature: \x89PNG\r\n\x1a\n (8 bytes)
	const pngSig = "\x89PNG\r\n\x1a\n"
	header := make([]byte, len(pngSig))
	n, readErr := io.ReadFull(body, header)
	if readErr == nil && string(header) == pngSig {
		// Valid PNG already, stream as-is
		reader := io.MultiReader(bytes.NewReader(header), body)
		return AvatarImage{
			readCloser: struct {
				io.Reader
				io.Closer
			}{reader, body},
			etag: etag,
		}, nil
	}

	// Stored object is not PNG (e.g. legacy JPEG/GIF/WebP before upload normalization).
	// Decode stored bytes and convert to PNG to guarantee Content-Type: image/png invariant.
	var fullData []byte
	if n > 0 {
		fullData = append(fullData, header[:n]...)
	}
	rest, _ := io.ReadAll(body)
	_ = body.Close()
	fullData = append(fullData, rest...)

	img, _, decodeErr := image.Decode(bytes.NewReader(fullData))
	if decodeErr != nil {
		return AvatarImage{}, fmt.Errorf("failed to decode stored avatar: %w", decodeErr)
	}

	var buf bytes.Buffer
	if encodeErr := png.Encode(&buf, img); encodeErr != nil {
		return AvatarImage{}, fmt.Errorf("failed to encode stored avatar to PNG: %w", encodeErr)
	}

	return AvatarImage{
		readCloser: io.NopCloser(&buf),
		etag:       etag,
	}, nil
}

func (uc *AvatarsUseCase) deleteAvatarFile(ctx context.Context, key string) {
	if err := uc.storage.DeleteFile(ctx, uc.avatarBucket, key); err != nil {
		slog.Warn("failed to delete avatar file", "key", key, "error", err)
	}
}
