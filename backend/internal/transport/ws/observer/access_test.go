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

	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type accessStub struct {
	interfaces.AuthUC
	interfaces.PermissionsUC
	interfaces.ContestsUC
	mu            sync.Mutex
	user          models.User
	permissions   models.ContestPermissions
	contest       models.Contest
	authErr       error
	permissionErr error
	contestErr    error
	onPermissions func()
}

func (m *accessStub) Authenticate(context.Context, uuid.UUID) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.user, m.authErr
}
func (m *accessStub) GetContestPermissions(context.Context, uuid.UUID, uuid.UUID) (*models.ContestPermissions, error) {
	m.mu.Lock()
	p, err, hook := m.permissions, m.permissionErr, m.onPermissions
	m.onPermissions = nil
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	return &p, err
}
func (m *accessStub) GetContest(context.Context, uuid.UUID) (models.Contest, error) {
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
	stub := &accessStub{user: models.User{Id: uuid.New(), Role: models.UserRoleUser}, contest: models.Contest{ID: uuid.New()}, permissions: models.ContestPermissions{GetContest: true, ListOwnSubmissions: true}}
	f := &wsFixture{stub: stub, hub: NewHub(100, nil), contests: NewContestsHub(100), session: uuid.New()}
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
	ctx := context.WithValue(context.Background(), models.MessageStreamSequenceKey, seq)
	if strings.HasPrefix(eventType, "submissions.") {
		require.NoError(t, f.hub.Handle(ctx, eventType, payload))
	} else {
		require.NoError(t, f.contests.Handle(ctx, eventType, payload))
	}
}
func (f *wsFixture) meta(author uuid.UUID) models.SubmissionEventMeta {
	return models.SubmissionEventMeta{UserId: &author, ContestId: &f.stub.contest.ID, ProblemId: ptrUUID(uuid.New()), CreatedAt: time.Now(), Language: models.Cpp}
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
			permissions models.ContestPermissions
			status      int
		}{
			{name: "guest", status: 401},
			{name: "malformed contest", auth: true, suffix: "&contestId=bad", status: 400},
			{name: "malformed since", auth: true, suffix: "&since=bad", status: 400},
			{name: "empty since", auth: true, suffix: "&since=", status: 400},
			{name: "malformed user", auth: true, suffix: "&userId=bad", status: 400},
			{name: "missing contest", auth: true, err: pkg.ErrNotFound, status: 403},
			{name: "private stranger", auth: true, status: 403},
			{name: "database error", auth: true, err: errors.New("db unavailable"), status: 500},
			{name: "allowed", auth: true, permissions: models.ContestPermissions{GetContest: true, GetMonitor: true}, status: 101},
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
		f.stub.change(func() { f.stub.user.Role = models.UserRoleAdmin })
		f.connect(t, "/ws/submissions")
	})
}

func TestSubmissionHistoryPendingLiveAndRevocation(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = models.ContestPermissions{GetMonitor: true}
	f.stub.contest.Settings = map[string]any{"freeze_status": models.FreezeStatusFrozen}
	other := uuid.New()
	meta := f.meta(other)
	created := &models.SubmissionCreatedEvent{SubmissionEventMeta: meta, Id: uuid.New(), State: models.Saved, Source: "secret code"}
	completed := &models.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: created.Id, State: models.Accepted, Score: 100, TimeStat: 42, MemoryStat: 256}
	f.publish(t, models.SubmissionEventCreated, created)
	// Run during history authorization. Synchronous broadcast proves no hub/subscriber lock is held.
	f.stub.onPermissions = func() {
		f.stub.change(func() { f.stub.onPermissions = func() { f.publish(t, models.SubmissionEventCompleted, completed) } })
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
	f.publish(t, models.SubmissionEventTestStarted, &models.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: created.Id, Number: 99})
	own := *completed
	own.SubmissionEventMeta = f.meta(f.stub.user.Id)
	f.publish(t, models.SubmissionEventCompleted, &own)
	payload := readPayload(t, conn)
	require.InDelta(t, float64(models.Accepted), payload["state"], 0)
	require.NotContains(t, payload, "frozen")
	f.stub.change(func() { f.stub.permissions = models.ContestPermissions{} })
	f.publish(t, models.SubmissionEventCompleted, completed)
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
			{"membership revoked", 1008, func(m *accessStub) { m.permissions = models.ContestPermissions{} }},
			{"contest deleted", 1008, func(m *accessStub) { m.permissionErr = pkg.ErrNotFound }},
			{"permissions database error", 1011, func(m *accessStub) { m.permissionErr = errors.New("db") }},
			{"session database error", 1011, func(m *accessStub) { m.authErr = errors.New("db") }},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				f := newWSFixture(t)
				f.stub.permissions = models.ContestPermissions{GetContest: true, GetMonitor: true}
				conn := f.connect(t, f.path(kind))
				f.stub.change(func() { test.change(f.stub) })
				if kind == "submissions" {
					f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(uuid.New()), Id: uuid.New()})
				} else {
					f.publish(t, models.ContestEventAnnouncementDeleted, &models.ContestAnnouncementDeletedEvent{ContestID: f.stub.contest.ID, ID: uuid.New()})
				}
				requireClose(t, conn, test.code)
			})
		}
	}
}

