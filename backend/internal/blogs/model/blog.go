package blogsmodel

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Title       string
	Text        string
	Description string
	AuthorUUID  uuid.UUID
	AuthorName  string
	ImageKey    string
}

type CreatePostParams struct {
	Title       string
	Text        string
	Description string
	AuthorUUID  uuid.UUID
	AuthorName  string
	ImageKey    string
}

type UpdatePostParams struct {
	ID          uuid.UUID
	Title       *string
	Text        *string
	Description *string
	ImageKey    *string
}

type ListPostsParams struct {
	Page      int
	PageSize  int
	SortOrder string // "asc" or "desc"
}

type ListPostsResponse struct {
	Posts      []Post
	TotalPages int
	Page       int
}
