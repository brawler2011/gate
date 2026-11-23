package observer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	sm "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Records reads the rows that notices name.
type Records interface {
	GetSubmission(ctx context.Context, id uuid.UUID) (sm.Submission, error)
	GetAnnouncementByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestAnnouncement, error)
	GetClarificationByID(ctx context.Context, id uuid.UUID) (*contestsmodel.ContestClarification, error)
}

// Feed turns change notices into hub events by reading the current rows.
type Feed struct {
	Hub      *Hub
	Contests *ContestsHub
	Records  Records
}

// Resync makes every client reconnect and refetch its snapshot.
func (f *Feed) Resync() {
	f.Hub.closeAll("resync")
	f.Contests.closeAll("resync")
}

func (f *Feed) Handle(ctx context.Context, payload string) {
	var notice eventdata.Notice
	if err := json.Unmarshal([]byte(payload), &notice); err != nil {
		slog.ErrorContext(ctx, "decode notice", "error", err)
		return
	}
	err := f.deliver(ctx, notice)
	// The row was deleted after the notice.
	if errors.Is(err, pkg.ErrNotFound) {
		return
	}
	if err != nil {
		slog.ErrorContext(ctx, "deliver notice", "type", notice.Type, "id", notice.ID, "error", err)
		f.Resync()
	}
}

type eventHandler interface {
	Handle(ctx context.Context, eventType string, payload []byte) error
}

func publish(ctx context.Context, h eventHandler, eventType string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode %s: %w", eventType, err)
	}
	return h.Handle(ctx, eventType, payload)
}

func (f *Feed) deliver(ctx context.Context, n eventdata.Notice) error {
	switch n.Type {
	case contestsmodel.ContestEventAnnouncementCreated:
		a, err := f.Records.GetAnnouncementByID(ctx, n.ID)
		if err != nil {
			return err
		}
		return publish(ctx, f.Contests, n.Type, a.CreatedEvent())
	case contestsmodel.ContestEventAnnouncementDeleted:
		if n.ContestID == nil {
			return fmt.Errorf("deleted announcement %s has no contest", n.ID)
		}
		return publish(ctx, f.Contests, n.Type, contestsmodel.ContestAnnouncementDeletedEvent{ID: n.ID, ContestID: *n.ContestID})
	case contestsmodel.ContestEventClarificationCreated, contestsmodel.ContestEventClarificationAnswered:
		c, err := f.Records.GetClarificationByID(ctx, n.ID)
		if err != nil {
			return err
		}
		if n.Type == contestsmodel.ContestEventClarificationCreated {
			return publish(ctx, f.Contests, n.Type, c.CreatedEvent())
		}
		return publish(ctx, f.Contests, n.Type, c.AnsweredEvent())
	case sm.SubmissionEventCreated, sm.SubmissionEventQueued, sm.SubmissionEventCompilingStarted,
		sm.SubmissionEventTestingStarted, sm.SubmissionEventTestStarted, sm.SubmissionEventCompleted:
		s, err := f.Records.GetSubmission(ctx, n.ID)
		if err != nil {
			return err
		}
		// A late progress notice of a finished attempt would otherwise carry the final revision and never clear.
		if n.Type != sm.SubmissionEventCreated && n.Type != sm.SubmissionEventCompleted && s.State != sm.Saved {
			return nil
		}
		meta := s.EventMeta()
		var event any
		switch n.Type {
		case sm.SubmissionEventCreated:
			event = s.CreatedEvent()
		case sm.SubmissionEventQueued:
			event = sm.SubmissionQueuedEvent{SubmissionEventMeta: meta, Id: s.ID}
		case sm.SubmissionEventCompilingStarted:
			event = sm.SubmissionCompilingStartedEvent{SubmissionEventMeta: meta, Id: s.ID}
		case sm.SubmissionEventTestingStarted:
			event = sm.SubmissionTestingStartedEvent{SubmissionEventMeta: meta, Id: s.ID}
		case sm.SubmissionEventTestStarted:
			event = sm.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: s.ID, Number: n.Number}
		default:
			event = s.CompletedEvent()
		}
		return publish(ctx, f.Hub, n.Type, event)
	default:
		return fmt.Errorf("unknown notice type %q", n.Type)
	}
}
