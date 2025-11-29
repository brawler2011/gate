package submissions

import (
	"context"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type SolutionsUC interface {
	GetSubmissions(ctx context.Context, id uuid.UUID) (domain.Submission, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*domain.SubmissionsList, error)
}

type PermissionsUC interface {
	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action permissions.ContestAction, opts ...permissions.PermissionOption) (bool, error)
}

type UsersUC interface {
	GetUserByKratosId(ctx context.Context, kratosId string) (domain.User, error)
}

type SolutionsHandlers struct {
	solutionsUC   SolutionsUC
	permissionsUC PermissionsUC
	usersUC       UsersUC
}

func getUserID(h *SolutionsHandlers, c *fiber.Ctx) (uuid.UUID, error) {
	ctx := c.UserContext()

	session, err := middleware.GetSession(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	user, err := h.usersUC.GetUserByKratosId(ctx, session.Identity.Id)
	if err != nil {
		return uuid.Nil, err
	}

	return user.ID, nil
}

func NewHandlers(
	solutionsUC SolutionsUC,
	permissionsUC PermissionsUC,
	usersUC UsersUC,
) *SolutionsHandlers {
	handlers := &SolutionsHandlers{
		solutionsUC:   solutionsUC,
		permissionsUC: permissionsUC,
		usersUC:       usersUC,
	}

	return handlers
}

const (
	maxSolutionSize int64 = 10 * 1024 * 1024 // 10 MB
)

func (h *SolutionsHandlers) CreateSubmission(c *fiber.Ctx, params testerv1.CreateSubmissionParams) error {
	ctx := c.UserContext()

	userID, err := getUserID(h, c)
	if err != nil {
		return err
	}

	// Check if user can create solution in this contest
	canCreate, err := h.permissionsUC.HasContestPermission(ctx, params.ContestId, userID, permissions.ActionCreateSubmission)
	if err != nil {
		return err
	}
	if !canCreate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to create solution")
	}

	// Parse request body
	var req testerv1.CreateSubmissionRequestModel
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
	if err := langName.Valid(); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid language")
	}

	solutionCreation := &models.SubmissionCreation{
		UserId:    userID,
		ProblemId: params.ProblemId,
		ContestId: params.ContestId,
		Language:  langName,
		Solution:  req.Submission,
		Penalty:   20,
	}

	solutionID, err := h.solutionsUC.CreateSubmission(ctx, solutionCreation)
	if err != nil {
		return err
	}

	return c.JSON(testerv1.CreationResponseModel{Id: solutionID})
}

func (h *SolutionsHandlers) GetSubmission(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	userID, err := getUserID(h, c)
	if err != nil {
		return err
	}

	submission, err := h.solutionsUC.GetSubmissions(ctx, id)
	if err != nil {
		return err
	}

	// User can only view their own submission (simplified for now)
	if submission.CreatedBy != userID {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to view this submission")
	}

	return c.JSON(testerv1.GetSubmissionResponseModel{Submission: SolutionDTO(submission)})
}

func (h *SolutionsHandlers) ListSubmissions(c *fiber.Ctx, params testerv1.ListSubmissionsParams) error {
	ctx := c.UserContext()

	// Get the current user
	session, err := middleware.GetSession(ctx)
	if err != nil {
		return err
	}

	user, err := h.usersUC.GetUserByKratosId(ctx, session.Identity.Id)
	if err != nil {
		return err
	}

	// Only admins can list submissions
	if !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "only admins can list submissions")
	}

	filter := ListSolutionsParamsDTO(params)

	solutionsList, err := h.solutionsUC.ListSolutions(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListSolutionsResponseDTO(solutionsList))
}

func ListSolutionsParamsDTO(params testerv1.ListSubmissionsParams) models.SolutionsFilter {
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
	var order *int64 = nil
	if params.SortOrder != nil {
		var orderVal int64
		if *params.SortOrder == testerv1.ListSubmissionsParamsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	return models.SolutionsFilter{
		ContestId: params.ContestId,
		Page:      params.Page,
		PageSize:  params.PageSize,
		ProblemId: params.ProblemId,
		Language:  langName,
		Order:     order,
		State:     state,
	}
}

func ListSolutionsResponseDTO(solutionsList *domain.SubmissionsList) *testerv1.ListSubmissionsResponseModel {
	resp := testerv1.ListSubmissionsResponseModel{
		Submissions: make([]testerv1.SubmissionsListItemModel, len(solutionsList.Submissions)),
		Pagination:  PaginationDTO(solutionsList.Pagination),
	}

	for i, solution := range solutionsList.Submissions {
		resp.Submissions[i] = SubmissionListItemDTO(solution)
	}

	return &resp
}

func PaginationDTO(p domain.Pagination) testerv1.PaginationModel {
	return testerv1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
	}
}

func SubmissionListItemDTO(s domain.Submission) testerv1.SubmissionsListItemModel {
	return testerv1.SubmissionsListItemModel{
		Id: s.ID,

		Username: s.Username,

		State:      int64(s.State),
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   int64(s.Language),

		ProblemId:    s.ProblemID,
		ProblemTitle: s.ProblemTitle,

		Position: s.Position,

		ContestId:    s.ContestID,
		ContestTitle: s.ContestTitle,

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func SolutionDTO(s domain.Submission) testerv1.SubmissionModel {
	return testerv1.SubmissionModel{
		Id: s.ID,

		Username: s.Username,

		Submission: s.Submission,

		State:      int64(s.State),
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   int64(s.Language),

		ProblemId:    s.ProblemID,
		ProblemTitle: s.ProblemTitle,

		Position: s.Position,

		ContestId:    s.ContestID,
		ContestTitle: s.ContestTitle,

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
