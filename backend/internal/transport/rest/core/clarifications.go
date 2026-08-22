package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) ListContestClarifications(ctx context.Context, request corev1.ListContestClarificationsRequestObject) (corev1.ListContestClarificationsResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	isModerator, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	if err != nil {
		return nil, err
	}

	page := int32(1)
	if request.Params.Page != nil && *request.Params.Page > 0 {
		page = *request.Params.Page
	}
	pageSize := int32(50)
	if request.Params.PageSize != nil && *request.Params.PageSize > 0 {
		pageSize = *request.Params.PageSize
	}

	var list *models.ContestClarificationsList
	if isModerator {
		filter := &models.ContestClarificationsFilter{
			ProblemID: request.Params.ProblemId,
			Status:    request.Params.Status,
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

	return corev1.ListContestClarifications200JSONResponse(*ContestClarificationsListResponseDTO(list)), nil
}

func (h *CoreServer) CreateContestClarification(ctx context.Context, request corev1.CreateContestClarificationRequestObject) (corev1.CreateContestClarificationResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	clarification, err := h.clarificationsUC.CreateClarification(ctx, &models.CreateContestClarificationInput{
		ContestID: contest.ID,
		ProblemID: body.ProblemId,
		UserID:    user.Id,
		Question:  body.Question,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestClarification200JSONResponse(*ContestClarificationDTO(clarification)), nil
}

func (h *CoreServer) AnswerContestClarification(ctx context.Context, request corev1.AnswerContestClarificationRequestObject) (corev1.AnswerContestClarificationResponseObject, error) {
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
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

	body := request.Body
	if body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	publishAsAnnouncement := false
	if body.PublishAsAnnouncement != nil {
		publishAsAnnouncement = *body.PublishAsAnnouncement
	}
	announcementTitle := ""
	if body.AnnouncementTitle != nil {
		announcementTitle = *body.AnnouncementTitle
	}

	clarification, err := h.clarificationsUC.AnswerClarification(ctx, &models.AnswerContestClarificationInput{
		ClarificationID:       request.ClarificationId,
		ContestID:             contest.ID,
		Answer:                body.Answer,
		AnsweredBy:            user.Id,
		PublishAsAnnouncement: publishAsAnnouncement,
		AnnouncementTitle:     announcementTitle,
	})
	if err != nil {
		return nil, err
	}

	return corev1.AnswerContestClarification200JSONResponse(*ContestClarificationDTO(clarification)), nil
}
