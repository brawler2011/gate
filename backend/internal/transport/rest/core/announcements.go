package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) ListContestAnnouncements(ctx context.Context, request corev1.ListContestAnnouncementsRequestObject) (corev1.ListContestAnnouncementsResponseObject, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
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

	list, err := h.announcementsUC.ListAnnouncements(ctx, contest.ID, page, pageSize)
	if err != nil {
		return nil, err
	}

	return corev1.ListContestAnnouncements200JSONResponse(*ContestAnnouncementsListResponseDTO(list)), nil
}

func (h *CoreServer) CreateContestAnnouncement(ctx context.Context, request corev1.CreateContestAnnouncementRequestObject) (corev1.CreateContestAnnouncementResponseObject, error) {
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
		return nil, pkg.Wrap(pkg.NoPermission, nil, "permission denied: only contest moderators can create announcements")
	}

	body := request.Body
	if body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	announcement, err := h.announcementsUC.CreateAnnouncement(ctx, &models.CreateContestAnnouncementInput{
		ContestID: contest.ID,
		ProblemID: body.ProblemId,
		AuthorID:  user.Id,
		Title:     body.Title,
		Body:      body.Body,
	})
	if err != nil {
		return nil, err
	}

	return corev1.CreateContestAnnouncement200JSONResponse(*ContestAnnouncementDTO(announcement)), nil
}

func (h *CoreServer) DeleteContestAnnouncement(ctx context.Context, request corev1.DeleteContestAnnouncementRequestObject) (corev1.DeleteContestAnnouncementResponseObject, error) {
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
		return nil, pkg.Wrap(pkg.NoPermission, nil, "permission denied: only contest moderators can delete announcements")
	}

	if err := h.announcementsUC.DeleteAnnouncement(ctx, request.AnnouncementId, contest.ID); err != nil {
		return nil, err
	}

	return corev1.DeleteContestAnnouncement200Response{}, nil
}
