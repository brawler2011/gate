package pg

import (
	"context"
	"strings"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
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

func (r *UsersRepo) ListUserResponses(ctx context.Context, filter models.UsersListFilter) (*corev1.ListUsersResponseModel, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	total, err := r.queries.CountUsers(ctx, sqlc.CountUsersParams{
		Search: filter.Search,
		Role:   filter.Role,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	users, err := r.queries.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  filter.PageSize,
		Offset: Offset(filter.Page, filter.PageSize),
		Search: filter.Search,
		Role:   filter.Role,
	})
	if err != nil {
		return nil, HandlePgErr(err)
	}

	userRecords := make([]corev1.UserModel, len(users))
	for i, u := range users {
		userRecords[i] = userResponse(u)
	}

	return &corev1.ListUsersResponseModel{
		Users:      userRecords,
		Pagination: corev1.PaginationModel{Page: filter.Page, Total: models.Total(total, filter.PageSize)},
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

func (r *UsersRepo) ListClaimedAccountResponses(ctx context.Context, claimedByUserID uuid.UUID) (*corev1.ListClaimedAccountsResponseModel, error) {
	users, err := r.queries.ListClaimedAccountsByUserId(ctx, claimedByUserID)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	res := make([]corev1.ClaimedAccountItem, len(users))
	for i, u := range users {
		res[i] = corev1.ClaimedAccountItem{ID: u.ID, Username: u.Username, ExpiresAt: corev1.OptDateTime{Value: u.ExpiresAt.Time, Set: u.ExpiresAt.Valid}}
		if u.ClaimedAt.Valid {
			res[i].ClaimedAt = u.ClaimedAt.Time
		}
	}
	return &corev1.ListClaimedAccountsResponseModel{Accounts: res}, nil
}

func (r *UsersRepo) ListExistingUsernamesByPrefix(ctx context.Context, prefix string) ([]string, error) {
	usernames, err := r.queries.ListExistingUsernamesByPrefix(ctx, prefix)
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return usernames, nil
}

func userResponse(u sqlc.User) corev1.UserModel {
	user := corev1.UserModel{
		ID:              u.ID,
		Username:        u.Username,
		Role:            string(u.Role),
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
		ExpiresAt:       corev1.OptDateTime{Value: u.ExpiresAt.Time, Set: u.ExpiresAt.Valid},
		ClaimedAt:       corev1.OptDateTime{Value: u.ClaimedAt.Time, Set: u.ClaimedAt.Valid},
		ClaimedByUserID: corev1.OptUUID{Value: uuid.UUID(u.ClaimedByUserID.Bytes), Set: u.ClaimedByUserID.Valid && uuid.UUID(u.ClaimedByUserID.Bytes) != uuid.Nil},
		IsEmailVerified: corev1.NewOptBool(u.IsEmailVerified),
	}
	if u.Email != nil {
		user.Email = corev1.NewOptString(*u.Email)
	}
	if u.AvatarUrl != nil && *u.AvatarUrl != "" {
		if id, err := uuid.Parse(*u.AvatarUrl); err == nil {
			user.ImgId = corev1.NewOptUUID(id)
		}
	}
	return user
}

func (r *UsersRepo) GetUserResponse(ctx context.Context, username string) (*corev1.GetUserResponseModel, error) {
	user, err := r.queries.GetUserByUsername(ctx, strings.TrimPrefix(username, "@"))
	if err != nil {
		return nil, HandlePgErr(err)
	}
	return &corev1.GetUserResponseModel{User: userResponse(user)}, nil
}
