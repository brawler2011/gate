package builder

import (
	"context"
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

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
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
	PublishBuild(context.Context, uuid.UUID, uuid.UUID, model.StoredObject, []model.Sample) (uuid.UUID, error)
	FinishBuildAttempt(context.Context, uuid.UUID, uuid.UUID, json.RawMessage, bool) error
}
type Pipeline struct {
	Repo    BuildRepository
	Store   ObjectStore
	Bucket  string
	TempDir string
	Client  *sandbox.Client
	Files   *sandbox.FileTransport
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
	layout   packageLayout
	compiled map[string]compilation
	manifest *gfmt.Problem
	samples  []model.Sample
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
	tree := filepath.Join(root, "tree")
	if err := os.Mkdir(tree, 0700); err != nil {
		return uuid.Nil, fmt.Errorf("mkdir: %w", err)
	}
	e := &execution{pipeline: p, build: b, dir: &directory{root: tree, remaining: int64(s.Limits.DiskBytes), maxFiles: s.Limits.FileCount}, sandbox: sandbox.NewSandbox(p.Client, p.Languages), layout: placeDraft(s.Draft, p.Languages), compiled: make(map[string]compilation), samples: []model.Sample{}}
	defer e.releaseComponents()
	if err := e.materialize(ctx); err != nil {
		return uuid.Nil, fail("materialize", 0, "infrastructure", err)
	}
	e.makeManifest()
	if err := e.compile(ctx); err != nil {
		return uuid.Nil, err
	}
	for _, test := range s.Draft.Tests {
		if err := e.test(ctx, test); err != nil {
			return uuid.Nil, err
		}
	}
	data, err := yaml.Marshal(e.manifest)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal: %w", err)
	}
	if err := e.dir.write("problem.yaml", data); err != nil {
		return uuid.Nil, fail("manifest", 0, "author", err)
	}
	if _, err := gfmt.OpenBuiltPackage(tree); err != nil {
		return uuid.Nil, fail("package", 0, "author", err)
	}
	archivePath := filepath.Join(root, "package.zip")
	digest, size, err := e.dir.archive(ctx, archivePath)
	if err != nil {
		return uuid.Nil, fail("archive", 0, "infrastructure", err)
	}
	object := model.NewStoredObject(b.OrganizationID, model.UploadSpec{Purpose: "package", SHA256: digest, Size: size})
	f, err := os.Open(archivePath)
	if err != nil {
		return uuid.Nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	if err := p.Store.UploadFileWithSHA256(ctx, p.Bucket, object.Key, io.NewSectionReader(f, 0, size), "application/octet-stream", digest); err != nil {
		return uuid.Nil, fail("upload", 0, "infrastructure", err)
	}
	pid, err := p.Repo.PublishBuild(ctx, b.ID, *b.LeaseToken, object, e.samples)
	if err != nil {
		return uuid.Nil, fail("publish", 0, "infrastructure", err)
	}
	return pid, nil
}

func objectReferences(d model.Draft, id uuid.UUID) string {
	refs := []string{}
	for _, test := range d.Tests {
		if test.Input.ObjectID != nil && *test.Input.ObjectID == id {
			refs = append(refs, fmt.Sprintf("test %d input", test.Ordinal))
		}
		if test.Answer.ObjectID != nil && *test.Answer.ObjectID == id {
			refs = append(refs, fmt.Sprintf("test %d answer", test.Ordinal))
		}
	}
	for _, asset := range d.Assets {
		if asset.ObjectID == id {
			refs = append(refs, fmt.Sprintf("asset %q", asset.Name))
		}
	}
	return fmt.Sprintf("object %s (%s)", id, strings.Join(refs, ", "))
}

