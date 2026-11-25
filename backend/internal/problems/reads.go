package problems

import (
	"context"
	"errors"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type Template struct {
	ID, Title, Description, ProblemType, Grading string
	IsBuiltin                           bool
}

func (uc *ProblemsUseCase) ListProblems(ctx context.Context, filter *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error) {
	if filter == nil {
		return nil, pkg.ErrInvalid
	}
	copy := *filter
	var err error
	copy.Page, copy.PageSize, err = paging.Normalize(copy.Page, copy.PageSize, 50)
	if err != nil {
		return nil, err
	}
	return uc.repo.ListProblems(ctx, &copy)
}

func (uc *ProblemsUseCase) ListProblemMembers(ctx context.Context, id uuid.UUID, request paging.Request) (*paging.List[problemsmodel.ProblemMember], error) {
	if _, err := access.Problem(ctx, uc.access, id, accessmodel.ProblemManage); err != nil {
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

func (uc *ProblemsUseCase) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]Template, error) {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return nil, err
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
		result = append(result, Template{ID: b.ID, Title: b.Title, Description: b.Description, ProblemType: b.ProblemType, Grading: b.Grading, IsBuiltin: true})
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
		template := Template{ID: p.ID.String(), Title: p.Title, Description: "Пользовательский шаблон организации", ProblemType: "standard", Grading: "pass-fail"}
		// New problems copy the template's latest package, so the list shows that package too.
		ready, err := uc.templates.GetReadyPackage(ctx, p.ID)
		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			return nil, err
		}
		if err == nil {
			template.Title, template.ProblemType, template.Grading = ready.Title, ready.Type, ready.Grading
			if s := pickStatement(ready.Statements, ""); s != nil && s.Legend != "" {
				template.Description = s.Legend
			}
		}
		result = append(result, template)
	}
	return result, nil
}
