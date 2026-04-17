package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) ListProblems(ctx context.Context, request corev1.ListProblemsRequestObject) (corev1.ListProblemsResponseObject, error) {
	searchStr := ""
	if request.Params.Search != nil {
		searchStr = *request.Params.Search
	}

	filter := &models.ProblemsFilter{
		Page:     request.Params.Page,
		PageSize: request.Params.PageSize,
		Search:   searchStr,
	}

	if request.Params.PageSize < minPageSize || request.Params.PageSize > maxPageSize {
		return nil, badPageSize
	}

	if request.Params.Page < minPage {
		return nil, badPage
	}

	if request.Params.Search != nil {
		if !isLengthBetween(*request.Params.Search, 0, maxSearchLength) {
			return nil, badSearch
		}
		filter.Search = *request.Params.Search
	}

	if request.Params.Owner != nil {
		user := middleware.GetUser(ctx)

		filter.OwnerID = &user.Id
	}

	if request.Params.OrganizationId != nil {
		orgID, err := uuid.Parse(request.Params.OrganizationId.String())
		if err == nil {
			filter.OrganizationID = &orgID
		}
	}

	problemsList, err := h.problemsUC.ListProblems(ctx, filter)
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
	return corev1.ListProblems200JSONResponse(resp), nil
}

func (h *CoreServer) CreateProblem(ctx context.Context, request corev1.CreateProblemRequestObject) (corev1.CreateProblemResponseObject, error) {
	user := middleware.GetUser(ctx)

	if request.Params.Title == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	var orgID uuid.UUID
	if request.Params.OrganizationId != nil {
		orgID = uuid.UUID(*request.Params.OrganizationId)
	} else {
		orgs, err := h.organizationsUC.GetUserOrganizations(ctx, user.Id)
		if err != nil {
			return nil, err
		}
		if len(orgs) == 0 {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "user has no organizations")
		}
		orgID = orgs[0].ID
	}

	shortName := "problem-" + uuid.New().String()[:8]

	input := &models.CreateProblemInput{
		OrganizationID: orgID,
		OwnerID:        &user.Id,
		Title:          request.Params.Title,
		ShortName:      shortName,
		Visibility:     models.ProblemVisibilityPrivate,
	}

	problemID, err := h.problemsUC.CreateProblem(ctx, input)
	if err != nil {
		return nil, err
	}

	return corev1.CreateProblem200JSONResponse{Id: problemID}, nil
}

func (h *CoreServer) DeleteProblem(ctx context.Context, request corev1.DeleteProblemRequestObject) (corev1.DeleteProblemResponseObject, error) {
	err := h.problemsUC.DeleteProblem(ctx, request.Id)
	if err != nil {
		return nil, err
	}

	return corev1.DeleteProblem200Response{}, nil
}

func (h *CoreServer) GetProblem(ctx context.Context, request corev1.GetProblemRequestObject) (corev1.GetProblemResponseObject, error) {
	problem, err := h.problemsUC.GetProblemById(ctx, request.Id)
	if err != nil {
		return nil, err
	}

	statement := h.loadProblemStatement(ctx, request.Id)

	return corev1.GetProblem200JSONResponse{Problem: *ProblemDTO(problem, statement)}, nil
}

func (h *CoreServer) UpdateProblem(ctx context.Context, request corev1.UpdateProblemRequestObject) (corev1.UpdateProblemResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}
	req := *request.Body

	// Build update params
	update := &models.ProblemUpdate{}

	// Handle title update
	if req.Title != nil {
		update.Title = req.Title
	}

	// Handle visibility update
	if req.Visibility != nil {
		update.Visibility = req.Visibility
	}

	// Note: Other fields (Legend, InputFormat, etc.) are now stored in git repos
	// and managed through the workshop/publish workflow

	err := h.problemsUC.UpdateProblem(ctx, request.Id, update)
	if err != nil {
		return nil, err
	}

	return corev1.UpdateProblem200Response{}, nil
}
