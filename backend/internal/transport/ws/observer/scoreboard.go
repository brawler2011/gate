package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	observerv1 "github.com/brawler2011/contracts/observer/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func scoreboardConfiguration(contest models.Contest) string {
	return fmt.Sprintf("%t/%v/%v/%v/%s/%d", contest.IsFrozenAt(time.Now()), contest.StartTime, contest.EndTime,
		contest.GetFreezeTime(), contest.GetFreezeStatus(), contest.GetPenaltyPerAttempt())
}

func (h *Hub) trackScoreboard(ctx context.Context, contestID *uuid.UUID) error {
	if contestID == nil || h.contests == nil {
		return nil
	}
	h.scoreboardMu.Lock()
	defer h.scoreboardMu.Unlock()
	if _, ok := h.scoreboardConfigurations[*contestID]; ok {
		h.scoreboardReferences[*contestID]++
		return nil
	}
	contest, err := h.contests.GetContest(ctx, *contestID)
	if err != nil {
		return err
	}
	h.scoreboardConfigurations[*contestID] = scoreboardConfiguration(contest)
	h.scoreboardReferences[*contestID] = 1
	return nil
}

func scoreboardEnvelope(scoreboard *models.ScoreboardResponse, userID uuid.UUID, unfrozen, replace bool, seq uint64) ([]byte, error) {
	if seq > math.MaxInt64 {
		return nil, fmt.Errorf("scoreboard sequence exceeds int64")
	}
	raw, err := json.Marshal(scoreboard)
	if err != nil {
		return nil, fmt.Errorf("marshal scoreboard: %w", err)
	}
	var dto observerv1.ScoreboardResponseModel
	if err := json.Unmarshal(raw, &dto); err != nil {
		return nil, fmt.Errorf("convert scoreboard: %w", err)
	}
	msg := observerv1.ScoreboardUpdatedMessage{EventType: new(observerv1.ScoreboardUpdatedMessageEventType), Sequence: int64(seq),
		Payload: &observerv1.MessageScoreboardUpdated{UserId: userID, Unfrozen: unfrozen, Replace: replace, Scoreboard: &dto}}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal scoreboard update: %w", err)
	}
	return data, nil
}

func (h *Hub) projectScoreboard(ctx context.Context, event *WrappedEvent) error {
	if h.contests == nil {
		return nil
	}
	if event.EventType != models.SubmissionEventCreated && event.EventType != models.SubmissionEventCompleted {
		return nil
	}
	meta := eventMeta(event)
	if meta == nil || meta.ContestId == nil || meta.UserId == nil {
		return nil
	}
	h.scoreboardMu.Lock()
	defer h.scoreboardMu.Unlock()
	previous, active := h.scoreboardConfigurations[*meta.ContestId]
	if !active {
		return nil
	}
	contest, err := h.contests.GetContest(ctx, *meta.ContestId)
	if err != nil {
		return err
	}
	configuration := scoreboardConfiguration(contest)
	replace := configuration != previous
	var public *models.ScoreboardResponse
	if replace {
		public, err = h.contests.GetContestScoreboard(ctx, *meta.ContestId, uuid.Nil, false)
	} else {
		public, err = h.contests.GetContestScoreboardUser(ctx, *meta.ContestId, *meta.UserId, false)
	}
	if err != nil {
		return fmt.Errorf("project public scoreboard: %w", err)
	}
	live := public
	if public.IsFrozen {
		if replace {
			live, err = h.contests.GetContestScoreboard(ctx, *meta.ContestId, uuid.Nil, true)
		} else {
			live, err = h.contests.GetContestScoreboardUser(ctx, *meta.ContestId, *meta.UserId, true)
		}
		if err != nil {
			return fmt.Errorf("project unfrozen scoreboard: %w", err)
		}
	}
	event.scoreboardPublic, err = scoreboardEnvelope(public, *meta.UserId, false, replace, event.Seq)
	if err != nil {
		return err
	}
	event.scoreboardUnfrozen, err = scoreboardEnvelope(live, *meta.UserId, true, replace, event.Seq)
	if err != nil {
		return err
	}
	event.scoreboardConfiguration = configuration
	h.scoreboardConfigurations[*meta.ContestId] = configuration
	return nil
}

func (h *Hub) releaseScoreboard(contestID *uuid.UUID) {
	if contestID == nil || h.contests == nil {
		return
	}
	h.scoreboardMu.Lock()
	defer h.scoreboardMu.Unlock()
	h.scoreboardReferences[*contestID]--
	if h.scoreboardReferences[*contestID] == 0 {
		delete(h.scoreboardReferences, *contestID)
		delete(h.scoreboardConfigurations, *contestID)
	}
}
