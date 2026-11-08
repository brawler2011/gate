package observer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type accessStub struct {
	AuthUC
	PermissionsUC
	ContestReader
	Scoreboard
	mu            sync.Mutex
	user          accessmodel.User
	permissions   accessmodel.ContestPermissions
	contest       contestsmodel.Contest
	authErr       error
	permissionErr error
	contestErr    error
	onPermissions func()
}

func (m *accessStub) Authenticate(context.Context, uuid.UUID) (accessmodel.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.user, m.authErr
}

func (m *accessStub) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*accessmodel.ContestPermissions, error) {
	m.mu.Lock()
	p, err, hook := m.permissions, m.permissionErr, m.onPermissions
	m.onPermissions = nil
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	return &p, err
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
	seq      uint64
}

func newWSFixture(t *testing.T) *wsFixture {
	t.Helper()
	stub := &accessStub{user: accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}, contest: contestsmodel.Contest{ID: uuid.New(), OrganizationID: uuid.New(), Visibility: "public"}, permissions: accessmodel.ContestPermissions{GetContest: true, ListOwnSubmissions: true}}
	f := &wsFixture{stub: stub, hub: NewHub(100, stub, nil), contests: NewContestsHub(100), session: uuid.New()}
	observer := NewObserver(f.hub, f.contests, middleware.AuthMiddleware(stub), stub, stub, stub)
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

func (f *wsFixture) path(kind string) string {
	return "/ws/" + kind + "?contestId=" + f.stub.contest.ID.String() + "&since=0"
}

func (f *wsFixture) publish(t *testing.T, eventType string, event any) {
	t.Helper()
	seq := atomic.AddUint64(&f.seq, 1)
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), eventdata.MessageStreamSequenceKey, seq)
	if strings.HasPrefix(eventType, "submissions.") {
		require.NoError(t, f.hub.Handle(ctx, eventType, payload))
	} else {
		require.NoError(t, f.contests.Handle(ctx, eventType, payload))
	}
}

func (f *wsFixture) meta(author uuid.UUID) submissionsmodel.SubmissionEventMeta {
	return submissionsmodel.SubmissionEventMeta{UserId: &author, ContestId: &f.stub.contest.ID, ProblemId: ptrUUID(uuid.New()), CreatedAt: time.Now(), Language: submissionsmodel.Cpp}
}

func ptrUUID(v uuid.UUID) *uuid.UUID { return &v }

func readPayload(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	var msg struct {
		Payload map[string]any `json:"payload"`
	}
	require.NoError(t, conn.ReadJSON(&msg))
	return msg.Payload
}

