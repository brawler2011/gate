package contests

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	"github.com/brawler2011/gate/backend/internal/access/accesstest"
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
func (s *detailStore) GetPendingContestJoinRequest(context.Context, uuid.UUID, uuid.UUID) (*contestsmodel.ContestJoinRequest, error) {
	return nil, nil
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

type detailStatements struct {
	calls     int
	fileCalls int
	fileHash  string
	pdfCalls  int
}

func (s *detailStatements) ReadStatement(context.Context, uuid.UUID, uuid.UUID, string) problemsmodel.StatementContent {
	s.calls++
	return problemsmodel.StatementContent{Statement: &problemsmodel.Statement{Legend: "Visible statement", Tutorial: "Tutorial"}, Samples: []problemsmodel.Sample{{Input: "1", Output: "2"}}, Assets: map[string]string{"assets/pic.png": "abc"}}
}

func (s *detailStatements) OpenImage(_ context.Context, _ uuid.UUID, hash string) (io.ReadCloser, string, error) {
	s.fileCalls++
	s.fileHash = hash
	return io.NopCloser(strings.NewReader("png")), "image/png", nil
}

func (s *detailStatements) OpenPDF(context.Context, uuid.UUID, uuid.UUID, string) (io.ReadCloser, error) {
	s.pdfCalls++
	return io.NopCloser(strings.NewReader("pdf")), nil
}

func (s *detailStatements) OpenFiles(context.Context, uuid.UUID) (io.ReadCloser, error) {
	return nil, pkg.ErrNotFound
}

type detailBooklets struct{ downloads int }

func (b *detailBooklets) ListBooklets(_ context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error) {
	booklet := contestsmodel.Booklet{BookletInputs: contestsmodel.BookletInputs{ContestID: contestID, Language: "ru"}}
	booklet.Stored = &contestsmodel.StoredBooklet{Key: booklet.Key(), State: contestsmodel.BookletSucceeded, ObjectKey: "booklet"}
	return []contestsmodel.Booklet{booklet}, nil
}

func (b *detailBooklets) DownloadFile(context.Context, string, string, *string) (io.ReadCloser, string, error) {
	b.downloads++
	return io.NopCloser(strings.NewReader("pdf")), "application/pdf", nil
}

func detailAccess(contest contestsmodel.Contest, actor uuid.UUID, role accessmodel.ContestRole) *accesstest.Loader {
	l := &accesstest.Loader{Contests: map[uuid.UUID]access.ContestFacts{contest.ID: {Contest: contest, Now: time.Now()}}}
	l.Grant(contest.ID, actor, string(role))
	return l
}

func TestContestDetailsApplyStatementVisibility(t *testing.T) {
	for _, tc := range []struct {
		name                string
		hidden, beforeStart bool
		role                accessmodel.ContestRole
		problem             error
		statement           bool
		booklet             error
	}{
		{name: "participant", role: accessmodel.ContestParticipant, statement: true},
		{name: "hidden participant", hidden: true, role: accessmodel.ContestParticipant, booklet: pkg.ErrStatementsHidden},
		{name: "hidden moderator", hidden: true, role: accessmodel.ContestModerator, statement: true},
		{name: "before start", beforeStart: true, role: accessmodel.ContestParticipant, problem: pkg.ErrForbidden, booklet: pkg.ErrContestNotStarted},
		{name: "outsider", problem: pkg.ErrNotFound, booklet: pkg.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now().Add(-time.Hour)
			if tc.beforeStart {
				start = time.Now().Add(time.Hour)
			}
			contest := contestsmodel.Contest{ID: uuid.New(), Visibility: contestsmodel.ContestVisibilityPrivate, StartTime: &start, Settings: contestsmodel.ContestSettings{HideStatements: tc.hidden}}
			store := &detailStore{contest: contest, link: contestsmodel.ContestProblem{ProblemID: uuid.New(), Ordinal: 1}}
			statements, stored := &detailStatements{}, &detailBooklets{}
			actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			loader := detailAccess(contest, actor.Id, tc.role)
			reader := NewDetailsReader(store, statements, loader)
			booklets := &Booklets{Store: stored, Files: stored, Access: loader}
			ctx := accessmodel.WithUser(context.Background(), actor)

			result, err := reader.GetContestProblemDetails(ctx, contest.ID, store.link.ProblemID)
			require.ErrorIs(t, err, tc.problem)
			if tc.problem == nil {
				require.Equal(t, tc.statement, result.Statement != nil)
				require.Equal(t, tc.statement, len(result.Assets) == 1)
			}
			require.Equal(t, tc.statement, statements.calls == 1)

			file, contentType, err := reader.OpenProblemAsset(ctx, contest.ID, store.link.ProblemID, "abc")
			if tc.statement {
				require.NoError(t, err)
				require.NoError(t, file.Close())
				require.Equal(t, "image/png", contentType)
				require.Equal(t, "abc", statements.fileHash)
			} else {
				require.Error(t, err)
				require.Zero(t, statements.fileCalls)
			}

			_, err = reader.OpenProblemPDF(ctx, contest.ID, store.link.ProblemID, "")
			require.Equal(t, tc.statement, err == nil)
			require.Equal(t, tc.statement, statements.pdfCalls == 1)

			_, err = booklets.Open(ctx, contest.ID, "ru")
			require.ErrorIs(t, err, tc.booklet)
			require.Equal(t, tc.booklet == nil, stored.downloads == 1)
		})
	}
}

