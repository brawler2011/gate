// This fixture serves the real workshop handler and generated router for the
// frontend HTTP-client integration regression.
package main

import (
	"context"
	"fmt"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/interfaces"
	"github.com/brawler2011/gate/backend/internal/transport/rest/core"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
)

type fixtureRepo struct {
	interfaces.ProblemsRepo
	mu   sync.Mutex
	hash string
}

func (r *fixtureRepo) GetWorkspaceHash(context.Context, uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hash, nil
}
func (r *fixtureRepo) CommitWorkspace(_ context.Context, _ uuid.UUID, previousHash, hash string, _ *models.WorkspaceMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hash != previousHash {
		return pkg.ErrConflict
	}
	r.hash = hash
	return nil
}

type fixtureSecurity struct{}

func (fixtureSecurity) HandleCookieAuth(ctx context.Context, _ corev1.OperationName, _ corev1.CookieAuth) (context.Context, error) {
	return ctx, nil
}

func run() error {
	id := uuid.MustParse(os.Args[3])
	repo := &fixtureRepo{}
	ws := usecase.NewWorkspaceStorage(storage.NewLocalStorage(os.Args[1]), "fixture", repo)
	if err := ws.WriteFile(context.Background(), id, "problem.yaml", []byte("type: pass-fail\nsubtasks: {}\n")); err != nil {
		return err
	}
	for _, name := range []string{"nested/edge.in", "nested/edge.ans", "nested/space #?.in"} {
		if err := ws.WriteFile(context.Background(), id, "tests/"+name, []byte{0, 255, 128, 10}); err != nil {
			return err
		}
	}
	uc := usecase.NewWorkshopUseCase(ws, nil)
	handler := core.NewCoreServer(nil, nil, nil, nil, nil, nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router, err := corev1.NewServer(handler, fixtureSecurity{}, corev1.WithErrorHandler(func(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
		w.WriteHeader(pkg.ToREST(err))
	}))
	if err != nil {
		return fmt.Errorf("create workshop router: %w", err)
	}
	server := &http.Server{Addr: os.Args[2], ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "session_id", Value: "fixture"})
		router.ServeHTTP(w, r)
	})}
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("serve workshop fixture: %w", err)
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		slog.Error("workshop HTTP fixture failed", "error", err)
		os.Exit(1)
	}
}
