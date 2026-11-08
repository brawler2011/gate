package core

import (
	"bytes"
	"context"
	"fmt"
	"io"

	paging "github.com/brawler2011/gate/backend/internal/page"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackagesReader interface {
	ListPackages(ctx context.Context, problemID uuid.UUID, request paging.Request) (*paging.List[problemsmodel.ProblemPackage], error)
}

// ImportProblem handles POST /problems/{id}/import
func (h *CoreServer) ImportProblem(ctx context.Context, req *corev1.ImportProblemReq, params corev1.ImportProblemParams) (*corev1.ImportProblemOK, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "expected 'package' field")
	}

	mpFile := req.Package
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
	_, err = h.problems.ImportProblemPackage(ctx, bytes.NewReader(fileBytes), int64(len(fileBytes)), params.ID)
	if err != nil {
		return nil, workshopWriteError(err, "failed to import problem")
	}

	return &corev1.ImportProblemOK{
		Message: corev1.NewOptString("Problem package imported successfully"),
	}, nil
}

// PublishProblem handles POST /problems/{id}/publish
func (h *CoreServer) PublishProblem(ctx context.Context, params corev1.PublishProblemParams) (*corev1.PublishProblemOK, error) {
	result, err := h.problems.PublishProblem(ctx, params.ID)
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
	packages, err := h.problems.ListPackages(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	items := make([]corev1.ListProblemPackagesOKPackagesItem, len(packages.Items))
	for i, p := range packages.Items {
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
	return &corev1.ListProblemPackagesOK{Packages: items, Pagination: PaginationDTO(packages.Pagination)}, nil
}

// GetPublishedPackage handles GET /problems/{id}/package/{version}
func (h *CoreServer) GetPublishedPackage(ctx context.Context, params corev1.GetPublishedPackageParams) (*corev1.GetPublishedPackageFound, error) {
	// Get presigned URL
	packageURL, err := h.problems.GetPublishedPackageURL(ctx, params.ID, params.Version)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get package URL")
	}

	return &corev1.GetPublishedPackageFound{
		Location: corev1.NewOptString(packageURL),
	}, nil
}
