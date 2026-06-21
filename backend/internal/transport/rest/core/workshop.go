package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/gate/backend/internal/domain/models"
	"github.com/gate149/gate/backend/internal/transport/middleware"
	"github.com/gate149/gate/backend/pkg"
	"github.com/google/uuid"
)

const (
	checkerDir    = "checkers"
	generatorDir  = "generators"
	interactorDir = "interactors"
	validatorDir  = "validators"
	mediaDir      = "media"
	solutionDir   = "solutions"
	testDir       = "tests"
)


// GetProblemReadme handles GET /problems/{problemId}/readme
func (h *CoreServer) GetProblemReadme(ctx context.Context, request corev1.GetProblemReadmeRequestObject) (corev1.GetProblemReadmeResponseObject, error) {
	if !h.workshopUC.IsInitialized(ctx, request.ProblemId) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	content, err := h.workshopUC.ReadProblemFile(ctx, request.ProblemId, "README.md")
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "README.md not found")
	}

	return corev1.GetProblemReadme200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemReadme handles PUT /problems/{problemId}/readme
func (h *CoreServer) UpdateProblemReadme(ctx context.Context, request corev1.UpdateProblemReadmeRequestObject) (corev1.UpdateProblemReadmeResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if !h.workshopUC.IsInitialized(ctx, request.ProblemId) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	content, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read request body")
	}

	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: request.ProblemId,
		UserID:    middleware.GetUser(ctx).Id,
		Path:      "README.md",
		Content:   content,
	}); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update README.md")
	}

	return corev1.UpdateProblemReadme200JSONResponse{Message: strPtr("README.md updated successfully")}, nil
}

