package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// These stores keep updates so HTTP reads exercise the response mapping too.
type patchContestsModule struct {
	ContestsModule
	contest    contestsmodel.Contest
	lastUpdate contestsmodel.ContestUpdateInput
}

func (m *patchContestsModule) GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error) {
	return m.contest, nil
}

func (m *patchContestsModule) UpdateContest(_ context.Context, update contestsmodel.ContestUpdateInput) error {
	m.lastUpdate = update
	if update.Settings != nil {
		for key, value := range *update.Settings {
			m.contest.Settings[key] = value
		}
	}
	return nil
}

func (m *patchContestsModule) GetContestDetails(context.Context, string, string) (*contests.ContestDetails, error) {
	return &contests.ContestDetails{Contest: m.contest}, nil
}

func TestPatchContestAllowClarifications(t *testing.T) {
	for _, initial := range []bool{true, false} {
		for _, tc := range []struct {
			name           string
			body           string
			want           bool
			settingsUpdate bool
		}{
			{"false", `{"allow_clarifications":false}`, false, true},
			{"true", `{"allow_clarifications":true}`, true, true},
			{"omitted", `{"title":"Updated"}`, initial, false},
		} {
			t.Run(fmt.Sprintf("initial=%t/%s", initial, tc.name), func(t *testing.T) {
				store := &patchContestsModule{contest: contestsmodel.Contest{
					ID: uuid.New(), Login: "round", OrganizationLogin: "org", Visibility: contestsmodel.ContestVisibilityPublic,
					Settings: map[string]interface{}{"allow_clarifications": initial, "hide_statements": true},
				}}
				server := NewCoreServer(nil, nil, store, nil, nil, nil, nil, nil, false)
				handler, err := corev1.NewServer(server, middleware.NewSecurityHandler())
				require.NoError(t, err)
				path := "/organizations/org/contests/round"
				req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				var flagSet bool
				if store.lastUpdate.Settings != nil {
					_, flagSet = (*store.lastUpdate.Settings)["allow_clarifications"]
				}
				require.Equal(t, tc.settingsUpdate, flagSet)
				require.Equal(t, tc.want, store.contest.Settings["allow_clarifications"])
				require.Equal(t, true, store.contest.Settings["hide_statements"])

				rec = httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), fmt.Sprintf(`"allow_clarifications":%t`, tc.want))
			})
		}
	}
}

type patchProblemsModule struct {
	ProblemsModule
	problem problemsmodel.Problem
	updates int
}

func (m *patchProblemsModule) UpdateProblem(_ context.Context, _ uuid.UUID, update *problemsmodel.ProblemUpdate) error {
	m.updates++
	if update.Title != nil {
		m.problem.Title = *update.Title
	}
	if update.Visibility != nil {
		m.problem.Visibility = *update.Visibility
	}
	if update.IsTemplate != nil {
		m.problem.IsTemplate = *update.IsTemplate
	}
	return nil
}

func (m *patchProblemsModule) GetProblemDetails(context.Context, uuid.UUID) (*problemsmodel.ProblemDetails, error) {
	return &problemsmodel.ProblemDetails{Problem: m.problem}, nil
}

func TestPatchProblemMetadata(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		title      string
		visibility string
		template   bool
	}{
		{"all", `{"title":"Updated","visibility":"public","is_template":false}`, "Updated", "public", false},
		{"omitted", `{}`, "Original", "private", true},
		{"title only", `{"title":"Updated"}`, "Updated", "private", true},
		{"false", `{"is_template":false}`, "Original", "private", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &patchProblemsModule{problem: problemsmodel.Problem{ID: uuid.New(), Title: "Original", Visibility: "private", IsTemplate: true}}
			server := NewCoreServer(nil, nil, nil, store, nil, nil, nil, nil, false)
			handler, err := corev1.NewServer(server, middleware.NewSecurityHandler())
			require.NoError(t, err)
			path := "/problems/" + store.problem.ID.String()
			req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, 1, store.updates)
			rec = httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), `"title":"`+tc.title+`"`)
			require.Contains(t, rec.Body.String(), `"visibility":"`+tc.visibility+`"`)
			require.Contains(t, rec.Body.String(), fmt.Sprintf(`"is_template":%t`, tc.template))
		})
	}
}

func TestPatchProblemRejectsObsoleteFields(t *testing.T) {
	for _, field := range []string{"legend", "input_format", "output_format", "notes", "scoring", "memory_limit", "time_limit"} {
		for _, mixed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mixed=%t", field, mixed), func(t *testing.T) {
				value := `"obsolete"`
				if field == "memory_limit" || field == "time_limit" {
					value = "256"
				}
				body := fmt.Sprintf(`{"%s":%s`, field, value)
				if mixed {
					body += `,"title":"Updated"`
				}
				body += "}"
				store := &patchProblemsModule{problem: problemsmodel.Problem{ID: uuid.New(), Title: "Original"}}
				server := NewCoreServer(nil, nil, nil, store, nil, nil, nil, nil, false)
				handler, err := corev1.NewServer(server, middleware.NewSecurityHandler())
				require.NoError(t, err)
				req := httptest.NewRequest(http.MethodPatch, "/problems/"+store.problem.ID.String(), strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				require.Contains(t, rec.Body.String(), field)
				require.Zero(t, store.updates)
				require.Equal(t, "Original", store.problem.Title)
			})
		}
	}
}
