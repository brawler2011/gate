package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) ListContestClarifications(ctx context.Context, params corev1.ListContestClarificationsParams) (*corev1.ListContestClarificationsResponseModel, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	isModerator, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	if err != nil {
		return nil, err
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(50)

	var list *models.ContestClarificationsList
	if isModerator {
		var problemID *uuid.UUID
		if params.ProblemID.IsSet() {
			problemID = &params.ProblemID.Value
		}
		var status *string
		if params.Status.IsSet() {
			status = &params.Status.Value
		}
		filter := &models.ContestClarificationsFilter{
			ProblemID: problemID,
			Status:    status,
			Page:      page,
			PageSize:  pageSize,
		}
		list, err = h.clarificationsUC.ListClarificationsForModerator(ctx, contest.ID, filter)
	} else {
		list, err = h.clarificationsUC.ListClarificationsForUser(ctx, contest.ID, user.Id, page, pageSize)
	}
	if err != nil {
		return nil, err
	}

	return ContestClarificationsListResponseDTO(list), nil
}

func (h *CoreServer) CreateContestClarification(ctx context.Context, req *corev1.CreateContestClarificationRequestModel, params corev1.CreateContestClarificationParams) (*corev1.ContestClarificationModel, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	var problemID *uuid.UUID
	if req.ProblemID.IsSet() {
		problemID = &req.ProblemID.Value
	}

	clarification, err := h.clarificationsUC.CreateClarification(ctx, &models.CreateContestClarificationInput{
		ContestID: contest.ID,
		ProblemID: problemID,
		UserID:    user.Id,
		Question:  req.Question,
	})
	if err != nil {
		return nil, err
	}

	return ContestClarificationDTO(clarification), nil
}

func (h *CoreServer) AnswerContestClarification(ctx context.Context, req *corev1.AnswerContestClarificationRequestModel, params corev1.AnswerContestClarificationParams) (*corev1.ContestClarificationModel, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "permission denied: only contest moderators can answer clarifications")
	}

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	publishAsAnnouncement := req.PublishAsAnnouncement.Or(false)
	announcementTitle := req.AnnouncementTitle.Or("")

	clarification, err := h.clarificationsUC.AnswerClarification(ctx, &models.AnswerContestClarificationInput{
		ClarificationID:       params.ClarificationID,
		ContestID:             contest.ID,
		Answer:                req.Answer,
		AnsweredBy:            user.Id,
		PublishAsAnnouncement: publishAsAnnouncement,
		AnnouncementTitle:     announcementTitle,
	})
	if err != nil {
		return nil, err
	}

	return ContestClarificationDTO(clarification), nil
}