// GetProblemLimits handles GET /problems/{problemId}/limits
func (h *CoreServer) GetProblemLimits(ctx context.Context, request corev1.GetProblemLimitsRequestObject) (corev1.GetProblemLimitsResponseObject, error) {
	manifest, err := h.readWorkshopManifest(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	return corev1.GetProblemLimits200JSONResponse(h.toContractLimits(manifest)), nil
}

// UpdateProblemLimits handles PATCH /problems/{problemId}/limits
func (h *CoreServer) UpdateProblemLimits(ctx context.Context, request corev1.UpdateProblemLimitsRequestObject) (corev1.UpdateProblemLimitsResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	manifest, err := h.readWorkshopManifest(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body.ProblemType != nil {
		manifest.ProblemType = *body.ProblemType
	}
	if body.TimeLimitMs != nil {
		manifest.TimeLimitMs = *body.TimeLimitMs
	}
	if body.MemoryLimitMb != nil {
		manifest.MemoryLimitMb = *body.MemoryLimitMb
	}
	if body.StdoutLimitMb != nil {
		manifest.StdoutLimitMb = *body.StdoutLimitMb
	}
	if body.CodeSizeLimitKb != nil {
		manifest.CodeSizeLimitKb = *body.CodeSizeLimitKb
	}
	if body.MaxScore != nil {
		score := *body.MaxScore
		manifest.MaxScore = &score
	}
	if manifest.ProblemType != "scoring" {
		manifest.MaxScore = nil
	}

	if err := validateManifest(manifest); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid limits update")
	}

	if err := h.saveWorkshopManifest(ctx, request.ProblemId, middleware.GetUser(ctx).Id, manifest); err != nil {
		return nil, err
	}

	return corev1.UpdateProblemLimits200JSONResponse(h.toContractLimits(manifest)), nil
}

// GetProblemStatement handles GET /problems/{problemId}/statement
func (h *CoreServer) GetProblemStatement(ctx context.Context, request corev1.GetProblemStatementRequestObject) (corev1.GetProblemStatementResponseObject, error) {
	manifest, err := h.readWorkshopManifest(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	return corev1.GetProblemStatement200JSONResponse(h.toContractStatement(manifest)), nil
}

// UpdateProblemStatement handles PATCH /problems/{problemId}/statement
func (h *CoreServer) UpdateProblemStatement(ctx context.Context, request corev1.UpdateProblemStatementRequestObject) (corev1.UpdateProblemStatementResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	manifest, err := h.readWorkshopManifest(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body.Title != nil {
		manifest.Statement.Title = *body.Title
	}
	if body.Legend != nil {
		manifest.Statement.Legend = *body.Legend
	}
	if body.InputFormat != nil {
		manifest.Statement.InputFormat = *body.InputFormat
	}
	if body.OutputFormat != nil {
		manifest.Statement.OutputFormat = *body.OutputFormat
	}
	if body.Notes != nil {
		manifest.Statement.Notes = *body.Notes
	}
	if body.Interaction != nil {
		manifest.Statement.Interaction = *body.Interaction
	}
	if body.Scoring != nil {
		manifest.Statement.Scoring = *body.Scoring
	}

	if err := validateManifest(manifest); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid statement update")
	}

	if err := h.saveWorkshopManifest(ctx, request.ProblemId, middleware.GetUser(ctx).Id, manifest); err != nil {
		return nil, err
	}
	if err := h.syncProblemTitleIfNeeded(ctx, request.ProblemId, manifest.Statement.Title); err != nil {
		return nil, err
	}

	return corev1.UpdateProblemStatement200JSONResponse(h.toContractStatement(manifest)), nil
}

// ListProblemCheckers handles GET /problems/{problemId}/checkers
func (h *CoreServer) ListProblemCheckers(ctx context.Context, request corev1.ListProblemCheckersRequestObject) (corev1.ListProblemCheckersResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, checkerDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemCheckers200JSONResponse(resp), nil
}

// CreateProblemChecker handles POST /problems/{problemId}/checkers
func (h *CoreServer) CreateProblemChecker(ctx context.Context, request corev1.CreateProblemCheckerRequestObject) (corev1.CreateProblemCheckerResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, checkerDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemChecker200JSONResponse{Message: strPtr("Checker created successfully")}, nil
}

// GetProblemChecker handles GET /problems/{problemId}/checkers/{name}
func (h *CoreServer) GetProblemChecker(ctx context.Context, request corev1.GetProblemCheckerRequestObject) (corev1.GetProblemCheckerResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, checkerDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemChecker200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemChecker handles PUT /problems/{problemId}/checkers/{name}
func (h *CoreServer) UpdateProblemChecker(ctx context.Context, request corev1.UpdateProblemCheckerRequestObject) (corev1.UpdateProblemCheckerResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, checkerDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemChecker200JSONResponse{Message: strPtr("Checker updated successfully")}, nil
}

// DeleteProblemChecker handles DELETE /problems/{problemId}/checkers/{name}
func (h *CoreServer) DeleteProblemChecker(ctx context.Context, request corev1.DeleteProblemCheckerRequestObject) (corev1.DeleteProblemCheckerResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, checkerDir, request.Name, "checker"); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemChecker200JSONResponse{Message: strPtr("Checker deleted successfully")}, nil
}

// SetProblemCheckerMain handles PATCH /problems/{problemId}/checkers/main
func (h *CoreServer) SetProblemCheckerMain(ctx context.Context, request corev1.SetProblemCheckerMainRequestObject) (corev1.SetProblemCheckerMainResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if err := h.setMainComponent(ctx, request.ProblemId, checkerDir, "checker", request.Body.Name); err != nil {
		return nil, err
	}
	return corev1.SetProblemCheckerMain200JSONResponse{Message: strPtr("Main checker selected successfully")}, nil
}

// ListProblemGenerators handles GET /problems/{problemId}/generators
func (h *CoreServer) ListProblemGenerators(ctx context.Context, request corev1.ListProblemGeneratorsRequestObject) (corev1.ListProblemGeneratorsResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, generatorDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemGenerators200JSONResponse(resp), nil
}

// CreateProblemGenerator handles POST /problems/{problemId}/generators
func (h *CoreServer) CreateProblemGenerator(ctx context.Context, request corev1.CreateProblemGeneratorRequestObject) (corev1.CreateProblemGeneratorResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, generatorDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemGenerator200JSONResponse{Message: strPtr("Generator created successfully")}, nil
}

// GetProblemGenerator handles GET /problems/{problemId}/generators/{name}
func (h *CoreServer) GetProblemGenerator(ctx context.Context, request corev1.GetProblemGeneratorRequestObject) (corev1.GetProblemGeneratorResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, generatorDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemGenerator200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemGenerator handles PUT /problems/{problemId}/generators/{name}
func (h *CoreServer) UpdateProblemGenerator(ctx context.Context, request corev1.UpdateProblemGeneratorRequestObject) (corev1.UpdateProblemGeneratorResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, generatorDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemGenerator200JSONResponse{Message: strPtr("Generator updated successfully")}, nil
}

// DeleteProblemGenerator handles DELETE /problems/{problemId}/generators/{name}
func (h *CoreServer) DeleteProblemGenerator(ctx context.Context, request corev1.DeleteProblemGeneratorRequestObject) (corev1.DeleteProblemGeneratorResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, generatorDir, request.Name, "generator"); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemGenerator200JSONResponse{Message: strPtr("Generator deleted successfully")}, nil
}

// SetProblemGeneratorMain handles PATCH /problems/{problemId}/generators/main
func (h *CoreServer) SetProblemGeneratorMain(ctx context.Context, request corev1.SetProblemGeneratorMainRequestObject) (corev1.SetProblemGeneratorMainResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if err := h.setMainComponent(ctx, request.ProblemId, generatorDir, "generator", request.Body.Name); err != nil {
		return nil, err
	}
	return corev1.SetProblemGeneratorMain200JSONResponse{Message: strPtr("Main generator selected successfully")}, nil
}

// ListProblemInteractors handles GET /problems/{problemId}/interactors
func (h *CoreServer) ListProblemInteractors(ctx context.Context, request corev1.ListProblemInteractorsRequestObject) (corev1.ListProblemInteractorsResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, interactorDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemInteractors200JSONResponse(resp), nil
}

// CreateProblemInteractor handles POST /problems/{problemId}/interactors
func (h *CoreServer) CreateProblemInteractor(ctx context.Context, request corev1.CreateProblemInteractorRequestObject) (corev1.CreateProblemInteractorResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, interactorDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemInteractor200JSONResponse{Message: strPtr("Interactor created successfully")}, nil
}

// GetProblemInteractor handles GET /problems/{problemId}/interactors/{name}
func (h *CoreServer) GetProblemInteractor(ctx context.Context, request corev1.GetProblemInteractorRequestObject) (corev1.GetProblemInteractorResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, interactorDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemInteractor200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemInteractor handles PUT /problems/{problemId}/interactors/{name}
func (h *CoreServer) UpdateProblemInteractor(ctx context.Context, request corev1.UpdateProblemInteractorRequestObject) (corev1.UpdateProblemInteractorResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, interactorDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemInteractor200JSONResponse{Message: strPtr("Interactor updated successfully")}, nil
}

// DeleteProblemInteractor handles DELETE /problems/{problemId}/interactors/{name}
func (h *CoreServer) DeleteProblemInteractor(ctx context.Context, request corev1.DeleteProblemInteractorRequestObject) (corev1.DeleteProblemInteractorResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, interactorDir, request.Name, "interactor"); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemInteractor200JSONResponse{Message: strPtr("Interactor deleted successfully")}, nil
}

// SetProblemInteractorMain handles PATCH /problems/{problemId}/interactors/main
func (h *CoreServer) SetProblemInteractorMain(ctx context.Context, request corev1.SetProblemInteractorMainRequestObject) (corev1.SetProblemInteractorMainResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if err := h.setMainComponent(ctx, request.ProblemId, interactorDir, "interactor", request.Body.Name); err != nil {
		return nil, err
	}
	return corev1.SetProblemInteractorMain200JSONResponse{Message: strPtr("Main interactor selected successfully")}, nil
}

// ListProblemValidators handles GET /problems/{problemId}/validators
func (h *CoreServer) ListProblemValidators(ctx context.Context, request corev1.ListProblemValidatorsRequestObject) (corev1.ListProblemValidatorsResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, validatorDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemValidators200JSONResponse(resp), nil
}

// CreateProblemValidator handles POST /problems/{problemId}/validators
func (h *CoreServer) CreateProblemValidator(ctx context.Context, request corev1.CreateProblemValidatorRequestObject) (corev1.CreateProblemValidatorResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, validatorDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemValidator200JSONResponse{Message: strPtr("Validator created successfully")}, nil
}

// GetProblemValidator handles GET /problems/{problemId}/validators/{name}
func (h *CoreServer) GetProblemValidator(ctx context.Context, request corev1.GetProblemValidatorRequestObject) (corev1.GetProblemValidatorResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, validatorDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemValidator200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemValidator handles PUT /problems/{problemId}/validators/{name}
func (h *CoreServer) UpdateProblemValidator(ctx context.Context, request corev1.UpdateProblemValidatorRequestObject) (corev1.UpdateProblemValidatorResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, validatorDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemValidator200JSONResponse{Message: strPtr("Validator updated successfully")}, nil
}

// DeleteProblemValidator handles DELETE /problems/{problemId}/validators/{name}
func (h *CoreServer) DeleteProblemValidator(ctx context.Context, request corev1.DeleteProblemValidatorRequestObject) (corev1.DeleteProblemValidatorResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, validatorDir, request.Name, "validator"); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemValidator200JSONResponse{Message: strPtr("Validator deleted successfully")}, nil
}

// SetProblemValidatorMain handles PATCH /problems/{problemId}/validators/main
func (h *CoreServer) SetProblemValidatorMain(ctx context.Context, request corev1.SetProblemValidatorMainRequestObject) (corev1.SetProblemValidatorMainResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if err := h.setMainComponent(ctx, request.ProblemId, validatorDir, "validator", request.Body.Name); err != nil {
		return nil, err
	}
	return corev1.SetProblemValidatorMain200JSONResponse{Message: strPtr("Main validator selected successfully")}, nil
}

// ListProblemMediaFiles handles GET /problems/{problemId}/media
func (h *CoreServer) ListProblemMediaFiles(ctx context.Context, request corev1.ListProblemMediaFilesRequestObject) (corev1.ListProblemMediaFilesResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, mediaDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemMediaFiles200JSONResponse(resp), nil
}

// CreateProblemMediaFile handles POST /problems/{problemId}/media
func (h *CoreServer) CreateProblemMediaFile(ctx context.Context, request corev1.CreateProblemMediaFileRequestObject) (corev1.CreateProblemMediaFileResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemMediaFile200JSONResponse{Message: strPtr("Media file created successfully")}, nil
}

// GetProblemMediaFile handles GET /problems/{problemId}/media/{name}
func (h *CoreServer) GetProblemMediaFile(ctx context.Context, request corev1.GetProblemMediaFileRequestObject) (corev1.GetProblemMediaFileResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemMediaFile200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemMediaFile handles PUT /problems/{problemId}/media/{name}
func (h *CoreServer) UpdateProblemMediaFile(ctx context.Context, request corev1.UpdateProblemMediaFileRequestObject) (corev1.UpdateProblemMediaFileResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemMediaFile200JSONResponse{Message: strPtr("Media file updated successfully")}, nil
}

// DeleteProblemMediaFile handles DELETE /problems/{problemId}/media/{name}
func (h *CoreServer) DeleteProblemMediaFile(ctx context.Context, request corev1.DeleteProblemMediaFileRequestObject) (corev1.DeleteProblemMediaFileResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Name, ""); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemMediaFile200JSONResponse{Message: strPtr("Media file deleted successfully")}, nil
}

// ListProblemWorkshopSubmissions handles GET /problems/{problemId}/submissions
func (h *CoreServer) ListProblemWorkshopSubmissions(ctx context.Context, request corev1.ListProblemWorkshopSubmissionsRequestObject) (corev1.ListProblemWorkshopSubmissionsResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, solutionDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemWorkshopSubmissions200JSONResponse(resp), nil
}

// CreateProblemWorkshopSubmission handles POST /problems/{problemId}/submissions
func (h *CoreServer) CreateProblemWorkshopSubmission(ctx context.Context, request corev1.CreateProblemWorkshopSubmissionRequestObject) (corev1.CreateProblemWorkshopSubmissionResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, solutionDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemWorkshopSubmission200JSONResponse{Message: strPtr("Author solution file created successfully")}, nil
}

// GetProblemWorkshopSubmission handles GET /problems/{problemId}/submissions/{name}
func (h *CoreServer) GetProblemWorkshopSubmission(ctx context.Context, request corev1.GetProblemWorkshopSubmissionRequestObject) (corev1.GetProblemWorkshopSubmissionResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, solutionDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemWorkshopSubmission200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemWorkshopSubmission handles PUT /problems/{problemId}/submissions/{name}
func (h *CoreServer) UpdateProblemWorkshopSubmission(ctx context.Context, request corev1.UpdateProblemWorkshopSubmissionRequestObject) (corev1.UpdateProblemWorkshopSubmissionResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, solutionDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemWorkshopSubmission200JSONResponse{Message: strPtr("Author solution file updated successfully")}, nil
}

// DeleteProblemWorkshopSubmission handles DELETE /problems/{problemId}/submissions/{name}
func (h *CoreServer) DeleteProblemWorkshopSubmission(ctx context.Context, request corev1.DeleteProblemWorkshopSubmissionRequestObject) (corev1.DeleteProblemWorkshopSubmissionResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, solutionDir, request.Name, ""); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemWorkshopSubmission200JSONResponse{Message: strPtr("Author solution file deleted successfully")}, nil
}

// ListProblemTests handles GET /problems/{problemId}/tests
func (h *CoreServer) ListProblemTests(ctx context.Context, request corev1.ListProblemTestsRequestObject) (corev1.ListProblemTestsResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, testDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemTests200JSONResponse(resp), nil
}

// CreateProblemTestFile handles POST /problems/{problemId}/tests
func (h *CoreServer) CreateProblemTestFile(ctx context.Context, request corev1.CreateProblemTestFileRequestObject) (corev1.CreateProblemTestFileResponseObject, error) {
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, testDir, request.Params.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.CreateProblemTestFile200JSONResponse{Message: strPtr("Test file created successfully")}, nil
}

// UpdateProblemTestsConfig handles PATCH /problems/{problemId}/tests/config
func (h *CoreServer) UpdateProblemTestsConfig(ctx context.Context, request corev1.UpdateProblemTestsConfigRequestObject) (corev1.UpdateProblemTestsConfigResponseObject, error) {
	if request.Body == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if !h.workshopUC.IsInitialized(ctx, request.ProblemId) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	manifest, err := h.readWorkshopManifest(ctx, request.ProblemId)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := json.Marshal(request.Body)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid tests config payload")
	}

	var testsMeta models.TestsMetadata
	if err := json.Unmarshal(bodyBytes, &testsMeta); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to parse tests config")
	}
	if err := validateTestsMetadata(&testsMeta, manifest); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid tests config")
	}

	testsBytes, err := json.MarshalIndent(testsMeta, "", "  ")
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to encode tests config")
	}

	updateReq := models.UpdateFileRequest{
		ProblemID: request.ProblemId,
		UserID:    middleware.GetUser(ctx).Id,
		Path:      filepath.Join(testDir, "tests.json"),
		Content:   testsBytes,
	}
	if err := h.workshopUC.UpdateProblemFile(ctx, updateReq); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update tests config")
	}

	return corev1.UpdateProblemTestsConfig200JSONResponse{Message: strPtr("Tests config updated successfully")}, nil
}

