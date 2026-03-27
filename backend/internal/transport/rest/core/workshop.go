package core

import (
	"bytes"
	"context"
	"io"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
)

// InitProblemWorkshop handles POST /problems/{problemId}/workshop/init
func (h *CoreServer) InitProblemWorkshop(ctx context.Context, request corev1.InitProblemWorkshopRequestObject) (corev1.InitProblemWorkshopResponseObject, error) {
	if h.workshopUC == nil {
		return nil, pkg.Wrap(pkg.NotImplemented, nil, "workshop functionality not available")
	}

	// Get problem to retrieve title
	problem, err := h.problemsUC.GetProblemById(ctx, request.ProblemId)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "problem not found")
	}

	// Get title from problem or use default
	title := "New Problem"
	if problem.Title != "" {
		title = problem.Title
	}

	// Initialize workshop
	if err := h.workshopUC.InitProblemWorkshop(ctx, request.ProblemId, title); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to initialize workshop")
	}

	return corev1.InitProblemWorkshop200JSONResponse{
		Message: strPtr("Workshop initialized successfully"),
	}, nil
}

// ListWorkshopFiles handles GET /problems/{problemId}/workshop/files
func (h *CoreServer) ListWorkshopFiles(ctx context.Context, request corev1.ListWorkshopFilesRequestObject) (corev1.ListWorkshopFilesResponseObject, error) {
	if !h.workshopUC.IsInitialized(ctx, request.ProblemId) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	path := ""
	if request.Params.Path != nil {
		path = *request.Params.Path
	}

	files, err := h.workshopUC.ListProblemFiles(ctx, request.ProblemId, path)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to list files")
	}

	// Convert vcs.FileEntry to contract FileEntry
	contractFiles := make([]corev1.FileEntry, len(files))
	for i, f := range files {
		contractFiles[i] = corev1.FileEntry{
			Path:        strPtr(f.Path),
			IsDirectory: boolPtr(f.IsDirectory),
			Size:        int64Ptr(f.Size),
		}
	}

	return corev1.ListWorkshopFiles200JSONResponse{
		Files: &contractFiles,
	}, nil
}

// GetWorkshopFile handles GET /problems/{problemId}/workshop/files/{path}
func (h *CoreServer) GetWorkshopFile(ctx context.Context, request corev1.GetWorkshopFileRequestObject) (corev1.GetWorkshopFileResponseObject, error) {
	content, err := h.workshopUC.ReadProblemFile(ctx, request.ProblemId, request.Path)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "file not found")
	}

	return corev1.GetWorkshopFile200ApplicationoctetStreamResponse{
		Body: io.NopCloser(bytes.NewReader(content)),
	}, nil
}

// UpdateWorkshopFile handles PUT /problems/{problemId}/workshop/files/{path}
func (h *CoreServer) UpdateWorkshopFile(ctx context.Context, request corev1.UpdateWorkshopFileRequestObject) (corev1.UpdateWorkshopFileResponseObject, error) {
	user := middleware.GetUser(ctx)

	// Read request body
	content, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read request body")
	}

	// Update file
	updateReq := models.UpdateFileRequest{
		ProblemID: request.ProblemId,
		UserID:    user.Id,
		Path:      request.Path,
		Content:   content,
	}

	if err := h.workshopUC.UpdateProblemFile(ctx, updateReq); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update file")
	}

	return corev1.UpdateWorkshopFile200JSONResponse{
		Message: strPtr("File updated successfully"),
	}, nil
}

// DeleteWorkshopFile handles DELETE /problems/{problemId}/workshop/files/{path}
func (h *CoreServer) DeleteWorkshopFile(ctx context.Context, request corev1.DeleteWorkshopFileRequestObject) (corev1.DeleteWorkshopFileResponseObject, error) {
	if err := h.workshopUC.DeleteProblemFile(ctx, request.ProblemId, request.Path); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to delete file")
	}

	return corev1.DeleteWorkshopFile200Response{}, nil
}

