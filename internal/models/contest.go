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

type Contest struct {
	Id uuid.UUID `db:"id" json:"Id"`

	Title       string `db:"title" json:"Title"`
	Description string `db:"description" json:"Description"`

	Visibility             string `db:"visibility" json:"Visibility"`
	MonitorScope           string `db:"monitor_scope" json:"MonitorScope"`
	SubmissionsListScope   string `db:"submissions_list_scope" json:"SubmissionsListScope"`
	SubmissionsReviewScope string `db:"submissions_review_scope" json:"SubmissionsReviewScope"`

	CreatedBy uuid.UUID `db:"created_by" json:"CreatedBy"`

	CreatedAt time.Time `db:"created_at" json:"CreatedAt"`
	UpdatedAt time.Time `db:"updated_at" json:"UpdatedAt"`
}

func (c *Contest) IsPublic() bool {
	return c.Visibility == ContestVisibilityPublic
}

func (c *Contest) IsPrivate() bool {
	return c.Visibility == ContestVisibilityPrivate
}

type CreateContestParams struct {
	Id     uuid.UUID
	Title  string
	UserId uuid.UUID
}

type CreateContestInput struct {
	Title  string
	UserId uuid.UUID
}

type ContestsList struct {
	Contests   []*Contest
	Pagination Pagination
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

type ContestUpdate struct {
	Id                     uuid.UUID `db:"id"`
	Title                  *string   `json:"title"`
	Description            *string   `json:"description"`
	Visibility             *string   `db:"visibility"`
	MonitorScope           *string   `db:"monitor_scope"`
	SubmissionsListScope   *string   `db:"submissions_list_scope"`
	SubmissionsReviewScope *string   `db:"submissions_review_scope"`
}

type ContestProblemsListItem struct {
	ProblemId   uuid.UUID `db:"problem_id"`
	Position    int64     `db:"position"`
	Title       string    `db:"title"`
	TimeLimit   int64     `db:"time_limit"`
	MemoryLimit int64     `db:"memory_limit"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type ContestProblem struct {
	ProblemId   uuid.UUID `db:"problem_id"`
	Title       string    `db:"title"`
	TimeLimit   int64     `db:"time_limit"`
	MemoryLimit int64     `db:"memory_limit"`

	Position int64 `db:"position"`

	LegendHtml       string `db:"legend_html"`
	InputFormatHtml  string `db:"input_format_html"`
	OutputFormatHtml string `db:"output_format_html"`
	NotesHtml        string `db:"notes_html"`
	ScoringHtml      string `db:"scoring_html"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
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

type ContestMember struct {
	UserId      uuid.UUID `db:"user_id"`
	ContestId   uuid.UUID `db:"contest_id"`
	Username    string    `db:"username"`
	Role        string    `db:"role"`
	ContestRole string    `db:"contest_role"`
	KratosId    string    `db:"kratos_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type ParticipantsList struct {
	Users      []*ContestMember
	Pagination *Pagination
}

type ContestPermissionGet struct {
	ContestId uuid.UUID
	UserId    uuid.UUID
}

type ContestMemberRecord struct {
	ContestId uuid.UUID `db:"contest_id" json:"ContestId"`
	UserId    uuid.UUID `db:"user_id" json:"UserId"`
	Role      string    `db:"role" json:"Role"`
}

func (cm *ContestMemberRecord) IsOwner() bool {
	return cm.Role == ContestRoleOwner
}

func (cm *ContestMemberRecord) IsModerator() bool {
	return cm.Role == ContestRoleModerator
}

func (cm *ContestMemberRecord) IsParticipant() bool {
	return cm.Role == ContestRoleParticipant
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
