package judging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type PackageJudge struct {
	Client *sandbox.Client
	Files  *sandbox.FileTransport
	Languages *sandbox.Config
	Programs  *sandbox.ProgramCache
	Logger    *slog.Logger
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

func (j *PackageJudge) program(ctx context.Context, pkg *LoadedPackage, source gfmt.Program) (sandbox.Executable, error) {
	read := func(path string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(pkg.Format.Path, path))
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return data, nil
	}
	text, err := read(source.Source)
	if err != nil {
		return sandbox.Executable{}, err
	}
	deps := map[string][]byte{}
	library, err := os.ReadDir(filepath.Join(pkg.Format.Path, gfmt.LibraryFolder))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return sandbox.Executable{}, fmt.Errorf("readdir: %w", err)
	}
	for _, f := range library {
		if deps[f.Name()], err = read(gfmt.LibraryFolder + "/" + f.Name()); err != nil {
			return sandbox.Executable{}, err
		}
	}
	limits := pkg.Descriptor.Snapshot.Limits.Compile
	dir, err := j.Programs.Get(ctx, sandbox.Program{Source: text, Language: source.Language, Deps: deps}, limits)
	if err != nil {
		return sandbox.Executable{}, fmt.Errorf("compile %s: %w", source.Source, err)
	}
	return j.Files.UploadCompiled(ctx, dir, int64(limits.Output)) //nolint:wrapcheck // already describes the upload
}

type roleProgram struct {
	executable sandbox.Executable
	language   string
}

func (j *PackageJudge) roleProgram(ctx context.Context, pkg *LoadedPackage, role string) (*roleProgram, error) {
	source, ok := pkg.Format.Problem.Programs[role]
	if !ok {
		return nil, nil
	}
	executable, err := j.program(ctx, pkg, source)
	if err != nil {
		return nil, err
	}
	return &roleProgram{executable: executable, language: source.Language}, nil
}
func (j *PackageJudge) releaseRoleProgram(p *roleProgram) {
	if p != nil {
		j.releaseExecutable(p.executable)
	}
}

func (j *PackageJudge) releaseExecutable(executable sandbox.Executable) {
	j.release(executable.PrimaryFileID)
	for _, id := range executable.Dependencies {
		j.release(id)
	}
}

func (j *PackageJudge) Judge(ctx context.Context, pkg *LoadedPackage, submissionID uuid.UUID, source string, lang string, progress Progress, meta submissionsmodel.SubmissionEventMeta) (*FinalVerdict, error) {
	s := pkg.Descriptor.Snapshot
	sb := sandbox.NewSandbox(j.Client, j.Languages)
	sol, err := sb.CompileWithLimits(ctx, []byte(source), lang, nil, j.Languages.SubmissionCompileLimits(lang, s.Limits.Compile))
	if err != nil {
		// A missing worker language is a system configuration error, not a contestant compilation error.
		if IsTemporary(err) || errors.Is(err, sandbox.ErrUnknownLanguage) {
			return nil, err
		}
		output := err.Error()
		if len(output) > maxTestDetailSize {
			output = output[:maxTestDetailSize]
		}
		return &FinalVerdict{State: submissionsmodel.GotCE, Message: output, TestDetails: &submissionsmodel.SubmissionTestDetails{CompilerOutput: &output}}, nil
	}
	defer j.releaseExecutable(sol)
	// An optional checker decides the verdict after the interactor accepts.
	checker, err := j.roleProgram(ctx, pkg, "checker")
	defer j.releaseRoleProgram(checker)
	if err != nil {
		return nil, err
	}
	var interactor *roleProgram
	if s.Draft.Type == "interactive" {
		interactor, err = j.roleProgram(ctx, pkg, "interactor")
		defer j.releaseRoleProgram(interactor)
		if err != nil {
			return nil, err
		}
		if interactor == nil {
			return nil, errors.New("package has no interactor")
		}
	} else if checker == nil {
		return nil, errors.New("package has no checker")
	}
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
		result, err := j.test(ctx, sb, pkg, sol, lang, checker, interactor, test, !hasFailureDetails)
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
func (j *PackageJudge) test(ctx context.Context, sb *sandbox.Sandbox, pkg *LoadedPackage, sol sandbox.Executable, lang string, checker, interactor *roleProgram, test FlatTest, details bool) (TestResult, error) {
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
	answer, err := j.upload(ctx, pkg.Format.Path, test.Test.Answer, int64(s.Limits.ObjectBytes))
	if err != nil {
		return TestResult{}, err
	}
	defer j.release(answer)
	result := TestResult{TestNumber: test.TestIndex}
	var output string
	if interactor != nil {
		r, err := sb.InteractFiles(ctx, sol, lang, interactor.executable, interactor.language, input, answer, limits, s.Limits.Auxiliary)
		if err != nil {
			return result, err
		}
		defer j.release(r.OutputID)
		output = r.OutputID
		result.Verdict = string(r.Status)
		result.Message = r.Message
		result.Time = r.SolutionResult.Time.Nanoseconds()
		result.Memory = r.SolutionResult.Memory
		result.Score = r.Score
	} else {
		r, err := sb.RunFiles(ctx, sol, lang, input, nil, nil, limits)
		if err != nil {
			return result, err
		}
		defer j.release(r.OutputID)
		output = r.OutputID
		result.Verdict = string(r.Status)
		result.Message = string(r.Stderr)
		result.Time = r.Time.Nanoseconds()
		result.Memory = r.Memory
	}
	if checker != nil && result.Verdict == string(sandbox.StatusOK) {
		check, err := sb.CheckFiles(ctx, checker.executable, checker.language, input, output, answer, s.Limits.Auxiliary)
		if err != nil {
			return result, err
		}
		result.Verdict = string(check.Status)
		result.Message = check.Message
		result.CheckerOutput = check.Message
		result.Score = check.Score
	}
	if details && result.Verdict != "OK" {
		if interactor == nil {
			preview, truncated, err := j.Files.Preview(ctx, output, maxTestDetailSize)
			if err != nil && output != "" {
				return result, err
			}
			result.Output = string(preview)
			result.IsTruncated = truncated
			result.Answer, err = packagePreview(pkg.Format.Path, test.Test.Answer)
			if err != nil {
				return result, err
			}
		}
		result.Input, err = packagePreview(pkg.Format.Path, test.Test.Manual)
		if err != nil {
			return result, err
		}
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

// Manifest order controls test numbering and sample scoring independently of filesystem or group order.
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
