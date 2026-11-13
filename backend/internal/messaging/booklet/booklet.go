// Package booklet compiles contest booklets on judge workers: API instances have
// no sandbox, so they send the LaTeX source over NATS request-reply and a worker
// answers with the PDF.
package booklet

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	subject = "booklets.compile"
	// All workers share one queue group, so each request reaches one of them.
	queue = "booklet_compilers"
	// errorHeader marks a reply whose body is a worker's compile error, not a PDF.
	errorHeader = "Gate-Booklet-Error"
	// maxErrorBytes keeps an error reply, which may hold a LaTeX log, well within
	// the NATS payload limit.
	maxErrorBytes = 64 << 10
)

// RequestTimeout covers the sandbox limit of a booklet compilation
// (pkg/sandbox) plus the transfer.
const RequestTimeout = 90 * time.Second

// Compiler compiles a LaTeX booklet into a PDF.
type Compiler interface {
	CompilePDF(ctx context.Context, texSource string) ([]byte, error)
}

// Client asks a worker to compile a booklet.
type Client struct {
	conn *nats.Conn
}

func NewClient(conn *nats.Conn) *Client {
	return &Client{conn: conn}
}

// CompilePDF sends the source to a worker and waits for its PDF.
func (c *Client) CompilePDF(ctx context.Context, texSource string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	reply, err := c.conn.RequestMsgWithContext(ctx, &nats.Msg{Subject: subject, Data: []byte(texSource)})
	if errors.Is(err, nats.ErrNoResponders) {
		return nil, fmt.Errorf("compile booklet: no judge worker is running: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("compile booklet: %w", err)
	}
	if reply.Header.Get(errorHeader) != "" {
		return nil, fmt.Errorf("compile booklet on a worker: %s", reply.Data)
	}
	return reply.Data, nil
}

// Responder compiles the booklets that clients request.
type Responder struct {
	conn     *nats.Conn
	compiler Compiler
	logger   *slog.Logger
}

func NewResponder(conn *nats.Conn, compiler Compiler) *Responder {
	return &Responder{conn: conn, compiler: compiler, logger: slog.Default().With("component", "booklet_responder")}
}

// Run answers requests one at a time until ctx ends.
func (r *Responder) Run(ctx context.Context) error {
	sub, err := r.conn.QueueSubscribeSync(subject, queue)
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", subject, err)
	}
	defer func() { _ = sub.Unsubscribe() }()
	for {
		msg, err := sub.NextMsgWithContext(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receive booklet request: %w", err)
		}
		r.respond(ctx, msg)
	}
}

func (r *Responder) respond(ctx context.Context, msg *nats.Msg) {
	pdf, err := r.compiler.CompilePDF(ctx, string(msg.Data))
	if err == nil {
		err = msg.Respond(pdf)
		if errors.Is(err, nats.ErrMaxPayload) {
			err = fmt.Errorf("booklet PDF is %d bytes, more than NATS carries (%d)", len(pdf), r.conn.MaxPayload())
		}
	}
	if err == nil {
		return
	}
	r.logger.ErrorContext(ctx, "failed to compile booklet", "error", err)
	failure := nats.NewMsg(msg.Reply)
	failure.Header.Set(errorHeader, "1")
	message := err.Error()
	if len(message) > maxErrorBytes {
		message = message[:maxErrorBytes]
	}
	failure.Data = []byte(message)
	if err := msg.RespondMsg(failure); err != nil {
		r.logger.ErrorContext(ctx, "failed to reply with booklet error", "error", err)
	}
}
