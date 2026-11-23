package access_test

import (
	"fmt"
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
	cols := actors("member", "admin", "owner")
	for _, tc := range []struct {
		action m.OrganizationAction
		join   m.JoinPolicy
		want   string
	}{
		{m.OrganizationView, m.JoinInviteOnly, "111111"},
		{m.OrganizationListMembers, m.JoinInviteOnly, "001111"},
		{m.OrganizationManage, m.JoinInviteOnly, "000111"},
		{m.OrganizationManageMembers, m.JoinInviteOnly, "000111"},
		{m.OrganizationDelete, m.JoinInviteOnly, "000011"},
		{m.OrganizationJoin, m.JoinOpen, "010001"},
		{m.OrganizationJoin, m.JoinByRequest, "000000"},
		{m.OrganizationRequestJoin, m.JoinByRequest, "010001"},
		{m.OrganizationRequestJoin, m.JoinInviteOnly, "000000"},
		{m.OrganizationLeave, m.JoinOpen, "001110"},
	} {
		check(t, fmt.Sprint(tc.action, " ", tc.join), cols, tc.want, func(a actor) bool {
			return access.OrganizationAllows(a.user, access.OrganizationFacts{Role: m.OrganizationRole(a.role), JoinPolicy: tc.join}, tc.action)
		})
	}
}

func TestTeamPolicy(t *testing.T) {
	cols := actors("member", "maintainer", "manager")
	for _, tc := range []struct {
		action m.TeamAction
		secret bool
		want   string
	}{
		{m.TeamView, false, "011111"},
		{m.TeamView, true, "001111"},
		{m.TeamManage, true, "000111"},
		{m.TeamDelete, true, "000011"},
		// Managers act through organization administration and are not members.
		{m.TeamLeave, true, "001100"},
	} {
		check(t, fmt.Sprint(tc.action, " secret=", tc.secret), cols, tc.want, func(a actor) bool {
			member := a.role == "member" || a.role == "maintainer"
			return access.TeamAllows(a.user, access.TeamFacts{Role: m.TeamRole(a.role), Member: member, Secret: tc.secret}, tc.action)
		})
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

// contest is private, invite-only and running unless a case changes it; every scope is moderator-only.
func contest(p phase, change func(*cm.Contest)) (cm.Contest, time.Time) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	now := map[phase]time.Time{before: start.Add(-time.Nanosecond), running: start, finished: end}[p]
	c := cm.Contest{Visibility: cm.ContestVisibilityPrivate, StartTime: &start, EndTime: &end, Settings: cm.ContestSettings{
		ParticipationMode: cm.ParticipationModeInviteOnly,
		MonitorScope:      cm.ScopeModerator, SubmissionsListScope: cm.ScopeModerator,
		SubmissionsReviewScope: cm.ScopeModerator, SubmissionDetailsScope: cm.ScopeModerator,
	}}
	if change != nil {
		change(&c)
	}
	return c, now
}

func public(c *cm.Contest)    { c.Visibility = cm.ContestVisibilityPublic }
func upsolving(c *cm.Contest) { c.Settings.EnableUpsolving = true }
func openEntry(c *cm.Contest) { public(c); c.Settings.ParticipationMode = cm.ParticipationModeOpen }
func hidden(c *cm.Contest)    { c.Settings.HideStatements = true }
func monitor(s cm.ContestScope) func(*cm.Contest) {
	return func(c *cm.Contest) { c.Settings.MonitorScope = s }
}
func both(a, b func(*cm.Contest)) func(*cm.Contest) { return func(c *cm.Contest) { a(c); b(c) } }

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
	over := func(now time.Time) *cm.VirtualRun { return &cm.VirtualRun{StartedAt: now.Add(-2 * time.Hour), EndsAt: now} }
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
	for _, tc := range []struct {
		name   string
		action m.SubmissionAction
		author uuid.UUID
		change func(*cm.Contest)
		want   string
	}{
		{"own", m.SubmissionView, author, nil, "001111"},
		{"another's", m.SubmissionView, uuid.New(), nil, "000111"},
		{"own, moderator details", m.SubmissionViewDetails, author, nil, "000111"},
		{"own, participant details", m.SubmissionViewDetails, author, func(c *cm.Contest) { c.Settings.SubmissionDetailsScope = cm.ScopeParticipant }, "001111"},
		{"public review", m.SubmissionView, uuid.New(), func(c *cm.Contest) { public(c); c.Settings.SubmissionsReviewScope = cm.ScopePublic }, "111111"},
	} {
		c, now := contest(running, tc.change)
		check(t, fmt.Sprint(tc.action, " ", tc.name), cols, tc.want, func(a actor) bool {
			return access.SubmissionAllows(a.user, access.ContestFacts{Contest: c, Role: m.ContestRole(a.role), Now: now}, tc.author, tc.action)
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
