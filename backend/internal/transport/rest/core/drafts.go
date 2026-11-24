package core

import (
	"context"
	"log/slog"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) CreateContestDraft(ctx context.Context, req *corev1.CreateContestDraftRequestModel, params corev1.CreateContestDraftParams) (*corev1.CreationResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "missing request body")
	}

	draftCreation := &contestsmodel.ContestDraftCreation{
		ContestID: params.ContestID,
		Code:      req.Code,
	}

	draftID, err := h.Drafts.CreateDraft(ctx, draftCreation)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "contest draft created", "draft_id", draftID, "user_id", user.Id, "contest_id", params.ContestID)

	return &corev1.CreationResponseModel{ID: draftID}, nil
}

func (h *CoreServer) ListContestDrafts(ctx context.Context, params corev1.ListContestDraftsParams) (*corev1.ListContestDraftsResponseModel, error) {
	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(20)

	draftsList, err := h.Drafts.ListDrafts(ctx, contestsmodel.ContestDraftsFilter{
		ContestID: params.ContestID,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		return nil, err
	}

	return ListContestDraftsResponseDTO(draftsList), nil
}

func (h *CoreServer) DeleteContestDraft(ctx context.Context, params corev1.DeleteContestDraftParams) error {
	user := accessmodel.UserFromContext(ctx)

	if err := h.Drafts.DeleteDraft(ctx, params.ContestID, params.DraftID); err != nil {
		return err
	}

	slog.InfoContext(ctx, "contest draft deleted", "draft_id", params.DraftID, "user_id", user.Id, "contest_id", params.ContestID)

	return nil
}
