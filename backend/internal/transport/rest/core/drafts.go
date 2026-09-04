package core

import (
	"context"
	"log/slog"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
)

func (h *CoreServer) CreateContestDraft(ctx context.Context, req *corev1.CreateContestDraftRequestModel, params corev1.CreateContestDraftParams) (*corev1.CreationResponseModel, error) {
	user := middleware.GetUser(ctx)

	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	// Check if contest has ended. If ended, non-managers cannot create drafts (read-only mode)
	if contest.EndTime != nil && time.Now().After(*contest.EndTime) {
		isManager := false
		if user.IsAdmin() {
			isManager = true
		} else {
			allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
			if err == nil && allowed {
				isManager = true
			}
		}
		if !isManager {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "contest has ended; new drafts cannot be created")
		}
	}

	draftCreation := &models.ContestDraftCreation{
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
	user := middleware.GetUser(ctx)

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return nil, err
	}

	page := params.Page.Or(1)
	pageSize := params.PageSize.Or(20)

	draftsList, err := h.draftsUC.ListDrafts(ctx, models.ContestDraftsFilter{
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
	user := middleware.GetUser(ctx)

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, params.OrgLogin, params.ContestLogin)
	if err != nil {
		return err
	}

	isManager := false
	if user.IsAdmin() {
		isManager = true
	} else {
		allowed, err := h.permissionsUC.HasContestPermission(ctx, contest.ID, user.Id, models.ActionManageContest)
		if err == nil && allowed {
			isManager = true
		}
	}

	if err := h.draftsUC.DeleteDraft(ctx, params.DraftID, user.Id, isManager); err != nil {
		return err
	}

	slog.Info("contest draft deleted", "draft_id", params.DraftID, "user_id", user.Id, "contest_id", contest.ID)

	return nil
}
