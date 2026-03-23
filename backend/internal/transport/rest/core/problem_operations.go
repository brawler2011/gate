package core

import (
	"bytes"
	"context"
	"io"
	"time"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/google/uuid"
)

// ImportProblem handles POST /problems/import
func (h *CoreServer) ImportProblem(ctx context.Context, request corev1.ImportProblemRequestObject) (corev1.ImportProblemResponseObject, error) {
	user := middleware.GetUser(ctx)

	// Check if user is authenticated
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "authentication required")
	}

	// Only admins can import problems (for now)
	// TODO: Check organization-specific permissions
	if !user.IsAdmin() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "only admins can import problems")
	}

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

	// Generate problem ID
	problemID := uuid.New()

	// Import problem
	_, err = h.importUC.ImportProblemPackage(ctx, bytes.NewReader(fileBytes), int64(len(fileBytes)), problemID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to import problem")
	}

	// Return problem ID
	return corev1.ImportProblem200JSONResponse{
		Id: problemID,
	}, nil
}

// PublishProblem handles POST /problems/{id}/publish
func (h *CoreServer) PublishProblem(ctx context.Context, request corev1.PublishProblemRequestObject) (corev1.PublishProblemResponseObject, error) {
	user := middleware.GetUser(ctx)

	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "authentication required")
	}

	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, request.Id, user.Id, models.ActionEditProblem)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to publish problem")
	}

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
	user := middleware.GetUser(ctx)
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "authentication required")
	}

	canView, err := h.permissionsUC.HasProblemPermission(ctx, request.Id, user.Id, models.ActionViewProblem)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions")
	}

	packages, err := h.publishUC.ListPackages(ctx, request.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list packages")
	}

	resp := corev1.ListProblemPackages200JSONResponse{}
	items := make([]struct {
		CompiledAt    *time.Time          `json:"compiled_at,omitempty"`
		CreatedAt     *time.Time          `json:"created_at,omitempty"`
		GitCommitHash *string             `json:"git_commit_hash,omitempty"`
		Id            *openapi_types.UUID `json:"id,omitempty"`
		Status        *string             `json:"status,omitempty"`
		Version       *int32              `json:"version,omitempty"`
	}, len(packages))
	for i, p := range packages {
		id := openapi_types.UUID(p.ID)
		hash := p.GitCommitHash
		status := p.Status
		version := p.Version
		createdAt := p.CreatedAt
		items[i].Id = &id
		items[i].GitCommitHash = &hash
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
