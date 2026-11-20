package blogs

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type BlogsUseCase struct {
	transactions Transactions
	storage      storage.Storage
	imageBucket  string
}

type PostImage struct {
	readCloser  io.ReadCloser
	contentType string
	etag        string
}

func (p PostImage) ReadCloser() io.ReadCloser {
	return p.readCloser
}

func (p PostImage) ContentType() string {
	return p.contentType
}

func (p PostImage) Etag() string {
	return p.etag
}

func NewBlogsUseCase(
	transactions Transactions,
	storage storage.Storage,
	imageBucket string,
) *BlogsUseCase {
	return &BlogsUseCase{
		transactions: transactions,
		storage:      storage,
		imageBucket:  imageBucket,
	}
}

func (uc *BlogsUseCase) CreatePost(
	ctx context.Context,
	title, text, description string,
	imageReader io.Reader,
	filename string,
) (uuid.UUID, error) {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return uuid.Nil, err
	}
	imageKey, err := uc.uploadImage(ctx, imageReader, filename)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to upload image: %w", err)
	}

	var postID uuid.UUID
	err = uc.withinAdmin(ctx, func(store TxStore, actor accessmodel.User) error {
		var err error
		postID, err = store.CreatePost(ctx, blogsmodel.CreatePostParams{
			Title: title, Text: text, Description: description, AuthorUUID: actor.Id, AuthorName: actor.Username, ImageKey: imageKey,
		})
		return err
	})

	if err != nil {
		uc.deleteImage(ctx, imageKey)
		return uuid.Nil, fmt.Errorf("failed to create post: %w", err)
	}

	return postID, nil
}

func (uc *BlogsUseCase) UpdatePost(
	ctx context.Context,
	id uuid.UUID,
	title, text, description *string,
	imageReader io.Reader,
	filename string,
) error {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return err
	}

	var newImageKey *string
	if imageReader != nil {
		imageKey, err := uc.uploadImage(ctx, imageReader, filename)
		if err != nil {
			return fmt.Errorf("failed to upload image: %w", err)
		}
		newImageKey = &imageKey
	}

	var oldImageKey string
	err := uc.withinAdmin(ctx, func(store TxStore, _ accessmodel.User) error {
		if err := store.LockPost(ctx, id); err != nil {
			return err
		}
		post, err := store.GetPostByID(ctx, id)
		if err != nil {
			return err
		}
		oldImageKey = post.ImageKey
		return store.UpdatePost(ctx, blogsmodel.UpdatePostParams{ID: id, Title: title, Text: text, Description: description, ImageKey: newImageKey})
	})

	if err != nil {
		if newImageKey != nil {
			uc.deleteImage(ctx, *newImageKey)
		}
		return fmt.Errorf("failed to update post: %w", err)
	}

	if newImageKey != nil && oldImageKey != "" {
		uc.deleteImage(ctx, oldImageKey)
	}

	return nil
}

func (uc *BlogsUseCase) DeletePost(ctx context.Context, id uuid.UUID) error {
	if err := accessmodel.RequireAdmin(ctx); err != nil {
		return err
	}
	var oldImageKey string
	err := uc.withinAdmin(ctx, func(store TxStore, _ accessmodel.User) error {
		if err := store.LockPost(ctx, id); err != nil {
			return err
		}
		post, err := store.GetPostByID(ctx, id)
		if err != nil {
			return err
		}
		oldImageKey = post.ImageKey
		return store.DeletePost(ctx, id)
	})
	if err != nil {
		return err
	}
	if oldImageKey != "" {
		uc.deleteImage(ctx, oldImageKey)
	}

	return nil
}

func (uc *BlogsUseCase) GetPostImage(ctx context.Context, imageKey string, ifNoneMatch *string) (PostImage, error) {
	body, etag, err := uc.storage.DownloadFile(ctx, uc.imageBucket, imageKey, ifNoneMatch)
	if err != nil {
		return PostImage{}, fmt.Errorf("failed to download image: %w", err)
	}

	return PostImage{
		readCloser:  body,
		contentType: "image/png",
		etag:        etag,
	}, nil
}

func (uc *BlogsUseCase) uploadImage(ctx context.Context, imageReader io.Reader, filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}

	if !allowedExts[ext] {
		return "", pkg.Wrap(pkg.ErrInvalid, nil, "invalid image file extension")
	}

	img, _, err := image.Decode(imageReader)
	if err != nil {
		return "", pkg.Wrap(pkg.ErrInvalid, err, "failed to decode image")
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode image to PNG: %w", err)
	}

	timestamp := time.Now().Unix()
	randomID := uuid.New().String()[:8]
	key := fmt.Sprintf("%d-%s.png", timestamp, randomID)

	err = uc.storage.UploadFile(ctx, uc.imageBucket, key, &buf, "image/png")
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return key, nil
}

func (uc *BlogsUseCase) withinAdmin(ctx context.Context, fn func(TxStore, accessmodel.User) error) error {
	actor := accessmodel.UserFromContext(ctx)
	return uc.transactions.Within(ctx, func(store TxStore) error {
		user, err := store.GetUserById(ctx, actor.Id)
		if err != nil {
			return err
		}
		if user.IsExpired() || !user.IsAdmin() {
			return pkg.ErrForbidden
		}
		return fn(store, user)
	})
}
func (uc *BlogsUseCase) deleteImage(ctx context.Context, key string) {
	if err := uc.storage.DeleteFile(ctx, uc.imageBucket, key); err != nil {
		slog.WarnContext(ctx, "failed to delete blog image", "key", key, "error", err)
	}
}
