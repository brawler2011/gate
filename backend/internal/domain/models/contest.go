package models

import (
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
	ShortName      string
	Description    string
	Settings       map[string]interface{}
	AccessPolicy   map[string]interface{}
	StartTime      *time.Time
	EndTime        *time.Time
}

type CreateContestInput struct {
	OrganizationID uuid.UUID
	OwnerID        *uuid.UUID
	Title          string
	ShortName      string
	Description    string
	Visibility     ContestVisibility
	Settings       map[string]interface{}
	AccessPolicy   map[string]interface{}
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
	ID           uuid.UUID
	Title        *string
	Description  *string
	Visibility   *ContestVisibility
	Settings     *map[string]interface{}
	AccessPolicy *map[string]interface{}
	StartTime    *time.Time
	EndTime      *time.Time
	OwnerID      *uuid.UUID
}

type ContestUpdateParams struct {
	ID           uuid.UUID
	Title        *string
	Description  *string
	Visibility   *ContestVisibility
	Settings     *map[string]interface{}
	AccessPolicy *map[string]interface{}
	StartTime    *time.Time
	EndTime      *time.Time
	OwnerID      *uuid.UUID
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
	ID             uuid.UUID
	OrganizationID uuid.UUID
	OwnerID        *uuid.UUID
	Visibility     ContestVisibility
	Title          string
	ShortName      string
	Description    string
	Settings       map[string]interface{} // JSONB for contest settings
	AccessPolicy   map[string]interface{} // JSONB for access policies
	StartTime      *time.Time
	EndTime        *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
	KratosID        string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ContestMembersList struct {
	Members    []ContestMember
	Pagination Pagination
}
