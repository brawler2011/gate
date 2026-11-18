package problems

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"slices"
	"strings"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackageContent struct {
	Repo interface {
		GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error)
		GetPackageStatement(context.Context, uuid.UUID) (model.PackageStatement, error)
		FindPackageAssets(context.Context, uuid.UUID, string) ([]model.PackageAsset, error)
	}
	Store  DraftObjectStore
	Bucket string
}

// OpenImage exposes only statement assets, never tests or executables.
func (c *PackageContent) OpenImage(ctx context.Context, id uuid.UUID, sha256 string) (io.ReadCloser, string, error) {
	assets, err := c.Repo.FindPackageAssets(ctx, id, sha256)
	if err != nil {
		return nil, "", err
	}
	for _, a := range assets {
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(a.Name)))
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
	return selectStatement(d.Title, d.Statements, language)
}

// Missing languages fall back to English, then alphabetical order.
func selectStatement(title string, statements []model.DraftStatement, language string) *model.Statement {
	find := func(language string) *model.DraftStatement {
		i := slices.IndexFunc(statements, func(s model.DraftStatement) bool { return s.Language == language })
		if i < 0 {
			return nil
		}
		return &statements[i]
	}
	s := find(language)
	if s == nil {
		s = find("en")
	}
	if s == nil && len(statements) > 0 {
		first := slices.MinFunc(statements, func(a, b model.DraftStatement) int { return strings.Compare(a.Language, b.Language) })
		s = &first
	}
	statement := model.Statement{Title: title}
	if s != nil {
		statement.Legend, statement.InputFormat, statement.OutputFormat = s.Legend, s.Input, s.Output
		statement.Notes, statement.Interaction, statement.Scoring = s.Notes, s.Interaction, s.Scoring
	}
	return &statement
}

// ReadStatement leaves Statement nil when the package is unavailable.
func (c *PackageContent) ReadStatement(ctx context.Context, problemID, packageID uuid.UUID, language string) model.StatementContent {
	p, err := c.Repo.GetPackageStatement(ctx, packageID)
	if err != nil || p.ProblemID != problemID {
		if err != nil && !errors.Is(err, pkg.ErrNotFound) {
			slog.ErrorContext(ctx, "read package statement", "error", err, "package_id", packageID)
		}
		return model.StatementContent{Samples: []model.Sample{}, Assets: map[string]string{}}
	}
	return model.StatementContent{Statement: selectStatement(p.Title, p.Statements, language), Samples: p.Samples, Assets: p.Assets}
}