// GetProblemTestFile handles GET /problems/{problemId}/tests/{name}
func (h *CoreServer) GetProblemTestFile(ctx context.Context, request corev1.GetProblemTestFileRequestObject) (corev1.GetProblemTestFileResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, testDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemTestFile200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
}

// UpdateProblemTestFile handles PUT /problems/{problemId}/tests/{name}
func (h *CoreServer) UpdateProblemTestFile(ctx context.Context, request corev1.UpdateProblemTestFileRequestObject) (corev1.UpdateProblemTestFileResponseObject, error) {
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, testDir, request.Name, request.Body); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemTestFile200JSONResponse{Message: strPtr("Test file updated successfully")}, nil
}

// DeleteProblemTestFile handles DELETE /problems/{problemId}/tests/{name}
func (h *CoreServer) DeleteProblemTestFile(ctx context.Context, request corev1.DeleteProblemTestFileRequestObject) (corev1.DeleteProblemTestFileResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, testDir, request.Name, ""); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemTestFile200JSONResponse{Message: strPtr("Test file deleted successfully")}, nil
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

	testNumbers := make([]int, len(request.Body.TestNumbers))
	for i, num := range request.Body.TestNumbers {
		testNumbers[i] = num
	}

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

	return corev1.GenerateTests200JSONResponse{Message: strPtr("Tests generated successfully")}, nil
}

