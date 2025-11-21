package contests

import (
	"context"
	"unicode/utf8"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ContestsUC interface {
	CreateContest(ctx context.Context, c *models.CreateContestInput) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (*models.Contest, error)
	ListContests(ctx context.Context, filter models.ContestsFilter) (*models.ContestsList, error)
	UpdateContest(ctx context.Context, c models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (*models.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]*models.ContestProblemsListItem, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	CreateParticipant(ctx context.Context, c models.ParticipantCreation) error
	DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error
	ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*models.ParticipantsList, error)

	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (*models.ContestMemberRecord, error)
}

type SubmissionsUC interface {
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*models.SolutionsList, error)
}

type PermissionsUC interface {
	HasContestPermission(ctx context.Context, contestID uuid.UUID, userID uuid.UUID, action permissions.ContestAction, opts ...permissions.PermissionOption) (bool, error)
}

type ContestsHandlers struct {
	contestsUC    ContestsUC
	permissionsUC PermissionsUC
	submissionsUC SubmissionsUC
}

func NewHandlers(contestsUC ContestsUC, permissionsUC PermissionsUC, submissionsUC SubmissionsUC) *ContestsHandlers {
	return &ContestsHandlers{
		contestsUC:    contestsUC,
		permissionsUC: permissionsUC,
		submissionsUC: submissionsUC,
	}
}

func validateCreateContestParams(params corev1.CreateContestParams) error {
	if params.Title == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	titleLength := utf8.RuneCountInString(params.Title)
	if titleLength < 3 || titleLength > 64 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title must be between 3 and 64 characters")
	}

	return nil
}

func (h *ContestsHandlers) CreateContest(c *fiber.Ctx, params corev1.CreateContestParams) error {
	ctx := c.UserContext()

	err := validateCreateContestParams(params)
	if err != nil {
		return err
	}

	session, err := middleware.GetSession(ctx)
	if err != nil {
		return err
	}
	if session == nil {
		return pkg.Wrap(pkg.NoPermission, nil, "no session found")
	}

	userIdStr, ok := session.Identity.MetadataPublic["user_id"].(string)
	if !ok {
		return pkg.Wrap(pkg.NoPermission, nil, "no user id found in session")
	}
	userId, err := uuid.Parse(userIdStr)
	if err != nil {
		return pkg.Wrap(pkg.NoPermission, nil, "invalid user id")
	}

	contestCreation := &models.CreateContestInput{
		Title:  params.Title,
		UserId: userId,
	}

	contestID, err := h.contestsUC.CreateContest(ctx, contestCreation)
	if err != nil {
		return err
	}

	return c.JSON(&corev1.CreationResponseModel{Id: contestID})
}

func (h *ContestsHandlers) GetContest(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	// Check if user can view contest (members can list their own submissions)
	canView, err := h.permissionsUC.HasContestPermission(ctx, id, user.Id, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canView {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
	}

	contest, err := h.contestsUC.GetContest(ctx, id)
	if err != nil {
		return err
	}

	ps, err := h.contestsUC.GetContestProblems(ctx, id)
	if err != nil {
		return err
	}

	return c.JSON(GetContestResponseDTO(contest, ps))
}

func publicOrPrivate(s string) bool {
	return s == "private" || s == "public"
}

func checkScope(scope string) bool {
	return scope == "participant" || scope == "moderator" || scope == "owner"
}

func checkLength(s string, min, max int) bool {
	length := utf8.RuneCountInString(s)
	return length >= min && length <= max
}

func validateUpdateContestRequest(params corev1.UpdateContestRequestModel) error {
	if params.Title != nil && !checkLength(*params.Title, 3, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title must be between 3 and 64 characters")
	}

	if params.Description != nil && !checkLength(*params.Description, 0, 2048) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "description length must be less than 2048 characters")
	}

	if params.Visibility != nil && !publicOrPrivate(*params.Visibility) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid visibility value")
	}

	if params.MonitorScope != nil && !checkScope(*params.MonitorScope) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid monitor scope value")
	}

	if params.SubmissionsListScope != nil && !checkScope(*params.SubmissionsListScope) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid submissions list scope value")
	}

	if params.SubmissionsReviewScope != nil && !checkScope(*params.SubmissionsReviewScope) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid submissions review scope value")
	}

	return nil
}

