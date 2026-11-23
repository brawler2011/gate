package booklet

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/stretchr/testify/require"
)

type fakeCompiler struct{}

func (fakeCompiler) CompilePDF(_ context.Context, tex string, _ sandbox.FileRunLimits) ([]byte, error) {
	if tex == "broken" {
		return nil, errors.New("LaTeX compilation error:\n! Undefined control sequence.")
	}
	return []byte("%PDF " + tex), nil
}

func TestBookletCompiledOnWorker(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("POST "+Path, Handler(fakeCompiler{}, sandbox.FileRunLimits{}))
	worker := httptest.NewServer(mux)
	client := NewClient(worker.URL, sandbox.FileRunLimits{})

	pdf, err := client.CompilePDF(context.Background(), "contest")
	require.NoError(t, err)
	require.Equal(t, "%PDF contest", string(pdf))

	_, err = client.CompilePDF(context.Background(), "broken")
	require.ErrorContains(t, err, "! Undefined control sequence.")

	worker.Close()
	_, err = client.CompilePDF(context.Background(), "contest")
	require.Error(t, err, "an unreachable worker fails the request")
}