// ValidateAllTests handles POST /problems/{problemId}/workshop/tests/validate
func (h *CoreServer) ValidateAllTests(ctx context.Context, request corev1.ValidateAllTestsRequestObject) (corev1.ValidateAllTestsResponseObject, error) {
	user := middleware.GetUser(ctx)

	report, err := h.workshopUC.ValidateAllTests(ctx, request.ProblemId, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to validate tests")
	}

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

func (h *CoreServer) listWorkshopCollection(ctx context.Context, problemID uuid.UUID, dir string) (corev1.WorkshopFileListResponse, error) {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return corev1.WorkshopFileListResponse{}, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	files, err := h.workshopUC.ListProblemFiles(ctx, problemID, dir)
	if err != nil {
		return corev1.WorkshopFileListResponse{}, pkg.Wrap(pkg.ErrInternal, err, "failed to list files")
	}

	contractFiles := toContractFileEntries(files)
	return corev1.WorkshopFileListResponse{Files: &contractFiles}, nil
}

func (h *CoreServer) createWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name string, body io.Reader) error {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}
	if body == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	cleanName, err := sanitizeFileName(name)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid file name")
	}

	content, err := io.ReadAll(body)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to read request body")
	}

	user := middleware.GetUser(ctx)
	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    user.Id,
		Path:      filepath.Join(dir, cleanName),
		Content:   content,
	}); err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to create file")
	}

	return nil
}

