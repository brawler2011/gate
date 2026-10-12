package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type judgeFunc func(context.Context, uuid.UUID) error

func (f judgeFunc) JudgeSubmission(ctx context.Context, id uuid.UUID) error { return f(ctx, id) }

type testMessage struct {
	jetstream.Msg
	header   nats.Header
	data     []byte
	ack, nak int
	ackErr   error
}

func (m *testMessage) Headers() nats.Header { return m.header }
func (m *testMessage) Data() []byte         { return m.data }
func (m *testMessage) Ack() error           { m.ack++; return m.ackErr }
func (m *testMessage) Nak() error           { m.nak++; return nil }
func (m *testMessage) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: 2}, nil
}

func TestJudgeMessageDiagnosticsAndAcknowledgement(t *testing.T) {
	for _, failure := range []error{nil, errors.New("Bearer secret-request-body"), context.DeadlineExceeded} {
		for _, queued := range []string{"", "garbage-secret", time.Now().Add(time.Hour).Format(time.RFC3339Nano), time.Now().Add(-2 * time.Second).Format(time.RFC3339Nano)} {
			for _, valid := range []bool{true, false} {
				t.Run(queued+"/"+strings.ReplaceAll(time.Now().String(), " ", ""), func(t *testing.T) {
					id, eventID := uuid.New(), uuid.New()
					data, err := json.Marshal(models.SubmissionCreatedEvent{Id: id, SubmissionEventMeta: models.SubmissionEventMeta{CreatedAt: time.Now().Add(-24 * time.Hour)}})
					require.NoError(t, err)
					header := nats.Header{}
					header.Set(queuecontext.QueuedAtHeader, queued)
					if valid {
						header.Set(requestid.Header, "request-123")
						header.Set(queuecontext.EventIDHeader, eventID.String())
					} else {
						header.Set(requestid.Header, "bad secret")
						header.Set(queuecontext.EventIDHeader, "secret")
					}
					msg := &testMessage{header: header, data: data, ackErr: errors.New("ack-secret")}
					var buf bytes.Buffer
					uc := judgeFunc(func(ctx context.Context, got uuid.UUID) error {
						require.Equal(t, id, got)
						require.NoError(t, ctx.Err())
						if valid {
							require.Equal(t, "request-123", requestid.Get(ctx))
						} else {
							require.Empty(t, requestid.Get(ctx))
						}
						deadline, ok := ctx.Deadline()
						require.True(t, ok)
						require.WithinDuration(t, time.Now().Add(5*time.Minute), deadline, time.Second)
						return failure
					})
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					w := JudgeWorker{judgeUC: uc}
					w.handleMessage(ctx, msg, logging.New(&buf))
					require.Equal(t, 1, msg.ack)
					require.Zero(t, msg.nak)
					require.Contains(t, buf.String(), id.String())
					require.Contains(t, buf.String(), `"delivery_attempt":2`)
					require.NotContains(t, buf.String(), "secret")
					require.Contains(t, buf.String(), `"duration_ms"`)
					if failure == nil {
						require.Contains(t, buf.String(), "submission_judging_completed")
					} else {
						require.Contains(t, buf.String(), "submission_judging_failed")
					}
					parsed, e := time.Parse(time.RFC3339Nano, queued)
					if e == nil && parsed.Before(time.Now()) {
						var started map[string]interface{}
						require.NoError(t, json.Unmarshal([]byte(strings.Split(buf.String(), "\n")[0]), &started))
						require.InDelta(t, 2000, started["queue_wait_ms"], 500)
					} else {
						require.NotContains(t, buf.String(), `"queue_wait_ms"`)
						require.Contains(t, buf.String(), "queue_wait_unavailable_reason")
					}
					if valid {
						require.Contains(t, buf.String(), eventID.String())
					} else {
						require.NotContains(t, buf.String(), `"event_id"`)
					}
				})
			}
		}
	}
}

func TestJudgeMalformedPayloadIsNacked(t *testing.T) {
	var buf bytes.Buffer
	msg := &testMessage{header: nats.Header{}, data: []byte("invalid")}
	uc := judgeFunc(func(context.Context, uuid.UUID) error { t.Fatal("must not judge invalid payload"); return nil })
	(&JudgeWorker{judgeUC: uc}).handleMessage(context.Background(), msg, logging.New(&buf))
	require.Equal(t, 1, msg.nak)
	require.Zero(t, msg.ack)
}
