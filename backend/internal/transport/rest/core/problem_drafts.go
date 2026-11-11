package core

import (
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
	GetDraft(context.Context, uuid.UUID) (model.Draft, error)
	SaveDraft(context.Context, uuid.UUID, int64, string, model.Draft) (model.MutationResult, error)
	RequestBuild(context.Context, uuid.UUID, int64, string) (model.MutationResult, error)
	GetBuild(context.Context, uuid.UUID, uuid.UUID) (model.ProblemBuild, error)
	ListBuilds(context.Context, uuid.UUID) ([]model.ProblemBuild, error)
	CancelBuild(context.Context, uuid.UUID, uuid.UUID, string) (model.MutationResult, error)
	Upload(context.Context, uuid.UUID, int64, string, model.UploadChange, io.Reader, io.Reader) (model.MutationResult, error)
	Import(context.Context, uuid.UUID, int64, string, io.Reader) (model.MutationResult, error)
	DownloadObject(context.Context, uuid.UUID, uuid.UUID) (io.ReadCloser, string, error)
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
func draftResult[T any](result model.MutationResult, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	var value T
	if err := json.Unmarshal(result.Body, &value); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &value, nil
}
func (h *CoreServer) GetProblemDraft(ctx context.Context, p corev1.GetProblemDraftParams) (*corev1.ProblemDraft, error) {
	d, err := h.problems.GetDraft(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return draftContract[corev1.ProblemDraft](d)
}
func (h *CoreServer) SaveProblemDraft(ctx context.Context, req *corev1.ProblemDraft, p corev1.SaveProblemDraftParams) (*corev1.ProblemDraft, error) {
	if req == nil {
		return nil, pkg.ErrBadInput
	}
	d, err := draftContract[model.Draft](req)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid draft")
	}
	return draftResult[corev1.ProblemDraft](h.problems.SaveDraft(ctx, p.ID, p.ExpectedRevision, p.IdempotencyKey, *d))
}
func (h *CoreServer) BuildProblemPackage(ctx context.Context, req *corev1.BuildProblemPackageReq, p corev1.BuildProblemPackageParams) (*corev1.ProblemBuildAccepted, error) {
	if req == nil {
		return nil, pkg.ErrBadInput
	}
	return draftResult[corev1.ProblemBuildAccepted](h.problems.RequestBuild(ctx, p.ID, req.ExpectedRevision, p.IdempotencyKey))
}
func (h *CoreServer) GetProblemBuild(ctx context.Context, p corev1.GetProblemBuildParams) (*corev1.ProblemBuild, error) {
	b, err := h.problems.GetBuild(ctx, p.ID, p.BuildId)
	if err != nil {
		return nil, err
	}
	return draftContract[corev1.ProblemBuild](b)
}
func (h *CoreServer) ListProblemBuilds(ctx context.Context, p corev1.ListProblemBuildsParams) ([]corev1.ProblemBuild, error) {
	builds, err := h.problems.ListBuilds(ctx, p.ID)
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
	return draftResult[corev1.ProblemBuildCancelled](h.problems.CancelBuild(ctx, p.ID, p.BuildId, p.IdempotencyKey))
}
func (h *CoreServer) UploadProblemDraft(ctx context.Context, req *corev1.UploadProblemDraftReq, p corev1.UploadProblemDraftParams) (*corev1.ProblemDraft, error) {
	if req == nil || req.File.File == nil {
		return nil, pkg.ErrBadInput
	}
	var change model.UploadChange
	decoder := json.NewDecoder(strings.NewReader(req.Metadata))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&change); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid upload metadata")
	}
	var answer io.Reader
	if req.Answer.IsSet() {
		answer = req.Answer.Value.File
	}
	return draftResult[corev1.ProblemDraft](h.problems.Upload(ctx, p.ID, p.ExpectedRevision, p.IdempotencyKey, change, req.File.File, answer))
}
func (h *CoreServer) GetProblemDraftObject(ctx context.Context, p corev1.GetProblemDraftObjectParams) (corev1.GetProblemDraftObjectOK, error) {
	reader, _, err := h.problems.DownloadObject(ctx, p.ID, p.ObjectId)
	if err != nil {
		return corev1.GetProblemDraftObjectOK{}, err
	}
	return corev1.GetProblemDraftObjectOK{Data: reader}, nil
}
