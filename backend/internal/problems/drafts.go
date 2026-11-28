package problems

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
)

type DraftRepository interface {
	FinalizeImport(context.Context, uuid.UUID, int64, model.Draft, []model.StoredObject) (model.Draft, error)
	FinalizeCreate(ctx context.Context, org, user uuid.UUID, draft model.Draft, objects []model.StoredObject) error
	GetDraft(context.Context, uuid.UUID) (model.Draft, error)
	SaveDraft(context.Context, model.Draft) (model.Draft, error)
	EnqueueBuild(context.Context, uuid.UUID, int64, model.BuildOptions) (uuid.UUID, error)
	GetBuild(context.Context, uuid.UUID) (model.ProblemBuild, error)
	ListBuilds(context.Context, uuid.UUID) ([]model.ProblemBuild, error)
	CancelBuild(context.Context, uuid.UUID) error
	FinalizeUploads(context.Context, uuid.UUID, int64, model.UploadChange, map[string]model.StoredObject) (model.Draft, error)
	GetObject(context.Context, uuid.UUID) (model.StoredObject, error)
}
type DraftObjectStore interface {
	UploadFileWithSHA256(context.Context, string, string, io.Reader, string, string) error
	DownloadFile(context.Context, string, string, *string) (io.ReadCloser, string, error)
}
type DraftUseCase struct {
	DraftQuotas    model.DraftQuotas
	Organizations  OrganizationsRepo
	Templates      PackagesRepo
	PackageContent *PackageContent
	Repo           DraftRepository
	Problems       interface {
		GetProblemById(context.Context, uuid.UUID) (model.Problem, error)
	}
	Access       access.Loader
	Store        DraftObjectStore
	Bucket       string
	TempDir      string
	UploadTTL    time.Duration
	BuildOptions model.BuildOptions
	Languages    *sandbox.Config
	Typeset      Typesetter
}

type Typesetter interface {
	statement.Converter
	LaTeX(ctx context.Context, from, text string) (string, []sandbox.PandocMessage, error)
	PDF(ctx context.Context, doc string, files map[string][]byte) ([]byte, error)
	Pictures(ctx context.Context, files map[string][]byte) (map[string][]byte, error)
}

// authorize fails fast before uploads; mutations authorize again inside their transaction.
func (uc *DraftUseCase) authorize(ctx context.Context, id uuid.UUID, edit bool) (model.Problem, error) {
	action := accessmodel.ProblemRead
	if edit {
		action = accessmodel.ProblemEdit
	}
	if _, err := access.Problem(ctx, uc.Access, id, action); err != nil {
		return model.Problem{}, err
	}
	return uc.Problems.GetProblemById(ctx, id)
}
func (uc *DraftUseCase) GetDraft(ctx context.Context, id uuid.UUID) (model.Draft, error) {
	if _, err := uc.authorize(ctx, id, false); err != nil {
		return model.Draft{}, err
	}
	return uc.Repo.GetDraft(ctx, id)
}
func (uc *DraftUseCase) SaveDraft(ctx context.Context, id uuid.UUID, expected int64, d model.Draft) (model.Draft, error) {
	if _, err := uc.authorize(ctx, id, true); err != nil {
		return model.Draft{}, err
	}
	if d.ProblemID != id {
		return model.Draft{}, pkg.ErrInvalid
	}
	if expected != d.Revision {
		return model.Draft{}, pkg.ErrStaleRevision
	}
	d.TrimStatements()
	return uc.Repo.SaveDraft(ctx, d)
}
func (uc *DraftUseCase) RequestBuild(ctx context.Context, id uuid.UUID, expected int64) (uuid.UUID, error) {
	if _, err := uc.authorize(ctx, id, true); err != nil {
		return uuid.Nil, err
	}
	return uc.Repo.EnqueueBuild(ctx, id, expected, uc.BuildOptions)
}
func (uc *DraftUseCase) GetBuild(ctx context.Context, id, bid uuid.UUID) (model.ProblemBuild, error) {
	if _, err := uc.authorize(ctx, id, false); err != nil {
		return model.ProblemBuild{}, err
	}
	b, err := uc.Repo.GetBuild(ctx, bid)
	if err != nil {
		return b, err
	}
	if b.ProblemID != id {
		return model.ProblemBuild{}, pkg.ErrNotFound
	}
	return b, nil
}
func (uc *DraftUseCase) ListBuilds(ctx context.Context, id uuid.UUID) ([]model.ProblemBuild, error) {
	if _, err := uc.authorize(ctx, id, false); err != nil {
		return nil, err
	}
	return uc.Repo.ListBuilds(ctx, id)
}
func (uc *DraftUseCase) CancelBuild(ctx context.Context, id, bid uuid.UUID) error {
	if _, err := uc.authorize(ctx, id, true); err != nil {
		return err
	}
	b, err := uc.Repo.GetBuild(ctx, bid)
	if err != nil {
		return err
	}
	if b.ProblemID != id {
		return pkg.ErrNotFound
	}
	return uc.Repo.CancelBuild(ctx, bid)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, fmt.Errorf("upload cancelled: %w", err)
	}
	n, err := r.reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, fmt.Errorf("read upload: %w", err)
	}
	return n, err //nolint:wrapcheck // io.EOF must pass through unchanged.
}

