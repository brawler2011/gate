package problems

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackageContent struct {
	Repo interface {
		GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error)
	}
	Store           DraftObjectStore
	Bucket          string
	TempDir         string
	MaxArchiveBytes int64
	MaxEntries      int
}
type contentArchive struct {
	zip  *zip.ReadCloser
	root string
}

func (a *contentArchive) Close() error {
	err := a.zip.Close()
	removeErr := os.RemoveAll(a.root)
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	if removeErr != nil {
		return fmt.Errorf("remove archive directory: %w", removeErr)
	}
	return nil
}
func (c *PackageContent) openArchive(ctx context.Context, id uuid.UUID) (*contentArchive, error) {
	p, err := c.Repo.GetPackageDescriptor(ctx, id)
	if err != nil {
		return nil, err
	}
	o := p.Object
	if o.State != "available" || o.Size > c.MaxArchiveBytes {
		return nil, pkg.ErrConflict
	}
	root, err := os.MkdirTemp(c.TempDir, "content-")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(root)
		}
	}()
	path := filepath.Join(root, "package.zip")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	r, _, err := c.Store.DownloadFile(ctx, c.Bucket, o.Key, nil)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, o.Size+1))
	fileErr, readErr := f.Close(), r.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("copy: %w", copyErr)
	}
	if fileErr != nil {
		return nil, fmt.Errorf("close: %w", fileErr)
	}
	if readErr != nil {
		return nil, fmt.Errorf("close: %w", readErr)
	}
	if n != o.Size || hex.EncodeToString(hash.Sum(nil)) != o.SHA256 {
		return nil, fmt.Errorf("package content checksum mismatch")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("openreader: %w", err)
	}
	if len(z.File) > c.MaxEntries {
		_ = z.Close()
		return nil, fmt.Errorf("package entry quota exceeded")
	}
	ok = true
	return &contentArchive{zip: z, root: root}, nil
}

type packageFile struct {
	io.ReadCloser
	archive *contentArchive
}

func (f *packageFile) Close() error {
	err := f.ReadCloser.Close()
	archiveErr := f.archive.Close()
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return archiveErr
}
func (c *PackageContent) OpenFile(ctx context.Context, id uuid.UUID, path string) (io.ReadCloser, error) {
	if !model.ValidDraftPath(path) {
		return nil, pkg.ErrBadInput
	}
	p, err := c.Repo.GetPackageDescriptor(ctx, id)
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, a := range p.Snapshot.Draft.Assets {
		if a.Path == path {
			allowed = true
			break
		}
	}
	// This endpoint exposes only statement resources, never tests or executables.
	if !allowed {
		return nil, pkg.ErrNotFound
	}
	a, err := c.openArchive(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, entry := range a.zip.File {
		if entry.Name == path {
			reader, err := entry.Open()
			if err != nil {
				_ = a.Close()
				return nil, fmt.Errorf("open: %w", err)
			}
			return &packageFile{ReadCloser: reader, archive: a}, nil
		}
	}
	_ = a.Close()
	return nil, pkg.ErrNotFound
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
func (c *PackageContent) ReadStatement(ctx context.Context, problemID, packageID uuid.UUID, language string) (*model.Statement, []model.Sample) {
	p, err := c.Repo.GetPackageDescriptor(ctx, packageID)
	if err != nil || p.ProblemID != problemID {
		return nil, nil
	}
	statement := statementFromDraft(p.Snapshot.Draft, language)
	samples := []model.Sample{}
	needSamples := false
	for _, t := range p.Snapshot.Draft.Tests {
		needSamples = needSamples || t.Sample
	}
	if !needSamples {
		return statement, samples
	}
	a, err := c.openArchive(ctx, packageID)
	if err != nil {
		slog.ErrorContext(ctx, "open package samples", "error", err, "package_id", packageID)
		return statement, samples
	}
	defer a.Close()
	byPath := make(map[string]*zip.File, len(a.zip.File))
	for _, entry := range a.zip.File {
		byPath[entry.Name] = entry
	}
	read := func(path string) (string, error) {
		if path == "" {
			return "", nil
		}
		entry, ok := byPath[path]
		if !ok {
			return "", pkg.ErrNotFound
		}
		f, err := entry.Open()
		if err != nil {
			return "", fmt.Errorf("open: %w", err)
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, 128*1024+1))
		if len(data) > 128*1024 {
			data = append(data[:128*1024], []byte("\n…")...)
		}
		return string(data), err
	}
	for _, t := range p.Snapshot.Draft.Tests {
		if !t.Sample {
			continue
		}
		inputPath := ".gate/tests/" + t.ID.String() + ".in"
		if t.Input.Mode == "manual" {
			inputPath = ".gate/objects/" + t.Input.ObjectID.String()
		}
		answerPath := ""
		if t.Answer.Mode == "generated" {
			answerPath = ".gate/tests/" + t.ID.String() + ".ans"
		}
		if t.Answer.Mode == "provided" {
			answerPath = ".gate/objects/" + t.Answer.ObjectID.String()
		}
		input, inErr := read(inputPath)
		answer, ansErr := read(answerPath)
		if inErr != nil || ansErr != nil {
			slog.ErrorContext(ctx, "read package sample", "input_error", inErr, "answer_error", ansErr)
			continue
		}
		samples = append(samples, model.Sample{Input: input, Output: answer})
	}
	return statement, samples
}
