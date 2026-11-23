package access_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	m "github.com/brawler2011/gate/backend/internal/access/model"
	cm "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

// actor is a column of a policy matrix: who acts and which direct role the facts give them.
type actor struct {
	name string
	user m.User
	role string
}

func actors(roles ...string) []actor {
	user := func(role m.UserRole) m.User { return m.User{Id: uuid.New(), Role: role} }
	list := []actor{{"guest", m.Guest, ""}, {"outsider", user(m.UserRoleUser), ""}}
	for _, r := range roles {
		list = append(list, actor{r, user(m.UserRoleUser), r})
	}
	return append(list, actor{"site admin", user(m.UserRoleAdmin), ""})
}

// check compares one row: want holds a 1 or 0 per actor column.
func check(t *testing.T, row string, cols []actor, want string, allows func(actor) bool) {
	t.Helper()
	if len(want) != len(cols) {
		t.Fatalf("%s: pattern %q has %d columns, want %d", row, want, len(want), len(cols))
	}
	for i, a := range cols {
		if got := allows(a); got != (want[i] == '1') {
			t.Errorf("%s / %s: allowed=%v, want %v", row, a.name, got, !got)
		}
	}
}

func TestOrganizationPolicy(t *testing.T) {
	cols := actors("member", "maintainer", "admin", "owner")
	for _, tc := range []struct {
		action m.OrganizationAction
		join   m.JoinPolicy
		want   string
	}{
		{m.OrganizationView, m.JoinInviteOnly, "1111111"},
		{m.OrganizationListMembers, m.JoinInviteOnly, "0011111"},
		{m.OrganizationCreate, m.JoinInviteOnly, "0001111"},
		{m.OrganizationInvite, m.JoinInviteOnly, "0001111"},
		{m.OrganizationManage, m.JoinInviteOnly, "0000111"},
		{m.OrganizationManageMembers, m.JoinInviteOnly, "0000111"},
		{m.OrganizationDelete, m.JoinInviteOnly, "0000011"},
		{m.OrganizationJoin, m.JoinOpen, "0100001"},
		{m.OrganizationJoin, m.JoinByRequest, "0000000"},
		{m.OrganizationRequestJoin, m.JoinByRequest, "0100001"},
		{m.OrganizationRequestJoin, m.JoinInviteOnly, "0000000"},
		{m.OrganizationLeave, m.JoinOpen, "0011110"},
	} {
		check(t, fmt.Sprint(tc.action, " ", tc.join), cols, tc.want, func(a actor) bool {
			return access.OrganizationAllows(a.user, access.OrganizationFacts{Role: m.OrganizationRole(a.role), JoinPolicy: tc.join}, tc.action)
		})
	}
}

func TestTeamPolicy(t *testing.T) {
	// Columns starting with "org " hold that organization role and no team role.
	cols := actors("member", "maintainer", "org maintainer", "org admin")
	for _, tc := range []struct {
		action m.TeamAction
		want   string
	}{
		{m.TeamView, "0111111"},
		{m.TeamManage, "0001011"},
		{m.TeamDelete, "0001011"},
		{m.TeamLeave, "0011000"},
	} {
		check(t, fmt.Sprint(tc.action), cols, tc.want, func(a actor) bool {
			f := access.TeamFacts{Role: m.TeamRole(a.role)}
			if org, ok := strings.CutPrefix(a.role, "org "); ok {
				f = access.TeamFacts{OrganizationRole: m.OrganizationRole(org)}
			}
			return access.TeamAllows(a.user, f, tc.action)
		})
	}
}

