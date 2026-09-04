package core

import (
	"context"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) ListContestAnnouncements(ctx context.Context, params corev1.ListContestAnnouncementsParams) (*corev1.ListContestAnnouncementsResponseModel, error) {
	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	list, err := h.announcementsUC.ListAnnouncements(ctx, contest.ID, params.Page.Value, params.PageSize.Value)
	if err != nil {
		return nil, err
	}

	return ContestAnnouncementsListResponseDTO(list), nil
}

func (h *CoreServer) CreateContestAnnouncement(ctx context.Context, req *corev1.CreateContestAnnouncementRequestModel, params corev1.CreateContestAnnouncementParams) (*corev1.ContestAnnouncementModel, error) {
	// FIXME: должно быть в middleware, а не здесь
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	// FIXME: должно быть в middleware, а не здесь
	allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "permission denied: only contest moderators can create announcements")
	}

	// FIXME: create OptUUID struct instead of using pointer
	var problemID *uuid.UUID
	if req.ProblemID.IsSet() {
		problemID = &req.ProblemID.Value
	}

	announcement, err := h.announcementsUC.CreateAnnouncement(ctx, &models.CreateContestAnnouncementInput{
		ContestID: contest.ID,
		ProblemID: problemID,
		AuthorID:  user.Id,
		Title:     req.Title,
		Body:      req.Body,
	})
	if err != nil {
		return nil, err
	}

	return ContestAnnouncementDTO(announcement), nil
}

func (h *CoreServer) DeleteContestAnnouncement(ctx context.Context, params corev1.DeleteContestAnnouncementParams) error {
	// FIXME: должно быть в middleware, а не здесь
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	// FIXME: должно быть в middleware, а не здесь
	allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
	if err != nil {
		return err
	}
	if !allowed {
		return pkg.Wrap(pkg.NoPermission, nil, "permission denied: only contest moderators can delete announcements")
	}

	if err := h.announcementsUC.DeleteAnnouncement(ctx, params.AnnouncementID, contest.ID); err != nil {
		return err
	}

	return nil
}
