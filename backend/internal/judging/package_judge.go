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
	"time"

	eventdata "github.com/brawler2011/gate/backend/internal/events"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	submissionsmodel "github.com/brawler2011/gate/backend/internal/submissions/model"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/calibration"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type PackageJudge struct {
	Client    *sandbox.Client
	Files     *sandbox.FileTransport
	Languages *sandbox.Config
	Programs  *sandbox.ProgramCache
	// System limits apply to every package, whenever it was built.
	Limits model.BuildLimits
	// Packages are read by go-judge from the worker's disk cache.
	WorkDir sandbox.SharedDir
	// A shared CPU can slow a run past its limit, so a TLE runs again before it stands.
	RerunTLE bool
	// Scales time limits to this host and reported times back to the reference machine.
	Calibration func() calibration.Calibration
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

func (j *PackageJudge) program(ctx context.Context, pkg *LoadedPackage, path, language string) (sandbox.Executable, error) {
	read := func(path string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(pkg.Format.Path, path))
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		return data, nil
	}
	text, err := read(path)
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
	limits := j.Limits.Compile
	compiled, err := j.Programs.Get(ctx, sandbox.Program{Source: text, Language: language, Deps: deps}, limits)
	if err != nil {
		return sandbox.Executable{}, fmt.Errorf("compile %s: %w", path, err)
	}
	defer compiled.Release()
	return j.Files.UploadCompiled(ctx, compiled.Dir(), int64(limits.Output)) //nolint:wrapcheck // already describes the upload
}


func (j *PackageJudge) roleProgram(ctx context.Context, pkg *LoadedPackage, role string) (*sandbox.Jury, error) {
	path, source, ok := pkg.Format.Problem.Program(role)
	if !ok {
		return nil, nil
	}
	executable, err := j.program(ctx, pkg, path, source.Language)
	if err != nil {
		return nil, err
	}
	protocol := sandbox.Protocol(source.Protocol)
	if protocol == "" {
		protocol = sandbox.ProtocolTestlib
	}
	return &sandbox.Jury{Executable: executable, Language: source.Language, Protocol: protocol, Args: source.Args}, nil
}
func (j *PackageJudge) releaseRoleProgram(p *sandbox.Jury) {
	if p != nil {
		j.releaseExecutable(p.Executable)
	}
}

func (j *PackageJudge) releaseExecutable(executable sandbox.Executable) {
	j.release(executable.PrimaryFileID)
	for _, id := range executable.Dependencies {
		j.release(id)
	}
}

