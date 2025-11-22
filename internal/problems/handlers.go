package problems

import (
	"context"
	"fmt"
	"io"
	"unicode/utf8"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ProblemsUC interface {
	CreateProblem(ctx context.Context, input *models.CreateProblemInput) (uuid.UUID, error)
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problemUpdate *models.ProblemUpdate) error
	UploadProblemTests(ctx context.Context, problemId uuid.UUID, zipData []byte) error
}

type PermissionsUC interface {
	HasProblemPermission(ctx context.Context, problemID uuid.UUID, userID uuid.UUID, action permissions.ProblemAction, opts ...permissions.PermissionOption) (bool, error)
}

type ProblemsHandlers struct {
	problemsUC    ProblemsUC
	permissionsUC PermissionsUC
}

func NewHandlers(problemsUC ProblemsUC, permissionsUC PermissionsUC) *ProblemsHandlers {
	return &ProblemsHandlers{
		problemsUC:    problemsUC,
		permissionsUC: permissionsUC,
	}
}

const (
	minPage         = 1
	minPageSize     = 1
	maxPageSize     = 20
	maxSearchLength = 50
)

func badInput(msg string) error {
	return pkg.Wrap(pkg.ErrBadInput, nil, msg)
}

var (
	badPageSize = badInput(
		fmt.Sprintf("page_size parameter must be between %d and %d", minPageSize, maxPageSize),
	)
	badPage = badInput(
		fmt.Sprintf("page parameter must be >= %d", minPage),
	)
	badSearch = badInput(
		fmt.Sprintf("search parameter length must be between 0 and %d characters", maxSearchLength),
	)
)

func isLengthBetween(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

func (h *ProblemsHandlers) ListProblems(c *fiber.Ctx, params testerv1.ListProblemsParams) error {
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
		user, err := middleware.GetUser(ctx)
		if err != nil {
			return err
		}

		filter.OwnerId = &user.Id
	}

	if params.Descending != nil {
		filter.Descending = *params.Descending
	}

	problemsList, err := h.problemsUC.ListProblems(ctx, filter)
	if err != nil {
		return err
	}

	resp := testerv1.ListProblemsResponseModel{
		Problems:   make([]testerv1.ProblemsListItemModel, len(problemsList.Problems)),
		Pagination: PaginationDTO(problemsList.Pagination),
	}

	for i, problem := range problemsList.Problems {
		resp.Problems[i] = ProblemsListItemDTO(*problem)
	}
	return c.JSON(resp)
}

func (h *ProblemsHandlers) CreateProblem(c *fiber.Ctx, params testerv1.CreateProblemParams) error {
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

	userIdStr, ok := session.Identity.MetadataPublic["user_id"].(string)
	if !ok {
		return pkg.Wrap(pkg.NoPermission, nil, "no user id found in session")
	}
	userId, err := uuid.Parse(userIdStr)
	if err != nil {
		return pkg.Wrap(pkg.NoPermission, nil, "invalid user id")
	}

	input := &models.CreateProblemInput{
		Title:  params.Title,
		UserId: userId,
	}

	problemID, err := h.problemsUC.CreateProblem(ctx, input)
	if err != nil {
		return err
	}

	return c.JSON(&testerv1.CreationResponseModel{Id: problemID})
}

func (h *ProblemsHandlers) DeleteProblem(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	canAdmin, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, permissions.ActionAdminProblem, permissions.WithUser(user))
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

func (h *ProblemsHandlers) GetProblem(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	canView, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, permissions.ActionGetProblem, permissions.WithUser(user))
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

	return c.JSON(&testerv1.GetProblemResponseModel{Problem: *ProblemDTO(problem)})
}

func (h *ProblemsHandlers) UpdateProblem(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, permissions.ActionUpdateProblem, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canEdit {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to update problem")
	}

	var req testerv1.UpdateProblemRequestModel

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

func PaginationDTO(p models.Pagination) testerv1.PaginationModel {
	return testerv1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
	}
}

func ProblemsListItemDTO(p models.ProblemsListItem) testerv1.ProblemsListItemModel {
	return testerv1.ProblemsListItemModel{
		Id:          p.Id,
		Title:       p.Title,
		MemoryLimit: p.MemoryLimit,
		TimeLimit:   p.TimeLimit,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func ProblemDTO(p *models.Problem) *testerv1.ProblemModel {
	return &testerv1.ProblemModel{
		Id:          p.Id,
		Title:       p.Title,
		TimeLimit:   p.TimeLimit,
		MemoryLimit: p.MemoryLimit,

		Legend:       p.Legend,
		InputFormat:  p.InputFormat,
		OutputFormat: p.OutputFormat,
		Notes:        p.Notes,
		Scoring:      p.Scoring,

		LegendHtml:       p.LegendHtml,
		InputFormatHtml:  p.InputFormatHtml,
		OutputFormatHtml: p.OutputFormatHtml,
		NotesHtml:        p.NotesHtml,
		ScoringHtml:      p.ScoringHtml,

		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

const (
	maxArchiveSize = 10 * 1024 * 1024 // 10 MB
)

func (h *ProblemsHandlers) UploadProblemTests(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	// Check permissions: owner or moderator can upload tests
	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, id, user.Id, permissions.ActionUpdateProblem, permissions.WithUser(user))
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