func (h *CoreServer) getWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name string) ([]byte, error) {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	cleanName, err := sanitizeFileName(name)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid file name")
	}

	content, err := h.workshopUC.ReadProblemFile(ctx, problemID, filepath.Join(dir, cleanName))
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "file not found")
	}
	return content, nil
}

func (h *CoreServer) updateWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name string, body io.Reader) error {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}
	if body == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	cleanName, err := sanitizeFileName(name)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid file name")
	}

	content, err := io.ReadAll(body)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to read request body")
	}

	user := middleware.GetUser(ctx)
	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    user.Id,
		Path:      filepath.Join(dir, cleanName),
		Content:   content,
	}); err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to update file")
	}

	return nil
}

func (h *CoreServer) deleteWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name, componentType string) error {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	cleanName, err := sanitizeFileName(name)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid file name")
	}

	path := filepath.Join(dir, cleanName)
	if err := h.workshopUC.DeleteProblemFile(ctx, problemID, path); err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to delete file")
	}

	if componentType != "" {
		if err := h.removeMainComponentIfMatches(ctx, problemID, middleware.GetUser(ctx).Id, componentType, path); err != nil {
			return err
		}
	}

	return nil
}

func (h *CoreServer) setMainComponent(ctx context.Context, problemID uuid.UUID, dir, componentType, name string) error {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	cleanName, err := sanitizeFileName(name)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid file name")
	}

	componentPath := filepath.Join(dir, cleanName)
	if _, err := h.workshopUC.ReadProblemFile(ctx, problemID, componentPath); err != nil {
		return pkg.Wrap(pkg.ErrNotFound, err, "file not found")
	}

	manifest, err := h.readWorkshopManifest(ctx, problemID)
	if err != nil {
		return err
	}

	index := -1
	for i := range manifest.FilesMetadata {
		if manifest.FilesMetadata[i].Type == componentType {
			index = i
			break
		}
	}

	if index >= 0 {
		manifest.FilesMetadata[index].Filename = componentPath
		if strings.TrimSpace(manifest.FilesMetadata[index].Compiler) == "" {
			manifest.FilesMetadata[index].Compiler = compilerByFilename(cleanName)
		}
		manifest.FilesMetadata[index].BinarySha256 = nil
		manifest.FilesMetadata = append(manifest.FilesMetadata, models.FileMetadata{
			Type:         componentType,
			Filename:     componentPath,
			Compiler:     compilerByFilename(cleanName),
			BinarySha256: nil,
			Dependencies: []models.Dependency{},
		})
	}

	if err := validateManifest(manifest); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to set main component")
	}

	if err := h.saveWorkshopManifest(ctx, problemID, middleware.GetUser(ctx).Id, manifest); err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) removeMainComponentIfMatches(ctx context.Context, problemID, userID uuid.UUID, componentType, deletedPath string) error {
	manifest, err := h.readWorkshopManifest(ctx, problemID)
	if err != nil {
		return err
	}

	changed := false
	filtered := make([]models.FileMetadata, 0, len(manifest.FilesMetadata))
	for _, meta := range manifest.FilesMetadata {
		if meta.Type == componentType && meta.Filename == deletedPath {
			changed = true
			continue
		}
		filtered = append(filtered, meta)
	}

	if !changed {
		return nil
	}

	manifest.FilesMetadata = filtered
	if err := validateManifest(manifest); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to update manifest metadata")
	}
	return h.saveWorkshopManifest(ctx, problemID, userID, manifest)
}

