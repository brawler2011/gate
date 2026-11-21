package problems

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/brawler2011/gate/backend/internal/problems/archive"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

// Export writes the draft as a gfmt author package that Import accepts.
func (uc *DraftUseCase) Export(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	d, err := uc.GetDraft(ctx, id)
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(uc.TempDir, "export-")
	if err != nil {
		return nil, fmt.Errorf("createtemp: %w", err)
	}
	// The open descriptor keeps the unlinked file readable until Close.
	_ = os.Remove(f.Name())
	open := func(oid uuid.UUID) (io.ReadCloser, error) {
		o, err := uc.Repo.GetObject(ctx, oid)
		if err != nil {
			return nil, err
		}
		r, _, err := uc.Store.DownloadFile(ctx, uc.Bucket, o.Key, nil)
		return r, err
	}
	if err := writeExport(f, d, uc.Languages, uc.BuildOptions.Limits, open); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// The export is refused when the import would reject it, so every export imports back.
func writeExport(f *os.File, d model.Draft, languages *sandbox.Config, limits model.BuildLimits, open func(uuid.UUID) (io.ReadCloser, error)) error {
	p, err := archive.Layout(d, languages, nil)
	if err != nil {
		return err
	}
	stats, err := p.Write(f, open)
	if err != nil {
		return err
	}
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	if !stats.Fits(size, limits) {
		return pkg.ErrPayloadTooLarge
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	return nil
}
