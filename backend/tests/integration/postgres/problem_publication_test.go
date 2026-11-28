//go:build integration

package postgres_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/smithy-go"
	"github.com/brawler2011/gate/backend/internal/problems/builder"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Fault hooks use real S3, repository transactions and sandbox calls.
type publicationStore struct {
	*storage.S3Storage
	beforeUpload   func(io.Reader) (io.Reader, error)
	afterUpload    func(context.Context, string, string) error
	beforeDownload func(string) error
}

func (s publicationStore) UploadFile(ctx context.Context, bucket, key string, r io.Reader, contentType string) error {
	if err := s.S3Storage.UploadFile(ctx, bucket, key, r, contentType); err != nil {
		return err
	}
	if s.afterUpload != nil {
		return s.afterUpload(ctx, bucket, key)
	}
	return nil
}

func (s publicationStore) UploadFileWithSHA256(ctx context.Context, bucket, key string, r io.Reader, contentType, digest string) error {
	if s.beforeUpload != nil {
		var err error
		r, err = s.beforeUpload(r)
		if err != nil {
			return err
		}
	}
	if err := s.S3Storage.UploadFileWithSHA256(ctx, bucket, key, r, contentType, digest); err != nil {
		return err
	}
	if s.afterUpload != nil {
		return s.afterUpload(ctx, bucket, key)
	}
	return nil
}

func (s publicationStore) DownloadFile(ctx context.Context, bucket, key string, ifNoneMatch *string) (io.ReadCloser, string, error) {
	if s.beforeDownload != nil {
		if err := s.beforeDownload(key); err != nil {
			return nil, "", err
		}
	}
	return s.S3Storage.DownloadFile(ctx, bucket, key, ifNoneMatch)
}

func publicationHarness(t *testing.T) *pipelineHarness {
	t.Helper()
	h := newPipelineHarness(t, 4<<20, 256<<20)
	h.options.Lease = time.Minute
	group := uuid.New()
	h.save(t, func(d *model.Draft) {
		d.Checker = checker("python3", "import sys\nassert open(sys.argv[2]).read() == open(sys.argv[3]).read()\n")
		d.Statements = []model.DraftStatement{{Language: "en", Legend: "Original statement"}}
		d.Testset = model.DraftGroup{ID: group, Aggregate: "sum", Feedback: "complete"}
	})
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	test := model.DraftTest{ID: uuid.New(), Ordinal: 1, GroupID: group, Points: 100,
		Input: model.DraftInput{Mode: "manual"}, Answer: model.DraftAnswer{Mode: "provided"}}
	_, err = h.uploads.Upload(h.ctx, current.ProblemID, current.Revision, model.UploadChange{Test: &test}, map[string]io.Reader{"input": strings.NewReader("original input\n"), "answer": strings.NewReader("original answer\n")})
	require.NoError(t, err)
	return h
}

func publishedFiles(t *testing.T, h *pipelineHarness, id uuid.UUID) (map[string][]byte, *gfmt.Problem) {
	t.Helper()
	desc, err := h.repo.GetPackageDescriptor(h.ctx, id)
	require.NoError(t, err)
	r, _, err := h.services.store.DownloadFile(h.ctx, h.services.bucket, desc.Object.Key, nil)
	require.NoError(t, err)
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	files := map[string][]byte{}
	for _, entry := range z.File {
		f, err := entry.Open()
		require.NoError(t, err)
		files[entry.Name], err = io.ReadAll(f)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}
	var manifest gfmt.Problem
	require.NoError(t, yaml.Unmarshal(files["problem.yaml"], &manifest))
	requireStoredObject(t, h.ctx, h.services.store, h.services.bucket, desc.Object.Key, desc.Object.Size, desc.Object.SHA256)
	return files, &manifest
}