func (h *CoreServer) readWorkshopManifest(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	content, err := h.workshopUC.ReadProblemFile(ctx, problemID, "manifest.json")
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "manifest not found")
	}

	var manifest models.ProblemManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to decode manifest")
	}

	return &manifest, nil
}

func (h *CoreServer) saveWorkshopManifest(ctx context.Context, problemID, userID uuid.UUID, manifest *models.ProblemManifest) error {
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to encode manifest")
	}

	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    userID,
		Path:      "manifest.json",
		Content:   manifestBytes,
	}); err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to update manifest")
	}

	return nil
}

func (h *CoreServer) syncProblemTitleIfNeeded(ctx context.Context, problemID uuid.UUID, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}

	problem, err := h.problemsUC.GetProblemById(ctx, problemID)
	if err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to get problem for title sync")
	}
	if strings.TrimSpace(problem.Title) == title {
		return nil
	}

	if err := h.problemsUC.UpdateProblem(ctx, problemID, &models.ProblemUpdate{Title: &title}); err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, "failed to sync problem title")
	}

	return nil
}

func (h *CoreServer) toContractLimits(manifest *models.ProblemManifest) corev1.ProblemLimits {
	return corev1.ProblemLimits{
		CodeSizeLimitKb: manifest.CodeSizeLimitKb,
		MaxScore:        manifest.MaxScore,
		MemoryLimitMb:   manifest.MemoryLimitMb,
		ProblemType:     manifest.ProblemType,
		StdoutLimitMb:   manifest.StdoutLimitMb,
		TimeLimitMs:     manifest.TimeLimitMs,
	}
}

