package problems

import (
	"context"
	"io"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type UC interface {
	CreateProblem(ctx context.Context, title string) (uuid.UUID, error)
	GetProblemById(ctx context.Context, id uuid.UUID) (*models.Problem, error)
	DownloadTestsArchive(ctx context.Context, id uuid.UUID) (string, error)
	DeleteProblem(ctx context.Context, id uuid.UUID) error
	ListProblems(ctx context.Context, filter *models.ProblemsFilter) (*models.ProblemsList, error)
	UpdateProblem(ctx context.Context, id uuid.UUID, problemUpdate *models.ProblemUpdate) error
	UploadProblem(ctx context.Context, id uuid.UUID, r io.ReaderAt, size int64) error
}

type PermissionsUC interface {
	GetProblemPermissions(ctx context.Context, p *models.ProblemPermissionGet) (*models.ProblemPermissions, error)
}

type ProblemsHandlers struct {
	problemsUC    UC
	permissionsUC PermissionsUC
}

func NewHandlers(problemsUC UC, permissionsUC PermissionsUC) *ProblemsHandlers {
	return &ProblemsHandlers{
		problemsUC:    problemsUC,
		permissionsUC: permissionsUC,
	}
}

func (h *ProblemsHandlers) ListProblems(c *fiber.Ctx, params testerv1.ListProblemsParams) error {
	ctx := c.Context()

	filter := &models.ProblemsFilter{
		Page:       params.Page,
		PageSize:   params.PageSize,
		Search:     params.Search,
		Descending: false,
	}

	if params.PageSize <= 0 || params.PageSize > 100 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "", "page_size parameter must be between 1 and 100")
	}

	if params.Page < 1 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "", "page parameter must be >= 1")
	}

	if params.Descending != nil {
		filter.Descending = *params.Descending
	}

	if params.Owner != nil {
		user, err := middleware.GetUser(ctx)
		if err != nil {
			return err
		}

		filter.OwnerId = &user.Id
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
	const op = "ProblemsHandlers.CreateProblem"
	ctx := c.Context()

	session, err := middleware.GetSession(ctx)
	if err != nil {
		return err
	}

	if session == nil {
		return pkg.Wrap(pkg.NoPermission, nil, op, "no session found")
	}

	if params.Title == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, op, "empty title")
	}

	problemID, err := h.problemsUC.CreateProblem(ctx, params.Title)
	if err != nil {
		return err
	}

	return c.JSON(&testerv1.CreationResponseModel{Id: problemID})
}

func (h *ProblemsHandlers) DeleteProblem(c *fiber.Ctx, id uuid.UUID) error {
	const op = "ProblemsHandlers.DeleteProblem"
	ctx := c.Context()

	// Get user database ID
	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, op, "no session found")
	}

	permissions, err := h.permissionsUC.GetProblemPermissions(ctx, &models.ProblemPermissionGet{
		ProblemId: id,
		UserId:    user.Id,
	})
	if err != nil {
		return err
	}

	if !permissions.AdminProblem {
		return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to delete problem")
	}

	err = h.problemsUC.DeleteProblem(ctx, id)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ProblemsHandlers) GetProblem(c *fiber.Ctx, id uuid.UUID) error {
	const op = "ProblemsHandlers.GetProblem"
	ctx := c.Context()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, op, "no session found")
	}

	permissions, err := h.permissionsUC.GetProblemPermissions(ctx, &models.ProblemPermissionGet{
		ProblemId: id,
		UserId:    user.Id,
	})
	if err != nil {
		return err
	}

	if !permissions.ViewProblem {
		return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to view problem")
	}

	problem, err := h.problemsUC.GetProblemById(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(&testerv1.GetProblemResponseModel{Problem: *ProblemDTO(problem)})
}

func (h *ProblemsHandlers) UpdateProblem(c *fiber.Ctx, id uuid.UUID) error {
	const op = "ProblemsHandlers.UpdateProblem"
	ctx := c.Context()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, op, "no session found")
	}

	permissions, err := h.permissionsUC.GetProblemPermissions(ctx, &models.ProblemPermissionGet{
		ProblemId: id,
		UserId:    user.Id,
	})
	if err != nil {
		return err
	}

	if !permissions.EditProblem {
		return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to update problem")
	}

	var req testerv1.UpdateProblemRequestModel

	err = c.BodyParser(&req)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "failed to parse request body")
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

func (h *ProblemsHandlers) UploadProblem(c *fiber.Ctx, id uuid.UUID) error {
	const op = "ProblemsHandlers.UploadProblem"
	ctx := c.Context()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user == nil {
		return pkg.Wrap(pkg.NoPermission, nil, op, "no session found")
	}

	permissions, err := h.permissionsUC.GetProblemPermissions(ctx, &models.ProblemPermissionGet{
		ProblemId: id,
		UserId:    user.Id,
	})
	if err != nil {
		return err
	}

	if !permissions.EditProblem {
		return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to upload problem archive")
	}

	a, err := c.FormFile("archive")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "no archive uploaded")
	}

	if a.Size == 0 || a.Size > 1024*1024*1024 {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "invalid archive size")
	}

	f, err := a.Open()
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "failed to open archive")
	}
	defer f.Close()

	err = h.problemsUC.UploadProblem(ctx, id, f, a.Size)
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