func TestOrganizationInvite(t *testing.T) {
	user := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	for _, tc := range []struct {
		inviter, invitee m.OrganizationRole
		want             bool
	}{
		{m.OrganizationMember, m.OrganizationMember, false},
		{m.OrganizationMaintainer, m.OrganizationMember, true},
		{m.OrganizationMaintainer, m.OrganizationMaintainer, true},
		{m.OrganizationMaintainer, m.OrganizationAdmin, false},
		{m.OrganizationAdmin, m.OrganizationMaintainer, true},
		{m.OrganizationAdmin, m.OrganizationAdmin, false},
		{m.OrganizationOwner, m.OrganizationOwner, true},
		{m.OrganizationOwner, "superuser", false},
	} {
		a := access.OrganizationAccess{Actor: user, Facts: access.OrganizationFacts{Role: tc.inviter}}
		if got := a.Invite(uuid.New(), tc.invitee) == nil; got != tc.want {
			t.Errorf("%s invites %s: got %v, want %v", tc.inviter, tc.invitee, got, tc.want)
		}
	}
}

func TestProblemPolicy(t *testing.T) {
	cols := actors("viewer", "editor", "admin", "owner")
	for _, tc := range []struct {
		action m.ProblemAction
		public bool
		want   string
	}{
		{m.ProblemView, false, "0011111"},
		{m.ProblemView, true, "1111111"},
		{m.ProblemUse, false, "0011111"},
		{m.ProblemUse, true, "0111111"},
		{m.ProblemRead, true, "0011111"},
		{m.ProblemEdit, true, "0001111"},
		{m.ProblemManage, true, "0000111"},
		{m.ProblemDelete, true, "0000011"},
	} {
		check(t, fmt.Sprint(tc.action, " public=", tc.public), cols, tc.want, func(a actor) bool {
			return access.ProblemAllows(a.user, access.ProblemFacts{Role: m.ProblemRole(a.role), Public: tc.public}, tc.action)
		})
	}
}

type phase int

const (
	before phase = iota
	running
	finished
)

// contest is private, invite-only and running unless a case changes it; every audience is moderator-only and authors see verdicts.
func contest(p phase, change func(*cm.Contest)) (cm.Contest, time.Time) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	now := map[phase]time.Time{before: start.Add(-time.Nanosecond), running: start, finished: end}[p]
	c := cm.Contest{Visibility: cm.ContestVisibilityPrivate, StartTime: &start, EndTime: &end, Settings: cm.ContestSettings{
		JoinPolicy: m.JoinInviteOnly,
		Scoreboard: cm.Same(cm.ScopeModerator), Submissions: cm.Same(cm.ScopeModerator),
		Sources: cm.Same(cm.ScopeModerator), Reports: cm.Same(cm.ScopeModerator), Feedback: cm.Same(cm.FeedbackVerdict),
	}}
	if change != nil {
		change(&c)
	}
	return c, now
}

func public(c *cm.Contest)    { c.Visibility = cm.ContestVisibilityPublic }
func upsolving(c *cm.Contest) { c.Settings.EnableUpsolving = true }
func openEntry(c *cm.Contest) { public(c); c.Settings.JoinPolicy = m.JoinOpen }
func hidden(c *cm.Contest)    { c.Settings.HideStatements = true }
func monitor(s cm.ContestScope) func(*cm.Contest) {
	return func(c *cm.Contest) { c.Settings.Scoreboard = cm.Same(s) }
}
func feedback(during cm.FeedbackLevel) func(*cm.Contest) {
	return func(c *cm.Contest) { c.Settings.Feedback.During = during }
}
func both(a, b func(*cm.Contest)) func(*cm.Contest) { return func(c *cm.Contest) { a(c); b(c) } }

// phased shows the scoreboard to participants while the contest runs and to everyone after it ends.
func phased(c *cm.Contest) {
	c.Settings.Scoreboard = cm.Phased[cm.ContestScope]{During: cm.ScopeParticipant, After: cm.ScopePublic}
}

