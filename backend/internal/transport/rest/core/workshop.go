package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/internal/usecase"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

const (
	checkerDir    = "checkers"
	generatorDir  = "generators"
	interactorDir = "interactors"
	validatorDir  = "validators"
	libDir        = "lib"
	mediaDir      = "media"
	solutionDir   = "solutions"
	testDir       = "tests"

	MaxSourceFileSize    = 2 * 1024 * 1024   // 2 MB
	MaxStatementFileSize = 2 * 1024 * 1024   // 2 MB
	MaxMediaFileSize     = 50 * 1024 * 1024  // 50 MB
	MaxTestFileSize      = 256 * 1024 * 1024 // 256 MB
	MaxPackageZipSize    = 500 * 1024 * 1024 // 500 MB
)

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

	if err := h.saveWorkshopManifest(ctx, request.ProblemId, manifest); err != nil {
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

	lang := "en"
	if request.Params.Lang != nil && *request.Params.Lang != "" {
		lang = *request.Params.Lang
	}

	// 1. Get list of available languages from statements/ folder
	var languages []string
	if files, err := h.workshopUC.ListProblemFiles(ctx, request.ProblemId, "statements"); err == nil {
		for _, f := range files {
			if !f.IsDirectory && strings.HasSuffix(f.Path, ".md") {
				base := filepath.Base(f.Path)
				langCode := strings.TrimSuffix(base, ".md")
				if langCode != "" {
					languages = append(languages, langCode)
				}
			}
		}
	}
	sort.Strings(languages)

	// Ensure the default language is listed if the list is empty
	if len(languages) == 0 {
		languages = []string{"en"}
	}

	// 2. Read statement for specific language from workspace
	var stmt models.Statement
	filePath := fmt.Sprintf("statements/%s.md", lang)
	fileData, err := h.workshopUC.ReadProblemFile(ctx, request.ProblemId, filePath)
	if err == nil {
		stmt = usecase.ParseStatementMarkdown(string(fileData))
		// If title tag is empty, fallback to the manifest title
		if strings.TrimSpace(stmt.Title) == "" {
			stmt.Title = manifest.Statement.Title
		}
	} else {
		// If requested lang is "en" and en.md is missing, fallback to manifest
		if lang == "en" {
			stmt = manifest.Statement
		} else {
			// Return empty statement with title of the main problem
			stmt.Title = manifest.Statement.Title
		}
	}

	resp := h.toContractStatementForLang(stmt, languages, lang)
	return corev1.GetProblemStatement200JSONResponse(resp), nil
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

	lang := "en"
	if request.Params.Lang != nil && *request.Params.Lang != "" {
		lang = *request.Params.Lang
	}

	// 1. Get existing statement for this language to apply patch
	var stmt models.Statement
	filePath := fmt.Sprintf("statements/%s.md", lang)
	fileData, err := h.workshopUC.ReadProblemFile(ctx, request.ProblemId, filePath)
	if err == nil {
		stmt = usecase.ParseStatementMarkdown(string(fileData))
		if strings.TrimSpace(stmt.Title) == "" {
			stmt.Title = manifest.Statement.Title
		}
	} else {
		if lang == "en" {
			stmt = manifest.Statement
		} else {
			stmt.Title = manifest.Statement.Title
		}
	}

	body := request.Body
	if body.Title != nil {
		stmt.Title = *body.Title
	}
	if body.Legend != nil {
		stmt.Legend = *body.Legend
	}
	if body.InputFormat != nil {
		stmt.InputFormat = *body.InputFormat
	}
	if body.OutputFormat != nil {
		stmt.OutputFormat = *body.OutputFormat
	}
	if body.Notes != nil {
		stmt.Notes = *body.Notes
	}
	if body.Interaction != nil {
		stmt.Interaction = *body.Interaction
	}
	if body.Scoring != nil {
		stmt.Scoring = *body.Scoring
	}

	// 2. Write statement back to workspace storage
	stmtBytes := []byte(usecase.RenderStatementMarkdown(stmt))
	if int64(len(stmtBytes)) > MaxStatementFileSize {
		return nil, pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "statement size exceeds maximum allowed limit of 2MB")
	}
	user := middleware.GetUser(ctx)
	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: request.ProblemId,
		UserID:    user.Id,
		Path:      filePath,
		Content:   stmtBytes,
	}); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to save statement file")
	}

	// 3. If saving default language statement (en), sync to DB manifest and problem.yaml
	if lang == "en" {
		manifest.Statement = stmt
		if err := validateManifest(manifest); err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid statement update")
		}

		if err := h.saveWorkshopManifest(ctx, request.ProblemId, manifest); err != nil {
			return nil, err
		}
		if err := h.syncProblemTitleIfNeeded(ctx, request.ProblemId, manifest.Statement.Title); err != nil {
			return nil, err
		}
	}

	// 4. Retrieve list of languages for response
	var languages []string
	if files, err := h.workshopUC.ListProblemFiles(ctx, request.ProblemId, "statements"); err == nil {
		for _, f := range files {
			if !f.IsDirectory && strings.HasSuffix(f.Path, ".md") {
				base := filepath.Base(f.Path)
				langCode := strings.TrimSuffix(base, ".md")
				if langCode != "" {
					languages = append(languages, langCode)
				}
			}
		}
	}
	sort.Strings(languages)
	if len(languages) == 0 {
		languages = []string{"en"}
	}

	resp := h.toContractStatementForLang(stmt, languages, lang)
	return corev1.UpdateProblemStatement200JSONResponse(resp), nil
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
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.createWorkshopCollectionTextFile(ctx, request.ProblemId, checkerDir, request.Params.Name, content); err != nil {
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
	return corev1.GetProblemChecker200TextResponse(string(content)), nil
}

