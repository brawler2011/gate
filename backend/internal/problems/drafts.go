package problems

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type DraftRepository interface {
	FinalizeImport(context.Context, model.MutationKey, uuid.UUID, int64, model.Draft, []model.StoredObject) (model.MutationResult, error)
	FinalizeCreate(context.Context, model.MutationKey, uuid.UUID, model.Draft, []model.StoredObject) (model.MutationResult, error)
	GetDraft(context.Context, uuid.UUID) (model.Draft, error)
	SaveDraft(context.Context, model.MutationKey, model.Draft) (model.MutationResult, error)
	EnqueueBuild(context.Context, model.MutationKey, uuid.UUID, int64, model.BuildOptions) (model.MutationResult, error)
	GetBuild(context.Context, uuid.UUID) (model.ProblemBuild, error)
	ListBuilds(context.Context, uuid.UUID) ([]model.ProblemBuild, error)
	CancelBuild(context.Context, model.MutationKey, uuid.UUID) (model.MutationResult, error)
	BeginOperation(context.Context, model.MutationKey) (*model.MutationResult, error)
	FinalizeUploads(context.Context, model.MutationKey, uuid.UUID, int64, model.UploadChange, map[string]model.StoredObject) (model.MutationResult, error)
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
	Permissions  ProblemPermissions
	Store        DraftObjectStore
	Bucket       string
	TempDir      string
	UploadTTL    time.Duration
	BuildOptions model.BuildOptions
	Languages    *sandbox.Config
}

func (uc *DraftUseCase) authorize(ctx context.Context, id uuid.UUID, edit bool) (model.Problem, error) {
	action := accessmodel.ActionViewProblem
	if edit {
		action = accessmodel.ActionEditProblem
	}
	if err := authorizeProblem(ctx, uc.Permissions, id, action); err != nil {
		return model.Problem{}, err
	}
	return uc.Problems.GetProblemById(ctx, id)
}
func (uc *DraftUseCase) key(org, id uuid.UUID, operation, key string, body any) (model.MutationKey, error) {
	if len(key) == 0 || len(key) > 200 {
		return model.MutationKey{}, pkg.Wrap(pkg.ErrInvalid, nil, "Idempotency-Key is required (up to 200 bytes)")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return model.MutationKey{}, fmt.Errorf("marshal: %w", err)
	}
	digest := sha256.Sum256(data)
	return model.MutationKey{OrganizationID: org, Operation: operation + ":" + id.String(), Key: key, Fingerprint: hex.EncodeToString(digest[:]), Token: uuid.New()}, nil
}
func (uc *DraftUseCase) GetDraft(ctx context.Context, id uuid.UUID) (model.Draft, error) {
	if _, err := uc.authorize(ctx, id, false); err != nil {
		return model.Draft{}, err
	}
	return uc.Repo.GetDraft(ctx, id)
}
func (uc *DraftUseCase) SaveDraft(ctx context.Context, id uuid.UUID, expected int64, key string, d model.Draft) (model.MutationResult, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.MutationResult{}, err
	}
	if d.ProblemID != id {
		return model.MutationResult{}, pkg.ErrInvalid
	}
	if expected != d.Revision {
		return model.MutationResult{}, pkg.ErrStaleRevision
	}
	d.TrimStatements()
	k, err := uc.key(p.OrganizationID, id, "save", key, d)
	if err != nil {
		return model.MutationResult{}, err
	}
	return uc.Repo.SaveDraft(ctx, k, d)
}
func (uc *DraftUseCase) RequestBuild(ctx context.Context, id uuid.UUID, expected int64, key string) (model.MutationResult, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.MutationResult{}, err
	}
	k, err := uc.key(p.OrganizationID, id, "build", key, expected)
	if err != nil {
		return model.MutationResult{}, err
	}
	return uc.Repo.EnqueueBuild(ctx, k, id, expected, uc.BuildOptions)
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
func (uc *DraftUseCase) CancelBuild(ctx context.Context, id, bid uuid.UUID, key string) (model.MutationResult, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.MutationResult{}, err
	}
	b, err := uc.Repo.GetBuild(ctx, bid)
	if err != nil {
		return model.MutationResult{}, err
	}
	if b.ProblemID != id {
		return model.MutationResult{}, pkg.ErrNotFound
	}
	k, err := uc.key(p.OrganizationID, id, "cancel", key, bid)
	if err != nil {
		return model.MutationResult{}, err
	}
	return uc.Repo.CancelBuild(ctx, k, bid)
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
func (uc *DraftUseCase) Upload(ctx context.Context, id uuid.UUID, expected int64, key string, change model.UploadChange, files map[string]io.Reader) (model.MutationResult, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.MutationResult{}, err
	}
	if uc.UploadTTL <= 0 {
		return model.MutationResult{}, pkg.ErrInternal
	}
	ctx, cancel := context.WithTimeout(ctx, uc.UploadTTL)
	defer cancel()
	dir, err := os.MkdirTemp(uc.TempDir, "upload-")
	if err != nil {
		return model.MutationResult{}, fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(dir)
	if len(files) == 0 {
		return model.MutationResult{}, pkg.ErrInvalid
	}
	uploads := map[string]spooledUpload{}
	specs := map[string]model.UploadSpec{}
	for _, slot := range model.UploadSlots {
		body, ok := files[slot]
		if !ok {
			continue
		}
		purpose := change.Purpose(slot)
		if purpose == "" {
			return model.MutationResult{}, pkg.Wrap(pkg.ErrInvalid, nil, "the change has no "+slot+" file")
		}
		u, err := uc.spool(ctx, dir, purpose, body)
		if err != nil {
			return model.MutationResult{}, err
		}
		defer u.file.Close()
		uploads[slot], specs[slot] = u, u.spec
	}
	if len(uploads) != len(files) {
		return model.MutationResult{}, pkg.ErrInvalid
	}
	k, err := uc.key(p.OrganizationID, id, "upload", key, struct {
		Revision int64
		Change   model.UploadChange
		Files    map[string]model.UploadSpec
	}{expected, change, specs})
	if err != nil {
		return model.MutationResult{}, err
	}
	replay, err := uc.Repo.BeginOperation(ctx, k)
	if err != nil {
		return model.MutationResult{}, err
	}
	if replay != nil {
		return *replay, nil
	}
	objects := map[string]model.StoredObject{}
	for slot, u := range uploads {
		objects[slot], err = uc.storeObject(ctx, p.OrganizationID, u.spec, u.file)
		if err != nil {
			return model.MutationResult{}, err
		}
	}
	return uc.Repo.FinalizeUploads(ctx, k, id, expected, change, objects)
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