func TestContestDetailsShowTutorialsToTheirAudience(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ended    bool
		role     accessmodel.ContestRole
		tutorial bool
	}{
		{"participant during", false, accessmodel.ContestParticipant, false},
		{"moderator during", false, accessmodel.ContestModerator, true},
		{"participant after", true, accessmodel.ContestParticipant, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, end := time.Now().Add(-2*time.Hour), time.Now().Add(time.Hour)
			if tc.ended {
				end = time.Now().Add(-time.Hour)
			}
			contest := contestsmodel.Contest{ID: uuid.New(), Visibility: contestsmodel.ContestVisibilityPrivate, StartTime: &start, EndTime: &end,
				Settings: contestsmodel.ContestSettings{Tutorials: contestsmodel.Phased[contestsmodel.ContestScope]{During: contestsmodel.ScopeModerator, After: contestsmodel.ScopeParticipant}}}
			store := &detailStore{contest: contest, link: contestsmodel.ContestProblem{ProblemID: uuid.New(), Ordinal: 1}}
			actor := accessmodel.User{Id: uuid.New(), Role: accessmodel.UserRoleUser}
			reader := NewDetailsReader(store, &detailStatements{}, detailAccess(contest, actor.Id, tc.role))
			result, err := reader.GetContestProblemDetails(accessmodel.WithUser(context.Background(), actor), contest.ID, store.link.ProblemID)
			require.NoError(t, err)
			require.Equal(t, "Visible statement", result.Statement.Legend)
			require.Equal(t, tc.tutorial, result.Statement.Tutorial != "")
		})
	}
}

func TestContestListUsesFlatProjection(t *testing.T) {
	store := &detailStore{contest: contestsmodel.Contest{ID: uuid.New(), Visibility: contestsmodel.ContestVisibilityPublic}, link: contestsmodel.ContestProblem{Title: "Projected", TimeLimitMs: 1234, MemoryLimitMb: 512}}
	reader := NewDetailsReader(store, nil, detailAccess(store.contest, uuid.Nil, ""))
	result, err := reader.GetContestDetails(context.Background(), "org", "contest")
	require.NoError(t, err)
	require.Len(t, result.Problems, 1)
	require.Equal(t, 1234, result.Problems[0].TimeLimitMs)
	require.Equal(t, 512, result.Problems[0].MemoryLimitMb)
	require.Zero(t, store.catalogCalls, "list metadata must come from the projected SQL rows")
}
