//go:build integration
// +build integration

package integration

import (
	"context"
	"errors"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestTransactionsWithinRollback() {
	owner := s.createUser("rollback_owner", accessmodel.UserRoleUser)
	org := s.createOrganization("rollback-lifecycle", "Rollback", owner.Id)
	provider := pgstore.NewProblemTransactions(s.dbPool)
	failure := errors.New("injected callback failure")
	for _, mode := range []string{"error", "panic", "canceled"} {
		s.Run(mode, func() {
			id := uuid.New()
			ctx, cancel := context.WithCancel(s.ctx)
			defer cancel()
			callback := func(tx problems.ProblemTxStore) error {
				err := tx.CreateProblem(ctx, &problemsmodel.CreateProblemParams{
					ID:             id,
					OrganizationID: org.ID,
					Title:          "Rollback",
					ShortName:      mode,
					Visibility:     problemsmodel.ProblemVisibilityPrivate,
				})
				if err != nil {
					return err
				}
				if mode == "panic" {
					panic(failure)
				}
				if mode == "canceled" {
					cancel()
				}
				return failure
			}
			if mode == "panic" {
				s.PanicsWithValue(failure, func() { _ = provider.Within(ctx, callback) })
			} else {
				err := provider.Within(ctx, callback)
				s.Same(failure, err, "preserve the original callback error")
			}
			checkCtx, stop := context.WithTimeout(s.ctx, time.Second)
			defer stop()
			var count int
			err := s.dbPool.QueryRow(checkCtx, `SELECT count(*) FROM problems WHERE id=$1`, id).Scan(&count)
			s.Require().NoError(err)
			s.Zero(count, "uncommitted rows must be rolled back")
			s.Zero(s.dbPool.Stat().AcquiredConns(), "rollback must release the connection")
		})
	}
}
