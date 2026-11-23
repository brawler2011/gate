package submissions

import (
	"context"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
)

type noticeLog struct{ notices []eventdata.Notice }

func (l *noticeLog) Notify(_ context.Context, n eventdata.Notice) error {
	l.notices = append(l.notices, n)
	return nil
}
