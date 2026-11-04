package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type AnnouncementsReader interface {
	ListAnnouncements(ctx context.Context, contestID uuid.UUID, page, pageSize int32) (*contestsmodel.ContestAnnouncementsList, error)
}

func (h *CoreServer) ListContestAnnouncements(ctx context.Context, params corev1.ListContestAnnouncementsParams) (*corev1.ListContestAnnouncementsResponseModel, error) {
	contest, err := h.contestsReader.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	list, err := h.announcementsReader.ListAnnouncements(ctx, contest.ID, params.Page.Value, params.PageSize.Value)
	if err != nil {
		return nil, err
	}

	return ContestAnnouncementsListResponseDTO(list), nil
}

func (h *CoreServer) CreateContestAnnouncement(ctx context.Context, req *corev1.CreateContestAnnouncementRequestModel, params corev1.CreateContestAnnouncementParams) (*corev1.ContestAnnouncementModel, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsReader.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	// FIXME: create OptUUID struct instead of using pointer
	var problemID *uuid.UUID
	if req.ProblemID.IsSet() {
		problemID = &req.ProblemID.Value
	}

	announcement, err := h.announcementsUC.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{
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
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return pkg.Wrap(pkg.ErrUnauthenticated, nil, "authentication required")
	}

	contest, err := h.contestsReader.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	if err := h.announcementsUC.DeleteAnnouncement(ctx, params.AnnouncementID, contest.ID); err != nil {
		return err
	}

	return nil
}
