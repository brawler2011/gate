//go:build integration

package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTeamOrganizationNameTrigramMigration(t *testing.T) {
	pool := getSharedTestPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback(ctx)) }()

	migration, err := os.ReadFile("../../migrations/20261107120000_team_organization_name_trgm.sql")
	require.NoError(t, err)
	up, down, ok := strings.Cut(string(migration), "-- +goose Down")
	require.True(t, ok)
	up = strings.TrimPrefix(up, "-- +goose Up")
	indexes := map[string]string{
		"teams_name_trgm_idx":         "teams",
		"organizations_name_trgm_idx": "organizations",
	}
	checkIndexes := func() {
		for index, table := range indexes {
			var definition string
			err := tx.QueryRow(ctx, "SELECT pg_get_indexdef(to_regclass($1))", index).Scan(&definition)
			require.NoError(t, err)
			require.Contains(t, definition, "ON public."+table+" USING gin (name gin_trgm_ops)")
		}
	}
	checkIndexes()
	_, err = tx.Exec(ctx, down)
	require.NoError(t, err)
	for index := range indexes {
		var absent bool
		require.NoError(t, tx.QueryRow(ctx, "SELECT to_regclass($1) IS NULL", index).Scan(&absent))
		require.True(t, absent, index)
	}
	_, err = tx.Exec(ctx, up)
	require.NoError(t, err)
	checkIndexes()
}
