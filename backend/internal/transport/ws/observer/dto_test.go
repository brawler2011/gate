package observer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDiscriminatedEnvelopes(t *testing.T) {
	id := uuid.New()
	tests := []struct {
		kind   string
		event  any
		encode func(string, any) ([]byte, error)
	}{
		{models.SubmissionEventCreated, &models.SubmissionCreatedEvent{Id: id}, SubmissionsEnvelopeDTO},
		{models.SubmissionEventQueued, &models.SubmissionQueuedEvent{Id: id}, SubmissionsEnvelopeDTO},
		{models.SubmissionEventCompilingStarted, &models.SubmissionCompilingStartedEvent{Id: id}, SubmissionsEnvelopeDTO},
		{models.SubmissionEventTestingStarted, &models.SubmissionTestingStartedEvent{Id: id}, SubmissionsEnvelopeDTO},
		{models.SubmissionEventTestStarted, &models.SubmissionTestStartedEvent{Id: id, Number: 7}, SubmissionsEnvelopeDTO},
		{models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{Id: id}, SubmissionsEnvelopeDTO},
		{models.ContestEventAnnouncementCreated, &models.ContestAnnouncementCreatedEvent{ID: id}, ContestEnvelopeDTO},
		{models.ContestEventAnnouncementDeleted, &models.ContestAnnouncementDeletedEvent{ID: id}, ContestEnvelopeDTO},
		{models.ContestEventClarificationCreated, &models.ContestClarificationCreatedEvent{ID: id}, ContestEnvelopeDTO},
		{models.ContestEventClarificationAnswered, &models.ContestClarificationAnsweredEvent{ID: id}, ContestEnvelopeDTO},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			data, err := tt.encode(tt.kind, tt.event)
			require.NoError(t, err)
			var got struct {
				EventType string         `json:"event_type"`
				Payload   map[string]any `json:"payload"`
			}
			require.NoError(t, json.Unmarshal(data, &got))
			require.Equal(t, tt.kind, got.EventType)
			require.Equal(t, id.String(), got.Payload["id"])
			if tt.kind == models.SubmissionEventTestStarted {
				require.InDelta(t, 7, got.Payload["number"], 0)
			} else {
				require.NotContains(t, got.Payload, "number")
			}
		})
	}
}

func TestCompletedEnvelopePreservesZeroNullAndFormats(t *testing.T) {
	id := uuid.New()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	position := int32(0)
	event := &models.SubmissionCompletedEvent{
		Id: id,
		SubmissionEventMeta: models.SubmissionEventMeta{
			UserId: &id, CreatedAt: created, Position: &position,
		},
	}
	data, err := SubmissionsEnvelopeDTO(models.SubmissionEventCompleted, event)
	require.NoError(t, err)
	var got struct {
		Payload map[string]any `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(data, &got))
	for _, key := range []string{"score", "state", "penalty", "time_stat", "memory_stat", "position"} {
		require.Contains(t, got.Payload, key)
		require.InDelta(t, 0, got.Payload[key], 0)
	}
	require.Contains(t, got.Payload, "failed_test")
	require.Nil(t, got.Payload["failed_test"])
	require.Equal(t, id.String(), got.Payload["user_id"])
	require.Equal(t, created.Format(time.RFC3339), got.Payload["created_at"])
	for _, key := range []string{"contest_id", "problem_id", "language", "username", "frozen"} {
		require.NotContains(t, got.Payload, key)
	}
	failedTest := int32(3)
	event.FailedTest = &failedTest
	data, err = SubmissionsEnvelopeDTO(models.SubmissionEventCompleted, event)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &got))
	require.InDelta(t, 3, got.Payload["failed_test"], 0)
}

func TestReplayCloseFrames(t *testing.T) {
	for _, tt := range []struct {
		err    error
		code   int
		reason string
	}{
		{ErrBufferEmpty, 4408, "buffer_empty"},
		{ErrHistoryLost, 4409, "history_lost"},
		{ErrInvalidRange, 4410, "invalid_range"},
	} {
		code, reason := getClosingFrameData(tt.err)
		require.Equal(t, tt.code, code)
		require.Equal(t, tt.reason, string(reason))
	}
}
