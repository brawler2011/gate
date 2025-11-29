package contests

import (
	"context"
	"unicode/utf8"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain"
	"github.com/gate149/core/internal/middleware"
	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/permissions"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type ContestsUC interface {
	CreateContest(ctx context.Context, c *models.CreateContestInput) (uuid.UUID, error)
	GetContest(ctx context.Context, id uuid.UUID) (domain.Contest, error)
	UpdateContest(ctx context.Context, c models.ContestUpdate) error
	DeleteContest(ctx context.Context, id uuid.UUID) error

	ListAdminContests(ctx context.Context, filter models.AdminContestsFilter) (*domain.ContestsList, error)
	ListUserContests(ctx context.Context, filter models.UserContestsFilter) (*domain.ContestsList, error)
	ListWorkshopContests(ctx context.Context, filter models.WorkshopContestsFilter) (*domain.ContestsList, error)
	ListPublicContests(ctx context.Context, filter models.PublicContestsFilter) (*domain.ContestsList, error)

	CreateContestProblem(ctx context.Context, c models.ContestProblemCreation) error
	GetContestProblem(ctx context.Context, c models.ContestProblemGet) (domain.ContestProblem, error)
	GetContestProblems(ctx context.Context, contestId uuid.UUID) ([]domain.ContestProblem, error)
	DeleteContestProblem(ctx context.Context, c models.ContestProblemDeletion) error

	CreateParticipant(ctx context.Context, c models.ParticipantCreation) error
	DeleteParticipant(ctx context.Context, c models.ParticipantDeletion) error
	ListParticipants(ctx context.Context, filter models.ParticipantsFilter) (*domain.ContestMembersList, error)

	GetContestMember(ctx context.Context, c *models.ContestPermissionGet) (domain.ContestMember, error)
	UpdateContestMember(ctx context.Context, contestId uuid.UUID, userId uuid.UUID, role string) error
}

type SubmissionsUC interface {
	ListSolutions(ctx context.Context, filter models.SolutionsFilter) (*domain.SubmissionsList, error)
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

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	contestCreation := &models.CreateContestInput{
		Title:  params.Title,
		UserId: user.ID,
	}

	contestID, err := h.contestsUC.CreateContest(ctx, contestCreation)
	if err != nil {
		return err
	}

	return c.JSON(&corev1.CreationResponseModel{Id: contestID})
}

func (h *ContestsHandlers) GetContest(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	// Allow unauthenticated access for public contests
	user := middleware.GetUserOrAnonymous(ctx)

	// Get contest first to pass to permissions check
	contest, err := h.contestsUC.GetContest(ctx, id)
	if err != nil {
		return err
	}

	// Check if user can view contest (includes public contest check)
	canView, err := h.permissionsUC.HasContestPermission(ctx, id, user.ID, permissions.ActionGetContest,
		permissions.WithUser(user),
		permissions.WithContest(contest)) // Need to fix this later if WithContest expects domain.Contest
	if err != nil {
		return err
	}
	if !canView {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
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

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, id, user.ID, permissions.ActionUpdateContest, permissions.WithUser(user))
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

	canAdmin, err := h.permissionsUC.HasContestPermission(ctx, id, user.ID, permissions.ActionAdminContest, permissions.WithUser(user))
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

func validateListContestsParams(page, pageSize int64, search *string) error {
	if page < 1 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page must be greater than 0")
	}

	if pageSize < 1 || pageSize > 100 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "page size must be between 1 and 100")
	}

	if search != nil && !checkLength(*search, 0, 64) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "search length must be less than 64 characters")
	}

	return nil
}

func (h *ContestsHandlers) ListAdminContests(c *fiber.Ctx, params corev1.ListAdminContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	if user.Role != "admin" {
		return pkg.Wrap(pkg.NoPermission, nil, "admin access required")
	}

	var visibility *string
	if params.Visibility != nil {
		v := string(*params.Visibility)
		visibility = &v
	}
	var sortBy *string
	if params.SortBy != nil {
		s := string(*params.SortBy)
		sortBy = &s
	}
	var sortOrder *string
	if params.SortOrder != nil {
		s := string(*params.SortOrder)
		sortOrder = &s
	}

	filter := models.AdminContestsFilter{
		Page:       params.Page,
		PageSize:   params.PageSize,
		Search:     params.Search,
		Visibility: visibility,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}

	contestsList, err := h.contestsUC.ListAdminContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListContestsResponseDTO(contestsList))
}

