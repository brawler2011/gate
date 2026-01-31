package pg

import (
	"context"
	"fmt"
	"math"

	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BlogsRepo implements the BlogsRepo interface
type BlogsRepo struct {
	pool *pgxpool.Pool
}

// NewBlogsRepo creates a new BlogsRepo
func NewBlogsRepo(pool *pgxpool.Pool) *BlogsRepo {
	return &BlogsRepo{pool: pool}
}

// CreatePost creates a new blog post
func (r *BlogsRepo) CreatePost(ctx context.Context, params models.CreatePostParams) (uuid.UUID, error) {
	query := `
		INSERT INTO posts (title, text, description, author_uuid, author_name, image_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	var id uuid.UUID
	err := r.pool.QueryRow(ctx, query,
		params.Title,
		params.Text,
		params.Description,
		params.AuthorUUID,
		params.AuthorName,
		params.ImageKey,
	).Scan(&id)

	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create post: %w", err)
	}

	return id, nil
}

// GetPostByID retrieves a post by its ID
func (r *BlogsRepo) GetPostByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	query := `
		SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
		FROM posts
		WHERE id = $1
	`

	var post models.Post
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&post.ID,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.Title,
		&post.Text,
		&post.Description,
		&post.AuthorUUID,
		&post.AuthorName,
		&post.ImageKey,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	return &post, nil
}

// ListPosts retrieves a paginated list of posts
func (r *BlogsRepo) ListPosts(ctx context.Context, params models.ListPostsParams) ([]models.Post, int, error) {
	// Get total count
	var totalCount int
	countQuery := `SELECT COUNT(*) FROM posts`
	err := r.pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count posts: %w", err)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(totalCount) / float64(params.PageSize)))

	// Build query based on sort order
	var query string
	if params.SortOrder == "asc" {
		query = `
			SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
			FROM posts
			ORDER BY created_at ASC
			LIMIT $1 OFFSET $2
		`
	} else {
		query = `
			SELECT id, created_at, updated_at, title, text, description, author_uuid, author_name, image_key
			FROM posts
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
	}

	offset := (params.Page - 1) * params.PageSize
	rows, err := r.pool.Query(ctx, query, params.PageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list posts: %w", err)
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var post models.Post
		err := rows.Scan(
			&post.ID,
			&post.CreatedAt,
			&post.UpdatedAt,
			&post.Title,
			&post.Text,
			&post.Description,
			&post.AuthorUUID,
			&post.AuthorName,
			&post.ImageKey,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan post: %w", err)
		}
		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating posts: %w", err)
	}

	return posts, totalPages, nil
}

// UpdatePost updates an existing post
func (r *BlogsRepo) UpdatePost(ctx context.Context, params models.UpdatePostParams) error {
	query := `
		UPDATE posts
		SET title = COALESCE($2, title),
		    text = COALESCE($3, text),
		    description = COALESCE($4, description),
		    image_key = COALESCE($5, image_key),
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		params.ID,
		params.Title,
		params.Text,
		params.Description,
		params.ImageKey,
	)

	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

// DeletePost deletes a post by its ID
func (r *BlogsRepo) DeletePost(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM posts WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}
