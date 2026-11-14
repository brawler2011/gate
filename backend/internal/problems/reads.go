package problems

import (
	"context"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Template struct {
	ID, Title, Description, ProblemType string
	IsBuiltin                           bool
}

func (uc *ProblemsUseCase) ListProblems(ctx context.Context, filter *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error) {
	if filter == nil {
		return nil, pkg.ErrBadInput
	}
	if err := access.CheckReadActor(ctx); err != nil {
		return nil, err
	}
	copy := *filter
	var err error
	copy.Page, copy.PageSize, err = paging.Normalize(copy.Page, copy.PageSize, 50)
	if err != nil {
		return nil, err
	}
	user := accessmodel.UserFromContext(ctx)
	copy.ViewerID = &user.Id
	return uc.repo.ListProblems(ctx, &copy)
}

func (uc *ProblemsUseCase) authorizeView(ctx context.Context, id uuid.UUID) error {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}
	allowed, err := uc.permissions.HasProblemPermission(ctx, id, user.Id, accessmodel.ActionViewProblem)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.NoPermission
	}
	return nil
}

func (uc *ProblemsUseCase) ListProblemMembers(ctx context.Context, id uuid.UUID, request paging.Request) (*paging.List[problemsmodel.ProblemMember], error) {
	if err := uc.authorizeView(ctx, id); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, total, err := uc.repo.ListProblemMembers(ctx, id, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[problemsmodel.ProblemMember]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, total)}, nil
}

func (uc *ProblemsUseCase) GetProblemTeams(ctx context.Context, id uuid.UUID) ([]problemsmodel.ProblemTeam, error) {
	if err := uc.authorizeView(ctx, id); err != nil {
		return nil, err
	}
	return uc.repo.GetProblemTeams(ctx, id)
}

func (uc *ProblemsUseCase) ListDashboardProblems(ctx context.Context, user uuid.UUID, limit int32) ([]problemsmodel.DashboardProblem, error) {
	if err := accessmodel.RequireSelfOrAdmin(ctx, user); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, pkg.ErrBadInput
	}
	return uc.repo.ListDashboardProblems(ctx, user, limit)
}

func (uc *ProblemsUseCase) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]Template, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}
	if orgID == uuid.Nil {
		organizations, err := uc.organizations.GetUserOrganizations(ctx, user.Id)
		if err != nil {
			return nil, err
		}
		if len(organizations) > 0 {
			orgID = organizations[0].ID
		}
	}
	result := make([]Template, 0)
	for _, b := range templates.ListBuiltinTemplates() {
		result = append(result, Template{ID: b.ID, Title: b.Title, Description: b.Description, ProblemType: b.ProblemType, IsBuiltin: true})
	}
	if orgID == uuid.Nil {
		return result, nil
	}
	isTemplate := true
	list, err := uc.ListProblems(ctx, &problemsmodel.ProblemsFilter{Page: 1, PageSize: 100, OrganizationID: &orgID, IsTemplate: &isTemplate})
	if err != nil {
		return nil, err
	}
	for _, p := range list.Problems {
		template := Template{ID: p.ID.String(), Title: p.Title, Description: "Пользовательский шаблон организации", ProblemType: "pass-fail"}
		manifest, err := uc.drafts.GetDraft(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		{
			if manifest.Type != "" {
				template.ProblemType = manifest.Type
			}
			if statementFromDraft(manifest, "").Legend != "" {
				template.Description = statementFromDraft(manifest, "").Legend
			}
		}
		result = append(result, template)
	}
	return result, nil
}
