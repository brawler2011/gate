package submissions

import (
	"context"

	testerv1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type SolutionsUC interface {
	GetSubmissions(ctx context.Context, id uuid.UUID) (*models.Submission, error)
	CreateSubmission(ctx context.Context, creation *models.SubmissionCreation) (uuid.UUID, error)
	UpdateSubmission(ctx context.Context, id uuid.UUID, update *models.SubmissionUpdate) error
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error)
}

type ContestsUC interface {
	CreateContest(ctx context.Context, creation models.ContestCreation) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
	ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error)
	UpdateContest(ctx context.Context, id uuid.UUID, contestUpdate models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error
	CreateContestProblem(ctx context.Context, contestId, problemId uuid.UUID) error
	GetContestProblem(ctx context.Context, contestId, problemId uuid.UUID) (*models.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error)
	DeleteContestProblem(ctx context.Context, contestId, problemId uuid.UUID) error
	CreateParticipant(ctx context.Context, contestId, userId uuid.UUID) error
	DeleteParticipant(ctx context.Context, contestId, userId uuid.UUID) error
}

type PermissionsUC interface {
	GetContestPermissions(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestPermissions, error)
}

type UsersUC interface {
	GetUserByKratosId(ctx context.Context, kratosId string) (*models.User, error)
}

type SolutionsHandlers struct {
	solutionsUC   SolutionsUC
	contestsUC    ContestsUC
	permissionsUC PermissionsUC
	usersUC       UsersUC
}

func getUserID(h *SolutionsHandlers, c *fiber.Ctx) (uuid.UUID, error) {
	kratosID, err := getUserFromSession(c)
	if err != nil {
		return uuid.Nil, err
	}

	user, err := h.usersUC.GetUserByKratosId(c.Context(), kratosID)
	if err != nil {
		return uuid.Nil, err
	}

	return user.Id, nil
}

func NewHandlers(
	solutionsUC SolutionsUC,
	contestsUC ContestsUC,
	permissionsUC PermissionsUC,
	usersUC UsersUC,
) *SolutionsHandlers {
	handlers := &SolutionsHandlers{
		solutionsUC:   solutionsUC,
		contestsUC:    contestsUC,
		permissionsUC: permissionsUC,
		usersUC:       usersUC,
	}

	return handlers
}

const (
	maxSolutionSize int64 = 10 * 1024 * 1024 // 10 MB
)

func (h *SolutionsHandlers) CreateSolution(c *fiber.Ctx, params testerv1.CreateSubmissionParams) error {
	const op = "SolutionsHandlers.CreateSubmission"
	ctx := c.Context()

	userID, err := getUserID(h, c)
	if err != nil {
		return err
	}

	// Check if user can create solution in this contest
	permissions, err := h.permissionsUC.GetContestPermissions(ctx, &models.ContestPermissionGet{
		ContestId: params.ContestId,
		UserId:    userID,
	})
	if err != nil {
		return err
	}

	if !permissions {
		return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to create solution")
	}

	s, err := c.FormFile("solution")
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "failed to get solution file")
	}

	if s.Size == 0 || s.Size > maxSolutionSize {
		return pkg.Wrap(pkg.ErrBadInput, nil, op, "invalid solution size")
	}

	f, err := s.Open()
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "failed to open solution file")
	}
	defer f.Close()

	b := make([]byte, s.Size)
	_, err = f.Read(b)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "failed to read solution file")
	}

	solution := string(b)

	langName := models.LanguageName(params.Language)
	if err := langName.Valid(); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, op, "invalid language")
	}

	solutionCreation := &models.SubmissionCreation{
		UserId:    userID,
		ProblemId: params.ProblemId,
		ContestId: params.ContestId,
		Language:  langName,
		Solution:  solution,
		Penalty:   20,
	}

	solutionID, err := h.solutionsUC.CreateSubmission(ctx, solutionCreation)
	if err != nil {
		return err
	}

	return c.JSON(testerv1.CreationResponseModel{Id: solutionID})
}

func (h *SolutionsHandlers) GetSolution(c *fiber.Ctx, id uuid.UUID) error {
	const op = "SolutionsHandlers.GetSubmissions"
	ctx := c.Context()

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
		return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to view this submission")
	}

	return c.JSON(testerv1.GetSubmissionResponseModel{Submission: SolutionDTO(*submission)})
}

func (h *SolutionsHandlers) ListSolutions(c *fiber.Ctx, params testerv1.ListSubmissionsParams) error {
	const op = "SolutionsHandlers.ListSolutions"
	ctx := c.Context()

	userID, err := getUserID(h, c)
	if err != nil {
		return err
	}

	filter := ListSolutionsParamsDTO(params)

	// If contestId is provided, check permissions for that contest
	if params.ContestId != nil {
		// Get contest to check permissions
		contest, err := h.contestsUC.GetContest(ctx, *params.ContestId)
		if err != nil {
			return pkg.Wrap(pkg.ErrInternal, err, op, "failed to get contest")
		}

		// Check if user can view contest
		canView, err := h.permissionsUC.CanViewContest(ctx, userID, contest)
		if err != nil {
			return pkg.Wrap(pkg.ErrInternal, err, op, "failed to check contest view permission")
		}
		if !canView {
			return pkg.Wrap(pkg.NoPermission, nil, op, "insufficient permissions to view contest submissions")
		}
	}

	// Users can only view their own submissions (simplified)
	// For admin/teacher users, they can see all submissions if no userId specified
	if params.UserId == nil || *params.UserId != userID {
		filter.UserId = &userID
	}

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

	return models.SolutionsFilter{
		ContestId: params.ContestId,
		Page:      params.Page,
		PageSize:  params.PageSize,
		ProblemId: params.ProblemId,
		Language:  langName,
		Order:     params.Order,
		State:     state,
	}
}

func ListSolutionsResponseDTO(solutionsList *models.SolutionsList) *testerv1.ListSubmissionsResponseModel {
	resp := testerv1.ListSubmissionsResponseModel{
		Submissions: make([]testerv1.SubmissionsListItemModel, len(solutionsList.Solutions)),
		Pagination:  PaginationDTO(solutionsList.Pagination),
	}

	for i, solution := range solutionsList.Solutions {
		resp.Submissions[i] = SubmissionListItemDTO(*solution)
	}

	return &resp
}

func PaginationDTO(p models.Pagination) testerv1.PaginationModel {
	return testerv1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
	}
}

func SubmissionListItemDTO(s models.SubmissionListItem) testerv1.SubmissionsListItemModel {
	return testerv1.SubmissionsListItemModel{
		Id: s.Id,

		Username: s.Username,

		State:      int64(s.State),
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   int64(s.Language),

		ProblemId:    s.ProblemId,
		ProblemTitle: s.ProblemTitle,

		Position: s.Position,

		ContestId:    s.ContestId,
		ContestTitle: s.ContestTitle,

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func SolutionDTO(s models.Submission) testerv1.SubmissionModel {
	return testerv1.SubmissionModel{
		Id: s.Id,

		Username: s.Username,

		Submission: s.Submission,

		State:      int64(s.State),
		Score:      s.Score,
		Penalty:    s.Penalty,
		TimeStat:   s.TimeStat,
		MemoryStat: s.MemoryStat,
		Language:   int64(s.Language),

		ProblemId:    s.ProblemId,
		ProblemTitle: s.ProblemTitle,

		Position: s.Position,

		ContestId:    s.ContestId,
		ContestTitle: s.ContestTitle,

		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