func (h *ContestsHandlers) ListUserContests(c *fiber.Ctx, id uuid.UUID, params corev1.ListUserContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	// Only allow user to see their own contests or admins
	if id != user.ID && user.Role != "admin" {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	var sortBy *string
	if params.SortBy != nil {
		s := string(*params.SortBy)
		sortBy = &s
	}
	var sortOrder *string
	if params.SortOrder != nil {
		s := string(*params.SortOrder)
		sortOrder = &s
	}

	filter := models.UserContestsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserId:    id,
		Search:    params.Search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListUserContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListUserContestsResponseDTO(contestsList))
}

func (h *ContestsHandlers) ListWorkshopContests(c *fiber.Ctx, params corev1.ListWorkshopContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	var sortBy *string
	if params.SortBy != nil {
		s := string(*params.SortBy)
		sortBy = &s
	}
	var sortOrder *string
	if params.SortOrder != nil {
		s := string(*params.SortOrder)
		sortOrder = &s
	}

	filter := models.WorkshopContestsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserId:    user.ID,
		Search:    params.Search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListWorkshopContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListContestsResponseDTO(contestsList))
}

func (h *ContestsHandlers) ListPublicContests(c *fiber.Ctx, params corev1.ListPublicContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	var sortBy *string
	if params.SortBy != nil {
		s := string(*params.SortBy)
		sortBy = &s
	}
	var sortOrder *string
	if params.SortOrder != nil {
		s := string(*params.SortOrder)
		sortOrder = &s
	}

	filter := models.PublicContestsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		Search:    params.Search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListPublicContests(ctx, filter)
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

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionUpdateContest, permissions.WithUser(user))
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

	// Allow unauthenticated access for public contests
	user := middleware.GetUserOrAnonymous(ctx)

	// Check if user can view contest - this includes public contests
	canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionGetContest, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canView {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest problem")
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

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionUpdateContest, permissions.WithUser(user))
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

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionUpdateContest, permissions.WithUser(user))
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

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionUpdateContest, permissions.WithUser(user))
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

