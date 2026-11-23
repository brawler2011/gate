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

// Explicit timeout signals avoid waiting for real transport deadlines.
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
	subscribe func(websocketConnection) *subscription
	broadcast func(*WrappedEvent)
	count     func() int
	event     func(uint64, bool) *WrappedEvent
}

func newQueueHarness(kind string) *queueHarness {
	contestID := uuid.New()
	q := &queueHarness{}
	if kind == "submissions" {
		h := NewHub(nil, nil)
		q.subscribe = func(conn websocketConnection) *subscription {
			sub := h.subscribe(&SubmissionsFilter{ContestId: &contestID})
			sub.attach(conn)
			return sub.subscription
		}
		q.broadcast = h.broadcast
		q.count = func() int { h.mu.RLock(); defer h.mu.RUnlock(); return len(h.subscribers) }
		q.event = func(seq uint64, matches bool) *WrappedEvent {
			id := contestID
			if !matches {
				id = uuid.New()
			}
			return &WrappedEvent{DTOPayload: fmt.Appendf(nil, "%d", seq), RawEvent: &submissionsmodel.SubmissionCreatedEvent{SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{ContestId: &id}}}
		}
	} else {
		h := NewContestsHub()
		q.subscribe = func(conn websocketConnection) *subscription {
			sub := h.subscribe(&ContestsFilter{ContestId: contestID})
			sub.attach(conn)
			return sub.subscription
		}
		q.broadcast = h.broadcast
		q.count = func() int { h.mu.RLock(); defer h.mu.RUnlock(); return len(h.subscribers) }
		q.event = func(seq uint64, matches bool) *WrappedEvent {
			id := contestID
			if !matches {
				id = uuid.New()
			}
			return &WrappedEvent{DTOPayload: fmt.Appendf(nil, "%d", seq), RawEvent: &contestsmodel.ContestAnnouncementDeletedEvent{ContestID: id}}
		}
	}
	return q
}

func (q *queueHarness) run(sub *subscription) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		sub.run(func(event *WrappedEvent) error {
			return writeEvent(sub.conn, event.DTOPayload)
		})
	}()
	return done
}

func TestQueueOverflowIsolatesSlowSubscriber(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		t.Run(kind, func(t *testing.T) {
			q := newQueueHarness(kind)
			slow := newControlledConnection()
			// Acquiring the hub lock here proves transport closure is outside it.
			slow.onClose = func() { q.count() }
			sub := q.subscribe(slow)
			done := q.run(sub)
			t.Cleanup(func() { sub.close("test finished", nil); receive(t, done) })
			seq := uint64(1)
			q.broadcast(q.event(seq, true))
			seq++
			receive(t, slow.started)
			healthy := newControlledConnection()
			close(healthy.release)
			other := q.subscribe(healthy)
			otherDone := q.run(other)
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
			require.Empty(t, sub.outbox)
		})
	}
}

func TestWriteFailureReleasesSubscriptionAndReader(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, phase := range []string{"live", "deadline setup", "read failure"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				q := newQueueHarness(kind)
				conn := newControlledConnection()
				if phase == "deadline setup" {
					conn.deadlineErr = net.ErrClosed
				}
				sub := q.subscribe(conn)
				before := time.Now()
				done := q.run(sub)
				t.Cleanup(func() { sub.close("test finished", nil); receive(t, done) })
				q.broadcast(q.event(1, true))
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

// Accelerate write deadlines; a real WebSocket timeout must release the HTTP subscription.
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
