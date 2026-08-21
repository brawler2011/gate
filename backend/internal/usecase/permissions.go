package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg"
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
	user          *models.User
	contest       *models.Contest
	contestRole   *models.ContestRole
	isPublic      bool
	isStarted     bool
	isFinished    bool
	isOwner       bool
	isModerator   bool
	isParticipant bool
	isAdmin       bool
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
	var user models.User
	var err error
	if userID != uuid.Nil {
		user, err = uc.usersUC.GetUserById(ctx, userID)
		if err != nil {
			return false, fmt.Errorf("get user: %w", err)
		}
	} else {
		user = models.Guest
	}

	contest, err := uc.contestsRepo.GetContest(ctx, contestID)
	if err != nil {
		return false, fmt.Errorf("get contest: %w", err)
	}

	contestRole, err := uc.resolveEffectiveContestRole(ctx, &contest, userID)
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

	isAdmin := user.Role == models.UserRoleAdmin
	isOwner := isAdmin || (contest.OwnerID != nil && *contest.OwnerID == userID) || (contestRole != nil && *contestRole == models.ContestRoleOwner)
	isModerator := isOwner || (contestRole != nil && *contestRole == models.ContestRoleModerator)
	isParticipant := isModerator || (contestRole != nil && *contestRole == models.ContestRoleParticipant)

	cc := &contestContext{
		user:          &user,
		contest:       &contest,
		contestRole:   contestRole,
		isPublic:      contest.Visibility == models.ContestVisibilityPublic,
		isStarted:     isStarted,
		isFinished:    isFinished,
		isOwner:       isOwner,
		isModerator:   isModerator,
		isParticipant: isParticipant,
		isAdmin:       isAdmin,
	}

	switch action {
	case models.ActionGetContest:
		return uc.canViewContest(cc), nil
	case models.ActionGetContestProblem:
		return uc.canViewContestProblem(cc), nil
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
	case models.ActionGetSubmissionDetails:
		return uc.canViewSubmissionDetails(cc), nil
	case models.ActionCreateSubmission:
		return uc.canSubmit(cc), nil
	default:
		return false, fmt.Errorf("unknown contest action: %s", action)
	}
}

func (uc *PermissionsUseCase) canViewContest(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}
	if cc.isPublic {
		return true
	}
	return cc.isParticipant
}

func (uc *PermissionsUseCase) canViewContestProblem(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}
	if !cc.isStarted {
		return false
	}
	if cc.isPublic {
		return true
	}
	return cc.isParticipant
}

func (uc *PermissionsUseCase) canAdminContest(cc *contestContext) bool {
	return cc.isOwner
}

func (uc *PermissionsUseCase) canManageContest(cc *contestContext) bool {
	return cc.isModerator
}

func (uc *PermissionsUseCase) canViewMonitor(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}
	if !cc.isStarted {
		return false
	}

	settings := cc.contest.TypedSettings()
	scope := settings.MonitorScope
	if scope == "" {
		scope = "participant"
	}

	switch scope {
	case "public":
		return uc.canViewContest(cc)
	case "participant":
		return cc.isParticipant
	case "moderator":
		return cc.isModerator
	default:
		return cc.isParticipant
	}
}

func (uc *PermissionsUseCase) canListAllSubmissions(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}

	settings := cc.contest.TypedSettings()
	scope := settings.SubmissionsListScope
	if scope == "" {
		scope = "moderator"
	}

	switch scope {
	case "public":
		return uc.canViewContest(cc) && cc.isStarted
	case "participant":
		return cc.isParticipant && cc.isStarted
	case "moderator":
		return cc.isModerator
	default:
		return cc.isModerator
	}
}

func (uc *PermissionsUseCase) canListOwnSubmissions(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}
	if cc.user.IsGuest() || cc.user.Id == uuid.Nil {
		return false
	}
	return cc.isParticipant
}

func (uc *PermissionsUseCase) canGetOwnSubmission(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}
	if cc.user.IsGuest() || cc.user.Id == uuid.Nil {
		return false
	}
	return cc.isParticipant
}

func (uc *PermissionsUseCase) canViewOtherSubmission(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}

	settings := cc.contest.TypedSettings()
	scope := settings.SubmissionsReviewScope
	if scope == "" {
		scope = "moderator"
	}

	switch scope {
	case "public":
		return uc.canViewContest(cc) && cc.isStarted
	case "participant":
		return cc.isParticipant && cc.isStarted
	case "moderator":
		return cc.isModerator
	default:
		return cc.isModerator
	}
}