func requireClose(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
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
			permissions accessmodel.ContestPermissions
			status      int
		}{
			{name: "guest without audience", status: 403},
			{name: "malformed contest", auth: true, suffix: "&contestId=bad", status: 400},
			{name: "malformed since", auth: true, suffix: "&since=bad", status: 400},
			{name: "empty since", auth: true, suffix: "&since=", status: 400},
			{name: "malformed user", auth: true, suffix: "&userId=bad", status: 400},
			{name: "missing contest", auth: true, err: pkg.ErrNotFound, status: 403},
			{name: "private stranger", auth: true, status: 403},
			{name: "database error", auth: true, err: errors.New("db unavailable"), status: 500},
			{name: "allowed", auth: true, permissions: accessmodel.ContestPermissions{GetContest: true, GetMonitor: true}, status: 101},
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

func TestSubmissionHistoryQueueLiveAndRevocation(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = accessmodel.ContestPermissions{GetMonitor: true, ListOwnSubmissions: true}
	f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": contestsmodel.FreezeStatusFrozen}
	other := uuid.New()
	meta := f.meta(other)
	created := &submissionsmodel.SubmissionCreatedEvent{SubmissionEventMeta: meta, Id: uuid.New(), State: submissionsmodel.Saved, Source: "secret code"}
	completed := &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: created.Id, State: submissionsmodel.Accepted, Score: 100, TimeStat: 42, MemoryStat: 256}
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	// Run during history authorization. Synchronous broadcast proves no hub/subscriber lock is held.
	f.stub.onPermissions = func() {
		f.stub.change(func() {
			f.stub.onPermissions = func() { f.publish(t, submissionsmodel.SubmissionEventCompleted, completed) }
		})
	}
	conn := f.connect(t, f.path("submissions"))
	for range 2 {
		payload := readPayload(t, conn)
		require.Equal(t, true, payload["frozen"])
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
	require.NotContains(t, payload, "frozen")
	f.stub.change(func() { f.stub.permissions = accessmodel.ContestPermissions{} })
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
			{"membership revoked", 1008, func(m *accessStub) { m.permissions = accessmodel.ContestPermissions{} }},
			{"contest deleted", 1008, func(m *accessStub) { m.permissionErr = pkg.ErrNotFound }},
			{"permissions database error", 1011, func(m *accessStub) { m.permissionErr = errors.New("db") }},
			{"session database error", 1011, func(m *accessStub) { m.authErr = errors.New("db") }},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				f := newWSFixture(t)
				f.stub.permissions = accessmodel.ContestPermissions{GetContest: true, GetMonitor: true}
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
	f.stub.permissions = accessmodel.ContestPermissions{GetMonitor: true, ManageContest: true, GetOtherUserSubmission: true, GetSubmissionDetails: true}
	f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": contestsmodel.FreezeStatusFrozen}
	conn := f.connect(t, f.path("submissions"))
	meta := f.meta(uuid.New())
	id := uuid.New()
	created := &submissionsmodel.SubmissionCreatedEvent{SubmissionEventMeta: meta, Id: id, State: 1, Source: "code"}
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	require.Equal(t, "code", readPayload(t, conn)["source"])
	f.publish(t, submissionsmodel.SubmissionEventTestStarted, &submissionsmodel.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: id, Number: 7})
	require.InDelta(t, float64(7), readPayload(t, conn)["number"], 0)
	f.stub.change(func() { f.stub.permissions = accessmodel.ContestPermissions{GetMonitor: true} })
	requireClose(t, conn, 1011)
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	require.Equal(t, true, readPayload(t, conn)["frozen"])
	f.stub.change(func() {
		f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": contestsmodel.FreezeStatusUnfrozen}
	})
	requireClose(t, conn, 1011)
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	payload := readPayload(t, conn)
	require.NotContains(t, payload, "source")
	require.NotContains(t, payload, "frozen")
	f.publish(t, submissionsmodel.SubmissionEventTestStarted, &submissionsmodel.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: id, Number: 7})
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: id, State: 200})
	require.InDelta(t, float64(200), readPayload(t, conn)["state"], 0)
	// A participant can reconnect to the own projection without review permission.
	f.stub.change(func() { f.stub.permissions.ListOwnSubmissions = true })
	requireClose(t, conn, 1011)
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	created.SubmissionEventMeta = f.meta(f.stub.user.Id)
	f.publish(t, submissionsmodel.SubmissionEventCreated, created)
	require.Equal(t, "code", readPayload(t, conn)["source"])
}

