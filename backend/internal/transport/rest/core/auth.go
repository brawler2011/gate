package core

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

// customRegisterResponse sets cookie and writes AuthResponseModel JSON
type customRegisterResponse struct {
	User      corev1.UserModel
	SessionID uuid.UUID
}

func (r customRegisterResponse) VisitRegisterResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    r.SessionID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // set true in production if needed; for local dev, false is fine
		MaxAge:   7 * 24 * 60 * 60, // 7 days (sliding)
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(corev1.AuthResponseModel{
		User:      r.User,
		SessionId: r.SessionID,
	})
}

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
		SessionId: r.SessionID,
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

	user, sessionID, err := h.authUC.Register(ctx, request.Body.Username, string(request.Body.Email), request.Body.Password)
	if err != nil {
		return nil, err
	}

	return customRegisterResponse{
		User:      userDTO(user),
		SessionID: sessionID,
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