func TestSourceDetailsAndModeratorDemotion(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = models.ContestPermissions{GetMonitor: true, ManageContest: true, GetOtherUserSubmission: true, GetSubmissionDetails: true}
	f.stub.contest.Settings = map[string]any{"freeze_status": models.FreezeStatusFrozen}
	conn := f.connect(t, f.path("submissions"))
	meta := f.meta(uuid.New())
	id := uuid.New()
	created := &models.SubmissionCreatedEvent{SubmissionEventMeta: meta, Id: id, State: 1, Source: "code"}
	f.publish(t, models.SubmissionEventCreated, created)
	require.Equal(t, "code", readPayload(t, conn)["source"])
	f.publish(t, models.SubmissionEventTestStarted, &models.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: id, Number: 7})
	require.InDelta(t, float64(7), readPayload(t, conn)["number"], 0)
	f.stub.change(func() { f.stub.permissions = models.ContestPermissions{GetMonitor: true} })
	f.publish(t, models.SubmissionEventCreated, created)
	require.Equal(t, true, readPayload(t, conn)["frozen"])
	f.stub.change(func() { f.stub.contest.Settings = map[string]any{"freeze_status": models.FreezeStatusUnfrozen} })
	f.publish(t, models.SubmissionEventCreated, created)
	payload := readPayload(t, conn)
	require.NotContains(t, payload, "source")
	require.NotContains(t, payload, "frozen")
	f.publish(t, models.SubmissionEventTestStarted, &models.SubmissionTestStartedEvent{SubmissionEventMeta: meta, Id: id, Number: 7})
	f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: id, State: 200})
	require.InDelta(t, float64(200), readPayload(t, conn)["state"], 0)
	// Source belongs to author even without review permission.
	created.SubmissionEventMeta = f.meta(f.stub.user.Id)
	f.publish(t, models.SubmissionEventCreated, created)
	require.Equal(t, "code", readPayload(t, conn)["source"])
}

func TestClarificationsUseCurrentManagePermission(t *testing.T) {
	f := newWSFixture(t)
	f.stub.permissions = models.ContestPermissions{GetContest: true, ManageContest: true}
	other := uuid.New()
	event := &models.ContestClarificationCreatedEvent{ContestID: f.stub.contest.ID, UserID: other, ID: uuid.New(), Question: "private"}
	f.publish(t, models.ContestEventClarificationCreated, event)
	conn := f.connect(t, f.path("contests"))
	require.Equal(t, "private", readPayload(t, conn)["question"])
	f.stub.change(func() { f.stub.permissions.ManageContest = false })
	f.publish(t, models.ContestEventClarificationCreated, event)
	eventCopy := *event
	eventCopy.UserID = f.stub.user.Id
	eventCopy.Question = "mine"
	f.publish(t, models.ContestEventClarificationCreated, &eventCopy)
	require.Equal(t, "mine", readPayload(t, conn)["question"])
	f.publish(t, models.ContestEventAnnouncementDeleted, &models.ContestAnnouncementDeletedEvent{ContestID: f.stub.contest.ID, ID: uuid.New()})
	require.Contains(t, readPayload(t, conn), "id")
}

func TestFiltersNarrowAccessAndRequireMetadata(t *testing.T) {
	f := newWSFixture(t)
	conn := f.connect(t, f.path("submissions"))
	f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(uuid.New()), Id: uuid.New(), State: 106})
	own := &models.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(f.stub.user.Id), Id: uuid.New(), State: 200}
	f.publish(t, models.SubmissionEventCompleted, own)
	require.Equal(t, own.Id.String(), readPayload(t, conn)["id"])
	for _, filter := range []*SubmissionsFilter{{ContestId: ptrUUID(uuid.New())}, {UserId: ptrUUID(uuid.New())}, {ProblemId: ptrUUID(uuid.New())}} {
		require.False(t, filter.Matches(&WrappedEvent{RawEvent: &models.SubmissionCompletedEvent{}}))
	}
	_, resp, err := f.dial(t, f.path("submissions")+"&userId="+uuid.NewString(), true)
	require.Error(t, err)
	require.Equal(t, 403, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

func TestCatchUpAccessLoss(t *testing.T) {
	for _, kind := range []string{"submissions", "contests"} {
		for _, phase := range []string{"history", "pending"} {
			for _, code := range []int{1008, 1011} {
				t.Run(fmt.Sprintf("%s/%s/%d", kind, phase, code), func(t *testing.T) {
					f := newWSFixture(t)
					f.stub.permissions = models.ContestPermissions{GetContest: true, GetMonitor: true}
					publish := func() {
						if kind == "submissions" {
							f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{SubmissionEventMeta: f.meta(uuid.New()), Id: uuid.New(), State: 200})
						} else {
							f.publish(t, models.ContestEventAnnouncementDeleted, &models.ContestAnnouncementDeletedEvent{ContestID: f.stub.contest.ID, ID: uuid.New()})
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
					conn := f.connect(t, f.path(kind))
					if phase == "pending" {
						require.Contains(t, readPayload(t, conn), "id")
					}
					requireClose(t, conn, code)
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
		{name: "before", status: "auto", offset: -time.Nanosecond},
		{name: "boundary", status: "auto", frozen: true},
		{name: "after", status: "auto", offset: time.Nanosecond, frozen: true},
		{name: "own", status: "auto", own: true},
		{name: "moderator", status: "auto", moderator: true},
		{name: "unfrozen", status: "unfrozen"},
		{name: "forced without boundary", status: "frozen", noBoundary: true, frozen: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newWSFixture(t)
			f.stub.permissions = models.ContestPermissions{GetMonitor: true, ManageContest: test.moderator}
			end := time.Now().Add(30 * time.Minute)
			boundary := end.Add(-time.Hour)
			f.stub.contest.EndTime = &end
			f.stub.contest.Settings = map[string]any{"freeze_status": test.status, "freeze_duration_minutes": 60}
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
			f.publish(t, models.SubmissionEventCompleted, &models.SubmissionCompletedEvent{SubmissionEventMeta: meta, Id: uuid.New(), State: 106, FailedTest: &failed})
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
