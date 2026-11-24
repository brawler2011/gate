package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

func (h *CoreServer) ListContestClarifications(ctx context.Context, params corev1.ListContestClarificationsParams) (*corev1.ListContestClarificationsResponseModel, error) {
	var problemID *uuid.UUID
	if params.ProblemID.IsSet() {
		problemID = &params.ProblemID.Value
	}
	var status *string
	if params.Status.IsSet() {
		status = &params.Status.Value
	}
	list, err := h.Clarifications.ListClarifications(ctx, params.ContestID, &contestsmodel.ContestClarificationsFilter{
		ProblemID: problemID, Status: status, Page: params.Page.Or(1), PageSize: params.PageSize.Or(50),
	})
	if err != nil {
		return nil, err
	}

	return ContestClarificationsListResponseDTO(list), nil
}

func (h *CoreServer) CreateContestClarification(ctx context.Context, req *corev1.CreateContestClarificationRequestModel, params corev1.CreateContestClarificationParams) (*corev1.ContestClarificationModel, error) {
	var problemID *uuid.UUID
	if req.ProblemID.IsSet() {
		problemID = &req.ProblemID.Value
	}

	clarification, err := h.Clarifications.CreateClarification(ctx, &contestsmodel.CreateContestClarificationInput{
		ContestID: params.ContestID,
		ProblemID: problemID,
		Question:  req.Question,
	})
	if err != nil {
		return nil, err
	}

	return ContestClarificationDTO(clarification), nil
}

func (h *CoreServer) AnswerContestClarification(ctx context.Context, req *corev1.AnswerContestClarificationRequestModel, params corev1.AnswerContestClarificationParams) (*corev1.ContestClarificationModel, error) {
	publishAsAnnouncement := req.PublishAsAnnouncement.Or(false)
	announcementTitle := req.AnnouncementTitle.Or("")

	clarification, err := h.Clarifications.AnswerClarification(ctx, &contestsmodel.AnswerContestClarificationInput{
		ClarificationID:       params.ClarificationID,
		ContestID:             params.ContestID,
		Answer:                req.Answer,
		PublishAsAnnouncement: publishAsAnnouncement,
		AnnouncementTitle:     announcementTitle,
	})
	if err != nil {
		return nil, err
	}

	return ContestClarificationDTO(clarification), nil
}
