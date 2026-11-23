package observer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// contestPermissions describes a scenario; accessStub turns it into the role and scopes that grant it.
type contestPermissions struct {
	GetContest, ListOwnSubmissions, GetMonitor, ListUsersSubmissions bool
	ManageContest, GetOtherUserSubmission, GetSubmissionDetails      bool
}

type accessStub struct {
	accesstest.Loader
	AuthUC
	ContestReader
	Scoreboard
	mu            sync.Mutex
	user          accessmodel.User
	permissions   contestPermissions
	contest       contestsmodel.Contest
	authErr       error
	permissionErr error
	contestErr    error
	onPermissions func()
	// facts, when set, replaces the flag translation below.
	facts *access.ContestFacts
}

func (m *accessStub) Authenticate(context.Context, uuid.UUID) (accessmodel.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.user, m.authErr
}

func (m *accessStub) Actor(context.Context, uuid.UUID) (accessmodel.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.user, nil
}

func (m *accessStub) ContestFacts(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]access.ContestFacts, error) {
	m.mu.Lock()
	p, err, hook, c, fixed := m.permissions, m.permissionErr, m.onPermissions, m.contest, m.facts
	m.onPermissions = nil
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	if err != nil {
		return nil, err
	}
	if fixed != nil {
		return map[uuid.UUID]access.ContestFacts{c.ID: *fixed}, nil
	}
	scope := func(granted bool) contestsmodel.ContestScope {
		if granted {
			return contestsmodel.ScopePublic
		}
		return contestsmodel.ScopeModerator
	}
	var role accessmodel.ContestRole
	switch {
	case p.ManageContest:
		role = accessmodel.ContestModerator
	case p.ListOwnSubmissions:
		role = accessmodel.ContestParticipant
	}
	c.Visibility = contestsmodel.ContestVisibilityPrivate
	// Under the policy every audience grant implies seeing the contest.
	if p.GetContest || p.GetMonitor || p.ListUsersSubmissions || p.GetOtherUserSubmission || p.GetSubmissionDetails {
		c.Visibility = contestsmodel.ContestVisibilityPublic
	}
	c.Settings.JoinPolicy = accessmodel.JoinInviteOnly
	c.Settings.Scoreboard = contestsmodel.Same(scope(p.GetMonitor))
	c.Settings.Submissions = contestsmodel.Same(scope(p.ListUsersSubmissions))
	c.Settings.Sources = contestsmodel.Same(scope(p.GetOtherUserSubmission))
	c.Settings.Reports = contestsmodel.Same(scope(p.GetSubmissionDetails))
	c.Settings.Feedback = contestsmodel.Same(contestsmodel.FeedbackVerdict)
	if p.GetSubmissionDetails {
		c.Settings.Feedback = contestsmodel.Same(contestsmodel.FeedbackFull)
	}
	facts := map[uuid.UUID]access.ContestFacts{}
	for _, id := range ids {
		if id == c.ID {
			facts[id] = access.ContestFacts{Contest: c, Role: role, Now: time.Now()}
		}
	}
	return facts, nil
}

func (m *accessStub) GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.contest, m.contestErr
}

func (m *accessStub) change(fn func()) { m.mu.Lock(); defer m.mu.Unlock(); fn() }

type wsFixture struct {
	stub     *accessStub
	hub      *Hub
	contests *ContestsHub
	server   *httptest.Server
	session  uuid.UUID
}

func newWSFixture(t *testing.T) *wsFixture {
	t.Helper()
	stub := &accessStub{user: accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}, contest: contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Visibility: "public", Settings: contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, FreezeStatus: contestsmodel.FreezeStatusAuto, Scoreboard: contestsmodel.Same(contestsmodel.ScopeParticipant), Submissions: contestsmodel.Same(contestsmodel.ScopeModerator), Sources: contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}}, permissions: contestPermissions{GetContest: true, ListOwnSubmissions: true}}
	f := &wsFixture{stub: stub, hub: NewHub(stub, nil), contests: NewContestsHub(), session: uuid.New()}
	observer := NewObserver(f.hub, f.contests, middleware.AuthMiddleware(stub, middleware.ResponseErrorHandler(slog.Default())), stub, stub, stub)
	f.server = httptest.NewServer(observer.Handler())
	t.Cleanup(f.server.Close)
	return f
}

