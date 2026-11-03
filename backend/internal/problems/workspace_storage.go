package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type WorkspaceStorage struct {
	storage storage.Storage
	bucket  string
	repo    WorkspaceRepo
}

func NewWorkspaceStorage(storage storage.Storage, bucket string, repo WorkspaceRepo) *WorkspaceStorage {
	return &WorkspaceStorage{
		storage: storage,
		bucket:  bucket,
		repo:    repo,
	}
}

// The editable GFMT package is stored as one ZIP object. Each operation reads
// one package and commits all of its changes with one atomic reference update.
// There is no database manifest, file cache, or repair on read.
type workspacePackage struct {
	files map[string][]byte
	hash  string
	dirty bool
}

type workspacePackageKey struct {
	storage *WorkspaceStorage
	id      uuid.UUID
}

func (w *WorkspaceStorage) getWorkspaceKey(problemID uuid.UUID, hash string) string {
	return fmt.Sprintf("workspaces/%s/%s.zip", problemID, hash)
}

func (w *WorkspaceStorage) withPackage(ctx context.Context, id uuid.UUID, fn func(context.Context, *workspacePackage) error) error {
	key := workspacePackageKey{w, id}
	if pkg, ok := ctx.Value(key).(*workspacePackage); ok {
		return fn(ctx, pkg)
	}
	pkg, err := w.loadPackage(ctx, id)
	if err != nil {
		return err
	}
	return w.updatePackage(ctx, id, pkg, fn)
}

func (w *WorkspaceStorage) loadPackage(ctx context.Context, id uuid.UUID) (*workspacePackage, error) {
	pkg := &workspacePackage{files: make(map[string][]byte)}
	hash, err := w.repo.GetWorkspaceHash(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("read workspace reference: %w", err)
	}
	pkg.hash = hash
	if hash != "" {
		reader, _, err := w.storage.DownloadFile(ctx, w.bucket, w.getWorkspaceKey(id, hash), nil)
		if err != nil {
			return nil, fmt.Errorf("read workspace package: %w", err)
		}

		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, fmt.Errorf("read workspace package: %w", err)
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, fmt.Errorf("open workspace package: %w", err)
		}
		for _, file := range archive.File {
			if file.FileInfo().IsDir() {
				continue
			}
			if !fs.ValidPath(file.Name) || file.Mode()&fs.ModeSymlink != 0 {
				return nil, fmt.Errorf("invalid package path %q", file.Name)
			}
			content, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("open package file %s: %w", file.Name, err)
			}
			data, readErr := io.ReadAll(io.LimitReader(content, 500*1024*1024+1))
			closeErr := content.Close()
			if err := errors.Join(readErr, closeErr); err != nil {
				return nil, err
			}
			if len(data) > 500*1024*1024 {
				return nil, fmt.Errorf("package file too large: %s", file.Name)
			}
			pkg.files[file.Name] = data
		}
	}
	return pkg, nil
}

// prepareChange uploads an immutable candidate without changing the database reference.
func (w *WorkspaceStorage) prepareChange(ctx context.Context, id uuid.UUID, fn func(context.Context) error) (string, *workspaceVersion, error) {
	current, err := w.loadPackage(ctx, id)
	if err != nil {
		return "", nil, err
	}
	if err := fn(context.WithValue(ctx, workspacePackageKey{w, id}, current)); err != nil {
		return "", nil, err
	}
	if !current.dirty {
		return current.hash, nil, nil
	}
	version, err := w.uploadPackage(ctx, id, current)
	if err != nil {
		return "", nil, err
	}
	return current.hash, &version, nil
}

// Replacement imports need only the current reference, so a complete valid
// package can replace a missing or corrupt previous archive.
func (w *WorkspaceStorage) replacePackage(ctx context.Context, id uuid.UUID, fn func(context.Context, *workspacePackage) error) error {
	hash, err := w.repo.GetWorkspaceHash(ctx, id)
	if err != nil {
		return err
	}
	return w.updatePackage(ctx, id, &workspacePackage{files: make(map[string][]byte), hash: hash}, fn)
}

type workspaceVersion struct {
	hash     string
	metadata *problemsmodel.WorkspaceMetadata
}

func (w *WorkspaceStorage) preparePackage(ctx context.Context, id uuid.UUID, fn func(context.Context) error) (workspaceVersion, error) {
	pkg := &workspacePackage{files: make(map[string][]byte)}
	ctx = context.WithValue(ctx, workspacePackageKey{w, id}, pkg)
	if err := fn(ctx); err != nil {
		return workspaceVersion{}, err
	}
	return w.uploadPackage(ctx, id, pkg)
}

func (w *WorkspaceStorage) updatePackage(ctx context.Context, id uuid.UUID, pkg *workspacePackage, fn func(context.Context, *workspacePackage) error) error {
	if err := fn(context.WithValue(ctx, workspacePackageKey{w, id}, pkg), pkg); err != nil {
		return err
	}
	if !pkg.dirty {
		return nil
	}
	version, err := w.uploadPackage(ctx, id, pkg)
	if err != nil {
		return err
	}
	if version.hash == pkg.hash {
		return nil
	}
	if err := w.repo.CommitWorkspace(ctx, id, pkg.hash, version.hash, version.metadata); err != nil {
		return fmt.Errorf("commit workspace package: %w", err)
	}
	pkg.hash = version.hash
	return nil
}

