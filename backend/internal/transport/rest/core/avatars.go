package core

import (
	"bytes"
	"context"
	"errors"
	"io"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/pkg"
	"github.com/gate149/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

// UploadAvatar handles POST /users/{id}/avatar
func (h *CoreServer) UploadAvatar(ctx context.Context, request corev1.UploadAvatarRequestObject) (corev1.UploadAvatarResponseObject, error) {
	// Parse multipart form to extract file
	// The Body field contains a multipart.Reader
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "no multipart body provided")
	}

	var fileData []byte
	var filename string
	var contentType string

	// Read parts from multipart reader
	for {
		part, err := request.Body.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read multipart part")
		}

		// Look for the avatar file field
		if part.FormName() == "avatar" || part.FormName() == "file" {
			filename = part.FileName()
			contentType = part.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			// Read file data
			fileData, err = io.ReadAll(part)
			if err != nil {
				return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read file data")
			}
			break
		}
	}

	if len(fileData) == 0 {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "avatar file is required")
	}

	// Upload avatar using use case
	fileReader := bytes.NewReader(fileData)
	imgIDStr, err := h.avatarsUC.UploadAvatar(ctx, request.Id, fileReader, filename, contentType)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to upload avatar")
	}

	// Parse the string image ID into a UUID
	parsedUUID, err := uuid.Parse(imgIDStr)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to parse uploaded avatar uuid")
	}

	// Return response with imgId
	return corev1.UploadAvatar200JSONResponse{
		ImgId: &parsedUUID,
	}, nil
}

// DeleteAvatar handles DELETE /users/{id}/avatar
func (h *CoreServer) DeleteAvatar(ctx context.Context, request corev1.DeleteAvatarRequestObject) (corev1.DeleteAvatarResponseObject, error) {
	// Delete avatar using use case
	err := h.avatarsUC.DeleteAvatar(ctx, request.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to delete avatar")
	}

	return corev1.DeleteAvatar200Response{}, nil
}

// GetUserAvatar handles GET /users/{id}/avatar
func (h *CoreServer) GetUserAvatar(ctx context.Context, request corev1.GetUserAvatarRequestObject) (corev1.GetUserAvatarResponseObject, error) {
	// Retrieve avatar using use case
	avatarImg, err := h.avatarsUC.GetAvatar(ctx, request.Id, request.Params.IfNoneMatch)
	if err != nil {
		if errors.Is(err, storage.ErrNotModified) {
			var etag string
			if request.Params.IfNoneMatch != nil {
				etag = *request.Params.IfNoneMatch
			}
			return corev1.GetUserAvatar304Response{
				Headers: corev1.GetUserAvatar304ResponseHeaders{
					ETag: etag,
				},
			}, nil
		}
		if errors.Is(err, storage.ErrNotFound) {
			errMsg := "avatar not found"
			return corev1.GetUserAvatar404JSONResponse{
				Error: &errMsg,
			}, nil
		}
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get avatar")
	}

	return corev1.GetUserAvatar200ImagepngResponse{
		Body:          avatarImg.ReadCloser(),
		Headers:       corev1.GetUserAvatar200ResponseHeaders{ETag: avatarImg.Etag()},
		ContentLength: 0,
	}, nil
}
