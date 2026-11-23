package access

import (
	m "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

// rank is the actor's strength on a resource: guests hold nothing and site administrators hold the top role.
func rank[R m.Role](actor m.User, role R) int {
	switch {
	case actor.IsGuest():
		return 0
	case actor.IsAdmin():
		return m.Rank(m.Top[R]())
	default:
		return m.Rank(role)
	}
}

func OrganizationAllows(actor m.User, f OrganizationFacts, action m.OrganizationAction) bool {
	r := rank(actor, f.Role)
	user := !actor.IsGuest()
	switch action {
	case m.OrganizationView:
		return true
	case m.OrganizationListMembers:
		return r >= 1
	case m.OrganizationManage, m.OrganizationManageMembers:
		return r >= 2
	case m.OrganizationDelete:
		return r >= 3
	case m.OrganizationJoin:
		return user && f.Role == "" && f.JoinPolicy == m.JoinOpen
	case m.OrganizationRequestJoin:
		return user && f.Role == "" && f.JoinPolicy == m.JoinByRequest
	case m.OrganizationLeave:
		return user && f.Role != ""
	}
	return false
}

func TeamAllows(actor m.User, f TeamFacts, action m.TeamAction) bool {
	r := rank(actor, f.Role)
	user := !actor.IsGuest()
	switch action {
	case m.TeamView:
		return user && (r >= 1 || !f.Secret)
	case m.TeamManage:
		return r >= 2
	case m.TeamDelete:
		return r >= 3
	case m.TeamLeave:
		return user && f.Member
	}
	return false
}

func ProblemAllows(actor m.User, f ProblemFacts, action m.ProblemAction) bool {
	r := rank(actor, f.Role)
	switch action {
	case m.ProblemView:
		return f.Public || r >= 1
	case m.ProblemUse:
		return !actor.IsGuest() && (f.Public || r >= 1)
	case m.ProblemEdit:
		return r >= 2
	case m.ProblemManage:
		return r >= 3
	case m.ProblemDelete:
		return r >= 4
	}
	return false
}

type contestState struct {
	manager, view, started, finished bool
	// competitor may submit or has submitted: a participant, a virtual runner, or anyone upsolving a finished contest.
	participant, competitor bool
	// virtual: the actor has a virtual run; replaying: it is under way.
	virtual, replaying bool
	f                  ContestFacts
}

func newContestState(actor m.User, f ContestFacts) contestState {
	c := f.Contest
	r := rank(actor, f.Role)
	public := c.Visibility == contestsmodel.ContestVisibilityPublic
	s := contestState{
		manager:  r >= 2,
		view:     public || r >= 1,
		started:  c.StartTime == nil || !f.Now.Before(*c.StartTime),
		finished: c.EndTime != nil && !f.Now.Before(*c.EndTime),
		f:        f,
	}
	user := !actor.IsGuest()
	s.participant = user && (r >= 1 || public && c.Settings.ParticipationMode == contestsmodel.ParticipationModeOpen)
	s.virtual = user && s.view && f.Run != nil
	s.replaying = s.virtual && !f.Now.Before(f.Run.StartedAt) && f.Now.Before(f.Run.Over())
	s.competitor = s.participant || s.replaying || user && s.view && s.finished && (c.Settings.EnableUpsolving || s.virtual)
	return s
}

// audience reports whether a scope setting admits the actor; managers pass every scope.
func (s contestState) audience(scope contestsmodel.ContestScope) bool {
	switch scope {
	case contestsmodel.ScopePublic:
		return s.manager || s.view && s.started
	case contestsmodel.ScopeParticipant:
		return s.manager || s.participant && s.started
	default:
		return s.manager
	}
}

func ContestAllows(actor m.User, f ContestFacts, action m.ContestAction) bool {
	s := newContestState(actor, f)
	settings := f.Contest.Settings
	switch action {
	case m.ContestView:
		return s.view
	case m.ContestViewProblems:
		return s.manager || s.view && s.started
	case m.ContestViewStatements:
		return s.manager || s.view && s.started && !settings.HideStatements
	case m.ContestManage:
		return s.manager
	case m.ContestDelete:
		return rank(actor, f.Role) >= 3
	case m.ContestViewScoreboard:
		return s.audience(settings.MonitorScope) || s.virtual && settings.MonitorScope != contestsmodel.ScopeModerator
	case m.ContestListSubmissions:
		return s.audience(settings.SubmissionsListScope)
	case m.ContestListOwnSubmissions:
		return s.competitor
	case m.ContestSubmit:
		switch {
		case s.replaying:
			return true
		case s.finished:
			return settings.EnableUpsolving && s.competitor
		}
		return s.started && s.participant
	case m.ContestStartVirtual:
		c := f.Contest
		return !actor.IsGuest() && s.view && s.finished && c.StartTime != nil && settings.AllowVirtual && f.Run == nil && !f.Competed
	}
	return false
}

func SubmissionAllows(actor m.User, f ContestFacts, author uuid.UUID, action m.SubmissionAction) bool {
	s := newContestState(actor, f)
	settings := f.Contest.Settings
	own := !actor.IsGuest() && author == actor.Id && s.competitor
	view := own || s.audience(settings.SubmissionsReviewScope)
	switch action {
	case m.SubmissionView:
		return view
	case m.SubmissionViewDetails:
		return view && s.audience(settings.SubmissionDetailsScope)
	}
	return false
}

// ChangeRole decides setting target's role from current to next, where "" means no role.
// Members may only lower their own role; others need a stronger role than both, unless they hold the top role.
func ChangeRole[R m.Role](actor m.User, actorRole R, target uuid.UUID, current, next R) bool {
	from, to := m.Rank(current), m.Rank(next)
	if from < 0 || to < 0 || from == to {
		return false
	}
	if target == actor.Id {
		return to < from
	}
	r := rank(actor, actorRole)
	return r == m.Rank(m.Top[R]()) || r > from && r > to
}
