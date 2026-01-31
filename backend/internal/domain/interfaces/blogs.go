package interfaces

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

// BlogsRepo defines the interface for blog post repository operations
type BlogsRepo interface {
	// CreatePost creates a new blog post
	CreatePost(ctx context.Context, params models.CreatePostParams) (uuid.UUID, error)

	// GetPostByID retrieves a post by its ID
	GetPostByID(ctx context.Context, id uuid.UUID) (*models.Post, error)

	// ListPosts retrieves a paginated list of posts
	ListPosts(ctx context.Context, params models.ListPostsParams) ([]models.Post, int, error)

	// UpdatePost updates an existing post
	UpdatePost(ctx context.Context, params models.UpdatePostParams) error

	// DeletePost deletes a post by its ID
	DeletePost(ctx context.Context, id uuid.UUID) error
}
