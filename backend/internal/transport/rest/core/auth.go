package core

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// customLoginResponse sets cookie and writes AuthResponseModel JSON
type customLoginResponse struct {
	User      corev1.UserModel
	SessionID uuid.UUID
}

func (r customLoginResponse) VisitLoginResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    r.SessionID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   7 * 24 * 60 * 60, // 7 days
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(corev1.AuthResponseModel{
		User:      r.User,
		SessionId: &r.SessionID,
	})
}

// customVerifyEmailResponse sets cookie and writes AuthResponseModel JSON
type customVerifyEmailResponse struct {
	User      corev1.UserModel
	SessionID uuid.UUID
}

func (r customVerifyEmailResponse) VisitVerifyEmailResponse(w http.ResponseWriter) error {
	if r.SessionID != uuid.Nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    r.SessionID.String(),
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
			MaxAge:   7 * 24 * 60 * 60,
			SameSite: http.SameSiteLaxMode,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	var sessID *uuid.UUID
	if r.SessionID != uuid.Nil {
		sessID = &r.SessionID
	}
	return json.NewEncoder(w).Encode(corev1.AuthResponseModel{
		User:      r.User,
		SessionId: sessID,
	})
}

// customChangePasswordResponse sets cookie and writes AuthResponseModel JSON
type customChangePasswordResponse struct {
	User      corev1.UserModel
	SessionID uuid.UUID
}

func (r customChangePasswordResponse) VisitChangePasswordResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    r.SessionID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   7 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(corev1.AuthResponseModel{
		User:      r.User,
		SessionId: &r.SessionID,
	})
}

// customLogoutResponse clears cookie
type customLogoutResponse struct{}

func (r customLogoutResponse) VisitLogoutResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusOK)
	return nil
}

// Register implements corev1.StrictServerInterface
func (h *CoreServer) Register(ctx context.Context, request corev1.RegisterRequestObject) (corev1.RegisterResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user, err := h.authUC.Register(ctx, request.Body.Username, string(request.Body.Email), request.Body.Password)
	if err != nil {
		return nil, err
	}

	return corev1.Register200JSONResponse{
		User: userDTO(user),
	}, nil
}

// Login implements corev1.StrictServerInterface
func (h *CoreServer) Login(ctx context.Context, request corev1.LoginRequestObject) (corev1.LoginResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user, sessionID, err := h.authUC.Login(ctx, request.Body.Identifier, request.Body.Password)
	if err != nil {
		return nil, err
	}

	return customLoginResponse{
		User:      userDTO(user),
		SessionID: sessionID,
	}, nil
}

// Logout implements corev1.StrictServerInterface
func (h *CoreServer) Logout(ctx context.Context, request corev1.LogoutRequestObject) (corev1.LogoutResponseObject, error) {
	if session, err := middleware.GetSession(ctx); err == nil {
		_ = h.authUC.Logout(ctx, session.ID)
	}

	return customLogoutResponse{}, nil
}

// VerifyEmail implements corev1.StrictServerInterface
func (h *CoreServer) VerifyEmail(ctx context.Context, request corev1.VerifyEmailRequestObject) (corev1.VerifyEmailResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	user, sessionID, err := h.authUC.VerifyEmail(ctx, request.Body.Token)
	if err != nil {
		return nil, err
	}

	return customVerifyEmailResponse{
		User:      userDTO(user),
		SessionID: sessionID,
	}, nil
}

// ResendVerification implements corev1.StrictServerInterface
func (h *CoreServer) ResendVerification(ctx context.Context, request corev1.ResendVerificationRequestObject) (corev1.ResendVerificationResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ResendVerificationEmail(ctx, request.Body.Identifier)
	if err != nil {
		return nil, err
	}

	return corev1.ResendVerification200Response{}, nil
}

// ForgotPassword implements corev1.StrictServerInterface
func (h *CoreServer) ForgotPassword(ctx context.Context, request corev1.ForgotPasswordRequestObject) (corev1.ForgotPasswordResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ForgotPassword(ctx, request.Body.Identifier)
	if err != nil {
		return nil, err
	}

	return corev1.ForgotPassword200Response{}, nil
}

// ResetPassword implements corev1.StrictServerInterface
func (h *CoreServer) ResetPassword(ctx context.Context, request corev1.ResetPasswordRequestObject) (corev1.ResetPasswordResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ResetPassword(ctx, request.Body.Token, request.Body.Password)
	if err != nil {
		return nil, err
	}

	return corev1.ResetPassword200Response{}, nil
}

// ConfirmEmailChange implements corev1.StrictServerInterface
func (h *CoreServer) ConfirmEmailChange(ctx context.Context, request corev1.ConfirmEmailChangeRequestObject) (corev1.ConfirmEmailChangeResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	err := h.authUC.ConfirmEmailChange(ctx, request.Body.Token)
	if err != nil {
		return nil, err
	}

	return corev1.ConfirmEmailChange200Response{}, nil
}