func TestContestPolicy(t *testing.T) {
	cols := actors("participant", "moderator", "owner")
	for _, tc := range []struct {
		name   string
		action m.ContestAction
		phase  phase
		change func(*cm.Contest)
		want   string
	}{
		{"private", m.ContestView, before, nil, "001111"},
		{"public", m.ContestView, before, public, "111111"},
		{"before start", m.ContestViewProblems, before, public, "000111"},
		{"at start", m.ContestViewProblems, running, public, "111111"},
		{"private", m.ContestViewProblems, running, nil, "001111"},
		{"hidden statements keep the list", m.ContestViewProblems, running, hidden, "001111"},
		{"hidden statements", m.ContestViewStatements, running, hidden, "000111"},
		{"shown statements", m.ContestViewStatements, running, nil, "001111"},
		{"", m.ContestManage, running, nil, "000111"},
		{"", m.ContestDelete, running, nil, "000011"},
		{"moderator scope", m.ContestViewScoreboard, running, public, "000111"},
		{"participant scope", m.ContestViewScoreboard, running, monitor(cm.ScopeParticipant), "001111"},
		{"public scope", m.ContestViewScoreboard, running, both(public, monitor(cm.ScopePublic)), "111111"},
		{"public scope before start", m.ContestViewScoreboard, before, both(public, monitor(cm.ScopePublic)), "000111"},
		{"public scope of a private contest", m.ContestViewScoreboard, running, monitor(cm.ScopePublic), "001111"},
		{"participants during", m.ContestViewScoreboard, running, both(public, phased), "001111"},
		{"everyone after", m.ContestViewScoreboard, finished, both(public, phased), "111111"},
		{"hidden own results", m.ContestViewScoreboard, running, both(both(public, monitor(cm.ScopePublic)), feedback(cm.FeedbackHidden)), "000111"},
		{"hidden own results after the end", m.ContestViewScoreboard, finished, both(both(public, monitor(cm.ScopePublic)), feedback(cm.FeedbackHidden)), "111111"},
		{"moderator scope", m.ContestListSubmissions, running, nil, "000111"},
		{"", m.ContestListOwnSubmissions, running, nil, "001111"},
		{"upsolving", m.ContestListOwnSubmissions, finished, both(public, upsolving), "011111"},
		{"before start", m.ContestSubmit, before, nil, "000000"},
		{"invited", m.ContestSubmit, running, nil, "001111"},
		{"public by invitation", m.ContestSubmit, running, public, "001111"},
		{"open entry", m.ContestSubmit, running, openEntry, "011111"},
		{"finished", m.ContestSubmit, finished, openEntry, "000000"},
		{"upsolving private", m.ContestSubmit, finished, upsolving, "001111"},
		{"upsolving public", m.ContestSubmit, finished, both(public, upsolving), "011111"},
		{"upsolving before end", m.ContestSubmit, running, both(public, upsolving), "001111"},
	} {
		c, now := contest(tc.phase, tc.change)
		check(t, fmt.Sprint(tc.action, " ", tc.name), cols, tc.want, func(a actor) bool {
			return access.ContestAllows(a.user, access.ContestFacts{Contest: c, Role: m.ContestRole(a.role), Now: now}, tc.action)
		})
	}
}

