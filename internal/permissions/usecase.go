package permissions

import (
	"context"

	"github.com/gate149/core/internal/models"
	"github.com/google/uuid"
)

type ContestsUC interface {
	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMember, error)
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
}

type UsersUC interface {
	GetUserById(ctx context.Context, id uuid.UUID) (*models.User, error)
}

type ProblemsUC interface {
	GetProblemMember(ctx context.Context, problemId uuid.UUID, userId uuid.UUID) (*models.ProblemMember, error)
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
}

type PermissionsUseCase struct {
	contestsUC ContestsUC
	usersUC    UsersUC
	problemsUC ProblemsUC
}

func NewUseCase(contestsUC ContestsUC, usersUC UsersUC, problemsUC ProblemsUC) *PermissionsUseCase {
	return &PermissionsUseCase{
		contestsUC: contestsUC,
		usersUC:    usersUC,
		problemsUC: problemsUC,
	}
}

func (uc *PermissionsUseCase) GetContestPermissions(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestPermissions, error) {
	contest, err := uc.contestsUC.GetContest(ctx, c.ContestId)
	if err != nil {
		return nil, err
	}

	member, err := uc.contestsUC.GetContestMember(ctx, c)
	if err != nil {
		return nil, err
	}

	user, err := uc.usersUC.GetUserById(ctx, c.UserId)
	if err != nil {
		return nil, err
	}

	permissions := &models.ContestPermissions{}

	if user.IsAdmin() || member.IsOwner() {
		permissions = &models.ContestPermissions{
			ViewContest:        true,
			EditContest:        true,
			AdminContest:       true,
			ViewMonitor:        true,
			ListAllSubmissions: true,
			ViewAnySubmission:  true,
		}
		return permissions, nil
	}

	permissions = &models.ContestPermissions{
		ViewContest:        true,
		EditContest:        models.RoleGraterOrEquals(member.Role, models.ContestRoleModerator),
		AdminContest:       false,
		ViewMonitor:        models.RoleGraterOrEquals(member.Role, contest.MonitorScope),
		ListAllSubmissions: models.RoleGraterOrEquals(member.Role, contest.SubmissionsListScope),
		ViewAnySubmission:  models.RoleGraterOrEquals(member.Role, contest.SubmissionsReviewScope),
	}

	return permissions, nil
}

func (uc *PermissionsUseCase) GetProblemPermissions(ctx context.Context, p *models.ProblemPermissionGet) (*models.ProblemPermissions, error) {
	problem, err := uc.problemsUC.GetProblemById(ctx, p.ProblemId)
	if err != nil {
		return nil, err
	}

	member, err := uc.problemsUC.GetProblemMember(ctx, p.ProblemId, p.UserId)
	if err != nil {
		return nil, err
	}

	user, err := uc.usersUC.GetUserById(ctx, p.UserId)
	if err != nil {
		return nil, err
	}

	permissions := &models.ProblemPermissions{}

	if user.IsAdmin() || member.IsOwner() {
		permissions = &models.ProblemPermissions{
			ViewProblem:  true,
			EditProblem:  true,
			AdminProblem: true,
		}
	}

	if member.IsModerator() {
		permissions = &models.ProblemPermissions{
			ViewProblem:  true,
			EditProblem:  true,
			AdminProblem: false,
		}
		return permissions, nil
	}

	if problem.Visibility == models.ProblemVisibilityPublic {
		permissions = &models.ProblemPermissions{
			ViewProblem:  true,
			EditProblem:  false,
			AdminProblem: false,
		}
		return permissions, nil
	}

	return permissions, nil
}