// Judge runs only pretests when asked and the package has a proper subset of them.
func (j *PackageJudge) Judge(ctx context.Context, pkg *LoadedPackage, submissionID uuid.UUID, source string, lang string, pretests bool, progress Progress) (*FinalVerdict, error) {
	p := pkg.Format.Problem
	if pretests {
		p, pretests = p.Pretests()
	}
	calib := j.Calibration()
	sb := sandbox.NewSandbox(j.Client, j.Languages)
	sol, err := sb.CompileWithLimits(ctx, []byte(source), lang, nil, j.Limits.Compile)
	if err != nil {
		// A missing worker language is a system configuration error, not a contestant compilation error.
		if IsTemporary(err) || errors.Is(err, sandbox.ErrUnknownLanguage) {
			return nil, err
		}
		output := err.Error()
		if len(output) > maxTestDetailSize {
			output = output[:maxTestDetailSize]
		}
		return &FinalVerdict{State: submissionsmodel.GotCE, Message: output, TestDetails: &submissionsmodel.SubmissionTestDetails{CompilerOutput: &output}, Calibration: calib}, nil
	}
	defer j.releaseExecutable(sol)
	// An optional checker decides the verdict after the interactor accepts.
	checker, err := j.roleProgram(ctx, pkg, "checker")
	defer j.releaseRoleProgram(checker)
	if err != nil {
		return nil, err
	}
	var interactor *sandbox.Jury
	if p.Type == "interactive" {
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
	if err := progress.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventTestingStarted, ID: submissionID}); err != nil {
		return nil, err
	}
	tests := p.Tests()
	results := make([]TestResult, 0, len(tests))
	hasFailureDetails := false
	for _, test := range tests {
		if err := progress.Notify(ctx, eventdata.Notice{Type: submissionsmodel.SubmissionEventTestStarted, ID: submissionID, Number: safeInt32(test.Test.Ordinal)}); err != nil {
			return nil, err
		}
		result, err := j.test(ctx, sb, pkg, sol, lang, checker, interactor, test, calib, !hasFailureDetails)
		if err != nil {
			return nil, err
		}
		if result.Verdict != "OK" && !hasFailureDetails {
			hasFailureDetails = true
		}
		results = append(results, result)
		if result.Verdict != "OK" && p.Type != "scoring" {
			break
		}
	}
	verdict := NewVerdictCalculator(p.Type, p).Calculate(results)
	verdict.Calibration = calib
	verdict.Pretests = pretests
	return verdict, nil
}
func (j *PackageJudge) test(ctx context.Context, sb *sandbox.Sandbox, pkg *LoadedPackage, sol sandbox.Executable, lang string, checker, interactor *sandbox.Jury, test gfmt.GroupTest, calib calibration.Calibration, details bool) (TestResult, error) {
	p := pkg.Format.Problem
	limits := j.Limits.Solution(calib.Limit(time.Duration(p.Limits.TimeMs)*time.Millisecond), bytesize.Size(p.Limits.MemoryMb)*bytesize.MiB)
	inputPath, answerPath := gfmt.TestFolder+"/"+test.Test.Input, gfmt.TestFolder+"/"+test.Test.Answer
	input, err := j.WorkDir.File(filepath.Join(pkg.Format.Path, inputPath))
	if err != nil {
		return TestResult{}, err //nolint:wrapcheck // names the path
	}
	answer, err := j.WorkDir.File(filepath.Join(pkg.Format.Path, answerPath))
	if err != nil {
		return TestResult{}, err //nolint:wrapcheck // names the path
	}
	run := func() (TestResult, string, error) {
		result := TestResult{TestNumber: test.Test.Ordinal, Sample: test.Test.Sample, Public: p.ShowsInput(test.Test), Group: test.Group.Name}
		if interactor != nil {
			r, err := sb.InteractFiles(ctx, sol, lang, interactor.With(test.Group.CheckerArgs), input, answer, limits, j.Limits.Auxiliary)
			if err != nil {
				return result, "", err
			}
			result.Verdict, result.Message, result.Score = string(r.Status), r.Message, r.Score
			result.Time, result.Memory = calib.Normalize(r.SolutionResult.Time).Nanoseconds(), r.SolutionResult.Memory
			return result, r.OutputID, nil
		}
		r, err := sb.RunSolution(ctx, sol, lang, input, sandbox.Streams{Input: p.InputFile, Output: p.OutputFile}, limits)
		if err != nil {
			return result, "", err
		}
		result.Verdict, result.Message, result.Time, result.Memory = string(r.Status), string(r.Stderr), calib.Normalize(r.Time).Nanoseconds(), r.Memory
		return result, r.OutputID, nil
	}
	result, output, err := run()
	if err == nil && j.RerunTLE && result.Verdict == string(sandbox.StatusTLE) {
		j.release(output)
		result, output, err = run()
	}
	if err != nil {
		return result, err
	}
	defer j.release(output)
	if checker != nil && result.Verdict == string(sandbox.StatusOK) {
		check, err := sb.CheckFiles(ctx, checker.With(test.Group.CheckerArgs), input, sandbox.FileID(output), answer, j.Limits.Auxiliary)
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
			result.Answer, err = packagePreview(pkg.Format.Path, answerPath)
			if err != nil {
				return result, err
			}
		}
		result.Input, err = packagePreview(pkg.Format.Path, inputPath)
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