func (uc *PermissionsUseCase) canViewSubmissionDetails(cc *contestContext) bool {
	if cc.isModerator {
		return true
	}

	settings := cc.contest.TypedSettings()
	scope := settings.SubmissionDetailsScope
	if scope == "" {
		scope = "moderator"
	}

	switch scope {
	case "public":
		return uc.canViewContest(cc) && cc.isStarted
	case "participant":
		return cc.isParticipant && cc.isStarted
	case "moderator":
		return cc.isModerator
	default:
		return cc.isModerator
	}
}

func (uc *PermissionsUseCase) canSubmit(cc *contestContext) bool {
	// Moderator/Owner can always submit for testing
	if cc.isModerator {
		return true
	}

	// Guests cannot submit
	if cc.user.IsGuest() || cc.user.Id == uuid.Nil {
		return false
	}

	// Before start: regular participants cannot submit
	if !cc.isStarted {
		return false
	}

	// Running contest: participants can submit
	if !cc.isFinished {
		return cc.isParticipant
	}

	// Finished contest: upsolving mode (Codeforces style)
	settings := cc.contest.TypedSettings()
	if settings.GetEnableUpsolving() {
		return uc.canViewContest(cc)
	}

	return false
}

// HasProblemPermission проверяет право на действие с задачей
func (uc *PermissionsUseCase) HasProblemPermission(
	ctx context.Context,
	problemID uuid.UUID,
	userID uuid.UUID,
	action models.ProblemAction,
) (bool, error) {
	var user models.User
	var err error
	if userID != uuid.Nil {
		user, err = uc.usersUC.GetUserById(ctx, userID)
		if err != nil {
			return false, fmt.Errorf("get user: %w", err)
		}
	} else {
		user = models.Guest
	}

	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return false, fmt.Errorf("get problem: %w", err)
	}

	permission, err := uc.resolveProblemPermission(ctx, &problem, userID)
	if err != nil {
		return false, err
	}

	isAdmin := user.Role == models.UserRoleAdmin
	isOwner := isAdmin || (problem.OwnerID != nil && *problem.OwnerID == userID) || (permission != nil && *permission == models.ProblemPermissionAdmin)

	pc := &problemContext{
		user:       &user,
		problem:    &problem,
		permission: permission,
		isPublic:   problem.Visibility == models.ProblemVisibilityPublic,
		isOwner:    isOwner,
		isAdmin:    isAdmin,
	}

	switch action {
	case models.ActionViewProblem:
		return uc.canViewProblem(pc), nil
	case models.ActionEditProblem:
		return uc.canEditProblem(pc), nil
	case models.ActionAdminProblem, models.ActionDeleteProblem:
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

	role, err := uc.resolveEffectiveContestRole(ctx, &contest, userID)
	if err != nil {
		return nil, 0, err
	}

	var mask models.ContestPermissionMask
	if role != nil {
		if defaultMask, ok := models.ContestRoleDefaultPermissionMask(*role); ok {
			mask = defaultMask
		}
	}

	return role, mask, nil
}

