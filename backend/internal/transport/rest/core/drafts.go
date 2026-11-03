package core

import (
	"context"
	"log/slog"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
)

type DraftsReader interface {
	ListDrafts(ctx context.Context, filter contestsmodel.ContestDraftsFilter) (*contestsmodel.ContestDraftsList, error)
}

func (h *CoreServer) CreateContestDraft(ctx context.Context, req *corev1.CreateContestDraftRequestModel, params corev1.CreateContestDraftParams) (*corev1.CreationResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	draftCreation := &contestsmodel.ContestDraftCreation{
		ContestID: contest.ID,
		UserID:    user.Id,
		Code:      req.Code,
	}

	draftID, err := h.draftsUC.CreateDraft(ctx, draftCreation)
	if err != nil {
		return nil, err
	}

	slog.Info("contest draft created", "draft_id", draftID, "user_id", user.Id, "contest_id", contest.ID)

	return &corev1.CreationResponseModel{ID: draftID}, nil
}

func (h *CoreServer) ListContestDrafts(ctx context.Context, params corev1.ListContestDraftsParams) (*corev1.ListContestDraftsResponseModel, error) {
	user := accessmodel.UserFromContext(ctx)

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(20)

	draftsList, err := h.draftsReader.ListDrafts(ctx, contestsmodel.ContestDraftsFilter{
		ContestID: contest.ID,
		UserID:    user.Id,
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

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	if err := h.draftsUC.DeleteDraft(ctx, contest.ID, params.DraftID); err != nil {
		return err
	}

	slog.Info("contest draft deleted", "draft_id", params.DraftID, "user_id", user.Id, "contest_id", contest.ID)

	return nil
}
