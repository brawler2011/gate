package contestsmodel

import (
	"slices"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/google/uuid"
)

type ContestVisibility = string

const (
	ContestVisibilityPublic  ContestVisibility = "public"
	ContestVisibilityPrivate ContestVisibility = "private"
)

type UserContestMembership struct {
	ContestID uuid.UUID
	Role      accessmodel.ContestRole
}

type FreezeStatus string

const (
	FreezeStatusAuto     FreezeStatus = "auto"
	FreezeStatusFrozen   FreezeStatus = "frozen"
	FreezeStatusUnfrozen FreezeStatus = "unfrozen"
)

// ContestScope is the lowest audience that may see a contest view.
type ContestScope string

const (
	ScopePublic      ContestScope = "public"
	ScopeParticipant ContestScope = "participant"
	ScopeModerator   ContestScope = "moderator"
)

const PenaltyPerAttempt int32 = 20

type CreateContestParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Visibility     ContestVisibility
	Title          string
	Login          string
	Description    string
	StartTime      *time.Time
	EndTime        *time.Time
}

type CreateContestInput struct {
	OrganizationID uuid.UUID
	Title          string
	Login          string
	Description    string
	Visibility     ContestVisibility
	StartTime      *time.Time
	EndTime        *time.Time
}

type ContestsFilter struct {
	Page      int32
	PageSize  int32
	OwnerId   *uuid.UUID
	SortOrder paging.SortOrder
	Search    string
}

type AdminContestsFilter struct {
	Page       int32
	PageSize   int32
	Search     string
	Visibility *ContestVisibility
	SortBy     string
	SortOrder  paging.SortOrder
}

type UserContestsFilter struct {
	Page      int32
	PageSize  int32
	UserId    uuid.UUID
	Search    string
	SortBy    string
	SortOrder paging.SortOrder
}

type WorkshopContestsFilter struct {
	Page           int32
	PageSize       int32
	UserId         uuid.UUID
	Search         string
	SortBy         string
	SortOrder      paging.SortOrder
	OrganizationID *uuid.UUID
}

type PublicContestsFilter struct {
	Page      int32
	PageSize  int32
	Search    string
	SortBy    string
	SortOrder paging.SortOrder
}

type ContestUpdateInput struct {
	ID           uuid.UUID
	Login        *string
	Title        *string
	Description  *string
	Visibility   *ContestVisibility
	Settings     ContestSettingsPatch
	StartTime    *time.Time
	StartTimeSet bool
	EndTime      *time.Time
	EndTimeSet   bool
}

type ContestProblemGet struct {
	ContestId uuid.UUID
	ProblemId uuid.UUID
}

type ContestProblemCreation struct {
	ContestId uuid.UUID
	ProblemId uuid.UUID
	PackageId uuid.UUID
}

type ContestProblemDeletion struct {
	ContestId uuid.UUID
	ProblemId uuid.UUID
}

type ContestProblemReorderItem struct {
	ProblemID uuid.UUID `json:"problem_id"`
	Position  int32     `json:"position"`
}

type ParticipantDeletion struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
}

type ParticipantsFilter struct {
	Page      int32
	PageSize  int32
	ContestId uuid.UUID
}

type ContestPermissionGet struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
}

type CreateContestMemberInput struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
	Role      string
}

type CreateContestMemberParams struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
	Role      accessmodel.ContestRole
}

