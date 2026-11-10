package problemsmodel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type BuildOptions struct {
	Toolchain   Toolchain
	Limits      BuildLimits
	MaxAttempts int
	Duration    time.Duration
	Lease       time.Duration
}

type MutationKey struct {
	OrganizationID uuid.UUID
	Operation      string
	Key            string
	Fingerprint    string
	Token          uuid.UUID
}

type MutationResult struct {
	Status int
	Body   json.RawMessage
}

type ProblemBuild struct {
	ID             uuid.UUID       `json:"id"`
	ProblemID      uuid.UUID       `json:"problem_id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	Revision       int64           `json:"revision"`
	Snapshot       BuildSnapshot   `json:"-"`
	State          string          `json:"state"`
	Attempt        int             `json:"attempt"`
	MaxAttempts    int             `json:"max_attempts"`
	LeaseToken     *uuid.UUID      `json:"-"`
	LeaseDeadline  *time.Time      `json:"-"`
	Deadline       time.Time       `json:"deadline"`
	CreatedAt      time.Time       `json:"created_at"`
	FinishedAt     *time.Time      `json:"finished_at"`
	Diagnostic     json.RawMessage `json:"diagnostic"`
}

type StoredObject struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"-"`
	Purpose        string    `json:"purpose"`
	Key            string    `json:"-"`
	SHA256         string    `json:"sha256"`
	Size           int64     `json:"size"`
	State          string    `json:"-"`
	UploadDeadline time.Time `json:"-"`
}

// UploadChange atomically adds/replaces a resource or one test. A manual test
// with a provided answer can upload both bodies in the same revision change.
type UploadChange struct {
	AssetPath string     `json:"asset_path,omitempty"`
	Test      *DraftTest `json:"test,omitempty"`
	Part      string     `json:"part"` // asset, input, answer, pair
}

type UploadSpec struct {
	Purpose string
	SHA256  string
	Size    int64
}

type UploadReservation struct {
	Objects []StoredObject
	Replay  *MutationResult
}

type PackageDescriptor struct {
	ID        uuid.UUID
	BuildID   uuid.UUID
	ProblemID uuid.UUID
	Object    StoredObject
	Snapshot  BuildSnapshot
}
