package problems

import (
	"context"
	"io"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	paging "github.com/brawler2011/gate/backend/internal/page"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackageUseCase struct {
	Repo        PackagesRepo
	Content     *PackageContent
	Permissions ProblemPermissions
}

func (uc *PackageUseCase) ListPackages(ctx context.Context, id uuid.UUID, request paging.Request) (*paging.List[model.ProblemPackage], error) {
	if err := authorizeProblem(ctx, uc.Permissions, id, accessmodel.ActionViewProblem); err != nil {
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
func (uc *PackageUseCase) DownloadPackage(ctx context.Context, id, pid uuid.UUID) (io.ReadCloser, error) {
	if err := authorizeProblem(ctx, uc.Permissions, id, accessmodel.ActionViewProblem); err != nil {
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
