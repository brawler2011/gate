package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
)

func (h *CoreServer) CreateSubmission(ctx context.Context, request corev1.CreateSubmissionRequestObject) (corev1.CreateSubmissionResponseObject, error) {
	user := middleware.GetUser(ctx)

	// Check if user can create solution in this contest
	canCreate, err := h.permissionsUC.HasContestPermission(ctx, request.Params.ContestId, user.Id, models.ActionCreateSubmission)
	if err != nil {
		return nil, err
	}
	if !canCreate {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to create solution")
	}

	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}
	req := *request.Body

	// Validate solution size
	solutionSize := int64(len(req.Submission))
	if solutionSize == 0 || solutionSize > maxSolutionSize {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid solution size")
	}

	langName := models.LanguageName(request.Params.Language)
	if err := models.LanguageNameValid(langName); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid language")
	}

	solutionCreation := &models.SubmissionCreation{
		UserId:    user.Id,
		ProblemId: request.Params.ProblemId,
		ContestId: request.Params.ContestId,
		Language:  langName,
		Solution:  req.Submission,
		Penalty:   20,
	}

	solutionID, err := h.submissionsUC.CreateSubmission(ctx, solutionCreation)
	if err != nil {
		return nil, err
	}

	return corev1.CreateSubmission200JSONResponse{Id: solutionID}, nil
}

func (h *CoreServer) GetSubmission(ctx context.Context, request corev1.GetSubmissionRequestObject) (corev1.GetSubmissionResponseObject, error) {
	user := middleware.GetUser(ctx)

	submission, err := h.submissionsUC.GetSubmission(ctx, request.SubmissionId)
	if err != nil {
		return nil, err
	}

	// User can only view their own submission (simplified for now)
	if submission.CreatedBy == nil || *submission.CreatedBy != user.Id {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view this submission")
	}

	return corev1.GetSubmission200JSONResponse{Submission: SolutionDTO(submission)}, nil
}

func (h *CoreServer) ListSubmissions(ctx context.Context, request corev1.ListSubmissionsRequestObject) (corev1.ListSubmissionsResponseObject, error) {
	user := middleware.GetUser(ctx)

	if !user.IsAdmin() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "only admins can list submissions")
	}

	filter := ListSolutionsParamsDTO(request.Params)

	solutionsList, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListSubmissions200JSONResponse(*ListSolutionsResponseDTO(solutionsList)), nil
}

func ListSolutionsParamsDTO(params corev1.ListSubmissionsParams) models.SubmissionsFilter {
	var langName *models.LanguageName = nil
	if params.Language != nil {
		t := models.LanguageName(*params.Language)
		langName = &t
	}

	var state *models.State = nil
	if params.State != nil {
		t := models.State(*params.State)
		state = &t
	}

	// Convert sortOrder string to integer: -1 for desc, 0 for asc
	var order *int32 = nil
	if params.SortOrder != nil {
		var orderVal int32
		if *params.SortOrder == corev1.ListSubmissionsParamsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	return models.SubmissionsFilter{
		ContestId: params.ContestId,
		Page:      params.Page,
		PageSize:  params.PageSize,
		ProblemId: params.ProblemId,
		Language:  langName,
		Order:     order,
		State:     state,
	}
}

func ListSolutionsResponseDTO(solutionsList *models.SubmissionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		Submissions: make([]corev1.SubmissionsListItemModel, len(solutionsList.Submissions)),
		Pagination:  PaginationDTO(solutionsList.Pagination),
	}

	for i, solution := range solutionsList.Submissions {
		resp.Submissions[i] = SubmissionListItemDTO(solution)
	}

	return &resp
}
