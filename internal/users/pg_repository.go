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

func (r *Repository) CreateUser(ctx context.Context, user *models.UserCreation) (uuid.UUID, error) {
	const op = "UsersRepository.CreateUser"

	rows, err := r.db.QueryxContext(
		ctx,
		CreateUserQuery,
		user.Id,
		user.Username,
		user.Role,
		user.KratosId,
	)
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err, op)
	}

	defer rows.Close()
	var id uuid.UUID
	rows.Next()
	err = rows.Scan(&id)
	if err != nil {
		return uuid.Nil, pkg.HandlePgErr(err, op)
	}

	return id, nil
}

//go:embed sql/get_user_by_id.sql
var GetUserByIdQuery string

func (r *Repository) GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const op = "UsersRepository.ReadUserById"

	var user models.User
	err := r.db.GetContext(ctx, &user, GetUserByIdQuery, id)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &user, nil
}

//go:embed sql/get_user_by_kratos_id.sql
var GetUserByKratosIdQuery string

func (r *Repository) GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error) {
	const op = "UsersRepository.ReadUserByKratosId"

	var user models.User
	err := r.db.GetContext(ctx, &user, GetUserByKratosIdQuery, kratosId)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &user, nil
}

//go:embed sql/search_users.sql
var SearchUsersQuery string

//go:embed sql/count_search_users.sql
var CountSearchUsersQuery string

// SearchUsers searches users by search query and role with pagination using PostgreSQL trigram
func (r *Repository) SearchUsers(ctx context.Context, search *models.UsersSearch) (*models.UsersList, error) {
	const op = "UsersRepository.SearchUsers"

	offset := (search.Page - 1) * search.Page

	var total int64
	err := r.db.GetContext(ctx, &total, CountSearchUsersQuery, search.Search, search.Role)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	var users []*models.User
	err = r.db.SelectContext(ctx, &users, SearchUsersQuery, search.Search, search.Role, search.Page, offset)
	if err != nil {
		return nil, pkg.HandlePgErr(err, op)
	}

	return &models.UsersList{
		Users: users,
		Pagination: models.Pagination{
			Total: total,
			Page:  search.Page,
		},
	}, nil
}
