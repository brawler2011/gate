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
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/storage"
	"github.com/brawler2011/gate/backend/pkg/ziputil"
	"github.com/google/uuid"
)

type PackageObjects interface {
	GetPackageDescriptor(context.Context, uuid.UUID) (model.PackageDescriptor, error)
}
type LoadedPackage struct {
	Dir        string
	Format     *gfmt.GateFormat
	Descriptor model.PackageDescriptor
}

func (l *LoadedPackage) Cleanup() { _ = os.RemoveAll(l.Dir) }

type PackageLoader struct {
	Store    storage.Storage
	Bucket   string
	TempDir  string
	Packages PackageObjects
	Limits   model.BuildLimits
}

func (l *PackageLoader) LoadPackage(ctx context.Context, id uuid.UUID) (result *LoadedPackage, err error) {
	p, err := l.Packages.GetPackageDescriptor(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Object.State != "available" || p.Object.Size < 0 || p.Object.Size >= int64(l.Limits.DiskBytes) {
		return nil, fmt.Errorf("package object is unavailable or exceeds disk quota")
	}
	root, err := os.MkdirTemp(l.TempDir, "package-")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(root)
		}
	}()
	r, _, err := l.Store.DownloadFile(ctx, l.Bucket, p.Object.Key, nil)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	archivePath := filepath.Join(root, "package.zip")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("openfile: %w", err)
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, p.Object.Size+1))
	closeErr := f.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return nil, err
	}
	if n != p.Object.Size || hex.EncodeToString(hash.Sum(nil)) != p.Object.SHA256 {
		return nil, fmt.Errorf("package object checksum or size mismatch")
	}
	z, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("openreader: %w", err)
	}
	defer z.Close()
	tree := filepath.Join(root, "tree")
	if err := ziputil.ExtractZipLimited(&z.Reader, tree, int64(l.Limits.DiskBytes)-p.Object.Size, l.Limits.FileCount); err != nil {
		return nil, err
	}
	format, err := gfmt.OpenBuiltPackage(tree)
	if err != nil {
		return nil, err
	}
	d := p.Snapshot.Draft
	if format.Problem.Title != d.Title || format.Problem.Type != d.Type || format.Problem.Limits.TimeMs != d.TimeLimitMs || format.Problem.Limits.MemoryMb != d.MemoryLimitMb || format.Problem.ToolchainDigest != p.Snapshot.Toolchain.Digest {
		return nil, fmt.Errorf("package metadata differs from build snapshot")
	}
	return &LoadedPackage{Dir: tree, Format: format, Descriptor: p}, nil
}
