//go:build integration

package integration

import (
	"errors"
	"sync"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	"github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	pgstore "github.com/brawler2011/gate/backend/internal/postgres"
	"github.com/brawler2011/gate/backend/pkg"
)

func (s *IntegrationTestSuite) TestDraftLimitSerializesConcurrentRequests() {
	_, contest, _, user := s.newJobSubmission()
	repo := pgstore.NewDraftsRepo(s.dbPool)
	for range 49 {
		_, err := repo.CreateDraft(s.ctx, &contestsmodel.ContestDraftCreation{ContestID: contest, UserID: user, Code: "seed"})
		s.Require().NoError(err)
	}
	uc := contests.NewDraftsUseCase(pgstore.NewDraftTransactions(s.dbPool))
	ctx := accessmodel.WithUser(s.ctx, accessmodel.User{Id: user, Role: accessmodel.UserRoleUser})
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			_, err := uc.CreateDraft(ctx, &contestsmodel.ContestDraftCreation{ContestID: contest, UserID: user, Code: "concurrent draft"})
			results <- err
		})
	}
	wg.Wait()
	close(results)
	success, limited := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, pkg.ErrBadInput) {
			limited++
		} else {
			s.Require().NoError(err)
		}
	}
	s.Equal(1, success)
	s.Equal(1, limited)
	count, err := repo.GetDraftsCount(s.ctx, contest, user)
	s.Require().NoError(err)
	s.EqualValues(50, count)
}
