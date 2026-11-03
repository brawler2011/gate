package core

import (
	"context"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type ProblemsReader interface {
	problems.Reads
	ListTemplates(context.Context, uuid.UUID) ([]problems.Template, error)
}

func (h *CoreServer) ListProblems(ctx context.Context, params corev1.ListProblemsParams) (*corev1.ListProblemsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)
	searchStr := params.Search.Or("")

	filter := &problemsmodel.ProblemsFilter{
		ViewerID: &user.Id,
		Page:     params.Page.Or(1),
		PageSize: params.PageSize.Or(50),
		Search:   searchStr,
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

	problemsList, err := h.problemsReader.ListProblems(ctx, filter)
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
	return &resp, nil
}

func (h *CoreServer) ListProblemTemplates(ctx context.Context, params corev1.ListProblemTemplatesParams) ([]corev1.ProblemTemplateModel, error) {
	templates, err := h.problemsReader.ListTemplates(ctx, params.OrganizationID.Or(uuid.Nil))
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
	id, err := h.problemsUC.CreateProblem(ctx, &problemsmodel.CreateProblemInput{OrganizationID: params.OrganizationID.Or(uuid.Nil), Title: params.Title, TemplateID: params.TemplateID})
	if err != nil {
		return nil, err
	}
	return &corev1.CreationResponseModel{ID: id}, nil
}

func (h *CoreServer) DeleteProblem(ctx context.Context, params corev1.DeleteProblemParams) error {
	err := h.problemsUC.DeleteProblem(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetProblem(ctx context.Context, params corev1.GetProblemParams) (*corev1.GetProblemResponseModel, error) {
	details, err := h.problemsUC.GetProblemDetails(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	return &corev1.GetProblemResponseModel{Problem: *ProblemDTO(details.Problem, details.Statement, SamplesDTO(details.Samples))}, nil

}

func (h *CoreServer) UpdateProblem(ctx context.Context, req *corev1.UpdateProblemRequestModel, params corev1.UpdateProblemParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	// Build update params
	update := &problemsmodel.ProblemUpdate{}

	// Handle title update
	if req.Title.IsSet() {
		update.Title = &req.Title.Value
	}

	// Handle visibility update
	if req.Visibility.IsSet() {
		update.Visibility = (*string)(&req.Visibility.Value)
	}

	// Handle is_template update
	if req.IsTemplate.IsSet() {
		update.IsTemplate = &req.IsTemplate.Value
	}

	// Note: Other fields (Legend, InputFormat, etc.) are now stored in git repos
	// and managed through the workshop/publish workflow

	err := h.problemsUC.UpdateProblem(ctx, params.ID, update)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListProblemTeams(ctx context.Context, params corev1.ListProblemTeamsParams) (*corev1.ListProblemTeamsResponseModel, error) {
	teams, err := h.problemsReader.GetProblemTeams(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	return listProblemTeamsDTO(teams), nil
}

func (h *CoreServer) CreateProblemTeam(ctx context.Context, params corev1.CreateProblemTeamParams) error {
	user := accessmodel.UserFromContext(ctx)

	permission := problemsmodel.ProblemPermissionRead
	if params.Permission.IsSet() && params.Permission.Value != "" {
		permission = problemsmodel.ProblemPermission(params.Permission.Value)
	}

	err := h.problemsUC.AddProblemTeam(ctx, params.ID, params.TeamID, user.Id, permission)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateProblemTeam(ctx context.Context, params corev1.UpdateProblemTeamParams) error {
	user := accessmodel.UserFromContext(ctx)

	err := h.problemsUC.UpdateProblemTeamPermission(ctx, params.ID, params.TeamID, user.Id, problemsmodel.ProblemPermission(params.Permission))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteProblemTeam(ctx context.Context, params corev1.DeleteProblemTeamParams) error {
	user := accessmodel.UserFromContext(ctx)

	err := h.problemsUC.RemoveProblemTeam(ctx, params.ID, params.TeamID, user.Id)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListProblemMembers(ctx context.Context, params corev1.ListProblemMembersParams) (*corev1.ListProblemMembersResponseModel, error) {
	members, err := h.problemsReader.ListProblemMembers(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	total := safeInt32(len(members))
	page := params.Page.Or(1)
	return listProblemMembersDTO(members, page, total), nil
}

func (h *CoreServer) CreateProblemMember(ctx context.Context, params corev1.CreateProblemMemberParams) error {
	user := accessmodel.UserFromContext(ctx)

	role := problemsmodel.ProblemRoleViewer
	if params.Role.IsSet() && params.Role.Value != "" {
		role = problemsmodel.ProblemRole(params.Role.Value)
	}

	err := h.problemsUC.CreateProblemMember(ctx, params.ID, params.UserID, user.Id, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateProblemMember(ctx context.Context, params corev1.UpdateProblemMemberParams) error {
	user := accessmodel.UserFromContext(ctx)

	err := h.problemsUC.UpdateProblemMemberRole(ctx, params.ID, params.UserID, user.Id, problemsmodel.ProblemRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteProblemMember(ctx context.Context, params corev1.DeleteProblemMemberParams) error {
	user := accessmodel.UserFromContext(ctx)

	err := h.problemsUC.RemoveProblemMember(ctx, params.ID, params.UserID, user.Id)
	if err != nil {
		return err
	}

	return nil
}