type ContestUserProblemBlock struct {
	ContestID uuid.UUID  `json:"contest_id"`
	UserID    uuid.UUID  `json:"user_id"`
	ProblemID uuid.UUID  `json:"problem_id"`
	Reason    *string    `json:"reason,omitempty"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type CreateContestUserProblemBlockParams struct {
	ContestID uuid.UUID
	UserID    uuid.UUID
	ProblemID uuid.UUID
	Reason    *string
	CreatedBy *uuid.UUID
}

type Contest struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	OrganizationLogin string
	Visibility        ContestVisibility
	Title             string
	Login             string
	Description       string
	Settings          ContestSettings
	StartTime         *time.Time
	EndTime           *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ContestsList struct {
	Contests   []Contest
	Pagination paging.Pagination
}

type ContestProblem struct {
	TimeLimitMs   int
	MemoryLimitMb int
	InputFile     string
	OutputFile    string
	UpdatedAt     time.Time
	ContestID     uuid.UUID
	ProblemID     uuid.UUID
	PackageID     uuid.UUID
	Ordinal       int
	Title         string
	ShortName     string
	Visibility    string
	CreatedAt     time.Time
}

type ContestMember struct {
	UserID      uuid.UUID
	ContestID   uuid.UUID
	Username    string
	Role        accessmodel.UserRole
	ContestRole accessmodel.ContestRole
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ContestJoinRequest struct {
	ID                uuid.UUID
	ContestID         uuid.UUID
	ContestTitle      string
	ContestLogin      string
	OrganizationLogin string
	UserID            uuid.UUID
	Username          string
	Email             string
	Message           *string
	Status            organizationsmodel.RequestStatus
	ReviewedBy        *uuid.UUID
	ReviewerUsername  *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateContestJoinRequestInput struct {
	ContestID uuid.UUID
	UserID    uuid.UUID
	Message   *string
}

type ContestSettings struct {
	JoinPolicy             accessmodel.JoinPolicy
	MonitorScope           ContestScope
	SubmissionsListScope   ContestScope
	SubmissionsReviewScope ContestScope
	SubmissionDetailsScope ContestScope
	FreezeStatus           FreezeStatus
	FreezeDurationMinutes  *int32
	FreezeStartedAt        *time.Time
	HideStatements         bool
	AllowClarifications    bool
	EnableDrafts           bool
	EnableUpsolving        bool
	AllowVirtual           bool
	Languages              []string
}

// Nil fields keep the stored value; FreezeDurationSet distinguishes clearing the duration from omitting it.
type ContestSettingsPatch struct {
	JoinPolicy             *accessmodel.JoinPolicy
	MonitorScope           *ContestScope
	SubmissionsListScope   *ContestScope
	SubmissionsReviewScope *ContestScope
	SubmissionDetailsScope *ContestScope
	FreezeStatus           *FreezeStatus
	FreezeDurationMinutes  *int32
	FreezeDurationSet      bool
	HideStatements         *bool
	AllowClarifications    *bool
	EnableDrafts           *bool
	EnableUpsolving        *bool
	AllowVirtual           *bool
	Languages              []string
}

// Contest language restrictions do not apply to problem programs.
func (s ContestSettings) AllowsLanguage(id string) bool {
	return len(s.Languages) == 0 || slices.Contains(s.Languages, id)
}

func (c *Contest) GetFreezeTime() *time.Time {
	if c.Settings.FreezeStatus == FreezeStatusFrozen {
		return c.Settings.FreezeStartedAt
	}
	duration := c.Settings.FreezeDurationMinutes
	if duration == nil || c.EndTime == nil {
		return nil
	}
	freezeTime := c.EndTime.Add(-time.Duration(*duration) * time.Minute)
	if c.StartTime != nil && freezeTime.Before(*c.StartTime) {
		freezeTime = *c.StartTime
	}
	return &freezeTime
}

func (c *Contest) IsFrozenAt(t time.Time) bool {
	if c.StartTime != nil && t.Before(*c.StartTime) || c.EndTime != nil && !t.Before(*c.EndTime) {
		return false
	}
	switch c.Settings.FreezeStatus {
	case FreezeStatusUnfrozen:
		return false
	case FreezeStatusFrozen:
		boundary := c.GetFreezeTime()
		return boundary == nil || !t.Before(*boundary)
	case FreezeStatusAuto:
		freezeTime := c.GetFreezeTime()
		if freezeTime == nil {
			return false
		}
		return !t.Before(*freezeTime)
	default:
		return false
	}
}

// VirtualRun is a user's personal replay of a finished contest within [StartedAt, EndsAt); FinishedAt ends it early.
type VirtualRun struct {
	UserID     uuid.UUID
	Username   string
	StartedAt  time.Time
	EndsAt     time.Time
	FinishedAt *time.Time
}

// Over is when the run stopped or will stop accepting submissions.
func (r VirtualRun) Over() time.Time {
	if r.FinishedAt != nil {
		return *r.FinishedAt
	}
	return r.EndsAt
}

// Shift moves a contest with a start time onto the run's window, keeping its freeze rules.
func (c *Contest) Shift(run VirtualRun) Contest {
	shifted := *c
	offset := run.StartedAt.Sub(*c.StartTime)
	start, end := run.StartedAt, run.EndsAt
	shifted.StartTime, shifted.EndTime = &start, &end
	if c.Settings.FreezeStartedAt != nil {
		freeze := c.Settings.FreezeStartedAt.Add(offset)
		shifted.Settings.FreezeStartedAt = &freeze
	}
	return shifted
}

type ContestProblemResult struct {
	ContestID       uuid.UUID  `json:"contest_id"`
	UserID          uuid.UUID  `json:"user_id"`
	ProblemID       uuid.UUID  `json:"problem_id"`
	Solved          bool       `json:"solved"`
	FailedAttempts  int32      `json:"failed_attempts"`
	PendingAttempts int32      `json:"pending_attempts"`
	FirstACTime     *time.Time `json:"first_ac_time,omitempty"`
	TimeMinutes     *int32     `json:"time_minutes,omitempty"`
	Penalty         int32      `json:"penalty"`
}

type ScoreboardItem struct {
	UserID         uuid.UUID              `json:"user_id"`
	Username       string                 `json:"username"`
	ProblemsSolved int32                  `json:"problems_solved"`
	TotalPenalty   int32                  `json:"total_penalty"`
	LastAcceptedAt *time.Time             `json:"last_accepted_at,omitempty"`
	ProblemResults []ContestProblemResult `json:"problem_results"`
	Virtual        bool                   `json:"virtual,omitempty"`
}

type ScoreboardProblemHeader struct {
	ProblemID uuid.UUID `json:"problem_id"`
	Title     string    `json:"title"`
	ShortName string    `json:"short_name"`
	Ordinal   int32     `json:"ordinal"`
}

type ScoreboardResponse struct {
	ContestID         uuid.UUID                 `json:"contest_id"`
	ContestLogin      string                    `json:"contest_login"`
	OrganizationLogin string                    `json:"organization_login"`
	PenaltyPerAttempt int32                     `json:"penalty_per_attempt"`
	IsFrozen          bool                      `json:"is_frozen"`
	FreezeTime        *time.Time                `json:"freeze_time,omitempty"`
	Problems          []ScoreboardProblemHeader `json:"problems"`
	Items             []ScoreboardItem          `json:"items"`
}

type ScoreboardSubmission struct {
	UserID    uuid.UUID
	ProblemID uuid.UUID
	SubmissionForScoreboard
}

type SubmissionForScoreboard struct {
	State     submissionsmodel.State
	CreatedAt time.Time
}

type ContestMembersList struct {
	Members    []ContestMember
	Pagination paging.Pagination
}

type DashboardContest struct {
	ID                 uuid.UUID
	Login              string
	Title              string
	StartTime          *time.Time
	EndTime            *time.Time
	CreatedAt          time.Time
	OrganizationID     uuid.UUID
	OrganizationName   string
	OrganizationLogin  string
	UserRole           accessmodel.ContestRole
	LastSubmissionTime *time.Time
}

// PackageRef identifies the package a contest problem pins.
type PackageRef struct {
	ID, ProblemID uuid.UUID
}
