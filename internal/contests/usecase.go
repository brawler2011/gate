package contests

import (
	"context"

	"github.com/gate149/core/internal/cache"
	contestssqlc "github.com/gate149/core/internal/contests/sqlc"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type ContestRepo interface {
	CreateContest(ctx context.Context, c *models.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (contestssqlc.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]contestssqlc.Contest, int64, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]contestssqlc.Contest, int64, error)
	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]contestssqlc.Contest, int64, error)
	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]contestssqlc.Contest, int64, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (contestssqlc.GetContestProblemRow, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestssqlc.GetContestProblemsRow, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]contestssqlc.ListContestMembersRow, int64, error)

	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (contestssqlc.GetContestMemberRow, error)
	CreateContestMember(ctx context.Context, c *models.ContestMemberParams) error
	DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
}

type UseCase struct {
	contestRepo ContestRepo
	cache       cache.Cache
}

func NewContestUseCase(
	contestRepo ContestRepo,
	cache cache.Cache,
) *UseCase {
	return &UseCase{
		contestRepo: contestRepo,
		cache:       cache,
	}
}

func (uc *UseCase) CreateContest(
	ctx context.Context,
	c *models.CreateContestInput,
) (uuid.UUID, error) {
	params := &models.CreateContestParams{
		Id:     uuid.New(),
		Title:  c.Title,
		UserId: c.UserId,
	}

	err := uc.contestRepo.CreateContest(ctx, params)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create contest")
	}

	err = uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
		ContestId: params.Id,
		UserId:    c.UserId,
		Role:      models.ContestRoleOwner,
	})
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create contest member")
	}

	return params.Id, nil
}

func (uc *UseCase) GetContest(ctx context.Context, id uuid.UUID) (domain.Contest, error) {
	// Try cache
	var cached domain.Contest
	if err := uc.cache.Get(ctx, cache.ContestKey(id), &cached); err == nil {
		return cached, nil
	}

	// Query DB
	contest, err := uc.contestRepo.GetContest(ctx, id)
	if err != nil {
		return domain.Contest{}, err
	}

	// Map
	domainContest := domain.ContestFromSqlc(contest)

	// Set cache
	_ = uc.cache.Set(ctx, cache.ContestKey(id), domainContest, cache.ContestTTL)

	return domainContest, nil
}

func (uc *UseCase) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*domain.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list admin contests from database")
	}

	domainContests := make([]domain.Contest, len(contests))
	for i, c := range contests {
		domainContests[i] = domain.ContestFromSqlc(c)
	}

	return &domain.ContestsList{
		Contests:   domainContests,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *UseCase) ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*domain.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListUserContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list user contests from database")
	}

	domainContests := make([]domain.Contest, len(contests))
	for i, c := range contests {
		domainContests[i] = domain.ContestFromSqlc(c)
	}

	return &domain.ContestsList{
		Contests:   domainContests,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *UseCase) ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*domain.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list workshop contests from database")
	}

	domainContests := make([]domain.Contest, len(contests))
	for i, c := range contests {
		domainContests[i] = domain.ContestFromSqlc(c)
	}

	return &domain.ContestsList{
		Contests:   domainContests,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *UseCase) ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*domain.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list public contests from database")
	}

	domainContests := make([]domain.Contest, len(contests))
	for i, c := range contests {
		domainContests[i] = domain.ContestFromSqlc(c)
	}

	return &domain.ContestsList{
		Contests:   domainContests,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

func (uc *UseCase) UpdateContest(ctx context.Context, c models.ContestUpdate) error {
	err := uc.contestRepo.UpdateContest(ctx, c)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update contest")
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ContestKey(c.Id))

	return nil
}

func (uc *UseCase) DeleteContest(ctx context.Context, id uuid.UUID) error {
	err := uc.contestRepo.DeleteContest(ctx, id)
	if err != nil {
		return err
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ContestKey(id))

	return nil
}

func (uc *UseCase) CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error {
	// Invalidate list of problems for this contest? We don't cache lists currently.
	// But we cache GetContestProblem.
	// No explicit invalidation needed unless we cache list of problem IDs.
	// But let's invalidate ContestProblemKey just in case.
	_ = uc.cache.Delete(ctx, cache.ContestProblemKey(c.ContestId, c.ProblemId))
	return uc.contestRepo.CreateContestProblem(ctx, c)
}

func (uc *UseCase) GetContestProblem(ctx context.Context, c models.ContestProblemGet) (domain.ContestProblem, error) {
	// Cache-aside
	key := cache.ContestProblemKey(c.ContestId, c.ProblemId)
	var cached domain.ContestProblem
	if err := uc.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	cp, err := uc.contestRepo.GetContestProblem(ctx, c)
	if err != nil {
		return domain.ContestProblem{}, err
	}

	domainCP := domain.ContestProblemFromSqlc(cp)
	_ = uc.cache.Set(ctx, key, domainCP, cache.ContestTTL)

	return domainCP, nil
}