func (h *ContestsHandlers) UpdateContest(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	var req corev1.UpdateContestRequestModel
	err := c.BodyParser(&req)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to parse request body")
	}

	err = validateUpdateContestRequest(req)
	if err != nil {
		return err
	}

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, id, user.Id, permissions.ActionUpdateContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.UpdateContest(ctx, models.ContestUpdate{
		Id:                     id,
		Title:                  req.Title,
		Description:            req.Description,
		Visibility:             req.Visibility,
		MonitorScope:           req.MonitorScope,
		SubmissionsListScope:   req.SubmissionsListScope,
		SubmissionsReviewScope: req.SubmissionsReviewScope,
	})
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ContestsHandlers) DeleteContest(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.Context()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	canAdmin, err := h.permissionsUC.HasContestPermission(ctx, id, user.Id, permissions.ActionAdminContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canAdmin {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to delete contest")
	}

	err = h.contestsUC.DeleteContest(ctx, id)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func validateListContestsParams(params corev1.ListContestsParams) error {
	if params.Page < 1 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page must be greater than 0")
	}

	if params.PageSize < 1 || params.PageSize > 100 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page size must be between 1 and 100")
	}

	if params.Search != nil && !checkLength(*params.Search, 0, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title length must be less than 64 characters")
	}

	return nil
}

func (h *ContestsHandlers) ListContests(c *fiber.Ctx, params corev1.ListContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params)
	if err != nil {
		return err
	}

	filter := models.ContestsFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		Search:   params.Search,
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

	contestsList, err := h.contestsUC.ListContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListContestsResponseDTO(contestsList))
}

func (h *ContestsHandlers) CreateContestProblem(c *fiber.Ctx, contestId uuid.UUID, params corev1.CreateContestProblemParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionUpdateContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.CreateContestProblem(ctx, models.ContestProblemCreation{
		ContestId: contestId,
		ProblemId: params.ProblemId,
	})
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ContestsHandlers) GetContestProblem(c *fiber.Ctx, contestId uuid.UUID, problemId uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	// Check if user can view contest (any permission implies view access)
	canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionGetMonitor, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canView {
		canView, err = h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
		if err != nil {
			return err
		}
	}
	if !canView {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
	}

	p, err := h.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
		ContestId: contestId,
		ProblemId: problemId,
	})
	if err != nil {
		return err
	}

	return c.JSON(GetContestProblemResponseDTO(p))
}

func (h *ContestsHandlers) DeleteContestProblem(c *fiber.Ctx, contestId uuid.UUID, problemId uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionUpdateContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.DeleteContestProblem(ctx, models.ContestProblemDeletion{
		ContestId: contestId,
		ProblemId: problemId,
	})
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ContestsHandlers) CreateContestMember(c *fiber.Ctx, contestId uuid.UUID, params corev1.CreateContestMemberParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionUpdateContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.CreateParticipant(ctx, models.ParticipantCreation{
		ContestId: contestId,
		UserId:    params.UserId,
	})
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ContestsHandlers) DeleteContestMember(c *fiber.Ctx, contestId uuid.UUID, params corev1.DeleteContestMemberParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionUpdateContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.DeleteParticipant(ctx, models.ParticipantDeletion{
		ContestId: contestId,
		UserId:    params.UserId,
	})
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *ContestsHandlers) ListContestMembers(c *fiber.Ctx, contestId uuid.UUID, params corev1.ListContestMembersParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	// Check if user can view contest (any permission implies view access)
	canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionGetMonitor, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canView {
		canView, err = h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
		if err != nil {
			return err
		}
	}
	if !canView {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
	}

	participantsList, err := h.contestsUC.ListParticipants(ctx, models.ParticipantsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		ContestId: contestId,
	})
	if err != nil {
		return err
	}

	resp := corev1.ListContestMembersResponseModel{
		Members:    make([]corev1.ContestMemberModel, len(participantsList.Users)),
		Pagination: PaginationDTO(*participantsList.Pagination),
	}

	for i, user := range participantsList.Users {
		resp.Members[i] = corev1.ContestMemberModel{
			ContestId:   contestId,
			ContestRole: user.ContestRole,
			UserId:      user.UserId,
			KratosId:    user.KratosId,
			Username:    user.Username,
			Role:        user.Role,
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return c.JSON(resp)
}

func (h *ContestsHandlers) GetMyContestRole(c *fiber.Ctx, contestId uuid.UUID) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	contestMember, err := h.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: contestId,
		UserId:    user.Id,
	})
	if err != nil {
		return err
	}

	return c.JSON(corev1.GetMyContestRoleResponseModel{
		Role: contestMember.Role,
	})
}

