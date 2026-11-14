package core

import (
	"context"

	paging "github.com/brawler2011/gate/backend/internal/page"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) ListProblems(ctx context.Context, params corev1.ListProblemsParams) (*corev1.ListProblemsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	searchStr := params.Search.Or("")
	descending := params.Descending.Or(true)

	filter := &problemsmodel.ProblemsFilter{
		ViewerID:   &user.Id,
		Page:       params.Page.Or(1),
		PageSize:   params.PageSize.Or(50),
		Search:     searchStr,
		Descending: &descending,
	}

	if params.Owner.IsSet() && params.Owner.Value {
		filter.OwnerID = &user.Id
	}

	if params.OrganizationID.IsSet() {
		orgID := params.OrganizationID.Value
		filter.OrganizationID = &orgID
	}

	if params.IsTemplate.IsSet() {
		isTmpl := params.IsTemplate.Value
		filter.IsTemplate = &isTmpl
	}

	problemsList, err := h.Problems.ListProblems(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := corev1.ListProblemsResponseModel{
		Problems:   make([]corev1.ProblemsListItemModel, len(problemsList.Problems)),
		Pagination: PaginationDTO(problemsList.Pagination),
	}

	for i, problem := range problemsList.Problems {
		resp.Problems[i] = ProblemsListItemDTO(problem)
	}
	return projectResponse(h, ctx, &resp)
}

func (h *CoreServer) ListProblemTemplates(ctx context.Context, params corev1.ListProblemTemplatesParams) ([]corev1.ProblemTemplateModel, error) {
	templates, err := h.Problems.ListTemplates(ctx, params.OrganizationID.Or(uuid.Nil))
	if err != nil {
		return nil, err
	}
	result := make([]corev1.ProblemTemplateModel, len(templates))
	for i, template := range templates {
		result[i] = corev1.ProblemTemplateModel{ID: template.ID, Title: template.Title, Description: template.Description, ProblemType: template.ProblemType, IsBuiltin: template.IsBuiltin}
	}
	return result, nil
}

func (h *CoreServer) CreateProblem(ctx context.Context, params corev1.CreateProblemParams) (*corev1.CreationResponseModel, error) {
	id, err := h.ProblemDrafts.CreateProblem(ctx, &problemsmodel.CreateProblemInput{OrganizationID: params.OrganizationID.Or(uuid.Nil), Title: params.Title, TemplateID: params.TemplateID, IdempotencyKey: params.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &corev1.CreationResponseModel{ID: id}, nil
}

func (h *CoreServer) DeleteProblem(ctx context.Context, params corev1.DeleteProblemParams) error {
	err := h.Problems.DeleteProblem(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetProblem(ctx context.Context, params corev1.GetProblemParams) (*corev1.GetProblemResponseModel, error) {
	details, err := h.Problems.GetProblemDetails(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	return projectResponse(h, ctx, &corev1.GetProblemResponseModel{Problem: *ProblemDTO(details.Problem, details.Statement, SamplesDTO(details.Samples))})

}

func (h *CoreServer) UpdateProblem(ctx context.Context, req *corev1.UpdateProblemRequestModel, params corev1.UpdateProblemParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	update := &problemsmodel.ProblemUpdate{}

	if req.Visibility.IsSet() {
		update.Visibility = (*string)(&req.Visibility.Value)
	}

	if req.IsTemplate.IsSet() {
		update.IsTemplate = &req.IsTemplate.Value
	}

	err := h.Problems.UpdateProblem(ctx, params.ID, update)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListProblemTeams(ctx context.Context, params corev1.ListProblemTeamsParams) (*corev1.ListProblemTeamsResponseModel, error) {
	teams, err := h.Problems.GetProblemTeams(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, listProblemTeamsDTO(teams))
}

func (h *CoreServer) CreateProblemTeam(ctx context.Context, params corev1.CreateProblemTeamParams) error {
	permission := problemsmodel.ProblemPermissionRead
	if params.Permission.IsSet() && params.Permission.Value != "" {
		permission = problemsmodel.ProblemPermission(params.Permission.Value)
	}

	err := h.Problems.AddProblemTeam(ctx, params.ID, params.TeamID, permission)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateProblemTeam(ctx context.Context, params corev1.UpdateProblemTeamParams) error {
	err := h.Problems.UpdateProblemTeamPermission(ctx, params.ID, params.TeamID, problemsmodel.ProblemPermission(params.Permission))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteProblemTeam(ctx context.Context, params corev1.DeleteProblemTeamParams) error {
	err := h.Problems.RemoveProblemTeam(ctx, params.ID, params.TeamID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListProblemMembers(ctx context.Context, params corev1.ListProblemMembersParams) (*corev1.ListProblemMembersResponseModel, error) {
	members, err := h.Problems.ListProblemMembers(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}

	return projectResponse(h, ctx, listProblemMembersDTO(members.Items, members.Pagination))
}

func (h *CoreServer) CreateProblemMember(ctx context.Context, params corev1.CreateProblemMemberParams) error {
	role := problemsmodel.ProblemRoleViewer
	if params.Role.IsSet() && params.Role.Value != "" {
		role = problemsmodel.ProblemRole(params.Role.Value)
	}

	err := h.Problems.CreateProblemMember(ctx, params.ID, params.UserID, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateProblemMember(ctx context.Context, params corev1.UpdateProblemMemberParams) error {
	err := h.Problems.UpdateProblemMemberRole(ctx, params.ID, params.UserID, problemsmodel.ProblemRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteProblemMember(ctx context.Context, params corev1.DeleteProblemMemberParams) error {
	err := h.Problems.RemoveProblemMember(ctx, params.ID, params.UserID)
	if err != nil {
		return err
	}

	return nil
}
