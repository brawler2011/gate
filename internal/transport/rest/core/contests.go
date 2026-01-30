package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/gate149/core/internal/transport/middleware"
	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateContest(ctx context.Context, request corev1.CreateContestRequestObject) (corev1.CreateContestResponseObject, error) {
	err := validateCreateContestParams(request.Params)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	// Create titles map from single title
	titles := make(map[string]string)
	if request.Params.Title != "" {
		titles["en"] = request.Params.Title
	}
	
	contestCreation := &models.CreateContestInput{
		OrganizationID: user.Id, // TODO: get proper organization ID
		OwnerID:        &user.Id,
		Titles:         titles,
		ShortName:      "",      // TODO: generate from title
		Description:    "",
		Visibility:     models.ContestVisibilityPrivate,
		Settings:       make(map[string]interface{}),
		AccessPolicy:   make(map[string]interface{}),
		StartTime:      nil,
		EndTime:        nil,
	}

	contestID, err := h.contestsUC.CreateContest(ctx, contestCreation)
	if err != nil {
		return nil, err
	}

	return corev1.CreateContest200JSONResponse{Id: contestID}, nil
}

func (h *CoreServer) GetContest(ctx context.Context, request corev1.GetContestRequestObject) (corev1.GetContestResponseObject, error) {
	user := middleware.GetUser(ctx)

	contest, err := h.contestsUC.GetContest(ctx, request.ContestId)
	if err != nil {
		return nil, err
	}

	canView, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionGetContest)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
	}

	ps, err := h.contestsUC.GetContestProblems(ctx, request.ContestId)
	if err != nil {
		return nil, err
	}

	var owner models.User
	if contest.OwnerID != nil {
		owner, err = h.usersUC.GetUserById(ctx, *contest.OwnerID)
	} else {
		// Use default user if no owner
		owner = models.User{}
		err = nil
	}
	if err != nil {
		return nil, err
	}

	return corev1.GetContest200JSONResponse(*GetContestResponseDTO(contest, ps, &owner)), nil
}

func (h *CoreServer) UpdateContest(ctx context.Context, request corev1.UpdateContestRequestObject) (corev1.UpdateContestResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}
	req := *request.Body

	err := validateUpdateContestRequest(req)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionUpdateContest)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	// Build titles map if Title is provided
	var titles *map[string]string
	if req.Title != nil {
		t := make(map[string]string)
		t["en"] = *req.Title
		titles = &t
	}
	
	err = h.contestsUC.UpdateContest(ctx, models.ContestUpdateInput{
		ID:           request.ContestId,
		Titles:       titles,
		Description:  req.Description,
		Visibility:   req.Visibility,
		Settings:     nil, // TODO: map from old fields
		AccessPolicy: nil, // TODO: map from old fields
		StartTime:    nil,
		EndTime:      nil,
		OwnerID:      nil,
	})
	if err != nil {
		return nil, err
	}

	return corev1.UpdateContest200Response{}, nil
}

func (h *CoreServer) DeleteContest(ctx context.Context, request corev1.DeleteContestRequestObject) (corev1.DeleteContestResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canAdmin, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionAdminContest)
	if err != nil {
		return nil, err
	}
	if !canAdmin {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to delete contest")
	}

	err = h.contestsUC.DeleteContest(ctx, request.ContestId)
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContest200Response{}, nil
}

