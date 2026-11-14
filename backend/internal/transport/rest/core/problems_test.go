package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProblemsModule struct {
	ProblemsUC
	lastFilter *problemsmodel.ProblemsFilter
}

func (m *mockProblemsModule) ListProblems(_ context.Context, filter *problemsmodel.ProblemsFilter) (*problemsmodel.ProblemsList, error) {
	m.lastFilter = filter
	return &problemsmodel.ProblemsList{
		Problems: []problemsmodel.Problem{},
	}, nil
}

func TestListProblems_DescendingFilterHonored(t *testing.T) {
	user := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
	ctx := accessmodel.WithUser(context.Background(), user)

	tests := []struct {
		name           string
		params         corev1.ListProblemsParams
		wantDescending bool
	}{
		{
			name:           "default descending when unset",
			params:         corev1.ListProblemsParams{},
			wantDescending: true,
		},
		{
			name: "explicit descending true",
			params: corev1.ListProblemsParams{
				Descending: corev1.NewOptBool(true),
			},
			wantDescending: true,
		},
		{
			name: "explicit descending false",
			params: corev1.ListProblemsParams{
				Descending: corev1.NewOptBool(false),
			},
			wantDescending: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &mockProblemsModule{}
			server := NewCoreServer(Services{Problems: stub}, false)

			resp, err := server.ListProblems(ctx, tt.params)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.NotNil(t, stub.lastFilter)
			require.NotNil(t, stub.lastFilter.Descending)
			assert.Equal(t, tt.wantDescending, *stub.lastFilter.Descending)
		})
	}
}

func TestListProblems_HTTPProbeDescending(t *testing.T) {
	stub := &mockProblemsModule{}
	server := NewCoreServer(Services{Problems: stub}, false)
	handler, err := corev1.NewServer(server, middleware.NewSecurityHandler())
	require.NoError(t, err)

	// 1. GET /problems without descending parameter -> defaults to descending = true
	req1 := httptest.NewRequest(http.MethodGet, "/problems", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)
	require.NotNil(t, stub.lastFilter)
	require.NotNil(t, stub.lastFilter.Descending)
	assert.True(t, *stub.lastFilter.Descending)

	// 2. GET /problems?descending=true -> descending = true
	req2 := httptest.NewRequest(http.MethodGet, "/problems?descending=true", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	require.NotNil(t, stub.lastFilter)
	require.NotNil(t, stub.lastFilter.Descending)
	assert.True(t, *stub.lastFilter.Descending)

	// 3. GET /problems?descending=false -> descending = false
	req3 := httptest.NewRequest(http.MethodGet, "/problems?descending=false", nil)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusOK, rec3.Code)
	require.NotNil(t, stub.lastFilter)
	require.NotNil(t, stub.lastFilter.Descending)
	assert.False(t, *stub.lastFilter.Descending)
}
