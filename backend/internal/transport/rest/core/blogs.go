package core

import (
	"context"
	"errors"
	"io"

	paging "github.com/brawler2011/gate/backend/internal/page"

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
	page := int(params.Page.Or(1))
	pageSize := int(params.PageSize.Or(10))
	sortOrder := string(params.SortOrder.Or("desc"))

	items, totalItems, err := s.BlogReads.ListPosts(ctx, blogsmodel.ListPostsParams{Page: page, PageSize: pageSize, SortOrder: sortOrder})
	if err != nil {
		return nil, err
	}

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
		Pagination: PaginationDTO(paging.NewPagination(safeInt32(page), safeInt32(pageSize), safeInt32(totalItems))),
		Posts:      posts,
	}, nil
}

func (s *CoreServer) CreatePost(ctx context.Context, req *corev1.CreatePostReq) (*corev1.CreatedPost, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}
	required := func(pointer string, missing bool) error {
		if missing {
			return pkg.AtField(pointer, pkg.Invalid(pkg.ErrRequired, pointer[1:]+" is required", nil))
		}
		return nil
	}
	if err := pkg.Fields(
		required("/title", req.Title == ""),
		required("/description", req.Description == ""),
		required("/text", req.Text == ""),
		required("/preview_image", req.PreviewImage.File == nil),
	); err != nil {
		return nil, err
	}

	file := req.PreviewImage
	title := req.Title
	description := req.Description
	text := req.Text

	postID, err := s.Blogs.CreatePost(ctx, title, text, description, file.File, file.Name)
	if err != nil {
		return nil, err
	}

	return &corev1.CreatedPost{
		PostID: corev1.NewOptUUID(postID),
	}, nil
}

func (s *CoreServer) GetPostById(ctx context.Context, params corev1.GetPostByIdParams) (*corev1.PostModel, error) {
	post, err := s.BlogReads.GetPostByID(ctx, params.ID)
	if err != nil {
		return nil, err
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

func (s *CoreServer) PatchPostById(ctx context.Context, req corev1.OptPatchPostByIdReq, params corev1.PatchPostByIdParams) error {
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

	err := s.Blogs.UpdatePost(ctx, params.ID, titlePtr, textPtr, descriptionPtr, imageReader, filename)
	if err != nil {
		return err
	}
	return nil
}

func (s *CoreServer) DeletePostById(ctx context.Context, params corev1.DeletePostByIdParams) error {
	err := s.Blogs.DeletePost(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (s *CoreServer) GetPostImage(ctx context.Context, params corev1.GetPostImageParams) (corev1.GetPostImageRes, error) {
	post, err := s.BlogReads.GetPostByID(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	var ifNoneMatch *string
	if params.IfNoneMatch.IsSet() {
		ifNoneMatch = &params.IfNoneMatch.Value
	}

	postImage, imageErr := s.Blogs.GetPostImage(ctx, post.ImageKey, ifNoneMatch)

	var re *http.ResponseError
	if errors.Is(imageErr, storage.ErrNotModified) || (errors.As(imageErr, &re) && re.HTTPStatusCode() == 304) {
		return &corev1.GetPostImageNotModified{
			ETag: params.IfNoneMatch,
		}, nil
	} else if imageErr != nil {
		return nil, imageErr
	}

	return &corev1.GetPostImageOKHeaders{
		Response: corev1.GetPostImageOK{
			Data: postImage.ReadCloser(),
		},
		ETag: corev1.NewOptString(postImage.Etag()),
	}, nil
}
