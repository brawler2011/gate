//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestUpdateContestDates() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("dates_owner_"+suffix, models.UserRoleUser)
	outsider := s.createUser("dates_outsider_"+suffix, models.UserRoleUser)
	org := s.createOrganization("contest-dates-org-"+suffix, "Contest dates", owner.Id)
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	newStart := start.Add(15 * time.Minute)
	newEnd := end.Add(time.Hour)
	states := []string{"omitted", "null", "value"}
	for _, startState := range states {
		for _, endState := range states {
			s.Run(startState+"_"+endState, func() {
				id := uuid.New()
				login := "dates-" + startState + "-" + endState
				err := s.contestsRepo.CreateContest(s.ctx, &models.CreateContestParams{
					ID: id, OrganizationID: org.ID, OwnerID: &owner.Id,
					Title: "Original title", Login: login, Description: "Keep description",
					Visibility: models.ContestVisibilityPublic,
					Settings:   map[string]interface{}{"enable_drafts": true},
					StartTime:  &start, EndTime: &end,
				})
				s.Require().NoError(err)
				body := map[string]interface{}{"title": "Updated title"}
				expectedStart, expectedEnd := &start, &end
				if startState == "null" {
					body["start_time"] = nil
					expectedStart = nil
				}
				if startState == "value" {
					body["start_time"] = newStart
					expectedStart = &newStart
				}
				if endState == "null" {
					body["end_time"] = nil
					expectedEnd = nil
				}
				if endState == "value" {
					body["end_time"] = newEnd
					expectedEnd = &newEnd
				}
				raw, err := json.Marshal(body)
				s.Require().NoError(err)
				path := "/organizations/" + org.Login + "/contests/" + login
				patch := func(userID uuid.UUID, body string) int {
					req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-Test-User-ID", userID.String())
					recorder := httptest.NewRecorder()
					s.handler.ServeHTTP(recorder, req)
					return recorder.Code
				}
				s.Equal(http.StatusOK, patch(owner.Id, string(raw)))
				// Refetch the settings data twice: cleared dates must stay unset on reload.
				for range 2 {
					response, err := s.client.GetContest(withTestUser(s.ctx, owner.Id), corev1.GetContestParams{OrgLogin: org.Login, ContestLogin: login})
					s.Require().NoError(err)
					for _, date := range []struct {
						actual   corev1.OptNilDateTime
						expected *time.Time
					}{
						{response.Contest.StartTime, expectedStart}, {response.Contest.EndTime, expectedEnd},
					} {
						value, set := date.actual.Get()
						if date.expected == nil {
							s.False(set)
						} else {
							s.True(set)
							s.True(value.Equal(*date.expected))
						}
					}
					s.Equal("Updated title", response.Contest.Title)
					s.Equal("Keep description", response.Contest.Description)
				}
				stored, err := s.contestsRepo.GetContest(s.ctx, id)
				s.Require().NoError(err)
				for _, date := range []struct{ actual, expected *time.Time }{{stored.StartTime, expectedStart}, {stored.EndTime, expectedEnd}} {
					if date.expected == nil {
						s.Nil(date.actual)
					} else {
						s.Require().NotNil(date.actual)
						s.True(date.actual.Equal(*date.expected))
					}
				}
				s.Equal(true, stored.Settings["enable_drafts"])
				s.Equal(&owner.Id, stored.OwnerID)
				s.Equal(models.ContestVisibilityPublic, stored.Visibility)
				s.Equal(http.StatusForbidden, patch(outsider.Id, `{"start_time":null,"end_time":null}`))
				s.Equal(http.StatusBadRequest, patch(owner.Id, `{"start_time":"2026-01-02T10:00:00Z","end_time":"2026-01-01T10:00:00Z"}`))
				afterRejected, err := s.contestsRepo.GetContest(s.ctx, id)
				s.Require().NoError(err)
				s.Equal(stored, afterRejected)
			})
		}
	}
}