// builtTest finds a test by ordinal; its files are under tests/.
func builtTest(t *testing.T, manifest *gfmt.Problem, ordinal int) gfmt.Test {
	t.Helper()
	for _, gt := range manifest.Tests() {
		if gt.Test.Ordinal == ordinal {
			return gt.Test
		}
	}
	t.Fatalf("package has no test %d", ordinal)
	return gfmt.Test{}
}

func TestPublicationRejectsInconsistentSaves(t *testing.T) {
	h := publicationHarness(t)
	cases := []struct {
		name, message string
		change        func(*model.Draft)
	}{
		{"library name taken", "/library/1/name: duplicate name", func(d *model.Draft) {
			d.Library = []model.DraftLibraryFile{{Name: "a.h"}, {Name: "a.h"}}
		}},
		{"unknown group", "/tests/0/group_id: unknown group", func(d *model.Draft) { d.Tests[0].GroupID = uuid.New() }},
		{"unknown requirement", "unknown group", func(d *model.Draft) {
			d.Testset.Groups = []model.DraftGroup{{ID: uuid.New(), Name: "a", Aggregate: "sum", Feedback: "complete", Requires: []uuid.UUID{uuid.New()}}}
		}},
		{"requirement cycle", "group requirements form a cycle", func(d *model.Draft) {
			a, b := uuid.New(), uuid.New()
			d.Testset.Groups = []model.DraftGroup{{ID: a, Name: "a", Aggregate: "sum", Feedback: "complete", Requires: []uuid.UUID{b}}, {ID: b, Name: "b", Aggregate: "sum", Feedback: "complete", Requires: []uuid.UUID{a}}}
		}},
		{"duplicate requirement", "duplicate requirement", func(d *model.Draft) {
			a, b := uuid.New(), uuid.New()
			d.Testset.Groups = []model.DraftGroup{{ID: a, Name: "a", Aggregate: "sum", Feedback: "complete", Requires: []uuid.UUID{b, b}}, {ID: b, Name: "b", Aggregate: "sum", Feedback: "complete"}}
		}},
		{"missing answer", "provided answer needs content and no solution", func(d *model.Draft) { d.Tests[0].Answer.ObjectID = nil }},
		{"missing generator", "/tests/0/input/steps/0/generator: unknown generator", func(d *model.Draft) {
			name := "gen"
			d.Tests[0].Input = model.DraftInput{Mode: "generator", Steps: []model.DraftStep{{Generator: name}}}
		}},
		{"duplicate test", "/tests/1/id: ID is not unique", func(d *model.Draft) { d.Tests = append(d.Tests, d.Tests[0]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
			require.NoError(t, err)
			candidate, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
			require.NoError(t, err)
			tc.change(&candidate)
			_, err = h.repo.SaveDraft(h.ctx, candidate)
			require.ErrorContains(t, err, tc.message)
			after, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
			require.NoError(t, err)
			require.Equal(t, before, after, "failed save must not partially replace files or metadata")
		})
	}
	// Publish the surviving revision, proving rejected edits never leak into it.
	b := h.enqueueAndClaim(t)
	pid, err := h.pipeline().Execute(h.ctx, b)
	require.NoError(t, err)
	files, manifest := publishedFiles(t, h, pid)
	test := builtTest(t, manifest, 1)
	require.Equal(t, "original input\n", string(files["tests/"+test.Input]))
	require.Equal(t, "original answer\n", string(files["tests/"+test.Answer]))
}

func TestPublicationMissingAndCorruptedObjects(t *testing.T) {
	h := publicationHarness(t)
	for _, tc := range []struct{ name, part, body string }{
		{"missing input", "input", ""}, {"truncated answer", "answer", "short"}, {"changed answer", "answer", "different bytes\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := h.enqueueAndClaim(t)
			id := b.Snapshot.Draft.Tests[0].Input.ObjectID
			if tc.part == "answer" {
				id = b.Snapshot.Draft.Tests[0].Answer.ObjectID
			}
			o, err := h.repo.GetObject(h.ctx, *id)
			require.NoError(t, err)
			t.Cleanup(func() {
				original := "original input\n"
				if tc.part == "answer" {
					original = "original answer\n"
				}
				require.NoError(t, h.services.store.UploadFile(h.ctx, h.services.bucket, o.Key, strings.NewReader(original), "application/octet-stream"))
			})
			if tc.body == "" {
				require.NoError(t, h.services.store.DeleteFile(h.ctx, h.services.bucket, o.Key))
			} else {
				require.NoError(t, h.services.store.UploadFile(h.ctx, h.services.bucket, o.Key, strings.NewReader(tc.body), "application/octet-stream"))
			}
			_, err = h.pipeline().Execute(h.ctx, b)
			var failure *builder.Failure
			require.ErrorAs(t, err, &failure)
			require.Equal(t, "materialize", failure.Stage)
			require.Contains(t, failure.Message, "test 1 "+tc.part)
			require.Contains(t, failure.Message, id.String())
			require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, b.ID))
			require.NoError(t, h.repo.FinishBuildAttempt(h.ctx, b.ID, *b.LeaseToken, json.RawMessage(`{"message":"expected storage fault"}`), false))
		})
	}
}

