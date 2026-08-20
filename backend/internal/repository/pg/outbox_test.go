package pg

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapOutboxEvent_HeadersHandling(t *testing.T) {
	testID := uuid.New()
	testAggregateID := uuid.New()
	now := time.Now().UTC()
	errMsg := "some error"

	t.Run("valid headers JSON", func(t *testing.T) {
		headersMap := map[string]string{
			"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			"tracestate":  "rojo=1",
			"x-custom":    "value",
		}
		rawJSON, err := json.Marshal(headersMap)
		require.NoError(t, err)

		sqlcEvent := sqlc.OutboxEvent{
			ID:           testID,
			AggregateID:  testAggregateID,
			EventType:    models.OutboxEventSubmissionCreated,
			Payload:      []byte(`{"key":"value"}`),
			Status:       models.OutboxEventStatusPending,
			RetryCount:   1,
			ErrorMessage: &errMsg,
			CreatedAt:    now,
			ProcessedAt:  &now,
			LockedAt:     &now,
			DeadlineAt:   &now,
			Headers:      rawJSON,
		}

		result := mapOutboxEvent(sqlcEvent)

		assert.Equal(t, testID, result.Id)
		assert.Equal(t, testAggregateID, result.AggregateID)
		assert.Equal(t, models.OutboxEventSubmissionCreated, result.EventType)
		assert.JSONEq(t, `{"key":"value"}`, string(result.Payload))
		assert.Equal(t, models.OutboxEventStatusPending, result.Status)
		assert.Equal(t, int32(1), result.RetryCount)
		assert.Equal(t, &errMsg, result.ErrorMessage)
		assert.Equal(t, now, result.CreatedAt)
		assert.Equal(t, &now, result.ProcessedAt)
		assert.Equal(t, &now, result.LockedAt)
		assert.Equal(t, &now, result.DeadlineAt)
		assert.NotNil(t, result.Headers)
		assert.Equal(t, headersMap, result.Headers)
	})

	t.Run("empty headers bytes returns non-nil map", func(t *testing.T) {
		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			Headers:     nil,
		}

		result := mapOutboxEvent(sqlcEvent)

		assert.NotNil(t, result.Headers)
		assert.Empty(t, result.Headers)
	})

	t.Run("empty JSON object returns non-nil map", func(t *testing.T) {
		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			Headers:     []byte("{}"),
		}

		result := mapOutboxEvent(sqlcEvent)

		assert.NotNil(t, result.Headers)
		assert.Empty(t, result.Headers)
	})

	t.Run("null JSON returns non-nil map", func(t *testing.T) {
		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			Headers:     []byte("null"),
		}

		result := mapOutboxEvent(sqlcEvent)

		assert.NotNil(t, result.Headers)
		assert.Empty(t, result.Headers)
	})

	t.Run("invalid JSON returns non-nil empty map without error", func(t *testing.T) {
		sqlcEvent := sqlc.OutboxEvent{
			ID:          testID,
			AggregateID: testAggregateID,
			EventType:   models.OutboxEventSubmissionCreated,
			Payload:     []byte(`{}`),
			Status:      models.OutboxEventStatusPending,
			Headers:     []byte("invalid-json{"),
		}

		result := mapOutboxEvent(sqlcEvent)

		assert.NotNil(t, result.Headers)
		assert.Empty(t, result.Headers)
	})
}
