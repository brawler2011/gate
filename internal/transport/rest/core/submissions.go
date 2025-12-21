package handlers

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateSubmission(c *fiber.Ctx, params corev1.CreateSubmissionParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	// Check if user can create solution in this contest
	canCreate, err := h.permissionsUC.HasContestPermission(ctx, params.ContestId, user.Id, models.ActionCreateSubmission)
	if err != nil {
		return err
	}
	if !canCreate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to create solution")
	}

	// Parse request body
	var req corev1.CreateSubmissionRequestModel
	err = c.BodyParser(&req)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to parse request body")
	}

	// Validate solution size
	solutionSize := int64(len(req.Submission))
	if solutionSize == 0 || solutionSize > maxSolutionSize {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid solution size")
	}

	langName := models.LanguageName(params.Language)
	if err := models.LanguageNameValid(langName); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid language")
	}

	solutionCreation := &models.SubmissionCreation{
		UserId:    user.Id,
		ProblemId: params.ProblemId,
		ContestId: params.ContestId,
		Language:  langName,
		Solution:  req.Submission,
		Penalty:   20,
	}

	solutionID, err := h.submissionsUC.CreateSubmission(ctx, solutionCreation)
	if err != nil {
		return err
	}

	return c.JSON(corev1.CreationResponseModel{Id: solutionID})
}

func (h *CoreServer) GetSubmission(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	submission, err := h.submissionsUC.GetSubmission(ctx, id)
	if err != nil {
		return err
	}

	// User can only view their own submission (simplified for now)
	if submission.CreatedBy == nil || *submission.CreatedBy != user.Id {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view this submission")
	}

	return c.JSON(corev1.GetSubmissionResponseModel{Submission: SolutionDTO(submission)})
}

func (h *CoreServer) ListSubmissions(c *fiber.Ctx, params corev1.ListSubmissionsParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	if !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "only admins can list submissions")
	}

	filter := ListSolutionsParamsDTO(params)

	solutionsList, err := h.submissionsUC.ListSubmissions(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListSolutionsResponseDTO(solutionsList))
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
