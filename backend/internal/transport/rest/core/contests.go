package core

import (
	"context"
	"log/slog"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateContest(ctx context.Context, request corev1.CreateContestRequestObject) (corev1.CreateContestResponseObject, error) {
	err := validateCreateContestParams(request.Params)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)

	var requestedOrgID *uuid.UUID
	if request.Params.OrganizationId != nil {
		requested := uuid.UUID(*request.Params.OrganizationId)
		requestedOrgID = &requested
	}

	orgID, err := h.organizationsUC.ResolveUserOrganizationID(ctx, user.Id, requestedOrgID)
	if err != nil {
		return nil, err
	}

	// Generate short_name from UUID to ensure uniqueness
	shortName := "contest-" + uuid.New().String()[:8]

	contestCreation := &models.CreateContestInput{
		OrganizationID: orgID,
		OwnerID:        &user.Id,
		Title:          request.Params.Title,
		ShortName:      shortName,
		Description:    "",
		Visibility:     models.ContestVisibilityPrivate,
		Settings:       make(map[string]interface{}),
		AccessPolicy:   models.DefaultContestAccessPolicy(),
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
	contest, err := h.contestsUC.GetContest(ctx, request.ContestId)
	if err != nil {
		return nil, err
	}

	user := middleware.GetUser(ctx)
	allowed, err := h.permissionsUC.HasContestPermission(ctx, request.ContestId, user.Id, models.ActionGetContestProblem)
	if err != nil {
		return nil, err
	}

	var ps []models.ContestProblem
	if allowed {
		ps, err = h.contestsUC.GetContestProblems(ctx, request.ContestId)
		if err != nil {
			return nil, err
		}
	}

	problemDetails := make(map[uuid.UUID]models.Problem, len(ps))
	for _, contestProblem := range ps {
		problem, err := h.problemsUC.GetProblemById(ctx, contestProblem.ProblemID)
		if err != nil {
			return nil, err
		}

		problemDetails[contestProblem.ProblemID] = problem
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

	return corev1.GetContest200JSONResponse(*GetContestResponseDTO(contest, ps, problemDetails, &owner)), nil
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

	var settings *map[string]interface{}
	if req.MonitorScope != nil || req.SubmissionsListScope != nil || req.SubmissionsReviewScope != nil {
		s := make(map[string]interface{})
		if req.MonitorScope != nil {
			s["monitor_scope"] = *req.MonitorScope
		}
		if req.SubmissionsListScope != nil {
			s["submissions_list_scope"] = *req.SubmissionsListScope
		}
		if req.SubmissionsReviewScope != nil {
			s["submissions_review_scope"] = *req.SubmissionsReviewScope
		}
		settings = &s
	}

	err = h.contestsUC.UpdateContest(ctx, models.ContestUpdateInput{
		ID:           request.ContestId,
		Title:        req.Title,
		Description:  req.Description,
		Visibility:   req.Visibility,
		Settings:     settings,
		AccessPolicy: nil,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		OwnerID:      nil,
	})
	if err != nil {
		return nil, err
	}

	return corev1.UpdateContest200Response{}, nil
}

func (h *CoreServer) DeleteContest(ctx context.Context, request corev1.DeleteContestRequestObject) (corev1.DeleteContestResponseObject, error) {
	err := h.contestsUC.DeleteContest(ctx, request.ContestId)
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

	if request.Params.OrganizationId != nil {
		orgID, err := uuid.Parse(request.Params.OrganizationId.String())
		if err == nil {
			filter.OrganizationID = &orgID
		}
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
	err := h.contestsUC.CreateContestProblem(ctx, models.ContestProblemCreation{
		ContestId: request.ContestId,
		ProblemId: request.Params.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestProblem200JSONResponse{}, nil
}

func (h *CoreServer) GetContestProblem(ctx context.Context, request corev1.GetContestProblemRequestObject) (corev1.GetContestProblemResponseObject, error) {
	p, err := h.contestsUC.GetContestProblem(ctx, models.ContestProblemGet{
		ContestId: request.ContestId,
		ProblemId: request.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	problem, err := h.problemsUC.GetProblemById(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	statement := h.loadProblemStatement(ctx, request.ProblemId)

	return corev1.GetContestProblem200JSONResponse(*GetContestProblemResponseDTO(p, problem, statement)), nil
}

func (h *CoreServer) DeleteContestProblem(ctx context.Context, request corev1.DeleteContestProblemRequestObject) (corev1.DeleteContestProblemResponseObject, error) {
	// Delete the problem
	err := h.contestsUC.DeleteContestProblem(ctx, models.ContestProblemDeletion{
		ContestId: request.ContestId,
		ProblemId: request.ProblemId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.DeleteContestProblem200Response{}, nil
}

func (h *CoreServer) CreateContestMember(ctx context.Context, request corev1.CreateContestMemberRequestObject) (corev1.CreateContestMemberResponseObject, error) {
	err := h.contestsUC.CreateParticipant(ctx, models.ParticipantCreation{
		ContestId: request.ContestId,
		UserId:    request.Params.UserId,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestMember200JSONResponse{}, nil
}

func (h *CoreServer) DeleteContestMember(ctx context.Context, request corev1.DeleteContestMemberRequestObject) (corev1.DeleteContestMemberResponseObject, error) {
	err := h.contestsUC.DeleteParticipant(ctx, models.ParticipantDeletion{
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

	userId, err := uuid.Parse(request.Params.UserId.String())
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid user_id")
	}

	// Prevent user from updating their own role
	if userId == user.Id {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "cannot update own role")
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

	contestRole, permissionsMask, err := h.permissionsUC.GetEffectiveContestRole(ctx, request.ContestId, user.Id)
	if err != nil {
		return nil, err
	}

	if contestRole == nil {
		return corev1.GetMyContestRole200JSONResponse{}, nil
	}

	permissionsMaskValue := int64(permissionsMask)

	return corev1.GetMyContestRole200JSONResponse{
		Role:            *contestRole,
		PermissionsMask: &permissionsMaskValue,
	}, nil
}

func (h *CoreServer) ListContestSubmissions(ctx context.Context, request corev1.ListContestSubmissionsRequestObject) (corev1.ListContestSubmissionsResponseObject, error) {
	filterUserId := request.Params.UserId

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

	var since int64
	lastSeq, seqErr := pkg.GetSubmissionsLastSequence(ctx, h.natsJS)
	if seqErr != nil {
		slog.Warn("failed to get submissions last sequence", "error", seqErr)
	} else {
		since = int64(lastSeq)
	}

	resp := *ListSolutionsResponseDTO(submissionsList)
	resp.Since = &since

	return corev1.ListContestSubmissions200JSONResponse(resp), nil
}
