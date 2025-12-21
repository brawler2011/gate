package handlers

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateContest(c *fiber.Ctx, params corev1.CreateContestParams) error {
	ctx := c.UserContext()

	err := validateCreateContestParams(params)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)
	if err != nil {
		return err
	}

	contestCreation := &models.CreateContestInput{
		Title:  params.Title,
		UserId: user.Id,
	}

	contestID, err := h.contestsUC.CreateContest(ctx, contestCreation)
	if err != nil {
		return err
	}

	return c.JSON(&corev1.CreationResponseModel{Id: contestID})
}

func (h *CoreServer) GetContest(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)

	contest, err := h.contestsUC.GetContest(ctx, id)
	if err != nil {
		return err
	}

	canView, err := h.permissionsUC.HasContestPermission(ctx, id, user.Id, models.ActionGetContest)
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

	owner, err := h.usersUC.GetUserById(ctx, contest.CreatedBy)
	if err != nil {
		return err
	}

	return c.JSON(GetContestResponseDTO(contest, ps, &owner))
}

func (h *CoreServer) UpdateContest(c *fiber.Ctx, id uuid.UUID) error {
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

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, id, user.Id, models.ActionUpdateContest)
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.UpdateContest(ctx, models.ContestUpdateInput{
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

func (h *CoreServer) DeleteContest(c *fiber.Ctx, id uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canAdmin, err := h.permissionsUC.HasContestPermission(ctx, id, user.Id, models.ActionAdminContest)
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

func (h *CoreServer) ListAdminContests(c *fiber.Ctx, params corev1.ListAdminContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	if !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "admin access required")
	}

	var visibility *string
	if params.Visibility != nil {
		v := string(*params.Visibility)
		visibility = &v
	}
	sortBy := "created_at"
	if params.SortBy != nil {
		sortBy = string(*params.SortBy)
	}
	sortOrder := models.SortOrderAsc
	if params.SortOrder != nil {
		s := string(*params.SortOrder)
		sortOrder = s
	}

	search := ""
	if params.Search != nil {
		search = *params.Search
	}

	filter := models.AdminContestsFilter{
		Page:       params.Page,
		PageSize:   params.PageSize,
		Search:     search,
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

func (h *CoreServer) ListUserContests(c *fiber.Ctx, id uuid.UUID, params corev1.ListUserContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	// Only allow user to see their own contests or admins
	if id != user.Id && !user.IsAdmin() {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	var sortBy string
	if params.SortBy != nil {
		sortBy = string(*params.SortBy)
	}
	var sortOrder string
	if params.SortOrder != nil {
		sortOrder = string(*params.SortOrder)
	}

	search := ""
	if params.Search != nil {
		search = *params.Search
	}

	filter := models.UserContestsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserId:    id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListUserContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListUserContestsResponseDTO(contestsList))
}

func (h *CoreServer) ListWorkshopContests(c *fiber.Ctx, params corev1.ListWorkshopContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	var sortBy string
	if params.SortBy != nil {
		sortBy = string(*params.SortBy)
	}
	var sortOrder string
	if params.SortOrder != nil {
		sortOrder = string(*params.SortOrder)
	}

	search := ""
	if params.Search != nil {
		search = *params.Search
	}

	filter := models.WorkshopContestsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListWorkshopContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListContestsResponseDTO(contestsList))
}

func (h *CoreServer) ListPublicContests(c *fiber.Ctx, params corev1.ListPublicContestsParams) error {
	ctx := c.UserContext()

	err := validateListContestsParams(params.Page, params.PageSize, params.Search)
	if err != nil {
		return err
	}

	var sortBy string
	if params.SortBy != nil {
		sortBy = string(*params.SortBy)
	}
	var sortOrder string
	if params.SortOrder != nil {
		sortOrder = string(*params.SortOrder)
	}

	search := ""
	if params.Search != nil {
		search = *params.Search
	}

	filter := models.PublicContestsFilter{
		Page:      params.Page,
		PageSize:  params.PageSize,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListPublicContests(ctx, filter)
	if err != nil {
		return err
	}

	return c.JSON(ListContestsResponseDTO(contestsList))
}

func (h *CoreServer) CreateContestProblem(c *fiber.Ctx, contestId uuid.UUID, params corev1.CreateContestProblemParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionUpdateContest)
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

func (h *CoreServer) GetContestProblem(c *fiber.Ctx, contestId uuid.UUID, problemId uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionGetContest)
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

func (h *CoreServer) DeleteContestProblem(c *fiber.Ctx, contestId uuid.UUID, problemId uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionUpdateContest)
	if err != nil {
		return err
	}
	if !canUpdate {
		return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	// Delete the problem
	err = h.contestsUC.DeleteContestProblem(ctx, models.ContestProblemDeletion{
		ContestId: contestId,
		ProblemId: problemId,
	})
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *CoreServer) CreateContestMember(c *fiber.Ctx, contestId uuid.UUID, params corev1.CreateContestMemberParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionUpdateContest)
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

func (h *CoreServer) DeleteContestMember(c *fiber.Ctx, contestId uuid.UUID, params corev1.DeleteContestMemberParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionUpdateContest)
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

func (h *CoreServer) UpdateContestMember(c *fiber.Ctx, contestId uuid.UUID, params corev1.UpdateContestMemberParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	userId, err := uuid.Parse(params.UserId.String())
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid user_id")
	}

	// Prevent user from updating their own role
	if userId == user.Id {
		return pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
	}

	// Check if user is contest owner or global admin
	// First check if user is global admin
	isAdmin := user.IsAdmin()

	// If not admin, check if user is contest owner
	if !isAdmin {
		contestMember, err := h.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
			ContestId: contestId,
			UserId:    user.Id,
		})
		if err != nil {
			return pkg.Wrap(pkg.NoPermission, err, "insufficient permission to update contest member")
		}
		if contestMember.ContestRole != models.ContestRoleOwner {
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

func (h *CoreServer) ListContestMembers(c *fiber.Ctx, contestId uuid.UUID, params corev1.ListContestMembersParams) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
	}

	canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionGetMonitor)
	if err != nil {
		return err
	}

	if !canView {
		canView, err = h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionListOwnSubmissions)
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
			Role:        string(user.Role),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return c.JSON(resp)
}

func (h *CoreServer) GetMyContestRole(c *fiber.Ctx, contestId uuid.UUID) error {
	ctx := c.UserContext()

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.ErrUnauthenticated
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

func (h *CoreServer) ListContestSubmissions(c *fiber.Ctx, contestId uuid.UUID, params corev1.ListContestSubmissionsParams) error {
	ctx := c.UserContext()

	// Allow unauthenticated access for public contests
	user := middleware.GetUser(ctx)

	// Determine which userId to filter by and check permissions
	var filterUserId *uuid.UUID

	if params.UserId == nil {
		// No userId specified - request to get all members' submissions
		// Need permission to list all users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionListUsersSubmissions)
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
		canView, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionListOwnSubmissions)
		if err != nil {
			return err
		}
		if !canView {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
		filterUserId = &user.Id
	} else {
		// UserId is different - need permission to list other users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, contestId, user.Id, models.ActionListUsersSubmissions)
		if err != nil {
			return err
		}
		if !canList {
			return pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view other users' submissions")
		}
		filterUserId = params.UserId
	}

	var order *int32
	if params.SortOrder != nil {
		var orderVal int32
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

	submissionsList, err := h.submissionsUC.ListSubmissions(ctx, models.SubmissionsFilter{
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
