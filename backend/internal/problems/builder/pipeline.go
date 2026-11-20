package builder

import (
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
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/internal/judging"
	"github.com/brawler2011/gate/backend/internal/problems/archive"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
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
	BuildInputObjects(context.Context, uuid.UUID) ([]model.StoredObject, error)
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
	Files     *sandbox.FileTransport
	Languages *sandbox.Config
	Programs  *sandbox.ProgramCache
	Logger    *slog.Logger
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
	e := &execution{pipeline: p, build: b, dir: &directory{root: work, remaining: int64(s.Limits.DiskBytes), maxFiles: s.Limits.FileCount}, sandbox: sandbox.NewSandbox(p.Client, p.Languages), tests: testFiles(s.Draft), compiled: make(map[string]compilation), samples: []model.Sample{}, solutions: map[string][]judging.TestResult{}, uncompiled: map[string]bool{}}
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
	for _, test := range s.Draft.Tests {
		if err := e.test(ctx, test); err != nil {
			return uuid.Nil, err
		}
	}
	if err := e.checkSolutions(ctx); err != nil {
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
	pid, err := p.Repo.PublishBuild(ctx, b.ID, *b.LeaseToken, object, model.NewPackageIndex(s.Draft, e.samples))
	if err != nil {
		return uuid.Nil, fail("publish", 0, "infrastructure", err)
	}
	return pid, nil
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
	objects, err := e.pipeline.Repo.BuildInputObjects(ctx, e.build.ID)
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
	for _, p := range programs(d, e.pipeline.Languages) {
		if !required[p.key] && !judged[p.key] {
			continue
		}
		// Judge programs compile with the whole library; solutions alone.
		deps := map[string][]byte{}
		if p.role != "solution" {
			for i := range d.Library {
				deps[d.Library[i].Name] = []byte(d.Library[i].Text())
			}
		}
		compiled, err := e.pipeline.Programs.Get(ctx, sandbox.Program{Source: []byte(p.program.Source), Language: p.program.Language, Deps: deps}, s.Limits.Compile)
		if err != nil && judged[p.key] && processKind(err) == "author" {
			e.uncompiled[strings.TrimPrefix(p.key, "solution/")] = true
			continue
		}
		if err != nil {
			return fail("compile", 0, processKind(err), fmt.Errorf("%s: %w", p.path, err))
		}
		executable, err := e.pipeline.Files.UploadCompiled(ctx, compiled.Dir(), int64(s.Limits.Compile.Output))
		compiled.Release()
		if err != nil {
			return fail("compile", 0, "infrastructure", fmt.Errorf("%s: %w", p.path, err))
		}
		e.compiled[p.key] = compilation{language: p.program.Language, executable: executable}
	}
	return nil
}
func (e *execution) solutionLimits() sandbox.FileRunLimits {
	d := e.build.Snapshot.Draft
	limits := e.build.Snapshot.Limits.Auxiliary
	limits.CPU = time.Duration(d.TimeLimitMs) * time.Millisecond
	limits.Wall = limits.CPU * 2
	limits.Memory = bytesize.Size(d.MemoryLimitMb) * bytesize.MiB
	limits.Output = e.build.Snapshot.Limits.TestOutputBytes
	return limits
}
func (e *execution) test(ctx context.Context, test model.DraftTest) error {
	s := e.build.Snapshot
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
			return fail("input", test.Ordinal, "infrastructure", err)
		}
		inputID = id
		ids = append(ids, id)
	} else {
		gen := e.compiled[programKey("generator", *test.Input.Generator)]
		result, err := e.sandbox.RunFiles(ctx, gen.executable, gen.language, sandbox.File{}, test.Input.Args, nil, aux)
		if err != nil {
			return fail("generate", test.Ordinal, "infrastructure", err)
		}
		ids = append(ids, result.OutputID)
		if result.Status != sandbox.StatusOK || result.ExitStatus != 0 {
			return fail("generate", test.Ordinal, string(result.Status), fmt.Errorf("exit %d: %s", result.ExitStatus, result.Stderr))
		}
		inputID = result.OutputID
		if err := e.download(ctx, files.Input, inputID, int64(s.Limits.ObjectBytes)); err != nil {
			return fail("generate", test.Ordinal, "infrastructure", err)
		}
	}
	var args []string
	if g := s.Draft.Subtask(test.SubtaskID); g != nil && g.Name != "" {
		args = []string{"--group", g.Name}
	}
	for _, validator := range s.Draft.Validators {
		if !validator.Applies(test.SubtaskID) {
			continue
		}
		v := e.compiled[programKey("validator", validator.Name)]
		result, err := e.sandbox.RunFiles(ctx, v.executable, v.language, sandbox.FileID(inputID), args, nil, aux)
		if err != nil {
			return fail("validate", test.Ordinal, "infrastructure", fmt.Errorf("validator %s: %w", validator.Name, err))
		}
		ids = append(ids, result.OutputID)
		if result.Status != sandbox.StatusOK || result.ExitStatus != 0 {
			return fail("validate", test.Ordinal, string(result.Status), fmt.Errorf("validator %s: exit %d: %s", validator.Name, result.ExitStatus, result.Stderr))
		}
	}
	var outputID, answerID string
	if s.Draft.Type == "interactive" {
		// The main solution's interactor output becomes the answer for later judging.
		sol := e.compiled[programKey("solution", *s.Draft.MainSolution)]
		intr := e.compiled["interactor"]
		result, err := e.sandbox.InteractFiles(ctx, sol.executable, sol.language, intr.executable, intr.language, sandbox.FileID(inputID), sandbox.File{}, e.solutionLimits(), aux)
		if err != nil {
			return fail("interact", test.Ordinal, "infrastructure", err)
		}
		ids = append(ids, result.OutputID)
		if result.Status != sandbox.StatusOK {
			return fail("interact", test.Ordinal, string(result.Status), errors.New(result.Message))
		}
		outputID, answerID = result.OutputID, result.OutputID
		if err := e.download(ctx, files.Answer, answerID, int64(s.Limits.ObjectBytes)); err != nil {
			return fail("interact", test.Ordinal, "infrastructure", err)
		}
	} else {
		if test.Answer.Mode == "provided" {
			id, err := e.upload(ctx, files.Answer)
			if err != nil {
				return fail("answer", test.Ordinal, "infrastructure", err)
			}
			answerID = id
			ids = append(ids, id)
		}
		solution := test.Answer.Solution
		if test.Answer.Mode == "provided" {
			solution = s.Draft.MainSolution
		}
		outputID = answerID
		if solution != nil {
			sol := e.compiled[programKey("solution", *solution)]
			result, err := e.sandbox.RunSolution(ctx, sol.executable, sol.language, sandbox.FileID(inputID), s.Draft.Streams(), e.solutionLimits())
			if err != nil {
				return fail("answer", test.Ordinal, "infrastructure", err)
			}
			ids = append(ids, result.OutputID)
			if result.Status != sandbox.StatusOK || result.ExitStatus != 0 {
				return fail("answer", test.Ordinal, string(result.Status), fmt.Errorf("exit %d: %s", result.ExitStatus, result.Stderr))
			}
			outputID = result.OutputID
			if test.Answer.Mode == "generated" {
				answerID = outputID
				if err := e.download(ctx, files.Answer, answerID, int64(s.Limits.ObjectBytes)); err != nil {
					return fail("answer", test.Ordinal, "infrastructure", err)
				}
			}
		}
	}
	if s.Draft.Checker != nil {
		checker := e.compiled["checker"]
		result, err := e.sandbox.CheckFiles(ctx, checker.executable, checker.language, sandbox.FileID(inputID), sandbox.FileID(outputID), sandbox.FileID(answerID), aux)
		if err != nil {
			return fail("check", test.Ordinal, "infrastructure", err)
		}
		if result.Status != sandbox.StatusOK {
			return fail("check", test.Ordinal, string(result.Status), errors.New(result.Message))
		}
	}
	for _, sol := range s.Draft.Solutions {
		if !sol.Runs() || e.uncompiled[sol.Name] {
			continue
		}
		result, err := e.judge(ctx, sol.Name, test.Ordinal, inputID, answerID)
		if err != nil {
			return fail("solutions", test.Ordinal, "infrastructure", fmt.Errorf("solution %s: %w", sol.Name, err))
		}
		e.solutions[sol.Name] = append(e.solutions[sol.Name], result)
	}
	if test.Sample {
		sampleAnswer := files.Answer
		if s.Draft.Type == "interactive" {
			sampleAnswer = "" // the interactor's output is no sample output
		}
		if err := e.addSample(files.Input, sampleAnswer); err != nil {
			return fail("sample", test.Ordinal, "infrastructure", err)
		}
	}
	return nil
}

