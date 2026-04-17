package core

import (
	"bytes"
	"context"
	"io"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/pkg"
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
	_, err := h.avatarsUC.UploadAvatar(ctx, request.Id, fileReader, filename, contentType)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to upload avatar")
	}

	// Return response (imgId is optional, we can leave it nil)
	return corev1.UploadAvatar200JSONResponse{}, nil
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
