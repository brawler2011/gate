package postgres

import (
	"context"
	"strings"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	identitymodel "github.com/brawler2011/gate/backend/internal/identity/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
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

func (r *UsersRepo) CreateUser(ctx context.Context, params identitymodel.CreateUserParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:              params.Id,
		Username:        params.Username,
		Role:            params.Role,
		PasswordHash:    params.PasswordHash,
		Email:           params.Email,
		AvatarUrl:       params.AvatarUrl,
		ExpiresAt:       params.ExpiresAt,
		IsEmailVerified: params.IsEmailVerified,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	return nil
}

func (r *UsersRepo) GetUserById(ctx context.Context, id uuid.UUID) (accessmodel.User, error) {
	user, err := r.queries.GetUserById(ctx, id)
	if err != nil {
		return accessmodel.User{}, HandlePgErr(err)
	}
	return mapUserToModel(user), nil
}

func mapUserToModel(user sqlc.User) accessmodel.User {
	return accessmodel.User{
		Id:              user.ID,
		Username:        user.Username,
		Role:            accessmodel.UserRole(user.Role),
		PasswordHash:    user.PasswordHash,
		Email:           user.Email,
		AvatarUrl:       user.AvatarUrl,
		ExpiresAt:       user.ExpiresAt,
		IsEmailVerified: user.IsEmailVerified,
		ClaimedByUserID: user.ClaimedByUserID,
		ClaimedAt:       user.ClaimedAt,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
	}
}

func (r *UsersRepo) GetUserByUsername(ctx context.Context, username string) (accessmodel.User, error) {
	user, err := r.queries.GetUserByUsername(ctx, strings.TrimPrefix(username, "@"))
	if err != nil {
		return accessmodel.User{}, HandlePgErr(err)
	}
	return mapUserToModel(user), nil
}

func (r *UsersRepo) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (accessmodel.User, error) {
	user, err := r.queries.GetUserByUsernameOrEmail(ctx, identifier)
	if err != nil {
		return accessmodel.User{}, HandlePgErr(err)
	}
	return mapUserToModel(user), nil
}

func (r *UsersRepo) ListUsers(ctx context.Context, filter identitymodel.UsersListFilter) (identitymodel.UsersList, error) {
	if err := filter.Validate(); err != nil {
		return identitymodel.UsersList{}, err
	}

	total, err := r.queries.CountUsers(ctx, sqlc.CountUsersParams{
		Search: filter.Search,
		Role:   filter.Role,
	})
	if err != nil {
		return identitymodel.UsersList{}, HandlePgErr(err)
	}

	users, err := r.queries.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  filter.PageSize,
		Offset: Offset(filter.Page, filter.PageSize),
		Search: filter.Search,
		Role:   filter.Role,
	})
	if err != nil {
		return identitymodel.UsersList{}, HandlePgErr(err)
	}

	userRecords := make([]identitymodel.UserProfile, len(users))
	for i, u := range users {
		userRecords[i] = mapUserProfile(u)
	}

	return identitymodel.UsersList{
		Users:      userRecords,
		Pagination: paging.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (r *UsersRepo) UpdateUser(
	ctx context.Context,
	params identitymodel.UpdateUserParams,
) error {
	if err := params.Validate(); err != nil {
		return err
	}

	err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:              params.Id,
		Username:        params.Username,
		Role:            params.Role,
		Email:           params.Email,
		AvatarUrl:       params.AvatarUrl,
		ExpiresAt:       params.ExpiresAt,
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
	affected, err := r.queries.ClaimTemporaryUser(ctx, sqlc.ClaimTemporaryUserParams{
		ID:              id,
		ClaimedByUserID: claimedByUserID,
		ClaimedAt:       claimedAt,
	})
	if err != nil {
		return HandlePgErr(err)
	}
	if affected != 1 {
		return pkg.Wrap(pkg.ErrConflict, nil, "temporary account is expired or already claimed")
	}
	return nil
}

func (r *UsersRepo) ListClaimedAccountsByUserId(ctx context.Context, claimedByUserID uuid.UUID) ([]identitymodel.ClaimedAccountItem, error) {
	users, err := r.queries.ListClaimedAccountsByUserId(ctx, claimedByUserID)
	if err != nil {
		return nil, HandlePgErr(err)
	}

	res := make([]identitymodel.ClaimedAccountItem, len(users))
	for i, u := range users {
		var claimedAt time.Time
		if u.ClaimedAt != nil {
			claimedAt = *u.ClaimedAt
		}
		res[i] = identitymodel.ClaimedAccountItem{ID: u.ID, Username: u.Username, ClaimedAt: claimedAt, ExpiresAt: u.ExpiresAt}
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

func (r *UsersRepo) LockUser(ctx context.Context, id uuid.UUID) error {
	_, err := r.queries.LockUser(ctx, id)
	return HandlePgErr(err)
}

func mapUserProfile(row sqlc.ListUsersRow) identitymodel.UserProfile {
	return identitymodel.UserProfile{Id: row.ID, Username: row.Username, Role: accessmodel.UserRole(row.Role), Email: row.Email, AvatarUrl: row.AvatarUrl, ExpiresAt: row.ExpiresAt, IsEmailVerified: row.IsEmailVerified, ClaimedByUserID: row.ClaimedByUserID, ClaimedAt: row.ClaimedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func (r *UsersRepo) GetUserProfileByUsername(ctx context.Context, username string) (identitymodel.UserProfile, error) {
	row, err := r.queries.GetUserProfileByUsername(ctx, strings.TrimPrefix(username, "@"))
	if err != nil {
		return identitymodel.UserProfile{}, HandlePgErr(err)
	}
	return mapUserProfile(sqlc.ListUsersRow(row)), nil
}

func (r *UsersRepo) GetUserProfileByID(ctx context.Context, id uuid.UUID) (identitymodel.UserProfile, error) {
	row, err := r.queries.GetUserProfileByID(ctx, id)
	if err != nil {
		return identitymodel.UserProfile{}, HandlePgErr(err)
	}
	return mapUserProfile(sqlc.ListUsersRow(row)), nil
}