func (f *wsFixture) dial(t *testing.T, path string, authenticated bool) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	headers := http.Header{}
	if authenticated {
		headers.Set("Cookie", "session_id="+f.session.String())
	}
	conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+path, headers)
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	if err != nil {
		return conn, resp, fmt.Errorf("dial observer: %w", err)
	}
	return conn, resp, nil
}

func (f *wsFixture) connect(t *testing.T, path string) *websocket.Conn {
	t.Helper()
	conn, resp, err := f.dial(t, path, true)
	if resp != nil {
		require.NoError(t, resp.Body.Close())
	}
	require.NoError(t, err)
	return conn
}

func (f *wsFixture) requireUnsubscribed(t *testing.T) {
	t.Helper()
	f.hub.mu.RLock()
	submissions := len(f.hub.subscribers)
	f.hub.mu.RUnlock()
	f.contests.mu.RLock()
	contests := len(f.contests.subscribers)
	f.contests.mu.RUnlock()
	require.Zero(t, submissions, "submission subscriptions after close")
	require.Zero(t, contests, "contest subscriptions after close")
}

func (f *wsFixture) waitSubscribed(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		f.hub.mu.RLock()
		defer f.hub.mu.RUnlock()
		return len(f.hub.subscribers) == 1
	}, 5*time.Second, 5*time.Millisecond)
}

func (f *wsFixture) waitContestSubscribed(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		f.contests.mu.RLock()
		defer f.contests.mu.RUnlock()
		return len(f.contests.subscribers) == 1
	}, 5*time.Second, 5*time.Millisecond)
}

func (f *wsFixture) path(kind string) string {
	return "/ws/" + kind + "?contestId=" + f.stub.contest.ID.String()
}

func (f *wsFixture) publish(t *testing.T, eventType string, event any) {
	t.Helper()
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	ctx := context.Background()
	if strings.HasPrefix(eventType, "submissions.") {
		require.NoError(t, f.hub.Handle(ctx, eventType, payload))
	} else {
		require.NoError(t, f.contests.Handle(ctx, eventType, payload))
	}
}

func (f *wsFixture) meta(author uuid.UUID) submissionsmodel.SubmissionEventMeta {
	return submissionsmodel.SubmissionEventMeta{UserId: &author, ContestId: &f.stub.contest.ID, ProblemId: ptrUUID(uuid.New()), CreatedAt: time.Now(), Language: "cpp17"}
}

func ptrUUID(v uuid.UUID) *uuid.UUID { return &v }

func readPayload(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	var msg struct {
		Payload map[string]any `json:"payload"`
	}
	require.NoError(t, conn.ReadJSON(&msg))
	return msg.Payload
}

func requireClose(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	_, _, err := conn.ReadMessage()
	require.True(t, websocket.IsCloseError(err, code), "expected %d, got %v", code, err)
}

