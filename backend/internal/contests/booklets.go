package contests

import (
	"context"
	"io"
	"slices"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type BookletStore interface {
	ListBooklets(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error)
}
type BookletFiles interface {
	DownloadFile(ctx context.Context, bucket, key string, version *string) (io.ReadCloser, string, error)
}

// Booklets serves the booklets workers built ahead, one per statement language of the contest's problems.
type Booklets struct {
	Store  BookletStore
	Files  BookletFiles
	Bucket string
	Access access.Loader
}

type BookletStatus struct {
	Language string
	// Empty while the booklet for the current inputs is not built yet.
	State      contestsmodel.BookletState
	Diagnostic string
}

// List shows build diagnostics only to contest managers.
func (b *Booklets) List(ctx context.Context, contestID uuid.UUID) ([]BookletStatus, error) {
	a, err := statementsAccess(ctx, b.Access, contestID)
	if err != nil {
		return nil, err
	}
	booklets, err := b.Store.ListBooklets(ctx, contestID)
	if err != nil {
		return nil, err
	}
	result := make([]BookletStatus, len(booklets))
	for i, booklet := range booklets {
		result[i].Language = booklet.Language
		if stored, ok := booklet.Current(); ok {
			result[i].State = stored.State
			if a.Can(accessmodel.ContestManage) {
				result[i].Diagnostic = stored.Diagnostic
			}
		}
	}
	return result, nil
}

// Open serves only a booklet built from the contest's current inputs.
func (b *Booklets) Open(ctx context.Context, contestID uuid.UUID, language string) (io.ReadCloser, error) {
	if _, err := statementsAccess(ctx, b.Access, contestID); err != nil {
		return nil, err
	}
	booklets, err := b.Store.ListBooklets(ctx, contestID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(booklets, func(x contestsmodel.Booklet) bool { return x.Language == language })
	if i < 0 {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "no statements in this language")
	}
	stored, ok := booklets[i].Current()
	if !ok || stored.State != contestsmodel.BookletSucceeded {
		return nil, pkg.Wrap(pkg.ErrBookletNotReady, nil, "the booklet is not built yet")
	}
	file, _, err := b.Files.DownloadFile(ctx, b.Bucket, stored.ObjectKey, nil)
	return file, err
}

func statementsAccess(ctx context.Context, loader access.Loader, contestID uuid.UUID) (access.ContestAccess, error) {
	a, err := access.Contest(ctx, loader, contestID, accessmodel.ContestView)
	if err != nil {
		return a, err
	}
	if !a.Can(accessmodel.ContestViewProblems) {
		if start := a.Facts.Contest.StartTime; start != nil && a.Facts.Now.Before(*start) {
			return a, pkg.Wrap(pkg.ErrContestNotStarted, nil, "contest has not started yet")
		}
		return a, pkg.ErrForbidden
	}
	if !a.Can(accessmodel.ContestViewStatements) {
		return a, pkg.Wrap(pkg.ErrStatementsHidden, nil, "statements are hidden for this contest")
	}
	return a, nil
}
