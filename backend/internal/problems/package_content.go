package problems

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"sort"
	"strings"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// PackageContent reads the statement-facing content of built packages. It is
// stored at publication, so no read here opens a package archive.
type PackageContent struct {
	Repo interface {
		GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error)
		GetPackageStatement(context.Context, uuid.UUID) (model.PackageStatement, error)
		FindPackageAssets(context.Context, uuid.UUID, string) ([]model.PackageAsset, error)
	}
	Store  DraftObjectStore
	Bucket string
}

// OpenImage opens a statement image of the package by its content hash and
// returns it with its content type. Only statement assets are reachable, never
// tests or executables.
func (c *PackageContent) OpenImage(ctx context.Context, id uuid.UUID, sha256 string) (io.ReadCloser, string, error) {
	assets, err := c.Repo.FindPackageAssets(ctx, id, sha256)
	if err != nil {
		return nil, "", err
	}
	for _, a := range assets {
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(a.Path)))
		if !strings.HasPrefix(contentType, "image/") {
			continue
		}
		r, _, err := c.Store.DownloadFile(ctx, c.Bucket, a.Key, nil)
		if err != nil {
			return nil, "", err
		}
		return r, contentType, nil
	}
	return nil, "", pkg.ErrNotFound
}
func statementFromDraft(d model.Draft, language string) *model.Statement {
	languages := []string{}
	for path := range d.Files {
		if strings.HasPrefix(path, "statements/") && strings.HasSuffix(path, ".md") {
			languages = append(languages, strings.TrimSuffix(strings.TrimPrefix(path, "statements/"), ".md"))
		}
	}
	sort.Strings(languages)
	if _, ok := d.Files["statements/"+language+".md"]; !ok {
		language = "en"
		if _, ok := d.Files["statements/en.md"]; !ok && len(languages) > 0 {
			language = languages[0]
		}
	}
	statement := ParseStatementMarkdown(d.Files["statements/"+language+".md"])
	statement.Title = d.Title
	return &statement
}
// ReadStatement returns the package statement in the requested language with
// the samples and statement asset hashes, or a nil statement when the package
// is unavailable.
func (c *PackageContent) ReadStatement(ctx context.Context, problemID, packageID uuid.UUID, language string) model.StatementContent {
	p, err := c.Repo.GetPackageStatement(ctx, packageID)
	if err != nil || p.ProblemID != problemID {
		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			slog.ErrorContext(ctx, "read package statement", "error", err, "package_id", packageID)
		}
		return model.StatementContent{Samples: []model.Sample{}, Assets: map[string]string{}}
	}
	return model.StatementContent{Statement: statementFromDraft(p.Draft, language), Samples: p.Samples, Assets: p.Assets}
}
