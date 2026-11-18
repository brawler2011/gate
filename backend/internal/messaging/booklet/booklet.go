// Package booklet delegates compilation to workers because API instances have no sandbox.
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
	// A shared queue group sends each compilation request to one worker.
	queue = "booklet_compilers"
	// Error replies carry compile diagnostics instead of PDF bytes.
	errorHeader = "Gate-Booklet-Error"
	// Bound LaTeX logs to keep error replies within the NATS payload limit.
	maxErrorBytes = 64 << 10
)

// Allow the sandbox compilation limit plus transfer time.
const RequestTimeout = 90 * time.Second

type Compiler interface {
	CompilePDF(ctx context.Context, texSource string) ([]byte, error)
}

type Client struct {
	conn *nats.Conn
}

func NewClient(conn *nats.Conn) *Client {
	return &Client{conn: conn}
}

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

type Responder struct {
	conn     *nats.Conn
	compiler Compiler
	logger   *slog.Logger
}

func NewResponder(conn *nats.Conn, compiler Compiler) *Responder {
	return &Responder{conn: conn, compiler: compiler, logger: slog.Default().With("component", "booklet_responder")}
}

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
