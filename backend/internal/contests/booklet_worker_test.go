package contests

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/stretchr/testify/require"
)

type outcomeQueue struct{ outcome string }

func (q *outcomeQueue) ClaimBooklet(context.Context, time.Duration) (*contestsmodel.BookletJob, error) {
	return nil, nil
}
func (q *outcomeQueue) CompleteBooklet(context.Context, contestsmodel.BookletJob, problemsmodel.StoredObject) error {
	q.outcome = "complete"
	return nil
}
func (q *outcomeQueue) FailBooklet(context.Context, contestsmodel.BookletJob, string) error {
	q.outcome = "fail"
	return nil
}
func (q *outcomeQueue) RequeueBooklet(context.Context, contestsmodel.BookletJob) error {
	q.outcome = "requeue"
	return nil
}

type failingCompiler struct{ err error }

func (c failingCompiler) CompilePDF(context.Context, string, sandbox.FileRunLimits) ([]byte, error) {
	return nil, c.err
}

func TestBookletWorkerRetriesOnlyInfrastructureFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		attempt int
		want    string
	}{
		{"latex error", errors.New("LaTeX compilation error"), 1, "fail"},
		{"sandbox down", sandbox.ErrUnavailable, 1, "requeue"},
		{"sandbox down on the last attempt", sandbox.ErrUnavailable, 3, "fail"},
		{"attempts exhausted", nil, 4, "fail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queue := &outcomeQueue{}
			w := &BookletWorker{Queue: queue, Compiler: failingCompiler{tc.err}, Limits: sandbox.FileRunLimits{Wall: time.Second}, MaxAttempts: 3, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			w.build(context.Background(), contestsmodel.BookletJob{Attempt: tc.attempt})
			require.Equal(t, tc.want, queue.outcome)
		})
	}
}