func (h *CoreServer) ListAdminContests(ctx context.Context, request corev1.ListAdminContestsRequestObject) (corev1.ListAdminContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	if !user.IsAdmin() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "admin access required")
	}

	var visibility *string
	if request.Params.Visibility != nil {
		v := string(*request.Params.Visibility)
		visibility = &v
	}
	sortBy := "created_at"
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	sortOrder := models.SortOrderAsc
	if request.Params.SortOrder != nil {
		s := string(*request.Params.SortOrder)
		sortOrder = s
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.AdminContestsFilter{
		Page:       request.Params.Page,
		PageSize:   request.Params.PageSize,
		Search:     search,
		Visibility: visibility,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}

	contestsList, err := h.contestsUC.ListAdminContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListAdminContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) ListUserContests(ctx context.Context, request corev1.ListUserContestsRequestObject) (corev1.ListUserContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	// Only allow user to see their own contests or admins
	if request.Id != user.Id && !user.IsAdmin() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view user contests")
	}

	var sortBy string
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	var sortOrder string
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.UserContestsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		UserId:    request.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListUserContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListUserContests200JSONResponse(*ListUserContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) ListWorkshopContests(ctx context.Context, request corev1.ListWorkshopContestsRequestObject) (corev1.ListWorkshopContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	var sortBy string
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	var sortOrder string
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.WorkshopContestsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		UserId:    user.Id,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListWorkshopContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListWorkshopContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) ListPublicContests(ctx context.Context, request corev1.ListPublicContestsRequestObject) (corev1.ListPublicContestsResponseObject, error) {
	err := validateListContestsParams(request.Params.Page, request.Params.PageSize, request.Params.Search)
	if err != nil {
		return nil, err
	}

	var sortBy string
	if request.Params.SortBy != nil {
		sortBy = string(*request.Params.SortBy)
	}
	var sortOrder string
	if request.Params.SortOrder != nil {
		sortOrder = string(*request.Params.SortOrder)
	}

	search := ""
	if request.Params.Search != nil {
		search = *request.Params.Search
	}

	filter := models.PublicContestsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	contestsList, err := h.contestsUC.ListPublicContests(ctx, filter)
	if err != nil {
		return nil, err
	}

	return corev1.ListPublicContests200JSONResponse(*ListContestsResponseDTO(contestsList)), nil
}

func (h *CoreServer) CreateContestProblem(ctx context.Context, request corev1.CreateContestProblemRequestObject) (corev1.CreateContestProblemResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionUpdateContest)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.CreateContestProblem(ctx, models.ContestProblemCreation{
		ContestId: request.ContestId,
		ProblemId: request.Params.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestProblem200JSONResponse{}, nil
}

func (h *CoreServer) GetContestProblem(ctx context.Context, request corev1.GetContestProblemRequestObject) (corev1.GetContestProblemResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canView, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionGetContest)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest problem")
	}

	p, err := h.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
		ContestId: request.ContestId,
		ProblemId: request.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.GetContestProblem200JSONResponse(*GetContestProblemResponseDTO(p)), nil
}

func (h *CoreServer) DeleteContestProblem(ctx context.Context, request corev1.DeleteContestProblemRequestObject) (corev1.DeleteContestProblemResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionUpdateContest)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	// Delete the problem
	err = h.contestsUC.DeleteContestProblem(ctx, models.ContestProblemDeletion{
		ContestId: request.ContestId,
		ProblemId: request.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContestProblem200Response{}, nil
}

func (h *CoreServer) CreateContestMember(ctx context.Context, request corev1.CreateContestMemberRequestObject) (corev1.CreateContestMemberResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionUpdateContest)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.CreateParticipant(ctx, models.ParticipantCreation{
		ContestId: request.ContestId,
		UserId:    request.Params.UserId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestMember200JSONResponse{}, nil
}

func (h *CoreServer) DeleteContestMember(ctx context.Context, request corev1.DeleteContestMemberRequestObject) (corev1.DeleteContestMemberResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canUpdate, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionUpdateContest)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to edit contest")
	}

	err = h.contestsUC.DeleteParticipant(ctx, models.ParticipantDeletion{
		ContestId: request.ContestId,
		UserId:    request.Params.UserId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContestMember200Response{}, nil
}

func (h *CoreServer) UpdateContestMember(ctx context.Context, request corev1.UpdateContestMemberRequestObject) (corev1.UpdateContestMemberResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	userId, err := uuid.Parse(request.Params.UserId.String())
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid user_id")
	}

	// Prevent user from updating their own role
	if userId == user.Id {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
	}

	// Check if user is contest owner or global admin
	// First check if user is global admin
	isAdmin := user.IsAdmin()

	// If not admin, check if user is contest owner
	if !isAdmin {
		contestMember, err := h.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
			ContestId: request.ContestId,
			UserId:    user.Id,
		})
		if err != nil {
			return nil, pkg.Wrap(pkg.NoPermission, err, "insufficient permission to update contest member")
		}
		if contestMember.ContestRole != models.ContestRoleOwner {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "only contest owner or admin can update contest member role")
		}
	}

	// Validate role value
	if request.Params.Role != models.ContestRoleOwner && request.Params.Role != models.ContestRoleModerator && request.Params.Role != models.ContestRoleParticipant {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "invalid role value")
	}

	// Call usecase to update the member
	err = h.contestsUC.UpdateContestMember(ctx, request.ContestId, userId, request.Params.Role)
	if err != nil {
		return nil, err
	}

	return corev1.UpdateContestMember200Response{}, nil
}

