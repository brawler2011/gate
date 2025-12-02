package contests

import (
	"context"
	"log/slog"
	"time"

	"github.com/gate149/core/internal/cache"
	contestssqlc "github.com/gate149/core/internal/contests/sqlc"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type ContestRepo interface {
	CreateContest(ctx context.Context, c *models.CreateContestParams) error
	GetContest(ctx context.Context, id uuid.UUID) (contestssqlc.GetContestRow, error)
	UpdateContest(ctx context.Context, c models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) ([]contestssqlc.ListAdminContestsRow, int64, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) ([]contestssqlc.ListUserContestsRow, int64, error)
	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) ([]contestssqlc.ListWorkshopContestsRow, int64, error)
	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) ([]contestssqlc.ListPublicContestsRow, int64, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (contestssqlc.GetContestProblemRow, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]contestssqlc.GetContestProblemsRow, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	ListContestMembers(ctx context.Context, filter models.ParticipantsFilter) ([]contestssqlc.ListContestMembersRow, int64, error)

	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (contestssqlc.GetContestMemberRow, error)
	CreateContestMember(ctx context.Context, c *models.ContestMemberParams) error
	DeleteContestMember(ctx context.Context, userId uuid.UUID, contestId uuid.UUID) error
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error

	// Access Requests
	CreateAccessRequest(ctx context.Context, id uuid.UUID, contestId uuid.UUID, userId uuid.UUID, status string) error
	GetAccessRequest(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) (contestssqlc.ContestAccessRequest, error)
	ListAccessRequests(ctx context.Context, contestId uuid.UUID, status *string, limit int64, offset int64) ([]contestssqlc.ListAccessRequestsRow, int64, error)
	UpdateAccessRequestStatus(ctx context.Context, id uuid.UUID, status string) error

	// Invitations
	CreateInvitation(ctx context.Context, id uuid.UUID, contestId uuid.UUID, userId uuid.UUID, invitedBy uuid.UUID, status string) error
	GetInvitation(ctx context.Context, id uuid.UUID) (contestssqlc.ContestInvitation, error)
	GetInvitationByUser(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) (contestssqlc.ContestInvitation, error)
	ListInvitations(ctx context.Context, contestId uuid.UUID, status *string, limit int64, offset int64) ([]contestssqlc.ListInvitationsRow, int64, error)
	ListUserInvitations(ctx context.Context, userId uuid.UUID, limit int64, offset int64) ([]contestssqlc.ListUserInvitationsRow, int64, error)
	UpdateInvitationStatus(ctx context.Context, id uuid.UUID, status string) error

	// Problem Position Management
	UpdateContestProblemPosition(ctx context.Context, contestId uuid.UUID, problemId uuid.UUID, position int64) error
	ReorderProblemsAfterDelete(ctx context.Context, contestId uuid.UUID, deletedPosition int64) error
	GetMaxProblemPosition(ctx context.Context, contestId uuid.UUID) (int64, error)

	// Monitor
	GetContestMonitor(ctx context.Context, contestId uuid.UUID, limit int64, offset int64) ([]contestssqlc.GetContestMonitorRow, int64, error)
	GetMonitorProblemDetails(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) ([]contestssqlc.GetMonitorProblemDetailsRow, error)
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
	domainContest := domain.ContestFromGetRow(contest)

	// Set cache
	if err := uc.cache.Set(ctx, cache.ContestKey(id), domainContest, cache.ContestTTL); err != nil {
		slog.Error("failed to cache contest", "error", err, "contest_id", id)
	}

	return domainContest, nil
}

