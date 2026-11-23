//go:build integration

package integration

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	access "github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
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
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Login: "ws-" + suffix, Title: "Observer", Visibility: contestsmodel.ContestVisibilityPrivate, StartTime: &start}, owner.Id))
	frozen := contestsmodel.FreezeStatusFrozen
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: contestID, Settings: contestsmodel.ContestSettingsPatch{FreezeStatus: &frozen}}))
	authRepo := pgstore.NewAuthRepo(s.dbPool)
	tx := pgstore.NewIdentityTransactions(s.dbPool)
	auth := identity.NewAuthUseCase(s.usersRepo, authRepo, tx, nil, nil)
	loader := pgstore.NewAccess(s.dbPool)
	contests := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), loader)
	hub := observer.NewHub(s.contestsRepo, contests)
	contestHub := observer.NewContestsHub()
	server := httptest.NewServer(observer.NewObserver(hub, contestHub, middleware.AuthMiddleware(auth, middleware.ResponseErrorHandler(slog.Default())), auth, loader, s.contestsRepo).Handler())
	defer server.Close()
	team, err := s.teamsRepo.CreateTeam(s.ctx, &organizationsmodel.CreateTeamInput{OrganizationID: org.ID, Name: "WS team", Slug: "ws-" + suffix, Privacy: organizationsmodel.TeamPrivacyClosed})
	s.Require().NoError(err)
	s.Require().NoError(s.contestsRepo.CreateContestTeam(s.ctx, contestID, team.ID, accessmodel.ContestModerator))
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
				s.Require().NoError(s.organizationsRepo.AddMember(s.ctx, org.ID, user.Id, accessmodel.OrganizationAdmin))
			case "direct moderator":
				s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: user.Id, Role: accessmodel.ContestModerator}))
			case "participant":
				s.Require().NoError(s.contestsRepo.CreateContestMember(s.ctx, &contestsmodel.CreateContestMemberParams{ContestId: contestID, UserId: user.Id, Role: accessmodel.ContestParticipant}))
			case "team moderator":
				s.Require().NoError(s.teamsRepo.AddTeamMember(s.ctx, team.ID, user.Id, accessmodel.TeamMember))
			}
			sessionID := uuid.New()
			s.Require().NoError(authRepo.CreateSession(s.ctx, sessionID, user.Id, time.Now().Add(40*time.Minute)))
			headers := http.Header{"Cookie": []string{"session_id=" + sessionID.String()}}
			conn, resp, dialErr := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/submissions?contestId="+contestID.String(), headers)
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
			event := submissionsmodel.SubmissionCompletedEvent{SubmissionEventMeta: submissionsmodel.SubmissionEventMeta{ContestId: &contestID, UserId: &owner.Id, CreatedAt: time.Now()}, Id: uuid.New(), State: 200, Score: 100}
			payload, marshalErr := json.Marshal(event)
			s.Require().NoError(marshalErr)
			s.Require().NoError(hub.Handle(s.ctx, submissionsmodel.SubmissionEventCompleted, payload))
			s.Require().NoError(conn.SetReadDeadline(time.Now().Add(3 * time.Second)))
			var msg struct {
				Payload map[string]any `json:"payload"`
			}
			s.Require().NoError(conn.ReadJSON(&msg))
			if source == "participant" {
				// Monitor access alone cannot reveal moderator-only submission events.
				s.Require().Contains(msg.Payload, "scoreboard")
				s.Require().NotContains(msg.Payload, "id")
				s.Require().NotContains(msg.Payload, "state")
			} else {
				s.Require().InDelta(200, msg.Payload["state"], 0)
			}
			granted, accessErr := access.Contest(accessmodel.WithUser(s.ctx, user), loader, contestID, accessmodel.ContestView)
			s.Require().NoError(accessErr)
			if granted.Can(accessmodel.ContestViewScoreboard) && source != "participant" {
				s.Require().NoError(conn.ReadJSON(&msg))
				s.Require().Equal(false, msg.Payload["unfrozen"])
				if granted.Can(accessmodel.ContestManage) {
					s.Require().NoError(conn.ReadJSON(&msg))
					s.Require().Equal(true, msg.Payload["unfrozen"])
				}
			}
			if source == "direct moderator" {
				s.Require().NoError(s.contestsRepo.UpdateContestMember(s.ctx, contestID, user.Id, accessmodel.ContestParticipant))
				_, _, closed := conn.ReadMessage()
				s.Require().True(websocket.IsCloseError(closed, websocket.CloseTryAgainLater), "%v", closed)
				var response *http.Response
				conn, response, dialErr = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/submissions?contestId="+contestID.String(), headers)
				s.Require().NoError(dialErr)
				s.Require().NoError(response.Body.Close())
				defer conn.Close()
				s.Require().NoError(hub.Handle(s.ctx, submissionsmodel.SubmissionEventCompleted, payload))
				s.Require().NoError(conn.SetReadDeadline(time.Now().Add(3 * time.Second)))
				// Decoding into a fresh struct prevents absent fields from surviving a previous frame.
				var downgraded struct {
					Payload map[string]any `json:"payload"`
				}
				s.Require().NoError(conn.ReadJSON(&downgraded))
				s.Contains(downgraded.Payload, "scoreboard")
				s.NotContains(downgraded.Payload, "id")
				s.NotContains(downgraded.Payload, "state")
				s.Equal(false, downgraded.Payload["unfrozen"])
				// Lease checks must not keep an idle browser session alive.
				before, err := authRepo.GetSession(s.ctx, sessionID)
				s.Require().NoError(err)
				time.Sleep(300 * time.Millisecond)
				after, err := authRepo.GetSession(s.ctx, sessionID)
				s.Require().NoError(err)
				s.Equal(before.ExpiresAt, after.ExpiresAt)
			}
			s.Require().NoError(authRepo.DeleteSession(s.ctx, sessionID))
			s.Require().NoError(hub.Handle(s.ctx, submissionsmodel.SubmissionEventCompleted, payload))
			_, _, readErr := conn.ReadMessage()
			s.Require().True(websocket.IsCloseError(readErr, 1008), "%v", readErr)
		})
	}
}
