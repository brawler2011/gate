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

// Project payloads before history or enqueue; delivery must not redact them again.
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
	// Each audience is projected for a representative actor; the policy is pure, so no grants are read.
	for _, audience := range []string{"public", "participants", "managers", "own"} {
		actor, role := m.Guest, m.ContestRole("")
		switch audience {
		case "participants":
			actor, role = m.User{Id: uuid.New(), Role: m.UserRoleUser}, m.ContestParticipant
		case "managers":
			actor, role = m.User{Id: uuid.New(), Role: m.UserRoleUser}, m.ContestModerator
		case "own":
			if meta.UserId == nil {
				continue
			}
			actor, role = m.User{Id: *meta.UserId, Role: m.UserRoleUser}, m.ContestParticipant
		}
		a := access.ContestAccess{Actor: actor, Facts: access.ContestFacts{Contest: c, Role: role, Now: now}}
		allowed := a.Can
		projection := topicProjection{}
		if audience == "own" || allowed(m.ContestListSubmissions) {
			view := submissions.SubmissionView(a, meta.UserId, meta.CreatedAt)
			if audience != "own" && audience != "managers" && c.IsFrozenAt(now) {
				view.Hide()
			}
			if (!view.Hidden() || e.EventType == sm.SubmissionEventCreated || e.EventType == sm.SubmissionEventCompleted) && (e.EventType != sm.SubmissionEventTestStarted || view.Results.Tests()) {
				payload, err := projectSubmission(e, view)
				if err != nil {
					return err
				}
				projection.submissions = append(projection.submissions, payload)
			}
		}
		if audience != "own" && allowed(m.ContestViewScoreboard) {
			if len(e.scoreboardPublic) > 0 {
				projection.scoreboard = append(projection.scoreboard, e.scoreboardPublic)
			}
			if allowed(m.ContestManage) && len(e.scoreboardUnfrozen) > 0 {
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
func projectSubmission(event *WrappedEvent, view submissions.SubmissionVisibility) ([]byte, error) {
	var envelope struct {
		EventType string                     `json:"event_type"`
		Payload   map[string]json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(event.DTOPayload, &envelope); err != nil {
		return nil, fmt.Errorf("decode submission projection: %w", err)
	}
	if !view.Source {
		delete(envelope.Payload, "source")
	}
	if view.Hidden() {
		for _, field := range []string{"state", "score", "penalty", "time_stat", "memory_stat", "failed_test", "test_details", "source"} {
			delete(envelope.Payload, field)
		}
		envelope.Payload["result_hidden"] = json.RawMessage("true")
	}
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
