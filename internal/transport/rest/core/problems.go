package handlers

import (
	"io"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func (h *CoreServer) ListProblems(c *fiber.Ctx, params corev1.ListProblemsParams) error {
	ctx := c.UserContext()

	filter := &models.ProblemsFilter{
		Page:       params.Page,
		PageSize:   params.PageSize,
		Search:     params.Search,
		Descending: false,
	}

	if params.PageSize < minPageSize || params.PageSize > maxPageSize {
		return badPageSize
	}

	if params.Page < minPage {
		return badPage
	}

	if params.Search != nil {
		if !isLengthBetween(*params.Search, 0, maxSearchLength) {
			return badSearch
		}
		filter.Search = params.Search
	}

	if params.Owner != nil {
		user := middleware.GetUser(ctx)

		filter.OwnerId = &user.Id
	}

	if params.Descending != nil {
		filter.Descending = *params.Descending
	}

	problemsList, err := h.problemsUC.ListProblems(ctx, filter)
	if err != nil {
		return err
	}

	resp := corev1.ListProblemsResponseModel{
		Problems:   make([]corev1.ProblemsListItemModel, len(problemsList.Problems)),
		Pagination: PaginationDTO(problemsList.Pagination),
	}

	for i, problem := range problemsList.Problems {
		resp.Problems[i] = ProblemsListItemDTO(problem)
	}
	return c.JSON(resp)
}

func (h *CoreServer) CreateProblem(c *fiber.Ctx, params corev1.CreateProblemParams) error {
	ctx := c.UserContext()

	session, err := middleware.GetSession(ctx)
	if err != nil {
		return err
	}

	if session == nil {
		return pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	if params.Title == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	user := middleware.GetUser(ctx)

	input := &models.CreateProblemInput{
		Title:  params.Title,
		UserId: user.Id,
	}

	problemID, err := h.problemsUC.CreateProblem(ctx, input)
	if err != nil {
		return err
	}

	return c.JSON(&corev1.CreationResponseModel{Id: problemID})
}

func (h *CoreServer) DeleteProblem(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	canAdmin, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, models.ActionAdminProblem)
	if err != nil {
		return err
	}
	if !canAdmin {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to delete problem")
	}

	err = h.problemsUC.DeleteProblem(ctx, id)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *CoreServer) GetProblem(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	canView, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, models.ActionGetProblem)
	if err != nil {
		return err
	}
	if !canView {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view problem")
	}

	problem, err := h.problemsUC.GetProblemById(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(&corev1.GetProblemResponseModel{Problem: *ProblemDTO(problem)})
}

func (h *CoreServer) UpdateProblem(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, models.ActionUpdateProblem)
	if err != nil {
		return err
	}
	if !canEdit {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to update problem")
	}

	var req corev1.UpdateProblemRequestModel

	err = c.BodyParser(&req)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to parse request body")
	}

	err = h.problemsUC.UpdateProblem(ctx, id, &models.ProblemUpdate{
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
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *CoreServer) UploadProblemTests(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	// Check permissions: owner or moderator can upload tests
	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, models.ActionUpdateProblem)
	if err != nil {
		return err
	}
	if !canEdit {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to upload tests")
	}

	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to get file from form")
	}

	// Validate file size
	if file.Size > maxArchiveSize {
		return pkg.Wrap(pkg.ErrBadInput, nil, "file size exceeds 10 MB limit")
	}

	// Open and read file
	src, err := file.Open()
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to open uploaded file")
	}
	defer src.Close()

	zipData, err := io.ReadAll(src)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to read uploaded file")
	}

	// Call usecase to process and save tests
	err = h.problemsUC.UploadProblemTests(ctx, id, zipData)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}
