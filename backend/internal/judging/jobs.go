package judging

import (
	"context"
	"errors"

	judgingmodel "github.com/brawler2011/gate/backend/internal/judging/model"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

var ErrBusy = errors.New("judge job is active")

var ErrLeaseLost = errors.New("judge job lease lost")

type Jobs interface {
	Claim(context.Context, judgingmodel.JobRef) (judgingmodel.Job, error)
	Renew(context.Context, judgingmodel.Job) error
	Release(context.Context, judgingmodel.Job) error
}

type Progress interface {
	PublishQueued(context.Context, uuid.UUID, submissionsmodel.SubmissionEventMeta) error
	PublishCompilingStarted(context.Context, uuid.UUID, submissionsmodel.SubmissionEventMeta) error
	PublishTestingStarted(context.Context, uuid.UUID, submissionsmodel.SubmissionEventMeta) error
	PublishTestStarted(context.Context, uuid.UUID, int32, submissionsmodel.SubmissionEventMeta) error
}
