package problems

import (
	"context"
	"errors"
	"io"

	"github.com/brawler2011/gate/backend/internal/access"
	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackageUseCase struct {
	Repo    PackagesRepo
	Content *PackageContent
	Access  access.Loader
}

func (uc *PackageUseCase) ListPackages(ctx context.Context, id uuid.UUID, request paging.Request) (*paging.List[model.ProblemPackage], error) {
	if _, err := access.Problem(ctx, uc.Access, id, accessmodel.ProblemRead); err != nil {
		return nil, err
	}
	request, err := request.Normalize()
	if err != nil {
		return nil, err
	}
	items, count, err := uc.Repo.ListPackages(ctx, id, request)
	if err != nil {
		return nil, err
	}
	return &paging.List[model.ProblemPackage]{Items: items, Pagination: paging.NewPagination(request.Page, request.PageSize, count)}, nil
}

// Statement reads the latest package's statement, which public visibility exposes; ok is false until a package is built.
func (uc *PackageUseCase) Statement(ctx context.Context, id uuid.UUID) (content model.StatementContent, ok bool, err error) {
	if _, err := access.Problem(ctx, uc.Access, id, accessmodel.ProblemView); err != nil {
		return content, false, err
	}
	ready, err := uc.Repo.GetReadyPackage(ctx, id)
	if errors.Is(err, pkg.ErrNotFound) {
		return content, false, nil
	}
	if err != nil {
		return content, false, err
	}
	content = uc.Content.ReadStatement(ctx, id, ready.ID, "")
	return content, content.Statement != nil, nil
}

func (uc *PackageUseCase) OpenStatementImage(ctx context.Context, id uuid.UUID, sha256 string) (io.ReadCloser, string, error) {
	if _, err := access.Problem(ctx, uc.Access, id, accessmodel.ProblemView); err != nil {
		return nil, "", err
	}
	ready, err := uc.Repo.GetReadyPackage(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return uc.Content.OpenImage(ctx, ready.ID, sha256)
}

func (uc *PackageUseCase) DownloadPackage(ctx context.Context, id, pid uuid.UUID) (io.ReadCloser, error) {
	if _, err := access.Problem(ctx, uc.Access, id, accessmodel.ProblemRead); err != nil {
		return nil, err
	}
	descriptor, err := uc.Content.Repo.GetPackageDescriptor(ctx, pid)
	if err != nil {
		return nil, err
	}
	if descriptor.ProblemID != id {
		return nil, pkg.ErrNotFound
	}
	reader, _, err := uc.Content.Store.DownloadFile(ctx, uc.Content.Bucket, descriptor.Object.Key, nil)
	return reader, err
}
