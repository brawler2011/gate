package core

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
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

	// Check if user is authenticated
	if user.IsGuest() {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "authentication required")
	}

	// Check permissions
	canEdit, err := h.permissionsUC.HasProblemPermission(ctx, request.Id, user.Id, models.ActionEditProblem)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "insufficient permissions to publish problem")
	}

	// Note: Current OpenAPI spec doesn't accept version in request body
	// Using a default version for now
	// TODO: Update OpenAPI spec to accept version and commit_sha in request body
	version := int32(1) // Simplified for now
	versionStr := fmt.Sprintf("v%d", version)

	// Publish problem
	err = h.publishUC.PublishProblem(ctx, request.Id, versionStr, "")
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to publish problem")
	}

	message := "Problem published successfully"
	return corev1.PublishProblem200JSONResponse{
		Version: &version,
		Message: &message,
	}, nil
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
