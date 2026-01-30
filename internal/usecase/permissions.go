package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/gate149/core/internal/domain/interfaces"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

type PermissionsUseCase struct {
	contestsUC interfaces.ContestsUC
	usersUC    interfaces.UsersUC
	problemsUC interfaces.ProblemsUC
}

func NewPermissionsUseCase(
	contestsUC interfaces.ContestsUC,
	usersUC interfaces.UsersUC,
	problemsUC interfaces.ProblemsUC,
) *PermissionsUseCase {
	return &PermissionsUseCase{
		contestsUC: contestsUC,
		usersUC:    usersUC,
		problemsUC: problemsUC,
	}
}

type contestContext struct {
	user       *models.User
	contest    *models.Contest
	member     *models.ContestMember
	isPublic   bool
	isStarted  bool
	isFinished bool
	isOwner    bool
	isAdmin    bool
}

type problemContext struct {
	user     *models.User
	problem  *models.Problem
	member   *models.ProblemMember
	isPublic bool
	isOwner  bool
	isAdmin  bool
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

	contest, err := uc.contestsUC.GetContest(ctx, contestID)
	if err != nil {
		return false, fmt.Errorf("get contest: %w", err)
	}

	member, _ := uc.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	})

	isStarted := true
	if contest.StartTime != nil {
		isStarted = contest.StartTime.Before(time.Now())
	}

	isFinished := false
	if contest.EndTime != nil {
		isFinished = contest.EndTime.Before(time.Now())
	}

	cc := &contestContext{
		user:       &user,
		contest:    &contest,
		member:     &member,
		isPublic:   contest.Visibility == models.ContestVisibilityPublic,
		isStarted:  isStarted,
		isFinished: isFinished,
		isOwner:    contest.OwnerID != nil && *contest.OwnerID == userID,
		isAdmin:    user.Role == models.UserRoleAdmin,
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
	case models.ActionListOwnSubmissions, models.ActionGetOwnSubmission:
		return uc.canViewOwnSubmissions(cc), nil
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
	return cc.isPublic || cc.member != nil
}

func (uc *PermissionsUseCase) canAdminContest(cc *contestContext) bool {
	return cc.isAdmin || cc.isOwner
}

func (uc *PermissionsUseCase) canManageContest(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	return cc.member != nil && cc.member.Role == models.ContestRoleModerator
}

func (uc *PermissionsUseCase) canViewMonitor(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	return cc.member != nil && models.RoleGraterOrEquals(cc.member.ContestRole, cc.contest.MonitorScope())
}

func (uc *PermissionsUseCase) canListAllSubmissions(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	return cc.member != nil && models.RoleGraterOrEquals(cc.member.ContestRole, cc.contest.SubmissionsListScope())
}

func (uc *PermissionsUseCase) canViewOwnSubmissions(cc *contestContext) bool {
	return cc.isAdmin || cc.isOwner || cc.member != nil
}

func (uc *PermissionsUseCase) canViewOtherSubmission(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	return cc.member != nil && models.RoleGraterOrEquals(cc.member.ContestRole, cc.contest.SubmissionsListScope())
}

func (uc *PermissionsUseCase) canSubmit(cc *contestContext) bool {
	if cc.isAdmin || cc.isOwner {
		return true
	}
	return cc.member != nil && cc.isStarted && !cc.isFinished
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

	problem, err := uc.problemsUC.GetProblemById(ctx, problemID)
	if err != nil {
		return false, fmt.Errorf("get problem: %w", err)
	}

	member, _ := uc.problemsUC.GetProblemMember(ctx, problemID, userID)

	pc := &problemContext{
		user:     &user,
		problem:  &problem,
		member:   &member,
		isPublic: problem.Visibility == models.ProblemVisibilityPublic,
		isOwner:  problem.OwnerID != nil && *problem.OwnerID == userID,
		isAdmin:  user.Role == models.UserRoleAdmin,
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
	return pc.isPublic || pc.member != nil
}

func (uc *PermissionsUseCase) canEditProblem(pc *problemContext) bool {
	return pc.isAdmin || pc.isOwner
}

func (uc *PermissionsUseCase) canAdminProblem(pc *problemContext) bool {
	return pc.isAdmin || pc.isOwner
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

	contest, err := uc.contestsUC.GetContest(ctx, contestID)
	if err != nil {
		return nil, fmt.Errorf("get contest: %w", err)
	}

	member, _ := uc.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestID,
		UserId:    userID,
	})

	isStarted := true
	if contest.StartTime != nil {
		isStarted = contest.StartTime.Before(time.Now())
	}

	isFinished := false
	if contest.EndTime != nil {
		isFinished = contest.EndTime.Before(time.Now())
	}

	cc := &contestContext{
		user:       &user,
		contest:    &contest,
		member:     &member,
		isPublic:   contest.Visibility == models.ContestVisibilityPublic,
		isStarted:  isStarted,
		isFinished: isFinished,
		isOwner:    contest.OwnerID != nil && *contest.OwnerID == userID,
		isAdmin:    user.Role == models.UserRoleAdmin,
	}

	return &models.ContestPermissions{
		GetContest:             uc.canViewContest(cc),
		UpdateContest:          uc.canAdminContest(cc),
		ManageContest:          uc.canManageContest(cc),
		AdminContest:           uc.canAdminContest(cc),
		GetMonitor:             uc.canViewMonitor(cc),
		ListUsersSubmissions:   uc.canListAllSubmissions(cc),
		ListOwnSubmissions:     uc.canViewOwnSubmissions(cc),
		GetOtherUserSubmission: uc.canViewOtherSubmission(cc),
		GetOwnSubmission:       uc.canViewOwnSubmissions(cc),
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

	problem, err := uc.problemsUC.GetProblemById(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("get problem: %w", err)
	}

	member, _ := uc.problemsUC.GetProblemMember(ctx, problemID, userID)

	pc := &problemContext{
		user:     &user,
		problem:  &problem,
		member:   &member,
		isPublic: problem.Visibility == models.ProblemVisibilityPublic,
		isOwner:  problem.OwnerID != nil && *problem.OwnerID == userID,
		isAdmin:  user.Role == models.UserRoleAdmin,
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

	// TODO: Check organization membership and roles
	// For now, allow all authenticated users to view organizations
	if action == models.ActionViewOrganization {
		return true, nil
	}

	// For other actions, deny by default (should check membership)
	return false, nil
}