// UpdateProblemChecker handles PUT /problems/{problemId}/checkers/{name}
func (h *CoreServer) UpdateProblemChecker(ctx context.Context, request corev1.UpdateProblemCheckerRequestObject) (corev1.UpdateProblemCheckerResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, request.ProblemId, checkerDir, request.Name, content); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemChecker200JSONResponse{Message: strPtr("Checker updated successfully")}, nil
}

// DeleteProblemChecker handles DELETE /problems/{problemId}/checkers/{name}
func (h *CoreServer) DeleteProblemChecker(ctx context.Context, request corev1.DeleteProblemCheckerRequestObject) (corev1.DeleteProblemCheckerResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, checkerDir, request.Name); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemChecker200JSONResponse{Message: strPtr("Checker deleted successfully")}, nil
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
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.createWorkshopCollectionTextFile(ctx, request.ProblemId, generatorDir, request.Params.Name, content); err != nil {
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
	return corev1.GetProblemGenerator200TextResponse(string(content)), nil
}

// UpdateProblemGenerator handles PUT /problems/{problemId}/generators/{name}
func (h *CoreServer) UpdateProblemGenerator(ctx context.Context, request corev1.UpdateProblemGeneratorRequestObject) (corev1.UpdateProblemGeneratorResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, request.ProblemId, generatorDir, request.Name, content); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemGenerator200JSONResponse{Message: strPtr("Generator updated successfully")}, nil
}

// DeleteProblemGenerator handles DELETE /problems/{problemId}/generators/{name}
func (h *CoreServer) DeleteProblemGenerator(ctx context.Context, request corev1.DeleteProblemGeneratorRequestObject) (corev1.DeleteProblemGeneratorResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, generatorDir, request.Name); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemGenerator200JSONResponse{Message: strPtr("Generator deleted successfully")}, nil
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
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.createWorkshopCollectionTextFile(ctx, request.ProblemId, interactorDir, request.Params.Name, content); err != nil {
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
	return corev1.GetProblemInteractor200TextResponse(string(content)), nil
}

// UpdateProblemInteractor handles PUT /problems/{problemId}/interactors/{name}
func (h *CoreServer) UpdateProblemInteractor(ctx context.Context, request corev1.UpdateProblemInteractorRequestObject) (corev1.UpdateProblemInteractorResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, request.ProblemId, interactorDir, request.Name, content); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemInteractor200JSONResponse{Message: strPtr("Interactor updated successfully")}, nil
}

