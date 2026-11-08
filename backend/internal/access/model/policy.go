package accessmodel

import (
	"time"

	"github.com/google/uuid"
)

// PolicyInput fixes the actor and clock for a complete decision. LoadError must
// retain any failed facts read; only confirmed missing membership is a zero role.
// The policy layer has no repository or transaction dependency.
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

// TeamGrant contains a direct, active membership, not parent-team inheritance.
// Inactive or unrelated memberships never contribute a resource role.
type TeamGrant[R ~string] struct {
	OrganizationID uuid.UUID
	Membership     TeamRole
	Active         bool
	Role           R
}
type ProblemContext struct {
	ID           uuid.UUID
	Organization OrganizationContext
	OwnerID      uuid.UUID
	DirectRole   ProblemRole
	Teams        []TeamGrant[ProblemTeamRole]
	Visibility   Visibility
}

// ContestConditions supplies the lifecycle and audience gates from RFC §5.
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

// OwnerState is the explicit active owner set, excluding inherited org owners.
// PrimaryID is optional for organizations, mandatory when a resource uses owner_id.
// Mutation repositories must enforce the validated transition atomically; a
// policy precheck alone cannot protect against concurrent leaves or revocations.
type OwnerState struct {
	Owners    []User
	PrimaryID uuid.UUID
}

// RoleChange uses an empty role for an absent/revoked grant. AfterOwners is
// required whenever explicit ownership changes, including primary-owner changes.
type RoleChange[R ~string] struct {
	Target       User
	Old          R
	New          R
	BeforeOwners OwnerState
	AfterOwners  OwnerState
}