func TestVirtualRunPolicy(t *testing.T) {
	cols := actors("participant", "moderator", "owner")
	replayable := func(c *cm.Contest) { public(c); monitor(cm.ScopeParticipant)(c); c.Settings.AllowVirtual = true }
	// active is under way at the finished-phase clock, over has already ended.
	active := func(now time.Time) *cm.VirtualRun { return &cm.VirtualRun{StartedAt: now, EndsAt: now.Add(time.Hour)} }
	over := func(now time.Time) *cm.VirtualRun {
		return &cm.VirtualRun{StartedAt: now.Add(-2 * time.Hour), EndsAt: now}
	}
	for _, tc := range []struct {
		name     string
		action   m.ContestAction
		phase    phase
		change   func(*cm.Contest)
		run      func(time.Time) *cm.VirtualRun
		competed bool
		want     string
	}{
		{"finished", m.ContestStartVirtual, finished, replayable, nil, false, "011111"},
		{"not allowed", m.ContestStartVirtual, finished, public, nil, false, "000000"},
		{"running", m.ContestStartVirtual, running, replayable, nil, false, "000000"},
		{"competed", m.ContestStartVirtual, finished, replayable, nil, true, "000000"},
		{"second run", m.ContestStartVirtual, finished, replayable, over, false, "000000"},
		{"private", m.ContestStartVirtual, finished, func(c *cm.Contest) { c.Settings.AllowVirtual = true }, nil, false, "001111"},
		{"during the run without upsolving", m.ContestSubmit, finished, replayable, active, false, "011111"},
		{"during the run after the end moved later", m.ContestSubmit, running, replayable, active, false, "011111"},
		{"after the run without upsolving", m.ContestSubmit, finished, replayable, over, false, "000000"},
		{"participant scope without a run", m.ContestViewScoreboard, finished, replayable, nil, false, "001111"},
		{"participant scope with a run", m.ContestViewScoreboard, finished, replayable, over, false, "011111"},
		{"own submissions after the run", m.ContestListOwnSubmissions, finished, replayable, over, false, "011111"},
	} {
		c, now := contest(tc.phase, tc.change)
		var run *cm.VirtualRun
		if tc.run != nil {
			run = tc.run(now)
		}
		check(t, fmt.Sprint(tc.action, " ", tc.name), cols, tc.want, func(a actor) bool {
			return access.ContestAllows(a.user, access.ContestFacts{Contest: c, Role: m.ContestRole(a.role), Now: now, Run: run, Competed: tc.competed}, tc.action)
		})
	}
}

func TestSubmissionPolicy(t *testing.T) {
	cols := actors("participant", "moderator", "owner")
	author := cols[2].user.Id
	open := func(g access.SubmissionGrant) bool { return g.Open }
	source := func(g access.SubmissionGrant) bool { return g.Source }
	tests := func(g access.SubmissionGrant) bool { return g.Results.Tests() }
	verdict := func(g access.SubmissionGrant) bool { return g.Results != cm.FeedbackHidden }
	others := func(scope cm.ContestScope) func(*cm.Contest) {
		return func(c *cm.Contest) {
			public(c)
			c.Settings.Submissions = cm.Same(scope)
			c.Settings.Reports = cm.Same(scope)
		}
	}
	for _, tc := range []struct {
		name    string
		see     func(access.SubmissionGrant) bool
		author  uuid.UUID
		phase   phase
		change  func(*cm.Contest)
		want    string
		virtual bool
	}{
		{"own opens", open, author, running, nil, "001111", false},
		{"another's stays closed", open, uuid.New(), running, nil, "000111", false},
		{"own verdict", verdict, author, running, nil, "001111", false},
		{"own tests at verdict level", tests, author, running, nil, "000111", false},
		{"own tests", tests, author, running, feedback(cm.FeedbackTests), "001111", false},
		{"own results hidden", verdict, author, running, feedback(cm.FeedbackHidden), "000111", false},
		{"own code hidden", source, author, running, feedback(cm.FeedbackHidden), "000111", false},
		{"own results after the end", tests, author, finished, both(feedback(cm.FeedbackHidden), func(c *cm.Contest) { c.Settings.Feedback.After = cm.FeedbackFull }), "001111", false},
		{"public list", open, uuid.New(), running, others(cm.ScopePublic), "111111", false},
		{"public reports", tests, uuid.New(), running, both(others(cm.ScopePublic), feedback(cm.FeedbackFull)), "111111", false},
		{"reports for moderators", tests, uuid.New(), running, both(others(cm.ScopeModerator), feedback(cm.FeedbackFull)), "000111", false},
		{"never more than the author", tests, uuid.New(), running, others(cm.ScopePublic), "000111", false},
		{"author's hidden results", verdict, uuid.New(), running, both(others(cm.ScopePublic), feedback(cm.FeedbackHidden)), "000111", false},
		{"code after the end", source, uuid.New(), finished, func(c *cm.Contest) { public(c); c.Settings.Sources.After = cm.ScopePublic }, "111111", false},
		{"code while running", source, uuid.New(), running, func(c *cm.Contest) { public(c); c.Settings.Sources.After = cm.ScopePublic }, "000111", false},
		{"code the author cannot see", source, uuid.New(), running, both(func(c *cm.Contest) { public(c); c.Settings.Sources = cm.Same(cm.ScopePublic) }, feedback(cm.FeedbackHidden)), "000111", false},
		{"reports alone open", open, uuid.New(), running, func(c *cm.Contest) { public(c); c.Settings.Reports = cm.Same(cm.ScopePublic) }, "111111", false},
		{"a virtual run keeps its level after the end", tests, uuid.New(), finished, both(others(cm.ScopePublic), func(c *cm.Contest) { c.Settings.Feedback.After = cm.FeedbackFull }), "000111", true},
		{"a virtual run never shows more than the stricter phase", tests, uuid.New(), finished, both(others(cm.ScopePublic), func(c *cm.Contest) {
			c.Settings.Feedback = cm.Phased[cm.FeedbackLevel]{During: cm.FeedbackFull, After: cm.FeedbackHidden}
		}), "000111", true},
		{"own virtual submissions without a finished run", tests, author, finished, func(c *cm.Contest) { upsolving(c); c.Settings.Feedback.After = cm.FeedbackFull }, "000111", true},
	} {
		c, now := contest(tc.phase, tc.change)
		check(t, tc.name, cols, tc.want, func(a actor) bool {
			return tc.see(access.SubmissionAllows(a.user, access.ContestFacts{Contest: c, Role: m.ContestRole(a.role), Now: now}, tc.author, tc.virtual))
		})
	}
}