// DeleteProblemInteractor handles DELETE /problems/{problemId}/interactors/{name}
func (h *CoreServer) DeleteProblemInteractor(ctx context.Context, request corev1.DeleteProblemInteractorRequestObject) (corev1.DeleteProblemInteractorResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, interactorDir, request.Name); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemInteractor200JSONResponse{Message: strPtr("Interactor deleted successfully")}, nil
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
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.createWorkshopCollectionTextFile(ctx, request.ProblemId, validatorDir, request.Params.Name, content); err != nil {
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
	return corev1.GetProblemValidator200TextResponse(string(content)), nil
}

// UpdateProblemValidator handles PUT /problems/{problemId}/validators/{name}
func (h *CoreServer) UpdateProblemValidator(ctx context.Context, request corev1.UpdateProblemValidatorRequestObject) (corev1.UpdateProblemValidatorResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, request.ProblemId, validatorDir, request.Name, content); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemValidator200JSONResponse{Message: strPtr("Validator updated successfully")}, nil
}

// DeleteProblemValidator handles DELETE /problems/{problemId}/validators/{name}
func (h *CoreServer) DeleteProblemValidator(ctx context.Context, request corev1.DeleteProblemValidatorRequestObject) (corev1.DeleteProblemValidatorResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, validatorDir, request.Name); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemValidator200JSONResponse{Message: strPtr("Validator deleted successfully")}, nil
}

// ListProblemLibs handles GET /problems/{problemId}/lib
func (h *CoreServer) ListProblemLibs(ctx context.Context, request corev1.ListProblemLibsRequestObject) (corev1.ListProblemLibsResponseObject, error) {
	resp, err := h.listWorkshopCollection(ctx, request.ProblemId, libDir)
	if err != nil {
		return nil, err
	}
	return corev1.ListProblemLibs200JSONResponse(resp), nil
}

// CreateProblemLib handles POST /problems/{problemId}/lib
func (h *CoreServer) CreateProblemLib(ctx context.Context, request corev1.CreateProblemLibRequestObject) (corev1.CreateProblemLibResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.createWorkshopCollectionTextFile(ctx, request.ProblemId, libDir, request.Params.Name, content); err != nil {
		return nil, err
	}
	return corev1.CreateProblemLib200JSONResponse{Message: strPtr("Library file created successfully")}, nil
}

// GetProblemLib handles GET /problems/{problemId}/lib/{name}
func (h *CoreServer) GetProblemLib(ctx context.Context, request corev1.GetProblemLibRequestObject) (corev1.GetProblemLibResponseObject, error) {
	content, err := h.getWorkshopCollectionFile(ctx, request.ProblemId, libDir, request.Name)
	if err != nil {
		return nil, err
	}
	return corev1.GetProblemLib200TextResponse(string(content)), nil
}

// UpdateProblemLib handles PUT /problems/{problemId}/lib/{name}
func (h *CoreServer) UpdateProblemLib(ctx context.Context, request corev1.UpdateProblemLibRequestObject) (corev1.UpdateProblemLibResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, request.ProblemId, libDir, request.Name, content); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemLib200JSONResponse{Message: strPtr("Library file updated successfully")}, nil
}

// DeleteProblemLib handles DELETE /problems/{problemId}/lib/{name}
func (h *CoreServer) DeleteProblemLib(ctx context.Context, request corev1.DeleteProblemLibRequestObject) (corev1.DeleteProblemLibResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, libDir, request.Name); err != nil {
		return nil, err
	}
	return corev1.DeleteProblemLib200JSONResponse{Message: strPtr("Library file deleted successfully")}, nil
}

type mediaFileResponse struct {
	content     []byte
	contentType string
}

