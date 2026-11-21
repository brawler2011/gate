//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestUpdateContestDates() {
	suffix := uuid.NewString()[:8]
	owner := s.createUser("dates_owner_"+suffix, accessmodel.UserRoleUser)
	outsider := s.createUser("dates_outsider_"+suffix, accessmodel.UserRoleUser)
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
				err := s.createContest(contestsmodel.CreateContestParams{
					ID: id, OrganizationID: org.ID, Title: "Original title", Login: login, Description: "Keep description",
					Visibility: contestsmodel.ContestVisibilityPublic,
					StartTime:  &start, EndTime: &end,
				}, owner.Id)
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
				// Cleared dates must stay unset across repeated reloads.
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
				s.True(stored.Settings.EnableDrafts)
				s.Equal(contestsmodel.ContestVisibilityPublic, stored.Visibility)
				s.Equal(http.StatusForbidden, patch(outsider.Id, `{"start_time":null,"end_time":null}`))
				s.Equal(http.StatusBadRequest, patch(owner.Id, `{"start_time":"2026-01-02T10:00:00Z","end_time":"2026-01-01T10:00:00Z"}`))
				afterRejected, err := s.contestsRepo.GetContest(s.ctx, id)
				s.Require().NoError(err)
				s.Equal(stored, afterRejected)
			})
		}
	}
}

func (s *IntegrationTestSuite) TestContestPatchSettingsPresence() {
	owner := s.createUser("settings_owner_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("settings-"+uuid.NewString()[:8], "Settings", owner.Id)
	for _, tc := range []struct {
		name, body   string
		status       int
		freeze       *int32
		drafts       bool
		description  string
		freezeStatus contestsmodel.FreezeStatus
		monitor      contestsmodel.ContestScope
	}{
		{"omitted", `{}`, http.StatusOK, new(int32(30)), true, "Original", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopeParticipant},
		{"null", `{"freeze_duration_minutes":null}`, http.StatusOK, nil, true, "Original", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopeParticipant},
		{"zero", `{"freeze_duration_minutes":0}`, http.StatusBadRequest, new(int32(30)), true, "Original", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopeParticipant},
		{"false", `{"enable_drafts":false}`, http.StatusOK, new(int32(30)), false, "Original", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopeParticipant},
		{"empty", `{"description":""}`, http.StatusOK, new(int32(30)), true, "", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopeParticipant},
		{"public_scope", `{"monitor_scope":"public"}`, http.StatusOK, new(int32(30)), true, "Original", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopePublic},
		{"unknown_scope", `{"monitor_scope":"owner"}`, http.StatusBadRequest, new(int32(30)), true, "Original", contestsmodel.FreezeStatusFrozen, contestsmodel.ScopeParticipant},
		{"explicit_auto", `{"freeze_status":"auto"}`, http.StatusOK, new(int32(30)), true, "Original", contestsmodel.FreezeStatusAuto, contestsmodel.ScopeParticipant},
	} {
		s.Run(tc.name, func() {
			id := uuid.New()
			login := "settings-" + tc.name
			s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{
				ID: id, OrganizationID: org.ID, Title: "Settings", Login: login,
				Description: "Original", Visibility: contestsmodel.ContestVisibilityPublic,
			}, owner.Id))
			frozen := contestsmodel.FreezeStatusFrozen
			s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: id, Settings: contestsmodel.ContestSettingsPatch{
				FreezeStatus: &frozen, FreezeDurationMinutes: new(int32(30)), FreezeDurationSet: true,
			}}))
			req := httptest.NewRequest(http.MethodPatch, "/organizations/"+org.Login+"/contests/"+login, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-User-ID", owner.Id.String())
			recorder := httptest.NewRecorder()
			s.handler.ServeHTTP(recorder, req)
			s.Equal(tc.status, recorder.Code, recorder.Body.String())
			got, err := s.contestsRepo.GetContest(s.ctx, id)
			s.Require().NoError(err)
			s.Equal(tc.freeze, got.Settings.FreezeDurationMinutes)
			s.Equal(tc.drafts, got.Settings.EnableDrafts)
			s.True(got.Settings.EnableUpsolving)
			s.Equal(tc.description, got.Description)
			s.Equal(tc.freezeStatus, got.Settings.FreezeStatus)
			s.Equal(tc.freezeStatus == contestsmodel.FreezeStatusFrozen, got.Settings.FreezeStartedAt != nil, "manual freeze records its start")
			s.Equal(tc.monitor, got.Settings.MonitorScope)
		})
	}
}

func (s *IntegrationTestSuite) TestContestPatchRejectsNullFreezeStatus() {
	owner := s.createUser("freeze_owner_"+uuid.NewString()[:8], accessmodel.UserRoleUser)
	org := s.createOrganization("freeze-"+uuid.NewString()[:8], "Freeze status", owner.Id)
	id := uuid.New()
	login := "null-freeze-status"
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{
		ID: id, OrganizationID: org.ID, Title: "Freeze status", Login: login,
		Visibility: contestsmodel.ContestVisibilityPublic,
	}, owner.Id))
	frozen := contestsmodel.FreezeStatusFrozen
	s.Require().NoError(s.contestsRepo.UpdateContest(s.ctx, contestsmodel.ContestUpdateInput{ID: id, Settings: contestsmodel.ContestSettingsPatch{FreezeStatus: &frozen}}))

	req := httptest.NewRequest(http.MethodPatch, "/organizations/"+org.Login+"/contests/"+login, strings.NewReader(`{"freeze_status":null}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", owner.Id.String())
	recorder := httptest.NewRecorder()
	s.handler.ServeHTTP(recorder, req)
	s.Require().Equal(http.StatusBadRequest, recorder.Code, recorder.Body.String())

	got, err := s.contestsRepo.GetContest(s.ctx, id)
	s.Require().NoError(err)
	s.Equal(contestsmodel.FreezeStatusFrozen, got.Settings.FreezeStatus)
}
