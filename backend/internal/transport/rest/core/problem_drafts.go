package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type ProblemDraftUC interface {
	CreateProblem(context.Context, *model.CreateProblemInput) (uuid.UUID, error)
	GetDraft(context.Context, uuid.UUID) (model.Draft, error)
	SaveDraft(context.Context, uuid.UUID, int64, model.Draft) (model.Draft, error)
	RequestBuild(context.Context, uuid.UUID, int64) (uuid.UUID, error)
	GetBuild(context.Context, uuid.UUID, uuid.UUID) (model.ProblemBuild, error)
	ListBuilds(context.Context, uuid.UUID) ([]model.ProblemBuild, error)
	CancelBuild(context.Context, uuid.UUID, uuid.UUID) error
	Upload(context.Context, uuid.UUID, int64, model.UploadChange, map[string]io.Reader) (model.Draft, error)
	Import(context.Context, uuid.UUID, int64, io.Reader) (model.Draft, error)
	DownloadObject(context.Context, uuid.UUID, uuid.UUID) (io.ReadCloser, string, error)
	Export(context.Context, uuid.UUID) (io.ReadCloser, error)
}

func draftContract[T any](source any) (*T, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &value, nil
}

// Reject contract fields absent from the draft model instead of silently dropping them.
func draftModel(source *corev1.ProblemDraft) (*model.Draft, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var d model.Draft
	if err := decoder.Decode(&d); err != nil {
		return nil, fmt.Errorf("decode draft: %w", err)
	}
	return &d, nil
}
func draftResult(d model.Draft, err error) (*corev1.ProblemDraft, error) {
	if err != nil {
		return nil, err
	}
	return draftContract[corev1.ProblemDraft](d)
}
func (h *CoreServer) GetProblemDraft(ctx context.Context, p corev1.GetProblemDraftParams) (*corev1.ProblemDraft, error) {
	d, err := h.ProblemDrafts.GetDraft(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return draftContract[corev1.ProblemDraft](d)
}
func (h *CoreServer) SaveProblemDraft(ctx context.Context, req *corev1.ProblemDraft, p corev1.SaveProblemDraftParams) (*corev1.ProblemDraft, error) {
	if req == nil {
		return nil, pkg.ErrInvalid
	}
	d, err := draftModel(req)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, err, "invalid draft")
	}
	return draftResult(h.ProblemDrafts.SaveDraft(ctx, p.ID, p.ExpectedRevision, *d))
}
func (h *CoreServer) BuildProblemPackage(ctx context.Context, req *corev1.BuildProblemPackageReq, p corev1.BuildProblemPackageParams) (*corev1.ProblemBuildAccepted, error) {
	if req == nil {
		return nil, pkg.ErrInvalid
	}
	id, err := h.ProblemDrafts.RequestBuild(ctx, p.ID, req.ExpectedRevision)
	if err != nil {
		return nil, err
	}
	return &corev1.ProblemBuildAccepted{BuildID: id, Revision: req.ExpectedRevision}, nil
}
func (h *CoreServer) GetProblemBuild(ctx context.Context, p corev1.GetProblemBuildParams) (*corev1.ProblemBuild, error) {
	b, err := h.ProblemDrafts.GetBuild(ctx, p.ID, p.BuildId)
	if err != nil {
		return nil, err
	}
	return draftContract[corev1.ProblemBuild](b)
}
func (h *CoreServer) ListProblemBuilds(ctx context.Context, p corev1.ListProblemBuildsParams) ([]corev1.ProblemBuild, error) {
	builds, err := h.ProblemDrafts.ListBuilds(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	result, err := draftContract[[]corev1.ProblemBuild](builds)
	if err != nil {
		return nil, err
	}
	return *result, nil
}
func (h *CoreServer) CancelProblemBuild(ctx context.Context, p corev1.CancelProblemBuildParams) (*corev1.ProblemBuildCancelled, error) {
	if err := h.ProblemDrafts.CancelBuild(ctx, p.ID, p.BuildId); err != nil {
		return nil, err
	}
	return &corev1.ProblemBuildCancelled{BuildID: p.BuildId, State: corev1.ProblemBuildCancelledStateCancelled}, nil
}
func (h *CoreServer) UploadProblemDraft(ctx context.Context, req *corev1.UploadProblemDraftReq, p corev1.UploadProblemDraftParams) (*corev1.ProblemDraft, error) {
	if req == nil {
		return nil, pkg.ErrInvalid
	}
	var change model.UploadChange
	decoder := json.NewDecoder(strings.NewReader(req.Metadata))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&change); err != nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, err, "invalid upload metadata")
	}
	files := map[string]io.Reader{}
	for slot, file := range map[string]corev1.OptMultipartFile{"asset": req.Asset, "input": req.Input, "output": req.Output, "answer": req.Answer} {
		if file.IsSet() {
			files[slot] = file.Value.File
		}
	}
	return draftResult(h.ProblemDrafts.Upload(ctx, p.ID, p.ExpectedRevision, change, files))
}
func (h *CoreServer) ExportProblemDraft(ctx context.Context, p corev1.ExportProblemDraftParams) (corev1.ExportProblemDraftOK, error) {
	reader, err := h.ProblemDrafts.Export(ctx, p.ID)
	if err != nil {
		return corev1.ExportProblemDraftOK{}, err
	}
	return corev1.ExportProblemDraftOK{Data: reader}, nil
}

func (h *CoreServer) GetProblemDraftObject(ctx context.Context, p corev1.GetProblemDraftObjectParams) (corev1.GetProblemDraftObjectOK, error) {
	reader, _, err := h.ProblemDrafts.DownloadObject(ctx, p.ID, p.ObjectId)
	if err != nil {
		return corev1.GetProblemDraftObjectOK{}, err
	}
	return corev1.GetProblemDraftObjectOK{Data: reader}, nil
}
