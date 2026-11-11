package judging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type PackageJudge struct {
	Client      *sandbox.Client
	Files       *sandbox.FileTransport
	Environment model.Toolchain
	Logger      *slog.Logger
}

func (j *PackageJudge) release(id string) {
	if id == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := j.Files.Delete(ctx, id); err != nil {
		j.Logger.WarnContext(ctx, "release judge sandbox file", "error", err)
	}
}
func (j *PackageJudge) upload(ctx context.Context, root, path string, limit int64) (string, error) {
	f, err := os.Open(filepath.Join(root, path))
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	return j.Files.Upload(ctx, f, limit)
}
func (j *PackageJudge) restore(ctx context.Context, pkg *LoadedPackage, component gfmt.CompiledComponent) (sandbox.Executable, error) {
	executable := sandbox.Executable{Dependencies: map[string]string{}}
	id, err := j.upload(ctx, pkg.Format.Path, component.Entry, int64(pkg.Descriptor.Snapshot.Limits.Compile.Output))
	if err != nil {
		return executable, err
	}
	executable.PrimaryFileID = id
	for path, artifact := range component.Dependencies {
		id, err := j.upload(ctx, pkg.Format.Path, artifact, int64(pkg.Descriptor.Snapshot.Limits.Compile.Output))
		if err != nil {
			j.release(executable.PrimaryFileID)
			for _, id := range executable.Dependencies {
				j.release(id)
			}
			return sandbox.Executable{}, err
		}
		executable.Dependencies[path] = id
	}
	return executable, nil
}

// errEnvironmentMismatch means this worker cannot reproduce the package's pinned toolchain.
var errEnvironmentMismatch = errors.New("pinned judging environment unavailable")

