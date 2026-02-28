package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/transport/middleware"
)

// ListPosts implements the ListPosts operation
func (s *CoreServer) ListPosts(ctx context.Context, request corev1.ListPostsRequestObject) (corev1.ListPostsResponseObject, error) {
	page := 1
	if request.Params.Page != nil {
		page = *request.Params.Page
	}

	pageSize := 10
	if request.Params.PageSize != nil {
		pageSize = *request.Params.PageSize
	}

	sortOrder := "desc"
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	result, err := s.blogsUC.ListPosts(ctx, page, pageSize, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to list posts: %w", err)
	}

	// Convert to response model
	posts := make([]corev1.PostModel, len(result.Posts))
	for i, post := range result.Posts {
		posts[i] = corev1.PostModel{
			Id:             &post.ID,
			CreatedAt:      &post.CreatedAt,
			UpdatedAt:      &post.UpdatedAt,
			Title:          &post.Title,
			Text:           &post.Text,
			Description:    &post.Description,
			PreviewImageId: &post.ImageKey,
			AuthorId:       &post.AuthorUUID,
			AuthorUsername: &post.AuthorName,
		}
	}

	totalPages := result.TotalPages
	currentPage := result.Page

	return corev1.ListPosts200JSONResponse{
		Pagination: &corev1.BlogPaginationModel{
			Total: &totalPages,
			Page:  &currentPage,
		},
		Posts: &posts,
	}, nil
}

// CreatePost implements the CreatePost operation
func (s *CoreServer) CreatePost(ctx context.Context, request corev1.CreatePostRequestObject) (corev1.CreatePostResponseObject, error) {
	// Get user from context
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return corev1.CreatePost401JSONResponse{
			Error: stringPtr("unauthorized"),
		}, nil
	}

	// Check if user is admin
	if user.Role != "admin" {
		return corev1.CreatePost403JSONResponse{
			Error: stringPtr("forbidden: only admins can create posts"),
		}, nil
	}

	// Parse multipart form
	title, description, text, imageReader, filename, err := parsePostForm(request.Body)
	if err != nil {
		return corev1.CreatePost400JSONResponse{
			Error: stringPtr(err.Error()),
		}, nil
	}

	// Validate required fields
	if title == "" || description == "" || text == "" || imageReader == nil {
		return corev1.CreatePost400JSONResponse{
			Error: stringPtr("title, description, text, and preview_image are required"),
		}, nil
	}

	// Create post
	postID, err := s.blogsUC.CreatePost(ctx, title, text, description, user.Id, user.Username, imageReader, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	return corev1.CreatePost201JSONResponse{
		PostId: &postID,
	}, nil
}

// GetPostById implements the GetPostById operation
func (s *CoreServer) GetPostById(ctx context.Context, request corev1.GetPostByIdRequestObject) (corev1.GetPostByIdResponseObject, error) {
	post, err := s.blogsUC.GetPost(ctx, request.Id)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return corev1.GetPostById404JSONResponse{
			Error: stringPtr("post not found"),
		}, nil
	}

	return corev1.GetPostById200JSONResponse{
		Id:             &post.ID,
		CreatedAt:      &post.CreatedAt,
		UpdatedAt:      &post.UpdatedAt,
		Title:          &post.Title,
		Text:           &post.Text,
		Description:    &post.Description,
		PreviewImageId: &post.ImageKey,
		AuthorId:       &post.AuthorUUID,
		AuthorUsername: &post.AuthorName,
	}, nil
}

