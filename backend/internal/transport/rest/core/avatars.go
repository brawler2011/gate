package core

import (
	"context"
	"errors"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
)

func wrapAvatarError(err error, fallbackMsg string) error {
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
	return pkg.Wrap(nil, err, fallbackMsg)
}

func (h *CoreServer) UploadAvatar(ctx context.Context, req *corev1.UploadAvatarReq, params corev1.UploadAvatarParams) (*corev1.UploadAvatarOK, error) {
	file := req.Avatar
	filename := file.Name
	contentType := file.Header.Get("Content-Type")
	// FIXME: эта проверка уже есть в Header.Get,
	// NOTE: вообще стоит изучить как правильно загружать медиа файлы
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	imgID, err := h.identity.UploadAvatar(ctx, params.Username, file.File, filename, contentType)
	if err != nil {
		return nil, wrapAvatarError(err, "failed to upload avatar")
	}

	return &corev1.UploadAvatarOK{
		ImgID: imgID,
	}, nil
}

func (h *CoreServer) DeleteAvatar(ctx context.Context, params corev1.DeleteAvatarParams) error {
	err := h.identity.DeleteAvatar(ctx, params.Username)
	if err != nil {
		return wrapAvatarError(err, "failed to delete avatar")
	}

	return nil
}

// FIXME: блядь здесь происходит что-то не то
func (h *CoreServer) GetUserAvatar(ctx context.Context, params corev1.GetUserAvatarParams) (corev1.GetUserAvatarRes, error) {
	var ifNoneMatch *string
	if params.IfNoneMatch.IsSet() {
		ifNoneMatch = &params.IfNoneMatch.Value
	}

	avatarImg, err := h.identity.GetAvatar(ctx, params.Username, ifNoneMatch)
	if err != nil {
		if errors.Is(err, storage.ErrNotModified) {
			var etag corev1.OptString
			if params.IfNoneMatch.IsSet() {
				etag = params.IfNoneMatch
			}
			return &corev1.GetUserAvatarNotModified{
				ETag: etag,
			}, nil
		}
		if errors.Is(err, storage.ErrNotFound) {
			return &corev1.GetUserAvatarNotFound{
				Error: corev1.NewOptString("avatar not found"),
			}, nil
		}
		return nil, wrapAvatarError(err, "failed to get avatar")
	}

	return &corev1.GetUserAvatarOKHeaders{
		Response: corev1.GetUserAvatarOK{
			Data: avatarImg.ReadCloser(),
		},
		ETag: corev1.NewOptString(avatarImg.Etag()),
	}, nil
}
