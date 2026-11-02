package pg

import (
	"context"
	"fmt"
	"math"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BlogsRepo implements the BlogsRepo interface
type BlogsRepo struct {
	queries *sqlc.Queries
}

// NewBlogsRepo creates a new BlogsRepo
func NewBlogsRepo(pool *pgxpool.Pool) *BlogsRepo {
	return &BlogsRepo{queries: sqlc.New(pool)}
}

// CreatePost creates a new blog post
func (r *BlogsRepo) CreatePost(ctx context.Context, params models.CreatePostParams) (uuid.UUID, error) {
	id, err := r.queries.CreatePost(ctx, sqlc.CreatePostParams{
		Title:       params.Title,
		Text:        params.Text,
		Description: params.Description,
		AuthorUuid:  params.AuthorUUID,
		AuthorName:  params.AuthorName,
		ImageKey:    params.ImageKey,
	})

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create post: %w", err)
	}

	return id, nil
}

// GetPostByID retrieves a post by its ID
func (r *BlogsRepo) GetPostByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	row, err := r.queries.GetPostByID(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	post := postFromSQLC(row)
	return &post, nil
}

// ListPosts retrieves a paginated list of posts
func (r *BlogsRepo) ListPosts(ctx context.Context, params models.ListPostsParams) ([]models.Post, int, error) {
	// Get total count
	totalCount, err := r.queries.CountPosts(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count posts: %w", err)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(params.PageSize)))

	// Build query based on sort order
	offset := (params.Page - 1) * params.PageSize
	var rows []sqlc.Post
	if params.SortOrder == "asc" {
		rows, err = r.queries.ListPostsAsc(ctx, sqlc.ListPostsAscParams{
			Limit:  safeInt32(params.PageSize),
			Offset: safeInt32(offset),
		})
	} else {
		rows, err = r.queries.ListPostsDesc(ctx, sqlc.ListPostsDescParams{
			Limit:  safeInt32(params.PageSize),
			Offset: safeInt32(offset),
		})
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list posts: %w", err)
	}
	var posts []models.Post
	for _, row := range rows {
		posts = append(posts, postFromSQLC(row))
	}

	return posts, totalPages, nil
}

// UpdatePost updates an existing post
func (r *BlogsRepo) UpdatePost(ctx context.Context, params models.UpdatePostParams) error {
	affected, err := r.queries.UpdatePost(ctx, sqlc.UpdatePostParams{
		ID:          params.ID,
		Title:       params.Title,
		Text:        params.Text,
		Description: params.Description,
		ImageKey:    params.ImageKey,
	})

	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

// DeletePost deletes a post by its ID
func (r *BlogsRepo) DeletePost(ctx context.Context, id uuid.UUID) error {
	affected, err := r.queries.DeletePost(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	if affected == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

func postFromSQLC(row sqlc.Post) models.Post {
	return models.Post{
		ID:          row.ID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		Title:       row.Title,
		Text:        row.Text,
		Description: row.Description,
		AuthorUUID:  row.AuthorUuid,
		AuthorName:  row.AuthorName,
		ImageKey:    row.ImageKey,
	}
}