// PatchPostById implements the PatchPostById operation
func (s *CoreServer) PatchPostById(ctx context.Context, request corev1.PatchPostByIdRequestObject) (corev1.PatchPostByIdResponseObject, error) {
	// Get user from context
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return corev1.PatchPostById401JSONResponse{
			Error: stringPtr("unauthorized"),
		}, nil
	}

	// Check if user is admin
	if user.Role != "admin" {
		return corev1.PatchPostById403JSONResponse{
			Error: stringPtr("forbidden: only admins can update posts"),
		}, nil
	}

	// Parse multipart form
	title, description, text, imageReader, filename, err := parsePostForm(request.Body)
	if err != nil {
		return corev1.PatchPostById400JSONResponse{
			Error: stringPtr(err.Error()),
		}, nil
	}

	// Convert to pointers for optional fields
	var titlePtr, descriptionPtr, textPtr *string
	if title != "" {
		titlePtr = &title
	}
	if description != "" {
		descriptionPtr = &description
	}
	if text != "" {
		textPtr = &text
	}

	// Update post
	err = s.blogsUC.UpdatePost(ctx, request.Id, titlePtr, textPtr, descriptionPtr, imageReader, filename)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return corev1.PatchPostById404JSONResponse{
			Error: stringPtr("post not found"),
		}, nil
	}

	return corev1.PatchPostById200Response{}, nil
}

// DeletePostById implements the DeletePostById operation
func (s *CoreServer) DeletePostById(ctx context.Context, request corev1.DeletePostByIdRequestObject) (corev1.DeletePostByIdResponseObject, error) {
	// Get user from context
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return corev1.DeletePostById401JSONResponse{
			Error: stringPtr("unauthorized"),
		}, nil
	}

	// Check if user is admin
	if user.Role != "admin" {
		return corev1.DeletePostById403JSONResponse{
			Error: stringPtr("forbidden: only admins can delete posts"),
		}, nil
	}

	// Delete post
	err := s.blogsUC.DeletePost(ctx, request.Id)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return corev1.DeletePostById404JSONResponse{
			Error: stringPtr("post not found"),
		}, nil
	}

	return corev1.DeletePostById200Response{}, nil
}

// GetPostImage implements the GetPostImage operation
func (s *CoreServer) GetPostImage(ctx context.Context, request corev1.GetPostImageRequestObject) (corev1.GetPostImageResponseObject, error) {
	// First get the post to retrieve the image key
	post, err := s.blogsUC.GetPost(ctx, request.Id)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		return corev1.GetPostImage404JSONResponse{
			Error: stringPtr("post not found"),
		}, nil
	}

	// Get image from S3
	postImage, err := s.blogsUC.GetPostImage(ctx, post.ImageKey, request.Params.IfNoneMatch)

	var re *http.ResponseError
	if errors.As(err, &re) && re.HTTPStatusCode() == 304 {
		return corev1.GetPostImage304Response{
			Headers: corev1.GetPostImage304ResponseHeaders{
				ETag: *request.Params.IfNoneMatch,
			},
		}, nil
	} else if err != nil {
		return corev1.GetPostImage404JSONResponse{
			Error: stringPtr("image not found"),
		}, nil
	}

	return corev1.GetPostImage200ImagepngResponse{
		Body: postImage.ReadCloser(),
		Headers: corev1.GetPostImage200ResponseHeaders{
			ETag: postImage.Etag(),
		},
	}, nil
}

// parsePostForm parses a multipart form and extracts post fields
func parsePostForm(reader *multipart.Reader) (title, description, text string, imageReader io.Reader, filename string, err error) {
	if reader == nil {
		return "", "", "", nil, "", fmt.Errorf("no multipart data")
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", "", nil, "", fmt.Errorf("failed to read multipart part: %w", err)
		}

		fieldName := part.FormName()
		switch fieldName {
		case "title":
			buf := make([]byte, 1024*10) // 10KB max for title
			n, _ := part.Read(buf)
			title = string(buf[:n])
		case "description":
			buf := make([]byte, 1024*100) // 100KB max for description
			n, _ := part.Read(buf)
			description = string(buf[:n])
		case "text":
			buf := make([]byte, 1024*1024) // 1MB max for text
			n, _ := part.Read(buf)
			text = string(buf[:n])
		case "preview_image":
			filename = part.FileName()
			imageReader = part
			// Don't close part here, it will be read later
			return title, description, text, imageReader, filename, nil
		}
		part.Close()
	}

	return title, description, text, imageReader, filename, nil
}

// stringPtr is a helper to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}