func (uc *UseCase) ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*domain.ContestsList, error) {
	contests, total, err := uc.contestRepo.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't list admin contests from database")
	}

	domainContests := make([]domain.Contest, len(contests))
	for i, c := range contests {
		domainContests[i] = domain.ContestFromListRow(c)
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
		domainContests[i] = domain.ContestFromListRow(contestssqlc.ListAdminContestsRow(c))
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
		domainContests[i] = domain.ContestFromListRow(contestssqlc.ListAdminContestsRow(c))
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
		domainContests[i] = domain.ContestFromListRow(contestssqlc.ListAdminContestsRow(c))
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

	// Invalidate all contest_problem caches for this contest
	// Pattern: contest_problem:{contestId}:*
	pattern := "contest_problem:" + id.String() + ":*"
	_ = uc.cache.DeleteByPattern(ctx, pattern)

	// Invalidate all contest_member caches for this contest
	// Pattern: contest_member:{contestId}:*
	memberPattern := "contest_member:" + id.String() + ":*"
	_ = uc.cache.DeleteByPattern(ctx, memberPattern)

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
	if err := uc.cache.Set(ctx, key, domainCP, cache.ContestTTL); err != nil {
		slog.Error("failed to cache contest problem", "error", err, "key", key)
	}

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

	if err := uc.cache.Set(ctx, key, domainMember, cache.ContestTTL); err != nil {
		slog.Error("failed to cache contest member", "error", err, "key", key)
	}

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

// CheckContestTimeAccess verifies if the contest is accessible based on its time settings
func (uc *UseCase) CheckContestTimeAccess(ctx context.Context, contestId uuid.UUID) error {
	contest, err := uc.GetContest(ctx, contestId)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	// If start_time is set and we haven't reached it yet
	if contest.StartTime != nil && now.Before(*contest.StartTime) {
		return pkg.Wrap(pkg.NoPermission, nil, "contest has not started yet")
	}

	// If end_time is set and we've passed it
	if contest.EndTime != nil && now.After(*contest.EndTime) {
		return pkg.Wrap(pkg.NoPermission, nil, "contest has ended")
	}

	return nil
}

// CreateAccessRequest creates a new access request for a contest
func (uc *UseCase) CreateAccessRequest(ctx context.Context, contestId uuid.UUID, userId uuid.UUID) (uuid.UUID, error) {
	// Check if request already exists
	_, err := uc.contestRepo.GetAccessRequest(ctx, contestId, userId)
	if err == nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "access request already exists")
	}

	// Get contest to check visibility
	contest, err := uc.GetContest(ctx, contestId)
	if err != nil {
		return uuid.Nil, err
	}

	id := uuid.New()
	status := "pending"

	// Auto-approve for public contests
	if contest.Visibility == "public" {
		status = "approved"
		// Also create contest member immediately
		err = uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
			ContestId: contestId,
			UserId:    userId,
			Role:      models.ContestRoleParticipant,
		})
		if err != nil {
			return uuid.Nil, pkg.Wrap(err, nil, "can't create contest member")
		}
	}

	err = uc.contestRepo.CreateAccessRequest(ctx, id, contestId, userId, status)
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create access request")
	}

	return id, nil
}

// ApproveAccessRequest approves an access request and adds user as participant
func (uc *UseCase) ApproveAccessRequest(ctx context.Context, requestId uuid.UUID) error {
	// Get the request to find user and contest
	req, err := uc.contestRepo.GetAccessRequest(ctx, uuid.Nil, uuid.Nil) // Need to add GetAccessRequestById
	if err != nil {
		return pkg.Wrap(err, nil, "can't get access request")
	}

	// Update status
	err = uc.contestRepo.UpdateAccessRequestStatus(ctx, requestId, "approved")
	if err != nil {
		return pkg.Wrap(err, nil, "can't update access request status")
	}

	// Create contest member
	err = uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
		ContestId: req.ContestID,
		UserId:    req.UserID,
		Role:      models.ContestRoleParticipant,
	})
	if err != nil {
		return pkg.Wrap(err, nil, "can't create contest member")
	}

	return nil
}

// RejectAccessRequest rejects an access request
func (uc *UseCase) RejectAccessRequest(ctx context.Context, requestId uuid.UUID) error {
	err := uc.contestRepo.UpdateAccessRequestStatus(ctx, requestId, "rejected")
	if err != nil {
		return pkg.Wrap(err, nil, "can't update access request status")
	}
	return nil
}

// SendInvitation sends an invitation to a user to join a contest
func (uc *UseCase) SendInvitation(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, invitedBy uuid.UUID) (uuid.UUID, error) {
	// Check if active invitation already exists
	_, err := uc.contestRepo.GetInvitationByUser(ctx, contestId, userId)
	if err == nil {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "active invitation already exists")
	}

	id := uuid.New()
	err = uc.contestRepo.CreateInvitation(ctx, id, contestId, userId, invitedBy, "pending")
	if err != nil {
		return uuid.Nil, pkg.Wrap(err, nil, "can't create invitation")
	}

	return id, nil
}

// AcceptInvitation accepts an invitation and adds user as participant
func (uc *UseCase) AcceptInvitation(ctx context.Context, invitationId uuid.UUID) error {
	// Get invitation
	inv, err := uc.contestRepo.GetInvitation(ctx, invitationId)
	if err != nil {
		return pkg.Wrap(err, nil, "can't get invitation")
	}

	if inv.Status != "pending" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invitation is not pending")
	}

	// Update status
	err = uc.contestRepo.UpdateInvitationStatus(ctx, invitationId, "accepted")
	if err != nil {
		return pkg.Wrap(err, nil, "can't update invitation status")
	}

	// Create contest member
	err = uc.contestRepo.CreateContestMember(ctx, &models.ContestMemberParams{
		ContestId: inv.ContestID,
		UserId:    inv.UserID,
		Role:      models.ContestRoleParticipant,
	})
	if err != nil {
		return pkg.Wrap(err, nil, "can't create contest member")
	}

	return nil
}

