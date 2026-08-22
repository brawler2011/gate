package models

import (
	"math"
	"time"

	"github.com/google/uuid"
)

type ContestVisibility = string

const (
	ContestVisibilityPublic   ContestVisibility = "public"
	ContestVisibilityPrivate  ContestVisibility = "private"
	ContestVisibilityUnlisted ContestVisibility = "unlisted"
)

type ContestRole = string

const (
	ContestRoleOwner       ContestRole = "owner"
	ContestRoleModerator   ContestRole = "moderator"
	ContestRoleParticipant ContestRole = "participant"
)

type UserContestMembership struct {
	ContestID uuid.UUID
	Role      ContestRole
}


type ContestFreezeStatus = string

const (
	FreezeStatusAuto     ContestFreezeStatus = "auto"
	FreezeStatusFrozen   ContestFreezeStatus = "frozen"
	FreezeStatusUnfrozen ContestFreezeStatus = "unfrozen"
)

// ContestPermissionMask stores action permissions as bit flags.
type ContestPermissionMask uint64

const (
	ContestPermissionGetContest ContestPermissionMask = 1 << iota
	ContestPermissionManageContest
	ContestPermissionGetMonitor
	ContestPermissionListUsersSubmissions
	ContestPermissionListOwnSubmissions
	ContestPermissionGetOwnSubmission
	ContestPermissionGetOtherUserSubmission
	ContestPermissionGetSubmissionDetails
	ContestPermissionCreateSubmission
)

const (
	ContestPermissionMaskParticipantDefault = ContestPermissionGetContest |
		ContestPermissionListOwnSubmissions |
		ContestPermissionGetOwnSubmission |
		ContestPermissionCreateSubmission
	ContestPermissionMaskModeratorDefault = ContestPermissionGetContest |
		ContestPermissionManageContest |
		ContestPermissionGetMonitor |
		ContestPermissionListUsersSubmissions |
		ContestPermissionListOwnSubmissions |
		ContestPermissionGetOwnSubmission |
		ContestPermissionGetOtherUserSubmission |
		ContestPermissionGetSubmissionDetails |
		ContestPermissionCreateSubmission
	ContestPermissionMaskOwnerDefault = ContestPermissionMaskModeratorDefault
)

func (m ContestPermissionMask) Has(permission ContestPermissionMask) bool {
	return m&permission == permission
}

func ContestRoleDefaultPermissionMask(role ContestRole) (ContestPermissionMask, bool) {
	switch role {
	case ContestRoleOwner:
		return ContestPermissionMaskOwnerDefault, true
	case ContestRoleModerator:
		return ContestPermissionMaskModeratorDefault, true
	case ContestRoleParticipant:
		return ContestPermissionMaskParticipantDefault, true
	default:
		return 0, false
	}
}

type CreateContestParams struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	OwnerID        *uuid.UUID
	Visibility     ContestVisibility
	Title          string
	Login          string
	Description    string
	Settings       map[string]interface{}
	StartTime      *time.Time
	EndTime        *time.Time
}

type CreateContestInput struct {
	OrganizationID uuid.UUID
	OwnerID        *uuid.UUID
	Title          string
	Login          string
	Description    string
	Visibility     ContestVisibility
	Settings       map[string]interface{}
	StartTime      *time.Time
	EndTime        *time.Time
}

type ContestsFilter struct {
	Page      int32
	PageSize  int32
	OwnerId   *uuid.UUID
	SortOrder SortOrder
	Search    string
}

type AdminContestsFilter struct {
	Page       int32
	PageSize   int32
	Search     string
	Visibility *ContestVisibility
	SortBy     string
	SortOrder  SortOrder
}

type UserContestsFilter struct {
	Page      int32
	PageSize  int32
	UserId    uuid.UUID
	Search    string
	SortBy    string
	SortOrder SortOrder
}

type WorkshopContestsFilter struct {
	Page           int32
	PageSize       int32
	UserId         uuid.UUID
	Search         string
	SortBy         string
	SortOrder      SortOrder
	OrganizationID *uuid.UUID
}

type PublicContestsFilter struct {
	Page      int32
	PageSize  int32
	Search    string
	SortBy    string
	SortOrder SortOrder
}

type ContestUpdateInput struct {
	ID          uuid.UUID
	Login       *string
	Title       *string
	Description *string
	Visibility  *ContestVisibility
	Settings    *map[string]interface{}
	StartTime   *time.Time
	EndTime     *time.Time
	OwnerID     *uuid.UUID
}

