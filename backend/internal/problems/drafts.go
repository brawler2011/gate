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
	"github.com/google/uuid"
)

type DraftRepository interface {
	FinalizeImport(context.Context, model.MutationKey, uuid.UUID, int64, model.Draft) (model.MutationResult, error)
	FinalizeCreate(context.Context, model.MutationKey, uuid.UUID, model.Draft) (model.MutationResult, error)
	GetDraft(context.Context, uuid.UUID) (model.Draft, error)
	SaveDraft(context.Context, model.MutationKey, model.Draft) (model.MutationResult, error)
	EnqueueBuild(context.Context, model.MutationKey, uuid.UUID, int64, model.BuildOptions) (model.MutationResult, error)
	GetBuild(context.Context, uuid.UUID) (model.ProblemBuild, error)
	ListBuilds(context.Context, uuid.UUID) ([]model.ProblemBuild, error)
	CancelBuild(context.Context, model.MutationKey, uuid.UUID) (model.MutationResult, error)
	ReserveUploads(context.Context, model.MutationKey, []model.UploadSpec, time.Duration) (model.UploadReservation, error)
	FinalizeUploads(context.Context, model.MutationKey, uuid.UUID, int64, model.UploadChange) (model.MutationResult, error)
	GetObject(context.Context, uuid.UUID) (model.StoredObject, error)
}
type DraftObjectStore interface {
	UploadFile(context.Context, string, string, io.Reader, string) error
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
	ObjectBytes  int64
	BuildOptions model.BuildOptions
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
		return model.MutationKey{}, pkg.Wrap(pkg.ErrBadInput, nil, "Idempotency-Key is required (up to 200 bytes)")
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
	if d.ProblemID != id || expected != d.Revision {
		return model.MutationResult{}, pkg.ErrBadInput
	}
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
	if reader == nil || uc.ObjectBytes <= 0 {
		return spooledUpload{}, pkg.ErrBadInput
	}
	f, err := os.CreateTemp(dir, "body-")
	if err != nil {
		return spooledUpload{}, fmt.Errorf("createtemp: %w", err)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(contextReader{ctx, reader}, uc.ObjectBytes+1))
	if err != nil {
		_ = f.Close()
		return spooledUpload{}, fmt.Errorf("copy: %w", err)
	}
	if n > uc.ObjectBytes {
		_ = f.Close()
		return spooledUpload{}, pkg.ErrPayloadTooLarge
	}
	return spooledUpload{file: f, spec: model.UploadSpec{Purpose: purpose, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}}, nil
}
func (uc *DraftUseCase) Upload(ctx context.Context, id uuid.UUID, expected int64, key string, change model.UploadChange, body, answer io.Reader) (model.MutationResult, error) {
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
	purpose := "test-input"
	if change.Part == "asset" {
		purpose = "asset"
	}
	if change.Part == "answer" {
		purpose = "test-answer"
	}
	first, err := uc.spool(ctx, dir, purpose, body)
	if err != nil {
		return model.MutationResult{}, err
	}
	defer first.file.Close()
	uploads := []spooledUpload{first}
	if change.Part == "pair" {
		second, err := uc.spool(ctx, dir, "test-answer", answer)
		if err != nil {
			return model.MutationResult{}, err
		}
		defer second.file.Close()
		uploads = append(uploads, second)
	} else if answer != nil {
		return model.MutationResult{}, pkg.ErrBadInput
	}
	specs := make([]model.UploadSpec, len(uploads))
	for i, u := range uploads {
		specs[i] = u.spec
	}
	k, err := uc.key(p.OrganizationID, id, "upload", key, struct {
		Revision int64
		Change   model.UploadChange
		Files    []model.UploadSpec
	}{expected, change, specs})
	if err != nil {
		return model.MutationResult{}, err
	}
	reservation, err := uc.Repo.ReserveUploads(ctx, k, specs, uc.UploadTTL)
	if err != nil {
		return model.MutationResult{}, err
	}
	if reservation.Replay != nil {
		return *reservation.Replay, nil
	}
	for i, object := range reservation.Objects {
		if err := uc.Store.UploadFile(ctx, uc.Bucket, object.Key, io.NewSectionReader(uploads[i].file, 0, object.Size), "application/octet-stream"); err != nil {
			return model.MutationResult{}, err
		}
	}
	return uc.Repo.FinalizeUploads(ctx, k, id, expected, change)
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
	if o.State != "available" {
		return nil, "", pkg.ErrConflict
	}
	r, _, err := uc.Store.DownloadFile(ctx, uc.Bucket, o.Key, nil)
	return r, o.SHA256, err
}
