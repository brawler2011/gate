package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Register implements corev1.Handler
func (h *CoreServer) Register(ctx context.Context, req *corev1.RegisterRequestModel) (*corev1.AuthResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user, err := h.authUC.Register(ctx, req.Username, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	return &corev1.AuthResponseModel{
		User: userDTO(user),
	}, nil
}

// Login implements corev1.Handler
func (h *CoreServer) Login(ctx context.Context, req *corev1.LoginRequestModel) (*corev1.AuthResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user, sessionID, err := h.authUC.Login(ctx, req.Identifier, req.Password)
	if err != nil {
		return nil, err
	}

	middleware.SetSessionCookie(ctx, sessionID)

	return &corev1.AuthResponseModel{
		User:      userDTO(user),
		SessionID: corev1.NewOptUUID(sessionID),
	}, nil
}

// Logout implements corev1.Handler
func (h *CoreServer) Logout(ctx context.Context) error {
	if session, err := middleware.GetSession(ctx); err == nil {
		_ = h.authUC.Logout(ctx, session.ID)
	}

	middleware.ClearSessionCookie(ctx)
	return nil
}

// VerifyEmail implements corev1.Handler
func (h *CoreServer) VerifyEmail(ctx context.Context, req *corev1.VerifyEmailRequestModel) (*corev1.AuthResponseModel, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user, sessionID, err := h.authUC.VerifyEmail(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	if sessionID != uuid.Nil {
		middleware.SetSessionCookie(ctx, sessionID)
	}

	var sessOpt corev1.OptUUID
	if sessionID != uuid.Nil {
		sessOpt = corev1.NewOptUUID(sessionID)
	}

	return &corev1.AuthResponseModel{
		User:      userDTO(user),
		SessionID: sessOpt,
	}, nil
}

// ResendVerification implements corev1.Handler
func (h *CoreServer) ResendVerification(ctx context.Context, req *corev1.ResendVerificationRequestModel) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ResendVerificationEmail(ctx, req.Identifier)
	if err != nil {
		return err
	}

	return nil
}

// ForgotPassword implements corev1.Handler
func (h *CoreServer) ForgotPassword(ctx context.Context, req *corev1.ForgotPasswordRequestModel) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ForgotPassword(ctx, req.Identifier)
	if err != nil {
		return err
	}

	return nil
}

// ResetPassword implements corev1.Handler
func (h *CoreServer) ResetPassword(ctx context.Context, req *corev1.ResetPasswordRequestModel) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ResetPassword(ctx, req.Token, req.Password)
	if err != nil {
		return err
	}

	return nil
}

// ConfirmEmailChange implements corev1.Handler
func (h *CoreServer) ConfirmEmailChange(ctx context.Context, req *corev1.ConfirmEmailChangeRequestModel) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ConfirmEmailChange(ctx, req.Token)
	if err != nil {
		return err
	}

	return nil
}
