package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gate149/gate/backend/internal/domain/interfaces"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

type PermissionsUseCase struct {
	contestsRepo interfaces.ContestsRepo
	usersUC      interfaces.UsersUC
	problemsRepo interfaces.ProblemsRepo
	teamsRepo    interfaces.TeamsRepo
	orgsRepo     interfaces.OrganizationsRepo
}

func NewPermissionsUseCase(
	contestsRepo interfaces.ContestsRepo,
	usersUC interfaces.UsersUC,
	problemsRepo interfaces.ProblemsRepo,
	teamsRepo interfaces.TeamsRepo,
	orgsRepo interfaces.OrganizationsRepo,
) *PermissionsUseCase {
	return &PermissionsUseCase{
		contestsRepo: contestsRepo,
		usersUC:      usersUC,
		problemsRepo: problemsRepo,
		teamsRepo:    teamsRepo,
		orgsRepo:     orgsRepo,
	}
}

type contestContext struct {
	user               *models.User
	contest            *models.Contest
	contestRole        *models.ContestRole
	contestPermissions models.ContestPermissionMask
	isPublic           bool
	isStarted          bool
	isFinished         bool
	isOwner            bool
	isAdmin            bool
}

type problemContext struct {
	user       *models.User
	problem    *models.Problem
	permission *models.ProblemPermission
	isPublic   bool
	isOwner    bool
	isAdmin    bool
}

func (uc *PermissionsUseCase) HasContestPermission(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
	action models.ContestAction,
) (bool, error) {
	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	contest, err := uc.contestsRepo.GetContest(ctx, contestID)
	if err != nil {
		return false, fmt.Errorf("get contest: %w", err)
	}

	contestRole, contestPermissions, err := uc.resolveContestRoleAndMask(ctx, contestID, contest.OrganizationID, userID)
	if err != nil {
		return false, err
	}

	isStarted := true
	if contest.StartTime != nil {
		isStarted = contest.StartTime.Before(time.Now())
	}

	isFinished := false
	if contest.EndTime != nil {
		isFinished = contest.EndTime.Before(time.Now())
	}

	cc := &contestContext{
		user:               &user,
		contest:            &contest,
		contestRole:        contestRole,
		contestPermissions: contestPermissions,
		isPublic:           contest.Visibility == models.ContestVisibilityPublic,
		isStarted:          isStarted,
		isFinished:         isFinished,
		isOwner:            contest.OwnerID != nil && *contest.OwnerID == userID,
		isAdmin:            user.Role == models.UserRoleAdmin,
	}

	switch action {
	case models.ActionGetContest:
		return uc.canViewContest(cc), nil
	case models.ActionUpdateContest, models.ActionAdminContest:
		return uc.canAdminContest(cc), nil
	case models.ActionManageContest:
		return uc.canManageContest(cc), nil
	case models.ActionGetMonitor:
		return uc.canViewMonitor(cc), nil
	case models.ActionListUsersSubmissions:
		return uc.canListAllSubmissions(cc), nil
	case models.ActionListOwnSubmissions:
		return uc.canListOwnSubmissions(cc), nil
	case models.ActionGetOwnSubmission:
		return uc.canGetOwnSubmission(cc), nil
	case models.ActionGetOtherUserSubmission:
		return uc.canViewOtherSubmission(cc), nil
	case models.ActionCreateSubmission:
		return uc.canSubmit(cc), nil
	default:
		return false, fmt.Errorf("unknown contest action: %s", action)
	}
}

func (uc *PermissionsUseCase) canViewContest(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	if cc.isPublic {
		return true
	}

	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionGetContest)
}

func (uc *PermissionsUseCase) canAdminContest(cc *contestContext) bool {
	return cc.isAdmin || cc.isOwner
}

func (uc *PermissionsUseCase) canManageContest(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionManageContest)
}

func (uc *PermissionsUseCase) canViewMonitor(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionGetMonitor)
}

func (uc *PermissionsUseCase) canListAllSubmissions(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionListUsersSubmissions)
}