func TestHandshake(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, test := range []struct {
			name        string
			auth        bool
			suffix      string
			err         error
			permissions contestPermissions
			status      int
		}{
			{name: "guest without audience", status: 401},
			{name: "malformed contest", auth: true, suffix: "&contestId=bad", status: 400},
			{name: "malformed user", auth: true, suffix: "&userId=bad", status: 400},
			{name: "missing contest", auth: true, err: pkg.ErrNotFound, status: 403},
			{name: "private stranger", auth: true, status: 403},
			{name: "database error", auth: true, err: errors.New("db unavailable"), status: 500},
			{name: "allowed", auth: true, permissions: contestPermissions{GetContest: true, GetMonitor: true}, status: 101},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				f := newWSFixture(t)
				f.stub.permissions = test.permissions
				f.stub.permissionErr = test.err
				conn, resp, err := f.dial(t, f.path(kind)+test.suffix, test.auth)
				require.NotNil(t, resp)
				require.Equal(t, test.status, resp.StatusCode)
				if conn == nil {
					require.Error(t, err)
					require.NoError(t, resp.Body.Close())
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
	t.Run("global admin only", func(t *testing.T) {
		f := newWSFixture(t)
		_, resp, err := f.dial(t, "/ws/submissions", true)
		require.Error(t, err)
		require.Equal(t, 403, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
		f.stub.change(func() { f.stub.user.Role = accessmodel.UserRoleAdmin })
		f.connect(t, "/ws/submissions")
	})
}

func TestSubmissionQueueLiveAndRevocation(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = contestPermissions{GetMonitor: true, ListOwnSubmissions: true}
	f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, Scoreboard: contestsmodel.Same(contestsmodel.ScopePublic), Submissions: contestsmodel.Same(contestsmodel.ScopePublic), FreezeStatus: contestsmodel.FreezeStatusFrozen, Sources: contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}
	other := uuid.New()
	meta := f.meta(other)
	created := &submissionsmodel.SubmissionCreatedEvent{SubmissionEventMeta: meta, Id: uuid.New(), State: submissionsmodel.Saved, Source: "secret code"}
	completed := &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: created.Id, State: submissionsmodel.Accepted, Score: 100, TimeStat: 42, MemoryStat: 256}
	// Run during delivery authorization. Synchronous broadcast proves no hub/subscriber lock is held.
	f.stub.onPermissions = func() {
		f.stub.change(func() {
			f.stub.onPermissions = func() { f.publish(t, submissionsmodel.SubmissionEventCompleted, completed) }
		})
	}
	conn := f.connect(t, f.path("submissions"))
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	for range 2 {
		payload := readPayload(t, conn)
		require.Equal(t, true, payload["result_hidden"])
		for _, key := range []string{"state", "score", "penalty", "time_stat", "memory_stat", "source", "failed_test"} {
			require.NotContains(t, payload, key)
		}
	}
	// Hidden test progress never arrives; the own completion that follows remains complete.
	f.publish(t, submissionsmodel.SubmissionEventTestStarted, &submissionsmodel.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: created.Id, Number: 99})
	own := *completed
	own.SubmissionEventMeta = f.meta(f.stub.user.Id)
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &own)
	payload := readPayload(t, conn)
	require.InDelta(t, float64(submissionsmodel.Accepted), payload["state"], 0)
	require.NotContains(t, payload, "result_hidden")
	f.stub.change(func() { f.stub.permissions = contestPermissions{} })
	f.publish(t, submissionsmodel.SubmissionEventCompleted, completed)
	requireClose(t, conn, 1008)
}

func TestDeliveryRevalidates(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, test := range []struct {
			name   string
			code   int
			change func(*accessStub)
		}{
			{"session revoked", 1008, func(m *accessStub) { m.authErr = pkg.ErrUnauthenticated }},
			{"membership revoked", 1008, func(m *accessStub) { m.permissions = contestPermissions{} }},
			{"contest deleted", 1008, func(m *accessStub) { m.permissionErr = pkg.ErrNotFound }},
			{"permissions database error", 1011, func(m *accessStub) { m.permissionErr = errors.New("db") }},
			{"session database error", 1011, func(m *accessStub) { m.authErr = errors.New("db") }},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				f := newWSFixture(t)
				f.stub.permissions = contestPermissions{GetContest: true, GetMonitor: true}
				conn := f.connect(t, f.path(kind))
				f.stub.change(func() { test.change(f.stub) })
				if kind == "submissions" {
					f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(uuid.New()), Id: uuid.New()})
				} else {
					f.publish(t, contestsmodel.ContestEventAnnouncementDeleted, &contestsmodel.ContestAnnouncementDeletedEvent{ContestID: f.stub.contest.ID, ID: uuid.New()})
				}
				requireClose(t, conn, test.code)
			})
		}
	}
}

