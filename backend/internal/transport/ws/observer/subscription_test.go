package observer

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// A controlled connection blocks its first event write until release, timeout,
// or transport closure. Timeout is signalled explicitly, without a real 10s wait.
type controlledConnection struct {
	closed      chan struct{}
	started     chan struct{}
	release     chan struct{}
	expire      chan struct{}
	readFailure chan error
	writes      chan string
	deadline    chan time.Time
	readLimit   atomic.Int64
	closeCount  atomic.Int32
	deadlineErr error
	onClose     func()
	once        sync.Once
}

func newControlledConnection() *controlledConnection {
	return &controlledConnection{
		closed: make(chan struct{}), started: make(chan struct{}),
		release: make(chan struct{}), expire: make(chan struct{}), readFailure: make(chan error, 1),
		writes: make(chan string, 2000), deadline: make(chan time.Time, 2000),
	}
}

func (c *controlledConnection) SetReadLimit(limit int64) { c.readLimit.Store(limit) }
func (c *controlledConnection) SetWriteDeadline(deadline time.Time) error {
	c.deadline <- deadline
	return c.deadlineErr
}
func (c *controlledConnection) ReadMessage() (int, []byte, error) {
	select {
	case <-c.closed:
		return 0, nil, net.ErrClosed
	case err := <-c.readFailure:
		return 0, nil, err
	}
}
func (c *controlledConnection) WriteControl(int, []byte, time.Time) error { return nil }
func (c *controlledConnection) WriteMessage(_ int, payload []byte) error {
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.closed:
		return net.ErrClosed
	case <-c.expire:
		return os.ErrDeadlineExceeded
	case <-c.release:
		c.writes <- string(payload)
		return nil
	}
}
func (c *controlledConnection) Close() error {
	if c.closeCount.Add(1) == 1 {
		if c.onClose != nil {
			c.onClose()
		}
		close(c.closed)
	}
	return nil
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for connection progress")
		var zero T
		return zero
	}
}

type queueHarness struct {
	subscribe func(websocketConnection, uint64, bool) (*subscription, []*WrappedEvent, error)
	broadcast func(*WrappedEvent)
	count     func() int
	remove    func(uuid.UUID)
	event     func(uint64, bool) *WrappedEvent
}

func newQueueHarness(kind string) *queueHarness {
	contestID := uuid.New()
	q := &queueHarness{}
	if kind == "submissions" {
		h := NewHub(2, nil, nil)
		q.subscribe = func(conn websocketConnection, since uint64, replay bool) (*subscription, []*WrappedEvent, error) {
			sub, history, err := h.subscribe(conn, &SubmissionsFilter{ContestId: &contestID, Since: since}, replay)
			if err != nil {
				return nil, history, err
			}
			return sub.subscription, history, nil
		}
		q.broadcast = h.broadcast
		q.count = func() int { h.mu.RLock(); defer h.mu.RUnlock(); return len(h.subscribers) }
		q.remove = func(id uuid.UUID) { h.mu.Lock(); defer h.mu.Unlock(); delete(h.subscribers, id) }
		q.event = func(seq uint64, matches bool) *WrappedEvent {
			id := contestID
			if !matches {
				id = uuid.New()
			}
			return &WrappedEvent{Seq: seq, DTOPayload: fmt.Appendf(nil, "%d", seq), RawEvent: &submissionsmodel.SubmissionCreatedEvent{SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{ContestId: &id}}}
		}
	} else {
		h := NewContestsHub(2)
		q.subscribe = func(conn websocketConnection, since uint64, replay bool) (*subscription, []*WrappedEvent, error) {
			sub, history, err := h.subscribe(conn, &ContestsFilter{ContestId: contestID, Since: since}, replay)
			if err != nil {
				return nil, history, err
			}
			return sub.subscription, history, nil
		}
		q.broadcast = h.broadcast
		q.count = func() int { h.mu.RLock(); defer h.mu.RUnlock(); return len(h.subscribers) }
		q.remove = func(id uuid.UUID) { h.mu.Lock(); defer h.mu.Unlock(); delete(h.subscribers, id) }
		q.event = func(seq uint64, matches bool) *WrappedEvent {
			id := contestID
			if !matches {
				id = uuid.New()
			}
			return &WrappedEvent{Seq: seq, DTOPayload: fmt.Appendf(nil, "%d", seq), RawEvent: &contestsmodel.ContestAnnouncementDeletedEvent{ContestID: id}}
		}
	}
	return q
}

func (q *queueHarness) run(sub *subscription, history []*WrappedEvent) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer q.remove(sub.id)
		sub.run(history, func(*WrappedEvent) bool { return true }, func(event *WrappedEvent) error {
			return writeEvent(sub.conn, event.DTOPayload)
		})
	}()
	return done
}

