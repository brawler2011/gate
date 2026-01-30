package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/pkg"
)

// PublishProblem handles POST /problems/{id}/publish
// Note: This is implemented separately in ProblemPublishHandler (problem_publish.go)
// This stub is required for the StrictServerInterface
func (h *CoreServer) PublishProblem(ctx context.Context, request corev1.PublishProblemRequestObject) (corev1.PublishProblemResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "PublishProblem not implemented in CoreServer (use separate handler)")
}

// GetPublishedPackage handles GET /problems/{id}/package/{version}
// Note: This is implemented separately in ProblemPublishHandler (problem_publish.go)
// This stub is required for the StrictServerInterface
func (h *CoreServer) GetPublishedPackage(ctx context.Context, request corev1.GetPublishedPackageRequestObject) (corev1.GetPublishedPackageResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GetPublishedPackage not implemented in CoreServer (use separate handler)")
}

// ImportProblem handles POST /problems/import
// Note: This is implemented separately in ProblemImportHandler (problem_import.go)
// This stub is required for the StrictServerInterface
func (h *CoreServer) ImportProblem(ctx context.Context, request corev1.ImportProblemRequestObject) (corev1.ImportProblemResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ImportProblem not implemented in CoreServer (use separate handler)")
}