func TestSourceDetailsAndModeratorDemotion(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = contestPermissions{GetMonitor: true, ManageContest: true, GetOtherUserSubmission: true, GetSubmissionDetails: true}
	f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, Scoreboard: contestsmodel.Same(contestsmodel.ScopePublic), Submissions: contestsmodel.Same(contestsmodel.ScopePublic), FreezeStatus: contestsmodel.FreezeStatusFrozen, Sources: contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}
	conn := f.connect(t, f.path("submissions"))
	meta := f.meta(uuid.New())
	id := uuid.New()
	created := &submissionsmodel.SubmissionCreatedEvent{SubmissionEventMeta: meta, Id: id, State: 1, Source: "code"}
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	require.Equal(t, "code", readPayload(t, conn)["source"])
	f.publish(t, submissionsmodel.SubmissionEventTestStarted, &submissionsmodel.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: id, Number: 7})
	require.InDelta(t, float64(7), readPayload(t, conn)["number"], 0)
	f.stub.change(func() { f.stub.permissions = contestPermissions{GetMonitor: true} })
	requireClose(t, conn, websocket.CloseTryAgainLater)
	f.requireUnsubscribed(t)
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	f.waitSubscribed(t)
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	require.Equal(t, true, readPayload(t, conn)["result_hidden"])
	f.stub.change(func() {
		f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, Scoreboard: contestsmodel.Same(contestsmodel.ScopePublic), Submissions: contestsmodel.Same(contestsmodel.ScopePublic), FreezeStatus: contestsmodel.FreezeStatusUnfrozen, Sources: contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}
	})
	requireClose(t, conn, websocket.CloseTryAgainLater)
	f.requireUnsubscribed(t)
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	f.waitSubscribed(t)
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	payload := readPayload(t, conn)
	require.NotContains(t, payload, "source")
	require.NotContains(t, payload, "result_hidden")
	f.publish(t, submissionsmodel.SubmissionEventTestStarted, &submissionsmodel.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: id, Number: 7})
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: id, State: 200})
	require.InDelta(t, float64(200), readPayload(t, conn)["state"], 0)
	// A participant can reconnect to the own projection without review permission.
	f.stub.change(func() { f.stub.permissions.ListOwnSubmissions = true })
	requireClose(t, conn, websocket.CloseTryAgainLater)
	f.requireUnsubscribed(t)
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	f.waitSubscribed(t)
	created.SubmissionEventMeta = f.meta(f.stub.user.Id)
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	require.Equal(t, "code", readPayload(t, conn)["source"])
}

func TestClarificationsUseCurrentManagePermission(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = contestPermissions{GetContest: true, ManageContest: true}
	other := uuid.New()
	event := &contestsmodel.ContestClarificationCreatedEvent{ContestID: f.stub.contest.ID, UserID: other, ID: uuid.New(), Question: "private"}
	conn := f.connect(t, f.path("contests"))
	f.publish(t, contestsmodel.ContestEventClarificationCreated, event)
	require.Equal(t, "private", readPayload(t, conn)["question"])
	f.stub.change(func() { f.stub.permissions.ManageContest = false; f.stub.permissions.ListOwnSubmissions = true })
	requireClose(t, conn, websocket.CloseTryAgainLater)
	f.requireUnsubscribed(t)
	conn = f.connect(t, "/ws/contests?contestId="+f.stub.contest.ID.String())
	f.waitContestSubscribed(t)
	f.publish(t, contestsmodel.ContestEventClarificationCreated, event)
	eventCopy := *event
	eventCopy.UserID = f.stub.user.Id
	eventCopy.Question = "mine"
	f.publish(t, contestsmodel.ContestEventClarificationCreated, &eventCopy)
	require.Equal(t, "mine", readPayload(t, conn)["question"])
	f.publish(t, contestsmodel.ContestEventAnnouncementDeleted, &contestsmodel.ContestAnnouncementDeletedEvent{ContestID: f.stub.contest.ID, ID: uuid.New()})
	require.Contains(t, readPayload(t, conn), "id")
}

