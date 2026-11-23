package observer

import (
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

// Never close the queue: broadcast can race with connection cleanup.
type subscription struct {
	id           uuid.UUID
	conn         websocketConnection
	outbox       chan *WrappedEvent
	done         chan struct{}
	closeOnce    sync.Once
	queueMu      sync.Mutex
	verify       func() error
	onInvalidate func(error)
	// Detach must tolerate repeated calls and run outside the hub lock.
	detach func()
}

func newSubscription() *subscription {
	return &subscription{
		id:     uuid.New(),
		outbox: make(chan *WrappedEvent, eventQueueCapacity),
		done:   make(chan struct{}),
	}
}

// attach binds the upgraded connection; subscribing before the handshake lets a client's snapshot miss no event.
func (s *subscription) attach(conn websocketConnection) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	s.conn = unregisteringConnection{websocketConnection: conn, sub: s}
	select {
	case <-s.done:
		_ = conn.Close()
	default:
	}
}

// Unregister before sending a close frame so reconnects cannot match a previous subscription.
func (s *subscription) unregister() {
	if s.detach != nil {
		s.detach()
	}
}

// Library-written close frames bypass the wrapper; read-pump failure unregisters them.
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
		conn := s.conn
		s.queueMu.Unlock()
		if conn != nil {
			_ = conn.Close()
		}
		slog.Info("websocket disconnected", "id", s.id, "reason", reason, "error", err)
	})
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

func (s *subscription) run(send func(*WrappedEvent) error) {
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
					s.close("access invalidated", err)
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
