package models

import (
	"time"

	"github.com/google/uuid"
)

type ContestVisibility = string

const (
	ContestVisibilityPublic  ContestVisibility = "public"
	ContestVisibilityPrivate ContestVisibility = "private"
)

type ContestRole = string

const (
	ContestRoleOwner       ContestRole = "owner"
	ContestRoleModerator   ContestRole = "moderator"
	ContestRoleParticipant ContestRole = "participant"
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
	Page      int32
	PageSize  int32
	UserId    uuid.UUID
	Search    string
	SortBy    string
	SortOrder SortOrder
}

type PublicContestsFilter struct {
	Page      int32
	PageSize  int32
	Search    string
	SortBy    string
	SortOrder SortOrder
}

type ContestUpdateInput struct {
	Id                     uuid.UUID
	Title                  *string
	Description            *string
	Visibility             *string
	MonitorScope           *string
	SubmissionsListScope   *string
	SubmissionsReviewScope *string
	StartTime              *time.Time
	EndTime                *time.Time
}

type ContestUpdateParams struct {
	Id                     uuid.UUID
	Title                  *string
	Description            *string
	Visibility             *ContestVisibility
	MonitorScope           *ContestRole
	SubmissionsListScope   *ContestRole
	SubmissionsReviewScope *ContestRole
	StartTime              *time.Time
	EndTime                *time.Time
}

type ContestProblemGet struct {
	ContestId uuid.UUID
	ProblemId uuid.UUID
}

type ContestProblemCreation struct {
	ContestId uuid.UUID
	ProblemId uuid.UUID
}

type ContestProblemDeletion struct {
	ContestId uuid.UUID
	ProblemId uuid.UUID
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

type ContestPermissions struct {
	GetContest             bool
	UpdateContest          bool
	ManageContest          bool
	AdminContest           bool
	GetMonitor             bool
	ListUsersSubmissions   bool
	ListOwnSubmissions     bool
	GetOtherUserSubmission bool
	GetOwnSubmission       bool
	CreateSubmission       bool
}

var roleHierarchy = map[ContestRole]int{
	ContestRoleOwner:       3,
	ContestRoleModerator:   2,
	ContestRoleParticipant: 1,
}

func RoleGraterOrEquals(r1 ContestRole, r2 ContestRole) bool {
	h1, ok1 := roleHierarchy[r1]
	h2, ok2 := roleHierarchy[r2]

	if !ok1 || !ok2 {
		return false
	}

	return h1 >= h2
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

type Contest struct {
	ID                     uuid.UUID
	Title                  string
	Description            string
	Visibility             ContestRole
	MonitorScope           ContestRole
	SubmissionsListScope   ContestRole
	SubmissionsReviewScope ContestRole
	CreatedBy              uuid.UUID
	StartTime              *time.Time
	EndTime                *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type ContestsList struct {
	Contests   []Contest
	Pagination Pagination
}

type ContestProblem struct {
	ProblemID        uuid.UUID
	Title            string
	TimeLimit        int32
	MemoryLimit      int32
	Position         int32
	LegendHtml       string
	InputFormatHtml  string
	OutputFormatHtml string
	NotesHtml        string
	ScoringHtml      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ConstestProblemSlop struct {
	ContestID   uuid.UUID
	ProblemID   uuid.UUID
	Ordinal     int
	PackageHash string
}

type ContestMember struct {
	UserID      uuid.UUID
	ContestID   uuid.UUID
	Username    string
	Role        UserRole
	ContestRole ContestRole
	KratosID    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ContestMembersList struct {
	Members    []ContestMember
	Pagination Pagination
}
