package core

import (
	"context"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/pkg"
)

// CommitWorkshopChanges handles POST /problems/{problemId}/workshop/commit
func (h *CoreServer) CommitWorkshopChanges(ctx context.Context, request corev1.CommitWorkshopChangesRequestObject) (corev1.CommitWorkshopChangesResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "CommitWorkshopChanges not implemented yet")
}

// CompileProblemComponent handles POST /problems/{problemId}/workshop/components/{componentType}/compile
func (h *CoreServer) CompileProblemComponent(ctx context.Context, request corev1.CompileProblemComponentRequestObject) (corev1.CompileProblemComponentResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "CompileProblemComponent not implemented yet")
}

// ListWorkshopFiles handles GET /problems/{problemId}/workshop/files
func (h *CoreServer) ListWorkshopFiles(ctx context.Context, request corev1.ListWorkshopFilesRequestObject) (corev1.ListWorkshopFilesResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ListWorkshopFiles not implemented yet")
}

// DeleteWorkshopFile handles DELETE /problems/{problemId}/workshop/files/{path}
func (h *CoreServer) DeleteWorkshopFile(ctx context.Context, request corev1.DeleteWorkshopFileRequestObject) (corev1.DeleteWorkshopFileResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "DeleteWorkshopFile not implemented yet")
}

// GetWorkshopFile handles GET /problems/{problemId}/workshop/files/{path}
func (h *CoreServer) GetWorkshopFile(ctx context.Context, request corev1.GetWorkshopFileRequestObject) (corev1.GetWorkshopFileResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GetWorkshopFile not implemented yet")
}

// UpdateWorkshopFile handles PUT /problems/{problemId}/workshop/files/{path}
func (h *CoreServer) UpdateWorkshopFile(ctx context.Context, request corev1.UpdateWorkshopFileRequestObject) (corev1.UpdateWorkshopFileResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "UpdateWorkshopFile not implemented yet")
}

// GetWorkshopHistory handles GET /problems/{problemId}/workshop/history
func (h *CoreServer) GetWorkshopHistory(ctx context.Context, request corev1.GetWorkshopHistoryRequestObject) (corev1.GetWorkshopHistoryResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GetWorkshopHistory not implemented yet")
}

// InitProblemWorkshop handles POST /problems/{problemId}/workshop/init
func (h *CoreServer) InitProblemWorkshop(ctx context.Context, request corev1.InitProblemWorkshopRequestObject) (corev1.InitProblemWorkshopResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "InitProblemWorkshop not implemented yet")
}

// TestSolution handles POST /problems/{problemId}/workshop/solutions/test
func (h *CoreServer) TestSolution(ctx context.Context, request corev1.TestSolutionRequestObject) (corev1.TestSolutionResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "TestSolution not implemented yet")
}

// GetWorkshopStatus handles GET /problems/{problemId}/workshop/status
func (h *CoreServer) GetWorkshopStatus(ctx context.Context, request corev1.GetWorkshopStatusRequestObject) (corev1.GetWorkshopStatusResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GetWorkshopStatus not implemented yet")
}

// GenerateTests handles POST /problems/{problemId}/workshop/tests/generate
func (h *CoreServer) GenerateTests(ctx context.Context, request corev1.GenerateTestsRequestObject) (corev1.GenerateTestsResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "GenerateTests not implemented yet")
}

// ValidateAllTests handles POST /problems/{problemId}/workshop/tests/validate
func (h *CoreServer) ValidateAllTests(ctx context.Context, request corev1.ValidateAllTestsRequestObject) (corev1.ValidateAllTestsResponseObject, error) {
	return nil, pkg.Wrap(pkg.NotImplemented, nil, "ValidateAllTests not implemented yet")
}
