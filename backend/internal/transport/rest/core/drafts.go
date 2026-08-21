package core

import (
	"context"
	"log/slog"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) CreateContestDraft(ctx context.Context, request corev1.CreateContestDraftRequestObject) (corev1.CreateContestDraftResponseObject, error) {
	user := middleware.GetUser(ctx)

	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}
	req := *request.Body

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
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
		ProblemID: req.ProblemId,
		Language:  models.LanguageName(req.Language),
		Code:      req.Code,
	}

	draftID, err := h.draftsUC.CreateDraft(ctx, draftCreation)
	if err != nil {
		return nil, err
	}

	slog.Info("contest draft created", "draft_id", draftID, "user_id", user.Id, "contest_id", contest.ID, "problem_id", req.ProblemId)

	return corev1.CreateContestDraft200JSONResponse{Id: draftID}, nil
}

func (h *CoreServer) ListContestDrafts(ctx context.Context, request corev1.ListContestDraftsRequestObject) (corev1.ListContestDraftsResponseObject, error) {
	user := middleware.GetUser(ctx)

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
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

	var filterUserID *uuid.UUID
	if isManager {
		// Manager can filter by participant or view all
		filterUserID = request.Params.UserId
	} else {
		// Regular user can only view their own drafts
		uid := user.Id
		filterUserID = &uid
	}

	var page int32 = 1
	if request.Params.Page != nil && *request.Params.Page > 0 {
		page = *request.Params.Page
	}

	var pageSize int32 = 20
	if request.Params.PageSize != nil && *request.Params.PageSize > 0 {
		pageSize = *request.Params.PageSize
	}

	draftsList, err := h.draftsUC.ListDrafts(ctx, models.ContestDraftsFilter{
		ContestID: contest.ID,
		UserID:    filterUserID,
		ProblemID: request.Params.ProblemId,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		return nil, err
	}

	return corev1.ListContestDrafts200JSONResponse(*ListContestDraftsResponseDTO(draftsList)), nil
}

func (h *CoreServer) DeleteContestDraft(ctx context.Context, request corev1.DeleteContestDraftRequestObject) (corev1.DeleteContestDraftResponseObject, error) {
	user := middleware.GetUser(ctx)

	contest, err := h.contestsUC.GetContestByOrgLoginAndContestLogin(ctx, request.OrgLogin, request.ContestLogin)
	if err != nil {
		return nil, err
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

	if err := h.draftsUC.DeleteDraft(ctx, request.DraftId, user.Id, isManager); err != nil {
		return nil, err
	}

	slog.Info("contest draft deleted", "draft_id", request.DraftId, "user_id", user.Id, "contest_id", contest.ID)

	return corev1.DeleteContestDraft200Response{}, nil
}
