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
	Statement(ctx context.Context, problemID uuid.UUID) (problemsmodel.ProblemPackage, problemsmodel.StatementContent, bool, error)
	OpenStatementImage(ctx context.Context, problemID uuid.UUID, sha256 string) (io.ReadCloser, string, error)
	OpenStatementPDF(ctx context.Context, problemID uuid.UUID, language string) (io.ReadCloser, error)
	OpenFiles(ctx context.Context, problemID uuid.UUID) (io.ReadCloser, error)
}

func (h *CoreServer) DownloadProblemFiles(ctx context.Context, params corev1.DownloadProblemFilesParams) (corev1.DownloadProblemFilesOK, error) {
	file, err := h.Packages.OpenFiles(ctx, params.ID)
	if err != nil {
		return corev1.DownloadProblemFilesOK{}, err
	}
	return corev1.DownloadProblemFilesOK{Data: file}, nil
}

func (h *CoreServer) DownloadProblemStatementPdf(ctx context.Context, params corev1.DownloadProblemStatementPdfParams) (corev1.DownloadProblemStatementPdfOK, error) {
	file, err := h.Packages.OpenStatementPDF(ctx, params.ID, params.Lang.Or(""))
	if err != nil {
		return corev1.DownloadProblemStatementPdfOK{}, err
	}
	return corev1.DownloadProblemStatementPdfOK{Data: file}, nil
}

func (h *CoreServer) GetProblemAsset(ctx context.Context, params corev1.GetProblemAssetParams) (*corev1.GetProblemAssetOKHeaders, error) {
	file, contentType, err := h.Packages.OpenStatementImage(ctx, params.ID, params.SHA256)
	if err != nil {
		return nil, err
	}
	return &corev1.GetProblemAssetOKHeaders{
		CacheControl:          corev1.NewOptString("private, max-age=31536000, immutable"),
		ETag:                  corev1.NewOptString(`"` + params.SHA256 + `"`),
		ContentSecurityPolicy: corev1.NewOptString(statementAssetPolicy),
		ContentType:           contentType,
		Response:              corev1.GetProblemAssetOK{Data: file},
	}, nil
}

func (h *CoreServer) ImportProblem(ctx context.Context, req *corev1.ImportProblemReq, params corev1.ImportProblemParams) (*corev1.ProblemDraft, error) {
	if req == nil || req.Package.File == nil {
		return nil, pkg.Wrap(pkg.ErrInvalid, nil, "expected 'package' field")
	}
	return draftResult(h.ProblemDrafts.Import(ctx, params.ID, params.ExpectedRevision, req.Package.File))
}

func (h *CoreServer) ListProblemPackages(ctx context.Context, params corev1.ListProblemPackagesParams) (*corev1.ListProblemPackagesOK, error) {
	packages, err := h.Packages.ListPackages(ctx, params.ID, paging.Request{Page: params.Page.Or(1), PageSize: params.PageSize.Or(50)})
	if err != nil {
		return nil, err
	}
	items := make([]corev1.ProblemPackage, len(packages.Items))
	for i, p := range packages.Items {
		items[i] = corev1.ProblemPackage{ID: p.ID, ProblemID: p.ProblemID, BuildID: p.BuildID, Revision: p.Revision, PackageHash: p.PackageHash, CreatedAt: p.CreatedAt}
	}
	return &corev1.ListProblemPackagesOK{Packages: items, Pagination: PaginationDTO(packages.Pagination)}, nil
}

func (h *CoreServer) DownloadProblemPackage(ctx context.Context, params corev1.DownloadProblemPackageParams) (corev1.DownloadProblemPackageOK, error) {
	reader, err := h.Packages.DownloadPackage(ctx, params.ID, params.PackageId)
	if err != nil {
		return corev1.DownloadProblemPackageOK{}, err
	}
	return corev1.DownloadProblemPackageOK{Data: reader}, nil
}