func (h *ContestsHandlers) UpdateContestMember(c *fiber.Ctx, contestId uuid.UUID, params corev1.UpdateContestMemberParams) error {
	ctx := c.UserContext()

	user, err := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	userId, err := uuid.Parse(params.UserId.String())
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid user_id")
	}

	// Prevent user from updating their own role
	if userId == user.ID {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
	}

	// Check if user is contest owner or global admin
	// First check if user is global admin
	isAdmin := user.Role == "admin"

	// If not admin, check if user is contest owner
	if !isAdmin {
		contestMember, err := h.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
			ContestId: contestId,
			UserId:    user.ID,
		})
		if err != nil {
			return pkg.Wrap(pkg.NoPermission, err, "insufficient permission to update contest member")
		}
		if contestMember.Role != models.ContestRoleOwner {
			return pkg.Wrap(pkg.NoPermission, nil, "only contest owner or admin can update contest member role")
		}
	}

	// Validate role value
	if params.Role != models.ContestRoleOwner && params.Role != models.ContestRoleModerator && params.Role != models.ContestRoleParticipant {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	// Call usecase to update the member
	err = h.contestsUC.UpdateContestMember(ctx, contestId, userId, params.Role)
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
	canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionGetMonitor, permissions.WithUser(user))
	if err != nil {
		return err
	}
	if !canView {
		canView, err = h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
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
		Members:    make([]corev1.ContestMemberModel, len(participantsList.Members)),
		Pagination: PaginationDTO(participantsList.Pagination),
	}

	for i, user := range participantsList.Members {
		resp.Members[i] = corev1.ContestMemberModel{
			ContestId:   contestId,
			ContestRole: user.ContestRole,
			UserId:      user.UserID,
			KratosId:    user.KratosID,
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
		UserId:    user.ID,
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

	// Allow unauthenticated access for public contests
	user := middleware.GetUserOrAnonymous(ctx)

	// Determine which userId to filter by and check permissions
	var filterUserId *uuid.UUID

	if params.UserId == nil {
		// No userId specified - request to get all members' submissions
		// Need permission to list all users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionListUsersSubmissions, permissions.WithUser(user))
		if err != nil {
			return err
		}
		if !canList {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to list all contest submissions")
		}
		// Don't filter by userId - get all submissions
		filterUserId = nil
	} else if *params.UserId == user.ID {
		// UserId matches current user - check permission to view own submissions
		canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionListOwnSubmissions, permissions.WithUser(user))
		if err != nil {
			return err
		}
		if !canView {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
		filterUserId = &user.ID
	} else {
		// UserId is different - need permission to list other users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.ID, permissions.ActionListUsersSubmissions, permissions.WithUser(user))
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

func GetContestResponseDTO(contest domain.Contest, problems []domain.ContestProblem) *corev1.GetContestResponseModel {
	resp := corev1.GetContestResponseModel{
		Contest:  ContestDTO(contest),
		Problems: make([]corev1.ContestProblemListItemModel, len(problems)),
	}

	for i, task := range problems {
		resp.Problems[i] = ContestProblemsListItemDTO(task)
	}

	return &resp
}

func ListContestsResponseDTO(contestsList *domain.ContestsList) *corev1.ListContestsResponseModel {
	resp := corev1.ListContestsResponseModel{
		Contests:   make([]corev1.ContestModel, len(contestsList.Contests)),
		Pagination: PaginationDTO(contestsList.Pagination),
	}

	for i, contest := range contestsList.Contests {
		resp.Contests[i] = ContestDTO(contest)
	}

	return &resp
}

func ListUserContestsResponseDTO(contestsList *domain.ContestsList) *corev1.ListUserContestsResponseModel {
	resp := corev1.ListUserContestsResponseModel{
		Contests:   make([]corev1.ContestModel, len(contestsList.Contests)),
		Pagination: PaginationDTO(contestsList.Pagination),
	}

	for i, contest := range contestsList.Contests {
		resp.Contests[i] = ContestDTO(contest)
	}

	return &resp
}

func GetContestProblemResponseDTO(p domain.ContestProblem) *corev1.GetContestProblemResponseModel {
	return &corev1.GetContestProblemResponseModel{
		Problem: corev1.ContestProblemModel{
			ProblemId:        p.ProblemID,
			Title:            p.Title,
			TimeLimit:        p.TimeLimit,
			MemoryLimit:      p.MemoryLimit,
			Position:         p.Position,
			LegendHtml:       p.LegendHtml,
			InputFormatHtml:  p.InputFormatHtml,
			OutputFormatHtml: p.OutputFormatHtml,
			NotesHtml:        p.NotesHtml,
			ScoringHtml:      p.ScoringHtml,
			CreatedAt:        p.CreatedAt,
			UpdatedAt:        p.UpdatedAt,
		},
	}
}

func PaginationDTO(p domain.Pagination) corev1.PaginationModel {
	return corev1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
	}
}

func SubmissionsListToDTO(solutionsList *domain.SubmissionsList) *corev1.ListSubmissionsResponseModel {
	resp := corev1.ListSubmissionsResponseModel{
		Submissions: make([]corev1.SubmissionsListItemModel, len(solutionsList.Submissions)),
		Pagination:  PaginationDTO(solutionsList.Pagination),
	}

	for i, solution := range solutionsList.Submissions {
		resp.Submissions[i] = SubmissionListItemDTO(solution)
	}

	return &resp
}

func SubmissionListItemDTO(s domain.Submission) corev1.SubmissionsListItemModel {
	return corev1.SubmissionsListItemModel{
		Id:           s.ID,
		UserId:       s.CreatedBy,
		Username:     s.Username,
		State:        int64(s.State),
		Score:        s.Score,
		Penalty:      s.Penalty,
		TimeStat:     s.TimeStat,
		MemoryStat:   s.MemoryStat,
		Language:     int64(s.Language),
		ProblemId:    s.ProblemID,
		ProblemTitle: s.ProblemTitle,
		Position:     s.Position,
		ContestId:    s.ContestID,
		ContestTitle: s.ContestTitle,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

func ContestDTO(c domain.Contest) corev1.ContestModel {
	return corev1.ContestModel{
		Id:                     c.ID,
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

func ContestProblemsListItemDTO(t domain.ContestProblem) corev1.ContestProblemListItemModel {
	return corev1.ContestProblemListItemModel{
		ProblemId:   t.ProblemID,
		Position:    t.Position,
		Title:       t.Title,
		MemoryLimit: t.MemoryLimit,
		TimeLimit:   t.TimeLimit,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func UserDTO(u domain.User) corev1.UserModel {
	return corev1.UserModel{
		Id:        u.ID,
		Username:  u.Username,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func ParticipantDTO(p domain.ContestMember) corev1.UserModel {
	return corev1.UserModel{
		Id:        p.UserID,
		Username:  p.Username,
		Role:      p.Role,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
