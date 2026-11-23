package contests

import (
	"context"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
)

type noticeLog struct{ notices []eventdata.Notice }

func (l *noticeLog) Notify(_ context.Context, n eventdata.Notice) error {
	l.notices = append(l.notices, n)
	return nil
}

type testTransactions[T any] struct{ store T }

func (t testTransactions[T]) Within(_ context.Context, fn func(T) error) error { return fn(t.store) }
