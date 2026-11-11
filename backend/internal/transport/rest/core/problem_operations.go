package core

import (
	"context"
	"io"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	paging "github.com/brawler2011/gate/backend/internal/page"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackagesReader interface {
	ListPackages(ctx context.Context, problemID uuid.UUID, request paging.Request) (*paging.List[problemsmodel.ProblemPackage], error)
	DownloadPackage(ctx context.Context, problemID, packageID uuid.UUID) (io.ReadCloser, error)
}

// ImportProblem handles POST /problems/{id}/import.
func (h *CoreServer) ImportProblem(ctx context.Context, req *corev1.ImportProblemReq, params corev1.ImportProblemParams) (*corev1.ProblemDraft, error) {
	if req == nil || req.Package.File == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "expected 'package' field")
	}
	return draftResult[corev1.ProblemDraft](h.problems.Import(ctx, params.ID, params.ExpectedRevision, params.IdempotencyKey, req.Package.File))
}

// ListProblemPackages handles GET /problems/{id}/packages.
func (h *CoreServer) ListProblemPackages(ctx context.Context, params corev1.ListProblemPackagesParams) (*corev1.ListProblemPackagesOK, error) {
	packages, err := h.problems.ListPackages(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}
	items := make([]corev1.ProblemPackage, len(packages.Items))
	for i, p := range packages.Items {
		items[i] = corev1.ProblemPackage{ID: p.ID, ProblemID: p.ProblemID, BuildID: p.BuildID, Revision: p.Revision, PackageHash: p.PackageHash, CreatedAt: p.CreatedAt}
	}
	return &corev1.ListProblemPackagesOK{Packages: items, Pagination: PaginationDTO(packages.Pagination)}, nil
}

// DownloadProblemPackage handles GET /problems/{id}/packages/{packageId}/download.
func (h *CoreServer) DownloadProblemPackage(ctx context.Context, params corev1.DownloadProblemPackageParams) (corev1.DownloadProblemPackageOK, error) {
	reader, err := h.problems.DownloadPackage(ctx, params.ID, params.PackageId)
	if err != nil {
		return corev1.DownloadProblemPackageOK{}, err
	}
	return corev1.DownloadProblemPackageOK{Data: reader}, nil
}