func (h *ContestsHandlers) ListContestSubmissions(c *fiber.Ctx, contestId uuid.UUID, params corev1.ListContestSubmissionsParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	// Determine which userId to filter by and check permissions
	var filterUserId *uuid.UUID

	if params.UserId == nil {
		// No userId specified - request to get all members' submissions
		// Need permission to list all users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionListUsersSubmissions)
		if err != nil {
			return err
		}
		if !canList {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to list all contest submissions")
		}
		// Don't filter by userId - get all submissions
		filterUserId = nil
	} else if *params.UserId == user.Id {
		// UserId matches current user - check permission to view own submissions
		canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
		if err != nil {
			return err
		}
		if !canView {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
		filterUserId = &user.Id
	} else {
		// UserId is different - need permission to list other users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, permissions.ActionListUsersSubmissions)
		if err != nil {
			return err
		}
		if !canList {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view other users' submissions")
		}
		filterUserId = params.UserId
	}

	var order *int64
	if params.SortOrder != nil {
		var orderVal int64
		if *params.SortOrder == corev1.ListContestSubmissionsParamsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	var state *models.State
	if params.State != nil {
		stateVal := models.State(*params.State)
		state = &stateVal
	}

	var langName *models.LanguageName
	if params.Language != nil {
		lang := models.LanguageName(*params.Language)
		langName = &lang
	}

	submissionsList, err := h.submissionsUC.ListSolutions(ctx, models.SolutionsFilter{
		ContestId: &contestId,
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserId:    filterUserId,
		ProblemId: params.ProblemId,
		Language:  langName,
		State:     state,
		Order:     order,
	})
	if err != nil {
		return err
	}

	return c.JSON(SubmissionsListToDTO(submissionsList))
}

func GetContestResponseDTO(contest *models.Contest, problems []*models.ContestProblemsListItem) *corev1.GetContestResponseModel {
	resp := corev1.GetContestResponseModel{
		Contest:  ContestDTO(*contest),
		Problems: make([]corev1.ContestProblemListItemModel, len(problems)),
	}

	for i, task := range problems {
		resp.Problems[i] = ContestProblemsListItemDTO(*task)
	}

	return &resp
}

func ListContestsResponseDTO(contestsList *models.ContestsList) *corev1.ListContestsResponseModel {
	resp := corev1.ListContestsResponseModel{
		Contests:   make([]corev1.ContestModel, len(contestsList.Contests)),
		Pagination: PaginationDTO(contestsList.Pagination),
	}

	for i, contest := range contestsList.Contests {
		resp.Contests[i] = ContestDTO(*contest)
	}

	return &resp
}

func GetContestProblemResponseDTO(p *models.ContestProblem) *corev1.GetContestProblemResponseModel {
	resp := corev1.GetContestProblemResponseModel{
		Problem: corev1.ContestProblemModel{
			ProblemId:   p.ProblemId,
			Title:       p.Title,
			MemoryLimit: p.MemoryLimit,
			TimeLimit:   p.TimeLimit,

			Position: p.Position,

			LegendHtml:       p.LegendHtml,
			InputFormatHtml:  p.InputFormatHtml,
			OutputFormatHtml: p.OutputFormatHtml,
			NotesHtml:        p.NotesHtml,
			ScoringHtml:      p.ScoringHtml,

			CreatedAt: p.CreatedAt,
			UpdatedAt: p.UpdatedAt,
		},
	}

	return &resp
}

func PaginationDTO(p models.Pagination) corev1.PaginationModel {
	return corev1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
	}
}

func SubmissionsListToDTO(solutionsList *models.SolutionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		Submissions: make([]corev1.SubmissionsListItemModel, len(solutionsList.Solutions)),
		Pagination:  PaginationDTO(solutionsList.Pagination),
	}

	for i, solution := range solutionsList.Solutions {
		resp.Submissions[i] = SubmissionListItemDTO(*solution)
	}

	return &resp
}

func SubmissionListItemDTO(s models.SubmissionListItem) corev1.SubmissionsListItemModel {
	return corev1.SubmissionsListItemModel{
		Id:           s.Id,
		UserId:       s.CreatedBy,
		Username:     s.Username,
		State:        int64(s.State),
		Score:        s.Score,
		Penalty:      s.Penalty,
		TimeStat:     s.TimeStat,
		MemoryStat:   s.MemoryStat,
		Language:     int64(s.Language),
		ProblemId:    s.ProblemId,
		ProblemTitle: s.ProblemTitle,
		Position:     s.Position,
		ContestId:    s.ContestId,
		ContestTitle: s.ContestTitle,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

func ContestDTO(c models.Contest) corev1.ContestModel {
	return corev1.ContestModel{
		Id:                     c.Id,
		Title:                  c.Title,
		Description:            c.Description,
		Visibility:             c.Visibility,
		MonitorScope:           c.MonitorScope,
		SubmissionsListScope:   c.SubmissionsListScope,
		SubmissionsReviewScope: c.SubmissionsReviewScope,
		CreatedBy:              c.CreatedBy,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
	}
}

func ContestProblemsListItemDTO(t models.ContestProblemsListItem) corev1.ContestProblemListItemModel {
	return corev1.ContestProblemListItemModel{
		ProblemId:   t.ProblemId,
		Position:    t.Position,
		Title:       t.Title,
		MemoryLimit: t.MemoryLimit,
		TimeLimit:   t.TimeLimit,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func UserDTO(u models.User) corev1.UserModel {
	return corev1.UserModel{
		Id:        u.Id,
		Username:  u.Username,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func ParticipantDTO(p models.ContestMember) corev1.UserModel {
	return corev1.UserModel{
		// UserId:    p.UserId,
		// ContestId: p.ContestId,
		Id:        p.UserId,
		Username:  p.Username,
		Role:      p.Role,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
