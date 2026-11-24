package core

import (
	"context"

	"github.com/brawler2011/gate/backend/pkg/outputs"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateSubmission(ctx context.Context, req *corev1.CreateSubmissionRequestModel, params corev1.CreateSubmissionParams) (*corev1.CreationResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}

	solutionSize := int64(len(req.Submission))
	if solutionSize == 0 {
		return nil, pkg.AtField("/submission", pkg.Invalid(pkg.ErrRequired, "solution is empty", nil))
	}
	if solutionSize > maxSolutionSize {
		return nil, pkg.AtField("/submission", pkg.Invalid(pkg.ErrTooLarge, "solution is too large", map[string]any{"max": maxSolutionSize}))
	}

	if _, ok := h.languages.Langs[params.Language]; !ok {
		return nil, pkg.Invalid(pkg.ErrUnknownLanguage, "unknown language configuration", map[string]any{"id": params.Language})
	}

	contest, err := h.Contests.GetContest(ctx, params.ContestID)
	if err != nil {
		return nil, err
	}
	if !contest.Settings.AllowsLanguage(params.Language) {
		return nil, pkg.Invalid(pkg.ErrLanguageNotAllowed, "language configuration is not allowed in this contest", map[string]any{"id": params.Language})
	}

	solutionCreation := &submissionsmodel.SubmissionCreation{
		UserId:    user.Id,
		ProblemId: params.ProblemID,
		ContestId: contest.ID,
		Language:  params.Language,
		Solution:  req.Submission,
		Penalty:   20,
	}

	solutionID, err := h.Submissions.CreateSubmission(ctx, solutionCreation)
	if err != nil {
		return nil, err
	}

	return &corev1.CreationResponseModel{ID: solutionID}, nil
}

func (h *CoreServer) CreateOutputsSubmission(ctx context.Context, req *corev1.CreateOutputsSubmissionReq, params corev1.CreateOutputsSubmissionParams) (*corev1.CreationResponseModel, error) {
	files := make([]outputs.File, len(req.Files))
	for i, f := range req.Files {
		files[i] = outputs.File{Name: f.Name, Data: f.File}
	}
	archive, sha, err := outputs.Pack(files)
	if err != nil {
		return nil, err //nolint:wrapcheck // validation errors point at the field
	}
	creation := &submissionsmodel.SubmissionCreation{UserId: accessmodel.UserFromContext(ctx).Id, ProblemId: params.ProblemID, ContestId: params.ContestID, Penalty: 20}
	id, err := h.Submissions.CreateOutputsSubmission(ctx, creation, archive, sha)
	if err != nil {
		return nil, err
	}
	return &corev1.CreationResponseModel{ID: id}, nil
}

func (h *CoreServer) DownloadSubmissionOutputs(ctx context.Context, params corev1.DownloadSubmissionOutputsParams) (corev1.DownloadSubmissionOutputsOK, error) {
	file, err := h.Submissions.OpenOutputs(ctx, params.SubmissionID)
	if err != nil {
		return corev1.DownloadSubmissionOutputsOK{}, err
	}
	return corev1.DownloadSubmissionOutputsOK{Data: file}, nil
}

func (h *CoreServer) GetSubmission(ctx context.Context, params corev1.GetSubmissionParams) (*corev1.GetSubmissionResponseModel, error) {
	submission, err := h.Submissions.GetSubmission(ctx, params.SubmissionID)
	if err != nil {
		return nil, err
	}

	return &corev1.GetSubmissionResponseModel{Submission: SolutionDTO(submission)}, nil
}

func (h *CoreServer) UpdateSubmission(ctx context.Context, req *corev1.UpdateSubmissionRequestModel, params corev1.UpdateSubmissionParams) error {
	var reason *string
	if v, ok := req.Reason.Get(); ok {
		reason = &v
	}
	return h.Submissions.SetSubmissionBlocked(ctx, params.SubmissionID, req.Blocked, reason)
}

func (h *CoreServer) ListSubmissions(ctx context.Context, params corev1.ListSubmissionsParams) (*corev1.ListSubmissionsResponseModel, error) {
	filter := ListSolutionsParamsDTO(params)

	solutionsList, err := h.Submissions.ListSubmissions(ctx, filter)
	if err != nil {
		return nil, err
	}

	return ListSolutionsResponseDTO(solutionsList), nil
}

func ListSolutionsParamsDTO(params corev1.ListSubmissionsParams) submissionsmodel.SubmissionsFilter {
	var langName *string
	if params.Language.IsSet() {
		langName = &params.Language.Value
	}

	var state *submissionsmodel.State = nil
	if params.State.IsSet() {
		t := submissionsmodel.State(params.State.Value)
		state = &t
	}

	var order *int32 = nil
	if params.SortOrder.IsSet() {
		var orderVal int32
		if params.SortOrder.Value == corev1.SubmissionSortOrderQueryDesc {
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

	return submissionsmodel.SubmissionsFilter{
		ContestId: contestID,
		Page:      params.Page.Or(1),
		PageSize:  params.PageSize.Or(50),
		UserId:    userID,
		ProblemId: problemID,
		Language:  langName,
		Order:     order,
		State:     state,
	}
}

func ListSolutionsResponseDTO(solutionsList *submissionsmodel.SubmissionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		Submissions: make([]corev1.SubmissionsListItemModel, len(solutionsList.Submissions)),
		Pagination:  PaginationDTO(solutionsList.Pagination),
	}

	for i, solution := range solutionsList.Submissions {
		resp.Submissions[i] = SubmissionListItemDTO(solution)
	}

	return &resp
}

func (h *CoreServer) RejudgeContest(ctx context.Context, req corev1.OptRejudgeContestRequestModel, params corev1.RejudgeContestParams) error {
	filter := submissionsmodel.RejudgeFilter{ContestID: params.ContestID}
	if v, ok := req.Value.ProblemID.Get(); ok {
		filter.ProblemID = &v
	}
	if v, ok := req.Value.SubmissionID.Get(); ok {
		filter.SubmissionID = &v
	}
	if filter.ProblemID != nil && filter.SubmissionID != nil {
		return pkg.Wrap(pkg.ErrInvalid, nil, "set problem_id or submission_id, not both")
	}
	_, err := h.Submissions.RejudgeSubmissions(ctx, filter)
	return err
}