func TestFiltersNarrowAccessAndRequireMetadata(t *testing.T) {
	f := newWSFixture(t)
	conn := f.connect(t, f.path("submissions"))
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(uuid.New()), Id: uuid.New(), State: 106})
	own := &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(f.stub.user.Id), Id: uuid.New(), State: 200}
	f.publish(t, submissionsmodel.SubmissionEventCompleted, own)
	require.Equal(t, own.Id.String(), readPayload(t, conn)["id"])
	for _, filter := range []*SubmissionsFilter{{ContestId: ptrUUID(uuid.New())}, {UserId: ptrUUID(uuid.New())}, {ProblemId: ptrUUID(uuid.New())}} {
		require.False(t, filter.Matches(&WrappedEvent{RawEvent: &submissionsmodel.SubmissionCompletedEvent{}}))
	}
	_, resp, err := f.dial(t, f.path("submissions")+"&userId="+uuid.NewString(), true)
	require.Error(t, err)
	require.Equal(t, 403, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestCatchUpAccessLoss(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, phase := range []string{"handshake", "queue"} {
			for _, code := range []int{1008, 1011} {
				t.Run(fmt.Sprintf("%s/%s/%d", kind, phase, code), func(t *testing.T) {
					f := newWSFixture(t)
					f.stub.permissions = contestPermissions{GetContest: true, GetMonitor: true}
					publish := func() {
						if kind == "submissions" {
							f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(uuid.New()), Id: uuid.New(), State: 200})
						} else {
							f.publish(t, contestsmodel.ContestEventAnnouncementDeleted, &contestsmodel.ContestAnnouncementDeletedEvent{ContestID: f.stub.contest.ID, ID: uuid.New()})
						}
					}
					publish()
					revoke := func() {
						f.stub.change(func() {
							if code == 1008 {
								f.stub.authErr = pkg.ErrUnauthenticated
							} else {
								f.stub.authErr = errors.New("db")
							}
						})
					}
					if phase == "handshake" {
						f.stub.onPermissions = revoke
					} else {
						f.stub.onPermissions = func() { f.stub.change(func() { f.stub.onPermissions = func() { publish(); revoke() } }) }
					}
					if phase == "handshake" {
						conn, response, err := f.dial(t, f.path(kind), true)
						require.Error(t, err)
						require.Nil(t, conn)
						expected := 500
						if code == 1008 {
							expected = 401
						}
						require.Equal(t, expected, response.StatusCode)
						require.NoError(t, response.Body.Close())
					} else {
						conn := f.connect(t, f.path(kind))
						requireClose(t, conn, code)
					}
				})
			}
		}
	}
}

