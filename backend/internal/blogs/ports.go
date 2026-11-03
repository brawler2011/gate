package blogs

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	"github.com/google/uuid"
)

// BlogsRepo defines the interface for blog post repository operations
type BlogsRepo interface {
	// CreatePost creates a new blog post
	CreatePost(ctx context.Context, params blogsmodel.CreatePostParams) (uuid.UUID, error)

	// GetPostByID retrieves a post by its ID
	GetPostByID(ctx context.Context, id uuid.UUID) (*blogsmodel.Post, error)

	// UpdatePost updates an existing post
	UpdatePost(ctx context.Context, params blogsmodel.UpdatePostParams) error

	// DeletePost deletes a post by its ID
	DeletePost(ctx context.Context, id uuid.UUID) error
}

type TxStore interface {
	BlogsRepo
	LockPost(context.Context, uuid.UUID) error
	GetUserById(context.Context, uuid.UUID) (accessmodel.User, error)
}
type Transactions interface {
	Within(context.Context, func(TxStore) error) error
}
