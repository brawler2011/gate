package observer

import (
	"context"
	"testing"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	sm "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type feedRecords struct{ submission sm.Submission }

func (r *feedRecords) GetSubmission(context.Context, uuid.UUID) (sm.Submission, error) {
	return r.submission, nil
}
func (*feedRecords) GetAnnouncementByID(context.Context, uuid.UUID) (*contestsmodel.ContestAnnouncement, error) {
	return nil, pkg.ErrNotFound
}
func (*feedRecords) GetClarificationByID(context.Context, uuid.UUID) (*contestsmodel.ContestClarification, error) {
	return nil, pkg.ErrNotFound
}

func TestFeedDropsProgressOfFinishedSubmission(t *testing.T) {
	hub := NewHub(nil, nil)
	sub := hub.subscribe(&SubmissionsFilter{})
	id := uuid.New()
	records := &feedRecords{submission: sm.Submission{ID: id, State: sm.Disqualified, Revision: 3}}
	feed := &Feed{Hub: hub, Contests: NewContestsHub(), Records: records}
	notice := `{"type":"submissions.test_started","id":"` + id.String() + `","number":4}`
	feed.Handle(context.Background(), notice)
	require.Empty(t, sub.outbox, "a late notice after the verdict must not show testing again")
	feed.Handle(context.Background(), `{"type":"submissions.completed","id":"`+id.String()+`"}`)
	require.Len(t, sub.outbox, 1)
	records.submission.State = sm.Saved
	feed.Handle(context.Background(), notice)
	require.Len(t, sub.outbox, 2)
}
