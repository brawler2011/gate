//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problems "github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
)

func (s *IntegrationTestSuite) TestContestBookletIsBuiltAheadAndNeverServedStale() {
	owner := s.createUser("booklet_owner", accessmodel.UserRoleAdmin)
	org := s.createOrganization("booklet-org", "Booklet Organization", owner.Id)
	ctx := withTestUser(s.ctx, owner.Id)
	created, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{Title: "Booklet", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	packageID := s.buildPackage(owner.Id, created.ID)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Visibility: contestsmodel.ContestVisibilityPrivate, Title: "Booklet contest", Login: "booklet-contest"}, owner.Id))

	states := func() map[string]corev1.ContestBookletState {
		list, err := s.client.ListContestBooklets(ctx, corev1.ListContestBookletsParams{ContestID: contestID})
		s.Require().NoError(err)
		result := map[string]corev1.ContestBookletState{}
		for _, b := range list.Booklets {
			result[b.Language] = b.State
		}
		return result
	}
	allIn := func(state corev1.ContestBookletState) bool {
		st := states()
		for _, v := range st {
			if v != state {
				return false
			}
		}
		return len(st) > 0
	}
	download := func() ([]byte, int) {
		resp, err := s.client.DownloadContestStatementsPdf(ctx, corev1.DownloadContestStatementsPdfParams{ContestID: contestID, Lang: "ru"})
		if err != nil {
			return nil, s.getStatusCode(err)
		}
		data, err := io.ReadAll(resp.Data)
		s.Require().NoError(err)
		return data, http.StatusOK
	}

	// Adding a problem queues a booklet per statement language in the same transaction.
	s.Require().NoError(s.client.CreateContestProblem(ctx, corev1.CreateContestProblemParams{ContestID: contestID, ProblemID: created.ID, PackageID: corev1.NewOptUUID(packageID)}))
	s.Require().True(allIn(corev1.ContestBookletStateBuilding))
	s.Contains(states(), "ru")
	_, status := download()
	s.Equal(http.StatusConflict, status)

	worker := &contests.BookletWorker{Queue: s.booklets, Statements: &problems.PackageContent{Repo: s.problemDrafts, Store: s.objectStore, Bucket: s.bucket},
		Typeset: dbfixtures.Typesetter(s.T()), Store: s.objectStore, Bucket: s.bucket, Lease: 2 * time.Minute, MaxAttempts: 3, Poll: 100 * time.Millisecond,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	runWorker := func() {
		runCtx, stop := context.WithCancel(s.ctx)
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			_ = worker.Run(runCtx)
		}()
		s.Eventually(func() bool { return allIn(corev1.ContestBookletStateReady) }, 3*time.Minute, 200*time.Millisecond)
		stop()
		<-stopped
	}
	runWorker()

	// Object GC keeps the stored booklet.
	for {
		deleted, err := s.problemDrafts.DeleteUnreferencedObjects(s.ctx)
		s.Require().NoError(err)
		if len(deleted) == 0 {
			break
		}
	}
	pdf, status := download()
	s.Require().Equal(http.StatusOK, status)
	s.True(bytes.HasPrefix(pdf, []byte("%PDF")))

	// The organization's time zone moves the printed start date, so it re-keys the booklet and the old one is not served.
	start := time.Date(2026, 11, 24, 22, 0, 0, 0, time.UTC)
	s.Require().NoError(s.client.UpdateContest(ctx, &corev1.UpdateContestRequestModel{StartTime: corev1.NewOptNilDateTime(start)}, corev1.UpdateContestParams{ContestID: contestID}))
	runWorker()
	s.Require().NoError(s.client.UpdateOrganization(ctx, &corev1.UpdateOrganizationRequestModel{TimeZone: corev1.NewOptString("Europe/Moscow")}, corev1.UpdateOrganizationParams{Login: org.Login}))
	s.True(allIn(corev1.ContestBookletStateBuilding))
	_, status = download()
	s.Equal(http.StatusConflict, status)
	err = s.client.UpdateOrganization(ctx, &corev1.UpdateOrganizationRequestModel{TimeZone: corev1.NewOptString("Mars/Olympus")}, corev1.UpdateOrganizationParams{Login: org.Login})
	s.Equal(http.StatusBadRequest, s.getStatusCode(err))

	// A claimed build whose contest changes underneath cannot publish.
	job, err := s.booklets.ClaimBooklet(s.ctx, time.Minute)
	s.Require().NoError(err)
	s.Require().NotNil(job)
	s.Require().NoError(s.client.UpdateContest(ctx, &corev1.UpdateContestRequestModel{Place: corev1.NewOptString("Somewhere else")}, corev1.UpdateContestParams{ContestID: contestID}))
	err = s.booklets.CompleteBooklet(s.ctx, *job, problemsmodel.NewStoredObject(job.OrganizationID, problemsmodel.UploadSpec{Purpose: "booklet", SHA256: sha256Hex(pdf), Size: int64(len(pdf))}))
	s.ErrorIs(err, pkg.ErrConflict)

	s.Require().NoError(s.client.DeleteContestProblem(ctx, corev1.DeleteContestProblemParams{ContestID: contestID, ProblemID: created.ID}))
	s.Empty(states(), "a contest without problems has no booklets")
}
