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
	case m.OrganizationCreate, m.OrganizationInvite:
		return r >= 2
	case m.OrganizationManage, m.OrganizationManageMembers:
		return r >= 3
	case m.OrganizationDelete:
		return r >= 4
	case m.OrganizationJoin:
		return user && f.Role == "" && f.JoinPolicy == m.JoinOpen
	case m.OrganizationRequestJoin:
		return user && f.Role == "" && f.JoinPolicy == m.JoinByRequest
	case m.OrganizationLeave:
		return user && f.Role != ""
	}
	return false
}

// teamRole lets organization administrators act as maintainers of every team.
func teamRole(actor m.User, f TeamFacts) m.TeamRole {
	if rank(actor, f.OrganizationRole) >= m.Rank(m.OrganizationAdmin) {
		return m.TeamMaintainer
	}
	return f.Role
}

func TeamAllows(actor m.User, f TeamFacts, action m.TeamAction) bool {
	r := rank(actor, teamRole(actor, f))
	user := !actor.IsGuest()
	switch action {
	case m.TeamView:
		return user
	case m.TeamManage, m.TeamDelete:
		return r >= 2
	case m.TeamLeave:
		return user && f.Role != ""
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
	case m.ProblemRead:
		return r >= 1
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
	// after selects the settings for a finished contest; a replay still runs under the "during" ones.
	after bool
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
	s.participant = user && (r >= 1 || public && c.Settings.JoinPolicy == m.JoinOpen)
	s.virtual = user && s.view && f.Run != nil
	s.replaying = s.virtual && !f.Now.Before(f.Run.StartedAt) && f.Now.Before(f.Run.Over())
	s.competitor = s.participant || s.replaying || user && s.view && s.finished && (c.Settings.EnableUpsolving || s.virtual)
	s.after = s.finished && !s.replaying
	return s
}

// audience reports whether a setting admits the actor in the current phase; managers pass every setting.
func (s contestState) audience(rule contestsmodel.Phased[contestsmodel.ContestScope]) bool {
	switch rule.At(s.after) {
	case contestsmodel.ScopePublic:
		return s.manager || s.view && s.started
	case contestsmodel.ScopeParticipant:
		return s.manager || (s.participant || s.virtual) && s.started
	default:
		return s.manager
	}
}

func (s contestState) feedback() contestsmodel.FeedbackLevel {
	return s.f.Contest.Settings.Feedback.At(s.after)
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
		// The scoreboard would reveal verdicts that authors may not see yet.
		return s.manager || s.audience(settings.Scoreboard) && s.feedback() != contestsmodel.FeedbackHidden
	case m.ContestListSubmissions:
		return s.audience(settings.Submissions)
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

// SubmissionGrant is what the actor may see of one contest submission.
type SubmissionGrant struct {
	Open, Source bool
	Results      contestsmodel.FeedbackLevel
}

// SubmissionAllows grants authors their feedback level and others never more than the author sees.
// A virtual submission keeps the "during" level until facts show its author's run is over.
func SubmissionAllows(actor m.User, f ContestFacts, author uuid.UUID, virtual bool) SubmissionGrant {
	s := newContestState(actor, f)
	settings := f.Contest.Settings
	if s.manager {
		return SubmissionGrant{Open: true, Source: true, Results: contestsmodel.FeedbackFull}
	}
	own := !actor.IsGuest() && author == actor.Id && s.competitor
	after := s.after
	if virtual {
		after = own && f.Run != nil && !f.Now.Before(f.Run.Over())
	}
	level := settings.Feedback.At(after)
	if own {
		return SubmissionGrant{Open: true, Source: level != contestsmodel.FeedbackHidden, Results: level}
	}
	reports, source := s.audience(settings.Reports), s.audience(settings.Sources)
	if !source && !reports && !s.audience(settings.Submissions) {
		return SubmissionGrant{Results: contestsmodel.FeedbackHidden}
	}
	if level == contestsmodel.FeedbackHidden {
		return SubmissionGrant{Open: true, Results: level}
	}
	if !reports {
		level = contestsmodel.FeedbackVerdict
	}
	return SubmissionGrant{Open: true, Source: source, Results: level}
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
