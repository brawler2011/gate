package core

import (
	"context"
	"errors"
	"io"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	blogsmodel "github.com/brawler2011/gate/backend/internal/blogs/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type BlogsReader interface {
	ListPosts(ctx context.Context, params blogsmodel.ListPostsParams) ([]blogsmodel.Post, int, error)
	GetPostByID(ctx context.Context, id uuid.UUID) (*blogsmodel.Post, error)
}

func (s *CoreServer) ListPosts(ctx context.Context, params corev1.ListPostsParams) (*corev1.ListPostsResponseModel, error) {
	// FIXME: нужно использовать готовые модели пагинации в openapi.yaml
	// FIXME: удалить этот бойлерплейт
	page := int(params.Page.Or(1))
	pageSize := int(params.PageSize.Or(10))
	sortOrder := string(params.SortOrder.Or("desc"))

	items, totalPages, err := s.blogs.ListPosts(ctx, blogsmodel.ListPostsParams{Page: page, PageSize: pageSize, SortOrder: sortOrder})
	if err != nil {
		return nil, wrapBlogError(err, "failed to list posts")
	}

	// Convert to response model
	posts := make([]corev1.PostModel, len(items))
	for i, post := range items {
		posts[i] = corev1.PostModel{
			ID:             post.ID,
			CreatedAt:      post.CreatedAt,
			UpdatedAt:      post.UpdatedAt,
			Title:          post.Title,
			Text:           post.Text,
			Description:    post.Description,
			PreviewImageID: post.ImageKey,
			AuthorID:       post.AuthorUUID,
			AuthorUsername: post.AuthorName,
		}
	}

	return &corev1.ListPostsResponseModel{
		Pagination: corev1.PaginationModel{
			Total: safeInt32(totalPages), // FIXME: разобраться где возникает проблема типов
			Page:  safeInt32(page),       // FIXME: аналогично
		},
		Posts: posts,
	}, nil
}

// CreatePost implements the CreatePost operation
func (s *CoreServer) CreatePost(ctx context.Context, req *corev1.CreatePostReq) (corev1.CreatePostRes, error) {
	// FIXME: эти провери должны быть в мидлваре
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	if req == nil || !req.Title.IsSet() || !req.Description.IsSet() || !req.Text.IsSet() || !req.PreviewImage.IsSet() {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "title, description, text, and preview_image are required")
	}

	file := req.PreviewImage.Value // FIXME: эта хуйня не должна быть optional
	title := req.Title.Value
	description := req.Description.Value
	text := req.Text.Value

	postID, err := s.blogs.CreatePost(ctx, title, text, description, file.File, file.Name)
	if err != nil {
		return nil, wrapBlogError(err, "failed to create post")
	}

	// FIXME: вроде как в схеме есть reusable структура для таких ответов
	return &corev1.CreatedPost{
		PostID: corev1.NewOptUUID(postID), // FIXME: эта хуйня тоже не должна быть optional
	}, nil
}

func (s *CoreServer) GetPostById(ctx context.Context, params corev1.GetPostByIdParams) (corev1.GetPostByIdRes, error) {
	post, err := s.blogs.GetPostByID(ctx, params.ID)
	if err != nil {
		return nil, wrapBlogError(err, "post not found")
	}

	return &corev1.PostModel{
		ID:             post.ID,
		CreatedAt:      post.CreatedAt,
		UpdatedAt:      post.UpdatedAt,
		Title:          post.Title,
		Text:           post.Text,
		Description:    post.Description,
		PreviewImageID: post.ImageKey,
		AuthorID:       post.AuthorUUID,
		AuthorUsername: post.AuthorName,
	}, nil
}

func (s *CoreServer) PatchPostById(ctx context.Context, req corev1.OptPatchPostByIdReq, params corev1.PatchPostByIdParams) (corev1.PatchPostByIdRes, error) {
	var titlePtr, descriptionPtr, textPtr *string
	var imageReader io.Reader
	var filename string

	if req.IsSet() {
		r := req.Value
		if r.Title.IsSet() {
			titlePtr = &r.Title.Value
		}
		if r.Description.IsSet() {
			descriptionPtr = &r.Description.Value
		}
		if r.Text.IsSet() {
			textPtr = &r.Text.Value
		}
		if r.PreviewImage.IsSet() {
			file := r.PreviewImage.Value
			imageReader = file.File
			filename = file.Name
		}
	}

	err := s.blogs.UpdatePost(ctx, params.ID, titlePtr, textPtr, descriptionPtr, imageReader, filename)
	if err != nil {
		return nil, wrapBlogError(err, "post not found")
	}
	return &corev1.PatchPostByIdOK{}, nil
}

func (s *CoreServer) DeletePostById(ctx context.Context, params corev1.DeletePostByIdParams) (corev1.DeletePostByIdRes, error) {
	err := s.blogs.DeletePost(ctx, params.ID)
	if err != nil {
		return nil, wrapBlogError(err, "post not found")
	}

	return &corev1.DeletePostByIdOK{}, nil
}

func (s *CoreServer) GetPostImage(ctx context.Context, params corev1.GetPostImageParams) (corev1.GetPostImageRes, error) {
	post, err := s.blogs.GetPostByID(ctx, params.ID)
	if err != nil {
		return nil, wrapBlogError(err, "post not found")
	}

	var ifNoneMatch *string
	if params.IfNoneMatch.IsSet() {
		ifNoneMatch = &params.IfNoneMatch.Value
	}

	postImage, imageErr := s.blogs.GetPostImage(ctx, post.ImageKey, ifNoneMatch)

	var re *http.ResponseError
	if errors.Is(imageErr, storage.ErrNotModified) || (errors.As(imageErr, &re) && re.HTTPStatusCode() == 304) {
		return &corev1.GetPostImageNotModified{
			ETag: params.IfNoneMatch,
		}, nil
	} else if imageErr != nil {
		return nil, wrapBlogError(imageErr, "image not found")
	}

	return &corev1.GetPostImageOKHeaders{
		Response: corev1.GetPostImageOK{
			Data: postImage.ReadCloser(),
		},
		ETag: corev1.NewOptString(postImage.Etag()),
	}, nil
}

func wrapBlogError(err error, fallbackMsg string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pkg.NoPermission) || strings.Contains(err.Error(), "access denied") {
		return pkg.Wrap(pkg.NoPermission, err, fallbackMsg)
	}
	if errors.Is(err, pkg.ErrNotFound) || errors.Is(err, storage.ErrNotFound) {
		return pkg.Wrap(pkg.ErrNotFound, err, fallbackMsg)
	}
	if errors.Is(err, pkg.ErrBadInput) {
		return pkg.Wrap(pkg.ErrBadInput, err, fallbackMsg)
	}
	// Adding context must not change the use case error classification.
	// Unclassified failures still map to HTTP 500 in pkg.ToREST.
	return pkg.Wrap(nil, err, fallbackMsg)
}
