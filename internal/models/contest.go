package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ContestVisibilityPublic  = "public"
	ContestVisibilityPrivate = "private"
)

const (
	ContestRoleOwner       = "owner"
	ContestRoleModerator   = "moderator"
	ContestRoleParticipant = "participant"
)

type CreateContestParams struct {
	Id     uuid.UUID
	Title  string
	UserId uuid.UUID
}

type CreateContestInput struct {
	Title  string
	UserId uuid.UUID
}

type ContestsFilter struct {
	Page       int64
	PageSize   int64
	OwnerId    *uuid.UUID
	Descending bool
	Search     *string
}

func (f ContestsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type AdminContestsFilter struct {
	Page       int64
	PageSize   int64
	Search     *string
	Visibility *string
	SortBy     *string
	SortOrder  *string
}

func (f AdminContestsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type UserContestsFilter struct {
	Page      int64
	PageSize  int64
	UserId    uuid.UUID
	Search    *string
	SortBy    *string
	SortOrder *string
}

func (f UserContestsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type WorkshopContestsFilter struct {
	Page      int64
	PageSize  int64
	UserId    uuid.UUID
	Search    *string
	SortBy    *string
	SortOrder *string
}

func (f WorkshopContestsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type PublicContestsFilter struct {
	Page      int64
	PageSize  int64
	Search    *string
	SortBy    *string
	SortOrder *string
}

func (f PublicContestsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type ContestUpdate struct {
	Id                     uuid.UUID  `db:"id"`
	Title                  *string    `json:"title"`
	Description            *string    `json:"description"`
	Visibility             *string    `db:"visibility"`
	MonitorScope           *string    `db:"monitor_scope"`
	SubmissionsListScope   *string    `db:"submissions_list_scope"`
	SubmissionsReviewScope *string    `db:"submissions_review_scope"`
	StartTime              *time.Time `json:"start_time"`
	EndTime                *time.Time `json:"end_time"`
	ScoringMode            *string    `json:"scoring_mode"`
}

type ContestProblemGet struct {
	ContestId uuid.UUID `json:"contest_id"`
	ProblemId uuid.UUID `json:"problem_id"`
}

type ContestProblemCreation struct {
	ContestId uuid.UUID `json:"contest_id"`
	ProblemId uuid.UUID `json:"problem_id"`
}

type ContestProblemDeletion struct {
	ContestId uuid.UUID `json:"contest_id"`
	ProblemId uuid.UUID `json:"problem_id"`
}

type ParticipantCreation struct {
	ContestId uuid.UUID `json:"contest_id"`
	UserId    uuid.UUID `json:"user_id"`
}

type ParticipantDeletion struct {
	ContestId uuid.UUID `json:"contest_id"`
	UserId    uuid.UUID `json:"user_id"`
}

type ParticipantsFilter struct {
	Page      int64
	PageSize  int64
	ContestId uuid.UUID
}

func (f ParticipantsFilter) Offset() int64 {
	return (f.Page - 1) * f.PageSize
}

type ContestPermissionGet struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
}

type ContestPermissions struct {
	GetContest             bool
	UpdateContest          bool
	AdminContest           bool
	GetMonitor             bool
	ListUsersSubmissions   bool
	ListOwnSubmissions     bool
	GetOtherUserSubmission bool
	GetOwnSubmission       bool
	CreateSubmission       bool
}

var roleHierarchy = map[string]int{
	ContestRoleOwner:       3,
	ContestRoleModerator:   2,
	ContestRoleParticipant: 1,
}

func RoleGraterOrEquals(r1 string, r2 string) bool {
	h1, ok1 := roleHierarchy[r1]
	h2, ok2 := roleHierarchy[r2]

	if !ok1 || !ok2 {
		return false
	}

	return h1 >= h2
}

type CreateContestMemberInput struct {
	ContestId uuid.UUID `json:"contest_id"`
	UserId    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
}

type ContestMemberParams struct {
	ContestId uuid.UUID `json:"contest_id"`
	UserId    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
}
