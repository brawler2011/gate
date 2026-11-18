package accessmodel

import (
	"time"

	"github.com/google/uuid"
)

// Failed fact reads must survive in LoadError; only confirmed missing membership is a zero role.
type PolicyInput struct {
	Actor     User
	Now       time.Time
	LoadError error
}

type OrganizationRole string

const (
	OrganizationMember OrganizationRole = "member"
	OrganizationAdmin  OrganizationRole = "admin"
	OrganizationOwner  OrganizationRole = "owner"
)

type TeamRole string

const (
	TeamMember     TeamRole = "member"
	TeamMaintainer TeamRole = "maintainer"
)

type ProblemRole string

const (
	ProblemViewer    ProblemRole = "viewer"
	ProblemModerator ProblemRole = "moderator"
	ProblemAdmin     ProblemRole = "admin"
	ProblemOwner     ProblemRole = "owner"
)

type ProblemTeamRole string

const (
	ProblemTeamRead  ProblemTeamRole = "read"
	ProblemTeamWrite ProblemTeamRole = "write"
	ProblemTeamAdmin ProblemTeamRole = "admin"
)

type ContestRole string

const (
	ContestParticipant ContestRole = "participant"
	ContestModerator   ContestRole = "moderator"
	ContestOwner       ContestRole = "owner"
)

type ContestTeamRole string

const (
	ContestTeamParticipant ContestTeamRole = "participant"
	ContestTeamModerator   ContestTeamRole = "moderator"
)

type JoinPolicy string

const (
	JoinOpen       JoinPolicy = "open"
	JoinByRequest  JoinPolicy = "by_request"
	JoinInviteOnly JoinPolicy = "invite_only"
)

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type TeamPrivacy string

const (
	TeamClosed TeamPrivacy = "closed"
	TeamSecret TeamPrivacy = "secret"
)

type ParticipationMode string

const (
	ParticipationOpen       ParticipationMode = "open"
	ParticipationByRequest  ParticipationMode = "by_request"
	ParticipationInviteOnly ParticipationMode = "invite_only"
)

type OrganizationContext struct {
	Owners     []User
	ID         uuid.UUID
	Role       OrganizationRole
	JoinPolicy JoinPolicy
}
type TeamContext struct {
	ID           uuid.UUID
	Organization OrganizationContext
	Role         TeamRole
	Privacy      TeamPrivacy
}

// Only direct, active team membership contributes a resource role.
type TeamGrant[R ~string] struct {
	OrganizationID uuid.UUID
	Membership     TeamRole
	Active         bool
	Role           R
}
type ProblemContext struct {
	Owners       []User
	ID           uuid.UUID
	Organization OrganizationContext
	OwnerID      uuid.UUID
	DirectRole   ProblemRole
	Teams        []TeamGrant[ProblemTeamRole]
	Visibility   Visibility
}

type ContestConditions struct {
	ViewStatements        bool
	ViewScoreboard        bool
	ListSubmissions       bool
	ViewOtherSubmission   bool
	ViewSubmissionDetails bool
	Submit                bool
}
type SubmissionContext struct {
	OwnerID   uuid.UUID
	ContestID uuid.UUID
}
type ContestContext struct {
	BlockedProblems   []uuid.UUID
	Owners            []User
	StartTime         *time.Time
	EndTime           *time.Time
	Settings          map[string]interface{}
	ID                uuid.UUID
	Organization      OrganizationContext
	OwnerID           uuid.UUID
	DirectRole        ContestRole
	Teams             []TeamGrant[ContestTeamRole]
	Visibility        Visibility
	ParticipationMode ParticipationMode
	Conditions        *ContestConditions
	Submission        SubmissionContext
}

// Count explicit active owners, excluding inherited organization owners.
// PrimaryID is optional for organizations and required for resources with owner_id.
type OwnerState struct {
	Owners    []User
	PrimaryID uuid.UUID
}

// An empty role revokes a grant; ownership changes require the complete AfterOwners set.
type RoleChange[R ~string] struct {
	Target       User
	Old          R
	New          R
	BeforeOwners OwnerState
	AfterOwners  OwnerState
}
