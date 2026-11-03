package judgeworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/logging"
	"github.com/brawler2011/gate/backend/pkg/queuecontext"
	"github.com/brawler2011/gate/backend/pkg/requestid"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type judgeFunc func(context.Context, judgingmodel.JobRef) error

func (f judgeFunc) JudgeSubmission(ctx context.Context, reference judgingmodel.JobRef) error {
	return f(ctx, reference)
}

type testMessage struct {
	jetstream.Msg
	header         nats.Header
	data           []byte
	ack, nak, term int
	ackErr         error
}

func (m *testMessage) Headers() nats.Header { return m.header }

func (m *testMessage) Data() []byte { return m.data }

func (m *testMessage) Ack() error { m.ack++; return m.ackErr }

func (m *testMessage) NakWithDelay(time.Duration) error { m.nak++; return nil }

func (m *testMessage) Nak() error { m.nak++; return nil }

func (m *testMessage) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: 2}, nil
}

func TestJudgeMessageDiagnosticsAndAcknowledgement(t *testing.T) {
	for _, failure := range []error{nil, errors.New("Bearer secret-request-body"), context.DeadlineExceeded} {
		for _, queued := range []string{"", "garbage-secret", time.Now().Add(time.Hour).Format(time.RFC3339Nano), time.Now().Add(-2 * time.Second).Format(time.RFC3339Nano)} {
			for _, valid := range []bool{true, false} {
				t.Run(queued+"/"+strings.ReplaceAll(time.Now().String(), " ", ""), func(t *testing.T) {
					id, eventID, jobID := uuid.New(), uuid.New(), uuid.New()
					data, err := json.Marshal(submissionsmodel.SubmissionCreatedEvent{Id: id, JobID: jobID, Generation: 1, SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{CreatedAt: time.Now().Add(-24 * time.Hour)}})
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
					uc := judgeFunc(func(ctx context.Context, got judgingmodel.JobRef) error {
						require.Equal(t, id, got.SubmissionID)
						require.Equal(t, jobID, got.ID)
						require.EqualValues(t, 1, got.Generation)
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
					defer cancel()
					w := JudgeWorker{judgeUC: uc}
					w.handleMessage(ctx, msg, logging.New(&buf))
					if failure == nil {
						require.Equal(t, 1, msg.ack)
						require.Zero(t, msg.nak)
					} else {
						require.Zero(t, msg.ack)
						require.Equal(t, 1, msg.nak)
					}
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

func TestJudgeMalformedPayloadIsTerminated(t *testing.T) {
	var buf bytes.Buffer
	msg := &testMessage{header: nats.Header{}, data: []byte("invalid")}
	uc := judgeFunc(func(context.Context, judgingmodel.JobRef) error {
		t.Fatal("must not judge invalid payload")
		return nil
	})
	(&JudgeWorker{judgeUC: uc}).handleMessage(context.Background(), msg, logging.New(&buf))
	require.Equal(t, 1, msg.term)
	require.Zero(t, msg.nak)
	require.Zero(t, msg.ack)
}

func (m *testMessage) Term() error { m.term++; return nil }

func TestJudgeMessageRequiresJobReference(t *testing.T) {
	for _, event := range []submissionsmodel.SubmissionCreatedEvent{
		{Id: uuid.New(), Generation: 1},
		{Id: uuid.New(), JobID: uuid.New()},
		{Generation: 1, JobID: uuid.New()},
	} {
		data, err := json.Marshal(event)
		require.NoError(t, err)
		message := &testMessage{header: nats.Header{}, data: data}
		uc := judgeFunc(func(context.Context, judgingmodel.JobRef) error {
			t.Fatal("invalid reference must not reach judging")
			return nil
		})
		(&JudgeWorker{judgeUC: uc}).handleMessage(context.Background(), message, logging.New(io.Discard))
		require.Equal(t, 1, message.term)
		require.Zero(t, message.ack)
		require.Zero(t, message.nak)
	}
}

func TestBlockedSubmissionDoesNotStartJudging(t *testing.T) {
	data, err := json.Marshal(submissionsmodel.SubmissionCreatedEvent{Id: uuid.New(), Generation: 1, State: submissionsmodel.Disqualified})
	require.NoError(t, err)
	message := &testMessage{header: nats.Header{}, data: data}
	uc := judgeFunc(func(context.Context, judgingmodel.JobRef) error {
		t.Fatal("blocked submission must not reach judging")
		return nil
	})
	(&JudgeWorker{judgeUC: uc}).handleMessage(context.Background(), message, logging.New(io.Discard))
	require.Equal(t, 1, message.ack)
	require.Zero(t, message.term)
	require.Zero(t, message.nak)
}

func TestJudgePanicRetriesWithoutAcknowledgement(t *testing.T) {
	data, err := json.Marshal(submissionsmodel.SubmissionCreatedEvent{Id: uuid.New(), JobID: uuid.New(), Generation: 1})
	require.NoError(t, err)
	message := &testMessage{header: nats.Header{}, data: data}
	uc := judgeFunc(func(context.Context, judgingmodel.JobRef) error { panic("secret") })
	var logs bytes.Buffer
	require.NotPanics(t, func() { (&JudgeWorker{judgeUC: uc}).handleMessage(context.Background(), message, logging.New(&logs)) })
	require.Equal(t, 1, message.nak)
	require.Zero(t, message.ack)
	require.NotContains(t, logs.String(), "secret")
}