func (r mediaFileResponse) VisitGetProblemMediaFileResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", r.contentType)
	w.Header().Set("Content-Length", fmt.Sprint(len(r.content)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(r.contentType, "image/svg") {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'unsafe-inline'")
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.content)
	return err
}

func getMediaContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

func sanitizeSVG(content []byte) []byte {
	str := string(content)
	reScript := regexp.MustCompile(`(?i)<script[\s\S]*?>[\s\S]*?</script>`)
	str = reScript.ReplaceAllString(str, "")
	reOnEvent := regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	str = reOnEvent.ReplaceAllString(str, "")
	reJsHref := regexp.MustCompile(`(?i)href\s*=\s*["']?\s*javascript:[^"'>\s]*["']?`)
	str = reJsHref.ReplaceAllString(str, `href="#"`)
	return []byte(str)
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
	var bodyReader io.Reader = request.Body
	if strings.HasSuffix(strings.ToLower(request.Params.Name), ".svg") {
		if bodyBytes, err := io.ReadAll(request.Body); err == nil {
			sanitized := sanitizeSVG(bodyBytes)
			bodyReader = bytes.NewReader(sanitized)
		}
	}
	if err := h.createWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Params.Name, bodyReader); err != nil {
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
	return mediaFileResponse{
		content:     content,
		contentType: getMediaContentType(request.Name),
	}, nil
}

// UpdateProblemMediaFile handles PUT /problems/{problemId}/media/{name}
func (h *CoreServer) UpdateProblemMediaFile(ctx context.Context, request corev1.UpdateProblemMediaFileRequestObject) (corev1.UpdateProblemMediaFileResponseObject, error) {
	var bodyReader io.Reader = request.Body
	if strings.HasSuffix(strings.ToLower(request.Name), ".svg") {
		if bodyBytes, err := io.ReadAll(request.Body); err == nil {
			sanitized := sanitizeSVG(bodyBytes)
			bodyReader = bytes.NewReader(sanitized)
		}
	}
	if err := h.updateWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Name, bodyReader); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemMediaFile200JSONResponse{Message: strPtr("Media file updated successfully")}, nil
}

// DeleteProblemMediaFile handles DELETE /problems/{problemId}/media/{name}
func (h *CoreServer) DeleteProblemMediaFile(ctx context.Context, request corev1.DeleteProblemMediaFileRequestObject) (corev1.DeleteProblemMediaFileResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, mediaDir, request.Name); err != nil {
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
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.createWorkshopCollectionTextFile(ctx, request.ProblemId, solutionDir, request.Params.Name, content); err != nil {
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
	return corev1.GetProblemWorkshopSubmission200TextResponse(string(content)), nil
}

// UpdateProblemWorkshopSubmission handles PUT /problems/{problemId}/submissions/{name}
func (h *CoreServer) UpdateProblemWorkshopSubmission(ctx context.Context, request corev1.UpdateProblemWorkshopSubmissionRequestObject) (corev1.UpdateProblemWorkshopSubmissionResponseObject, error) {
	content := ""
	if request.Body != nil {
		content = string(*request.Body)
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, request.ProblemId, solutionDir, request.Name, content); err != nil {
		return nil, err
	}
	return corev1.UpdateProblemWorkshopSubmission200JSONResponse{Message: strPtr("Author solution file updated successfully")}, nil
}

// DeleteProblemWorkshopSubmission handles DELETE /problems/{problemId}/submissions/{name}
func (h *CoreServer) DeleteProblemWorkshopSubmission(ctx context.Context, request corev1.DeleteProblemWorkshopSubmissionRequestObject) (corev1.DeleteProblemWorkshopSubmissionResponseObject, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, solutionDir, request.Name); err != nil {
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

	bodyBytes, err := json.Marshal(request.Body)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid tests config payload")
	}

	var testsMeta models.TestsMetadata
	if err := json.Unmarshal(bodyBytes, &testsMeta); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to parse tests config")
	}
	if err := validateTestsMetadata(&testsMeta); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid tests config")
	}

	if err := h.workshopUC.UpdateTestsConfig(ctx, request.ProblemId, &testsMeta); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update tests config")
	}

	return corev1.UpdateProblemTestsConfig200JSONResponse{Message: strPtr("Tests config updated successfully")}, nil
}