// judge runs a solution on a test as the judge runs submissions.
func (e *execution) judge(ctx context.Context, name string, ordinal int, inputID, answerID string) (judging.TestResult, error) {
	s := e.build.Snapshot
	aux := s.Limits.Auxiliary
	sol := e.compiled[programKey("solution", name)]
	result := judging.TestResult{TestNumber: ordinal}
	var outputID string
	if s.Draft.Type == "interactive" {
		intr := e.compiled["interactor"]
		r, err := e.sandbox.InteractFiles(ctx, sol.executable, sol.language, intr.executable, intr.language, sandbox.FileID(inputID), sandbox.FileID(answerID), e.solutionLimits(), aux)
		if err != nil {
			return result, err
		}
		defer e.release(r.OutputID)
		outputID, result.Verdict, result.Time = r.OutputID, string(r.Status), r.SolutionResult.Time.Nanoseconds()
	} else {
		r, err := e.sandbox.RunSolution(ctx, sol.executable, sol.language, sandbox.FileID(inputID), s.Draft.Streams(), e.solutionLimits())
		if err != nil {
			return result, err
		}
		defer e.release(r.OutputID)
		outputID, result.Verdict, result.Time = r.OutputID, string(r.Status), r.Time.Nanoseconds()
	}
	if s.Draft.Checker != nil && result.Verdict == string(sandbox.StatusOK) {
		checker := e.compiled["checker"]
		r, err := e.sandbox.CheckFiles(ctx, checker.executable, checker.language, sandbox.FileID(inputID), sandbox.FileID(outputID), sandbox.FileID(answerID), aux)
		if err != nil {
			return result, err
		}
		result.Verdict = string(r.Status)
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
		} else if d.Type == "scoring" {
			score := judging.NewVerdictCalculator(d.Type, e.pkg.Manifest).Calculate(tests).Score
			result.Points = &score
		}
		result.Mismatch = sol.Mismatch(!e.uncompiled[sol.Name], verdicts, result.Points)
		if result.Mismatch != "" {
			mismatches = append(mismatches, fmt.Sprintf("solution %s: %s", sol.Name, result.Mismatch))
		}
		results = append(results, result)
	}
	if err := e.pipeline.Repo.RecordBuildSolutions(ctx, e.build.ID, *e.build.LeaseToken, results); err != nil {
		return fail("solutions", 0, "infrastructure", err)
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
			r, err := e.sandbox.CheckFiles(ctx, checker.executable, checker.language, files[0], files[1], files[2], aux)
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
			var args []string
			if t.Group != "" {
				args = []string{"--group", t.Group}
			}
			r, err := e.sandbox.RunFiles(ctx, validator.executable, validator.language, input, args, nil, aux)
			if err != nil {
				return fail("program-tests", 0, "infrastructure", fmt.Errorf("validator %s test %d: %w", v.Name, i+1, err))
			}
			e.release(r.OutputID)
			// A validator rejects with a non-zero exit; running out of limits is no answer.
			if r.Status != sandbox.StatusOK && r.Status != sandbox.StatusRE {
				return fail("program-tests", 0, string(r.Status), fmt.Errorf("validator %s test %d: %s", v.Name, i+1, r.Stderr))
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
func (e *execution) addSample(inputPath, answerPath string) error {
	input, err := e.readSample(inputPath)
	if err != nil {
		return err
	}
	answer, err := e.readSample(answerPath)
	if err != nil {
		return err
	}
	e.samples = append(e.samples, model.Sample{Input: input, Output: answer})
	return nil
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
		// Authors see the diagnostic; the cause is in the worker's log.
		failure = &Failure{Stage: "pipeline", Kind: "infrastructure", Message: "internal error"}
	}
	if len(failure.Message) > 8192 {
		failure.Message = strings.ToValidUTF8(failure.Message[:8192], "�")
		failure.Truncated = true
	}
	data, _ := json.Marshal(failure)
	return data
}