func (e *execution) materialize(ctx context.Context) error {
	s := e.build.Snapshot
	for path, text := range e.layout.texts {
		if err := e.dir.write(path, []byte(text)); err != nil {
			return err
		}
	}
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
		digest, n, copyErr := e.dir.copy(".gate/objects/"+o.ID.String(), r, o.Size)
		closeErr := r.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return fmt.Errorf("%s: %w", objectReferences(s.Draft, o.ID), err)
		}
		if n != o.Size || digest != o.SHA256 {
			return fmt.Errorf("%s: snapshot object bytes differ from registry", objectReferences(s.Draft, o.ID))
		}
	}
	for path, object := range e.layout.assets {
		f, err := os.Open(filepath.Join(e.dir.root, ".gate/objects/"+object.String()))
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		_, _, copyErr := e.dir.copy(path, f, int64(s.Limits.ObjectBytes))
		closeErr := f.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
	}
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return e.dir.write(".gate/snapshot.json", data)
}
func (e *execution) makeManifest() {
	s := e.build.Snapshot
	d := s.Draft
	m := &gfmt.Problem{FormatVersion: "2", Title: d.Title, Type: d.Type, MaxScore: d.MaxScore, Limits: gfmt.Limits{TimeMs: d.TimeLimitMs, MemoryMb: d.MemoryLimitMb}, Programs: map[string]gfmt.Program{}, Subtasks: map[string]gfmt.Subtask{}, Solutions: map[string]string{}}
	for _, g := range d.Subtasks {
		deps := []string{}
		for _, id := range g.DependsOn {
			deps = append(deps, id.String())
		}
		m.Subtasks[g.ID.String()] = gfmt.Subtask{Ordinal: g.Ordinal, Points: g.Points, Policy: g.Policy, Dependencies: deps, Tests: []gfmt.Test{}}
	}
	if d.MainSolution != nil {
		if p := e.layout.program(programKey("solution", *d.MainSolution)); p != nil {
			m.Solutions["main"] = filepath.Base(p.path)
		}
	}
	e.manifest = m
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
	for _, p := range e.layout.programs {
		if !required[p.key] {
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
		if err != nil {
			return fail("compile", 0, processKind(err), fmt.Errorf("%s: %w", p.path, err))
		}
		executable, err := e.pipeline.Files.UploadCompiled(ctx, compiled.Dir(), int64(s.Limits.Compile.Output))
		compiled.Release()
		if err != nil {
			return fail("compile", 0, "infrastructure", fmt.Errorf("%s: %w", p.path, err))
		}
		e.compiled[p.key] = compilation{language: p.program.Language, executable: executable}
		if p.role == "checker" || p.role == "interactor" {
			e.manifest.Programs[p.role] = gfmt.Program{Source: p.path, Language: p.program.Language}
		}
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
	inputPath := ".gate/objects/"
	var inputID string
	if test.Input.Mode == "manual" {
		inputPath += test.Input.ObjectID.String()
		id, err := e.upload(ctx, inputPath)
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
		inputPath = ".gate/tests/" + test.ID.String() + ".in"
		if err := e.download(ctx, inputPath, inputID, int64(s.Limits.ObjectBytes)); err != nil {
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
	answerPath := ""
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
		answerPath = ".gate/tests/" + test.ID.String() + ".ans"
		if err := e.download(ctx, answerPath, answerID, int64(s.Limits.ObjectBytes)); err != nil {
			return fail("interact", test.Ordinal, "infrastructure", err)
		}
	} else {
		if test.Answer.Mode == "provided" {
			answerPath = ".gate/objects/" + test.Answer.ObjectID.String()
			id, err := e.upload(ctx, answerPath)
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
			result, err := e.sandbox.RunFiles(ctx, sol.executable, sol.language, sandbox.FileID(inputID), nil, nil, e.solutionLimits())
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
				answerPath = ".gate/tests/" + test.ID.String() + ".ans"
				if err := e.download(ctx, answerPath, answerID, int64(s.Limits.ObjectBytes)); err != nil {
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
	if test.Sample {
		sampleAnswer := answerPath
		if s.Draft.Type == "interactive" {
			sampleAnswer = "" // the interactor's output is no sample output
		}
		if err := e.addSample(inputPath, sampleAnswer); err != nil {
			return fail("sample", test.Ordinal, "infrastructure", err)
		}
	}
	group := e.manifest.Subtasks[test.SubtaskID.String()]
	group.Tests = append(group.Tests, gfmt.Test{Ordinal: test.Ordinal, Points: test.Points, Manual: inputPath, Answer: answerPath, AnswerMode: test.Answer.Mode, Sample: test.Sample})
	e.manifest.Subtasks[test.SubtaskID.String()] = group
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
		failure = &Failure{Stage: "pipeline", Kind: "infrastructure", Message: err.Error()}
	}
	if len(failure.Message) > 8192 {
		failure.Message = strings.ToValidUTF8(failure.Message[:8192], "�")
		failure.Truncated = true
	}
	data, _ := json.Marshal(failure)
	return data
}
