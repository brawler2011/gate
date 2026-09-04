package core

import (
	"context"
	"errors"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

func (h *CoreServer) UploadAvatar(ctx context.Context, req *corev1.UploadAvatarReq, params corev1.UploadAvatarParams) (*corev1.UploadAvatarOK, error) {
	file := req.Avatar
	filename := file.Name
	contentType := file.Header.Get("Content-Type")
	// FIXME: эта проверка уже есть в Header.Get,
	// NOTE: вообще стоит изучить как правильно загружать медиа файлы
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	imgIDStr, err := h.avatarsUC.UploadAvatar(ctx, params.Username, file.File, filename, contentType)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to upload avatar")
	}

	// FIXME: бля так а нахуя сначала uuid->string, а потом обратно??
	parsedUUID, err := uuid.Parse(imgIDStr)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to parse uploaded avatar uuid")
	}

	return &corev1.UploadAvatarOK{
		ImgId: parsedUUID,
	}, nil
}

func (h *CoreServer) DeleteAvatar(ctx context.Context, params corev1.DeleteAvatarParams) error {
	err := h.avatarsUC.DeleteAvatar(ctx, params.Username)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to delete avatar")
	}

	return nil
}

// FIXME: блядь здесь происходит что-то не то
func (h *CoreServer) GetUserAvatar(ctx context.Context, params corev1.GetUserAvatarParams) (corev1.GetUserAvatarRes, error) {
	var ifNoneMatch *string
	if params.IfNoneMatch.IsSet() {
		ifNoneMatch = &params.IfNoneMatch.Value
	}

	avatarImg, err := h.avatarsUC.GetAvatar(ctx, params.Username, ifNoneMatch)
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
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get avatar")
	}

	return &corev1.GetUserAvatarOKHeaders{
		Response: corev1.GetUserAvatarOK{
			Data: avatarImg.ReadCloser(),
		},
		ETag: corev1.NewOptString(avatarImg.Etag()),
	}, nil
}
