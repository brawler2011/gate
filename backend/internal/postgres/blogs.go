package postgres

import (
	"context"

	"github.com/brawler2011/gate/backend/pkg"

	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BlogsRepo struct {
	queries *sqlc.Queries
}

func NewBlogsRepo(pool *pgxpool.Pool) *BlogsRepo {
	return &BlogsRepo{queries: sqlc.New(pool)}
}

func (r *BlogsRepo) CreatePost(ctx context.Context, params blogsmodel.CreatePostParams) (uuid.UUID, error) {
	id, err := r.queries.CreatePost(ctx, sqlc.CreatePostParams{
		Title:       params.Title,
		Text:        params.Text,
		Description: params.Description,
		AuthorID:    params.AuthorID,
		ImageKey:    params.ImageKey,
	})

	if err != nil {
		return uuid.Nil, dbError("create post", err)
	}

	return id, nil
}

func (r *BlogsRepo) GetPostByID(ctx context.Context, id uuid.UUID) (*blogsmodel.Post, error) {
	row, err := r.queries.GetPostByID(ctx, id)

	if err != nil {
		return nil, dbError("get post by id", err)
	}

	post := postFromSQLC(row)
	return &post, nil
}

func (r *BlogsRepo) ListPosts(ctx context.Context, params blogsmodel.ListPostsParams) ([]blogsmodel.Post, int, error) {
	totalCount, err := r.queries.CountPosts(ctx)
	if err != nil {
		return nil, 0, dbError("list posts", err)
	}

	offset := (params.Page - 1) * params.PageSize
	var posts []blogsmodel.Post
	if params.SortOrder == "asc" {
		var rows []sqlc.ListPostsAscRow
		rows, err = r.queries.ListPostsAsc(ctx, sqlc.ListPostsAscParams{
			Limit:  safeInt32(params.PageSize),
			Offset: safeInt32(offset),
		})
		for _, row := range rows {
			posts = append(posts, postFromSQLC(sqlc.GetPostByIDRow(row)))
		}
	} else {
		var rows []sqlc.ListPostsDescRow
		rows, err = r.queries.ListPostsDesc(ctx, sqlc.ListPostsDescParams{
			Limit:  safeInt32(params.PageSize),
			Offset: safeInt32(offset),
		})
		for _, row := range rows {
			posts = append(posts, postFromSQLC(sqlc.GetPostByIDRow(row)))
		}
	}
	if err != nil {
		return nil, 0, dbError("list posts", err)
	}

	return posts, int(totalCount), nil
}

func (r *BlogsRepo) UpdatePost(ctx context.Context, params blogsmodel.UpdatePostParams) error {
	affected, err := r.queries.UpdatePost(ctx, sqlc.UpdatePostParams{
		ID:          params.ID,
		Title:       params.Title,
		Text:        params.Text,
		Description: params.Description,
		ImageKey:    params.ImageKey,
	})

	if err != nil {
		return dbError("update post", err)
	}

	if affected == 0 {
		return pkg.ErrNotFound
	}

	return nil
}

func (r *BlogsRepo) DeletePost(ctx context.Context, id uuid.UUID) error {
	affected, err := r.queries.DeletePost(ctx, id)
	if err != nil {
		return dbError("delete post", err)
	}

	if affected == 0 {
		return pkg.ErrNotFound
	}

	return nil
}

func postFromSQLC(row sqlc.GetPostByIDRow) blogsmodel.Post {
	return blogsmodel.Post{
		ID:          row.ID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		Title:       row.Title,
		Text:        row.Text,
		Description: row.Description,
		AuthorID:    row.AuthorID,
		AuthorName:  row.AuthorName,
		ImageKey:    row.ImageKey,
	}
}

func (r *BlogsRepo) LockPost(ctx context.Context, id uuid.UUID) error {
	_, err := r.queries.LockPost(ctx, id)
	return dbError("lock post", err)
}
