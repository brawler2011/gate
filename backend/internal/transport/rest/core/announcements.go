package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

func (h *CoreServer) ListContestAnnouncements(ctx context.Context, params corev1.ListContestAnnouncementsParams) (*corev1.ListContestAnnouncementsResponseModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	list, err := h.Announcements.ListAnnouncements(ctx, contest.ID, params.Page.Value, params.PageSize.Value)
	if err != nil {
		return nil, err
	}

	return ContestAnnouncementsListResponseDTO(list), nil
}

func (h *CoreServer) CreateContestAnnouncement(ctx context.Context, req *corev1.CreateContestAnnouncementRequestModel, params corev1.CreateContestAnnouncementParams) (*corev1.ContestAnnouncementModel, error) {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	var problemID *uuid.UUID
	if req.ProblemID.IsSet() {
		problemID = &req.ProblemID.Value
	}

	announcement, err := h.Announcements.CreateAnnouncement(ctx, &contestsmodel.CreateContestAnnouncementInput{
		ContestID: contest.ID,
		ProblemID: problemID,
		Title:     req.Title,
		Body:      req.Body,
	})
	if err != nil {
		return nil, err
	}

	return ContestAnnouncementDTO(announcement), nil
}

func (h *CoreServer) DeleteContestAnnouncement(ctx context.Context, params corev1.DeleteContestAnnouncementParams) error {
	contest, err := h.Contests.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	if err := h.Announcements.DeleteAnnouncement(ctx, params.AnnouncementID, contest.ID); err != nil {
		return err
	}

	return nil
}
