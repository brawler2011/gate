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
	"slices"
	"time"

	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/internal/problems"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
)

type BookletQueue interface {
	ListBooklets(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error)
	RekeyBooklets(ctx context.Context, contestID uuid.UUID) error
	RekeyAllBooklets(ctx context.Context) error
	// ClaimBooklet returns nil when no booklet waits.
	ClaimBooklet(ctx context.Context, lease time.Duration) (*contestsmodel.BookletJob, error)
	// The finishing calls return pkg.ErrConflict once the booklet was re-keyed or reclaimed.
	CompleteBooklet(ctx context.Context, job contestsmodel.BookletJob, object problemsmodel.StoredObject) error
	FailBooklet(ctx context.Context, job contestsmodel.BookletJob, diagnostic string) error
	RequeueBooklet(ctx context.Context, job contestsmodel.BookletJob) error
}
type PrintedStatements interface {
	ReadPrinted(ctx context.Context, problemID, packageID uuid.UUID, language string) (problems.Printed, error)
}
type BookletUploader interface {
	UploadFileWithSHA256(ctx context.Context, bucket, key string, body io.Reader, contentType, sha256 string) error
}

// BookletWorker builds queued booklets in typeset and stores the PDFs.
type BookletWorker struct {
	Queue       BookletQueue
	Statements  PrintedStatements
	Typeset     Typesetter
	Store       BookletUploader
	Bucket      string
	Lease       time.Duration
	MaxAttempts int
	Poll        time.Duration
	// Wake receives a value when a booklet may have been queued.
	Wake   <-chan struct{}
	Logger *slog.Logger
}

type Typesetter interface {
	PDF(ctx context.Context, doc string, files map[string][]byte) ([]byte, error)
}

func (w *BookletWorker) Run(ctx context.Context) error {
	rekeyed := false
	for ctx.Err() == nil {
		if !rekeyed {
			err := w.Queue.RekeyAllBooklets(ctx)
			if rekeyed = err == nil; !rekeyed && ctx.Err() == nil {
				w.Logger.ErrorContext(ctx, "rekey booklets", "error", err)
			}
		}
		job, err := w.Queue.ClaimBooklet(ctx, w.Lease)
		if err != nil && ctx.Err() == nil {
			w.Logger.ErrorContext(ctx, "claim booklet", "error", err)
		}
		if job != nil {
			w.build(ctx, *job)
			continue
		}
		select {
		case <-ctx.Done():
		case <-w.Wake:
		case <-time.After(w.Poll):
		}
	}
	return nil
}

func (w *BookletWorker) build(ctx context.Context, job contestsmodel.BookletJob) {
	log := w.Logger.With("contest_id", job.ContestID, "language", job.Language, "attempt", job.Attempt)
	err := errors.New("booklet build attempts exhausted")
	if job.Attempt <= w.MaxAttempts {
		// A hung sandbox or storage call must not hold the worker past the lease.
		attemptCtx, cancel := context.WithTimeout(ctx, w.Lease)
		err = w.compile(attemptCtx, job)
		cancel()
	}
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var tex *statement.Error
	switch {
	case err == nil:
		return
	case !errors.As(err, &tex) && job.Attempt < w.MaxAttempts:
		log.WarnContext(ctx, "booklet build failed, retrying", "error", err)
		err = w.Queue.RequeueBooklet(finishCtx, job)
	default:
		log.InfoContext(ctx, "booklet build failed", "error", err)
		err = w.Queue.FailBooklet(finishCtx, job, err.Error())
	}
	if err != nil {
		log.WarnContext(ctx, "finish booklet build", "error", err)
	}
}

func (w *BookletWorker) compile(ctx context.Context, job contestsmodel.BookletJob) error {
	booklets, err := w.Queue.ListBooklets(ctx, job.ContestID)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(booklets, func(b contestsmodel.Booklet) bool { return b.Language == job.Language })
	if i < 0 || booklets[i].Key() != job.Key {
		// Contest changes re-key in their transaction; this catches inputs that changed outside it, such as the generator.
		return w.Queue.RekeyBooklets(ctx, job.ContestID)
	}
	in := booklets[i].BookletInputs
	doc := statement.Contest{Title: in.Title, Place: in.Place, Date: in.Date, Language: in.Language}
	files := statement.Files(in.Language)
	for n, p := range in.Problems {
		printed, err := w.Statements.ReadPrinted(ctx, p.ProblemID, p.PackageID, in.Language)
		if err != nil {
			return err
		}
		limits := problemsmodel.PrintedLimits{TimeLimitMs: &p.TimeLimitMs, MemoryLimitMb: p.MemoryLimitMb, InputFile: p.InputFile, OutputFile: p.OutputFile}
		printedProblem := problemsmodel.PrintedProblem(p.Title, limits, printed.Statement, printed.Samples)
		// Problems may show different images under one name.
		printedProblem.Letter, printedProblem.Assets = contestsmodel.ProblemLetter(p.Ordinal), fmt.Sprintf("assets/%d", n)
		for name, data := range printed.Assets {
			files[printedProblem.Assets+"/"+name] = data
		}
		doc.Problems = append(doc.Problems, printedProblem)
	}
	document := statement.NewDocument(doc)
	pdf, err := w.Typeset.PDF(ctx, document.Source, files)
	if err != nil {
		return explain(document, doc, err)
	}
	sum := sha256.Sum256(pdf)
	object := problemsmodel.NewStoredObject(job.OrganizationID, problemsmodel.UploadSpec{Purpose: "booklet", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(pdf))})
	if err := w.Store.UploadFileWithSHA256(ctx, w.Bucket, object.Key, bytes.NewReader(pdf), "application/pdf", object.SHA256); err != nil {
		return fmt.Errorf("upload booklet: %w", err)
	}
	return w.Queue.CompleteBooklet(ctx, job, object)
}

// explain names the problem a LaTeX error comes from, so organizers know which package to fix.
func explain(document statement.Document, doc statement.Contest, err error) error {
	var tex *sandbox.TeXError
	if !errors.As(err, &tex) {
		return err
	}
	explained := document.Explain(err)
	if problem, _, _, ok := document.Locate(tex.Line); ok {
		return fmt.Errorf("problem %s: %w", doc.Problems[problem].Letter, explained)
	}
	return explained
}