func TestClarificationsUseCurrentManagePermission(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = accessmodel.ContestPermissions{GetContest: true, ManageContest: true}
	other := uuid.New()
	event := &contestsmodel.ContestClarificationCreatedEvent{ContestID: f.stub.contest.ID, UserID: other, ID: uuid.New(), Question: "private"}
	f.publish(t, contestsmodel.ContestEventClarificationCreated, event)
	conn := f.connect(t, f.path("contests"))
	require.Equal(t, "private", readPayload(t, conn)["question"])
	f.stub.change(func() { f.stub.permissions.ManageContest = false; f.stub.permissions.ListOwnSubmissions = true })
	requireClose(t, conn, 1011)
	conn = f.connect(t, "/ws/contests?contestId="+f.stub.contest.ID.String())
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
		for _, phase := range []string{"history", "queue"} {
			for _, code := range []int{1008, 1011} {
				t.Run(fmt.Sprintf("%s/%s/%d", kind, phase, code), func(t *testing.T) {
					f := newWSFixture(t)
					f.stub.permissions = accessmodel.ContestPermissions{GetContest: true, GetMonitor: true}
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
					if phase == "history" {
						f.stub.onPermissions = revoke
					} else {
						f.stub.onPermissions = func() { f.stub.change(func() { f.stub.onPermissions = func() { publish(); revoke() } }) }
					}
					if phase == "history" {
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
			f.stub.permissions = accessmodel.ContestPermissions{GetMonitor: true, ManageContest: test.moderator, ListOwnSubmissions: test.own}
			end := time.Now().Add(30 * time.Minute)
			boundary := end.Add(-time.Hour)
			f.stub.contest.EndTime = &end
			f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": test.status, "freeze_duration_minutes": 60}
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
			f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: uuid.New(), State: 106, FailedTest: &failed})
			conn := f.connect(t, f.path("submissions"))
			payload := readPayload(t, conn)
			if test.frozen {
				require.Equal(t, true, payload["frozen"])
				require.NotContains(t, payload, "state")
				require.NotContains(t, payload, "failed_test")
			} else {
				require.NotContains(t, payload, "frozen")
				require.InDelta(t, 106, payload["state"], 0)
				require.InDelta(t, 7, payload["failed_test"], 0)
			}
		})
	}
}

func TestPublicGuestAudience(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = accessmodel.ContestPermissions{GetContest: true, GetMonitor: true}
	f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": "frozen"}
	conn, resp, err := f.dial(t, f.path("submissions"), false)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: 200, SubmissionEventMeta: f.meta(uuid.New())})
	payload := readPayload(t, conn)
	require.Equal(t, true, payload["frozen"])
	require.NotContains(t, payload, "state")
}

type revisionStub struct {
	mu    sync.Mutex
	value int64
	err   error
}

func (r *revisionStub) AccessRevision(context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.value, r.err
}
func TestIdleRevisionInvalidation(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			f := newWSFixture(t)
			r := &revisionStub{value: 1}
			server := httptest.NewServer(NewObserver(f.hub, f.contests, middleware.AuthMiddleware(f.stub), f.stub, f.stub, f.stub, r).Handler())
			defer server.Close()
			conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+f.path("submissions"), http.Header{"Cookie": {"session_id=" + f.session.String()}})
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			defer conn.Close()
			r.mu.Lock()
			if failure {
				r.err = errors.New("revision channel unavailable")
			} else {
				r.value++
			}
			r.mu.Unlock()
			requireClose(t, conn, 1011)
		})
	}
}

func TestReplayRejectsFormerLiveProjectionAfterFreeze(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = accessmodel.ContestPermissions{GetContest: true, GetMonitor: true}
	f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": "unfrozen"}
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: 200, SubmissionEventMeta: f.meta(uuid.New())})
	f.stub.change(func() {
		f.stub.contest.Settings = map[string]any{"monitor_scope": "public", "submissions_list_scope": "public", "freeze_status": "frozen"}
	})
	conn := f.connect(t, f.path("submissions"))
	requireClose(t, conn, 1011)
	// A fresh subscription receives only the newly projected, masked payload.
	conn = f.connect(t, "/ws/submissions?contestId="+f.stub.contest.ID.String())
	f.publish(t, submissionsmodel.SubmissionEventCompleted, &submissionsmodel.SubmissionCompletedEvent{Id: uuid.New(), State: 200, SubmissionEventMeta: f.meta(uuid.New())})
	p := readPayload(t, conn)
	require.Equal(t, true, p["frozen"])
	require.NotContains(t, p, "state")
}

func TestInvalidSessionNeverDowngradesToPublicGuest(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = accessmodel.ContestPermissions{GetContest: true, GetMonitor: true}
	f.stub.authErr = pkg.ErrUnauthenticated
	conn, response, err := f.dial(t, f.path("submissions"), true)
	require.Error(t, err)
	require.Nil(t, conn)
	require.Equal(t, 401, response.StatusCode)
	require.NoError(t, response.Body.Close())
}