func (uc *PermissionsUseCase) canListOwnSubmissions(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}

	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionListOwnSubmissions)
}

func (uc *PermissionsUseCase) canGetOwnSubmission(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}

	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionGetOwnSubmission)
}

func (uc *PermissionsUseCase) canViewOtherSubmission(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionGetOtherUserSubmission)
}

func (uc *PermissionsUseCase) canSubmit(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	if cc.contestRole == nil {
		return false
	}

	return models.HasContestActionPermission(cc.contestPermissions, models.ActionCreateSubmission) && cc.isStarted && !cc.isFinished
}

// HasProblemPermission проверяет право на действие с задачей
func (uc *PermissionsUseCase) HasProblemPermission(
	ctx context.Context,
	problemID uuid.UUID,
	userID uuid.UUID,
	action models.ProblemAction,
) (bool, error) {
	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return false, fmt.Errorf("get problem: %w", err)
	}

	permission, err := uc.resolveProblemPermission(ctx, problemID, problem.OrganizationID, userID)
	if err != nil {
		return false, err
	}

	pc := &problemContext{
		user:       &user,
		problem:    &problem,
		permission: permission,
		isPublic:   problem.Visibility == models.ProblemVisibilityPublic,
		isOwner:    problem.OwnerID != nil && *problem.OwnerID == userID,
		isAdmin:    user.Role == models.UserRoleAdmin,
	}

	switch action {
	case models.ActionViewProblem:
		return uc.canViewProblem(pc), nil
	case models.ActionEditProblem:
		return uc.canEditProblem(pc), nil
	case models.ActionAdminProblem:
		return uc.canAdminProblem(pc), nil
	default:
		return false, fmt.Errorf("unknown problem action: %s", action)
	}
}

func (uc *PermissionsUseCase) canViewProblem(pc *problemContext) bool {
	if pc.isAdmin || pc.isOwner {
		return true
	}
	return pc.isPublic || uc.hasProblemPermission(pc, models.ProblemPermissionRead)
}

func (uc *PermissionsUseCase) canEditProblem(pc *problemContext) bool {
	return pc.isAdmin || pc.isOwner || uc.hasProblemPermission(pc, models.ProblemPermissionWrite)
}

func (uc *PermissionsUseCase) canAdminProblem(pc *problemContext) bool {
	return pc.isAdmin || pc.isOwner || uc.hasProblemPermission(pc, models.ProblemPermissionAdmin)
}

func (uc *PermissionsUseCase) GetEffectiveContestRole(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
) (*models.ContestRole, models.ContestPermissionMask, error) {
	contest, err := uc.contestsRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, 0, fmt.Errorf("get contest: %w", err)
	}

	return uc.resolveContestRoleAndMask(ctx, contestID, contest.OrganizationID, userID)
}

func (uc *PermissionsUseCase) GetContestPermissions(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
) (*models.ContestPermissions, error) {
	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	contest, err := uc.contestsRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, fmt.Errorf("get contest: %w", err)
	}

	contestRole, contestPermissions, err := uc.resolveContestRoleAndMask(ctx, contestID, contest.OrganizationID, userID)
	if err != nil {
		return nil, err
	}

	isStarted := true
	if contest.StartTime != nil {
		isStarted = contest.StartTime.Before(time.Now())
	}

	isFinished := false
	if contest.EndTime != nil {
		isFinished = contest.EndTime.Before(time.Now())
	}

	cc := &contestContext{
		user:               &user,
		contest:            &contest,
		contestRole:        contestRole,
		contestPermissions: contestPermissions,
		isPublic:           contest.Visibility == models.ContestVisibilityPublic,
		isStarted:          isStarted,
		isFinished:         isFinished,
		isOwner:            contest.OwnerID != nil && *contest.OwnerID == userID,
		isAdmin:            user.Role == models.UserRoleAdmin,
	}

	return &models.ContestPermissions{
		GetContest:             uc.canViewContest(cc),
		UpdateContest:          uc.canAdminContest(cc),
		ManageContest:          uc.canManageContest(cc),
		AdminContest:           uc.canAdminContest(cc),
		GetMonitor:             uc.canViewMonitor(cc),
		ListUsersSubmissions:   uc.canListAllSubmissions(cc),
		ListOwnSubmissions:     uc.canListOwnSubmissions(cc),
		GetOtherUserSubmission: uc.canViewOtherSubmission(cc),
		GetOwnSubmission:       uc.canGetOwnSubmission(cc),
		CreateSubmission:       uc.canSubmit(cc),
	}, nil
}