func TestFreezeBoundaryJSON(t *testing.T) {
	for _, test := range []struct {
		name, status                       string
		offset                             time.Duration
		own, moderator, noBoundary, frozen bool
	}{
		{name: "before (late verdict remains hidden)", status: "auto", offset: -time.Nanosecond, frozen: true},
		{name: "boundary", status: "auto", frozen: true},
		{name: "after", status: "auto", offset: time.Nanosecond, frozen: true},
		{name: "own", status: "auto", own: true},
		{name: "moderator", status: "auto", moderator: true},
		{name: "unfrozen", status: "unfrozen"},
		{name: "forced without boundary", status: "frozen", noBoundary: true, frozen: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newWSFixture(t)
			f.stub.permissions = contestPermissions{GetMonitor: true, ManageContest: test.moderator, ListOwnSubmissions: test.own}
			end := time.Now().Add(30 * time.Minute)
			boundary := end.Add(-time.Hour)
			f.stub.contest.EndTime = &end
			f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, Scoreboard: contestsmodel.Same(contestsmodel.ScopePublic), Submissions: contestsmodel.Same(contestsmodel.ScopePublic), FreezeStatus: contestsmodel.FreezeStatus(test.status), FreezeDurationMinutes: new(int32(60)), Sources: contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}
			if test.noBoundary {
				f.stub.contest.EndTime = nil
			}
			author := uuid.New()
			if test.own {
				author = f.stub.user.Id
			}
			meta := f.meta(author)
			meta.CreatedAt = boundary.Add(test.offset)
			failed := int32(7)
			conn := f.connect(t, f.path("submissions"))
			f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: uuid.New(), State: 106, FailedTest: &failed})
			payload := readPayload(t, conn)
			if test.frozen {
				require.Equal(t, true, payload["result_hidden"])
				require.NotContains(t, payload, "state")
				require.NotContains(t, payload, "failed_test")
			} else {
				require.NotContains(t, payload, "result_hidden")
				require.InDelta(t, 106, payload["state"], 0)
				require.InDelta(t, 7, payload["failed_test"], 0)
			}
		})
	}
}

func TestPublicGuestAudience(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = contestPermissions{GetContest: true, GetMonitor: true}
	f.stub.contest.Settings = contestsmodel.ContestSettings{JoinPolicy: accessmodel.JoinOpen, Scoreboard: contestsmodel.Same(contestsmodel.ScopePublic), Submissions: contestsmodel.Same(contestsmodel.ScopePublic), FreezeStatus: contestsmodel.FreezeStatusFrozen, Sources: contestsmodel.Same(contestsmodel.ScopeModerator), Reports: contestsmodel.Same(contestsmodel.ScopeModerator), Feedback: contestsmodel.Same(contestsmodel.FeedbackVerdict)}
	conn, resp, err := f.dial(t, f.path("submissions"), false)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: 200, SubmissionEventMeta: f.meta(uuid.New())})
	payload := readPayload(t, conn)
	require.Equal(t, true, payload["result_hidden"])
	require.NotContains(t, payload, "state")
}

func TestUpsolverGetsPublicAudience(t *testing.T) {
	f := newWSFixture(t)
	end := time.Now().Add(-time.Minute)
	c := f.stub.contest
	c.EndTime = &end
	c.Settings.EnableUpsolving = true
	c.Settings.JoinPolicy = accessmodel.JoinInviteOnly
	c.Settings.Scoreboard = contestsmodel.Same(contestsmodel.ScopePublic)
	c.Settings.Submissions = contestsmodel.Same(contestsmodel.ScopeParticipant)
	f.stub.contest.Settings, f.stub.contest.EndTime = c.Settings, c.EndTime
	f.stub.facts = &access.ContestFacts{Contest: c, Now: time.Now()}
	conn := f.connect(t, f.path("submissions"))
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: 200, SubmissionEventMeta: f.meta(uuid.New())})
	own := uuid.New()
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: own, State: 200, SubmissionEventMeta: f.meta(f.stub.user.Id)})
	// Participant-scoped submissions of others must not reach an upsolver, so the first frame is its own.
	require.Equal(t, own.String(), readPayload(t, conn)["id"])
}

func TestInvalidSessionNeverDowngradesToPublicGuest(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = contestPermissions{GetContest: true, GetMonitor: true}
	f.stub.authErr = pkg.ErrUnauthenticated
	conn, response, err := f.dial(t, f.path("submissions"), true)
	require.Error(t, err)
	require.Nil(t, conn)
	require.Equal(t, 401, response.StatusCode)
	require.NoError(t, response.Body.Close())
}
