package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/storage"
	pb "github.com/criyle/go-judge/pb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkshopValidateAllTests_InputNamesAndOrder(t *testing.T) {
	uc, id := newWorkshopValidatorTest(t, true, map[string]string{
		"validators/validator.cpp": "validator",
		"problem.yaml":             "subtasks:\n  z:\n    tests:\n      - generate: generator 42\n  samples:\n    tests:\n      - manual: sample.in\n  a:\n    tests:\n      - manual: edge-case.in\n",
		"tests/sample.in":          "sample",
		"tests/edge-case.in":       "edge",
		"tests/03.in":              "generated",
	}, nil)
	inputs := make(chan string, 3)
	uc.sandbox = newWorkshopValidatorSandbox(t, func(_ context.Context, req *pb.Request) (*pb.Response, error) {
		inputs <- string(req.GetCmd()[0].GetFiles()[0].GetMemory().GetContent())
		return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{Status: pb.Response_Result_Accepted}.Build()}}.Build(), nil
	})
	report, err := uc.ValidateAllTests(context.Background(), id, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 3, report.TotalTests)
	require.Equal(t, 3, report.ValidTests)
	require.Zero(t, report.InvalidTests)
	require.Equal(t, "sample", <-inputs)
	require.Equal(t, "edge", <-inputs)
	require.Equal(t, "generated", <-inputs)
	meta, err := uc.readTestsMetadata(context.Background(), id)
	require.NoError(t, err)
	for i, name := range []string{"samples", "a", "z"} {
		require.Equal(t, name, meta.Groups[i].Name)
		require.Equal(t, i+1, report.Results[i].TestNumber)
		require.Equal(t, i+1, meta.Tests[i].Ordinal)
	}
}

func TestWorkshopValidateAllTests_EmptyTestSet(t *testing.T) {
	uc, id := newWorkshopValidatorTest(t, true, map[string]string{"validators/validator.cpp": "validator", "problem.yaml": "subtasks: {}\n"}, nil)
	uc.sandbox = newWorkshopValidatorSandbox(t, func(_ context.Context, _ *pb.Request) (*pb.Response, error) {
		t.Error("empty test set must not dispatch a validator run")
		return pb.Response_builder{}.Build(), nil
	})
	report, err := uc.ValidateAllTests(context.Background(), id, uuid.Nil)
	require.NoError(t, err)
	require.Zero(t, report.TotalTests)
	require.Zero(t, report.ValidTests)
	require.Zero(t, report.InvalidTests)
	require.NotNil(t, report.Results)
}

func TestWorkshopValidateAllTests_RuntimeErrors(t *testing.T) {
	storageErr := errors.New("input storage unavailable")
	for _, tc := range []struct {
		name        string
		runStatus   pb.Response_Result_StatusType
		exitStatus  int32
		dispatchErr error
		readErr     error
		cancel      bool
		noResults   bool
		wantError   string
		wantInvalid bool
	}{
		{name: "sandbox failure", runStatus: pb.Response_Result_InternalError, wantError: "validator sandbox failure"},
		{name: "dispatch failure", dispatchErr: status.Error(codes.Unavailable, "judge unavailable"), wantError: "judge unavailable"},
		{name: "no execution result", noResults: true, wantError: "returned no results"},
		{name: "input storage failure", readErr: storageErr, wantError: "failed to read test input tests/01.in"},
		{name: "execution cancellation", cancel: true, wantError: "failed to run validator"},
		{name: "nonzero accepted status", runStatus: pb.Response_Result_Accepted, exitStatus: 1, wantInvalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, id := newWorkshopValidatorTest(t, true, map[string]string{"validators/validator.cpp": "validator"}, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			uc.workspaceStorage.storage = &workshopValidatorStorage{Storage: uc.workspaceStorage.storage, suffix: "tests/01.in", readErr: tc.readErr, beforeRead: func() {
				if tc.cancel {
					cancel()
				}
			}}
			uc.sandbox = newWorkshopValidatorSandbox(t, func(_ context.Context, _ *pb.Request) (*pb.Response, error) {
				if tc.noResults {
					return pb.Response_builder{}.Build(), nil
				}
				return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{Status: tc.runStatus, ExitStatus: tc.exitStatus}.Build()}}.Build(), tc.dispatchErr
			})
			report, err := uc.ValidateAllTests(ctx, id, uuid.Nil)
			if tc.wantInvalid {
				require.NoError(t, err)
				require.Equal(t, 1, report.InvalidTests)
				require.Zero(t, report.ValidTests)
				return
			}
			require.ErrorContains(t, err, tc.wantError)
			require.Nil(t, report)
			if tc.readErr != nil {
				require.ErrorIs(t, err, storageErr)
			}
			if tc.cancel {
				require.Equal(t, codes.Canceled, status.Code(err))
			}
		})
	}
}

type workshopValidatorStorage struct {
	storage.Storage
	suffix     string
	readErr    error
	listErr    error
	readPaths  []string
	beforeRead func()
}

func (s *workshopValidatorStorage) DownloadFile(ctx context.Context, bucket, key string, etag *string) (io.ReadCloser, string, error) {
	s.readPaths = append(s.readPaths, key)
	if strings.HasSuffix(key, s.suffix) {
		if s.beforeRead != nil {
			s.beforeRead()
		}
		if s.readErr != nil {
			return nil, "", s.readErr
		}
	}
	return s.Storage.DownloadFile(ctx, bucket, key, etag)
}

func (s *workshopValidatorStorage) ListObjects(ctx context.Context, bucket, prefix string) ([]storage.ObjectInfo, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Storage.ListObjects(ctx, bucket, prefix)
}

type workshopValidatorExecutor struct {
	pb.UnimplementedExecutorServer
	run func(context.Context, *pb.Request) (*pb.Response, error)
}