func (j *PackageJudge) Judge(ctx context.Context, pkg *LoadedPackage, submissionID uuid.UUID, source string, language submissionsmodel.LanguageName, progress Progress, meta submissionsmodel.SubmissionEventMeta) (*FinalVerdict, error) {
	s := pkg.Descriptor.Snapshot
	if s.Toolchain.Digest != j.Environment.Digest || s.Toolchain.Architecture != j.Environment.Architecture || s.Toolchain.ABI != j.Environment.ABI {
		return nil, errEnvironmentMismatch
	}
	sb := sandbox.NewSandbox(j.Client, &sandbox.Config{Langs: s.Toolchain.Languages})
	lang := convertLanguage(language)
	sol, err := sb.CompileWithLimits(ctx, []byte(source), lang, nil, s.Limits.Compile)
	if err != nil {
		if IsTemporary(err) {
			return nil, err
		}
		output := err.Error()
		if len(output) > maxTestDetailSize {
			output = output[:maxTestDetailSize]
		}
		return &FinalVerdict{State: submissionsmodel.GotCE, Message: output, TestDetails: &submissionsmodel.SubmissionTestDetails{CompilerOutput: &output, ErrorLine: ParseErrorLine(language, output)}}, nil
	}
	defer func() {
		j.release(sol.PrimaryFileID)
		for _, id := range sol.Dependencies {
			j.release(id)
		}
	}()
	role := "checker"
	componentID := pkg.Format.Problem.Checker
	if s.Draft.Type == "interactive" {
		role = "interactor"
		componentID = pkg.Format.Problem.Interactor
	}
	component, ok := pkg.Format.Problem.Compiled[componentID]
	if !ok || component.Role != role {
		return nil, fmt.Errorf("missing published %s", role)
	}
	executable, err := j.restore(ctx, pkg, component)
	if err != nil {
		return nil, err
	}
	defer func() {
		j.release(executable.PrimaryFileID)
		for _, id := range executable.Dependencies {
			j.release(id)
		}
	}()
	if err := progress.PublishTestingStarted(ctx, submissionID, meta); err != nil {
		return nil, err
	}
	tests := collectFlatTests(pkg.Format.Problem)
	results := make([]TestResult, 0, len(tests))
	hasFailureDetails := false
	for _, test := range tests {
		if err := progress.PublishTestStarted(ctx, submissionID, safeInt32(test.TestIndex), meta); err != nil {
			return nil, err
		}
		result, err := j.test(ctx, sb, pkg, sol, lang, executable, component, test, !hasFailureDetails)
		if err != nil {
			return nil, err
		}
		if result.Verdict != "OK" && !hasFailureDetails {
			hasFailureDetails = true
		}
		results = append(results, result)
		if result.Verdict != "OK" && s.Draft.Type != "scoring" {
			break
		}
	}
	return NewVerdictCalculator(s.Draft.Type, pkg.Format.Problem).Calculate(results), nil
}
func (j *PackageJudge) test(ctx context.Context, sb *sandbox.Sandbox, pkg *LoadedPackage, sol sandbox.Executable, lang string, executable sandbox.Executable, component gfmt.CompiledComponent, test FlatTest, details bool) (TestResult, error) {
	s := pkg.Descriptor.Snapshot
	limits := s.Limits.Auxiliary
	limits.CPU = time.Duration(s.Draft.TimeLimitMs) * time.Millisecond
	limits.Wall = 2 * limits.CPU
	limits.Memory = bytesize.Size(s.Draft.MemoryLimitMb) * bytesize.MiB
	limits.Output = s.Limits.TestOutputBytes
	input, err := j.upload(ctx, pkg.Format.Path, test.Test.Manual, int64(s.Limits.ObjectBytes))
	if err != nil {
		return TestResult{}, err
	}
	defer j.release(input)
	result := TestResult{TestNumber: test.TestIndex}
	if s.Draft.Type == "interactive" {
		r, err := sb.InteractFiles(ctx, sol, lang, executable, component.Language, input, limits, s.Limits.Auxiliary)
		if err != nil {
			return result, err
		}
		result.Verdict = string(r.Status)
		result.Message = r.Message
		result.Time = r.SolutionResult.Time.Nanoseconds()
		result.Memory = r.SolutionResult.Memory
		result.Score = r.Score
	} else {
		answer, err := j.upload(ctx, pkg.Format.Path, test.Test.Answer, int64(s.Limits.ObjectBytes))
		if err != nil {
			return result, err
		}
		defer j.release(answer)
		r, err := sb.RunFiles(ctx, sol, lang, input, nil, nil, limits)
		if err != nil {
			return result, err
		}
		defer j.release(r.OutputID)
		result.Verdict = string(r.Status)
		result.Message = string(r.Stderr)
		result.Time = r.Time.Nanoseconds()
		result.Memory = r.Memory
		if r.Status == sandbox.StatusOK {
			check, err := sb.CheckFiles(ctx, executable, component.Language, input, r.OutputID, answer, s.Limits.Auxiliary)
			if err != nil {
				return result, err
			}
			result.Verdict = string(check.Status)
			result.Message = check.Message
			result.CheckerOutput = check.Message
			result.Score = check.Score
		}
		if details && result.Verdict != "OK" {
			preview, truncated, err := j.Files.Preview(ctx, r.OutputID, maxTestDetailSize)
			if err != nil && r.OutputID != "" {
				return result, err
			}
			result.Output = string(preview)
			result.IsTruncated = truncated
			result.Answer, err = packagePreview(pkg.Format.Path, test.Test.Answer)
			if err != nil {
				return result, err
			}
		}
	}
	if details && result.Verdict != "OK" {
		result.Input, err = packagePreview(pkg.Format.Path, test.Test.Manual)
		if err != nil {
			return result, err
		}
		result.ErrorLine = ParseErrorLine(0, result.Message)
	}
	return result, nil
}
func packagePreview(root, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	f, err := os.Open(filepath.Join(root, path))
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxTestDetailSize+1))
	return string(data), err
}

// The manifest explicitly owns global test order; filesystem names and group
// iteration order do not determine test numbering or sample scoring.
func collectFlatTests(prob *gfmt.Problem) []FlatTest {
	result := []FlatTest{}
	for name, group := range prob.Subtasks {
		for _, test := range group.Tests {
			result = append(result, FlatTest{SubtaskName: name, TestIndex: test.Ordinal, Test: test})
		}
	}
	sort.Slice(result, func(i, k int) bool { return result[i].TestIndex < result[k].TestIndex })
	return result
}

type FlatTest struct {
	SubtaskName string
	TestIndex   int
	Test        gfmt.Test
}

func convertLanguage(language submissionsmodel.LanguageName) string {
	switch language {
	case submissionsmodel.Cpp:
		return "cpp"
	case submissionsmodel.Python:
		return "python"
	case submissionsmodel.Golang:
		return "go"
	}
	return ""
}