func (uc *PermissionsUseCase) GetProblemPermissions(
	ctx context.Context,
	problemID uuid.UUID,
	userID uuid.UUID,
) (*models.ProblemPermissions, error) {
	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("get problem: %w", err)
	}

	permission, err := uc.resolveProblemPermission(ctx, problemID, problem.OrganizationID, userID)
	if err != nil {
		return nil, err
	}

	pc := &problemContext{
		user:       &user,
		problem:    &problem,
		permission: permission,
		isPublic:   problem.Visibility == models.ProblemVisibilityPublic,
		isOwner:    problem.OwnerID != nil && *problem.OwnerID == userID,
		isAdmin:    user.Role == models.UserRoleAdmin,
	}

	return &models.ProblemPermissions{
		ViewProblem:  uc.canViewProblem(pc),
		EditProblem:  uc.canEditProblem(pc),
		AdminProblem: uc.canAdminProblem(pc),
	}, nil
}

// HasOrganizationPermission checks if a user has permission to perform an action on an organization
// This is a stub implementation - full implementation would check organization membership and roles
func (uc *PermissionsUseCase) HasOrganizationPermission(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
	action models.OrgAction,
) (bool, error) {
	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	// Admins have full access
	if user.Role == models.UserRoleAdmin {
		return true, nil
	}

	member, err := uc.orgsRepo.GetMember(ctx, orgID, userID)
	if err != nil {
		// User is not a member of the organization
		if action == models.ActionViewOrganization {
			return true, nil
		}
		return false, nil
	}

	switch action {
	case models.ActionViewOrganization:
		return true, nil
	case models.ActionManageOrganization:
		return member.Role == models.OrgRoleOwner || member.Role == models.OrgRoleAdmin, nil
	case models.ActionDeleteOrganization:
		return member.Role == models.OrgRoleOwner, nil
	default:
		return false, fmt.Errorf("unknown organization action: %s", action)
	}
}

func (uc *PermissionsUseCase) resolveContestRoleAndMask(
	ctx context.Context,
	contestID uuid.UUID,
	organizationID uuid.UUID,
	userID uuid.UUID,
) (*models.ContestRole, models.ContestPermissionMask, error) {
	var bestRole *models.ContestRole
	var bestMask models.ContestPermissionMask

	directMember, err := uc.contestsRepo.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	})
	if err == nil {
		bestRole, bestMask = pickHigherContestRoleWithMask(bestRole, bestMask, directMember.ContestRole, directMember.PermissionsMask)
	} else if !errors.Is(err, pkg.ErrNotFound) {
		return nil, 0, fmt.Errorf("get contest member: %w", err)
	}

	contestTeams, err := uc.contestsRepo.GetContestTeams(ctx, contestID)
	if err != nil {
		return nil, 0, fmt.Errorf("get contest teams: %w", err)
	}
	if len(contestTeams) == 0 {
		return bestRole, bestMask, nil
	}

	userTeams, err := uc.teamsRepo.GetUserTeamsByOrganization(ctx, userID, organizationID)
	if err != nil {
		return nil, 0, fmt.Errorf("get user teams: %w", err)
	}

	userTeamIDs := make(map[uuid.UUID]struct{}, len(userTeams))
	for _, team := range userTeams {
		userTeamIDs[team.ID] = struct{}{}
	}

	for _, contestTeam := range contestTeams {
		if _, ok := userTeamIDs[contestTeam.TeamID]; ok {
			bestRole, bestMask = pickHigherContestRoleWithMask(bestRole, bestMask, contestTeam.Role, contestTeam.PermissionsMask)
		}
	}

	return bestRole, bestMask, nil
}