func (w *WorkspaceStorage) uploadPackage(ctx context.Context, id uuid.UUID, pkg *workspacePackage) (workspaceVersion, error) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	names := make([]string, 0, len(pkg.files))
	for name := range pkg.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := archive.Create(name)
		if err != nil {
			return workspaceVersion{}, fmt.Errorf("create package file %s: %w", name, err)
		}
		if _, err := entry.Write(pkg.files[name]); err != nil {
			return workspaceVersion{}, err
		}
	}
	if err := archive.Close(); err != nil {
		return workspaceVersion{}, fmt.Errorf("close package archive: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(buffer.Bytes()))
	if hash == pkg.hash {
		return workspaceVersion{hash: pkg.hash}, nil
	}
	var metadata *problemsmodel.WorkspaceMetadata
	if data, ok := pkg.files["problem.yaml"]; ok {
		var config *gfmt.Problem
		if err := yaml.Unmarshal(data, &config); err != nil {
			return workspaceVersion{}, fmt.Errorf("invalid problem.yaml: %w", err)
		}
		if config == nil {
			return workspaceVersion{}, fmt.Errorf("problem.yaml must contain a mapping")
		}
		metadata = &problemsmodel.WorkspaceMetadata{Title: config.Title, TimeLimitMs: config.Limits.TimeMs, MemoryLimitMb: config.Limits.MemoryMb}
	}
	if err := w.storage.UploadFile(ctx, w.bucket, w.getWorkspaceKey(id, hash), &buffer, "application/zip"); err != nil {
		return workspaceVersion{}, fmt.Errorf("upload workspace package: %w", err)
	}
	return workspaceVersion{hash: hash, metadata: metadata}, nil
}

func withWorkspaceValue[T any](ctx context.Context, w *WorkspaceStorage, id uuid.UUID, fn func(context.Context) (T, error)) (result T, err error) {
	err = w.withPackage(ctx, id, func(ctx context.Context, _ *workspacePackage) error {
		var callErr error
		result, callErr = fn(ctx)
		return callErr
	})
	return result, err
}

func (w *WorkspaceStorage) ReadFile(ctx context.Context, id uuid.UUID, path string) ([]byte, error) {
	return withWorkspaceValue(ctx, w, id, func(ctx context.Context) ([]byte, error) {
		pkg := ctx.Value(workspacePackageKey{w, id}).(*workspacePackage)
		data, ok := pkg.files[path]
		if !ok {
			return nil, storage.ErrNotFound
		}
		return bytes.Clone(data), nil
	})
}

func (w *WorkspaceStorage) WriteFile(ctx context.Context, id uuid.UUID, path string, content []byte) error {
	if !fs.ValidPath(path) {
		return fmt.Errorf("invalid package path %q", path)
	}
	return w.withPackage(ctx, id, func(_ context.Context, pkg *workspacePackage) error {
		pkg.files[path] = bytes.Clone(content)
		pkg.dirty = true
		return nil
	})
}

func (w *WorkspaceStorage) DeleteFile(ctx context.Context, id uuid.UUID, path string) error {
	return w.withPackage(ctx, id, func(_ context.Context, pkg *workspacePackage) error {
		delete(pkg.files, path)
		pkg.dirty = true
		return nil
	})
}

func (w *WorkspaceStorage) ListAllObjects(ctx context.Context, id uuid.UUID) ([]storage.ObjectInfo, error) {
	return withWorkspaceValue(ctx, w, id, func(ctx context.Context) ([]storage.ObjectInfo, error) {
		pkg := ctx.Value(workspacePackageKey{w, id}).(*workspacePackage)
		objects := make([]storage.ObjectInfo, 0, len(pkg.files))
		for path, data := range pkg.files {
			hidden := false
			for _, part := range strings.Split(path, "/") {
				if strings.HasPrefix(part, ".") {
					hidden = true
					break
				}
			}
			if !hidden {
				objects = append(objects, storage.ObjectInfo{Key: path, Size: int64(len(data))})
			}
		}
		sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
		return objects, nil
	})
}

func (w *WorkspaceStorage) ListAllFiles(ctx context.Context, problemID uuid.UUID) ([]string, error) {
	objects, err := w.ListAllObjects(ctx, problemID)
	if err != nil {
		return nil, err
	}
	files := make([]string, len(objects))
	for i, obj := range objects {
		files[i] = obj.Key
	}
	return files, nil
}

func (w *WorkspaceStorage) ListFiles(ctx context.Context, problemID uuid.UUID, dirPath string) ([]problemsmodel.FileEntry, error) {
	allObjects, err := w.ListAllObjects(ctx, problemID)
	if err != nil {
		return nil, err
	}

	prefix := ""
	if dirPath != "" && dirPath != "." {
		prefix = strings.TrimSuffix(dirPath, "/") + "/"
	}

	seen := make(map[string]bool)
	var entries []problemsmodel.FileEntry

	for _, obj := range allObjects {
		file := obj.Key
		if prefix != "" && !strings.HasPrefix(file, prefix) {
			continue
		}
		rel := strings.TrimPrefix(file, prefix)
		parts := strings.Split(rel, "/")

		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		name := parts[0]
		if seen[name] {
			continue
		}
		seen[name] = true

		isDir := len(parts) > 1
		var entryPath string
		if prefix != "" {
			entryPath = prefix + name
		} else {
			entryPath = name
		}

		size := obj.Size
		if isDir {
			size = 0
		}

		entries = append(entries, problemsmodel.FileEntry{
			Path:        entryPath,
			IsDirectory: isDir,
			Size:        size,
		})
	}

	return entries, nil
}

func (w *WorkspaceStorage) DeleteProblemWorkspace(ctx context.Context, id uuid.UUID) error {
	return w.withPackage(ctx, id, func(_ context.Context, pkg *workspacePackage) error {
		clear(pkg.files)
		pkg.dirty = true
		return nil
	})
}
