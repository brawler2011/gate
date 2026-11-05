package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/google/uuid"
)

func (h *CoreServer) Register(ctx context.Context, req *corev1.RegisterRequestModel) (*corev1.AuthResponseModel, error) {
	user, err := h.identity.Register(ctx, req.Username, req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	return &corev1.AuthResponseModel{
		User: userDTO(user),
	}, nil
}

func (h *CoreServer) Login(ctx context.Context, req *corev1.LoginRequestModel) (*corev1.AuthResponseModelHeaders, error) {
	user, sessionID, err := h.identity.Login(ctx, req.Identifier, req.Password)
	if err != nil {
		return nil, err
	}

	return &corev1.AuthResponseModelHeaders{
		SetCookie: h.sessionCookie(sessionID),
		Response: corev1.AuthResponseModel{
			User:      userDTO(user),
			SessionID: corev1.NewOptUUID(sessionID),
		},
	}, nil
}

func (h *CoreServer) Logout(ctx context.Context) (*corev1.LogoutOK, error) {
	if session, err := middleware.GetSession(ctx); err == nil {
		// FIXME: do not ignore errors
		_ = h.identity.Logout(ctx, session.ID)
	}

	return &corev1.LogoutOK{SetCookie: h.clearSessionCookie()}, nil
}

func (h *CoreServer) VerifyEmail(ctx context.Context, req *corev1.VerifyEmailRequestModel) (*corev1.VerifyEmailResponseHeaders, error) {
	user, sessionID, err := h.identity.VerifyEmail(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	var cookie corev1.OptString
	var sessOpt corev1.OptUUID
	if sessionID != uuid.Nil {
		cookie = corev1.NewOptString(h.sessionCookie(sessionID))
		sessOpt = corev1.NewOptUUID(sessionID)
	}

	return &corev1.VerifyEmailResponseHeaders{
		SetCookie: cookie,
		Response: corev1.AuthResponseModel{
			User:      userDTO(user),
			SessionID: sessOpt,
		},
	}, nil
}

func (h *CoreServer) ResendVerification(ctx context.Context, req *corev1.ResendVerificationRequestModel) error {
	err := h.identity.ResendVerificationEmail(ctx, req.Identifier)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ForgotPassword(ctx context.Context, req *corev1.ForgotPasswordRequestModel) error {
	err := h.identity.ForgotPassword(ctx, req.Identifier)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ResetPassword(ctx context.Context, req *corev1.ResetPasswordRequestModel) error {
	err := h.identity.ResetPassword(ctx, req.Token, req.Password)
	if err != nil {
		return err
	}

	return nil
}

// ConfirmEmailChange implements corev1.Handler
func (h *CoreServer) ConfirmEmailChange(ctx context.Context, req *corev1.ConfirmEmailChangeRequestModel) error {
	err := h.identity.ConfirmEmailChange(ctx, req.Token)
	if err != nil {
		return err
	}

	return nil
}
