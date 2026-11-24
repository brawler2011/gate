package contestsmodel

import (
	"time"

	"github.com/google/uuid"
)

const (
	BookletQueued    = "queued"
	BookletRunning   = "running"
	BookletSucceeded = "succeeded"
	BookletFailed    = "failed"
)

type Booklet struct {
	Language     string
	State        string
	SourceSHA256 string
	// Empty unless the build failed.
	Diagnostic string
	// Empty until the build succeeds.
	ObjectKey string
	UpdatedAt time.Time
}

// BookletJob is a claimed booklet build; Token fences its completion against a newer source.
type BookletJob struct {
	ContestID      uuid.UUID
	OrganizationID uuid.UUID
	Language       string
	Source         string
	Attempt        int
	Token          uuid.UUID
}
