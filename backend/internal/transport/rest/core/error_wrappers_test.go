package core

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestErrorWrappersPreserveClassification(t *testing.T) {
	wrappers := map[string]func(error, string) error{
		"organization": wrapOrgUCError,
		"team":         wrapTeamUCError,
		"contest":      wrapContestUCError,
		"blog":         wrapBlogError,
	}
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"conflict", pkg.ErrConflict, http.StatusConflict},
		{"duplicate entity", postgres.HandlePgErr(&pgconn.PgError{Code: pgerrcode.UniqueViolation}), http.StatusConflict},
		{"duplicate membership", postgres.HandlePgErr(&pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "membership_pkey"}), http.StatusConflict},
		{"unauthenticated", pkg.ErrUnauthenticated, http.StatusUnauthorized},
		{"bad input", pkg.ErrBadInput, http.StatusBadRequest},
		{"not found", pkg.ErrNotFound, http.StatusNotFound},
		{"permission", pkg.NoPermission, http.StatusForbidden},
		{"payload too large", pkg.ErrPayloadTooLarge, http.StatusRequestEntityTooLarge},
		{"internal", pkg.ErrInternal, http.StatusInternalServerError},
		{"unknown", errors.New("connection failed"), http.StatusInternalServerError},
	}
	for name, wrap := range wrappers {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, wrap(nil, "context"))
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := wrap(fmt.Errorf("use case: %w", tt.err), "operation failed")
					require.ErrorIs(t, err, tt.err)
					require.Contains(t, err.Error(), "operation failed")
					require.Equal(t, tt.status, pkg.ToREST(err))
					if tt.status != http.StatusInternalServerError {
						require.NotErrorIs(t, err, pkg.ErrInternal)
					}
				})
			}
			require.Equal(t, http.StatusForbidden, pkg.ToREST(wrap(errors.New("access denied"), "operation failed")))
		})
	}
	require.Equal(t, http.StatusNotFound, pkg.ToREST(wrapBlogError(storage.ErrNotFound, "image not found")))
}
