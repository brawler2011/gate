//go:build integration
// +build integration

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contests "github.com/brawler2011/gate/backend/internal/contests"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/google/uuid"
	ht "github.com/ogen-go/ogen/http"
)

// importAPlusB imports the gfmt a-plus-b package with the given files replaced.
func (s *IntegrationTestSuite) importAPlusB(userID, problemID uuid.UUID, replace map[string]string) {
	s.T().Helper()
	ctx := withTestUser(s.ctx, userID)
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	root := os.DirFS("../../pkg/testdata/gfmt/a-plus-b")
	s.Require().NoError(fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		if text, ok := replace[path]; ok {
			data = []byte(text)
		}
		f, err := w.Create(path)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}))
	s.Require().NoError(w.Close())
	draft, err := s.client.GetProblemDraft(ctx, corev1.GetProblemDraftParams{ID: problemID})
	s.Require().NoError(err)
	_, err = s.client.ImportProblem(ctx, &corev1.ImportProblemReq{Package: ht.MultipartFile{Name: "package.zip", File: bytes.NewReader(archive.Bytes()), Size: int64(archive.Len())}},
		corev1.ImportProblemParams{ID: problemID, ExpectedRevision: draft.Revision})
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) packageFiles(packageID uuid.UUID) map[string][]byte {
	s.T().Helper()
	descriptor, err := s.problemDrafts.GetPackageDescriptor(s.ctx, packageID)
	s.Require().NoError(err)
	r, _, err := s.objectStore.DownloadFile(s.ctx, s.bucket, descriptor.Object.Key, nil)
	s.Require().NoError(err)
	data, err := io.ReadAll(r)
	s.Require().NoError(errors.Join(err, r.Close()))
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	s.Require().NoError(err)
	files := map[string][]byte{}
	for _, f := range z.File {
		rc, err := f.Open()
		s.Require().NoError(err)
		files[f.Name], err = io.ReadAll(rc)
		s.Require().NoError(errors.Join(err, rc.Close()))
	}
	return files
}

func (s *IntegrationTestSuite) TestProblemBuildPrintsStatements() {
	owner := s.createUser("print_owner", accessmodel.UserRoleAdmin)
	org := s.createOrganization("print-org", "Print Organization", owner.Id)
	ctx := withTestUser(s.ctx, owner.Id)
	created, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{Title: "Print", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)

	s.importAPlusB(owner.Id, created.ID, map[string]string{"statements/en/legend.md": "Print $a+b$ \\undefinedmacro."})
	_, err = s.runBuild(owner.Id, created.ID)
	var failure *builder.Failure
	s.Require().ErrorAs(err, &failure)
	s.Equal("statements", failure.Stage)
	s.Equal("author", failure.Kind)
	s.Contains(failure.Message, "Undefined control sequence")

	s.importAPlusB(owner.Id, created.ID, nil)
	files := s.packageFiles(s.buildPackage(owner.Id, created.ID))
	for _, language := range []string{"en", "ru"} {
		s.True(bytes.HasPrefix(files["statements/"+language+"/statement.pdf"], []byte("%PDF")), language)
	}
}

func (s *IntegrationTestSuite) TestContestBookletIsBuiltAheadAndNeverServedStale() {
	owner := s.createUser("booklet_owner", accessmodel.UserRoleAdmin)
	org := s.createOrganization("booklet-org", "Booklet Organization", owner.Id)
	ctx := withTestUser(s.ctx, owner.Id)
	created, err := s.client.CreateProblem(ctx, corev1.CreateProblemParams{Title: "Booklet", OrganizationID: corev1.NewOptUUID(org.ID), TemplateID: "builtin:a-plus-b"})
	s.Require().NoError(err)
	s.importAPlusB(owner.Id, created.ID, nil)
	packageID := s.buildPackage(owner.Id, created.ID)
	contestID := uuid.New()
	s.Require().NoError(s.createContest(contestsmodel.CreateContestParams{ID: contestID, OrganizationID: org.ID, Visibility: contestsmodel.ContestVisibilityPrivate, Title: "Booklet contest", Login: "booklet-contest"}, owner.Id))

	// Adding a problem queues a booklet per statement language.
	s.Require().NoError(s.client.CreateContestProblem(ctx, corev1.CreateContestProblemParams{ContestID: contestID, ProblemID: created.ID, PackageID: corev1.NewOptUUID(packageID)}))
	states := func() map[string]corev1.ContestBookletState {
		list, err := s.client.ListContestBooklets(ctx, corev1.ListContestBookletsParams{ContestID: contestID})
		s.Require().NoError(err)
		result := map[string]corev1.ContestBookletState{}
		for _, b := range list.Booklets {
			result[b.Language] = b.State
		}
		return result
	}
	s.Equal(map[string]corev1.ContestBookletState{"en": corev1.ContestBookletStateBuilding, "ru": corev1.ContestBookletStateBuilding}, states())
	download := func() ([]byte, int) {
		resp, err := s.client.DownloadContestStatementsPdf(ctx, corev1.DownloadContestStatementsPdfParams{ContestID: contestID, Lang: "ru"})
		if err != nil {
			return nil, s.getStatusCode(err)
		}
		data, err := io.ReadAll(resp.Data)
		s.Require().NoError(err)
		return data, http.StatusOK
	}
	_, status := download()
	s.Equal(http.StatusConflict, status)

	client, _ := newGoJudge(s.T())
	worker := &contests.BookletWorker{Queue: s.booklets, Compiler: sandbox.NewSandbox(client, testLanguages(s.T())), Store: s.objectStore, Bucket: s.bucket,
		Limits: dbfixtures.BuildLimits(s.T()).Booklet, MaxAttempts: 3, Poll: 100 * time.Millisecond, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	runCtx, stop := context.WithCancel(s.ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_ = worker.Run(runCtx)
	}()
	s.Eventually(func() bool {
		st := states()
		return st["en"] == corev1.ContestBookletStateReady && st["ru"] == corev1.ContestBookletStateReady
	}, 3*time.Minute, 200*time.Millisecond)
	stop()
	<-stopped

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

	// A claimed build whose source changes underneath cannot publish, and the old PDF is not served meanwhile.
	rename := func(title string) {
		s.Require().NoError(s.client.UpdateContest(ctx, &corev1.UpdateContestRequestModel{Title: corev1.NewOptString(title)}, corev1.UpdateContestParams{ContestID: contestID}))
	}
	rename("Renamed contest")
	job, err := s.booklets.ClaimBooklet(s.ctx, time.Minute)
	s.Require().NoError(err)
	s.Require().NotNil(job)
	s.True(strings.Contains(job.Source, "Renamed contest"))
	rename("Renamed again")
	err = s.booklets.CompleteBooklet(s.ctx, *job, problemsmodel.NewStoredObject(job.OrganizationID, problemsmodel.UploadSpec{Purpose: "booklet", SHA256: sha256Hex(pdf), Size: int64(len(pdf))}))
	s.ErrorIs(err, pkg.ErrConflict)
	_, status = download()
	s.Equal(http.StatusConflict, status)

	s.Require().NoError(s.client.DeleteContestProblem(ctx, corev1.DeleteContestProblemParams{ContestID: contestID, ProblemID: created.ID}))
	s.Empty(states(), "a contest without problems has no booklets")
}
