package contests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"maps"
	"slices"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	contestsmodel "github.com/brawler2011/gate/backend/internal/contests/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/booklet"
	"github.com/google/uuid"
)

type BookletStore interface {
	// QueueBooklet keeps the booklet when its source is unchanged.
	QueueBooklet(ctx context.Context, contestID uuid.UUID, language, source, sourceSHA256 string) error
	KeepBooklets(ctx context.Context, contestID uuid.UUID, languages []string) error
	ListBooklets(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error)
}
type BookletContests interface {
	GetContest(context.Context, uuid.UUID) (contestsmodel.Contest, error)
	GetContestProblems(context.Context, uuid.UUID) ([]contestsmodel.ContestProblem, error)
}
type BookletStatements interface {
	GetPackageStatement(context.Context, uuid.UUID) (problemsmodel.PackageStatement, error)
}
type BookletFiles interface {
	DownloadFile(ctx context.Context, bucket, key string, version *string) (io.ReadCloser, string, error)
}

// Booklets serves contest booklets that workers compiled, one per statement language of the contest's problems.
type Booklets struct {
	Contests   BookletContests
	Statements BookletStatements
	Store      BookletStore
	Files      BookletFiles
	Bucket     string
	Access     access.Loader
}

// Refresh queues the booklets whose source changed; contest changes call it, and downloads repair what it missed.
func (b *Booklets) Refresh(ctx context.Context, contestID uuid.UUID) error {
	contest, err := b.Contests.GetContest(ctx, contestID)
	if err != nil {
		return err
	}
	sources, err := b.sources(ctx, contest)
	if err != nil {
		return err
	}
	for language, source := range sources {
		if err := b.Store.QueueBooklet(ctx, contestID, language, source, digest(source)); err != nil {
			return err
		}
	}
	return b.Store.KeepBooklets(ctx, contestID, slices.Sorted(maps.Keys(sources)))
}

// List shows build diagnostics only to contest managers.
func (b *Booklets) List(ctx context.Context, contestID uuid.UUID) ([]contestsmodel.Booklet, error) {
	a, err := statementsAccess(ctx, b.Access, contestID)
	if err != nil {
		return nil, err
	}
	booklets, err := b.Store.ListBooklets(ctx, contestID)
	if err != nil {
		return nil, err
	}
	for i := range booklets {
		booklets[i].ObjectKey = ""
		if !a.Can(accessmodel.ContestManage) {
			booklets[i].Diagnostic = ""
		}
	}
	return booklets, nil
}

// Open never serves a booklet whose source is out of date; it queues the rebuild instead.
func (b *Booklets) Open(ctx context.Context, contestID uuid.UUID, language string) (io.ReadCloser, error) {
	a, err := statementsAccess(ctx, b.Access, contestID)
	if err != nil {
		return nil, err
	}
	sources, err := b.sources(ctx, a.Facts.Contest)
	if err != nil {
		return nil, err
	}
	source, ok := sources[language]
	if !ok {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "no statements in this language")
	}
	booklets, err := b.Store.ListBooklets(ctx, contestID)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(booklets, func(x contestsmodel.Booklet) bool { return x.Language == language })
	switch {
	case i < 0 || booklets[i].SourceSHA256 != digest(source):
		if err := b.Store.QueueBooklet(ctx, contestID, language, source, digest(source)); err != nil {
			return nil, err
		}
		return nil, pkg.ErrBookletNotReady
	case booklets[i].State == contestsmodel.BookletFailed:
		return nil, pkg.ErrBookletFailed
	case booklets[i].State != contestsmodel.BookletSucceeded:
		return nil, pkg.ErrBookletNotReady
	}
	file, _, err := b.Files.DownloadFile(ctx, b.Bucket, booklets[i].ObjectKey, nil)
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

// sources has a booklet for every language some problem of the contest has a statement in.
func (b *Booklets) sources(ctx context.Context, contest contestsmodel.Contest) (map[string]string, error) {
	links, err := b.Contests.GetContestProblems(ctx, contest.ID)
	if err != nil {
		return nil, err
	}
	problems := make([]problemsmodel.BookletProblem, len(links))
	languages := map[string]bool{}
	for i, link := range links {
		p, err := b.Statements.GetPackageStatement(ctx, link.PackageID)
		if err != nil {
			return nil, err
		}
		for _, s := range p.Statements {
			languages[s.Language] = true
		}
		problems[i] = problemsmodel.BookletProblem{Letter: ordinalToLetter(link.Ordinal), Title: link.Title, TimeLimitMs: link.TimeLimitMs, MemoryLimitMb: link.MemoryLimitMb,
			InputFile: link.InputFile, OutputFile: link.OutputFile, Statements: p.Statements, Samples: p.Samples}
	}
	date := ""
	if contest.StartTime != nil {
		date = contest.StartTime.Format("02.01.2006")
	}
	sources := make(map[string]string, len(languages))
	for language := range languages {
		data := booklet.ContestData{Title: contest.Title, Organization: contest.OrganizationLogin, Date: date, Language: language}
		for _, p := range problems {
			data.Problems = append(data.Problems, p.Data(language))
		}
		source, err := booklet.GenerateLatex(data)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to generate LaTeX booklet")
		}
		sources[language] = source
	}
	return sources, nil
}

func digest(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func ordinalToLetter(ordinal int) string {
	if ordinal <= 0 {
		return "A"
	}
	res := ""
	for ordinal > 0 {
		rem := (ordinal - 1) % 26
		res = string(rune('A'+rem)) + res
		ordinal = (ordinal - 1) / 26
	}
	return res
}