func (uc *PermissionsUseCase) resolveProblemPermission(
	ctx context.Context,
	problemID uuid.UUID,
	organizationID uuid.UUID,
	userID uuid.UUID,
) (*models.ProblemPermission, error) {
	var bestPermission *models.ProblemPermission

	directMember, err := uc.problemsRepo.GetProblemMember(ctx, problemID, userID)
	if err == nil {
		if mappedPermission, ok := mapProblemRoleToPermission(directMember.Role); ok {
			bestPermission = pickHigherProblemPermission(bestPermission, mappedPermission)
		}
	} else if !errors.Is(err, pkg.ErrNotFound) {
		return nil, fmt.Errorf("get problem member: %w", err)
	}

	problemTeams, err := uc.problemsRepo.GetProblemTeams(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("get problem teams: %w", err)
	}
	if len(problemTeams) == 0 {
		return bestPermission, nil
	}

	userTeams, err := uc.teamsRepo.GetUserTeamsByOrganization(ctx, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("get user teams: %w", err)
	}

	userTeamIDs := make(map[uuid.UUID]struct{}, len(userTeams))
	for _, team := range userTeams {
		userTeamIDs[team.ID] = struct{}{}
	}

	for _, problemTeam := range problemTeams {
		if _, ok := userTeamIDs[problemTeam.TeamID]; ok {
			bestPermission = pickHigherProblemPermission(bestPermission, problemTeam.Permission)
		}
	}

	return bestPermission, nil
}

func resolvePersistedContestMask(mask *models.ContestPermissionMask, role models.ContestRole) models.ContestPermissionMask {
	if mask != nil {
		return *mask
	}

	fallbackMask, ok := models.ContestRoleDefaultPermissionMask(role)
	if !ok {
		return 0
	}

	return fallbackMask
}

func pickHigherContestRoleWithMask(
	currentRole *models.ContestRole,
	currentMask models.ContestPermissionMask,
	candidateRole models.ContestRole,
	candidateMask *models.ContestPermissionMask,
) (*models.ContestRole, models.ContestPermissionMask) {
	if !models.RoleGraterOrEquals(candidateRole, candidateRole) {
		return currentRole, currentMask
	}

	if currentRole == nil || models.RoleGraterOrEquals(candidateRole, *currentRole) {
		role := candidateRole
		return &role, resolvePersistedContestMask(candidateMask, candidateRole)
	}

	return currentRole, currentMask
}

func mapProblemRoleToPermission(role models.ProblemRole) (models.ProblemPermission, bool) {
	switch string(role) {
	case string(models.ProblemRoleOwner):
		return models.ProblemPermissionAdmin, true
	case string(models.ProblemRoleModerator):
		return models.ProblemPermissionWrite, true
	case string(models.ProblemRoleViewer):
		return models.ProblemPermissionRead, true
	default:
		return "", false
	}
}

func pickHigherProblemPermission(current *models.ProblemPermission, candidate models.ProblemPermission) *models.ProblemPermission {
	if problemPermissionRank(candidate) == 0 {
		return current
	}
	if current == nil || problemPermissionRank(candidate) >= problemPermissionRank(*current) {
		permission := candidate
		return &permission
	}
	return current
}

func problemPermissionRank(permission models.ProblemPermission) int {
	switch permission {
	case models.ProblemPermissionAdmin:
		return 3
	case models.ProblemPermissionWrite:
		return 2
	case models.ProblemPermissionRead:
		return 1
	default:
		return 0
	}
}

func (uc *PermissionsUseCase) hasProblemPermission(pc *problemContext, required models.ProblemPermission) bool {
	if pc.permission == nil {
		return false
	}
	return problemPermissionRank(*pc.permission) >= problemPermissionRank(required)
}