type ContestUpdateParams struct {
	ID          uuid.UUID
	Login       *string
	Title       *string
	Description *string
	Visibility  *ContestVisibility
	Settings    *map[string]interface{}
	StartTime   *time.Time
	EndTime     *time.Time
	OwnerID     *uuid.UUID
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

type ParticipantCreation struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
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

var roleHierarchy = map[ContestRole]int{
	ContestRoleOwner:       3,
	ContestRoleModerator:   2,
	ContestRoleParticipant: 1,
}

func IsValidContestRole(role ContestRole) bool {
	_, ok := roleHierarchy[role]
	return ok
}

func RoleGraterOrEquals(r1 ContestRole, r2 ContestRole) bool {
	h1, ok1 := roleHierarchy[r1]
	h2, ok2 := roleHierarchy[r2]

	if !ok1 || !ok2 {
		return false
	}

	return h1 >= h2
}

func RoleGreaterOrEquals(r1 ContestRole, r2 ContestRole) bool {
	return RoleGraterOrEquals(r1, r2)
}


type CreateContestMemberInput struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
	Role      string
}

type CreateContestMemberParams struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
	Role      ContestRole
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
	OwnerID           *uuid.UUID
	Visibility        ContestVisibility
	Title             string
	Login             string
	Description       string
	Settings          map[string]interface{} // JSONB for contest settings
	StartTime         *time.Time
	EndTime           *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ContestsList struct {
	Contests   []Contest
	Pagination Pagination
}

type ContestProblem struct {
	ContestID  uuid.UUID
	ProblemID  uuid.UUID
	PackageID  uuid.UUID
	Ordinal    int
	Title      string
	ShortName  string
	Visibility string
	PackageURL *string
	CreatedAt  time.Time
}

type ContestMember struct {
	UserID          uuid.UUID
	ContestID       uuid.UUID
	Username        string
	Role            UserRole
	ContestRole     ContestRole
	PermissionsMask *ContestPermissionMask
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ContestMembersList struct {
	Members    []ContestMember
	Pagination Pagination
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
	UserRole           string
	LastSubmissionTime *time.Time
}

type ContestParticipationMode = string

const (
	ParticipationModeOpen       ContestParticipationMode = "open"
	ParticipationModeByRequest  ContestParticipationMode = "by_request"
	ParticipationModeInviteOnly ContestParticipationMode = "invite_only"
)

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
	Status            RequestStatus
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
	PenaltyPerAttempt      int32  `json:"penalty_per_attempt,omitempty"`
	FreezeDurationMinutes  *int32 `json:"freeze_duration_minutes,omitempty"`
	FreezeStatus           string `json:"freeze_status,omitempty"`
	MonitorScope           string `json:"monitor_scope,omitempty"`
	SubmissionsListScope   string `json:"submissions_list_scope,omitempty"`
	SubmissionsReviewScope string `json:"submissions_review_scope,omitempty"`
	SubmissionDetailsScope string `json:"submission_details_scope,omitempty"`
	ShowVerdicts           *bool  `json:"show_verdicts,omitempty"`
	ShowTestDetails        *bool  `json:"show_test_details,omitempty"`
	AllowClarifications    *bool  `json:"allow_clarifications,omitempty"`
	EnableDrafts           *bool  `json:"enable_drafts,omitempty"`
	EnableUpsolving        *bool  `json:"enable_upsolving,omitempty"`
	EnableVirtualContests  *bool  `json:"enable_virtual_contests,omitempty"`
	ParticipationMode      string `json:"participation_mode,omitempty"`
	HideStatements         *bool  `json:"hide_statements,omitempty"`
}

func (s ContestSettings) GetHideStatements() bool {
	if s.HideStatements == nil {
		return false
	}
	return *s.HideStatements
}

func (s ContestSettings) GetPenaltyPerAttempt() int32 {
	if s.PenaltyPerAttempt <= 0 {
		return 20
	}
	return s.PenaltyPerAttempt
}

func (s ContestSettings) GetFreezeDurationMinutes() *int32 {
	if s.FreezeDurationMinutes == nil || *s.FreezeDurationMinutes <= 0 {
		return nil
	}
	return s.FreezeDurationMinutes
}

func (s ContestSettings) GetFreezeStatus() string {
	switch s.FreezeStatus {
	case FreezeStatusFrozen, FreezeStatusUnfrozen, FreezeStatusAuto:
		return s.FreezeStatus
	default:
		return FreezeStatusAuto
	}
}

func (s ContestSettings) GetEnableDrafts() bool {
	if s.EnableDrafts == nil {
		return true
	}
	return *s.EnableDrafts
}

func (s ContestSettings) GetAllowClarifications() bool {
	if s.AllowClarifications == nil {
		return true
	}
	return *s.AllowClarifications
}

func (s ContestSettings) GetEnableUpsolving() bool {
	if s.EnableUpsolving == nil {
		return true
	}
	return *s.EnableUpsolving
}

func (s ContestSettings) GetEnableVirtualContests() bool {
	if s.EnableVirtualContests == nil {
		return false
	}
	return *s.EnableVirtualContests
}

func (s ContestSettings) GetParticipationMode() string {
	if s.ParticipationMode == ParticipationModeInviteOnly {
		return ParticipationModeInviteOnly
	}
	if s.ParticipationMode == ParticipationModeByRequest {
		return ParticipationModeByRequest
	}
	return ParticipationModeOpen
}

