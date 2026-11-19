package sandbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// A go-judge that dies mid-response must be retried, not reported as a compile or checker failure.
func TestRunTruncatedResponseIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"status":"Acc`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	require.NoError(t, err)
	_, err = client.run(context.Background(), request{Cmd: []cmd{{Args: []string{"/bin/true"}}}})
	require.ErrorIs(t, err, ErrUnavailable)
}
