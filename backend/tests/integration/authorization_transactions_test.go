//go:build integration

package integration

import (
	"context"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/internal/submissions"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type optimisticSubmissionPermissions struct{ submissions.Permissions }

func (optimisticSubmissionPermissions) HasContestPermission(context.Context, uuid.UUID, uuid.UUID, accessmodel.ContestAction) (bool, error) {
	return true, nil
}

func (s *IntegrationTestSuite) TestCommandsRecheckTransactionBoundPermissions() {
	sub, contest, _, user := s.newJobSubmission()
	_, err := s.dbPool.Exec(s.ctx, `UPDATE organization_members SET role='member' WHERE user_id=$1 AND organization_id=(SELECT organization_id FROM contests WHERE id=$2)`, user, contest)
	s.Require().NoError(err)
	ctx := accessmodel.WithUser(s.ctx, accessmodel.User{Id: user, Role: accessmodel.UserRoleUser})
	repo := pgstore.NewSubmissionsRepo(s.dbPool)
	before, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	uc := submissions.NewSubmissionsUseCase(repo, nil, nil, pgstore.NewSubmissionTransactions(s.dbPool), optimisticSubmissionPermissions{})
	s.Require().ErrorIs(uc.BlockSubmission(ctx, contest, sub, nil), pkg.NoPermission, "a successful earlier permission check cannot authorize the transaction")
	after, err := repo.GetSubmission(s.ctx, sub)
	s.Require().NoError(err)
	s.Equal(before.State, after.State)
	s.Equal(before.Revision, after.Revision)
	s.Equal(before.Generation, after.Generation)
	command := contests.NewContestsUseCase(s.contestsRepo, s.organizationsRepo, pgstore.NewContestCommandTransactions(s.dbPool), nil)
	title := "Denied update"
	s.Require().ErrorIs(command.UpdateContest(ctx, contestsmodel.ContestUpdateInput{ID: contest, Title: &title}), pkg.NoPermission)
	saved, err := s.contestsRepo.GetContest(s.ctx, contest)
	s.Require().NoError(err)
	s.NotEqual(title, saved.Title)
}

func (s *IntegrationTestSuite) TestAuthorizationHoldsMembershipUntilCommit() {
	_, contest, _, user := s.newJobSubmission()
	checked := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	provider := pgstore.NewContestCommandTransactions(s.dbPool)
	go func() {
		finished <- provider.Within(s.ctx, func(tx contests.CommandTxStore) error {
			if err := tx.LockContest(s.ctx, contest); err != nil {
				return err
			}
			allowed, err := tx.HasContestPermission(s.ctx, contest, user, accessmodel.ActionManageContest)
			if err != nil {
				return err
			}
			if !allowed {
				return pkg.NoPermission
			}
			close(checked)
			<-release
			return nil
		})
	}()
	select {
	case <-checked:
	case err := <-finished:
		s.Require().NoError(err)
		s.FailNow("authorization did not run")
	case <-time.After(5 * time.Second):
		close(release)
		s.FailNow("authorization timed out")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 200*time.Millisecond)
	_, err := s.dbPool.Exec(ctx, `DELETE FROM organization_members WHERE user_id=$1 AND organization_id=(SELECT organization_id FROM contests WHERE id=$2)`, user, contest)
	cancel()
	close(release)
	s.Require().NoError(<-finished)
	s.Require().Error(err, "revocation must wait until the authorized command commits")
}
