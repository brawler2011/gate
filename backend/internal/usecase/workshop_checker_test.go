package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	pb "github.com/criyle/go-judge/pb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkshopTestSolution_CheckerPreparationErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		files     map[string]string
		deps      []models.Dependency
		wantError string
		wantCause error
	}{
		{name: "missing source", wantError: "failed to read checker checkers/checker.cpp", wantCause: storage.ErrNotFound},
		{name: "missing dependency", files: map[string]string{"checkers/checker.cpp": "checker"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to read checker dependency lib/testlib.h", wantCause: storage.ErrNotFound},
		{name: "compilation failure", files: map[string]string{"checkers/checker.cpp": "checker"}, wantError: "failed to compile checker checkers/checker.cpp: unsupported language: cpp"},
		{name: "relative dependency", files: map[string]string{"checkers/checker.cpp": "checker", "checkers/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to compile checker"},
		{name: "shared dependency", files: map[string]string{"checkers/checker.cpp": "checker", "lib/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to compile checker"},
		{name: "nested shared dependency", files: map[string]string{"checkers/checker.cpp": "checker", "lib/support/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to compile checker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := storage.NewLocalStorage(t.TempDir())
			uc, req := newWorkshopCheckerTest(t, store, tc.files, tc.deps)
			// An empty language config fails compilation without dispatching any execution.
			uc.sandbox = sandbox.NewSandbox(nil, &sandbox.Config{})
			report, err := uc.TestSolution(context.Background(), req)
			require.ErrorContains(t, err, tc.wantError)
			require.Nil(t, report)
			if tc.wantCause != nil {
				require.ErrorIs(t, err, tc.wantCause)
			}
		})
	}
}

func TestWorkshopTestSolution_CheckerCancellation(t *testing.T) {
	for _, tc := range []struct {
		name string
		code codes.Code
	}{
		{name: "cancelled", code: codes.Canceled},
		{name: "deadline", code: codes.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, req := newWorkshopCheckerTest(t, storage.NewLocalStorage(t.TempDir()), map[string]string{"checkers/checker.cpp": "checker"}, nil)
			client, err := sandbox.NewClient("127.0.0.1:1")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			cfg, err := sandbox.LoadConfig("../../languages.yaml")
			require.NoError(t, err)
			uc.sandbox = sandbox.NewSandbox(client, cfg)
			ctx, cancel := context.WithCancel(context.Background())
			if tc.code == codes.DeadlineExceeded {
				cancel()
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			}
			cancel()
			report, err := uc.TestSolution(ctx, req)
			require.ErrorIs(t, err, ctx.Err())
			require.Equal(t, tc.code, status.Code(status.FromContextError(err).Err()))
			require.Nil(t, report)
		})
	}
}

func TestWorkshopTestSolution_RequiresChecker(t *testing.T) {
	uc, req := newWorkshopCheckerTest(t, storage.NewLocalStorage(t.TempDir()), nil, nil)
	config, err := uc.readProblemConfig(context.Background(), req.ProblemID)
	require.NoError(t, err)
	config.Checker = ""
	require.NoError(t, uc.writeProblemConfig(context.Background(), req.ProblemID, config))
	// A nil sandbox would panic if solution compilation were attempted.
	report, err := uc.TestSolution(context.Background(), req)
	require.ErrorContains(t, err, "problem must have a checker")
	require.Nil(t, report)
}

func newWorkshopCheckerTest(t *testing.T, store storage.Storage, files map[string]string, deps []models.Dependency) (*WorkshopUseCase, models.TestSolutionRequest) {
	t.Helper()
	problemID := uuid.New()
	repo := newProblemImportMockProblemsRepo()
	manifest := defaultManifest("Checker test")
	manifest.FilesMetadata = []models.FileMetadata{{Type: "checker", Filename: "checkers/checker.cpp", Dependencies: deps}}
	ws := NewWorkspaceStorage(store, "test-workshop", repo)
	fixtureFiles := workshopFixtureFiles(manifest, files)
	fixtureFiles["solutions/main.cpp"] = "solution"
	seedWorkspaceFiles(t, ws, problemID, fixtureFiles)

	return NewWorkshopUseCase(ws, nil), models.TestSolutionRequest{ProblemID: problemID, SolutionPath: "solutions/main.cpp"}
}

func TestWorkshopTestSolution_ManifestFilesAndGenerators(t *testing.T) {
	uc, req := newWorkshopCheckerTest(t, storage.NewLocalStorage(t.TempDir()), map[string]string{
		"checkers/checker.cpp":     "checker",
		"generators/generator.cpp": "generator",
		"solutions/reference.cpp":  "reference",
		"tests/sample.in":          "sample", "tests/sample.ans": "sample answer",
		"tests/edge-case.in": "edge", "tests/edge-case.a": "edge answer",
		"tests/03.in": "stale generated input", "tests/03.out": "stale generated answer",
		"problem.yaml": `limits: {time_ms: 1000, memory_mb: 256}
solutions: {reference.cpp: main}
subtasks:
 z:
  tests: [{generate: generator 42}]
 a:
  tests: [{manual: edge-case.in}]
 samples:
  tests: [{manual: sample.in}]
`,
	}, nil)
	var inputs []string
	uc.sandbox = newWorkshopValidatorSandbox(t, func(_ context.Context, request *pb.Request) (*pb.Response, error) {
		cmd := request.GetCmd()[0]
		output := ""
		switch cmd.GetArgs()[0] {
		case "./generator":
			require.Equal(t, []string{"./generator", "42"}, cmd.GetArgs())
			output = "generated"
		case "./checker":
			input := string(cmd.GetCopyIn()["input.txt"].GetMemory().GetContent())
			answer := string(cmd.GetCopyIn()["answer.txt"].GetMemory().GetContent())
			require.Equal(t, input+" answer", answer)
			inputs = append(inputs, input)
		default:
			output = string(cmd.GetFiles()[0].GetMemory().GetContent()) + " answer"
		}
		return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{Status: pb.Response_Result_Accepted, Files: map[string][]byte{"stdout": []byte(output)}}.Build()}}.Build(), nil
	})
	meta, err := uc.GetTestsConfig(context.Background(), req.ProblemID)
	require.NoError(t, err)
	require.NoError(t, uc.UpdateTestsConfig(context.Background(), req.ProblemID, meta))
	report, err := uc.TestSolution(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 3, report.PassedTests)
	require.Zero(t, report.FailedTests)
	require.Equal(t, []string{"sample", "edge", "generated"}, inputs)
	req.TestNumbers = []int{2}
	inputs = nil
	report, err = uc.TestSolution(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 1, report.PassedTests)
	require.Equal(t, []string{"edge"}, inputs)
}
