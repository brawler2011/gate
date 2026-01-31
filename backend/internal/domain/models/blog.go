package models

import (
	"time"

	"github.com/google/uuid"
)

// Post represents a blog post
type Post struct {
	ID          uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Title       string
	Text        string
	Description string
	AuthorUUID  uuid.UUID
	AuthorName  string
	ImageKey    string // S3 key for the post image
}

// CreatePostParams contains parameters for creating a new post
type CreatePostParams struct {
	Title       string
	Text        string
	Description string
	AuthorUUID  uuid.UUID
	AuthorName  string
	ImageKey    string
}

// UpdatePostParams contains parameters for updating a post
type UpdatePostParams struct {
	ID          uuid.UUID
	Title       *string
	Text        *string
	Description *string
	ImageKey    *string
}

// ListPostsParams contains parameters for listing posts
type ListPostsParams struct {
	Page      int
	PageSize  int
	SortOrder string // "asc" or "desc"
}

// ListPostsResponse contains the response for listing posts
type ListPostsResponse struct {
	Posts      []Post
	TotalPages int
	Page       int
}
