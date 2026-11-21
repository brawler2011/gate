package blogs

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	"github.com/google/uuid"
)

type BlogsRepo interface {
	CreatePost(ctx context.Context, params blogsmodel.CreatePostParams) (uuid.UUID, error)

	GetPostByID(ctx context.Context, id uuid.UUID) (*blogsmodel.Post, error)

	UpdatePost(ctx context.Context, params blogsmodel.UpdatePostParams) error

	DeletePost(ctx context.Context, id uuid.UUID) error
}

type TxStore interface {
	BlogsRepo
	access.Loader
	LockPost(context.Context, uuid.UUID) error
}
type Transactions interface {
	Within(context.Context, func(TxStore) error) error
}
