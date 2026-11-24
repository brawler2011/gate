package problems

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"slices"
	"strings"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
)

type PackageContent struct {
	Repo interface {
		GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error)
		GetPackageStatement(context.Context, uuid.UUID) (model.PackageStatement, error)
		FindPackageAssets(context.Context, uuid.UUID, string) ([]model.PackageAsset, error)
		FindPackagePDF(ctx context.Context, packageID uuid.UUID, language string) (key string, err error)
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

// Missing languages fall back to English, then alphabetical order.
func pickStatement(statements []model.BuiltStatement, language string) *model.BuiltStatement {
	for _, l := range []string{language, "en"} {
		if i := slices.IndexFunc(statements, func(s model.BuiltStatement) bool { return s.Language == l }); i >= 0 {
			return &statements[i]
		}
	}
	if len(statements) == 0 {
		return nil
	}
	first := slices.MinFunc(statements, func(a, b model.BuiltStatement) int { return strings.Compare(a.Language, b.Language) })
	return &first
}

func selectStatement(title string, statements []model.BuiltStatement, language string) *model.Statement {
	statement := model.Statement{Title: title}
	if s := pickStatement(statements, language); s != nil {
		statement.Language = s.Language
		statement.Legend, statement.InputFormat, statement.OutputFormat = s.HTML["legend"], s.HTML["input"], s.HTML["output"]
		statement.Notes, statement.Interaction, statement.Scoring = s.HTML["notes"], s.HTML["interaction"], s.HTML["scoring"]
		if s.Title != "" {
			statement.Title = s.Title
		}
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

// OpenPDF opens the package's statement PDF in the language the statement shows.
func (c *PackageContent) OpenPDF(ctx context.Context, problemID, packageID uuid.UUID, language string) (io.ReadCloser, error) {
	p, err := c.Repo.GetPackageStatement(ctx, packageID)
	if err != nil {
		return nil, err
	}
	s := pickStatement(p.Statements, language)
	if p.ProblemID != problemID || s == nil {
		return nil, pkg.ErrNotFound
	}
	key, err := c.Repo.FindPackagePDF(ctx, packageID, s.Language)
	if err != nil {
		return nil, err
	}
	r, _, err := c.Store.DownloadFile(ctx, c.Bucket, key, nil)
	return r, err
}

// Printed is what a PDF of the package's statement needs.
type Printed struct {
	Statement model.DraftStatement
	Samples   []model.Sample
	// Images the statement shows, by asset name.
	Images map[string][]byte
}

// ReadPrinted reads the package's statement in the language the site shows, with its images.
func (c *PackageContent) ReadPrinted(ctx context.Context, problemID, packageID uuid.UUID, language string) (Printed, error) {
	p, err := c.Repo.GetPackageStatement(ctx, packageID)
	if err != nil {
		return Printed{}, err
	}
	s := pickStatement(p.Statements, language)
	if p.ProblemID != problemID || s == nil {
		return Printed{}, pkg.ErrNotFound
	}
	printed := Printed{Statement: s.DraftStatement, Samples: p.Samples, Images: map[string][]byte{}}
	if printed.Statement.Title == "" {
		printed.Statement.Title = p.Title
	}
	for _, name := range statement.Order {
		for _, ref := range model.ImageReferences(*s.Section(name)) {
			if _, done := printed.Images[ref]; done || p.Assets[ref] == "" {
				continue
			}
			if printed.Images[ref], err = c.readAsset(ctx, packageID, ref, p.Assets[ref]); err != nil {
				return Printed{}, err
			}
		}
	}
	return printed, nil
}

func (c *PackageContent) readAsset(ctx context.Context, packageID uuid.UUID, name, sha256 string) ([]byte, error) {
	assets, err := c.Repo.FindPackageAssets(ctx, packageID, sha256)
	if err != nil {
		return nil, err
	}
	for _, a := range assets {
		if a.Name == name {
			r, _, err := c.Store.DownloadFile(ctx, c.Bucket, a.Key, nil)
			if err != nil {
				return nil, err
			}
			defer r.Close()
			data, err := io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("read asset %s: %w", name, err)
			}
			return data, nil
		}
	}
	return nil, pkg.ErrNotFound
}
