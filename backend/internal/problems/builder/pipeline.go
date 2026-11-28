package builder

import (
	"math"
	"slices"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"

	"github.com/brawler2011/gate/backend/internal/judging"
	"github.com/brawler2011/gate/backend/internal/problems/archive"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

type ObjectStore interface {
	DownloadFile(context.Context, string, string, *string) (io.ReadCloser, string, error)
	UploadFileWithSHA256(context.Context, string, string, io.Reader, string, string) error
}
type BuildRepository interface {
	ClaimBuild(context.Context, time.Duration) (model.ProblemBuild, error)
	HeartbeatBuild(context.Context, uuid.UUID, uuid.UUID, time.Duration) error
	RecoverBuilds(context.Context) error
	StoredObjects(context.Context, []uuid.UUID) ([]model.StoredObject, error)
	PublishBuild(context.Context, uuid.UUID, uuid.UUID, model.StoredObject, model.PackageIndex) (uuid.UUID, error)
	FinishBuildAttempt(context.Context, uuid.UUID, uuid.UUID, json.RawMessage, bool) error
	RecordBuildSolutions(context.Context, uuid.UUID, uuid.UUID, []model.SolutionResult) error
	RecordBuildProgramTests(context.Context, uuid.UUID, uuid.UUID, []model.ProgramTestResult) error
}
type Pipeline struct {
	Repo      BuildRepository
	Store     ObjectStore
	Bucket    string
	TempDir   string
	Client    *sandbox.Client
	Typeset   Typesetter
	Files     *sandbox.FileTransport
	Languages *sandbox.Config
	Programs  *sandbox.ProgramCache
	// Programs one build runs at once.
	Parallelism int
	// Scales solution time limits to this host and reported times back to the reference machine.
	Calibration func() calibration.Calibration
	Logger      *slog.Logger
}
type Typesetter interface {
	statement.Converter
	PDF(ctx context.Context, doc string, files map[string][]byte) ([]byte, error)
}

type Failure struct {
	Stage     string `json:"stage"`
	Test      int    `json:"test,omitempty"`
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Truncated bool   `json:"truncated"`
	cause     error
}

func (f *Failure) Error() string { return f.Stage + ": " + f.Message }
func (f *Failure) Unwrap() error { return f.cause }
func fail(stage string, test int, kind string, err error) error {
	if kind == "infrastructure" && (errors.Is(err, errBudgetExceeded) || errors.Is(err, sandbox.ErrQuotaExceeded)) {
		kind = "quota"
	}
	text := err.Error()
	truncated := len(text) > 8192
	if truncated {
		text = text[:8192]
	}
	return &Failure{Stage: stage, Test: test, Kind: kind, Message: text, Truncated: truncated, cause: err}
}
func processKind(err error) string {
	// A missing language means API/worker catalog disagreement; retrying cannot fix it.
	if errors.Is(err, sandbox.ErrUnknownLanguage) {
		return "configuration"
	}
	var network net.Error
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sandbox.ErrUnavailable) || errors.As(err, &network) {
		return "infrastructure"
	}
	return "author"
}

type compilation struct {
	language   string
	executable sandbox.Executable
}
type execution struct {
	pipeline *Pipeline
	build    model.ProblemBuild
	dir      *directory
	sandbox  *sandbox.Sandbox
	pkg      *archive.Package
	// What each test materialized, relative to dir.
	tests    map[uuid.UUID]archive.BuiltTest
	compiled map[string]compilation
	samples  []model.Sample
	// Test results of the solutions that run, by name.
	solutions map[string][]judging.TestResult
	// Solutions that run only to be judged report compilation errors as their verdict.
	uncompiled map[string]bool
	// What the time rule found wrong with a solution's run times, by name; timeFailure stops the build.
	timing      map[string]string
	timeFailure error
}

