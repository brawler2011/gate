package pg

import (
	"context"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepo struct {
	queries *sqlc.Queries
}

func NewAuthRepo(p *pgxpool.Pool) interfaces.AuthRepo {
	return &AuthRepo{
		queries: sqlc.New(p),
	}
}

func (r *AuthRepo) CreateSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, expiresAt time.Time) error {
	err := r.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *AuthRepo) GetSession(ctx context.Context, sessionID uuid.UUID) (models.Session, error) {
	session, err := r.queries.GetSession(ctx, sessionID)
	if err != nil {
		return models.Session{}, HandlePgErr(err)
	}
	return models.Session{
		ID:        session.ID,
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt,
		CreatedAt: session.CreatedAt,
	}, nil
}

func (r *AuthRepo) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	err := r.queries.DeleteSession(ctx, sessionID)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *AuthRepo) UpdateSessionExpiry(ctx context.Context, sessionID uuid.UUID, expiresAt time.Time) error {
	err := r.queries.UpdateSessionExpiry(ctx, sqlc.UpdateSessionExpiryParams{
		ID:        sessionID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *AuthRepo) CleanupExpiredSessions(ctx context.Context, hardLimitCutoff time.Time) error {
	err := r.queries.CleanupExpiredSessions(ctx, hardLimitCutoff)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