func TestPublicationUploadVerificationAndRetry(t *testing.T) {
	h := publicationHarness(t)
	h.options.MaxAttempts = 3
	for _, fault := range []string{"truncated", "same size corruption", "empty upload", "lost upload response"} {
		t.Run(fault, func(t *testing.T) {
			b := h.enqueueAndClaim(t)
			pipeline := h.pipeline()
			pipeline.Store = publicationStore{S3Storage: h.services.store,
				beforeUpload: func(r io.Reader) (io.Reader, error) {
					switch fault {
					case "truncated":
						return io.LimitReader(r, 11), nil
					case "empty upload":
						return strings.NewReader(""), nil
					case "same size corruption":
						body, err := io.ReadAll(r)
						if err != nil {
							return nil, err
						}
						body[0] ^= 0xff
						return bytes.NewReader(body), nil
					default:
						return r, nil
					}
				},
				afterUpload: func(context.Context, string, string) error {
					if fault == "lost upload response" {
						return errors.New("injected lost upload response")
					}
					return nil
				},
			}
			_, err := pipeline.Execute(h.ctx, b)
			var failure *builder.Failure
			require.ErrorAs(t, err, &failure)
			require.Equal(t, "upload", failure.Stage)
			if fault != "lost upload response" {
				var apiErr smithy.APIError
				require.ErrorAs(t, err, &apiErr)
				require.Equal(t, "BadDigest", apiErr.ErrorCode(), "the S3 server must reject altered archive bytes")
			}
			require.Equal(t, "infrastructure", failure.Kind)
			require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, b.ID))
			require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM stored_objects o WHERE o.purpose='package' AND NOT EXISTS(SELECT 1 FROM problem_packages p WHERE p.object_id=o.id)`), "a failed attempt leaves its archive bytes without a row")
			h.save(t, func(d *model.Draft) {
				d.Title = "Later revision " + fault
				d.Statements[0].Legend = "Later statement"
			})
			require.NoError(t, h.repo.FinishBuildAttempt(h.ctx, b.ID, *b.LeaseToken, json.RawMessage(`{"message":"retry storage fault"}`), true))
			next, err := h.repo.ClaimBuild(h.ctx, time.Minute)
			require.NoError(t, err)
			require.Equal(t, b.ID, next.ID)
			// Successful publication must not download the archive it just wrote.
			var outputKey string
			retryPipeline := h.pipeline()
			retryPipeline.Store = publicationStore{S3Storage: h.services.store,
				afterUpload: func(_ context.Context, _, key string) error { outputKey = key; return nil },
				beforeDownload: func(key string) error {
					if key == outputKey {
						return errors.New("unexpected archive read-back")
					}
					return nil
				},
			}
			pid, err := retryPipeline.Execute(h.ctx, next)
			require.NoError(t, err)
			files, manifest := publishedFiles(t, h, pid)
			require.Equal(t, b.Snapshot.Draft.Title, manifest.Title)
			require.Equal(t, b.Snapshot.Draft.Statements[0].Legend, string(files["statements/en/legend.tex"]))
		})
	}
}

func TestPublicationConcurrentEdits(t *testing.T) {
	h := publicationHarness(t)
	b := h.enqueueAndClaim(t)
	uploading, release := make(chan struct{}), make(chan struct{})
	// Statement PDFs upload before the package; the edit happens while the first upload waits.
	var started sync.Once
	pipeline := h.pipeline()
	pipeline.Store = publicationStore{S3Storage: h.services.store, afterUpload: func(ctx context.Context, _, _ string) error {
		first := false
		started.Do(func() { close(uploading); first = true })
		if !first {
			return nil
		}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	type result struct {
		id  uuid.UUID
		err error
	}
	done := make(chan result, 1)
	go func() { id, err := pipeline.Execute(h.ctx, b); done <- result{id, err} }()
	// Always unblock the worker, including after an assertion failure.
	defer func() { close(release); <-done }()
	select {
	case <-uploading:
	case r := <-done:
		done <- r
		require.NoError(t, r.err)
		t.Fatal("pipeline stopped before upload")
	case <-time.After(45 * time.Second):
		t.Fatal("pipeline did not reach upload")
	}
	h.save(t, func(d *model.Draft) {
		d.Title = "Concurrent title"
		d.Statements[0].Legend = "Concurrent statement"
		d.Tests[0].Sample = true
		d.Tests[0].Ordinal = 7
	})
	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	_, err = h.uploads.Upload(h.ctx, current.ProblemID, current.Revision, model.UploadChange{Test: &current.Tests[0]}, map[string]io.Reader{"input": strings.NewReader("later input\n"), "answer": strings.NewReader("later answer\n")})
	require.NoError(t, err)
	release <- struct{}{}
	r := <-done
	done <- r
	require.NoError(t, r.err)
	files, manifest := publishedFiles(t, h, r.id)
	require.Equal(t, b.Snapshot.Draft.Title, manifest.Title)
	require.Equal(t, "Original statement", string(files["statements/en/legend.tex"]))
	test := builtTest(t, manifest, 1)
	require.False(t, test.Sample)
	require.Equal(t, "original input\n", string(files["tests/"+test.Input]))
	require.Equal(t, "original answer\n", string(files["tests/"+test.Answer]))
	desc, err := h.repo.GetPackageDescriptor(h.ctx, r.id)
	require.NoError(t, err)
	var revision int64
	var title string
	require.NoError(t, h.pool.QueryRow(h.ctx, `SELECT revision,title FROM problem_packages WHERE id=$1`, desc.ID).Scan(&revision, &title))
	require.Equal(t, b.Revision, revision)
	require.Equal(t, b.Snapshot.Draft.Title, title)
}

func TestPublicationCommitFailure(t *testing.T) {
	h := publicationHarness(t)
	b := h.enqueueAndClaim(t)
	t.Run("rollback at commit", func(t *testing.T) {
		// Fail at COMMIT after the object, package and build writes.
		_, err := h.pool.Exec(h.ctx, `CREATE FUNCTION reject_publication() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'injected publication commit failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_publication AFTER UPDATE ON problem_builds
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.state='succeeded') EXECUTE FUNCTION reject_publication();`)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := h.pool.Exec(h.ctx, `DROP TRIGGER reject_publication ON problem_builds; DROP FUNCTION reject_publication();`)
			require.NoError(t, err)
		})
		_, err = h.pipeline().Execute(h.ctx, b)
		require.ErrorContains(t, err, "injected publication commit failure")
		require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, b.ID))
		require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM stored_objects o WHERE o.purpose='package' AND NOT EXISTS(SELECT 1 FROM problem_packages p WHERE p.object_id=o.id)`), "the rolled-back publication leaves no object row")
		current, err := h.repo.GetBuild(h.ctx, b.ID)
		require.NoError(t, err)
		require.Equal(t, "running", current.State)
	})
	pid, err := h.pipeline().Execute(h.ctx, b)
	require.NoError(t, err)
	publishedFiles(t, h, pid)
	require.Equal(t, 1, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_packages WHERE build_id=$1`, b.ID))
}

func TestPublicationSupportedTypes(t *testing.T) {
	h := publicationHarness(t)
	for _, kind := range []string{"pass-fail", "points", "interactive"} {
		t.Run(kind, func(t *testing.T) {
			h.save(t, func(d *model.Draft) {
				d.Type, d.Grading = "standard", "pass-fail"
				if kind == "points" {
					d.Grading = "points"
					// Publication preserves group requirements and authored test membership.
					first, second := uuid.New(), uuid.New()
					d.Testset.Groups = []model.DraftGroup{{ID: first, Name: "first", Aggregate: "sum", Feedback: "complete"},
						{ID: second, Name: "second", Aggregate: "min", Feedback: "complete", Requires: []uuid.UUID{first}}}
					d.Tests[0].GroupID = first
					test := d.Tests[0]
					test.ID = uuid.New()
					test.Ordinal = 2
					test.GroupID = second
					d.Tests = append(d.Tests, test)
				}
				if kind == "interactive" {
					d.Type = "interactive"
					solution := "main"
					d.Interactor = &model.DraftInteractor{DraftProgram: *program("python3", "print('hello', flush=True)\n"), DraftJury: model.DraftJury{Protocol: sandbox.ProtocolTestlib}}
					d.Solutions = append(d.Solutions, solutionProgram(solution, "python3", "input()\n"))
					d.MainSolution = &solution
					for i := range d.Tests {
						d.Tests[i].Answer = model.DraftAnswer{Mode: "interactive"}
					}
				}
			})
			b := h.enqueueAndClaim(t)
			pid, err := h.pipeline().Execute(h.ctx, b)
			require.NoError(t, err)
			files, manifest := publishedFiles(t, h, pid)
			require.Equal(t, [2]string{b.Snapshot.Draft.Type, b.Snapshot.Draft.Grading}, [2]string{manifest.Type, manifest.Grading})
			d := b.Snapshot.Draft
			builtGroups := map[string]*gfmt.Group{}
			for g := range manifest.Testset.All() {
				builtGroups[g.Name] = g
			}
			for g := range d.Testset.All() {
				built := builtGroups[g.Name]
				require.Equal(t, g.Aggregate, built.Aggregate)
				require.Len(t, built.Requires, len(g.Requires))
				for i, id := range g.Requires {
					require.Equal(t, d.Group(id).Name, built.Requires[i])
				}
				for _, test := range built.Tests {
					require.Equal(t, "original input\n", string(files["tests/"+test.Input]))
					if kind == "interactive" {
						require.Contains(t, files, "tests/"+test.Answer, "the interactor's output on the main solution is the answer")
					} else {
						require.Equal(t, "original answer\n", string(files["tests/"+test.Answer]))
					}
				}
			}
		})
	}
}

func TestPublicationPartialPairUpload(t *testing.T) {
	h := publicationHarness(t)
	before, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	uploads := *h.uploads
	calls := 0
	uploads.Store = publicationStore{S3Storage: h.services.store, afterUpload: func(context.Context, string, string) error {
		calls++
		if calls == 2 {
			return errors.New("injected answer upload failure")
		}
		return nil
	}}
	_, err = uploads.Upload(h.ctx, before.ProblemID, before.Revision, model.UploadChange{Test: &before.Tests[0]}, map[string]io.Reader{"input": strings.NewReader("replacement input\n"), "answer": strings.NewReader("replacement answer\n")})
	require.ErrorContains(t, err, "injected answer upload failure")
	after, err := h.repo.GetDraft(h.ctx, before.ProblemID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	b := h.enqueueAndClaim(t)
	pid, err := h.pipeline().Execute(h.ctx, b)
	require.NoError(t, err)
	require.Equal(t, before, b.Snapshot.Draft)
	files, manifest := publishedFiles(t, h, pid)
	test := builtTest(t, manifest, 1)
	require.Equal(t, "original input\n", string(files["tests/"+test.Input]))
	require.Equal(t, "original answer\n", string(files["tests/"+test.Answer]))
}

func TestPublicationRejectsIncompleteDrafts(t *testing.T) {
	h := publicationHarness(t)
	before, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, message string
		change        func(*model.Draft)
	}{
		{"no checker", "checker is required", func(d *model.Draft) { d.Checker = nil }},
		{"no tests", "at least one test is required", func(d *model.Draft) { d.Tests = nil }},
		{"interactive without interactor", "interactor is required", func(d *model.Draft) {
			d.Type = "interactive"
			for i := range d.Tests {
				d.Tests[i].Answer = model.DraftAnswer{Mode: "interactive"}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h.save(t, tc.change)
			current, err := h.repo.GetDraft(h.ctx, before.ProblemID)
			require.NoError(t, err)
			_, err = h.repo.EnqueueBuild(h.ctx, current.ProblemID, current.Revision, h.options)
			require.ErrorContains(t, err, tc.message)
			require.Zero(t, countRows(t, h.ctx, h.pool, `SELECT count(*) FROM problem_builds WHERE problem_id=$1`, current.ProblemID))
			// A dropped object cannot come back, so the test is uploaded again as an author would.
			h.save(t, func(d *model.Draft) { revision := d.Revision; *d = before; d.Revision = revision; d.Tests = nil })
			current, err = h.repo.GetDraft(h.ctx, before.ProblemID)
			require.NoError(t, err)
			test := before.Tests[0]
			_, err = h.uploads.Upload(h.ctx, current.ProblemID, current.Revision, model.UploadChange{Test: &test},
				map[string]io.Reader{"input": strings.NewReader("original input\n"), "answer": strings.NewReader("original answer\n")})
			require.NoError(t, err)
		})
	}
}

// A published package re-imports into a draft that builds equivalent content.
func TestPublicationReimportsPackage(t *testing.T) {
	h := publicationHarness(t)
	b := h.enqueueAndClaim(t)
	pid, err := h.pipeline().Execute(h.ctx, b)
	require.NoError(t, err)
	files, _ := publishedFiles(t, h, pid)
	published := b.Snapshot
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	for name, data := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	current, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	_, err = h.uploads.Import(h.ctx, current.ProblemID, current.Revision, &archive)
	require.NoError(t, err)
	imported, err := h.repo.GetDraft(h.ctx, h.draft.ProblemID)
	require.NoError(t, err)
	require.Greater(t, imported.Revision, current.Revision)
	require.NoError(t, imported.ValidateBuild(h.uploads.DraftQuotas, h.options.Languages))
	require.Equal(t, published.Draft.Checker, imported.Checker)
	require.Equal(t, published.Draft.Library, imported.Library)
	require.Equal(t, published.Draft.Statements, imported.Statements)
	// An import identifies groups anew.
	withoutIDs := func(g model.DraftGroup) model.DraftGroup {
		g.ID = uuid.Nil
		return g
	}
	require.Equal(t, withoutIDs(published.Draft.Testset), withoutIDs(imported.Testset))
	require.Len(t, imported.Tests, 1)

	rebuilt := h.enqueueAndClaim(t)
	require.Equal(t, imported.Revision, rebuilt.Revision)
	pid, err = h.pipeline().Execute(h.ctx, rebuilt)
	require.NoError(t, err)
	files, manifest := publishedFiles(t, h, pid)
	test := builtTest(t, manifest, 1)
	require.Equal(t, "original input\n", string(files["tests/"+test.Input]))
	require.Equal(t, "original answer\n", string(files["tests/"+test.Answer]))
}