func (p *Pipeline) Execute(ctx context.Context, b model.ProblemBuild) (uuid.UUID, error) {
	s := b.Snapshot
	if s.Limits.DiskBytes <= 0 || s.Limits.FileCount <= 0 || s.Limits.ObjectBytes <= 0 || s.Limits.TestOutputBytes <= 0 {
		return uuid.Nil, fail("limits", 0, "infrastructure", fmt.Errorf("invalid fixed build budgets"))
	}
	var stat unix.Statfs_t
	if err := unix.Statfs(p.TempDir, &stat); err != nil {
		return uuid.Nil, fail("materialize", 0, "infrastructure", err)
	}
	if stat.Type == unix.TMPFS_MAGIC {
		return uuid.Nil, fail("materialize", 0, "infrastructure", fmt.Errorf("build directory must reside on disk"))
	}
	root, err := os.MkdirTemp(p.TempDir, "build-"+b.ID.String()+"-"+b.LeaseToken.String()+"-")
	if err != nil {
		return uuid.Nil, fail("materialize", 0, "infrastructure", err)
	}
	defer func() {
		if err := os.RemoveAll(root); err != nil {
			left, _ := os.ReadDir(root)
			names := []string{}
			for _, e := range left {
				names = append(names, e.Name())
			}
			p.Logger.WarnContext(ctx, "remove build directory", "error", err, "build_id", b.ID, "left", names)
		}
	}()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		return uuid.Nil, fmt.Errorf("mkdir: %w", err)
	}
	e := &execution{pipeline: p, build: b, dir: &directory{root: work, remaining: int64(s.Limits.DiskBytes), maxFiles: s.Limits.FileCount}, sandbox: sandbox.NewSandbox(p.Client, p.Languages), tests: testFiles(s.Draft), compiled: make(map[string]compilation), samples: []model.Sample{}, solutions: map[string][]judging.TestResult{}, uncompiled: map[string]bool{}, timing: map[string]string{}}
	defer e.releaseComponents()
	local := map[uuid.UUID]archive.BuiltTest{}
	for id, t := range e.tests {
		local[id] = archive.BuiltTest{Input: filepath.Join(work, t.Input), Answer: filepath.Join(work, t.Answer)}
	}
	if e.pkg, err = archive.Layout(s.Draft, p.Languages, local); err != nil {
		return uuid.Nil, fail("package", 0, "infrastructure", err)
	}
	if err := e.materialize(ctx); err != nil {
		return uuid.Nil, fail("materialize", 0, "infrastructure", err)
	}
	if err := e.compile(ctx); err != nil {
		return uuid.Nil, err
	}
	if err := e.checkPrograms(ctx); err != nil {
		return uuid.Nil, err
	}
	if err := e.runTests(ctx); err != nil {
		return uuid.Nil, err
	}
	if err := e.checkSolutions(ctx); err != nil {
		return uuid.Nil, err
	}
	statements, pdfs, err := e.typeset(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	f, err := os.OpenFile(filepath.Join(root, "package.zip"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return uuid.Nil, fail("archive", 0, "infrastructure", err)
	}
	defer f.Close()
	digest, size, err := e.archive(f)
	if err != nil {
		return uuid.Nil, err
	}
	object := model.NewStoredObject(b.OrganizationID, model.UploadSpec{Purpose: "package", SHA256: digest, Size: size})
	if err := p.Store.UploadFileWithSHA256(ctx, p.Bucket, object.Key, io.NewSectionReader(f, 0, size), "application/octet-stream", digest); err != nil {
		return uuid.Nil, fail("upload", 0, "infrastructure", err)
	}
	var files *model.StoredObject
	if s.Draft.Type == "output-only" || len(s.Draft.Attachments) > 0 {
		if files, err = e.uploadFiles(ctx, filepath.Join(root, "files.zip")); err != nil {
			return uuid.Nil, err
		}
	}
	// The run may have derived the time limit into the execution's draft.
	index := model.NewPackageIndex(e.build.Snapshot.Draft, statements, pdfs, e.samples, files)
	if len(s.Draft.Graders) > 0 {
		index.Languages = p.Languages.Graded(s.Draft.GraderNames(), s.Draft.SolutionName)
	}
	pid, err := p.Repo.PublishBuild(ctx, b.ID, *b.LeaseToken, object, index)
	if err != nil {
		return uuid.Nil, fail("publish", 0, "infrastructure", err)
	}
	return pid, nil
}

// uploadFiles packs the attachments and an output-only problem's test inputs as <ordinal>.in, zero-padded so the names sort.
func (e *execution) uploadFiles(ctx context.Context, path string) (*model.StoredObject, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, fail("files", 0, "infrastructure", err)
	}
	defer f.Close()
	d := e.build.Snapshot.Draft
	hash := sha256.New()
	w := zip.NewWriter(io.MultiWriter(f, hash))
	for _, a := range d.Attachments {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: a.Name, Method: zip.Deflate})
		if err == nil {
			_, err = io.WriteString(entry, a.Content)
		}
		if err != nil {
			return nil, fail("files", 0, "infrastructure", err)
		}
	}
	var tests []model.DraftTest
	if d.Type == "output-only" {
		tests = d.Tests
	}
	width := 0
	for _, t := range tests {
		width = max(width, len(strconv.Itoa(t.Ordinal)))
	}
	for _, t := range tests {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("%0*d.in", width, t.Ordinal), Method: zip.Deflate})
		if err != nil {
			return nil, fail("files", t.Ordinal, "infrastructure", err)
		}
		in, err := os.Open(filepath.Join(e.dir.root, e.tests[t.ID].Input))
		if err != nil {
			return nil, fail("files", t.Ordinal, "infrastructure", err)
		}
		_, err = io.Copy(entry, in)
		in.Close()
		if err != nil {
			return nil, fail("files", t.Ordinal, "infrastructure", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fail("files", 0, "infrastructure", err)
	}
	size, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fail("files", 0, "infrastructure", err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	object := model.NewStoredObject(e.build.OrganizationID, model.UploadSpec{Purpose: "files", SHA256: digest, Size: size})
	if err := e.pipeline.Store.UploadFileWithSHA256(ctx, e.pipeline.Bucket, object.Key, io.NewSectionReader(f, 0, size), "application/zip", digest); err != nil {
		return nil, fail("upload", 0, "infrastructure", err)
	}
	return &object, nil
}

// typeset makes the site's HTML and the PDF of each statement, so authors, not participants, meet LaTeX errors.
func (e *execution) typeset(ctx context.Context) ([]model.BuiltStatement, []model.StatementPDF, error) {
	d := e.build.Snapshot.Draft
	limits := model.PrintedLimits{TimeLimitMs: d.TimeLimitMs, MemoryLimitMb: d.MemoryLimitMb, InputFile: d.InputFile, OutputFile: d.OutputFile}
	built := make([]model.BuiltStatement, 0, len(d.Statements))
	var pdfs []model.StatementPDF
	for _, s := range d.Statements {
		html, err := statement.HTML(ctx, e.pipeline.Typeset, s.SiteSections(), d.HasAsset)
		if err != nil {
			return nil, nil, fail("statements", 0, typesetKind(err), fmt.Errorf("%s statement: %w", s.Language, err))
		}
		built = append(built, model.BuiltStatement{DraftStatement: s, HTML: html})
		files := statement.Files(s.Language)
		for _, a := range d.Assets {
			data, err := os.ReadFile(filepath.Join(e.dir.root, "objects", a.ObjectID.String()))
			if err != nil {
				return nil, nil, fail("statements", 0, "infrastructure", err)
			}
			files["assets/"+a.Name] = data
		}
		doc := statement.NewDocument(statement.Contest{Title: d.Title, Language: s.Language, Problems: []statement.Problem{model.PrintedProblem(d.Title, limits, s, e.samples)}})
		pdf, err := e.pipeline.Typeset.PDF(ctx, doc.Source, files)
		if err != nil {
			return nil, nil, fail("statements", 0, typesetKind(err), fmt.Errorf("%s statement: %w", s.Language, doc.Explain(err)))
		}
		sum := sha256.Sum256(pdf)
		digest := hex.EncodeToString(sum[:])
		object := model.NewStoredObject(e.build.OrganizationID, model.UploadSpec{Purpose: "statement", SHA256: digest, Size: int64(len(pdf))})
		if err := e.pipeline.Store.UploadFileWithSHA256(ctx, e.pipeline.Bucket, object.Key, bytes.NewReader(pdf), "application/pdf", digest); err != nil {
			return nil, nil, fail("upload", 0, "infrastructure", err)
		}
		pdfs = append(pdfs, model.StatementPDF{Language: s.Language, Object: object})
	}
	return built, pdfs, nil
}

func typesetKind(err error) string {
	if _, ok := errors.AsType[*statement.Error](err); ok {
		return "author"
	}
	return processKind(err)
}

func objectReferences(d model.Draft, id uuid.UUID) string {
	refs := []string{}
	d.EachObject(func(ref string, oid *uuid.UUID) {
		if *oid == id {
			refs = append(refs, ref)
		}
	})
	return fmt.Sprintf("object %s (%s)", id, strings.Join(refs, ", "))
}

// testFiles places every test's input and answer: objects keep their ID, built files the test's.
func testFiles(d model.Draft) map[uuid.UUID]archive.BuiltTest {
	files := make(map[uuid.UUID]archive.BuiltTest, len(d.Tests))
	for _, t := range d.Tests {
		f := archive.BuiltTest{Input: "tests/" + t.ID.String() + ".in", Answer: "tests/" + t.ID.String() + ".ans"}
		if t.Input.Mode == "manual" {
			f.Input = "objects/" + t.Input.ObjectID.String()
		}
		if t.Answer.Mode == "provided" {
			f.Answer = "objects/" + t.Answer.ObjectID.String()
		}
		files[t.ID] = f
	}
	return files
}

func (e *execution) materialize(ctx context.Context) error {
	s := e.build.Snapshot
	objects, err := e.pipeline.Repo.StoredObjects(ctx, s.InputObjects)
	if err != nil {
		return err
	}
	expected := make(map[uuid.UUID]bool, len(s.InputObjects))
	for _, id := range s.InputObjects {
		expected[id] = true
	}
	if len(expected) != len(objects) {
		return fmt.Errorf("snapshot root mismatch")
	}
	for _, o := range objects {
		if !expected[o.ID] || o.OrganizationID != s.OrganizationID || o.Size > int64(s.Limits.ObjectBytes) {
			return fmt.Errorf("invalid snapshot object")
		}
		r, _, err := e.pipeline.Store.DownloadFile(ctx, e.pipeline.Bucket, o.Key, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", objectReferences(s.Draft, o.ID), err)
		}
		digest, n, copyErr := e.dir.copy("objects/"+o.ID.String(), r, o.Size)
		closeErr := r.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return fmt.Errorf("%s: %w", objectReferences(s.Draft, o.ID), err)
		}
		if n != o.Size || digest != o.SHA256 {
			return fmt.Errorf("%s: snapshot object bytes differ from registry", objectReferences(s.Draft, o.ID))
		}
	}
	return nil
}

