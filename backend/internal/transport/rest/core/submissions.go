package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateSubmission(ctx context.Context, req *corev1.CreateSubmissionRequestModel, params corev1.CreateSubmissionParams) (*corev1.CreationResponseModel, error) {
	user := middleware.GetUser(ctx)

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	// Validate solution size
	solutionSize := int64(len(req.Submission))
	if solutionSize == 0 || solutionSize > maxSolutionSize {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid solution size")
	}

	langName := models.LanguageName(params.Language)
	if err := models.LanguageNameValid(langName); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid language")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrganizationLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	solutionCreation := &models.SubmissionCreation{
		UserId:    user.Id,
		ProblemId: params.ProblemID,
		ContestId: contest.ID,
		Language:  langName,
		Solution:  req.Submission,
		Penalty:   20,
	}

	solutionID, err := h.submissionsUC.CreateSubmission(ctx, solutionCreation)
	if err != nil {
		return nil, err
	}

	return &corev1.CreationResponseModel{ID: solutionID}, nil
}

func (h *CoreServer) GetSubmission(ctx context.Context, params corev1.GetSubmissionParams) (*corev1.GetSubmissionResponseModel, error) {
	submission, err := h.submissionsUC.GetSubmission(ctx, params.SubmissionID)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	canViewDetails := false

	switch {
	case user.IsAdmin():
		canViewDetails = true
	case submission.ContestID != nil:
		allowed, err := h.permissionsUC.HasContestPermission(
			ctx,
			*submission.ContestID,
			user.Id,
			models.ActionGetSubmissionDetails,
		)
		if err == nil && allowed {
			canViewDetails = true
		}
	case submission.CreatedBy != nil && *submission.CreatedBy == user.Id:
		canViewDetails = true
	}

	if !canViewDetails {
		submission.TestDetails = nil
	}

	return &corev1.GetSubmissionResponseModel{Submission: SolutionDTO(submission)}, nil
}

func (h *CoreServer) BlockSubmission(ctx context.Context, req corev1.OptBlockSubmissionRequestModel, params corev1.BlockSubmissionParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	var reason *string
	if req.IsSet() && req.Value.Reason.IsSet() {
		reason = &req.Value.Reason.Value
	}

	err = h.submissionsUC.BlockSubmission(ctx, contest.ID, params.SubmissionID, reason)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UnblockSubmission(ctx context.Context, params corev1.UnblockSubmissionParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	err = h.submissionsUC.UnblockSubmission(ctx, contest.ID, params.SubmissionID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListSubmissions(ctx context.Context, params corev1.ListSubmissionsParams) (*corev1.ListSubmissionsResponseModel, error) {
	filter := ListSolutionsParamsDTO(params)

	solutionsList, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return ListSolutionsResponseDTO(solutionsList), nil
}

func ListSolutionsParamsDTO(params corev1.ListSubmissionsParams) models.SubmissionsFilter {
	var langName *models.LanguageName = nil
	if params.Language.IsSet() {
		t := models.LanguageName(params.Language.Value)
		langName = &t
	}

	var state *models.State = nil
	if params.State.IsSet() {
		t := models.State(params.State.Value)
		state = &t
	}

	// Convert sortOrder string to integer: -1 for desc, 0 for asc
	var order *int32 = nil
	if params.SortOrder.IsSet() {
		var orderVal int32
		if params.SortOrder.Value == corev1.ListSubmissionsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	var contestID *uuid.UUID
	if params.ContestId.IsSet() {
		contestID = &params.ContestId.Value
	}

	var userID *uuid.UUID
	if params.UserId.IsSet() {
		userID = &params.UserId.Value
	}

	var problemID *uuid.UUID
	if params.ProblemId.IsSet() {
		problemID = &params.ProblemId.Value
	}

	return models.SubmissionsFilter{
		ContestId: contestID,
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserId:    userID,
		ProblemId: problemID,
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

func (h *CoreServer) RejudgeSubmission(ctx context.Context, params corev1.RejudgeSubmissionParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	filter := models.RejudgeFilter{
		ContestID:    contest.ID,
		SubmissionID: &params.SubmissionID,
	}

	_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) RejudgeContestProblem(ctx context.Context, params corev1.RejudgeContestProblemParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	filter := models.RejudgeFilter{
		ContestID: contest.ID,
		ProblemID: &params.ProblemID,
	}

	_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) RejudgeContest(ctx context.Context, params corev1.RejudgeContestParams) error {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	filter := models.RejudgeFilter{
		ContestID: contest.ID,
	}

	_, err = h.submissionsUC.RejudgeSubmissions(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

