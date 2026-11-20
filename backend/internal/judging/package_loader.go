package judging

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/diskcache"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/pkg/ziputil"
	"github.com/google/uuid"
)

type PackageObjects interface {
	GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error)
}
// LoadedPackage is shared with concurrent jobs: read it, never write to it.
type LoadedPackage struct {
	Format     *gfmt.GateFormat
	Descriptor model.PackageDescriptor
	handle     *diskcache.Handle
}

func (l *LoadedPackage) Release() { l.handle.Release() }

// PackageLoader keeps packages extracted after first use; they are immutable, so entries are only evicted.
type PackageLoader struct {
	Store    storage.Storage
	Bucket   string
	Cache    *diskcache.Cache
	Packages PackageObjects
	Limits   model.BuildLimits
}

func (l *PackageLoader) LoadPackage(ctx context.Context, id uuid.UUID) (*LoadedPackage, error) {
	p, err := l.Packages.GetPackageDescriptor(ctx, id)
	if err != nil {
		return nil, err
	}
	h, err := l.Cache.Acquire(ctx, "package-"+id.String(), func(ctx context.Context, dir string) error {
		return l.fetch(ctx, p, dir)
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // fetch errors keep their kind for the judge
	}
	tree := filepath.Join(h.Dir(), "tree")
	format, err := gfmt.OpenBuiltPackage(tree)
	if err != nil {
		h.Release()
		return nil, err
	}
	return &LoadedPackage{Format: format, Descriptor: p, handle: h}, nil
}

func (l *PackageLoader) fetch(ctx context.Context, p model.PackageDescriptor, root string) error {
	if p.Object.Size < 0 || p.Object.Size >= int64(l.Limits.DiskBytes) {
		return fmt.Errorf("package object exceeds disk quota")
	}
	r, _, err := l.Store.DownloadFile(ctx, l.Bucket, p.Object.Key, nil)
	if err != nil {
		return err
	}
	defer r.Close()
	archivePath := filepath.Join(root, "package.zip")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("openfile: %w", err)
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, p.Object.Size+1))
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	if n != p.Object.Size || hex.EncodeToString(hash.Sum(nil)) != p.Object.SHA256 {
		return fmt.Errorf("package object checksum or size mismatch")
	}
	z, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("openreader: %w", err)
	}
	tree := filepath.Join(root, "tree")
	err = ziputil.ExtractZipLimited(&z.Reader, tree, int64(l.Limits.DiskBytes)-p.Object.Size, l.Limits.FileCount)
	if err := errors.Join(err, z.Close(), os.Remove(archivePath)); err != nil {
		return err
	}
	format, err := gfmt.OpenBuiltPackage(tree)
	if err != nil {
		return err
	}
	d := p.Snapshot.Draft
	if format.Problem.Title != d.Title || format.Problem.Type != d.Type || format.Problem.Limits.TimeMs != d.TimeLimitMs || format.Problem.Limits.MemoryMb != d.MemoryLimitMb ||
		format.Problem.InputFile != d.InputFile || format.Problem.OutputFile != d.OutputFile {
		return fmt.Errorf("package metadata differs from build snapshot")
	}
	return nil
}
