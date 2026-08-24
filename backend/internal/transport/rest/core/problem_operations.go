package core

import (
	"bytes"
	"context"
	"io"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg"
)

// ImportProblem handles POST /problems/{id}/import
func (h *CoreServer) ImportProblem(ctx context.Context, req *corev1.ImportProblemReq, params corev1.ImportProblemParams) (*corev1.ImportProblemOK, error) {
	if req == nil || !req.Package.IsSet() {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "expected 'package' field")
	}

	mpFile := req.Package.Value
	if mpFile.File == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing file content")
	}

	// Read file into memory (limited to MaxPackageZipSize + 1)
	limitedReader := io.LimitReader(mpFile.File, MaxPackageZipSize+1)
	fileBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to read file")
	}
	if int64(len(fileBytes)) > MaxPackageZipSize {
		return nil, pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "package zip exceeds maximum allowed size of 500MB")
	}

	// Import problem
	_, err = h.importUC.ImportProblemPackage(ctx, bytes.NewReader(fileBytes), int64(len(fileBytes)), params.ID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to import problem")
	}

	return &corev1.ImportProblemOK{
		Message: corev1.NewOptString("Problem package imported successfully"),
	}, nil
}

// PublishProblem handles POST /problems/{id}/publish
func (h *CoreServer) PublishProblem(ctx context.Context, params corev1.PublishProblemParams) (*corev1.PublishProblemOK, error) {
	result, err := h.publishUC.PublishProblem(ctx, params.ID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to publish problem")
	}

	return &corev1.PublishProblemOK{
		Version: corev1.NewOptInt32(result.Version),
		Message: corev1.NewOptString("Problem published successfully"),
	}, nil
}

// ListProblemPackages handles GET /problems/{id}/packages
func (h *CoreServer) ListProblemPackages(ctx context.Context, params corev1.ListProblemPackagesParams) (*corev1.ListProblemPackagesOK, error) {
	packages, err := h.publishUC.ListPackages(ctx, params.ID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list packages")
	}

	items := make([]corev1.ListProblemPackagesOKPackagesItem, len(packages))
	for i, p := range packages {
		items[i] = corev1.ListProblemPackagesOKPackagesItem{
			ID:          corev1.NewOptUUID(p.ID),
			Version:     corev1.NewOptInt32(p.Version),
			Status:      corev1.NewOptString(p.Status),
			PackageHash: corev1.NewOptString(p.PackageHash),
			CreatedAt:   corev1.NewOptDateTime(p.CreatedAt),
		}
		if p.CompiledAt != nil {
			items[i].CompiledAt = corev1.NewOptDateTime(*p.CompiledAt)
		}
	}
	return &corev1.ListProblemPackagesOK{Packages: items}, nil
}

// GetPublishedPackage handles GET /problems/{id}/package/{version}
func (h *CoreServer) GetPublishedPackage(ctx context.Context, params corev1.GetPublishedPackageParams) (*corev1.GetPublishedPackageFound, error) {
	// Get presigned URL
	packageURL, err := h.publishUC.GetPublishedPackageURL(ctx, params.ID, params.Version)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get package URL")
	}

	return &corev1.GetPublishedPackageFound{
		Location: corev1.NewOptString(packageURL),
	}, nil
}
