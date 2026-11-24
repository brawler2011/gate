package contestsmodel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
)

type BookletState string

const (
	BookletQueued    BookletState = "queued"
	BookletRunning   BookletState = "running"
	BookletSucceeded BookletState = "succeeded"
	BookletFailed    BookletState = "failed"
)

type BookletProblem struct {
	Ordinal   int       `json:"ordinal"`
	ProblemID uuid.UUID `json:"problem_id"`
	PackageID uuid.UUID `json:"package_id"`
	// The package's, so its id covers them in the key.
	Title         string `json:"title"`
	TimeLimitMs   int    `json:"time_limit_ms"`
	MemoryLimitMb int    `json:"memory_limit_mb"`
	InputFile     string `json:"input_file"`
	OutputFile    string `json:"output_file"`
}

// BookletInputs is everything a booklet prints; packages are immutable, so their ids stand for their statements.
type BookletInputs struct {
	ContestID      uuid.UUID
	OrganizationID uuid.UUID
	Language       string
	Title          string
	Place          string
	// Date is empty when the contest has no start time.
	Date     string
	Problems []BookletProblem
}

// NewBookletInputs prints the start date in the organization's time zone.
func NewBookletInputs(contestID, organizationID uuid.UUID, language, title, place, organizationName string, start *time.Time, timeZone string, problems []BookletProblem) (BookletInputs, error) {
	in := BookletInputs{ContestID: contestID, OrganizationID: organizationID, Language: language, Title: title, Place: place, Problems: problems}
	if in.Place == "" {
		in.Place = organizationName
	}
	if start != nil {
		location, err := time.LoadLocation(timeZone)
		if err != nil {
			return in, fmt.Errorf("organization time zone: %w", err)
		}
		in.Date = start.In(location).Format("02.01.2006")
	}
	return in, nil
}

// Key identifies the booklet these inputs make with the current generator.
func (in BookletInputs) Key() string {
	pairs := make([][2]string, len(in.Problems))
	for i, p := range in.Problems {
		pairs[i] = [2]string{ProblemLetter(p.Ordinal), p.PackageID.String()}
	}
	data, _ := json.Marshal([]any{statement.Version, in.Language, in.Title, in.Place, in.Date, pairs})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type StoredBooklet struct {
	Key        string
	State      BookletState
	Diagnostic string
	UpdatedAt  time.Time
	ObjectKey  string
}

type Booklet struct {
	BookletInputs
	// Stored is the last booklet queued for the contest and language, possibly for other inputs.
	Stored *StoredBooklet
}

// Current reports the stored booklet only when it was built from these inputs.
func (b Booklet) Current() (StoredBooklet, bool) {
	if b.Stored == nil || b.Stored.Key != b.Key() {
		return StoredBooklet{}, false
	}
	return *b.Stored, true
}

type BookletJob struct {
	ContestID      uuid.UUID
	OrganizationID uuid.UUID
	Language       string
	Key            string
	Attempt        int
	LeaseToken     uuid.UUID
}

func ProblemLetter(ordinal int) string {
	if ordinal <= 0 {
		return "A"
	}
	res := ""
	for ordinal > 0 {
		rem := (ordinal - 1) % 26
		res = string(rune('A'+rem)) + res
		ordinal = (ordinal - 1) / 26
	}
	return res
}
