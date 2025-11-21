package users

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	_ "embed"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		db: db,
	}
}

//go:embed sql/create_user.sql
var CreateUserQuery string

func (r *Repository) CreateUser(ctx context.Context, params *models.CreateUserParams) error {
	_, err := r.db.ExecContext(
		ctx,
		CreateUserQuery,
		params.Id,
		params.Username,
		params.Role,
		params.KratosId,
	)
	if err != nil {
		return pkg.HandlePgErr(err)
	}
	return nil
}

//go:embed sql/get_user_by_id.sql
var GetUserByIdQuery string

func (r *Repository) GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, GetUserByIdQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return &user, nil
}

//go:embed sql/get_user_by_kratos_id.sql
var GetUserByKratosIdQuery string

func (r *Repository) GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, GetUserByKratosIdQuery, kratosId)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}
	return &user, nil
}

//go:embed sql/list_users.sql
var ListUsersQuery string

//go:embed sql/count_users.sql
var CountUsersQuery string

func (r *Repository) ListUsers(ctx context.Context, filter *models.UsersFilter) (*models.UsersList, error) {
	var total int64
	err := r.db.GetContext(ctx, &total, CountUsersQuery,
		filter.Search,
		filter.Role,
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	var users []*models.User
	err = r.db.SelectContext(ctx, &users, ListUsersQuery,
		filter.Search,
		filter.Role,
		filter.PageSize,
		filter.Offset(),
	)
	if err != nil {
		return nil, pkg.HandlePgErr(err)
	}

	return &models.UsersList{
		Users:      users,
		Pagination: models.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}