type spooledUpload struct {
	file *os.File
	spec model.UploadSpec
}

func (uc *DraftUseCase) spool(ctx context.Context, dir, purpose string, reader io.Reader) (spooledUpload, error) {
	if reader == nil || int64(uc.BuildOptions.Limits.ObjectBytes) <= 0 {
		return spooledUpload{}, pkg.ErrInvalid
	}
	f, err := os.CreateTemp(dir, "body-")
	if err != nil {
		return spooledUpload{}, fmt.Errorf("createtemp: %w", err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(contextReader{ctx, reader}, int64(uc.BuildOptions.Limits.ObjectBytes)+1))
	if err != nil {
		_ = f.Close()
		return spooledUpload{}, fmt.Errorf("copy: %w", err)
	}
	if n > int64(uc.BuildOptions.Limits.ObjectBytes) {
		_ = f.Close()
		return spooledUpload{}, pkg.ErrPayloadTooLarge
	}
	return spooledUpload{file: f, spec: model.UploadSpec{Purpose: purpose, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}}, nil
}

// Upload stores files, keyed by model.UploadSlots, into the slots of the change's entry.
func (uc *DraftUseCase) Upload(ctx context.Context, id uuid.UUID, expected int64, change model.UploadChange, files map[string]io.Reader) (model.Draft, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.Draft{}, err
	}
	if uc.UploadTTL <= 0 {
		return model.Draft{}, pkg.ErrInternal
	}
	ctx, cancel := context.WithTimeout(ctx, uc.UploadTTL)
	defer cancel()
	dir, err := os.MkdirTemp(uc.TempDir, "upload-")
	if err != nil {
		return model.Draft{}, fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(dir)
	if len(files) == 0 {
		return model.Draft{}, pkg.ErrInvalid
	}
	uploads := map[string]spooledUpload{}
	for _, slot := range model.UploadSlots {
		body, ok := files[slot]
		if !ok {
			continue
		}
		purpose := change.Purpose(slot)
		if purpose == "" {
			return model.Draft{}, pkg.Wrap(pkg.ErrInvalid, nil, "the change has no "+slot+" file")
		}
		u, err := uc.spool(ctx, dir, purpose, body)
		if err != nil {
			return model.Draft{}, err
		}
		defer u.file.Close()
		uploads[slot] = u
	}
	if len(uploads) != len(files) {
		return model.Draft{}, pkg.ErrInvalid
	}
	objects := map[string]model.StoredObject{}
	for slot, u := range uploads {
		objects[slot], err = uc.storeObject(ctx, p.OrganizationID, u.spec, u.file)
		if err != nil {
			return model.Draft{}, err
		}
	}
	return uc.Repo.FinalizeUploads(ctx, id, expected, change, objects)
}

func (uc *DraftUseCase) storeObject(ctx context.Context, org uuid.UUID, spec model.UploadSpec, body io.ReaderAt) (model.StoredObject, error) {
	object := model.NewStoredObject(org, spec)
	if err := uc.Store.UploadFileWithSHA256(ctx, uc.Bucket, object.Key, io.NewSectionReader(body, 0, spec.Size), "application/octet-stream", spec.SHA256); err != nil {
		return model.StoredObject{}, err
	}
	return object, nil
}
func (uc *DraftUseCase) DownloadObject(ctx context.Context, id, oid uuid.UUID) (io.ReadCloser, string, error) {
	d, err := uc.GetDraft(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if !slices.Contains(d.InputObjects(), oid) {
		return nil, "", pkg.ErrNotFound
	}
	o, err := uc.Repo.GetObject(ctx, oid)
	if err != nil {
		return nil, "", err
	}
	r, _, err := uc.Store.DownloadFile(ctx, uc.Bucket, o.Key, nil)
	return r, o.SHA256, err
}
