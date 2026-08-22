package pg

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
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

func (r *AuthRepo) DeleteSessionsByUserID(ctx context.Context, userID uuid.UUID) error {
	err := r.queries.DeleteSessionsByUserId(ctx, userID)
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

func (r *AuthRepo) CreateAuthToken(ctx context.Context, params models.CreateAuthTokenParams) error {
	payload := params.Payload
	if payload == nil {
		payload = []byte("{}")
	}
	err := r.queries.CreateAuthToken(ctx, sqlc.CreateAuthTokenParams{
		ID:        params.ID,
		UserID:    params.UserID,
		TokenType: string(params.TokenType),
		TokenHash: params.TokenHash,
		Payload:   payload,
		ExpiresAt: params.ExpiresAt,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *AuthRepo) GetAuthTokenByHash(ctx context.Context, tokenHash string, tokenType models.AuthTokenType) (models.AuthToken, error) {
	tok, err := r.queries.GetAuthTokenByHash(ctx, sqlc.GetAuthTokenByHashParams{
		TokenHash: tokenHash,
		TokenType: string(tokenType),
	})
	if err != nil {
		return models.AuthToken{}, HandlePgErr(err)
	}

	return models.AuthToken{
		ID:        tok.ID,
		UserID:    tok.UserID,
		TokenType: tokenType,
		TokenHash: tok.TokenHash,
		Payload:   tok.Payload,
		ExpiresAt: tok.ExpiresAt,
		CreatedAt: tok.CreatedAt,
	}, nil
}

func (r *AuthRepo) DeleteAuthToken(ctx context.Context, id uuid.UUID) error {
	err := r.queries.DeleteAuthToken(ctx, id)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *AuthRepo) DeleteAuthTokensByUserIdAndType(ctx context.Context, userID uuid.UUID, tokenType models.AuthTokenType) error {
	err := r.queries.DeleteAuthTokensByUserIdAndType(ctx, sqlc.DeleteAuthTokensByUserIdAndTypeParams{
		UserID:    userID,
		TokenType: string(tokenType),
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *AuthRepo) CleanupExpiredAuthTokens(ctx context.Context) error {
	err := r.queries.CleanupExpiredAuthTokens(ctx)
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

