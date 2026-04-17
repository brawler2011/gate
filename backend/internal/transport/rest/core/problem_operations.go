package core

import (
	"bytes"
	"context"
	"io"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/pkg"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ImportProblem handles POST /problems/{id}/import
func (h *CoreServer) ImportProblem(ctx context.Context, request corev1.ImportProblemRequestObject) (corev1.ImportProblemResponseObject, error) {
	// Parse multipart form
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	// Read the multipart form
	part, err := request.Body.NextPart()
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read form part")
	}
	defer part.Close()

	// Check field name
	if part.FormName() != "package" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "expected 'package' field")
	}

	// Read file into memory
	fileBytes, err := io.ReadAll(part)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to read file")
	}

	// Import problem
	_, err = h.importUC.ImportProblemPackage(ctx, bytes.NewReader(fileBytes), int64(len(fileBytes)), request.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to import problem")
	}

	message := "Problem package imported successfully"
	return corev1.ImportProblem200JSONResponse{
		Message: &message,
	}, nil
}

// PublishProblem handles POST /problems/{id}/publish
func (h *CoreServer) PublishProblem(ctx context.Context, request corev1.PublishProblemRequestObject) (corev1.PublishProblemResponseObject, error) {
	result, err := h.publishUC.PublishProblem(ctx, request.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to publish problem")
	}

	message := "Problem published successfully"
	return corev1.PublishProblem200JSONResponse{
		Version: &result.Version,
		Message: &message,
	}, nil
}

// ListProblemPackages handles GET /problems/{id}/packages
func (h *CoreServer) ListProblemPackages(ctx context.Context, request corev1.ListProblemPackagesRequestObject) (corev1.ListProblemPackagesResponseObject, error) {
	packages, err := h.publishUC.ListPackages(ctx, request.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list packages")
	}

	resp := corev1.ListProblemPackages200JSONResponse{}
	items := make([]struct {
		CompiledAt  *time.Time          `json:"compiled_at,omitempty"`
		CreatedAt   *time.Time          `json:"created_at,omitempty"`
		Id          *openapi_types.UUID `json:"id,omitempty"`
		PackageHash *string             `json:"package_hash,omitempty"`
		Status      *string             `json:"status,omitempty"`
		Version     *int32              `json:"version,omitempty"`
	}, len(packages))
	for i, p := range packages {
		id := openapi_types.UUID(p.ID)
		hash := p.PackageHash
		status := p.Status
		version := p.Version
		createdAt := p.CreatedAt
		items[i].Id = &id
		items[i].PackageHash = &hash
		items[i].Status = &status
		items[i].Version = &version
		items[i].CreatedAt = &createdAt
		if p.CompiledAt != nil {
			items[i].CompiledAt = p.CompiledAt
		}
	}
	resp.Packages = &items
	return resp, nil
}

// GetPublishedPackage handles GET /problems/{id}/package/{version}
func (h *CoreServer) GetPublishedPackage(ctx context.Context, request corev1.GetPublishedPackageRequestObject) (corev1.GetPublishedPackageResponseObject, error) {
	// Get presigned URL
	packageURL, err := h.publishUC.GetPublishedPackageURL(ctx, request.Id, request.Version)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to get package URL")
	}

	// Return 302 redirect to S3 presigned URL
	return corev1.GetPublishedPackage302Response{
		Headers: corev1.GetPublishedPackage302ResponseHeaders{
			Location: packageURL,
		},
	}, nil
}