func (uc *UseCase) GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]domain.ContestProblem, error) {
	// No caching for lists
	problems, err := uc.contestRepo.GetContestProblems(ctx, contestId)
	if err != nil {
		return nil, err
	}

	domainProblems := make([]domain.ContestProblem, len(problems))
	for i, p := range problems {
		domainProblems[i] = domain.ContestProblemsListRowFromSqlc(p)
	}

	return domainProblems, nil
}

func (uc *UseCase) DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error {
	_ = uc.cache.Delete(ctx, cache.ContestProblemKey(c.ContestId, c.ProblemId))
	return uc.contestRepo.DeleteContestProblem(ctx, c)
}

func (uc *UseCase) CreateParticipant(ctx context.Context, c models.ParticipantCreation) error {
	// Invalidate cache for this member
	_ = uc.cache.Delete(ctx, cache.ContestMemberKey(c.ContestId, c.UserId))
	return uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
		ContestId: c.ContestId,
		UserId:    c.UserId,
		Role:      models.ContestRoleParticipant,
	})
}

func (uc *UseCase) DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error {
	_ = uc.cache.Delete(ctx, cache.ContestMemberKey(c.ContestId, c.UserId))
	return uc.contestRepo.DeleteContestMember(ctx, c.UserId, c.ContestId)
}

func (uc *UseCase) ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*domain.ContestMembersList, error) {
	members, total, err := uc.contestRepo.ListContestMembers(ctx, filter)
	if err != nil {
		return nil, err
	}

	domainMembers := make([]domain.ContestMember, len(members))
	for i, m := range members {
		domainMembers[i] = domain.ContestMemberFromSqlc(m)
	}

	return &domain.ContestMembersList{
		Members:    domainMembers,
		Pagination: domain.NewPagination(filter.Page, filter.PageSize, total),
	}, nil
}

// GetContestMember returns minimal info (role) mostly for permissions, but now returns full ContestMember struct mapped to domain
// Actually GetContestMemberRepo returns GetContestMemberRow.
// But we might want to return domain.ContestMember?
// GetContestMemberRow has (contest_id, user_id, role). Minimal.
// ContestMemberFromSqlc expects ListContestMembersRow (full).
// I should add ContestMemberFromGetRow to mappers or just return custom struct.
// But plan said "Convert all return types to domain types".
// I'll create a domain struct for MemberRecord (minimal) or assume GetContestMemberRow is enough.
// Domain `ContestMember` has full info.
// I'll map `GetContestMemberRow` to `ContestMember` filling only available fields (Role). Or define `ContestMemberRecord` in domain.
// models.ContestMemberRecord existed.
// I'll check `domain/contest.go`. I defined `ContestMember` (full).
// I'll update `domain/contest.go` to add `ContestMemberRecord` (minimal) or just use `ContestMember` with empty fields.
// Or I can add `GetContestMemberRow` mapper.
// `GetContestMember` query returns `contest_id, user_id, role`.
// I'll add `ContestMemberRecord` to domain if needed, or reuse `ContestMember`.
// I'll reuse `ContestMember`.

func (uc *UseCase) GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (domain.ContestMember, error) {
	// Cache-aside
	key := cache.ContestMemberKey(c.ContestId, c.UserId)
	var cached domain.ContestMember
	if err := uc.cache.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	member, err := uc.contestRepo.GetContestMember(ctx, c)
	if err != nil {
		return domain.ContestMember{}, err
	}

	// Manual mapping because mapper expects List row
	domainMember := domain.ContestMember{
		ContestID: member.ContestID,
		UserID:    member.UserID,
		Role:      string(member.Role),
		// other fields empty
	}

	_ = uc.cache.Set(ctx, key, domainMember, cache.ContestTTL)

	return domainMember, nil
}

func (uc *UseCase) UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error {
	if role != models.ContestRoleOwner && role != models.ContestRoleModerator && role != models.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	// We use GetContestMember which uses cache.
	currentMember, err := uc.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestId,
		UserId:    userId,
	})
	if err != nil {
		return pkg.Wrap(err, nil, "can't get contest member")
	}

	if currentMember.Role == models.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role from owner")
	}

	if role == models.ContestRoleOwner {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot change role to owner")
	}

	err = uc.contestRepo.UpdateContestMember(ctx, contestId, userId, role)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update contest member")
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ContestMemberKey(contestId, userId))

	return nil
}