func TestChangeRole(t *testing.T) {
	user := m.User{Id: uuid.New(), Role: m.UserRoleUser}
	admin := m.User{Id: uuid.New(), Role: m.UserRoleAdmin}
	other := uuid.New()
	for _, tc := range []struct {
		name          string
		actor         m.User
		role          m.ProblemRole
		target        uuid.UUID
		current, next m.ProblemRole
		want          bool
	}{
		{"owner adds an owner", user, m.ProblemOwner, other, "", m.ProblemOwner, true},
		{"owner removes an owner", user, m.ProblemOwner, other, m.ProblemOwner, "", true},
		{"admin adds an editor", user, m.ProblemAdmin, other, "", m.ProblemEditor, true},
		{"admin cannot add a peer", user, m.ProblemAdmin, other, "", m.ProblemAdmin, false},
		{"admin cannot demote a peer", user, m.ProblemAdmin, other, m.ProblemAdmin, m.ProblemViewer, false},
		{"admin cannot remove an owner", user, m.ProblemAdmin, other, m.ProblemOwner, "", false},
		{"editor adds a viewer", user, m.ProblemEditor, other, "", m.ProblemViewer, true},
		{"viewer cannot grant", user, m.ProblemViewer, other, "", m.ProblemViewer, false},
		{"member lowers own role", user, m.ProblemEditor, user.Id, m.ProblemEditor, m.ProblemViewer, true},
		{"member leaves", user, m.ProblemEditor, user.Id, m.ProblemEditor, "", true},
		{"owner cannot raise own role", user, m.ProblemAdmin, user.Id, m.ProblemAdmin, m.ProblemOwner, false},
		{"site admin grants owner", admin, "", other, "", m.ProblemOwner, true},
		{"guest grants nothing", m.Guest, "", other, "", m.ProblemViewer, false},
		{"unchanged role", user, m.ProblemOwner, other, m.ProblemViewer, m.ProblemViewer, false},
		{"unknown role", user, m.ProblemOwner, other, "", "superuser", false},
	} {
		if got := access.ChangeRole(tc.actor, tc.role, tc.target, tc.current, tc.next); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
