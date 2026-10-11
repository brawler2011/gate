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

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/repository/pg"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/internal/transport/ws/observer"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (s *IntegrationTestSuite) TestObserverRoleSourcesAndRevocation() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("ws_owner_"+suffix, models.UserRoleUser)
	orgOwner := s.createUser("ws_org_owner_"+suffix, models.UserRoleUser)
	org := s.createOrganization("ws-"+suffix, "Observer", orgOwner.Id)
	contestID := uuid.New()
	start := time.Now().Add(-time.Hour)
	s.Require().NoError(s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{ID: contestID, OrganizationID: org.ID, OwnerID: &owner.Id, Login: "ws-" + suffix, Title: "Observer", Visibility: models.ContestVisibilityPrivate, StartTime: &start, Settings: map[string]any{"freeze_status": models.FreezeStatusFrozen}}))
	authRepo := pg.NewAuthRepo(s.dbPool)
	tx := pg.NewTransactor(s.dbPool)
	users := usecase.NewUsersUseCase(s.usersRepo, s.contestsRepo, pg.NewOutboxRepo(s.dbPool), tx, authRepo, nil)
	auth := usecase.NewAuthUseCase(s.usersRepo, authRepo, tx)
	permissions := usecase.NewPermissionsUseCase(s.contestsRepo, users, s.problemsRepo, s.teamsRepo, s.organizationsRepo)
	contests := usecase.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pg.NewSubmissionsRepo(s.dbPool))
	hub := observer.NewHub(100)
	contestHub := observer.NewContestsHub(100)
	server := httptest.NewServer(observer.NewObserver(hub, contestHub, middleware.AuthMiddleware(auth), auth, permissions, contests).Handler())
	defer server.Close()
	team, err := s.teamsRepo.CreateTeam(s.ctx, &models.CreateTeamInput{OrganizationID: org.ID, Name: "WS team", Slug: "ws-" + suffix, Privacy: models.TeamPrivacyClosed})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.CreateContestTeam(s.ctx, contestID, team.ID, models.ContestRoleModerator))
	var seq uint64
	for _, source := range []string{"global admin", "contest owner", "organization admin", "organization owner", "direct moderator", "team moderator", "participant", "stranger"} {
		s.Run(source, func() {
			user := owner
			if source == "organization owner" {
				user = orgOwner
			}
			if source != "contest owner" && source != "organization owner" {
				role := models.UserRoleUser
				if source == "global admin" {
					role = models.UserRoleAdmin
				}
				user = s.createUser("ws_"+strings.ReplaceAll(source, " ", "_")+"_"+suffix, role)
			}
			switch source {
			case "organization admin":
				s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, user.Id, models.OrgRoleAdmin))
			case "direct moderator":
				s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{ContestId: contestID, UserId: user.Id, Role: models.ContestRoleModerator}))
			case "participant":
				s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &models.CreateContestMemberParams{ContestId: contestID, UserId: user.Id, Role: models.ContestRoleParticipant}))
			case "team moderator":
				s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, user.Id, models.TeamRoleMember))
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
			event := models.SubmissionCompletedEvent{SubmissionEventMeta: models.SubmissionEventMeta{ContestId: &contestID, UserId: &owner.Id, CreatedAt: time.Now()}, Id: uuid.New(), State: 200, Score: 100}
			payload, marshalErr := json.Marshal(event)
			s.Require().NoError(marshalErr)
			s.Require().NoError(hub.Handle(context.WithValue(s.ctx, models.MessageStreamSequenceKey, seq), models.SubmissionEventCompleted, payload))
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
			s.Require().NoError(authRepo.DeleteSession(s.ctx, sessionID))
			seq++
			s.Require().NoError(hub.Handle(context.WithValue(s.ctx, models.MessageStreamSequenceKey, seq), models.SubmissionEventCompleted, payload))
			_, _, readErr := conn.ReadMessage()
			s.Require().True(websocket.IsCloseError(readErr, 1008), "%v", readErr)
		})
	}
}
