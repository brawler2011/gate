package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
)

func (h *CoreServer) ListProblems(ctx context.Context, request corev1.ListProblemsRequestObject) (corev1.ListProblemsResponseObject, error) {
	filter := &models.ProblemsFilter{
		Page:       request.Params.Page,
		PageSize:   request.Params.PageSize,
		Search:     request.Params.Search,
		Descending: false,
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
		filter.Search = request.Params.Search
	}

	if request.Params.Owner != nil {
		user := middleware.GetUser(ctx)

		filter.OwnerId = &user.Id
	}

	if request.Params.Descending != nil {
		filter.Descending = *request.Params.Descending
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

	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	if request.Params.Title == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	input := &models.CreateProblemInput{
		Title:  request.Params.Title,
		UserId: user.Id,
	}

	problemID, err := h.problemsUC.CreateProblem(ctx, input)
	if err != nil {
		return nil, err
	}

	return corev1.CreateProblem200JSONResponse{Id: problemID}, nil
}

func (h *CoreServer) DeleteProblem(ctx context.Context, request corev1.DeleteProblemRequestObject) (corev1.DeleteProblemResponseObject, error) {
	user := middleware.GetUser(ctx)

	canAdmin, err := h.permissionsUC.HasProblemPermission(ctx, request.Id, user.Id, models.ActionAdminProblem)
	if err != nil {
		return nil, err
	}
	if !canAdmin {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to delete problem")
	}

	err = h.problemsUC.DeleteProblem(ctx, request.Id)
	if err != nil {
		return nil, err
	}

	return corev1.DeleteProblem200Response{}, nil
}

func (h *CoreServer) GetProblem(ctx context.Context, request corev1.GetProblemRequestObject) (corev1.GetProblemResponseObject, error) {
	user := middleware.GetUser(ctx)

	canView, err := h.permissionsUC.HasProblemPermission(ctx, request.Id, user.Id, models.ActionGetProblem)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view problem")
	}

	problem, err := h.problemsUC.GetProblemById(ctx, request.Id)
	if err != nil {
		return nil, err
	}

	return corev1.GetProblem200JSONResponse{Problem: *ProblemDTO(problem)}, nil
}

func (h *CoreServer) UpdateProblem(ctx context.Context, request corev1.UpdateProblemRequestObject) (corev1.UpdateProblemResponseObject, error) {
	user := middleware.GetUser(ctx)

	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, request.Id, user.Id, models.ActionUpdateProblem)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to update problem")
	}

	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}
	req := *request.Body

	err = h.problemsUC.UpdateProblem(ctx, request.Id, &models.ProblemUpdate{
		Title:       req.Title,
		MemoryLimit: req.MemoryLimit,
		TimeLimit:   req.TimeLimit,
		Visibility:  req.Visibility,

		Legend:       req.Legend,
		InputFormat:  req.InputFormat,
		OutputFormat: req.OutputFormat,
		Notes:        req.Notes,
		Scoring:      req.Scoring,
	})

	if err != nil {
		return nil, err
	}

	return corev1.UpdateProblem200Response{}, nil
}
