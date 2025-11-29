package users

import (
	"context"
	"log/slog"

	"github.com/gate149/core/internal/cache"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	userssqlc "github.com/gate149/core/internal/users/sqlc"
	"github.com/google/uuid"
)

type Repo interface {
	CreateUser(ctx context.Context, params *models.CreateUserParams) error
	GetUserById(ctx context.Context, id uuid.UUID) (userssqlc.User, error)
	GetUserByKratosId(ctx context.Context, kratosId string) (userssqlc.User, error)
	ListUsers(ctx context.Context, filter *models.UsersFilter) ([]userssqlc.User, int64, error)
}

type UsersUseCase struct {
	repo  Repo
	cache cache.Cache
}

func NewUseCase(repo Repo, cache cache.Cache) *UsersUseCase {
	return &UsersUseCase{
		repo:  repo,
		cache: cache,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input *models.CreateUserInput) (uuid.UUID, error) {
	id := uuid.New()

	params := &models.CreateUserParams{
		Id:       id,
		Username: input.Username,
		Role:     input.Role,
		KratosId: input.KratosId,
	}

	if err := u.repo.CreateUser(ctx, params); err != nil {
		return uuid.Nil, err
	}

	// Invalidate cache if needed (though we just created it, so getting by ID might not be cached yet)
	// But getting by KratosID might be cached if we cached misses? We didn't implement that.
	// Just to be safe/consistent.
	_ = u.cache.Delete(ctx, cache.UserKey(id))

	return id, nil
}

func (u *UsersUseCase) GetUserById(ctx context.Context, id uuid.UUID) (domain.User, error) {
	// Try cache
	var cached domain.User
	if err := u.cache.Get(ctx, cache.UserKey(id), &cached); err == nil {
		return cached, nil
	}

	// Query DB
	user, err := u.repo.GetUserById(ctx, id)
	if err != nil {
		return domain.User{}, err
	}

	// Map to domain
	domainUser := domain.UserFromSqlc(user)

	// Set cache
	if err := u.cache.Set(ctx, cache.UserKey(id), domainUser, cache.UserTTL); err != nil {
		slog.Error("failed to cache user", "error", err, "user_id", id)
	}

	return domainUser, nil
}

func (u *UsersUseCase) GetUserByKratosId(ctx context.Context, kratosId string) (domain.User, error) {
	// We could cache by KratosID too, but let's stick to ID for primary cache
	// Or we can cache kratos_id -> uuid mapping.
	// For simplicity, just fetch from DB.
	user, err := u.repo.GetUserByKratosId(ctx, kratosId)
	if err != nil {
		return domain.User{}, err
	}

	return domain.UserFromSqlc(user), nil
}

func (u *UsersUseCase) ListUsers(ctx context.Context, filter *models.UsersFilter) (*domain.UsersList, error) {
	users, total, err := u.repo.ListUsers(ctx, filter)
	if err != nil {
		return nil, err
	}

	domainUsers := make([]domain.User, len(users))
	for i, user := range users {
		domainUsers[i] = domain.UserFromSqlc(user)
	}

	return &domain.UsersList{
		Users:      domainUsers,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}
