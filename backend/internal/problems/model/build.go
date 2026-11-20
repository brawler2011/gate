package problemsmodel

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type BuildOptions struct {
	Languages   map[string]sandbox.Language
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
	ID             uuid.UUID           `json:"id"`
	ProblemID      uuid.UUID           `json:"problem_id"`
	OrganizationID uuid.UUID           `json:"organization_id"`
	Revision       int64               `json:"revision"`
	Snapshot       BuildSnapshot       `json:"-"`
	State          string              `json:"state"`
	Attempt        int                 `json:"attempt"`
	MaxAttempts    int                 `json:"max_attempts"`
	LeaseToken     *uuid.UUID          `json:"-"`
	LeaseDeadline  *time.Time          `json:"-"`
	Deadline       time.Time           `json:"deadline"`
	CreatedAt      time.Time           `json:"created_at"`
	FinishedAt     *time.Time          `json:"finished_at"`
	Diagnostic     json.RawMessage     `json:"diagnostic"`
	Solutions      []SolutionResult    `json:"solutions"`
	ProgramTests   []ProgramTestResult `json:"program_tests"`
}

type StoredObject struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"-"`
	Purpose        string    `json:"purpose"`
	Key            string    `json:"-"`
	SHA256         string    `json:"sha256"`
	Size           int64     `json:"size"`
}

// NewStoredObject reserves a key; insert its row only after upload, atomically with its first reference.
func NewStoredObject(org uuid.UUID, spec UploadSpec) StoredObject {
	id := uuid.New()
	return StoredObject{ID: id, OrganizationID: org, Purpose: spec.Purpose, Key: fmt.Sprintf("objects/%s/%s", org, id), SHA256: spec.SHA256, Size: spec.Size}
}

// UploadChange sets exactly one entry; uploaded files fill its slots by name (UploadSlots) in one revision change.
type UploadChange struct {
	AssetName     string              `json:"asset_name,omitempty"`
	Test          *DraftTest          `json:"test,omitempty"`
	CheckerTest   *DraftCheckerTest   `json:"checker_test,omitempty"`
	Validator     string              `json:"validator,omitempty"`
	ValidatorTest *DraftValidatorTest `json:"validator_test,omitempty"`
}

var UploadSlots = []string{"asset", "input", "output", "answer"}

// Purpose names what an object uploaded into slot holds, or "" when the change has no such slot.
func (c *UploadChange) Purpose(slot string) string {
	switch {
	case c.AssetName != "" && slot == "asset":
		return "asset"
	case c.Test != nil && slot == "input":
		return "test-input"
	case c.Test != nil && slot == "answer":
		return "test-answer"
	case c.CheckerTest != nil && (slot == "input" || slot == "output" || slot == "answer"):
		return "checker-test"
	case c.ValidatorTest != nil && slot == "input":
		return "validator-test"
	}
	return ""
}

type UploadSpec struct {
	Purpose string
	SHA256  string
	Size    int64
}

type PackageDescriptor struct {
	ID        uuid.UUID
	BuildID   uuid.UUID
	ProblemID uuid.UUID
	Object    StoredObject
	Snapshot  BuildSnapshot
}