func (e *workshopValidatorExecutor) Exec(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	if cached := req.GetCmd()[0].GetCopyOutCached(); len(cached) > 0 {
		return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{
			Status: pb.Response_Result_Accepted, FileIDs: map[string]string{cached[0].GetName(): "validator"},
		}.Build()}}.Build(), nil
	}
	return e.run(ctx, req)
}

func newWorkshopValidatorSandbox(t *testing.T, run func(context.Context, *pb.Request) (*pb.Response, error)) *sandbox.Sandbox {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	pb.RegisterExecutorServer(server, &workshopValidatorExecutor{run: run})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	client, err := sandbox.NewClient(listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cfg, err := sandbox.LoadConfig("../../languages.yaml")
	require.NoError(t, err)
	return sandbox.NewSandbox(client, cfg)
}

func TestWorkshopValidateAllTests_PreparationErrors(t *testing.T) {
	storageErr := errors.New("validator storage unavailable")
	for _, tc := range []struct {
		name      string
		validator bool
		files     map[string]string
		deps      []models.Dependency
		readErr   error
		readPath  string
		listErr   error
		wantError string
		wantCause error
	}{
		{name: "missing validator", wantError: "problem must have a validator"},
		{name: "missing source", validator: true, wantError: "failed to read validator validators/validator.cpp", wantCause: storage.ErrNotFound},
		{name: "missing dependency", validator: true, files: map[string]string{"validators/validator.cpp": "validator"}, deps: []models.Dependency{{Filename: "testlib.h"}}, wantError: "failed to read validator dependency lib/testlib.h", wantCause: storage.ErrNotFound},
		{name: "compilation failure", validator: true, files: map[string]string{"validators/validator.cpp": "validator"}, wantError: "failed to compile validator validators/validator.cpp: unsupported language: cpp"},
		{name: "invalid metadata", validator: true, files: map[string]string{"problem.yaml": "subtasks: ["}, wantError: "failed to parse problem.yaml"},
		{name: "metadata storage failure", validator: true, readErr: storageErr, readPath: "problem.yaml", wantError: "failed to read problem.yaml", wantCause: storageErr},
		{name: "source storage failure", validator: true, readErr: storageErr, readPath: "validators/validator.cpp", wantError: "failed to read validator", wantCause: storageErr},
		{name: "dependency storage failure", validator: true, files: map[string]string{"validators/validator.cpp": "validator", "lib/testlib.h": "header"}, deps: []models.Dependency{{Filename: "testlib.h"}}, readErr: storageErr, readPath: "validators/testlib.h", wantError: "failed to read validator dependency validators/testlib.h", wantCause: storageErr},
		{name: "dependency listing failure", validator: true, files: map[string]string{"validators/validator.cpp": "validator"}, deps: []models.Dependency{{Filename: "testlib.h"}}, listErr: storageErr, wantError: "failed to list validator dependencies", wantCause: storageErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, id := newWorkshopValidatorTest(t, tc.validator, tc.files, tc.deps)
			store := &workshopValidatorStorage{Storage: uc.workspaceStorage.storage, suffix: tc.readPath, readErr: tc.readErr, listErr: tc.listErr}
			uc.workspaceStorage.storage = store
			uc.sandbox = sandbox.NewSandbox(nil, &sandbox.Config{})
			report, err := uc.ValidateAllTests(context.Background(), id, uuid.Nil)
			require.ErrorContains(t, err, tc.wantError)
			require.Nil(t, report)
			if tc.wantCause != nil {
				require.ErrorIs(t, err, tc.wantCause)
			}
			if tc.readPath == "validators/testlib.h" {
				require.NotContains(t, store.readPaths, uc.workspaceStorage.getWorkspaceKey(id, "lib/testlib.h"))
			}
		})
	}
}

func TestWorkshopValidateAllTests_Cancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		uc, id := newWorkshopValidatorTest(t, true, map[string]string{"validators/validator.cpp": "validator"}, nil)
		client, err := sandbox.NewClient("127.0.0.1:1")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, client.Close()) })
		cfg, err := sandbox.LoadConfig("../../languages.yaml")
		require.NoError(t, err)
		uc.sandbox = sandbox.NewSandbox(client, cfg)
		ctx, cancel := context.WithCancel(context.Background())
		wantCode := codes.Canceled
		if deadline {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			wantCode = codes.DeadlineExceeded
		}
		cancel()
		report, err := uc.ValidateAllTests(ctx, id, uuid.Nil)
		require.ErrorContains(t, err, "failed to compile validator")
		require.Equal(t, wantCode, status.Code(err))
		require.Nil(t, report)
	}
}

func newWorkshopValidatorTest(t *testing.T, validator bool, files map[string]string, deps []models.Dependency) (*WorkshopUseCase, uuid.UUID) {
	t.Helper()
	id := uuid.New()
	repo := newProblemImportMockProblemsRepo()
	manifest := defaultManifest("Validator test")
	if validator {
		manifest.FilesMetadata = []models.FileMetadata{{Type: "validator", Filename: "validators/validator.cpp", Dependencies: deps}}
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	repo.manifests[id] = data
	ws := NewWorkspaceStorage(storage.NewLocalStorage(t.TempDir()), "validator-test")
	require.NoError(t, ws.WriteFile(context.Background(), id, "problem.yaml", []byte("subtasks:\n  samples:\n    tests:\n      - manual: 01.in\n")))
	require.NoError(t, ws.WriteFile(context.Background(), id, "tests/01.in", []byte("invalid input\n")))
	for path, content := range files {
		require.NoError(t, ws.WriteFile(context.Background(), id, path, []byte(content)))
	}
	return NewWorkshopUseCase(repo, ws, nil, nil), id
}
