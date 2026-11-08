package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/submissions"
	sm "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type topicProjection struct{ submissions, scoreboard [][]byte }

func contestTopic(id uuid.UUID, audience string) string {
	return "contest/" + id.String() + "/" + audience
}
func ownTopic(id, user uuid.UUID) string { return contestTopic(id, "own/"+user.String()) }

// Event payloads are projected once before entering history or any subscriber
// queue. A queue contains only already-scoped frames; delivery never redacts.
func (h *Hub) projectTopics(ctx context.Context, e *WrappedEvent) error {
	meta := eventMeta(e)
	if meta == nil || meta.ContestId == nil {
		return nil
	}
	if h.contests == nil {
		return pkg.ErrInternal
	}
	var c contestsmodel.Contest
	if e.projectionContest != nil {
		c = *e.projectionContest
	} else {
		var err error
		c, err = h.contests.GetContest(ctx, *meta.ContestId)
		if err != nil {
			return err
		}
	}
	now := m.ReadTime(ctx)
	e.configuration = topicConfigurationAt(c, now)
	e.topics = map[string]topicProjection{}
	for _, audience := range []string{"public", "participants", "managers", "own"} {
		actor := m.Guest
		fact := m.ContestContext{ID: c.ID, StartTime: c.StartTime, EndTime: c.EndTime, Settings: c.Settings, Visibility: m.Visibility(c.Visibility), ParticipationMode: m.ParticipationMode(c.TypedSettings().GetParticipationMode()), Organization: m.OrganizationContext{ID: c.OrganizationID, JoinPolicy: m.JoinOpen}}
		if audience != "public" {
			actor = m.User{Id: uuid.New(), Role: m.UserRoleUser}
			fact.DirectRole = m.ContestParticipant
		}
		if audience == "managers" {
			fact.DirectRole = m.ContestModerator
		}
		if audience == "own" {
			if meta.UserId == nil {
				continue
			}
			actor.Id = *meta.UserId
		}
		in := m.PolicyInput{Actor: actor, Now: now}
		if err := access.ApplyContestLifecycle(in, &fact); err != nil {
			return err
		}
		allowed := func(a m.ContestAction) bool { return access.RequireContest(in, fact, a) == nil }
		fact.Submission = m.SubmissionContext{ContestID: c.ID, OwnerID: uuid.New()}
		if audience == "own" {
			fact.Submission.OwnerID = actor.Id
		}
		permissions := &m.ContestPermissions{ManageContest: allowed(m.ActionManageContest), GetOtherUserSubmission: allowed(m.ActionGetOtherUserSubmission), GetOwnSubmission: allowed(m.ActionGetOwnSubmission), GetSubmissionDetails: allowed(m.ActionGetSubmissionDetails), ListOwnSubmissions: allowed(m.ActionListOwnSubmissions)}
		projection := topicProjection{}
		if audience == "own" || allowed(m.ActionListUsersSubmissions) {
			view := submissions.SubmissionView(&c, meta.UserId, meta.CreatedAt, actor.Id, permissions, in.Now)
			if audience != "own" && audience != "managers" && c.IsFrozenAt(in.Now) {
				view.Frozen = true
				view.Source = false
				view.Details = false
			}
			if (!view.Frozen || e.EventType == sm.SubmissionEventCreated || e.EventType == sm.SubmissionEventCompleted) && (e.EventType != sm.SubmissionEventTestStarted || view.Details) {
				payload, err := projectSubmission(e, view.Source, view.Frozen)
				if err != nil {
					return err
				}
				projection.submissions = append(projection.submissions, payload)
			}
		}
		if audience != "own" && allowed(m.ActionGetMonitor) {
			if len(e.scoreboardPublic) > 0 {
				projection.scoreboard = append(projection.scoreboard, e.scoreboardPublic)
			}
			if allowed(m.ActionViewLiveScoreboard) && len(e.scoreboardUnfrozen) > 0 {
				projection.scoreboard = append(projection.scoreboard, e.scoreboardUnfrozen)
			}
		}
		topic := contestTopic(c.ID, audience)
		if audience == "own" {
			topic = ownTopic(c.ID, actor.Id)
		}
		e.topics[topic] = projection
	}
	return nil
}
func topicConfiguration(c contestsmodel.Contest) string {
	return topicConfigurationAt(c, time.Now().UTC())
}
func topicConfigurationAt(c contestsmodel.Contest, now time.Time) string {
	settings, _ := json.Marshal(c.Settings)
	return fmt.Sprintf("%s/%s/%v/%v/%s", scoreboardConfigurationAt(c, now), c.Visibility, c.StartTime, c.EndTime, settings)
}
func projectSubmission(event *WrappedEvent, source, frozen bool) ([]byte, error) {
	var envelope struct {
		EventType string                     `json:"event_type"`
		Sequence  uint64                     `json:"sequence"`
		Payload   map[string]json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(event.DTOPayload, &envelope); err != nil {
		return nil, fmt.Errorf("decode submission projection: %w", err)
	}
	if !source {
		delete(envelope.Payload, "source")
	}
	if frozen {
		for _, field := range []string{"state", "score", "penalty", "time_stat", "memory_stat", "failed_test", "test_details", "source", "previous_state"} {
			delete(envelope.Payload, field)
		}
		envelope.Payload["frozen"] = json.RawMessage("true")
	}
	envelope.Sequence = event.Seq
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode submission projection: %w", err)
	}
	return data, nil
}

func scopeEvent(e *WrappedEvent, topics []string) *WrappedEvent {
	if len(topics) == 0 {
		return e
	}
	clone := *e
	clone.DTOPayload = nil
	clone.topics = map[string]topicProjection{}
	for _, topic := range topics {
		if p, ok := e.topics[topic]; ok {
			clone.topics[topic] = p
		}
	}
	return &clone
}
func scopeHistory(events []*WrappedEvent, topics []string) []*WrappedEvent {
	result := make([]*WrappedEvent, len(events))
	for i, e := range events {
		result[i] = scopeEvent(e, topics)
	}
	return result
}
func projectContestTopics(e *WrappedEvent) {
	var contest, owner uuid.UUID
	switch event := e.RawEvent.(type) {
	case *contestsmodel.ContestAnnouncementCreatedEvent:
		contest = event.ContestID
	case *contestsmodel.ContestAnnouncementDeletedEvent:
		contest = event.ContestID
	case *contestsmodel.ContestClarificationCreatedEvent:
		contest = event.ContestID
		owner = event.UserID
	case *contestsmodel.ContestClarificationAnsweredEvent:
		contest = event.ContestID
		owner = event.UserID
	}
	p := topicProjection{submissions: [][]byte{e.DTOPayload}}
	e.topics = map[string]topicProjection{contestTopic(contest, "managers"): p}
	if owner != uuid.Nil {
		e.topics[ownTopic(contest, owner)] = p
	} else {
		e.topics[contestTopic(contest, "public")] = p
		e.topics[contestTopic(contest, "participants")] = p
	}
}