func (h *CoreServer) toContractStatement(manifest *models.ProblemManifest) corev1.ProblemStatement {
	return corev1.ProblemStatement{
		InputFormat:  manifest.Statement.InputFormat,
		Interaction:  optionalString(manifest.Statement.Interaction),
		Legend:       manifest.Statement.Legend,
		Notes:        optionalString(manifest.Statement.Notes),
		OutputFormat: manifest.Statement.OutputFormat,
		Scoring:      optionalString(manifest.Statement.Scoring),
		Title:        manifest.Statement.Title,
	}
}

func toContractFileEntries(files []models.FileEntry) []corev1.FileEntry {
	contractFiles := make([]corev1.FileEntry, len(files))
	for i, f := range files {
		contractFiles[i] = corev1.FileEntry{
			Path:        strPtr(f.Path),
			IsDirectory: boolPtr(f.IsDirectory),
			Size:        int64Ptr(f.Size),
		}
	}
	return contractFiles
}

func validateManifest(m *models.ProblemManifest) error {
	if m.TimeLimitMs <= 0 || m.MemoryLimitMb <= 0 {
		return fmt.Errorf("limits must be positive")
	}
	if strings.TrimSpace(m.Statement.Title) == "" {
		return fmt.Errorf("title is required")
	}
	return nil
}

func validateTestsMetadata(t *models.TestsMetadata, m *models.ProblemManifest) error {
	if len(t.Tests) == 0 {
		return fmt.Errorf("at least one test case is required")
	}
	return nil
}

func sanitizeFileName(name string) (string, error) {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "", fmt.Errorf("name is required")
	}
	if strings.Contains(clean, "..") || strings.Contains(clean, "/") || strings.Contains(clean, `\\`) {
		return "", fmt.Errorf("path separators are not allowed")
	}
	return clean, nil
}

func compilerByFilename(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".cpp", ".cc", ".cxx":
		return "cpp17"
	case ".py":
		return "python3"
	case ".go":
		return "golang"
	case ".java":
		return "java11"
	case ".c":
		return "c11"
	default:
		return "cpp17"
	}
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

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
