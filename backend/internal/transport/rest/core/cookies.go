package core

import (
	"net/http"

	"github.com/google/uuid"
)

func (h *CoreServer) sessionCookie(sessionID uuid.UUID) string {
	return (&http.Cookie{
		Name:     "session_id",
		Value:    sessionID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		MaxAge:   7 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	}).String()
}

func (h *CoreServer) clearSessionCookie() string {
	return (&http.Cookie{
		Name:     "session_id",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	}).String()
}
