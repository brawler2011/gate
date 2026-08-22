package pg

import (
	"context"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UsersRepo struct {
	queries *sqlc.Queries
}

func NewUsersRepo(p *pgxpool.Pool) *UsersRepo {
	return &UsersRepo{
		queries: sqlc.New(p),
	}
}

func (r *UsersRepo) WithTx(tx pgx.Tx) interfaces.UsersRepo {
	return &UsersRepo{
		queries: sqlc.New(tx),
	}
}

func (r *UsersRepo) CreateUser(ctx context.Context, params models.CreateUserParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	var expiresAt pgtype.Timestamptz
	if params.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{
			Time:  *params.ExpiresAt,
			Valid: true,
		}
	}

	err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:              params.Id,
		Username:        params.Username,
		Role:            sqlc.UserRole(params.Role),
		PasswordHash:    params.PasswordHash,
		Email:           params.Email,
		AvatarUrl:       params.AvatarUrl,
		ExpiresAt:       expiresAt,
		IsEmailVerified: params.IsEmailVerified,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *UsersRepo) GetUserById(ctx context.Context, id uuid.UUID) (models.User, error) {
	user, err := r.queries.GetUserById(ctx, id)
	if err != nil {
		return models.User{}, HandlePgErr(err)
	}
	return mapUserToModel(user), nil
}

func mapUserToModel(user sqlc.User) models.User {
	var expiresAt *time.Time
	if user.ExpiresAt.Valid {
		expiresAt = &user.ExpiresAt.Time
	}
	var claimedByUserID *uuid.UUID
	if user.ClaimedByUserID.Valid {
		uid := uuid.UUID(user.ClaimedByUserID.Bytes)
		claimedByUserID = &uid
	}
	var claimedAt *time.Time
	if user.ClaimedAt.Valid {
		claimedAt = &user.ClaimedAt.Time
	}

	return models.User{
		Id:              user.ID,
		Username:        user.Username,
		Role:            models.UserRole(user.Role),
		PasswordHash:    user.PasswordHash,
		Email:           user.Email,
		AvatarUrl:       user.AvatarUrl,
		ExpiresAt:       expiresAt,
		IsEmailVerified: user.IsEmailVerified,
		ClaimedByUserID: claimedByUserID,
		ClaimedAt:       claimedAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

func (r *UsersRepo) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	user, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return models.User{}, HandlePgErr(err)
	}
	return mapUserToModel(user), nil
}

func (r *UsersRepo) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (models.User, error) {
	user, err := r.queries.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		return models.User{}, HandlePgErr(err)
	}
	return mapUserToModel(user), nil
}

func (r *UsersRepo) ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error) {
	if err := filter.Validate(); err != nil {
		return models.UsersList{}, err
	}

	total, err := r.queries.CountUsers(ctx, sqlc.CountUsersParams{
		Search: filter.Search,
		Role:   filter.Role,
	})
	if err != nil {
		return models.UsersList{}, HandlePgErr(err)
	}

	users, err := r.queries.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  filter.PageSize,
		Offset: Offset(filter.Page, filter.PageSize),
		Search: filter.Search,
		Role:   filter.Role,
	})
	if err != nil {
		return models.UsersList{}, HandlePgErr(err)
	}

	userRecords := make([]models.User, len(users))
	for i, u := range users {
		userRecords[i] = mapUserToModel(u)
	}

	return models.UsersList{
		Users:      userRecords,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (r *UsersRepo) UpdateUser(
	ctx context.Context,
	params models.UpdateUserParams,
) error {
	if err := params.Validate(); err != nil {
		return err
	}

	var role sqlc.NullUserRole
	if params.Role != nil {
		role = sqlc.NullUserRole{
			UserRole: sqlc.UserRole(*params.Role),
			Valid:    true,
		}
	}

	var expiresAt pgtype.Timestamptz
	if params.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{
			Time:  *params.ExpiresAt,
			Valid: true,
		}
	}

	err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:              params.Id,
		Username:        params.Username,
		Role:            role,
		Email:           params.Email,
		AvatarUrl:       params.AvatarUrl,
		ExpiresAt:       expiresAt,
		IsEmailVerified: params.IsEmailVerified,
	})
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func (r *UsersRepo) SetUserEmailVerified(ctx context.Context, id uuid.UUID, isVerified bool) error {
	err := r.queries.SetUserEmailVerified(ctx, sqlc.SetUserEmailVerifiedParams{
		ID:              id,
		IsEmailVerified: isVerified,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *UsersRepo) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	err := r.queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		ID:           id,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *UsersRepo) UpdateUserEmail(ctx context.Context, id uuid.UUID, email string, isVerified bool) error {
	err := r.queries.UpdateUserEmail(ctx, sqlc.UpdateUserEmailParams{
		ID:              id,
		Email:           &email,
		IsEmailVerified: isVerified,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}


func (r *UsersRepo) ClaimTemporaryUser(ctx context.Context, id, claimedByUserID uuid.UUID, claimedAt time.Time) error {
	err := r.queries.ClaimTemporaryUser(ctx, sqlc.ClaimTemporaryUserParams{
		ID:              id,
		ClaimedByUserID: claimedByUserID,
		ClaimedAt:       claimedAt,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *UsersRepo) ListClaimedAccountsByUserId(ctx context.Context, claimedByUserID uuid.UUID) ([]models.User, error) {
	users, err := r.queries.ListClaimedAccountsByUserId(ctx, claimedByUserID)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	res := make([]models.User, len(users))
	for i, u := range users {
		res[i] = mapUserToModel(u)
	}
	return res, nil
}

func (r *UsersRepo) ListExistingUsernamesByPrefix(ctx context.Context, prefix string) ([]string, error) {
	usernames, err := r.queries.ListExistingUsernamesByPrefix(ctx, prefix)
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return usernames, nil
}