func (h *CoreServer) ListContestMembers(ctx context.Context, request corev1.ListContestMembersRequestObject) (corev1.ListContestMembersResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	canView, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionGetMonitor)
	if err != nil {
		return nil, err
	}

	if !canView {
		canView, err = h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionListOwnSubmissions)
		if err != nil {
			return nil, err
		}
	}
	if !canView {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view contest")
	}

	participantsList, err := h.contestsUC.ListParticipants(ctx, models.ParticipantsFilter{
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		ContestId: request.ContestId,
	})
	if err != nil {
		return nil, err
	}

	resp := corev1.ListContestMembersResponseModel{
		Members:    make([]corev1.ContestMemberModel, len(participantsList.Members)),
		Pagination: PaginationDTO(participantsList.Pagination),
	}

	for i, user := range participantsList.Members {
		resp.Members[i] = corev1.ContestMemberModel{
			ContestId:   request.ContestId,
			ContestRole: user.ContestRole,
			UserId:      user.UserID,
			KratosId:    user.KratosID,
			Username:    user.Username,
			Role:        string(user.Role),
			CreatedAt:   user.CreatedAt,
			UpdatedAt:   user.UpdatedAt,
		}
	}

	return corev1.ListContestMembers200JSONResponse(resp), nil
}

func (h *CoreServer) GetMyContestRole(ctx context.Context, request corev1.GetMyContestRoleRequestObject) (corev1.GetMyContestRoleResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.ErrUnauthenticated
	}

	contestMember, err := h.contestsUC.GetContestMember(ctx, &models.ContestPermissionGet{
		ContestId: request.ContestId,
		UserId:    user.Id,
	})
	if err != nil {
		return nil, err
	}

	return corev1.GetMyContestRole200JSONResponse{
		Role: contestMember.Role,
	}, nil
}

func (h *CoreServer) ListContestSubmissions(ctx context.Context, request corev1.ListContestSubmissionsRequestObject) (corev1.ListContestSubmissionsResponseObject, error) {
	// Allow unauthenticated access for public contests
	user := middleware.GetUser(ctx)

	// Determine which userId to filter by and check permissions
	var filterUserId *uuid.UUID

	if request.Params.UserId == nil {
		// No userId specified - request to get all members' submissions
		// Need permission to list all users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionListUsersSubmissions)
		if err != nil {
			return nil, err
		}
		if !canList {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to list all contest submissions")
		}
		// Don't filter by userId - get all submissions
		filterUserId = nil
	} else if *request.Params.UserId == user.Id {
		// UserId matches current user - check permission to view own submissions
		canView, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionListOwnSubmissions)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view own submissions in this contest")
		}
		filterUserId = &user.Id
	} else {
		// UserId is different - need permission to list other users' submissions
		canList, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionListUsersSubmissions)
		if err != nil {
			return nil, err
		}
		if !canList {
			return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permission to view other users' submissions")
		}
		filterUserId = request.Params.UserId
	}

	var order *int32
	if request.Params.SortOrder != nil {
		var orderVal int32
		if *request.Params.SortOrder == corev1.ListContestSubmissionsParamsSortOrderDesc {
			orderVal = -1
		} else {
			orderVal = 0
		}
		order = &orderVal
	}

	var state *models.State
	if request.Params.State != nil {
		stateVal := models.State(*request.Params.State)
		state = &stateVal
	}

	var langName *models.LanguageName
	if request.Params.Language != nil {
		lang := models.LanguageName(*request.Params.Language)
		langName = &lang
	}

	submissionsList, err := h.submissionsUC.ListSubmissions(ctx, models.SubmissionsFilter{
		ContestId: &request.ContestId,
		Page:      request.Params.Page,
		PageSize:  request.Params.PageSize,
		UserId:    filterUserId,
		ProblemId: request.Params.ProblemId,
		Language:  langName,
		State:     state,
		Order:     order,
	})
	if err != nil {
		return nil, err
	}

	return corev1.ListContestSubmissions200JSONResponse(*ListSolutionsResponseDTO(submissionsList)), nil
}
