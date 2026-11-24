package contests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

type BookletQueue interface {
	// ClaimBooklet returns nil when no booklet waits.
	ClaimBooklet(ctx context.Context, lease time.Duration) (*contestsmodel.BookletJob, error)
	// The finishing calls return pkg.ErrConflict once a newer source replaced the job.
	CompleteBooklet(ctx context.Context, job contestsmodel.BookletJob, object problemsmodel.StoredObject) error
	FailBooklet(ctx context.Context, job contestsmodel.BookletJob, diagnostic string) error
	RequeueBooklet(ctx context.Context, job contestsmodel.BookletJob) error
}
type PDFCompiler interface {
	CompilePDF(ctx context.Context, source string, limits sandbox.FileRunLimits) ([]byte, error)
}
type BookletUploader interface {
	UploadFileWithSHA256(ctx context.Context, bucket, key string, body io.Reader, contentType, sha256 string) error
}

// BookletWorker compiles queued booklets in the sandbox and stores the PDFs.
type BookletWorker struct {
	Queue       BookletQueue
	Compiler    PDFCompiler
	Store       BookletUploader
	Bucket      string
	Limits      sandbox.FileRunLimits
	MaxAttempts int
	Poll        time.Duration
	Logger      *slog.Logger
}

func (w *BookletWorker) Run(ctx context.Context) error {
	if w.MaxAttempts < 1 || w.Poll <= 0 || w.Limits.Wall <= 0 {
		return fmt.Errorf("invalid booklet worker configuration")
	}
	ticker := time.NewTicker(w.Poll)
	defer ticker.Stop()
	for {
		job, err := w.Queue.ClaimBooklet(ctx, w.Limits.Wall+time.Minute)
		if err != nil && ctx.Err() == nil {
			w.Logger.ErrorContext(ctx, "claim booklet", "error", err)
		}
		if job != nil {
			w.build(ctx, *job)
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *BookletWorker) build(ctx context.Context, job contestsmodel.BookletJob) {
	log := w.Logger.With("contest_id", job.ContestID, "language", job.Language, "attempt", job.Attempt)
	err := errors.New("booklet attempts exhausted")
	if job.Attempt <= w.MaxAttempts {
		err = w.compile(ctx, job)
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var finishErr error
	switch {
	case err == nil:
		return
	case retryable(err) && job.Attempt < w.MaxAttempts:
		log.WarnContext(ctx, "booklet build failed, retrying", "error", err)
		finishErr = w.Queue.RequeueBooklet(finishCtx, job)
	default:
		log.InfoContext(ctx, "booklet build failed", "error", err)
		finishErr = w.Queue.FailBooklet(finishCtx, job, err.Error())
	}
	if finishErr != nil && !errors.Is(finishErr, pkg.ErrConflict) {
		log.ErrorContext(ctx, "finish booklet build", "error", finishErr)
	}
}

func (w *BookletWorker) compile(ctx context.Context, job contestsmodel.BookletJob) error {
	pdf, err := w.Compiler.CompilePDF(ctx, job.Source, w.Limits)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(pdf)
	object := problemsmodel.NewStoredObject(job.OrganizationID, problemsmodel.UploadSpec{Purpose: "booklet", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(pdf))})
	if err := w.Store.UploadFileWithSHA256(ctx, w.Bucket, object.Key, bytes.NewReader(pdf), "application/pdf", object.SHA256); err != nil {
		return fmt.Errorf("%w: upload booklet: %w", errTransient, err)
	}
	if err := w.Queue.CompleteBooklet(ctx, job, object); err != nil && !errors.Is(err, pkg.ErrConflict) {
		return fmt.Errorf("%w: complete booklet: %w", errTransient, err)
	}
	return nil
}

// errTransient marks failures outside LaTeX, which another attempt may not meet.
var errTransient = errors.New("transient booklet failure")

func retryable(err error) bool {
	var network net.Error
	return errors.Is(err, errTransient) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, sandbox.ErrUnavailable) || errors.As(err, &network)
}
