package accessmodel

import "slices"

type OrganizationRole string

const (
	OrganizationMember OrganizationRole = "member"
	OrganizationAdmin  OrganizationRole = "admin"
	OrganizationOwner  OrganizationRole = "owner"
)

// TeamManager is derived from organization administration; nobody holds it directly.
type TeamRole string

const (
	TeamMember     TeamRole = "member"
	TeamMaintainer TeamRole = "maintainer"
	TeamManager    TeamRole = "manager"
)

type ProblemRole string

const (
	ProblemViewer ProblemRole = "viewer"
	ProblemEditor ProblemRole = "editor"
	ProblemAdmin  ProblemRole = "admin"
	ProblemOwner  ProblemRole = "owner"
)

type ContestRole string

const (
	ContestParticipant ContestRole = "participant"
	ContestModerator   ContestRole = "moderator"
	ContestOwner       ContestRole = "owner"
)

type JoinPolicy string

const (
	JoinOpen       JoinPolicy = "open"
	JoinByRequest  JoinPolicy = "by_request"
	JoinInviteOnly JoinPolicy = "invite_only"
)

// Role is a resource role whose strength is its position in Ladder; the empty role ranks 0.
type Role interface {
	~string
	Ladder() []string
}

func (OrganizationRole) Ladder() []string { return []string{"member", "admin", "owner"} }
func (TeamRole) Ladder() []string         { return []string{"member", "maintainer", "manager"} }
func (ProblemRole) Ladder() []string      { return []string{"viewer", "editor", "admin", "owner"} }
func (ContestRole) Ladder() []string      { return []string{"participant", "moderator", "owner"} }

// Rank is -1 for a value outside the ladder.
func Rank[R Role](r R) int {
	if r == "" {
		return 0
	}
	i := slices.Index(r.Ladder(), string(r))
	if i < 0 {
		return -1
	}
	return i + 1
}

func Top[R Role]() R {
	var r R
	ladder := r.Ladder()
	return R(ladder[len(ladder)-1])
}
