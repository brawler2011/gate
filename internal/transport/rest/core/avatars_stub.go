package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/pkg"
)

// UploadAvatar handles POST /users/{id}/avatar
// Note: This is implemented separately in AvatarsHandler (avatars.go)
// This stub is required for the StrictServerInterface
func (h *CoreServer) UploadAvatar(ctx context.Context, request corev1.UploadAvatarRequestObject) (corev1.UploadAvatarResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "UploadAvatar not implemented in CoreServer (use separate handler)")
}

// DeleteAvatar handles DELETE /users/{id}/avatar
// Note: This is implemented separately in AvatarsHandler (avatars.go)
// This stub is required for the StrictServerInterface
func (h *CoreServer) DeleteAvatar(ctx context.Context, request corev1.DeleteAvatarRequestObject) (corev1.DeleteAvatarResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "DeleteAvatar not implemented in CoreServer (use separate handler)")
}
