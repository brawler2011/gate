package observer

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	eventQueueCapacity = 1000
	writeTimeout       = 10 * time.Second
	readLimit          = 1024
)

type eventConnection interface {
	SetWriteDeadline(time.Time) error
	WriteMessage(int, []byte) error
	WriteControl(int, []byte, time.Time) error
}

type websocketConnection interface {
	eventConnection
	SetReadLimit(int64)
	ReadMessage() (int, []byte, error)
	Close() error
}

// The queue is never closed: broadcast may race with connection cleanup.
// done and closeOnce coordinate all sources of disconnection instead.
type subscription struct {
	id           uuid.UUID
	conn         websocketConnection
	outbox       chan *WrappedEvent
	done         chan struct{}
	closeOnce    sync.Once
	queueMu      sync.Mutex
	verify       func() error
	onInvalidate func(error)
	// detach removes the subscription from its hub. It must not be called while
	// holding the hub lock and must tolerate repeated calls.
	detach func()
}

func newSubscription(conn websocketConnection) *subscription {
	s := &subscription{
		id:     uuid.New(),
		outbox: make(chan *WrappedEvent, eventQueueCapacity),
		done:   make(chan struct{}),
	}
	s.conn = unregisteringConnection{websocketConnection: conn, sub: s}
	return s
}

// unregister detaches the subscription before the client can observe its end,
// so a client that saw the close frame and reconnected is never matched with
// its previous, still registered subscription.
func (s *subscription) unregister() {
	if s.detach != nil {
		s.detach()
	}
}

// unregisteringConnection unregisters the subscription before every close frame,
// whichever code path writes it.
type unregisteringConnection struct {
	websocketConnection
	sub *subscription
}

func (c unregisteringConnection) WriteControl(messageType int, data []byte, deadline time.Time) error {
	if messageType == websocket.CloseMessage {
		c.sub.unregister()
	}
	return c.websocketConnection.WriteControl(messageType, data, deadline)
}

// enqueue and invalidation share a lock so a closed queue cannot be repopulated.
func (s *subscription) enqueue(event *WrappedEvent) bool {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	select {
	case <-s.done:
		return true
	default:
	}
	select {
	case s.outbox <- event:
		return true
	default:
		return false
	}
}
func (s *subscription) close(reason string, err error) {
	s.closeOnce.Do(func() {
		s.unregister()
		s.queueMu.Lock()
		close(s.done)
	drain:
		for {
			select {
			case <-s.outbox:
			default:
				break drain
			}
		}
		s.queueMu.Unlock()
		_ = s.conn.Close()
		slog.Info("websocket disconnected", "id", s.id, "reason", reason, "error", err)
	})
}

// Call while holding the hub lock so history cannot be evicted by broadcast.
func snapshotHistory(ring *RingBuffer[*WrappedEvent], since uint64, replay bool) ([]*WrappedEvent, error) {
	barrier := ring.MaxSeq()
	if !replay || barrier == 0 {
		return nil, nil
	}
	history, err := ring.GetRange(since, barrier)
	if errors.Is(err, ErrBufferEmpty) {
		return nil, nil
	}
	return history, err
}

func rejectHistory(conn websocketConnection, err error) {
	sub := newSubscription(conn)
	code, message := getClosingFrameData(err)
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, string(message)), time.Now().Add(timeToClose))
	sub.close("cannot get history", err)
}

func (s *subscription) startReadPump() <-chan struct{} {
	readDone := make(chan struct{})
	s.conn.SetReadLimit(readLimit)
	go func() {
		defer close(readDone)
		for {
			if _, _, err := s.conn.ReadMessage(); err != nil {
				s.close("read failed", err)
				return
			}
		}
	}()
	return readDone
}

func (s *subscription) run(history []*WrappedEvent, matches func(*WrappedEvent) bool, send func(*WrappedEvent) error) {
	readDone := s.startReadPump()
	defer func() {
		s.close("sender stopped", nil)
		<-readDone
	}()

	deliver := func(event *WrappedEvent) bool {
		select {
		case <-s.done:
			return false
		default:
		}
		if s.verify != nil {
			if err := s.verify(); err != nil {
				if s.onInvalidate != nil {
					s.onInvalidate(err)
				}
				_ = closeVerification(s.conn, err)
				s.close("access invalidated", err)
				return false
			}
		}
		if err := send(event); err != nil {
			s.close("write failed", err)
			return false
		}
		return true
	}
	for _, event := range history {
		if matches(event) && !deliver(event) {
			return
		}
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if s.verify != nil {
				if err := s.verify(); err != nil {
					if s.onInvalidate != nil {
						s.onInvalidate(err)
					}
					_ = closeVerification(s.conn, err)
					return
				}
			}
		case <-s.done:
			return
		case event := <-s.outbox:
			if !deliver(event) {
				return
			}
		}
	}
}
