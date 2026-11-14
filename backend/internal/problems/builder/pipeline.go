package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	RegisterBuildOutput(context.Context, uuid.UUID, uuid.UUID, string, int64, time.Duration) (model.StoredObject, error)
	PublishBuild(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, []model.Sample) (uuid.UUID, error)
	FinishBuildAttempt(context.Context, uuid.UUID, uuid.UUID, json.RawMessage, bool) error
}
type Pipeline struct {
	Repo        BuildRepository
	Store       ObjectStore
	Bucket      string
	TempDir     string
	Client      *sandbox.Client
	Files       *sandbox.FileTransport
	Environment model.Toolchain
	UploadTTL   time.Duration
	Logger      *slog.Logger
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
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled, codes.ResourceExhausted:
		return "infrastructure"
	}
	return "author"
}

type compilation struct {
	source     model.DraftComponent
	executable sandbox.Executable
}
type execution struct {
	pipeline *Pipeline
	build    model.ProblemBuild
	dir      *directory
	sandbox  *sandbox.Sandbox
	compiled map[uuid.UUID]compilation
	manifest *gfmt.Problem
	samples  []model.Sample
}

func (p *Pipeline) Execute(ctx context.Context, b model.ProblemBuild) (uuid.UUID, error) {
	s := b.Snapshot
	if s.SchemaVersion != model.DraftSnapshotVersion || s.Toolchain.Digest != p.Environment.Digest || s.Toolchain.Architecture != p.Environment.Architecture || s.Toolchain.ABI != p.Environment.ABI {
		return uuid.Nil, fail("environment", 0, "infrastructure", fmt.Errorf("pinned toolchain is unavailable"))
	}
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
	e := &execution{pipeline: p, build: b, dir: &directory{root: tree, remaining: int64(s.Limits.DiskBytes), maxFiles: s.Limits.FileCount}, sandbox: sandbox.NewSandbox(p.Client, &sandbox.Config{Langs: s.Toolchain.Languages}), compiled: make(map[uuid.UUID]compilation), samples: []model.Sample{}}
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
	object, err := p.Repo.RegisterBuildOutput(ctx, b.ID, *b.LeaseToken, digest, size, p.UploadTTL)
	if err != nil {
		return uuid.Nil, fail("register", 0, "infrastructure", err)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return uuid.Nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	if err := p.Store.UploadFileWithSHA256(ctx, p.Bucket, object.Key, io.NewSectionReader(f, 0, size), "application/octet-stream", digest); err != nil {
		return uuid.Nil, fail("upload", 0, "infrastructure", err)
	}
	pid, err := p.Repo.PublishBuild(ctx, b.ID, *b.LeaseToken, object.ID, e.samples)
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
			refs = append(refs, fmt.Sprintf("asset %q", asset.Path))
		}
	}
	return fmt.Sprintf("object %s (%s)", id, strings.Join(refs, ", "))
}

