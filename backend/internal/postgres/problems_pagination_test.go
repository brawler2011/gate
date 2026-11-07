package postgres

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type problemsQueryCaptureDB struct {
	sqlc.DBTX
	query string
	args  []any
}

func (db *problemsQueryCaptureDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return countRow(5)
}

func (db *problemsQueryCaptureDB) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	db.query = query
	db.args = args
	return nil, context.Canceled
}

type countRow int64

func (r countRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*int64); ok {
			*p = int64(r)
		}
	}
	return nil
}

func TestProblemsRepo_ListProblemsDescendingParameter(t *testing.T) {
	trueVal := true
	falseVal := false
	orgID := uuid.New()

	for _, tc := range []struct {
		name           string
		descending     *bool
		wantDescending bool
	}{
		{name: "default when nil", descending: nil, wantDescending: true},
		{name: "explicit true", descending: &trueVal, wantDescending: true},
		{name: "explicit false", descending: &falseVal, wantDescending: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &problemsQueryCaptureDB{}
			repo := &ProblemsRepo{queries: sqlc.New(db)}
			_, err := repo.ListProblems(context.Background(), &problemsmodel.ProblemsFilter{
				OrganizationID: &orgID,
				Page:           1,
				PageSize:       10,
				Descending:     tc.descending,
			})
			require.ErrorIs(t, err, context.Canceled)
			require.Len(t, db.args, 11)
			require.Equal(t, tc.wantDescending, db.args[0])
			require.Equal(t, int64(0), db.args[9])
			require.Equal(t, int32(10), db.args[10])
		})
	}
}