func (uc *PermissionsUseCase) GetContestPermissions(
	ctx context.Context,
	contestID uuid.UUID,
	userID uuid.UUID,
) (*models.ContestPermissions, error) {
	var user models.User
	var err error
	if userID != uuid.Nil {
		user, err = uc.usersUC.GetUserById(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
	} else {
		user = models.Guest
	}

	contest, err := uc.contestsRepo.GetContest(ctx, contestID)
	if err != nil {
		return nil, fmt.Errorf("get contest: %w", err)
	}

	contestRole, err := uc.resolveEffectiveContestRole(ctx, &contest, userID)
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

	isAdmin := user.Role == models.UserRoleAdmin
	isOwner := isAdmin || (contest.OwnerID != nil && *contest.OwnerID == userID) || (contestRole != nil && *contestRole == models.ContestRoleOwner)
	isModerator := isOwner || (contestRole != nil && *contestRole == models.ContestRoleModerator)
	isParticipant := isModerator || (contestRole != nil && *contestRole == models.ContestRoleParticipant)

	cc := &contestContext{
		user:          &user,
		contest:       &contest,
		contestRole:   contestRole,
		isPublic:      contest.Visibility == models.ContestVisibilityPublic,
		isStarted:     isStarted,
		isFinished:    isFinished,
		isOwner:       isOwner,
		isModerator:   isModerator,
		isParticipant: isParticipant,
		isAdmin:       isAdmin,
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
		GetSubmissionDetails:   uc.canViewSubmissionDetails(cc),
		CreateSubmission:       uc.canSubmit(cc),
	}, nil
}

func (uc *PermissionsUseCase) GetProblemPermissions(
	ctx context.Context,
	problemID uuid.UUID,
	userID uuid.UUID,
) (*models.ProblemPermissions, error) {
	var user models.User
	var err error
	if userID != uuid.Nil {
		user, err = uc.usersUC.GetUserById(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
	} else {
		user = models.Guest
	}

	problem, err := uc.problemsRepo.GetProblemById(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("get problem: %w", err)
	}

	permission, err := uc.resolveProblemPermission(ctx, &problem, userID)
	if err != nil {
		return nil, err
	}

	isAdmin := user.Role == models.UserRoleAdmin
	isOwner := isAdmin || (problem.OwnerID != nil && *problem.OwnerID == userID) || (permission != nil && *permission == models.ProblemPermissionAdmin)

	pc := &problemContext{
		user:       &user,
		problem:    &problem,
		permission: permission,
		isPublic:   problem.Visibility == models.ProblemVisibilityPublic,
		isOwner:    isOwner,
		isAdmin:    isAdmin,
	}

	return &models.ProblemPermissions{
		ViewProblem:  uc.canViewProblem(pc),
		EditProblem:  uc.canEditProblem(pc),
		AdminProblem: uc.canAdminProblem(pc),
	}, nil
}

func (uc *PermissionsUseCase) HasOrganizationPermission(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
	action models.OrgAction,
) (bool, error) {
	if userID == uuid.Nil {
		if action == models.ActionViewOrganization {
			return true, nil
		}
		return false, nil
	}

	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get user: %w", err)
	}

	// Global admins have full access
	if user.Role == models.UserRoleAdmin {
		return true, nil
	}

	member, err := uc.orgsRepo.GetMember(ctx, orgID, userID)
	if err != nil {
		if errors.Is(err, pkg.ErrNotFound) {
			// User is not a member of the organization
			if action == models.ActionViewOrganization {
				return true, nil
			}
			return false, nil
		}
		return false, fmt.Errorf("get organization member: %w", err)
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

// RBAC Subject Resolution for Contests
func (uc *PermissionsUseCase) resolveEffectiveContestRole(
	ctx context.Context,
	contest *models.Contest,
	userID uuid.UUID,
) (*models.ContestRole, error) {
	if userID == uuid.Nil {
		return nil, nil
	}

	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// 1. Global Admin gets RoleOwner
	if user.Role == models.UserRoleAdmin {
		ownerRole := models.ContestRoleOwner
		return &ownerRole, nil
	}

	// 2. Contest Owner gets RoleOwner
	if contest.OwnerID != nil && *contest.OwnerID == userID {
		ownerRole := models.ContestRoleOwner
		return &ownerRole, nil
	}

	// 3. Organization Owner / Admin gets RoleOwner on all org contests
	if uc.orgsRepo != nil && contest.OrganizationID != uuid.Nil {
		orgMember, err := uc.orgsRepo.GetMember(ctx, contest.OrganizationID, userID)
		if err == nil {
			if orgMember.Role == models.OrgRoleOwner || orgMember.Role == models.OrgRoleAdmin {
				ownerRole := models.ContestRoleOwner
				return &ownerRole, nil
			}
		} else if !errors.Is(err, pkg.ErrNotFound) {
			return nil, fmt.Errorf("get org member: %w", err)
		}
	}

	var bestRole *models.ContestRole

	// 4. Direct Contest Member
	if uc.contestsRepo != nil {
		directMember, err := uc.contestsRepo.GetContestMember(ctx, &models.ContestPermissionGet{
			ContestId: contest.ID,
			UserId:    userID,
		})
		if err == nil {
			bestRole = pickHigherContestRole(bestRole, directMember.ContestRole)
		} else if !errors.Is(err, pkg.ErrNotFound) {
			return nil, fmt.Errorf("get contest member: %w", err)
		}

		// 5. Team Contest Members
		contestTeams, err := uc.contestsRepo.GetContestTeams(ctx, contest.ID)
		if err != nil {
			return nil, fmt.Errorf("get contest teams: %w", err)
		}
		if len(contestTeams) > 0 && uc.teamsRepo != nil {
			userTeams, err := uc.teamsRepo.GetUserTeamsByOrganization(ctx, userID, contest.OrganizationID)
			if err != nil {
				return nil, fmt.Errorf("get user teams: %w", err)
			}

			userTeamIDs := make(map[uuid.UUID]struct{}, len(userTeams))
			for _, team := range userTeams {
				userTeamIDs[team.ID] = struct{}{}
			}

			for _, contestTeam := range contestTeams {
				if _, ok := userTeamIDs[contestTeam.TeamID]; ok {
					bestRole = pickHigherContestRole(bestRole, contestTeam.Role)
				}
			}
		}
	}

	// 6. Public Contest with open participation gives RoleParticipant to any authenticated user
	if bestRole == nil && contest.Visibility == models.ContestVisibilityPublic {
		settings := contest.TypedSettings()
		if settings.GetParticipationMode() == models.ParticipationModeOpen {
			partRole := models.ContestRoleParticipant
			bestRole = &partRole
		}
	}

	return bestRole, nil
}

// RBAC Subject Resolution for Problems
func (uc *PermissionsUseCase) resolveProblemPermission(
	ctx context.Context,
	problem *models.Problem,
	userID uuid.UUID,
) (*models.ProblemPermission, error) {
	if userID == uuid.Nil {
		return nil, nil
	}

	user, err := uc.usersUC.GetUserById(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// 1. Global Admin gets ProblemPermissionAdmin
	if user.Role == models.UserRoleAdmin {
		adminPerm := models.ProblemPermissionAdmin
		return &adminPerm, nil
	}

	// 2. Problem Owner gets ProblemPermissionAdmin
	if problem.OwnerID != nil && *problem.OwnerID == userID {
		adminPerm := models.ProblemPermissionAdmin
		return &adminPerm, nil
	}

	// 3. Org Owner / Admin gets ProblemPermissionAdmin on all org problems
	if uc.orgsRepo != nil && problem.OrganizationID != uuid.Nil {
		orgMember, err := uc.orgsRepo.GetMember(ctx, problem.OrganizationID, userID)
		if err == nil {
			if orgMember.Role == models.OrgRoleOwner || orgMember.Role == models.OrgRoleAdmin {
				adminPerm := models.ProblemPermissionAdmin
				return &adminPerm, nil
			}
		} else if !errors.Is(err, pkg.ErrNotFound) {
			return nil, fmt.Errorf("get org member: %w", err)
		}
	}

	var bestPermission *models.ProblemPermission

	// 4. Direct Problem Member
	if uc.problemsRepo != nil {
		directMember, err := uc.problemsRepo.GetProblemMember(ctx, problem.ID, userID)
		if err == nil {
			if mappedPermission, ok := mapProblemRoleToPermission(directMember.Role); ok {
				bestPermission = pickHigherProblemPermission(bestPermission, mappedPermission)
			}
		} else if !errors.Is(err, pkg.ErrNotFound) {
			return nil, fmt.Errorf("get problem member: %w", err)
		}

		// 5. Team Problem Members
		problemTeams, err := uc.problemsRepo.GetProblemTeams(ctx, problem.ID)
		if err != nil {
			return nil, fmt.Errorf("get problem teams: %w", err)
		}
		if len(problemTeams) > 0 && uc.teamsRepo != nil {
			userTeams, err := uc.teamsRepo.GetUserTeamsByOrganization(ctx, userID, problem.OrganizationID)
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
		}
	}

	return bestPermission, nil
}

func pickHigherContestRole(
	currentRole *models.ContestRole,
	candidateRole models.ContestRole,
) *models.ContestRole {
	if !models.IsValidContestRole(candidateRole) {
		return currentRole
	}

	if currentRole == nil {
		role := candidateRole
		return &role
	}

	if models.RoleGraterOrEquals(candidateRole, *currentRole) {
		role := candidateRole
		return &role
	}

	return currentRole
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