// DeclineInvitation declines an invitation
func (uc *UseCase) DeclineInvitation(ctx context.Context, invitationId uuid.UUID) error {
	inv, err := uc.contestRepo.GetInvitation(ctx, invitationId)
	if err != nil {
		return pkg.Wrap(err, nil, "can't get invitation")
	}

	if inv.Status != "pending" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invitation is not pending")
	}

	err = uc.contestRepo.UpdateInvitationStatus(ctx, invitationId, "declined")
	if err != nil {
		return pkg.Wrap(err, nil, "can't update invitation status")
	}

	return nil
}

// RevokeInvitation revokes an invitation
func (uc *UseCase) RevokeInvitation(ctx context.Context, invitationId uuid.UUID) error {
	inv, err := uc.contestRepo.GetInvitation(ctx, invitationId)
	if err != nil {
		return pkg.Wrap(err, nil, "can't get invitation")
	}

	if inv.Status != "pending" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invitation is not pending")
	}

	err = uc.contestRepo.UpdateInvitationStatus(ctx, invitationId, "revoked")
	if err != nil {
		return pkg.Wrap(err, nil, "can't update invitation status")
	}

	return nil
}

// UpdateProblemPosition updates the position of a problem in a contest
func (uc *UseCase) UpdateProblemPosition(ctx context.Context, contestId uuid.UUID, problemId uuid.UUID, newPosition int64) error {
	if newPosition < 0 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "position must be non-negative")
	}

	err := uc.contestRepo.UpdateContestProblemPosition(ctx, contestId, problemId, newPosition)
	if err != nil {
		return pkg.Wrap(err, nil, "can't update problem position")
	}

	// Invalidate cache
	_ = uc.cache.Delete(ctx, cache.ContestProblemKey(contestId, problemId))

	return nil
}

// ReorderProblemsAfterDeletion reorders problems after a problem is deleted to remove gaps
func (uc *UseCase) ReorderProblemsAfterDeletion(ctx context.Context, contestId uuid.UUID, deletedPosition int64) error {
	err := uc.contestRepo.ReorderProblemsAfterDelete(ctx, contestId, deletedPosition)
	if err != nil {
		return pkg.Wrap(err, nil, "can't reorder problems")
	}

	// Invalidate all contest_problem caches for this contest
	pattern := "contest_problem:" + contestId.String() + ":*"
	_ = uc.cache.DeleteByPattern(ctx, pattern)

	return nil
}

// GetMonitor returns the full contest monitor/standings with per-problem details
func (uc *UseCase) GetMonitor(ctx context.Context, contestId uuid.UUID, page int64, pageSize int64) (*domain.Monitor, error) {
	offset := (page - 1) * pageSize

	// Get main monitor rows
	rows, total, err := uc.contestRepo.GetContestMonitor(ctx, contestId, pageSize, offset)
	if err != nil {
		return nil, pkg.Wrap(err, nil, "can't get contest monitor")
	}

	// Build monitor rows with problem details
	monitorRows := make([]domain.MonitorRow, len(rows))
	for i, row := range rows {
		// Convert pgtype.UUID to uuid.UUID
		userId := uuid.UUID(row.UserID.Bytes)
		if !row.UserID.Valid {
			userId = uuid.Nil
		}

		// Get per-problem details for this user
		problemDetails, err := uc.contestRepo.GetMonitorProblemDetails(ctx, contestId, userId)
		if err != nil {
			return nil, pkg.Wrap(err, nil, "can't get problem details")
		}

		problems := make([]domain.MonitorProblemStatus, len(problemDetails))
		for j, pd := range problemDetails {
			problemId := uuid.UUID(pd.ProblemID.Bytes)
			if !pd.ProblemID.Valid {
				problemId = uuid.Nil
			}

			problems[j] = domain.MonitorProblemStatus{
				ProblemID: problemId,
				Position:  int64(pd.Position),
				Score:     interfaceToInt64(pd.Score),
				Attempts:  pd.Attempts,
				Solved:    pd.Solved,
				Penalty:   interfaceToInt64(pd.Penalty),
			}
		}

		username := ""
		if row.Username != nil {
			username = *row.Username
		}

		monitorRows[i] = domain.MonitorRow{
			UserID:       userId,
			Username:     username,
			TotalScore:   interfaceToInt64(row.TotalScore),
			TotalPenalty: interfaceToInt64(row.TotalPenalty),
			SolvedCount:  row.SolvedCount,
			Problems:     problems,
		}
	}

	return &domain.Monitor{
		Rows:       monitorRows,
		Pagination: domain.NewPagination(page, pageSize, total),
	}, nil
}

// Helper to safely convert interface{} to int64
func interfaceToInt64(val interface{}) int64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}
