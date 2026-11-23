package judging

import (
	"context"
	"errors"

	eventdata "github.com/brawler2011/gate/backend/internal/events"
	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"
)

var ErrLeaseLost = errors.New("judge job lease lost")

type Jobs interface {
	ClaimNext(context.Context) (judgingmodel.Job, bool, error)
	Renew(context.Context, judgingmodel.Job) error
	Release(context.Context, judgingmodel.Job) error
	PickStalledJobs(context.Context, int32) ([]judgingmodel.Job, error)
	DeleteObsolete(context.Context) error
}

// Progress carries transient judging progress outside any transaction.
type Progress = eventdata.Writer
