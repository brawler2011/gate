package contests

import (
	"context"
	"testing"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type detailStore struct {
	contest      contestsmodel.Contest
	link         contestsmodel.ContestProblem
	catalogCalls int
}

func (s *detailStore) GetContestByOrgLoginAndContestLogin(context.Context, string, string) (contestsmodel.Contest, error) {
	return s.contest, nil
}
func (s *detailStore) GetContestProblem(context.Context, contestsmodel.ContestProblemGet) (contestsmodel.ContestProblem, error) {
	return s.link, nil
}
func (s *detailStore) GetContestProblems(context.Context, uuid.UUID) ([]contestsmodel.ContestProblem, error) {
	return []contestsmodel.ContestProblem{s.link}, nil
}
func (s *detailStore) GetProblemById(context.Context, uuid.UUID) (problemsmodel.Problem, error) {
	s.catalogCalls++
	return problemsmodel.Problem{Title: "Problem", TimeLimitMs: 1000}, nil
}

type detailPermissions struct{ view, problems, manage bool }

func (p detailPermissions) HasContestPermission(_ context.Context, _, _ uuid.UUID, action accessmodel.ContestAction) (bool, error) {
	switch action {
	case accessmodel.ActionManageContest:
		return p.manage, nil
	case accessmodel.ActionGetContestProblem:
		return p.problems, nil
	default:
		return p.view, nil
	}
}

type detailStatements struct{ calls int }

func (s *detailStatements) ReadStatement(context.Context, uuid.UUID, uuid.UUID, string) (*problemsmodel.Statement, []problemsmodel.Sample) {
	s.calls++
	return &problemsmodel.Statement{Legend: "Visible statement"}, []problemsmodel.Sample{{Input: "1", Output: "2"}}
}

type detailCompiler struct{ calls int }

func (c *detailCompiler) CompilePDF(context.Context, string) ([]byte, error) {
	c.calls++
	return []byte("pdf"), nil
}

func TestContestDetailsApplyStatementVisibility(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		hidden, manage, allowed, beforeStart bool
		statement, pdf                       bool
	}{
		{name: "participant", allowed: true, statement: true, pdf: true},
		{name: "hidden participant", hidden: true, allowed: true},
		{name: "hidden manager", hidden: true, manage: true, allowed: true, statement: true, pdf: true},
		{name: "before start", allowed: true, beforeStart: true, statement: true},
		{name: "outsider"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now().Add(-time.Hour)
			if tc.beforeStart {
				start = time.Now().Add(time.Hour)
			}
			store := &detailStore{contest: contestsmodel.Contest{ID: uuid.New(), StartTime: &start, Settings: map[string]interface{}{"hide_statements": tc.hidden}}, link: contestsmodel.ContestProblem{ProblemID: uuid.New(), Ordinal: 1}}
			statements, compiler := &detailStatements{}, &detailCompiler{}
			reader := NewDetailsReader(store, statements, detailPermissions{view: true, problems: tc.allowed, manage: tc.manage}, compiler, nil)
			ctx := accessmodel.WithUser(context.Background(), accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser})
			result, err := reader.GetContestProblemDetails(ctx, "org", "contest", store.link.ProblemID)
			if tc.allowed {
				require.NoError(t, err)
				require.Equal(t, tc.statement, result.Statement != nil)
			} else {
				require.ErrorIs(t, err, pkg.NoPermission)
			}
			if tc.statement {
				require.Equal(t, 1, statements.calls)
			} else {
				require.Zero(t, statements.calls)
			}
			_, err = reader.ReadBooklet(ctx, "org", "contest", "ru")
			if tc.pdf {
				require.NoError(t, err)
				require.Equal(t, 1, compiler.calls)
			} else {
				require.ErrorIs(t, err, pkg.NoPermission)
				require.Zero(t, compiler.calls)
			}
		})
	}
}

func TestContestListUsesFlatProjection(t *testing.T) {
	store := &detailStore{contest: contestsmodel.Contest{ID: uuid.New()}, link: contestsmodel.ContestProblem{Title: "Projected", TimeLimitMs: 1234, MemoryLimitMb: 512}}
	reader := NewDetailsReader(store, nil, detailPermissions{view: true, problems: true}, nil, nil)
	result, err := reader.GetContestDetails(context.Background(), "org", "contest")
	require.NoError(t, err)
	require.Len(t, result.Problems, 1)
	require.Equal(t, 1234, result.Problems[0].TimeLimitMs)
	require.Equal(t, 512, result.Problems[0].MemoryLimitMb)
	require.Zero(t, store.catalogCalls, "list metadata must come from the projected SQL rows")
}
