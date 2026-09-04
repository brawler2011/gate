package core

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
)

func (s *CoreServer) ListPosts(ctx context.Context, params corev1.ListPostsParams) (*corev1.ListPostsResponseModel, error) {
	// FIXME: нужно использовать готовые модели пагинации в openapi.yaml
	// FIXME: удалить этот бойлерплейт
	page := int(params.Page.Or(1))
	pageSize := int(params.PageSize.Or(10))
	sortOrder := string(params.SortOrder.Or("desc"))

	result, err := s.blogsUC.ListPosts(ctx, page, pageSize, sortOrder)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list posts")
	}

	// Convert to response model
	posts := make([]corev1.PostModel, len(result.Posts))
	for i, post := range result.Posts {
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
			Total: safeInt32(result.TotalPages), // FIXME: разобраться где возникает проблема типов
			Page:  safeInt32(result.Page),       // FIXME: аналогично
		},
		Posts: posts,
	}, nil
}

// CreatePost implements the CreatePost operation
func (s *CoreServer) CreatePost(ctx context.Context, req *corev1.CreatePostReq) (corev1.CreatePostRes, error) {
	// FIXME: эти провери должны быть в мидлваре
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	// FIXME: req не может быть nil, остальное надо сделать required в openapi
	if req == nil || !req.Title.IsSet() || !req.Description.IsSet() || !req.Text.IsSet() || !req.PreviewImage.IsSet() {
		return &corev1.CreatePostBadRequest{
			Error: corev1.NewOptString("title, description, text, and preview_image are required"),
		}, nil
	}

	file := req.PreviewImage.Value // FIXME: эта хуйня не должна быть optional
	title := req.Title.Value
	description := req.Description.Value
	text := req.Text.Value

	postID, err := s.blogsUC.CreatePost(ctx, title, text, description, user.Id, user.Username, file.File, file.Name)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to create post")
	}

	// FIXME: вроде как в схеме есть reusable структура для таких ответов
	return &corev1.CreatedPost{
		PostID: corev1.NewOptUUID(postID), // FIXME: эта хуйня тоже не должна быть optional
	}, nil
}

func (s *CoreServer) GetPostById(ctx context.Context, params corev1.GetPostByIdParams) (corev1.GetPostByIdRes, error) {
	post, err := s.blogsUC.GetPost(ctx, params.ID)
	if err != nil {

		// FIXME: какой смысл здесь это хендлить, если это должно хендлиться в middleware?
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}

		// FIXME: в проекте не приянто создавать касатомные ошибки внутри схемы апи
		return &corev1.GetPostByIdNotFound{
			Error: corev1.NewOptString("post not found"),
		}, nil
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
	// FIXME: антипаттерн, эти поля нужно объединить в структуру или тп
	var titlePtr, descriptionPtr, textPtr *string
	var imageReader io.Reader
	var filename string

	// FIXME: req должен быть required, поля мапяться в структуру
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

	err := s.blogsUC.UpdatePost(ctx, params.ID, titlePtr, textPtr, descriptionPtr, imageReader, filename)
	if err != nil {
		// FIXME: какой смысл здесь это хендлить, если это должно хендлиться в middleware?
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		// FIXME: в проекте так не принято
		return &corev1.PatchPostByIdNotFound{
			Error: corev1.NewOptString("post not found"),
		}, nil
	}
	// FIXME: в проекте так не принято
	return &corev1.PatchPostByIdOK{}, nil
}

func (s *CoreServer) DeletePostById(ctx context.Context, params corev1.DeletePostByIdParams) (corev1.DeletePostByIdRes, error) {
	err := s.blogsUC.DeletePost(ctx, params.ID)
	if err != nil {
		// FIXME: какой смысл здесь это хендлить, если это должно хендлиться в middleware?
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		// FIXME: в проекте так не принято
		return &corev1.DeletePostByIdNotFound{
			Error: corev1.NewOptString("post not found"),
		}, nil
	}

	// FIXME: в проекте так не принято
	return &corev1.DeletePostByIdOK{}, nil
}

func (s *CoreServer) GetPostImage(ctx context.Context, params corev1.GetPostImageParams) (corev1.GetPostImageRes, error) {
	post, err := s.blogsUC.GetPost(ctx, params.ID)
	if err != nil {
		// FIXME:
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		// FIXME:
		return &corev1.GetPostImageNotFound{
			Error: corev1.NewOptString("post not found"),
		}, nil
	}

	var ifNoneMatch *string
	if params.IfNoneMatch.IsSet() {
		ifNoneMatch = &params.IfNoneMatch.Value
	}

	postImage, imageErr := s.blogsUC.GetPostImage(ctx, post.ImageKey, ifNoneMatch)

	var re *http.ResponseError
	if errors.Is(imageErr, storage.ErrNotModified) || (errors.As(imageErr, &re) && re.HTTPStatusCode() == 304) {
		return &corev1.GetPostImageNotModified{
			ETag: params.IfNoneMatch,
		}, nil
	} else if imageErr != nil {
		// FIXME:
		if errors.Is(imageErr, context.DeadlineExceeded) || errors.Is(imageErr, context.Canceled) {
			return nil, imageErr
		}
		// FIXME:
		return &corev1.GetPostImageNotFound{
			Error: corev1.NewOptString("image not found"),
		}, nil
	}

	// FIXME:
	return &corev1.GetPostImageOKHeaders{
		Response: corev1.GetPostImageOK{
			Data: postImage.ReadCloser(),
		},
		ETag: corev1.NewOptString(postImage.Etag()),
	}, nil
}
