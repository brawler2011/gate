// Package booklet lets API instances, which have no sandbox, compile booklets on a worker over HTTP.
package booklet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

const Path = "/booklet"

// Requests wait for the booklet's wall-clock limit plus this much for queueing and transfer.
const transferTime = 30 * time.Second

type Compiler interface {
	CompilePDF(ctx context.Context, texSource string, limits sandbox.FileRunLimits) ([]byte, error)
}

// Handler compiles the LaTeX request body; failures answer 422 with the compile log.
func Handler(compiler Compiler, limits sandbox.FileRunLimits) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tex, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pdf, err := compiler.CompilePDF(r.Context(), string(tex), limits)
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to compile booklet", "error", err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	}
}

type Client struct {
	url    string
	client *http.Client
}

func NewClient(workerURL string, limits sandbox.FileRunLimits) *Client {
	return &Client{url: workerURL + Path, client: &http.Client{Timeout: limits.Wall + transferTime}}
}

func (c *Client) CompilePDF(ctx context.Context, texSource string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBufferString(texSource))
	if err != nil {
		return nil, fmt.Errorf("compile booklet: %w", err)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("compile booklet on a worker: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read booklet from a worker: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("compile booklet on a worker: %s: %s", res.Status, body)
	}
	return body, nil
}

// Local compiles in this process, for instances that run both the API and a sandbox.
type Local struct {
	Compiler Compiler
	Limits   sandbox.FileRunLimits
}

func (l Local) CompilePDF(ctx context.Context, texSource string) ([]byte, error) {
	return l.Compiler.CompilePDF(ctx, texSource, l.Limits)
}