// CompileProblemComponent handles POST /problems/{problemId}/workshop/components/{componentType}/compile
func (h *CoreServer) CompileProblemComponent(ctx context.Context, request corev1.CompileProblemComponentRequestObject) (corev1.CompileProblemComponentResponseObject, error) {
	user := middleware.GetUser(ctx)

	compileReq := models.CompileComponentRequest{
		ProblemID:     request.ProblemId,
		ComponentType: string(request.ComponentType),
		UserID:        user.Id,
	}

	result, err := h.workshopUC.CompileProblemComponent(ctx, compileReq)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to compile component")
	}

	return corev1.CompileProblemComponent200JSONResponse{
		CompileError: strPtr(result.CompileError),
		CompileLog:   strPtr(result.CompileLog),
		FileId:       strPtr(result.FileID),
		Sha256:       strPtr(result.SHA256),
		Success:      boolPtr(result.Success),
	}, nil
}

// GenerateTests handles POST /problems/{problemId}/workshop/tests/generate
func (h *CoreServer) GenerateTests(ctx context.Context, request corev1.GenerateTestsRequestObject) (corev1.GenerateTestsResponseObject, error) {
	user := middleware.GetUser(ctx)

	// Convert test numbers
	testNumbers := make([]int, len(request.Body.TestNumbers))
	for i, num := range request.Body.TestNumbers {
		testNumbers[i] = num
	}

	// Convert arguments
	var arguments [][]string
	if request.Body.Arguments != nil {
		arguments = make([][]string, len(*request.Body.Arguments))
		for i, args := range *request.Body.Arguments {
			arguments[i] = args
		}
	}

	generateReq := models.GenerateTestsRequest{
		ProblemID:     request.ProblemId,
		GeneratorName: request.Body.GeneratorName,
		TestNumbers:   testNumbers,
		Arguments:     arguments,
		UserID:        user.Id,
	}

	if err := h.workshopUC.GenerateTests(ctx, generateReq); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to generate tests")
	}

	return corev1.GenerateTests200JSONResponse{
		Message: strPtr("Tests generated successfully"),
	}, nil
}

// ValidateAllTests handles POST /problems/{problemId}/workshop/tests/validate
func (h *CoreServer) ValidateAllTests(ctx context.Context, request corev1.ValidateAllTestsRequestObject) (corev1.ValidateAllTestsResponseObject, error) {
	user := middleware.GetUser(ctx)

	report, err := h.workshopUC.ValidateAllTests(ctx, request.ProblemId, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to validate tests")
	}

	// Convert validation results
	results := make([]corev1.TestValidationResult, len(report.Results))
	for i, r := range report.Results {
		results[i] = corev1.TestValidationResult{
			Error:      strPtr(r.Error),
			Message:    strPtr(r.Message),
			TestNumber: intPtr(r.TestNumber),
			Valid:      boolPtr(r.Valid),
		}
	}

	return corev1.ValidateAllTests200JSONResponse{
		InvalidTests: intPtr(report.InvalidTests),
		Results:      &results,
		TotalTests:   intPtr(report.TotalTests),
		ValidTests:   intPtr(report.ValidTests),
	}, nil
}

// TestSolution handles POST /problems/{problemId}/workshop/solutions/test
func (h *CoreServer) TestSolution(ctx context.Context, request corev1.TestSolutionRequestObject) (corev1.TestSolutionResponseObject, error) {
	user := middleware.GetUser(ctx)

	// Convert test numbers
	var testNumbers []int
	if request.Body.TestNumbers != nil {
		testNumbers = make([]int, len(*request.Body.TestNumbers))
		for i, num := range *request.Body.TestNumbers {
			testNumbers[i] = num
		}
	}

	testReq := models.TestSolutionRequest{
		ProblemID:    request.ProblemId,
		SolutionPath: request.Body.SolutionPath,
		TestNumbers:  testNumbers,
		UserID:       user.Id,
	}

	report, err := h.workshopUC.TestSolution(ctx, testReq)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to test solution")
	}

	// Convert test results
	results := make([]corev1.TestResult, len(report.Results))
	for i, r := range report.Results {
		results[i] = corev1.TestResult{
			Memory:     int64Ptr(r.Memory),
			Message:    strPtr(r.Message),
			TestNumber: intPtr(r.TestNumber),
			Time:       int64Ptr(r.Time),
			Verdict:    strPtr(r.Verdict),
		}
	}

	return corev1.TestSolution200JSONResponse{
		FailedTests: intPtr(report.FailedTests),
		PassedTests: intPtr(report.PassedTests),
		Results:     &results,
		TotalTests:  intPtr(report.TotalTests),
	}, nil
}

// Helper functions to create pointers
func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
