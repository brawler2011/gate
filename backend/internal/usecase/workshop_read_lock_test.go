package usecase

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/pkg/storage"
	pb "github.com/criyle/go-judge/pb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type contentionWorkspaceLocker struct {
	mu        sync.Mutex
	attempted chan bool
}

func (l *contentionWorkspaceLocker) WithWorkspaceLock(ctx context.Context, _ uuid.UUID, fn func(context.Context) error) error {
	acquired := l.mu.TryLock()
	l.attempted <- !acquired
	if !acquired {
		l.mu.Lock()
	}
	defer l.mu.Unlock()
	return fn(ctx)
}

func TestWorkshopReaders_HoldLockThroughExecutionAndRejectPartialImport(t *testing.T) {
	for _, operation := range []string{"validate", "solution"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			var uc *WorkshopUseCase
			var id uuid.UUID
			var req models.TestSolutionRequest
			if operation == "validate" {
				uc, id = newWorkshopValidatorTest(t, true, map[string]string{"validators/validator.cpp": "validator", "tests/01.in": "original"}, nil)
			} else {
				uc, req = newWorkshopCheckerTest(t, storage.NewLocalStorage(t.TempDir()), map[string]string{"checkers/checker.cpp": "checker", "tests/01.in": "original", "tests/01.out": "answer"}, nil)
				id = req.ProblemID
			}
			manifest, err := uc.GetManifest(ctx, id)
			require.NoError(t, err)
			require.NoError(t, uc.SaveManifest(ctx, id, manifest))
			entered, release := make(chan struct{}), make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			var once sync.Once
			uc.sandbox = newWorkshopValidatorSandbox(t, func(_ context.Context, request *pb.Request) (*pb.Response, error) {
				if request.GetCmd()[0].GetArgs()[0] != "./checker" {
					require.Equal(t, "original", string(request.GetCmd()[0].GetFiles()[0].GetMemory().GetContent()))
				}
				once.Do(func() { close(entered); <-release })
				return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{Status: pb.Response_Result_Accepted, Files: map[string][]byte{"stdout": []byte("answer")}}.Build()}}.Build(), nil
			})
			locker := &contentionWorkspaceLocker{attempted: make(chan bool, 8)}
			uc.workspaceStorage.locker = locker
			read := func() error {
				if operation == "validate" {
					_, err := uc.ValidateAllTests(ctx, id, uuid.Nil)
					return err
				}
				_, err := uc.TestSolution(ctx, req)
				return err
			}
			readDone := make(chan error, 1)
			go func() { readDone <- read() }()
			<-entered
			require.False(t, <-locker.attempted)
			fault := errors.New("import manifest failure")
			repo := &manifestFaultRepo{ProblemsRepo: uc.problemsRepo, manifestErr: fault}
			archive := createZipBytes(t, map[string]string{"problem.yaml": "format_version: '1.0'\ntitle: Partial\ntype: pass-fail\nsubtasks: {samples: {tests: [{manual: 01.in}]}}\n", "tests/01.in": "PARTIAL IMPORT"})
			imported := make(chan error, 1)
			go func() {
				_, err := NewProblemImportUseCase(repo, uc.workspaceStorage).ImportProblemPackage(ctx, bytes.NewReader(archive), int64(len(archive)), id)
				imported <- err
			}()
			// The mutation reached the shared locker while execution was still running.
			// TryLock records contention synchronously, without a scheduling timeout.
			require.True(t, <-locker.attempted, "reader must retain its lock through the sandbox run")
			close(release)
			require.NoError(t, <-readDone)
			require.ErrorIs(t, <-imported, fault)
			require.ErrorContains(t, read(), "incomplete")
		})
	}
}
