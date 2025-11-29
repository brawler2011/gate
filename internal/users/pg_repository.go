package users

import (
	"context"

	"github.com/gate149/core/internal/models"
	userssqlc "github.com/gate149/core/internal/users/sqlc"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	queries *userssqlc.Queries
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		queries: userssqlc.New(db),
	}
}

func (r *Repository) CreateUser(ctx context.Context, params *models.CreateUserParams) error {
	err := r.queries.CreateUser(ctx, userssqlc.CreateUserParams{
		ID:       params.Id,
		Username: params.Username,
		Role:     userssqlc.UserRole(params.Role),
		KratosID: params.KratosId,
	})
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

func (r *Repository) GetUserById(ctx context.Context, id uuid.UUID) (userssqlc.User, error) {
	user, err := r.queries.GetUserById(ctx, id)
	if err != nil {
		return userssqlc.User{}, pkg.HandlePgErr(err)
	}
	return user, nil
}

func (r *Repository) GetUserByKratosId(ctx context.Context, kratosId string) (userssqlc.User, error) {
	user, err := r.queries.GetUserByKratosId(ctx, kratosId)
	if err != nil {
		return userssqlc.User{}, pkg.HandlePgErr(err)
	}
	return user, nil
}

func (r *Repository) ListUsers(ctx context.Context, filter *models.UsersFilter) ([]userssqlc.User, int64, error) {
	search := &filter.Search
	if filter.Search == "" {
		search = nil
	}
	role := &filter.Role
	if filter.Role == "" {
		role = nil
	}

	total, err := r.queries.CountUsers(ctx, userssqlc.CountUsersParams{
		Search: search,
		Role:   role,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	users, err := r.queries.ListUsers(ctx, userssqlc.ListUsersParams{
		Limit:  int32(filter.PageSize),
		Offset: int32(filter.Offset()),
		Search: search,
		Role:   role,
	})
	if err != nil {
		return nil, 0, pkg.HandlePgErr(err)
	}

	return users, total, nil
}
