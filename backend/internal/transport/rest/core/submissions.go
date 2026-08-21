package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) CreateSubmission(ctx context.Context, request corev1.CreateSubmissionRequestObject) (corev1.CreateSubmissionResponseObject, error) {
	user := middleware.GetUser(ctx)

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

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.Params.OrganizationLogin, request.Params.ContestLogin)
	if err != nil {
		return nil, err
	}

	solutionCreation := &models.SubmissionCreation{
		UserId:    user.Id,
		ProblemId: request.Params.ProblemId,
		ContestId: contest.ID,
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
	submission, err := h.submissionsUC.GetSubmission(ctx, request.SubmissionId)
	if err != nil {
		return nil, err
	}

	return corev1.GetSubmission200JSONResponse{Submission: SolutionDTO(submission)}, nil
}

func (h *CoreServer) ListSubmissions(ctx context.Context, request corev1.ListSubmissionsRequestObject) (corev1.ListSubmissionsResponseObject, error) {
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

func (h *CoreServer) RejudgeSubmission(ctx context.Context, request corev1.RejudgeSubmissionRequestObject) (corev1.RejudgeSubmissionResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	filter := models.RejudgeFilter{
		ContestID:    contest.ID,
		SubmissionID: &request.SubmissionId,
	}

	_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.RejudgeSubmission200Response{}, nil
}

func (h *CoreServer) RejudgeContestProblem(ctx context.Context, request corev1.RejudgeContestProblemRequestObject) (corev1.RejudgeContestProblemResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	filter := models.RejudgeFilter{
		ContestID: contest.ID,
		ProblemID: &request.ProblemId,
	}

	_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.RejudgeContestProblem200Response{}, nil
}

func (h *CoreServer) RejudgeContest(ctx context.Context, request corev1.RejudgeContestRequestObject) (corev1.RejudgeContestResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	filter := models.RejudgeFilter{
		ContestID: contest.ID,
	}

	_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.RejudgeContest200Response{}, nil
}

