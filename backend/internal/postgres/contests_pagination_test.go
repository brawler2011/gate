package postgres

import (
	"context"
	"testing"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type publicContestsPaginationDB struct {
	sqlc.DBTX
	args []any
}

func (db *publicContestsPaginationDB) Query(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
	db.args = args
	return nil, context.Canceled
}

func TestPublicContestsPagination(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		page, size, wantOffset, wantSize int32
		invalid                          bool
	}{
		{name: "defaults", wantSize: 50},
		{name: "default page", size: 100, wantSize: 100},
		{name: "default size", page: 3, wantOffset: 100, wantSize: 50},
		{name: "minimum", page: 1, size: 1, wantSize: 1},
		{name: "maximum size", page: 2, size: 100, wantOffset: 100, wantSize: 100},
		{name: "negative page", page: -1, size: 50, invalid: true},
		{name: "negative size", page: 1, size: -1, invalid: true},
		{name: "oversized", page: 1, size: 101, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &publicContestsPaginationDB{}
			repo := &ContestsRepo{queries: sqlc.New(db)}
			_, err := repo.ListPublicContests(context.Background(), contestsmodel.PublicContestsFilter{
				Page: tc.page, PageSize: tc.size, Search: "search", SortBy: "title", SortOrder: "asc",
			})
			if tc.invalid {
				require.ErrorIs(t, err, pkg.ErrBadInput)
				require.Nil(t, db.args)
				return
			}
			// Stop at the database boundary after capturing the normalized query parameters.
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, []any{"search", string(contestsmodel.ContestVisibilityPublic), "title", "asc", tc.wantOffset, tc.wantSize}, db.args)
		})
	}
}