func TestSnapshotQueueOrder(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		t.Run(kind, func(t *testing.T) {
			q := newQueueHarness(kind)
			q.broadcast(q.event(1, true))
			q.broadcast(q.event(2, true))
			conn := newControlledConnection()
			sub, history, err := q.subscribe(conn, 0, true)
			require.NoError(t, err)
			done := q.run(sub, history)
			t.Cleanup(func() { sub.close("test finished", nil); receive(t, done) })
			receive(t, conn.started)
			// Evict the entire ring while the first history write is paused.
			q.broadcast(q.event(3, true))
			q.broadcast(q.event(4, false))
			q.broadcast(q.event(5, true))
			require.Len(t, sub.outbox, 2)
			close(conn.release)
			for _, seq := range []string{"1", "2", "3", "5"} {
				require.Equal(t, seq, receive(t, conn.writes))
			}
			q.broadcast(q.event(6, true))
			require.Equal(t, "6", receive(t, conn.writes))
			require.Empty(t, conn.writes)
		})
	}
}

func TestQueueOverflowIsolatesSlowSubscriber(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, replay := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/history=%t", kind, replay), func(t *testing.T) {
				q := newQueueHarness(kind)
				q.broadcast(q.event(1, true))
				slow := newControlledConnection()
				// Acquiring the hub lock here proves transport closure is outside it.
				slow.onClose = func() { q.count() }
				sub, history, err := q.subscribe(slow, 0, replay)
				require.NoError(t, err)
				done := q.run(sub, history)
				t.Cleanup(func() { sub.close("test finished", nil); receive(t, done) })
				seq := uint64(2)
				if !replay {
					q.broadcast(q.event(seq, true))
					seq++
				}
				receive(t, slow.started)
				healthy := newControlledConnection()
				close(healthy.release)
				other, _, err := q.subscribe(healthy, 0, false)
				require.NoError(t, err)
				otherDone := q.run(other, nil)
				t.Cleanup(func() { other.close("test finished", nil); receive(t, otherDone) })
				for i := 0; i < eventQueueCapacity; i++ {
					q.broadcast(q.event(seq, true))
					require.Equal(t, fmt.Sprint(seq), receive(t, healthy.writes))
					seq++
				}
				require.Len(t, sub.outbox, eventQueueCapacity)
				require.Equal(t, eventQueueCapacity, cap(sub.outbox))
				require.Equal(t, 2, q.count())
				q.broadcast(q.event(seq, true))
				receive(t, slow.closed)
				receive(t, done)
				require.Equal(t, 1, q.count())
				require.Equal(t, fmt.Sprint(seq), receive(t, healthy.writes))
				sub.close("concurrent cleanup", errors.New("write failed"))
				require.EqualValues(t, 1, slow.closeCount.Load())
				q.broadcast(q.event(seq+1, true))
				require.Equal(t, fmt.Sprint(seq+1), receive(t, healthy.writes))
				require.Len(t, sub.outbox, eventQueueCapacity)
			})
		}
	}
}

func TestWriteFailureReleasesSubscriptionAndReader(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, phase := range []string{"history", "live", "deadline setup", "read failure"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				q := newQueueHarness(kind)
				q.broadcast(q.event(1, true))
				conn := newControlledConnection()
				if phase == "deadline setup" {
					conn.deadlineErr = net.ErrClosed
				}
				sub, history, err := q.subscribe(conn, 0, phase != "live")
				require.NoError(t, err)
				before := time.Now()
				done := q.run(sub, history)
				t.Cleanup(func() { sub.close("test finished", nil); receive(t, done) })
				if phase == "live" {
					q.broadcast(q.event(2, true))
				}
				deadline := receive(t, conn.deadline)
				require.WithinDuration(t, before.Add(10*time.Second), deadline, time.Second)
				if phase != "deadline setup" {
					receive(t, conn.started)
					if phase == "read failure" {
						conn.readFailure <- errors.New("client disconnected")
					} else {
						close(conn.expire)
					}
				}
				receive(t, done)
				require.Zero(t, q.count())
				require.EqualValues(t, 1024, conn.readLimit.Load())
				require.EqualValues(t, 1, conn.closeCount.Load())
			})
		}
	}
}

