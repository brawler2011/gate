package pg

import (
	"context"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        params.Id,
		Username:  params.Username,
		Role:      sqlc.UserRole(params.Role),
		KratosID:  params.KratosId,
		Email:     params.Email,
		Name:      params.Name,
		Surname:   params.Surname,
		Bio:       params.Bio,
		AvatarUrl: params.AvatarUrl,
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
	return models.User{
		Id:        user.ID,
		Username:  user.Username,
		Role:      models.UserRole(user.Role),
		KratosID:  user.KratosID,
		Email:     user.Email,
		Name:      user.Name,
		Surname:   user.Surname,
		Bio:       user.Bio,
		AvatarUrl: user.AvatarUrl,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func (r *UsersRepo) GetUserByKratosId(ctx context.Context, kratosId uuid.UUID) (models.User, error) {
	user, err := r.queries.GetUserByKratosId(ctx, kratosId)
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
		Users: userRecords,
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
		role = sqlc.NullUserRole{UserRole: sqlc.UserRole(*params.Role), Valid: true}
	}

	err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        params.Id,
		Username:  params.Username,
		Role:      role,
		Email:     params.Email,
		Name:      params.Name,
		Surname:   params.Surname,
		Bio:       params.Bio,
		AvatarUrl: params.AvatarUrl,
	})
	if err != nil {
		return HandlePgErr(err)
	}

	return nil
}

func userRoleToStringPtr(role *models.UserRole) *string {
	if role == nil {
		return nil
	}
	str := string(*role)
	return &str
}