// GetProblemTestFile handles GET /problems/{problemId}/tests/{name}
func (h *CoreServer) GetProblemTestFile(ctx context.Context, request corev1.GetProblemTestFileRequestObject) (corev1.GetProblemTestFileResponseObject, error) {
	if request.Name == "tests.json" {
		testsMeta, err := h.workshopUC.GetTestsConfig(ctx, request.ProblemId)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrNotFound, err, "tests config not found")
		}
		content, err := json.MarshalIndent(testsMeta, "", "  ")
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to marshal tests config")
		}
		return corev1.GetProblemTestFile200ApplicationoctetStreamResponse{Body: bytes.NewReader(content), ContentLength: int64(len(content))}, nil
	}

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
	if err := h.deleteWorkshopCollectionFile(ctx, request.ProblemId, testDir, request.Name); err != nil {
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
	copy(testNumbers, request.Body.TestNumbers)

	var arguments [][]string
	if request.Body.Arguments != nil {
		arguments = make([][]string, len(*request.Body.Arguments))
		copy(arguments, *request.Body.Arguments)
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
		copy(testNumbers, *request.Body.TestNumbers)
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

	manifest, err := h.readWorkshopManifest(ctx, problemID)
	if err != nil {
		manifest = &models.ProblemManifest{}
	}

	contractFiles := toContractFileEntries(files, manifest)
	return corev1.WorkshopFileListResponse{Files: &contractFiles}, nil
}

func validateCollectionFileExtension(dir, name string) error {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return fmt.Errorf("file extension is required")
	}

	allowedLangs := map[string]bool{
		".cpp":  true,
		".cc":   true,
		".cxx":  true,
		".py":   true,
		".go":   true,
		".java": true,
	}

	switch dir {
	case checkerDir, generatorDir, interactorDir, validatorDir, solutionDir:
		if !allowedLangs[ext] {
			return fmt.Errorf("unsupported file extension %q for %s. Allowed extensions: .cpp, .py, .go, .java", ext, dir)
		}
	case libDir:
		allowedLib := map[string]bool{
			".cpp":  true,
			".cc":   true,
			".cxx":  true,
			".h":    true,
			".hpp":  true,
			".inc":  true,
			".py":   true,
			".go":   true,
			".java": true,
		}
		if !allowedLib[ext] {
			return fmt.Errorf("unsupported file extension %q for library. Allowed extensions: .cpp, .h, .hpp, .inc, .py, .go, .java", ext)
		}
	}
	return nil
}

func (h *CoreServer) saveWorkshopCollectionBytes(ctx context.Context, problemID uuid.UUID, dir, name string, content []byte, actionErrMsg string) error {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	cleanName, err := sanitizeFileName(name)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "invalid file name")
	}

	if err := validateCollectionFileExtension(dir, cleanName); err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, err.Error())
	}

	var maxLimit int64
	switch dir {
	case testDir:
		maxLimit = MaxTestFileSize
	case mediaDir:
		maxLimit = MaxMediaFileSize
	default:
		maxLimit = MaxSourceFileSize
	}

	if int64(len(content)) > maxLimit {
		return pkg.Wrap(pkg.ErrPayloadTooLarge, nil, fmt.Sprintf("file size (%d bytes) exceeds maximum allowed limit of %d bytes", len(content), maxLimit))
	}

	user := middleware.GetUser(ctx)
	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: problemID,
		UserID:    user.Id,
		Path:      filepath.Join(dir, cleanName),
		Content:   content,
	}); err != nil {
		return pkg.Wrap(pkg.ErrInternal, err, actionErrMsg)
	}

	return nil
}

func (h *CoreServer) saveWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name string, body io.Reader, actionErrMsg string) error {
	if body == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	var maxLimit int64
	switch dir {
	case testDir:
		maxLimit = MaxTestFileSize
	case mediaDir:
		maxLimit = MaxMediaFileSize
	default:
		maxLimit = MaxSourceFileSize
	}

	limitedReader := io.LimitReader(body, maxLimit+1)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return pkg.Wrap(pkg.ErrBadInput, err, "failed to read request body")
	}
	if int64(len(content)) > maxLimit {
		return pkg.Wrap(pkg.ErrPayloadTooLarge, nil, fmt.Sprintf("file size exceeds maximum allowed limit of %d bytes", maxLimit))
	}

	return h.saveWorkshopCollectionBytes(ctx, problemID, dir, name, content, actionErrMsg)
}

func (h *CoreServer) createWorkshopCollectionTextFile(ctx context.Context, problemID uuid.UUID, dir, name, content string) error {
	if dir == checkerDir || dir == generatorDir || dir == interactorDir || dir == validatorDir {
		existing, err := h.workshopUC.ListProblemFiles(ctx, problemID, dir)
		if err == nil {
			for _, f := range existing {
				if !f.IsDirectory {
					return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("a component file already exists in %s. Only one instance is allowed per problem.", dir))
				}
			}
		}
	}
	return h.saveWorkshopCollectionBytes(ctx, problemID, dir, name, []byte(content), "failed to create file")
}

