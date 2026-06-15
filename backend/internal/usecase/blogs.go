package usecase

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

// BlogsUseCase handles blog post business logic
type BlogsUseCase struct {
	blogsRepo   interfaces.BlogsRepo
	storage     storage.Storage
	imageBucket string
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

// NewBlogsUseCase creates a new BlogsUseCase
func NewBlogsUseCase(
	blogsRepo interfaces.BlogsRepo,
	storage storage.Storage,
	imageBucket string,
) *BlogsUseCase {
	return &BlogsUseCase{
		blogsRepo:   blogsRepo,
		storage:     storage,
		imageBucket: imageBucket,
	}
}

// CreatePost creates a new blog post with an image
func (uc *BlogsUseCase) CreatePost(
	ctx context.Context,
	title, text, description string,
	authorUUID uuid.UUID,
	authorName string,
	imageReader io.Reader,
	filename string,
) (uuid.UUID, error) {
	// Validate and process image
	imageKey, err := uc.uploadImage(ctx, imageReader, filename)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Create post in database
	postID, err := uc.blogsRepo.CreatePost(ctx, models.CreatePostParams{
		Title:       title,
		Text:        text,
		Description: description,
		AuthorUUID:  authorUUID,
		AuthorName:  authorName,
		ImageKey:    imageKey,
	})

	if err != nil {
		// Clean up uploaded image on failure
		_ = uc.storage.DeleteFile(ctx, uc.imageBucket, imageKey)
		return uuid.Nil, fmt.Errorf("failed to create post: %w", err)
	}

	return postID, nil
}

// GetPost retrieves a post by ID
func (uc *BlogsUseCase) GetPost(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	post, err := uc.blogsRepo.GetPostByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	return post, nil
}

// ListPosts retrieves a paginated list of posts
func (uc *BlogsUseCase) ListPosts(
	ctx context.Context,
	page, pageSize int,
	sortOrder string,
) (*models.ListPostsResponse, error) {
	// Validate parameters
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	posts, totalPages, err := uc.blogsRepo.ListPosts(ctx, models.ListPostsParams{
		Page:      page,
		PageSize:  pageSize,
		SortOrder: sortOrder,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	return &models.ListPostsResponse{
		Posts:      posts,
		TotalPages: totalPages,
		Page:       page,
	}, nil
}

// UpdatePost updates an existing post
func (uc *BlogsUseCase) UpdatePost(
	ctx context.Context,
	id uuid.UUID,
	title, text, description *string,
	imageReader io.Reader,
	filename string,
) error {
	// Get existing post
	post, err := uc.blogsRepo.GetPostByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	var newImageKey *string
	if imageReader != nil {
		// Upload new image
		imageKey, err := uc.uploadImage(ctx, imageReader, filename)
		if err != nil {
			return fmt.Errorf("failed to upload image: %w", err)
		}
		newImageKey = &imageKey
	}

	// Update post
	err = uc.blogsRepo.UpdatePost(ctx, models.UpdatePostParams{
		ID:          id,
		Title:       title,
		Text:        text,
		Description: description,
		ImageKey:    newImageKey,
	})

	if err != nil {
		// Clean up new image on failure
		if newImageKey != nil {
			_ = uc.storage.DeleteFile(ctx, uc.imageBucket, *newImageKey)
		}
		return fmt.Errorf("failed to update post: %w", err)
	}

	// Delete old image if new one was uploaded
	if newImageKey != nil && post.ImageKey != "" {
		_ = uc.storage.DeleteFile(ctx, uc.imageBucket, post.ImageKey)
	}

	return nil
}

// DeletePost deletes a post and its associated image
func (uc *BlogsUseCase) DeletePost(ctx context.Context, id uuid.UUID) error {
	// Get post to find image key
	post, err := uc.blogsRepo.GetPostByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	// Delete post from database
	err = uc.blogsRepo.DeletePost(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	// Delete image from S3 (best effort)
	if post.ImageKey != "" {
		_ = uc.storage.DeleteFile(ctx, uc.imageBucket, post.ImageKey)
	}

	return nil
}

// GetPostImage retrieves a post image from storage
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

// uploadImage validates, processes, and uploads an image to S3
func (uc *BlogsUseCase) uploadImage(ctx context.Context, imageReader io.Reader, filename string) (string, error) {
	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}

	if !allowedExts[ext] {
		return "", fmt.Errorf("invalid file extension: %s (allowed: jpg, jpeg, png, gif)", ext)
	}

	// Decode image
	img, _, err := image.Decode(imageReader)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("failed to encode image to PNG: %w", err)
	}

	// Generate unique key: {timestamp}-{uuid}.png
	timestamp := time.Now().Unix()
	randomID := uuid.New().String()[:8]
	key := fmt.Sprintf("%d-%s.png", timestamp, randomID)

	// Upload to S3
	err = uc.storage.UploadFile(ctx, uc.imageBucket, key, &buf, "image/png")
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return key, nil
}
