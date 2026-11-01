package core

import (
	"context"
	"testing"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type snapshotStream struct {
	jetstream.Stream
	sequence uint64
}

func (s *snapshotStream) Info(context.Context, ...jetstream.StreamInfoOpt) (*jetstream.StreamInfo, error) {
	return &jetstream.StreamInfo{State: jetstream.StreamState{LastSeq: s.sequence}}, nil
}

type snapshotJS struct {
	jetstream.JetStream
	stream *snapshotStream
}

func (s snapshotJS) Stream(context.Context, string) (jetstream.Stream, error) { return s.stream, nil }

type snapshotUC struct {
	interfaces.ContestsUC
	interfaces.SubmissionsUC
	interfaces.PermissionsUC
	stream *snapshotStream
}

func (s snapshotUC) GetContestByOrgLoginAndContestLogin(context.Context, string, string) (models.Contest, error) {
	return models.Contest{ID: uuid.New()}, nil
}
func (s snapshotUC) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*models.ContestPermissions, error) {
	return &models.ContestPermissions{ManageContest: true}, nil
}
func (s snapshotUC) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, models.ContestAction) (bool, error) {
	return true, nil
}
func (s snapshotUC) ListSubmissions(context.Context, models.SubmissionsFilter) (*models.SubmissionsList, error) {
	// A new event arrives after the database snapshot has been read.
	s.stream.sequence++
	return &models.SubmissionsList{}, nil
}
func (s snapshotUC) GetContestScoreboard(context.Context, uuid.UUID, uuid.UUID, bool) (*models.ScoreboardResponse, error) {
	s.stream.sequence++
	return &models.ScoreboardResponse{}, nil
}
func TestSnapshotCursorDoesNotDiscardConcurrentEvent(t *testing.T) {
	for _, scoreboard := range []bool{false, true} {
		t.Run(map[bool]string{false: "submissions", true: "scoreboard"}[scoreboard], func(t *testing.T) {
			stream := &snapshotStream{sequence: 7}
			uc := snapshotUC{stream: stream}
			server := &CoreServer{contestsUC: uc, submissionsUC: uc, permissionsUC: uc, natsJS: snapshotJS{stream: stream}}
			var cursor corev1.OptInt64
			if scoreboard {
				response, err := server.GetContestScoreboard(context.Background(), corev1.GetContestScoreboardParams{})
				require.NoError(t, err)
				cursor = response.Since
			} else {
				response, err := server.ListContestSubmissions(context.Background(), corev1.ListContestSubmissionsParams{})
				require.NoError(t, err)
				cursor = response.Since
			}
			require.EqualValues(t, 7, cursor.Value)
			require.EqualValues(t, 8, stream.sequence)
		})
	}
}
