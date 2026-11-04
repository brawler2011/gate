package judgingmodel

import (
	"time"

	"github.com/google/uuid"
)

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

type JobRef struct {
	ID           uuid.UUID
	SubmissionID uuid.UUID
	Generation   int64
}

func (j Job) Ref() JobRef {
	return JobRef{ID: j.ID, SubmissionID: j.SubmissionID, Generation: j.Generation}
}
