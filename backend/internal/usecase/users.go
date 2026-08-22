package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

type UsersUseCase struct {
	usersRepo    interfaces.UsersRepo
	contestsRepo interfaces.ContestsRepo
	outboxRepo   interfaces.OutboxRepo
	txManager    interfaces.Transactor
}

func NewUsersUseCase(
	repo interfaces.UsersRepo,
	contestsRepo interfaces.ContestsRepo,
	outboxRepo interfaces.OutboxRepo,
	txManager interfaces.Transactor,
) *UsersUseCase {
	return &UsersUseCase{
		usersRepo:    repo,
		contestsRepo: contestsRepo,
		outboxRepo:   outboxRepo,
		txManager:    txManager,
	}
}

func (u *UsersUseCase) CreateUser(ctx context.Context, input models.CreateUserInput) (uuid.UUID, error) {
	id := uuid.New()

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrInternal, err, "failed to hash password")
	}

	params := models.CreateUserParams{
		Id:           id,
		Username:     input.Username,
		Role:         models.UserRole(input.Role),
		PasswordHash: string(hashed),
		Email:        input.Email,
		AvatarUrl:    input.AvatarUrl,
		ExpiresAt:    input.ExpiresAt,
	}

	// Create user directly (no image table anymore)
	err = u.usersRepo.CreateUser(ctx, params)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (u *UsersUseCase) GetUserById(ctx context.Context, id uuid.UUID) (models.User, error) {
	return u.usersRepo.GetUserById(ctx, id)
}

func (u *UsersUseCase) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	username = strings.TrimPrefix(username, "@")
	return u.usersRepo.GetUserByUsername(ctx, username)
}

func (u *UsersUseCase) ListUsers(ctx context.Context, filter models.UsersListFilter) (models.UsersList, error) {
	params := models.UsersListFilter{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		Search:   filter.Search,
		Role:     filter.Role,
	}

	usersList, err := u.usersRepo.ListUsers(ctx, params)
	if err != nil {
		return models.UsersList{}, err
	}

	return usersList, nil
}

func (u *UsersUseCase) UpdateUser(ctx context.Context, input models.UpdateUserInput) error {
	var role *models.UserRole
	if input.Role != nil {
		r := models.UserRole(*input.Role)
		role = &r
	}

	params := models.UpdateUserParams{
		Id:        input.Id,
		Username:  input.Username,
		Role:      role,
		Email:     input.Email,
		AvatarUrl: input.AvatarUrl,
		ExpiresAt: input.ExpiresAt,
	}

	return u.usersRepo.UpdateUser(ctx, params)
}

func (u *UsersUseCase) ClaimTemporaryUser(ctx context.Context, caller models.User, input models.ClaimTemporaryUserInput) (models.ClaimTemporaryUserResult, error) {
	if caller.IsGuest() {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}
	if caller.IsExpired() {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.NoPermission, nil, "permanent account has expired")
	}

	tempUser, err := u.usersRepo.GetUserByUsername(ctx, input.Username)
	if err != nil {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrBadInput, err, "temporary user not found")
	}

	// Must be a temporary user
	if !tempUser.IsTemporary() {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrBadInput, nil, "target account is not a temporary account")
	}

	// Must not be claimed already
	if tempUser.IsClaimed() {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrConflict, nil, "temporary account has already been claimed")
	}

	// Cannot claim oneself
	if tempUser.Id == caller.Id {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrBadInput, nil, "cannot claim own account")
	}

	// Verify temporary user password
	err = bcrypt.CompareHashAndPassword([]byte(tempUser.PasswordHash), []byte(input.Password))
	if err != nil {
		return models.ClaimTemporaryUserResult{}, pkg.Wrap(pkg.ErrUnauthenticated, err, "invalid credentials for temporary account")
	}

	var grantedContests []uuid.UUID

	err = u.txManager.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txUsersRepo := u.usersRepo.WithTx(tx)
		txContestsRepo := u.contestsRepo.WithTx(tx)

		now := time.Now()
		err := txUsersRepo.ClaimTemporaryUser(txCtx, tempUser.Id, caller.Id, now)
		if err != nil {
			return err
		}

		// Find contests of tempUser and grant access to caller
		memberships, err := txContestsRepo.ListUserContestMemberships(txCtx, tempUser.Id)
		if err != nil {
			return err
		}

		for _, m := range memberships {
			err = txContestsRepo.AddContestMemberIfNotExists(txCtx, m.ContestID, caller.Id, string(models.ContestRoleParticipant))
			if err != nil {
				return err
			}
			grantedContests = append(grantedContests, m.ContestID)
		}

		return nil
	})

	if err != nil {
		return models.ClaimTemporaryUserResult{}, err
	}

	return models.ClaimTemporaryUserResult{
		ClaimedUserID:   tempUser.Id,
		ClaimedUsername: tempUser.Username,
		ContestsGranted: grantedContests,
	}, nil
}

func (u *UsersUseCase) ListClaimedAccounts(ctx context.Context, userID uuid.UUID) ([]models.ClaimedAccountItem, error) {
	users, err := u.usersRepo.ListClaimedAccountsByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]models.ClaimedAccountItem, 0, len(users))
	for _, user := range users {
		var claimedAt time.Time
		if user.ClaimedAt != nil {
			claimedAt = *user.ClaimedAt
		}
		items = append(items, models.ClaimedAccountItem{
			ID:        user.Id,
			Username:  user.Username,
			ClaimedAt: claimedAt,
			ExpiresAt: user.ExpiresAt,
		})
	}

	return items, nil
}

