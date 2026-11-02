package judging

import (
	"context"
	"errors"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"

	"github.com/google/uuid"
)

var ErrBusy = errors.New("judge job is active")
var ErrLeaseLost = errors.New("judge job lease lost")

type Job struct {
	ID           uuid.UUID
	SubmissionID uuid.UUID
	Generation   int64
	PackageID    *uuid.UUID
	PackageHash  string
	Attempt      int32
	LeaseToken   uuid.UUID
	Deadline     time.Time
}

type Jobs interface {
	Claim(context.Context, uuid.UUID) (Job, error)
	Renew(context.Context, Job) error
	Release(context.Context, Job) error
}

type Progress interface {
	PublishQueued(context.Context, uuid.UUID, models.SubmissionEventMeta) error
	PublishCompilingStarted(context.Context, uuid.UUID, models.SubmissionEventMeta) error
	PublishTestingStarted(context.Context, uuid.UUID, models.SubmissionEventMeta) error
	PublishTestStarted(context.Context, uuid.UUID, int32, models.SubmissionEventMeta) error
}