func TestIncomingMessageLimit(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, fragmented := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fragmented=%t", kind, fragmented), func(t *testing.T) {
				f := newWSFixture(t)
				dialer := websocket.Dialer{WriteBufferSize: 128}
				conn, resp, err := dialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+f.path(kind), http.Header{"Cookie": {"session_id=" + f.session.String()}})
				if resp != nil {
					require.NoError(t, resp.Body.Close())
				}
				require.NoError(t, err)
				t.Cleanup(func() { _ = conn.Close() })
				// Exactly 1 KiB is accepted; an event confirms the read pump is alive.
				require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, make([]byte, 1024)))
				if kind == "submissions" {
					f.publish(t, submissionsmodel.SubmissionEventCreated, &submissionsmodel.SubmissionCreatedEvent{Id: uuid.New(), SubmissionEventMeta: f.meta(f.stub.user.Id)})
				} else {
					f.publish(t, contestsmodel.ContestEventAnnouncementDeleted, &contestsmodel.ContestAnnouncementDeletedEvent{ID: uuid.New(), ContestID: f.stub.contest.ID})
				}
				require.Contains(t, readPayload(t, conn), "id")
				if fragmented {
					writer, err := conn.NextWriter(websocket.TextMessage)
					require.NoError(t, err)
					for range 9 {
						_, err = writer.Write(make([]byte, 128))
						require.NoError(t, err)
					}
					require.NoError(t, writer.Close())
				} else {
					require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, make([]byte, 1025)))
				}
				requireClose(t, conn, websocket.CloseMessageTooBig)
				require.Eventually(t, func() bool {
					if kind == "submissions" {
						f.hub.mu.RLock()
						defer f.hub.mu.RUnlock()
						return len(f.hub.subscribers) == 0
					}
					f.contests.mu.RLock()
					defer f.contests.mu.RUnlock()
					return len(f.contests.subscribers) == 0
				}, time.Second, time.Millisecond)
			})
		}
	}
}

func TestHistoryRangeErrors(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		t.Run(kind, func(t *testing.T) {
			f := newWSFixture(t)
			f.hub.ring = NewRingBuffer[*WrappedEvent](1)
			f.contests.ring = NewRingBuffer[*WrappedEvent](1)
			for range 2 {
				if kind == "submissions" {
					f.publish(t, submissionsmodel.SubmissionEventCreated, &submissionsmodel.SubmissionCreatedEvent{Id: uuid.New(), SubmissionEventMeta: f.meta(f.stub.user.Id)})
				} else {
					f.publish(t, contestsmodel.ContestEventAnnouncementDeleted, &contestsmodel.ContestAnnouncementDeletedEvent{ID: uuid.New(), ContestID: f.stub.contest.ID})
				}
			}
			conn := f.connect(t, f.path(kind))
			requireClose(t, conn, 4409)
			for _, test := range []struct {
				err    error
				code   int
				reason string
			}{
				{ErrBufferEmpty, 4408, "buffer_empty"},
				{ErrHistoryLost, 4409, "history_lost"},
				{ErrInvalidRange, 4410, "invalid_range"},
			} {
				code, reason := getClosingFrameData(test.err)
				require.Equal(t, test.code, code)
				require.Equal(t, test.reason, string(reason))
			}
		})
	}
}

// Accelerate only transport write deadlines, preserving the real WebSocket
// writer and read pump. A deadline failure must release the HTTP subscription.
type expiringListener struct {
	net.Listener
	deadlines chan time.Time
}

func (l *expiringListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept deadline test connection: %w", err)
	}
	return &expiringConn{Conn: conn, deadlines: l.deadlines}, nil
}

type expiringConn struct {
	net.Conn
	deadlines chan time.Time
}

func (c *expiringConn) SetWriteDeadline(deadline time.Time) error {
	c.deadlines <- deadline
	if err := c.Conn.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		return fmt.Errorf("expire test write deadline: %w", err)
	}
	return nil
}

func TestTransportWriteDeadline(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		t.Run(kind, func(t *testing.T) {
			f := newWSFixture(t)
			f.server.Close()
			server := httptest.NewUnstartedServer(f.server.Config.Handler)
			deadlines := make(chan time.Time, 10)
			server.Listener = &expiringListener{Listener: server.Listener, deadlines: deadlines}
			server.Start()
			f.server = server
			t.Cleanup(server.Close)
			conn := f.connect(t, f.path(kind))
			before := time.Now()
			if kind == "submissions" {
				f.publish(t, submissionsmodel.SubmissionEventCreated, &submissionsmodel.SubmissionCreatedEvent{Id: uuid.New(), SubmissionEventMeta: f.meta(f.stub.user.Id)})
			} else {
				f.publish(t, contestsmodel.ContestEventAnnouncementDeleted, &contestsmodel.ContestAnnouncementDeletedEvent{ID: uuid.New(), ContestID: f.stub.contest.ID})
			}
			require.WithinDuration(t, before.Add(10*time.Second), receive(t, deadlines), time.Second)
			requireClose(t, conn, websocket.CloseAbnormalClosure)
			require.Eventually(t, func() bool {
				if kind == "submissions" {
					f.hub.mu.RLock()
					defer f.hub.mu.RUnlock()
					return len(f.hub.subscribers) == 0
				}
				f.contests.mu.RLock()
				defer f.contests.mu.RUnlock()
				return len(f.contests.subscribers) == 0
			}, time.Second, time.Millisecond)
		})
	}
}