func (e *execution) materialize(ctx context.Context) error {
	s := e.build.Snapshot
	for path, text := range s.Draft.Files {
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
		if !expected[o.ID] || o.State != "available" || o.OrganizationID != s.OrganizationID || o.Size > int64(s.Limits.ObjectBytes) {
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
	for _, a := range s.Draft.Assets {
		f, err := os.Open(filepath.Join(e.dir.root, ".gate/objects/"+a.ObjectID.String()))
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}
		_, _, copyErr := e.dir.copy(a.Path, f, int64(s.Limits.ObjectBytes))
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
	m := &gfmt.Problem{FormatVersion: "2", Title: d.Title, Type: d.Type, MaxScore: d.MaxScore, Limits: gfmt.Limits{TimeMs: d.TimeLimitMs, MemoryMb: d.MemoryLimitMb}, ToolchainDigest: s.Toolchain.Digest, Architecture: s.Toolchain.Architecture, ABI: s.Toolchain.ABI, Languages: map[string]gfmt.LanguageConfig{}, Compiled: map[string]gfmt.CompiledComponent{}, Subtasks: map[string]gfmt.Subtask{}, Solutions: map[string]string{}, Sources: map[string]gfmt.Source{}}
	for key, lang := range s.Toolchain.Languages {
		m.Languages[key] = gfmt.LanguageConfig{Type: lang.Type, Compile: lang.Compile, CodeFile: lang.CodeFile, CompileFile: lang.CompileFile, Execute: lang.Execute}
	}
	for _, g := range d.Subtasks {
		deps := []string{}
		for _, id := range g.DependsOn {
			deps = append(deps, id.String())
		}
		m.Subtasks[g.ID.String()] = gfmt.Subtask{Ordinal: g.Ordinal, Points: g.Points, Policy: g.Policy, Dependencies: deps, Tests: []gfmt.Test{}}
	}
	if d.CheckerID != nil {
		m.Checker = d.CheckerID.String()
	}
	if d.ValidatorID != nil {
		m.Validator = d.ValidatorID.String()
	}
	if d.InteractorID != nil {
		m.Interactor = d.InteractorID.String()
	}
	if d.MainSolutionID != nil {
		m.Solutions["main"] = d.MainSolutionID.String()
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
	required := map[uuid.UUID]bool{}
	for _, id := range []*uuid.UUID{s.Draft.CheckerID, s.Draft.ValidatorID, s.Draft.InteractorID, s.Draft.MainSolutionID} {
		if id != nil {
			required[*id] = true
		}
	}
	for _, test := range s.Draft.Tests {
		if test.Input.ComponentID != nil {
			required[*test.Input.ComponentID] = true
		}
		if test.Answer.ComponentID != nil {
			required[*test.Answer.ComponentID] = true
		}
	}
	for _, c := range s.Draft.Components {
		if !required[c.ID] {
			continue
		}
		deps := map[string][]byte{}
		for destination, path := range c.Dependencies {
			deps[destination] = []byte(s.Draft.Files[path])
		}
		executable, err := e.sandbox.CompileWithLimits(ctx, []byte(s.Draft.Files[c.Path]), c.Language, deps, s.Limits.Compile)
		if err != nil {
			return fail("compile", 0, processKind(err), fmt.Errorf("%s: %w", c.Path, err))
		}
		e.compiled[c.ID] = compilation{source: c, executable: executable}
		entry := ".gate/bin/" + c.ID.String() + "/entry"
		if err := e.download(ctx, entry, executable.PrimaryFileID, int64(s.Limits.Compile.Output)); err != nil {
			return fail("export", 0, "infrastructure", err)
		}
		artifact := gfmt.CompiledComponent{Role: c.Role, Language: c.Language, Entry: entry, Dependencies: map[string]string{}}
		for path, id := range executable.Dependencies {
			dest := ".gate/bin/" + c.ID.String() + "/dependencies/" + path
			if err := e.download(ctx, dest, id, int64(s.Limits.Compile.Output)); err != nil {
				return fail("export", 0, "infrastructure", err)
			}
			artifact.Dependencies[path] = dest
		}
		e.manifest.Compiled[c.ID.String()] = artifact
		e.manifest.Sources[c.Path] = gfmt.Source{Compiler: c.Language, DependencyMap: c.Dependencies}
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
		gen := e.compiled[*test.Input.ComponentID]
		args := append(append([]string{}, test.Input.Args...), strconv.FormatInt(*test.Input.Seed, 10))
		result, err := e.sandbox.RunFiles(ctx, gen.executable, gen.source.Language, "", args, nil, aux)
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
	if s.Draft.ValidatorID != nil {
		v := e.compiled[*s.Draft.ValidatorID]
		result, err := e.sandbox.RunFiles(ctx, v.executable, v.source.Language, inputID, nil, nil, aux)
		if err != nil {
			return fail("validate", test.Ordinal, "infrastructure", err)
		}
		ids = append(ids, result.OutputID)
		if result.Status != sandbox.StatusOK || result.ExitStatus != 0 {
			return fail("validate", test.Ordinal, string(result.Status), fmt.Errorf("exit %d: %s", result.ExitStatus, result.Stderr))
		}
	}
	answerPath := ""
	if s.Draft.Type == "interactive" {
		sol := e.compiled[*s.Draft.MainSolutionID]
		intr := e.compiled[*s.Draft.InteractorID]
		result, err := e.sandbox.InteractFiles(ctx, sol.executable, sol.source.Language, intr.executable, intr.source.Language, inputID, e.solutionLimits(), aux)
		if err != nil {
			return fail("interact", test.Ordinal, "infrastructure", err)
		}
		if result.Status != sandbox.StatusOK {
			return fail("interact", test.Ordinal, string(result.Status), errors.New(result.Message))
		}
	} else {
		var answerID string
		if test.Answer.Mode == "provided" {
			answerPath = ".gate/objects/" + test.Answer.ObjectID.String()
			id, err := e.upload(ctx, answerPath)
			if err != nil {
				return fail("answer", test.Ordinal, "infrastructure", err)
			}
			answerID = id
			ids = append(ids, id)
		}
		solID := test.Answer.ComponentID
		if test.Answer.Mode == "provided" {
			solID = s.Draft.MainSolutionID
		}
		outputID := answerID
		if solID != nil {
			sol := e.compiled[*solID]
			result, err := e.sandbox.RunFiles(ctx, sol.executable, sol.source.Language, inputID, nil, nil, e.solutionLimits())
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
		checker := e.compiled[*s.Draft.CheckerID]
		result, err := e.sandbox.CheckFiles(ctx, checker.executable, checker.source.Language, inputID, outputID, answerID, aux)
		if err != nil {
			return fail("check", test.Ordinal, "infrastructure", err)
		}
		if result.Status != sandbox.StatusOK {
			return fail("check", test.Ordinal, string(result.Status), errors.New(result.Message))
		}
	}
	if test.Sample {
		if err := e.addSample(inputPath, answerPath); err != nil {
			return fail("sample", test.Ordinal, "infrastructure", err)
		}
	}
	group := e.manifest.Subtasks[test.SubtaskID.String()]
	group.Tests = append(group.Tests, gfmt.Test{Ordinal: test.Ordinal, Points: test.Points, Manual: inputPath, Answer: answerPath, AnswerMode: test.Answer.Mode, Sample: test.Sample})
	e.manifest.Subtasks[test.SubtaskID.String()] = group
	return nil
}

// maxSampleBytes bounds each sample file shown in a statement.
const maxSampleBytes = 128 * 1024

// addSample records a sample test for the statement, which is stored with the
// Package because generated inputs exist only after the build.
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
