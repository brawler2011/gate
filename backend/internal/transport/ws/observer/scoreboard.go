package observer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/brawler2011/gate/backend/pkg"

	observerv1 "github.com/brawler2011/gate/backend/contracts/observer/v1"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

func scoreboardConfiguration(contest contestsmodel.Contest) string {
	return scoreboardConfigurationAt(contest, time.Now().UTC())
}
func scoreboardConfigurationAt(contest contestsmodel.Contest, now time.Time) string {
	started := contest.StartTime == nil || !now.Before(*contest.StartTime)
	finished := contest.EndTime != nil && !now.Before(*contest.EndTime)
	return fmt.Sprintf("%t/%t/%t/%v/%v/%v/%s/%d", started, finished, contest.IsFrozenAt(now), contest.StartTime, contest.EndTime,
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

func scoreboardEnvelope(scoreboard *contestsmodel.ScoreboardResponse, userID uuid.UUID, unfrozen, replace bool, seq uint64) ([]byte, error) {
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
	if h.contests == nil || h.scoreboard == nil {
		return nil
	}
	if event.EventType != submissionsmodel.SubmissionEventCreated && event.EventType != submissionsmodel.SubmissionEventCompleted {
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
	event.projectionContest = &contest
	configuration := scoreboardConfigurationAt(contest, m.ReadTime(ctx))
	replace := configuration != previous
	var public *contestsmodel.ScoreboardResponse
	if replace {
		public, err = h.scoreboard.ProjectScoreboard(ctx, *meta.ContestId, false)
	} else {
		public, err = h.scoreboard.GetContestScoreboardUser(ctx, *meta.ContestId, *meta.UserId, false)
	}
	if err != nil {
		return fmt.Errorf("project public scoreboard: %w", err)
	}
	live := public
	if public.IsFrozen {
		if replace {
			live, err = h.scoreboard.ProjectScoreboard(ctx, *meta.ContestId, true)
		} else {
			live, err = h.scoreboard.GetContestScoreboardUser(ctx, *meta.ContestId, *meta.UserId, true)
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
		delete(h.finalSnapshots, *contestID)
		delete(h.scoreboardConfigurations, *contestID)
	}
}

// A finish invalidates the running lease even without new judge events. Publish
// one shared final snapshot before closing that lease; reconnect refreshes REST.
type finalSnapshot struct {
	configuration string
	payload       []byte
}

func (s *Observer) sendFinalSnapshot(ctx context.Context, filter *SubmissionsFilter, conn eventConnection, cause error) {
	ctx = m.WithReadTime(ctx, time.Now().UTC())
	if !errors.Is(cause, pkg.ErrConflict) || filter.ContestId == nil || s.hub.scoreboard == nil {
		return
	}
	_, p, err := s.currentAccess(ctx, filter)
	if err != nil || !p.GetMonitor {
		return
	}
	c, err := s.contests.GetContest(ctx, *filter.ContestId)
	if err != nil || c.EndTime == nil || time.Now().Before(*c.EndTime) {
		return
	}
	h := s.hub
	h.scoreboardMu.Lock()
	configuration := topicConfiguration(c)
	cached, ok := h.finalSnapshots[c.ID]
	if !ok || cached.configuration != configuration {
		snapshot, err := h.scoreboard.ProjectScoreboard(ctx, c.ID, false)
		if err != nil {
			h.scoreboardMu.Unlock()
			return
		}
		h.mu.RLock()
		sequence := h.ring.MaxSeq()
		h.mu.RUnlock()
		payload, err := scoreboardEnvelope(snapshot, uuid.Nil, false, true, sequence)
		if err != nil {
			h.scoreboardMu.Unlock()
			return
		}
		cached = finalSnapshot{configuration: configuration, payload: payload}
		h.finalSnapshots[c.ID] = cached
	}
	h.scoreboardMu.Unlock()
	_ = writeEvent(conn, cached.payload)
}
