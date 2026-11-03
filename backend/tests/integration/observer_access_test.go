//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	access "github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	eventdata "github.com/brawler2011/gate/backend/internal/events"
	identity "github.com/brawler2011/gate/backend/internal/identity"
	organizationsmodel "github.com/brawler2011/gate/backend/internal/organizations/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/internal/transport/ws/observer"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (s *IntegrationTestSuite) TestObserverRoleSourcesAndRevocation() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("ws_owner_"+suffix, accessmodel.UserRoleUser)
	orgOwner := s.createUser("ws_org_owner_"+suffix, accessmodel.UserRoleUser)
	org := s.createOrganization("ws-"+suffix, "Observer", orgOwner.Id)
	contestID := uuid.New()
	start := time.Now().Add(-time.Hour)
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, OwnerID: &owner.Id, Login: "ws-" + suffix, Title: "Observer", Visibility: contestsmodel.ContestVisibilityPrivate, StartTime: &start, Settings: map[string]any{"freeze_status": contestsmodel.FreezeStatusFrozen}}))
	authRepo := pgstore.NewAuthRepo(s.dbPool)
	tx := pgstore.NewIdentityTransactions(s.dbPool)
	users := identity.NewUsersUseCase(s.usersRepo, tx, nil)
	auth := identity.NewAuthUseCase(s.usersRepo, authRepo, tx)
	permissions := access.NewPermissionsUseCase(s.contestsRepo, users, s.problemsRepo, s.teamsRepo, s.organizationsRepo)
	contests := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool))
	hub := observer.NewHub(100, contests)
	contestHub := observer.NewContestsHub(100)
	server := httptest.NewServer(observer.NewObserver(hub, contestHub, middleware.AuthMiddleware(auth), auth, permissions, contests).Handler())
	defer server.Close()
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "WS team", Slug: "ws-" + suffix, Privacy: organizationsmodel.TeamPrivacyClosed})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.CreateContestTeam(s.ctx, contestID, team.ID, contestsmodel.ContestRoleModerator))
	var seq uint64
	for _, source := range []string{"global admin", "contest owner", "organization admin", "organization owner", "direct moderator", "team moderator", "participant", "stranger"} {
		s.Run(source, func() {
			user := owner
			if source == "organization owner" {
				user = orgOwner
			}
			if source != "contest owner" && source != "organization owner" {
				role := accessmodel.UserRoleUser
				if source == "global admin" {
					role = accessmodel.UserRoleAdmin
				}
				user = s.createUser("ws_"+strings.ReplaceAll(source, " ", "_")+"_"+suffix, role)
			}
			switch source {
			case "organization admin":
				s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, user.Id, organizationsmodel.OrgRoleAdmin))
			case "direct moderator":
				s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: user.Id, Role: contestsmodel.ContestRoleModerator}))
			case "participant":
				s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: user.Id, Role: contestsmodel.ContestRoleParticipant}))
			case "team moderator":
				s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, user.Id, organizationsmodel.TeamRoleMember))
			}
			sessionID := uuid.New()
			s.Require().NoError(authRepo.CreateSession(s.ctx, sessionID, user.Id, time.Now().Add(40*time.Minute)))
			headers := http.Header{"Cookie": []string{"session_id=" + sessionID.String()}}
			conn, resp, dialErr := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/submissions?contestId="+contestID.String()+"&since="+strconv.FormatUint(seq, 10), headers)
			if resp != nil {
				s.Require().NoError(resp.Body.Close())
			}
			if source == "stranger" {
				s.Require().Error(dialErr)
				s.Require().Equal(403, resp.StatusCode)
				return
			}
			s.Require().NoError(dialErr)
			defer conn.Close()
			seq++
			event := submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{ContestId: &contestID, UserId: &owner.Id, CreatedAt: time.Now()}, Id: uuid.New(), State: 200, Score: 100}
			payload, marshalErr := json.Marshal(event)
			s.Require().NoError(marshalErr)
			s.Require().NoError(hub.Handle(context.WithValue(s.ctx, eventdata.MessageStreamSequenceKey, seq), submissionsmodel.SubmissionEventCompleted, payload))
			s.Require().NoError(conn.SetReadDeadline(time.Now().Add(3 * time.Second)))
			var msg struct {
				Payload map[string]any `json:"payload"`
			}
			s.Require().NoError(conn.ReadJSON(&msg))
			if source == "participant" {
				s.Require().Equal(true, msg.Payload["frozen"])
				s.Require().NotContains(msg.Payload, "state")
			} else {
				s.Require().InDelta(200, msg.Payload["state"], 0)
			}
			access, accessErr := permissions.GetContestPermissions(s.ctx, contestID, user.Id)
			s.Require().NoError(accessErr)
			if access.GetMonitor {
				s.Require().NoError(conn.ReadJSON(&msg))
				s.Require().Equal(false, msg.Payload["unfrozen"])
				if access.ManageContest {
					s.Require().NoError(conn.ReadJSON(&msg))
					s.Require().Equal(true, msg.Payload["unfrozen"])
				}
			}
			s.Require().NoError(authRepo.DeleteSession(s.ctx, sessionID))
			seq++
			s.Require().NoError(hub.Handle(context.WithValue(s.ctx, eventdata.MessageStreamSequenceKey, seq), submissionsmodel.SubmissionEventCompleted, payload))
			_, _, readErr := conn.ReadMessage()
			s.Require().True(websocket.IsCloseError(readErr, 1008), "%v", readErr)
		})
	}
}