func (h *CoreServer) updateWorkshopCollectionTextFile(ctx context.Context, problemID uuid.UUID, dir, name, content string) error {
	return h.saveWorkshopCollectionBytes(ctx, problemID, dir, name, []byte(content), "failed to update file")
}

func (h *CoreServer) createWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name string, body io.Reader) error {
	if dir == checkerDir || dir == generatorDir || dir == interactorDir || dir == validatorDir {
		existing, err := h.workshopUC.ListProblemFiles(ctx, problemID, dir)
		if err == nil {
			for _, f := range existing {
				if !f.IsDirectory {
					return pkg.Wrap(pkg.ErrBadInput, nil, fmt.Sprintf("a component file already exists in %s. Only one instance is allowed per problem.", dir))
				}
			}
		}
	}
	return h.saveWorkshopCollectionFile(ctx, problemID, dir, name, body, "failed to create file")
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
	return h.saveWorkshopCollectionFile(ctx, problemID, dir, name, body, "failed to update file")
}

func (h *CoreServer) deleteWorkshopCollectionFile(ctx context.Context, problemID uuid.UUID, dir, name string) error {
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

	return nil
}

func (h *CoreServer) readWorkshopManifest(ctx context.Context, problemID uuid.UUID) (*models.ProblemManifest, error) {
	if !h.workshopUC.IsInitialized(ctx, problemID) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	manifest, err := h.workshopUC.GetManifest(ctx, problemID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrNotFound, err, "manifest not found")
	}

	return manifest, nil
}

func (h *CoreServer) saveWorkshopManifest(ctx context.Context, problemID uuid.UUID, manifest *models.ProblemManifest) error {
	if err := h.workshopUC.SaveManifest(ctx, problemID, manifest); err != nil {
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
		MaxScore:      manifest.MaxScore,
		MemoryLimitMb: manifest.MemoryLimitMb,
		ProblemType:   manifest.ProblemType,
		TimeLimitMs:   manifest.TimeLimitMs,
	}
}

func (h *CoreServer) toContractStatementForLang(stmt models.Statement, languages []string, currentLang string) corev1.ProblemStatement {
	return corev1.ProblemStatement{
		InputFormat:  stmt.InputFormat,
		Interaction:  optionalString(stmt.Interaction),
		Legend:       stmt.Legend,
		Notes:        optionalString(stmt.Notes),
		OutputFormat: stmt.OutputFormat,
		Scoring:      optionalString(stmt.Scoring),
		Title:        stmt.Title,
		Languages:    &languages,
		CurrentLang:  &currentLang,
	}
}

func toContractFileEntries(files []models.FileEntry, manifest *models.ProblemManifest) []corev1.FileEntry {
	var mainChecker, mainInteractor, mainValidator string
	if manifest != nil {
		for _, meta := range manifest.FilesMetadata {
			switch meta.Type {
			case "checker":
				if mainChecker == "" {
					mainChecker = filepath.ToSlash(meta.Filename)
				}
			case "interactor":
				if mainInteractor == "" {
					mainInteractor = filepath.ToSlash(meta.Filename)
				}
			case "validator":
				if mainValidator == "" {
					mainValidator = filepath.ToSlash(meta.Filename)
				}
			}
		}
	}

	contractFiles := make([]corev1.FileEntry, len(files))
	for i, f := range files {
		isMain := false
		normalizedPath := filepath.ToSlash(f.Path)
		if manifest != nil {
			switch {
			case strings.HasPrefix(normalizedPath, "checkers/") && normalizedPath == mainChecker:
				isMain = true
			case strings.HasPrefix(normalizedPath, "interactors/") && normalizedPath == mainInteractor:
				isMain = true
			case strings.HasPrefix(normalizedPath, "validators/") && normalizedPath == mainValidator:
				isMain = true
			}
		}
		contractFiles[i] = corev1.FileEntry{
			Path:        strPtr(f.Path),
			IsDirectory: boolPtr(f.IsDirectory),
			Size:        int64Ptr(f.Size),
			IsMain:      boolPtr(isMain),
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

func validateTestsMetadata(t *models.TestsMetadata) error {
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