func parseFlexibleInt32(v interface{}, defaultVal int32) int32 {
	switch n := v.(type) {
	case float64:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return defaultVal
		}
		return int32(n)
	case int:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return defaultVal
		}
		return int32(n)
	case int32:
		return n
	case int64:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return defaultVal
		}
		return int32(n)
	default:
		return defaultVal
	}
}

func parseFlexibleInt32Ptr(v interface{}) *int32 {
	if v == nil {
		return nil
	}
	switch n := v.(type) {
	case float64:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return nil
		}
		res := int32(n)
		return &res
	case int:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return nil
		}
		res := int32(n)
		return &res
	case int32:
		res := n
		return &res
	case int64:
		if n > math.MaxInt32 || n < math.MinInt32 {
			return nil
		}
		res := int32(n)
		return &res
	default:
		return nil
	}
}

func parseFlexibleBoolPtr(v interface{}) *bool {
	if v == nil {
		return nil
	}
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}

func MapToContestSettings(m map[string]interface{}) ContestSettings {
	var s ContestSettings
	if m == nil {
		return s
	}

	if raw, ok := m["penalty_per_attempt"]; ok && raw != nil {
		s.PenaltyPerAttempt = parseFlexibleInt32(raw, 20)
	} else {
		s.PenaltyPerAttempt = 20
	}

	if raw, ok := m["freeze_duration_minutes"]; ok && raw != nil {
		s.FreezeDurationMinutes = parseFlexibleInt32Ptr(raw)
	}

	if raw, ok := m["freeze_status"]; ok && raw != nil {
		if str, ok := raw.(string); ok {
			s.FreezeStatus = str
		}
	}

	if raw, ok := m["monitor_scope"]; ok && raw != nil {
		if str, ok := raw.(string); ok {
			s.MonitorScope = str
		}
	}

	if raw, ok := m["submissions_list_scope"]; ok && raw != nil {
		if str, ok := raw.(string); ok {
			s.SubmissionsListScope = str
		}
	}

	if raw, ok := m["submissions_review_scope"]; ok && raw != nil {
		if str, ok := raw.(string); ok {
			s.SubmissionsReviewScope = str
		}
	}

	if raw, ok := m["submission_details_scope"]; ok && raw != nil {
		if str, ok := raw.(string); ok {
			s.SubmissionDetailsScope = str
		}
	}

	if raw, ok := m["show_verdicts"]; ok && raw != nil {
		s.ShowVerdicts = parseFlexibleBoolPtr(raw)
	}

	if raw, ok := m["show_test_details"]; ok && raw != nil {
		s.ShowTestDetails = parseFlexibleBoolPtr(raw)
	}

	if raw, ok := m["allow_clarifications"]; ok && raw != nil {
		s.AllowClarifications = parseFlexibleBoolPtr(raw)
	}

	if raw, ok := m["enable_drafts"]; ok && raw != nil {
		s.EnableDrafts = parseFlexibleBoolPtr(raw)
	}

	if raw, ok := m["enable_upsolving"]; ok && raw != nil {
		s.EnableUpsolving = parseFlexibleBoolPtr(raw)
	}

	if raw, ok := m["enable_virtual_contests"]; ok && raw != nil {
		s.EnableVirtualContests = parseFlexibleBoolPtr(raw)
	}

	if raw, ok := m["participation_mode"]; ok && raw != nil {
		if str, ok := raw.(string); ok {
			s.ParticipationMode = str
		}
	}

	if raw, ok := m["hide_statements"]; ok && raw != nil {
		s.HideStatements = parseFlexibleBoolPtr(raw)
	}

	return s
}

func (c *Contest) TypedSettings() ContestSettings {
	return MapToContestSettings(c.Settings)
}

func (c *Contest) GetHideStatements() bool {
	return c.TypedSettings().GetHideStatements()
}

func (c *Contest) GetPenaltyPerAttempt() int32 {
	return c.TypedSettings().GetPenaltyPerAttempt()
}

func (c *Contest) GetFreezeDurationMinutes() *int32 {
	return c.TypedSettings().GetFreezeDurationMinutes()
}

func (c *Contest) GetFreezeStatus() string {
	return c.TypedSettings().GetFreezeStatus()
}

func (c *Contest) GetFreezeTime() *time.Time {
	durPtr := c.GetFreezeDurationMinutes()
	if durPtr == nil || *durPtr <= 0 || c.EndTime == nil {
		return nil
	}
	freezeTime := c.EndTime.Add(-time.Duration(*durPtr) * time.Minute)
	return &freezeTime
}

func (c *Contest) IsFrozenAt(t time.Time) bool {
	status := c.GetFreezeStatus()
	switch status {
	case FreezeStatusUnfrozen:
		return false
	case FreezeStatusFrozen:
		return true
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

type UpsertContestProblemResultParams struct {
	ContestID      uuid.UUID
	UserID         uuid.UUID
	ProblemID      uuid.UUID
	Solved         bool
	FailedAttempts int32
	FirstACTime    *time.Time
	TimeMinutes    *int32
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

type SubmissionForScoreboard struct {
	State     State
	CreatedAt time.Time
}