// archive writes the package into f within the build budget and checks that the judge accepts it.
func (e *execution) archive(f *os.File) (string, int64, error) {
	hash := sha256.New()
	stats, err := e.pkg.Write(io.MultiWriter(quotaWriter{f, e.dir}, hash), func(oid uuid.UUID) (io.ReadCloser, error) {
		return os.Open(filepath.Join(e.dir.root, "objects", oid.String()))
	})
	if err != nil {
		return "", 0, fail("archive", 0, "infrastructure", err)
	}
	size, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", 0, fail("archive", 0, "infrastructure", err)
	}
	// Judges unpack the package within the same budget.
	if !stats.Extracts(size, e.build.Snapshot.Limits) {
		return "", 0, fail("package", 0, "quota", fmt.Errorf("package unpacks to %d bytes in %d files, over the disk quota", stats.Bytes+size, stats.Files))
	}
	err = e.pkg.Manifest.CheckBuilt(func(name string) error {
		if !stats.Names[name] {
			return fmt.Errorf("package has no %s", name)
		}
		return nil
	})
	if err != nil {
		return "", 0, fail("package", 0, "author", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
func (e *execution) release(id string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.pipeline.Files.Delete(ctx, id); err != nil {
		e.pipeline.Logger.WarnContext(ctx, "release sandbox file", "error", err, "build_id", e.build.ID)
	}
}
func (e *execution) releaseComponents() {
	for _, c := range e.compiled {
		e.release(c.executable.PrimaryFileID)
		for _, id := range c.executable.Dependencies {
			e.release(id)
		}
	}
}
func (e *execution) download(ctx context.Context, path, id string, limit int64) error {
	f, err := e.dir.create(path)
	if err != nil {
		return err
	}
	_, copyErr := e.pipeline.Files.Download(ctx, id, quotaWriter{f, e.dir}, limit)
	return errors.Join(copyErr, f.Close())
}
func (e *execution) upload(ctx context.Context, path string) (string, error) {
	f, err := os.Open(filepath.Join(e.dir.root, path))
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	return e.pipeline.Files.Upload(ctx, f, int64(e.build.Snapshot.Limits.ObjectBytes))
}
func (e *execution) compile(ctx context.Context) error {
	s := e.build.Snapshot
	d := s.Draft
	required := map[string]bool{"checker": true, "interactor": true}
	for _, v := range d.Validators {
		required[programKey("validator", v.Name)] = true
	}
	if d.MainSolution != nil {
		required[programKey("solution", *d.MainSolution)] = true
	}
	for _, test := range d.Tests {
		if test.Input.Generator != nil {
			required[programKey("generator", *test.Input.Generator)] = true
		}
		if test.Answer.Solution != nil {
			required[programKey("solution", *test.Answer.Solution)] = true
		}
	}
	judged := map[string]bool{}
	for _, s := range d.Solutions {
		if key := programKey("solution", s.Name); s.Runs() && !required[key] {
			judged[key] = true
		}
	}
	var needed []placedProgram
	for _, p := range programs(d, e.pipeline.Languages) {
		if required[p.key] || judged[p.key] {
			needed = append(needed, p)
		}
	}
	library := map[string][]byte{}
	for i := range d.Library {
		f := &d.Library[i]
		library[f.Name] = []byte(f.Text())
		if f.ObjectID != nil {
			data, err := os.ReadFile(filepath.Join(e.dir.root, "objects", f.ObjectID.String()))
			if err != nil {
				return fail("compile", 0, "infrastructure", err)
			}
			library[f.Name] = data
		}
	}
	var mu sync.Mutex
	return ordered(ctx, len(needed), e.pipeline.Parallelism, func(ctx context.Context, i int) error {
		p := needed[i]
		// Judge programs compile with the whole library; solutions with the graders, as submissions do.
		program := sandbox.Program{Source: []byte(p.program.Source), Language: p.program.Language, Deps: map[string][]byte{}}
		if p.role == "solution" {
			program.Graders = d.GraderFiles()
		} else {
			program.Deps = library
		}
		compiled, err := e.pipeline.Programs.Get(ctx, program, s.Limits.Compile)
		if err != nil && judged[p.key] && processKind(err) == "author" {
			mu.Lock()
			e.uncompiled[strings.TrimPrefix(p.key, "solution/")] = true
			mu.Unlock()
			return nil
		}
		if err != nil {
			return fail("compile", 0, processKind(err), fmt.Errorf("%s: %w", p.path, err))
		}
		executable, err := e.pipeline.Files.UploadCompiled(ctx, compiled.Dir(), int64(s.Limits.Compile.Output))
		compiled.Release()
		if err != nil {
			return fail("compile", 0, "infrastructure", fmt.Errorf("%s: %w", p.path, err))
		}
		mu.Lock()
		e.compiled[p.key] = compilation{language: p.program.Language, executable: executable}
		mu.Unlock()
		return nil
	})
}
// solutionLimits runs solutions under timeLimit, a reference time that the calibration scales to this machine.
func (e *execution) solutionLimits(timeLimit time.Duration) sandbox.FileRunLimits {
	d := e.build.Snapshot.Draft
	return e.build.Snapshot.Limits.Solution(e.pipeline.Calibration().Limit(timeLimit), bytesize.Size(d.MemoryLimitMb)*bytesize.MiB)
}

type testResult struct {
	solutions map[string]judging.TestResult
	sample    *model.Sample
}

// runTests judges every solution as the judge would. Under a time rule, accepted solutions run first: their times
// derive or check the limit, and the others then run with the margin the rule asks of solutions that must time out.
func (e *execution) runTests(ctx context.Context) error {
	d := &e.build.Snapshot.Draft
	tests := d.Tests
	limit := e.build.Snapshot.Limits.TimeCeiling
	if d.TimeLimitMs != nil {
		limit = time.Duration(*d.TimeLimitMs) * time.Millisecond
	}
	first := func(sol *model.DraftSolution) bool { return d.TimeRule == nil || sol.MustPass() }
	results := make([]testResult, len(tests))
	err := ordered(ctx, len(tests), e.pipeline.Parallelism, func(ctx context.Context, i int) error {
		var err error
		results[i], err = e.test(ctx, tests[i], first, limit)
		return err
	})
	if err != nil {
		return err
	}
	for _, r := range results {
		for name, result := range r.solutions {
			e.solutions[name] = append(e.solutions[name], result)
		}
		if r.sample != nil {
			e.samples = append(e.samples, *r.sample)
		}
	}
	if d.TimeRule == nil || !e.settleTimeLimit() {
		return nil
	}
	rest := []string{}
	for i := range d.Solutions {
		if sol := &d.Solutions[i]; sol.Runs() && !e.uncompiled[sol.Name] && !first(sol) {
			rest = append(rest, sol.Name)
		}
	}
	if len(rest) == 0 {
		return nil
	}
	tl := *d.TimeLimitMs
	margin := time.Duration(math.Ceil(float64(tl)*d.TimeRule.TLEFactor)) * time.Millisecond
	rejudged := make([]map[string]judging.TestResult, len(tests))
	err = ordered(ctx, len(tests), e.pipeline.Parallelism, func(ctx context.Context, i int) error {
		var err error
		rejudged[i], err = e.rejudge(ctx, tests[i], rest, margin)
		return err
	})
	if err != nil {
		return err
	}
	for _, r := range rejudged {
		for _, name := range rest {
			e.solutions[name] = append(e.solutions[name], r[name])
		}
	}
	for i := range d.Solutions {
		if sol := &d.Solutions[i]; slices.Contains(rest, sol.Name) {
			e.applyTimeLimit(sol, tl)
		}
	}
	return nil
}

// rejudge judges solutions on a test that test already made, from its files.
func (e *execution) rejudge(ctx context.Context, test model.DraftTest, names []string, limit time.Duration) (map[string]judging.TestResult, error) {
	d := e.build.Snapshot.Draft
	files := e.tests[test.ID]
	inputID, err := e.upload(ctx, files.Input)
	if err != nil {
		return nil, fail("solutions", test.Ordinal, "infrastructure", err)
	}
	defer e.release(inputID)
	var answerID string
	if _, err := os.Stat(filepath.Join(e.dir.root, files.Answer)); err == nil {
		if answerID, err = e.upload(ctx, files.Answer); err != nil {
			return nil, fail("solutions", test.Ordinal, "infrastructure", err)
		}
		defer e.release(answerID)
	}
	args := d.Group(test.GroupID).CheckerArgs
	results := map[string]judging.TestResult{}
	for _, name := range names {
		result, err := e.judge(ctx, name, test, args, inputID, answerID, limit)
		if err != nil {
			return nil, fail("solutions", test.Ordinal, "infrastructure", fmt.Errorf("solution %s: %w", name, err))
		}
		results[name] = result
	}
	return results, nil
}

// settleTimeLimit derives a missing limit from the accepted solutions, or checks that they fit the given one.
// It reports false when they failed, which checkSolutions reports, so the rest need not run.
func (e *execution) settleTimeLimit() bool {
	d := &e.build.Snapshot.Draft
	rule := d.TimeRule
	var slowest time.Duration
	accepted := false
	for i := range d.Solutions {
		sol := &d.Solutions[i]
		if !sol.Runs() || !sol.MustPass() || e.uncompiled[sol.Name] {
			continue
		}
		accepted = true
		for _, t := range e.solutions[sol.Name] {
			if t.Verdict != string(sandbox.StatusOK) {
				if t.Verdict == string(sandbox.StatusTLE) && d.TimeLimitMs == nil {
					e.timing[sol.Name] = fmt.Sprintf("the build runs accepted solutions for at most %s while it derives the time limit", e.build.Snapshot.Limits.TimeCeiling)
				}
				return false
			}
			slowest = max(slowest, time.Duration(t.Time))
		}
	}
	if d.TimeLimitMs == nil {
		if !accepted {
			e.timeFailure = errors.New("the time limit is derived from accepted solutions, but none runs")
			return false
		}
		tl := rule.Derive(slowest)
		d.TimeLimitMs, e.pkg.Manifest.Limits.TimeMs = &tl, tl
		return true
	}
	bound := float64(*d.TimeLimitMs) / rule.AcceptedFactor
	for i := range d.Solutions {
		sol := &d.Solutions[i]
		if !sol.Runs() || !sol.MustPass() || e.uncompiled[sol.Name] {
			continue
		}
		for _, t := range e.solutions[sol.Name] {
			if ms := float64(t.Time) / float64(time.Millisecond); ms > bound {
				e.timing[sol.Name] = fmt.Sprintf("test %d: runs %.0f ms, but the time rule needs accepted solutions within the limit / %g = %.0f ms", t.TestNumber, ms, rule.AcceptedFactor, bound)
				break
			}
		}
	}
	return true
}

// applyTimeLimit turns runs over the limit into TLE, since the rest ran with a margin above it, and checks
// that a solution which must time out exceeds the margin rather than the limit alone.
func (e *execution) applyTimeLimit(sol *model.DraftSolution, tl int) {
	d := &e.build.Snapshot.Draft
	timedOut, over := false, 0
	results := e.solutions[sol.Name]
	for i := range results {
		r := &results[i]
		switch {
		case r.Verdict == string(sandbox.StatusTLE):
			timedOut = true
		// An interactor that rejected the solution first decides the verdict, as the judge does, so only finished interactive runs change.
		case r.Time > int64(tl)*int64(time.Millisecond) && (!spec.Interactive(d.Type) || r.Verdict == string(sandbox.StatusOK) || r.Verdict == "POINTS"):
			over = r.TestNumber
			r.Verdict, r.Score = string(sandbox.StatusTLE), nil
			r.Award(0, d.Grading)
		}
	}
	if sol.MustTimeOut() && !timedOut && over != 0 {
		e.timing[sol.Name] = fmt.Sprintf("test %d: over the limit, but the time rule needs solutions that must time out to run at least %g times the limit", over, d.TimeRule.TLEFactor)
	}
}

// test makes the test's files and judges the solutions that judged selects, under limit.
func (e *execution) test(ctx context.Context, test model.DraftTest, judged func(*model.DraftSolution) bool, limit time.Duration) (testResult, error) {
	s := e.build.Snapshot
	tr := testResult{solutions: map[string]judging.TestResult{}}
	aux := s.Limits.Auxiliary
	ids := []string{}
	defer func() {
		for _, id := range ids {
			e.release(id)
		}
	}()
	files := e.tests[test.ID]
	var inputID string
	if test.Input.Mode == "manual" {
		id, err := e.upload(ctx, files.Input)
		if err != nil {
			return tr, fail("input", test.Ordinal, "infrastructure", err)
		}
		inputID = id
		ids = append(ids, id)
	} else {
		gen := e.compiled[programKey("generator", *test.Input.Generator)]
		result, err := e.sandbox.RunFiles(ctx, gen.executable, gen.language, sandbox.File{}, test.Input.Args, nil, aux)
		if err != nil {
			return tr, fail("generate", test.Ordinal, "infrastructure", err)
		}
		ids = append(ids, result.OutputID)
		if result.Status != sandbox.StatusOK || result.ExitStatus != 0 {
			return tr, fail("generate", test.Ordinal, string(result.Status), fmt.Errorf("exit %d: %s", result.ExitStatus, result.Stderr))
		}
		inputID = result.OutputID
		if err := e.download(ctx, files.Input, inputID, int64(s.Limits.ObjectBytes)); err != nil {
			return tr, fail("generate", test.Ordinal, "infrastructure", err)
		}
	}
	group := s.Draft.Group(test.GroupID)
	if group == nil {
		return tr, fail("validate", test.Ordinal, "infrastructure", errors.New("test has no group"))
	}
	for i := range s.Draft.Validators {
		validator := &s.Draft.Validators[i]
		if !s.Draft.Validates(validator, test.GroupID) {
			continue
		}
		v := e.compiled[programKey("validator", validator.Name)]
		result, err := e.sandbox.ValidateFiles(ctx, validator.Jury(v.executable, v.language).With(group.ValidatorArgs), sandbox.FileID(inputID), group.Name, aux)
		if err != nil {
			return tr, fail("validate", test.Ordinal, "infrastructure", fmt.Errorf("validator %s: %w", validator.Name, err))
		}
		if result.Status != sandbox.StatusOK {
			return tr, fail("validate", test.Ordinal, string(result.Status), fmt.Errorf("validator %s: %s", validator.Name, result.Message))
		}
	}
	var outputID, answerID string
	// answerer is the solution whose output the checker grades as the answer, if any.
	var answerer *string
	if spec.Interactive(s.Draft.Type) {
		answerer = s.Draft.MainSolution
		var answer sandbox.File
		if test.Answer.Mode == "provided" {
			id, err := e.upload(ctx, files.Answer)
			if err != nil {
				return tr, fail("answer", test.Ordinal, "infrastructure", err)
			}
			answerID, answer = id, sandbox.FileID(id)
			ids = append(ids, id)
		}
		sol := e.compiled[programKey("solution", *s.Draft.MainSolution)]
		intr := e.compiled["interactor"]
		result, err := e.sandbox.InteractFiles(ctx, sol.executable, sol.language, s.Draft.Interactor.Jury(intr.executable, intr.language).With(group.CheckerArgs), s.Draft.Processes, sandbox.FileID(inputID), answer, e.solutionLimits(limit), aux)
		if err != nil {
			return tr, fail("interact", test.Ordinal, "infrastructure", err)
		}
		ids = append(ids, result.OutputID)
		if r := e.award(test, string(result.Status), result.Score, result.Message); r.Verdict != string(sandbox.StatusOK) {
			return tr, fail("interact", test.Ordinal, r.Verdict, answerFailure(answerer, r.Verdict, r.Message))
		}
		outputID = result.OutputID
		// Without a provided answer, the main solution's interactor output becomes the answer for later judging.
		if answerID == "" {
			answerID = outputID
			if err := e.download(ctx, files.Answer, answerID, int64(s.Limits.ObjectBytes)); err != nil {
				return tr, fail("interact", test.Ordinal, "infrastructure", err)
			}
		}
	} else {
		if test.Answer.Mode == "provided" {
			id, err := e.upload(ctx, files.Answer)
			if err != nil {
				return tr, fail("answer", test.Ordinal, "infrastructure", err)
			}
			answerID = id
			ids = append(ids, id)
		}
		solution := test.Answer.Solution
		if test.Answer.Mode == "provided" {
			solution = s.Draft.MainSolution
		}
		outputID, answerer = answerID, solution
		if solution != nil {
			sol := e.compiled[programKey("solution", *solution)]
			result, err := e.sandbox.RunSolution(ctx, sol.executable, sol.language, sandbox.FileID(inputID), s.Draft.Streams(), e.solutionLimits(limit))
			if err != nil {
				return tr, fail("answer", test.Ordinal, "infrastructure", err)
			}
			ids = append(ids, result.OutputID)
			if result.Status != sandbox.StatusOK || result.ExitStatus != 0 {
				return tr, fail("answer", test.Ordinal, string(result.Status), answerFailure(solution, string(result.Status), fmt.Sprintf("exit %d: %s", result.ExitStatus, result.Stderr)))
			}
			outputID = result.OutputID
			if test.Answer.Mode == "generated" {
				answerID = outputID
				if err := e.download(ctx, files.Answer, answerID, int64(s.Limits.ObjectBytes)); err != nil {
					return tr, fail("answer", test.Ordinal, "infrastructure", err)
				}
			}
		}
	}
	if s.Draft.Checker != nil {
		checker := e.compiled["checker"]
		result, err := e.sandbox.CheckFiles(ctx, s.Draft.Checker.Jury(checker.executable, checker.language).With(group.CheckerArgs), sandbox.FileID(inputID), sandbox.FileID(outputID), sandbox.FileID(answerID), aux)
		if err != nil {
			return tr, fail("check", test.Ordinal, "infrastructure", err)
		}
		if r := e.award(test, string(result.Status), result.Score, result.Message); r.Verdict != string(sandbox.StatusOK) {
			return tr, fail("check", test.Ordinal, r.Verdict, answerFailure(answerer, r.Verdict, r.Message))
		}
	}
	for i := range s.Draft.Solutions {
		sol := &s.Draft.Solutions[i]
		if !sol.Runs() || e.uncompiled[sol.Name] || !judged(sol) {
			continue
		}
		result, err := e.judge(ctx, sol.Name, test, group.CheckerArgs, inputID, answerID, limit)
		if err != nil {
			return tr, fail("solutions", test.Ordinal, "infrastructure", fmt.Errorf("solution %s: %w", sol.Name, err))
		}
		tr.solutions[sol.Name] = result
	}
	// Interactive test files are interactor data, not what the participant sees.
	interactive := spec.Interactive(s.Draft.Type)
	if test.Sample && (!interactive || test.StatementInput != nil) {
		input, answer := files.Input, files.Answer
		if test.StatementInput != nil {
			input = ""
		}
		if test.StatementOutput != nil || interactive {
			answer = ""
		}
		sample, err := e.sample(input, answer)
		if err != nil {
			return tr, fail("sample", test.Ordinal, "infrastructure", err)
		}
		if test.StatementInput != nil {
			sample.Input = *test.StatementInput
		}
		if test.StatementOutput != nil {
			sample.Output = *test.StatementOutput
		}
		tr.sample = &sample
	}
	return tr, nil
}

// answerFailure names the solution whose output had to earn the test's points to serve as its answer.
func answerFailure(solution *string, verdict, message string) error {
	if solution == nil {
		return errors.New(message)
	}
	return fmt.Errorf("solution %s gets %s, but answers come from a solution with the full score; choose another main solution: %s", *solution, verdict, message)
}

// judge runs a solution on a test as the judge runs submissions. A TLE runs again before it counts,
// since concurrent steps contend for the CPU.
func (e *execution) judge(ctx context.Context, name string, test model.DraftTest, args []string, inputID, answerID string, limit time.Duration) (judging.TestResult, error) {
	for rerun := true; ; rerun = false {
		result, err := e.judgeOnce(ctx, name, test, args, inputID, answerID, limit)
		if err != nil || !rerun || result.Verdict != string(sandbox.StatusTLE) {
			return result, err
		}
	}
}

// award grades a jury verdict on the main solution, which must earn every point of the test.
func (e *execution) award(test model.DraftTest, status string, score *float64, message string) judging.TestResult {
	r := judging.TestResult{Verdict: status, Score: score, Message: message}
	r.Award(test.Points, e.build.Snapshot.Draft.Grading)
	return r
}

// judgeOnce passes args, those of the test's group, to the checker or interactor.
func (e *execution) judgeOnce(ctx context.Context, name string, test model.DraftTest, args []string, inputID, answerID string, limit time.Duration) (judging.TestResult, error) {
	s := e.build.Snapshot
	aux := s.Limits.Auxiliary
	sol := e.compiled[programKey("solution", name)]
	result := judging.TestResult{TestNumber: test.Ordinal}
	var outputID string
	if spec.Interactive(s.Draft.Type) {
		intr := e.compiled["interactor"]
		r, err := e.sandbox.InteractFiles(ctx, sol.executable, sol.language, s.Draft.Interactor.Jury(intr.executable, intr.language).With(args), s.Draft.Processes, sandbox.FileID(inputID), sandbox.FileID(answerID), e.solutionLimits(limit), aux)
		if err != nil {
			return result, err
		}
		defer e.release(r.OutputID)
		outputID, result.Verdict, result.Score, result.Time = r.OutputID, string(r.Status), r.Score, e.pipeline.Calibration().Normalize(r.SolutionResult.Time).Nanoseconds()
		result.Award(test.Points, s.Draft.Grading)
	} else {
		r, err := e.sandbox.RunSolution(ctx, sol.executable, sol.language, sandbox.FileID(inputID), s.Draft.Streams(), e.solutionLimits(limit))
		if err != nil {
			return result, err
		}
		defer e.release(r.OutputID)
		outputID, result.Verdict, result.Time = r.OutputID, string(r.Status), e.pipeline.Calibration().Normalize(r.Time).Nanoseconds()
	}
	if s.Draft.Checker != nil && result.Verdict == string(sandbox.StatusOK) {
		checker := e.compiled["checker"]
		r, err := e.sandbox.CheckFiles(ctx, s.Draft.Checker.Jury(checker.executable, checker.language).With(args), sandbox.FileID(inputID), sandbox.FileID(outputID), sandbox.FileID(answerID), aux)
		if err != nil {
			return result, err
		}
		result.Verdict, result.Score = string(r.Status), r.Score
		result.Award(test.Points, s.Draft.Grading)
	}
	return result, nil
}

// checkSolutions records what every solution got and fails the build when one misses its expectation.
func (e *execution) checkSolutions(ctx context.Context) error {
	d := e.build.Snapshot.Draft
	results := []model.SolutionResult{}
	mismatches := []string{}
	for _, sol := range d.Solutions {
		if !sol.Runs() {
			continue
		}
		tests := e.solutions[sol.Name]
		result := model.SolutionResult{Name: sol.Name, Verdict: string(sandbox.StatusOK)}
		verdicts := make([]model.TestVerdict, len(tests))
		for i, t := range tests {
			verdicts[i] = model.TestVerdict{Ordinal: t.TestNumber, Verdict: t.Verdict}
			result.TimeMs = max(result.TimeMs, t.Time/int64(time.Millisecond))
			if result.Test == nil && t.Verdict != string(sandbox.StatusOK) {
				result.Verdict, result.Test = t.Verdict, &t.TestNumber
			}
		}
		if e.uncompiled[sol.Name] {
			result.Verdict = string(sandbox.StatusCE)
		} else if d.Grading == spec.GradingPoints {
			score := judging.NewVerdictCalculator(e.pkg.Manifest).Calculate(tests).Score
			result.Points = &score
		}
		result.Mismatch = sol.Mismatch(!e.uncompiled[sol.Name], verdicts, result.Points)
		if result.Mismatch != "" && sol.MustPass() && d.Grading == spec.GradingPoints {
			result.Mismatch += "; a partial solution should expect its score instead"
		}
		if result.Mismatch == "" {
			result.Mismatch = e.timing[sol.Name]
		} else if note := e.timing[sol.Name]; note != "" {
			result.Mismatch += "; " + note
		}
		if result.Mismatch != "" {
			mismatches = append(mismatches, fmt.Sprintf("solution %s: %s", sol.Name, result.Mismatch))
		}
		results = append(results, result)
	}
	if err := e.pipeline.Repo.RecordBuildSolutions(ctx, e.build.ID, *e.build.LeaseToken, results); err != nil {
		return fail("solutions", 0, "infrastructure", err)
	}
	if e.timeFailure != nil {
		mismatches = append(mismatches, e.timeFailure.Error())
	}
	if len(mismatches) > 0 {
		return fail("solutions", 0, "author", errors.New(strings.Join(mismatches, "\n")))
	}
	return nil
}

func (e *execution) checkPrograms(ctx context.Context) error {
	d := e.build.Snapshot.Draft
	aux := e.build.Snapshot.Limits.Auxiliary
	uploaded := map[uuid.UUID]string{}
	defer func() {
		for _, id := range uploaded {
			e.release(id)
		}
	}()
	file := func(oid uuid.UUID) (sandbox.File, error) {
		if id, ok := uploaded[oid]; ok {
			return sandbox.FileID(id), nil
		}
		id, err := e.upload(ctx, "objects/"+oid.String())
		if err != nil {
			return sandbox.File{}, err
		}
		uploaded[oid] = id
		return sandbox.FileID(id), nil
	}
	results := []model.ProgramTestResult{}
	mismatches := []string{}
	if d.Checker != nil {
		checker := e.compiled["checker"]
		for i := range d.Checker.Tests {
			t := &d.Checker.Tests[i]
			var files [3]sandbox.File
			for j, oid := range []uuid.UUID{t.Input, t.Output, t.Answer} {
				f, err := file(oid)
				if err != nil {
					return fail("program-tests", 0, "infrastructure", fmt.Errorf("checker test %d: %w", i+1, err))
				}
				files[j] = f
			}
			r, err := e.sandbox.CheckFiles(ctx, d.Checker.Jury(checker.executable, checker.language), files[0], files[1], files[2], aux)
			if err != nil {
				return fail("program-tests", 0, "infrastructure", fmt.Errorf("checker test %d: %w", i+1, err))
			}
			result := t.Check(string(r.Status), r.Score)
			if result.Mismatch != "" {
				mismatches = append(mismatches, fmt.Sprintf("checker test %d: %s", i+1, result.Mismatch))
			}
			results = append(results, result)
		}
	}
	for _, v := range d.Validators {
		validator := e.compiled[programKey("validator", v.Name)]
		for i := range v.Tests {
			t := &v.Tests[i]
			input, err := file(t.Input)
			if err != nil {
				return fail("program-tests", 0, "infrastructure", fmt.Errorf("validator %s test %d: %w", v.Name, i+1, err))
			}
			r, err := e.sandbox.ValidateFiles(ctx, v.Jury(validator.executable, validator.language), input, t.Group, aux)
			if err != nil {
				return fail("program-tests", 0, "infrastructure", fmt.Errorf("validator %s test %d: %w", v.Name, i+1, err))
			}
			if r.Status != sandbox.StatusOK && r.Status != sandbox.StatusWA {
				return fail("program-tests", 0, string(r.Status), fmt.Errorf("validator %s test %d: %s", v.Name, i+1, r.Message))
			}
			result := t.Check(r.Status == sandbox.StatusOK)
			if result.Mismatch != "" {
				mismatches = append(mismatches, fmt.Sprintf("validator %s test %d: %s", v.Name, i+1, result.Mismatch))
			}
			results = append(results, result)
		}
	}
	if err := e.pipeline.Repo.RecordBuildProgramTests(ctx, e.build.ID, *e.build.LeaseToken, results); err != nil {
		return fail("program-tests", 0, "infrastructure", err)
	}
	if len(mismatches) > 0 {
		return fail("program-tests", 0, "author", errors.New(strings.Join(mismatches, "\n")))
	}
	return nil
}

const maxSampleBytes = 128 * 1024

// Generated samples become available only after materialization.
func (e *execution) sample(inputPath, answerPath string) (model.Sample, error) {
	input, err := e.readSample(inputPath)
	if err != nil {
		return model.Sample{}, err
	}
	answer, err := e.readSample(answerPath)
	if err != nil {
		return model.Sample{}, err
	}
	return model.Sample{Input: input, Output: answer}, nil
}
func (e *execution) readSample(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	f, err := os.Open(filepath.Join(e.dir.root, path))
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxSampleBytes+1))
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	if len(data) > maxSampleBytes {
		data = append(data[:maxSampleBytes], []byte("\n…")...)
	}
	// PostgreSQL JSON text holds neither NUL nor invalid UTF-8.
	return strings.ReplaceAll(strings.ToValidUTF8(string(data), "�"), "\x00", "�"), nil
}

func diagnostic(err error) json.RawMessage {
	var failure *Failure
	if !errors.As(err, &failure) {
		failure = &Failure{Stage: "pipeline", Kind: "infrastructure"}
	}
	if failure.Kind == "infrastructure" {
		// Authors see the diagnostic; the cause, with paths and tokens, is only in the worker's log.
		redacted := *failure
		redacted.Message, redacted.Truncated = "internal error", false
		failure = &redacted
	}
	if len(failure.Message) > 8192 {
		failure.Message = strings.ToValidUTF8(failure.Message[:8192], "�")
		failure.Truncated = true
	}
	data, _ := json.Marshal(failure)
	return data
}
