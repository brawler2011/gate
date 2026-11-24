package judging

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/brawler2011/gate/backend/pkg/outputs"

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
	Format     *gfmt.Built
	Descriptor model.PackageDescriptor
	handle     *diskcache.Handle
}

func (l *LoadedPackage) Release() { l.handle.Release() }

// Graders is nil for a package without graders.
func (l *LoadedPackage) Graders() (map[string][]byte, error) {
	dir := filepath.Join(l.Format.Path, gfmt.GraderFolder)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("readdir: %w", err)
	}
	graders := map[string][]byte{}
	for _, e := range entries {
		if graders[e.Name()], err = os.ReadFile(filepath.Join(dir, e.Name())); err != nil {
			return nil, fmt.Errorf("read grader: %w", err)
		}
	}
	return graders, nil
}

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
	format, err := gfmt.OpenBuilt(tree)
	if err != nil {
		h.Release()
		return nil, err
	}
	return &LoadedPackage{Format: format, Descriptor: p, handle: h}, nil
}

// LoadOutputs extracts an output-only submission's outputs archive, named by test ordinal, into the cache.
func (l *PackageLoader) LoadOutputs(ctx context.Context, sha string) (*diskcache.Handle, error) {
	return l.Cache.Acquire(ctx, "outputs-"+sha, func(ctx context.Context, dir string) error { //nolint:wrapcheck // fetch errors keep their kind for the judge
		r, _, err := l.Store.DownloadFile(ctx, l.Bucket, outputs.Key(sha), nil)
		if err != nil {
			return err
		}
		defer r.Close()
		// Deflate never grows the archive much past the outputs it packs.
		data, err := io.ReadAll(io.LimitReader(r, 2*outputs.MaxBytes))
		if err != nil {
			return fmt.Errorf("read outputs: %w", err)
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != sha {
			return fmt.Errorf("outputs archive checksum mismatch")
		}
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return fmt.Errorf("open outputs: %w", err)
		}
		return ziputil.ExtractZipLimited(z, dir, outputs.MaxBytes, outputs.MaxFiles) //nolint:wrapcheck // describes the archive
	})
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
	_, err = gfmt.OpenBuilt(tree)
	return err
}
