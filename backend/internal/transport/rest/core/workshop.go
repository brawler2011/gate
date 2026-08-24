package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
func (h *CoreServer) GetProblemLimits(ctx context.Context, params corev1.GetProblemLimitsParams) (*corev1.ProblemLimits, error) {
	manifest, err := h.readWorkshopManifest(ctx, params.ProblemId)
	if err != nil {
		return nil, err
	}

	return h.toContractLimits(manifest), nil
}

// UpdateProblemLimits handles PATCH /problems/{problemId}/limits
func (h *CoreServer) UpdateProblemLimits(ctx context.Context, req *corev1.UpdateProblemLimitsRequest, params corev1.UpdateProblemLimitsParams) (*corev1.ProblemLimits, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	manifest, err := h.readWorkshopManifest(ctx, params.ProblemId)
	if err != nil {
		return nil, err
	}

	if req.ProblemType.IsSet() {
		manifest.ProblemType = req.ProblemType.Value
	}
	if req.TimeLimitMs.IsSet() {
		manifest.TimeLimitMs = req.TimeLimitMs.Value
	}
	if req.MemoryLimitMB.IsSet() {
		manifest.MemoryLimitMb = req.MemoryLimitMB.Value
	}
	if req.MaxScore.IsSet() {
		if req.MaxScore.Null {
			manifest.MaxScore = nil
		} else {
			score := req.MaxScore.Value
			manifest.MaxScore = &score
		}
	}
	if manifest.ProblemType != "scoring" {
		manifest.MaxScore = nil
	}

	if err := validateManifest(manifest); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid limits update")
	}

	if err := h.saveWorkshopManifest(ctx, params.ProblemId, manifest); err != nil {
		return nil, err
	}

	return h.toContractLimits(manifest), nil
}

// GetProblemStatement handles GET /problems/{problemId}/statement
func (h *CoreServer) GetProblemStatement(ctx context.Context, params corev1.GetProblemStatementParams) (*corev1.ProblemStatement, error) {
	manifest, err := h.readWorkshopManifest(ctx, params.ProblemId)
	if err != nil {
		return nil, err
	}

	lang := params.Lang.Or("en")

	// 1. Get list of available languages from statements/ folder
	var languages []string
	if files, err := h.workshopUC.ListProblemFiles(ctx, params.ProblemId, "statements"); err == nil {
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
	fileData, err := h.workshopUC.ReadProblemFile(ctx, params.ProblemId, filePath)
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

	return h.toContractStatementForLang(stmt, languages, lang), nil
}

// UpdateProblemStatement handles PATCH /problems/{problemId}/statement
func (h *CoreServer) UpdateProblemStatement(ctx context.Context, req *corev1.UpdateProblemStatementRequest, params corev1.UpdateProblemStatementParams) (*corev1.ProblemStatement, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}

	manifest, err := h.readWorkshopManifest(ctx, params.ProblemId)
	if err != nil {
		return nil, err
	}

	lang := params.Lang.Or("en")

	// 1. Get existing statement for this language to apply patch
	var stmt models.Statement
	filePath := fmt.Sprintf("statements/%s.md", lang)
	fileData, err := h.workshopUC.ReadProblemFile(ctx, params.ProblemId, filePath)
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

	if req.Title.IsSet() {
		stmt.Title = req.Title.Value
	}
	if req.Legend.IsSet() {
		stmt.Legend = req.Legend.Value
	}
	if req.InputFormat.IsSet() {
		stmt.InputFormat = req.InputFormat.Value
	}
	if req.OutputFormat.IsSet() {
		stmt.OutputFormat = req.OutputFormat.Value
	}
	if req.Notes.IsSet() {
		stmt.Notes = req.Notes.Value
	}
	if req.Interaction.IsSet() {
		stmt.Interaction = req.Interaction.Value
	}
	if req.Scoring.IsSet() {
		stmt.Scoring = req.Scoring.Value
	}

	// 2. Write statement back to workspace storage
	stmtBytes := []byte(usecase.RenderStatementMarkdown(stmt))
	if int64(len(stmtBytes)) > MaxStatementFileSize {
		return nil, pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "statement size exceeds maximum allowed limit of 2MB")
	}
	user := middleware.GetUser(ctx)
	if err := h.workshopUC.UpdateProblemFile(ctx, models.UpdateFileRequest{
		ProblemID: params.ProblemId,
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

		if err := h.saveWorkshopManifest(ctx, params.ProblemId, manifest); err != nil {
			return nil, err
		}
		if err := h.syncProblemTitleIfNeeded(ctx, params.ProblemId, manifest.Statement.Title); err != nil {
			return nil, err
		}
	}

	// 4. Retrieve list of languages for response
	var languages []string
	if files, err := h.workshopUC.ListProblemFiles(ctx, params.ProblemId, "statements"); err == nil {
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

	return h.toContractStatementForLang(stmt, languages, lang), nil
}

// ListProblemCheckers handles GET /problems/{problemId}/checkers
func (h *CoreServer) ListProblemCheckers(ctx context.Context, params corev1.ListProblemCheckersParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, checkerDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemChecker handles POST /problems/{problemId}/checkers
func (h *CoreServer) CreateProblemChecker(ctx context.Context, req corev1.CreateProblemCheckerReq, params corev1.CreateProblemCheckerParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.createWorkshopCollectionTextFile(ctx, params.ProblemId, checkerDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Checker created successfully")}, nil
}

// GetProblemChecker handles GET /problems/{problemId}/checkers/{name}
func (h *CoreServer) GetProblemChecker(ctx context.Context, params corev1.GetProblemCheckerParams) (corev1.GetProblemCheckerOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, checkerDir, params.Name)
	if err != nil {
		return corev1.GetProblemCheckerOK{}, err
	}
	return corev1.GetProblemCheckerOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemChecker handles PUT /problems/{problemId}/checkers/{name}
func (h *CoreServer) UpdateProblemChecker(ctx context.Context, req corev1.UpdateProblemCheckerReq, params corev1.UpdateProblemCheckerParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, params.ProblemId, checkerDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Checker updated successfully")}, nil
}

// DeleteProblemChecker handles DELETE /problems/{problemId}/checkers/{name}
func (h *CoreServer) DeleteProblemChecker(ctx context.Context, params corev1.DeleteProblemCheckerParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, checkerDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Checker deleted successfully")}, nil
}

// ListProblemGenerators handles GET /problems/{problemId}/generators
func (h *CoreServer) ListProblemGenerators(ctx context.Context, params corev1.ListProblemGeneratorsParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, generatorDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemGenerator handles POST /problems/{problemId}/generators
func (h *CoreServer) CreateProblemGenerator(ctx context.Context, req corev1.CreateProblemGeneratorReq, params corev1.CreateProblemGeneratorParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.createWorkshopCollectionTextFile(ctx, params.ProblemId, generatorDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Generator created successfully")}, nil
}

// GetProblemGenerator handles GET /problems/{problemId}/generators/{name}
func (h *CoreServer) GetProblemGenerator(ctx context.Context, params corev1.GetProblemGeneratorParams) (corev1.GetProblemGeneratorOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, generatorDir, params.Name)
	if err != nil {
		return corev1.GetProblemGeneratorOK{}, err
	}
	return corev1.GetProblemGeneratorOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemGenerator handles PUT /problems/{problemId}/generators/{name}
func (h *CoreServer) UpdateProblemGenerator(ctx context.Context, req corev1.UpdateProblemGeneratorReq, params corev1.UpdateProblemGeneratorParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, params.ProblemId, generatorDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Generator updated successfully")}, nil
}

// DeleteProblemGenerator handles DELETE /problems/{problemId}/generators/{name}
func (h *CoreServer) DeleteProblemGenerator(ctx context.Context, params corev1.DeleteProblemGeneratorParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, generatorDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Generator deleted successfully")}, nil
}

// ListProblemInteractors handles GET /problems/{problemId}/interactors
func (h *CoreServer) ListProblemInteractors(ctx context.Context, params corev1.ListProblemInteractorsParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, interactorDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemInteractor handles POST /problems/{problemId}/interactors
func (h *CoreServer) CreateProblemInteractor(ctx context.Context, req corev1.CreateProblemInteractorReq, params corev1.CreateProblemInteractorParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.createWorkshopCollectionTextFile(ctx, params.ProblemId, interactorDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Interactor created successfully")}, nil
}

// GetProblemInteractor handles GET /problems/{problemId}/interactors/{name}
func (h *CoreServer) GetProblemInteractor(ctx context.Context, params corev1.GetProblemInteractorParams) (corev1.GetProblemInteractorOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, interactorDir, params.Name)
	if err != nil {
		return corev1.GetProblemInteractorOK{}, err
	}
	return corev1.GetProblemInteractorOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemInteractor handles PUT /problems/{problemId}/interactors/{name}
func (h *CoreServer) UpdateProblemInteractor(ctx context.Context, req corev1.UpdateProblemInteractorReq, params corev1.UpdateProblemInteractorParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, params.ProblemId, interactorDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Interactor updated successfully")}, nil
}

// DeleteProblemInteractor handles DELETE /problems/{problemId}/interactors/{name}
func (h *CoreServer) DeleteProblemInteractor(ctx context.Context, params corev1.DeleteProblemInteractorParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, interactorDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Interactor deleted successfully")}, nil
}

// ListProblemValidators handles GET /problems/{problemId}/validators
func (h *CoreServer) ListProblemValidators(ctx context.Context, params corev1.ListProblemValidatorsParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, validatorDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemValidator handles POST /problems/{problemId}/validators
func (h *CoreServer) CreateProblemValidator(ctx context.Context, req corev1.CreateProblemValidatorReq, params corev1.CreateProblemValidatorParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.createWorkshopCollectionTextFile(ctx, params.ProblemId, validatorDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Validator created successfully")}, nil
}

// GetProblemValidator handles GET /problems/{problemId}/validators/{name}
func (h *CoreServer) GetProblemValidator(ctx context.Context, params corev1.GetProblemValidatorParams) (corev1.GetProblemValidatorOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, validatorDir, params.Name)
	if err != nil {
		return corev1.GetProblemValidatorOK{}, err
	}
	return corev1.GetProblemValidatorOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemValidator handles PUT /problems/{problemId}/validators/{name}
func (h *CoreServer) UpdateProblemValidator(ctx context.Context, req corev1.UpdateProblemValidatorReq, params corev1.UpdateProblemValidatorParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, params.ProblemId, validatorDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Validator updated successfully")}, nil
}

// DeleteProblemValidator handles DELETE /problems/{problemId}/validators/{name}
func (h *CoreServer) DeleteProblemValidator(ctx context.Context, params corev1.DeleteProblemValidatorParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, validatorDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Validator deleted successfully")}, nil
}

// ListProblemLibs handles GET /problems/{problemId}/lib
func (h *CoreServer) ListProblemLibs(ctx context.Context, params corev1.ListProblemLibsParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, libDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemLib handles POST /problems/{problemId}/lib
func (h *CoreServer) CreateProblemLib(ctx context.Context, req corev1.CreateProblemLibReq, params corev1.CreateProblemLibParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.createWorkshopCollectionTextFile(ctx, params.ProblemId, libDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Library file created successfully")}, nil
}

// GetProblemLib handles GET /problems/{problemId}/lib/{name}
func (h *CoreServer) GetProblemLib(ctx context.Context, params corev1.GetProblemLibParams) (corev1.GetProblemLibOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, libDir, params.Name)
	if err != nil {
		return corev1.GetProblemLibOK{}, err
	}
	return corev1.GetProblemLibOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemLib handles PUT /problems/{problemId}/lib/{name}
func (h *CoreServer) UpdateProblemLib(ctx context.Context, req corev1.UpdateProblemLibReq, params corev1.UpdateProblemLibParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, params.ProblemId, libDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Library file updated successfully")}, nil
}

// DeleteProblemLib handles DELETE /problems/{problemId}/lib/{name}
func (h *CoreServer) DeleteProblemLib(ctx context.Context, params corev1.DeleteProblemLibParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, libDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Library file deleted successfully")}, nil
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
func (h *CoreServer) ListProblemMediaFiles(ctx context.Context, params corev1.ListProblemMediaFilesParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, mediaDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemMediaFile handles POST /problems/{problemId}/media
func (h *CoreServer) CreateProblemMediaFile(ctx context.Context, req corev1.CreateProblemMediaFileReq, params corev1.CreateProblemMediaFileParams) (*corev1.MessageResponse, error) {
	var bodyReader io.Reader = req.Data
	if strings.HasSuffix(strings.ToLower(params.Name), ".svg") {
		if bodyBytes, err := io.ReadAll(req.Data); err == nil {
			sanitized := sanitizeSVG(bodyBytes)
			bodyReader = bytes.NewReader(sanitized)
		}
	}
	if err := h.createWorkshopCollectionFile(ctx, params.ProblemId, mediaDir, params.Name, bodyReader); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Media file created successfully")}, nil
}

// GetProblemMediaFile handles GET /problems/{problemId}/media/{name}
func (h *CoreServer) GetProblemMediaFile(ctx context.Context, params corev1.GetProblemMediaFileParams) (corev1.GetProblemMediaFileOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, mediaDir, params.Name)
	if err != nil {
		return corev1.GetProblemMediaFileOK{}, err
	}
	return corev1.GetProblemMediaFileOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemMediaFile handles PUT /problems/{problemId}/media/{name}
func (h *CoreServer) UpdateProblemMediaFile(ctx context.Context, req corev1.UpdateProblemMediaFileReq, params corev1.UpdateProblemMediaFileParams) (*corev1.MessageResponse, error) {
	var bodyReader io.Reader = req.Data
	if strings.HasSuffix(strings.ToLower(params.Name), ".svg") {
		if bodyBytes, err := io.ReadAll(req.Data); err == nil {
			sanitized := sanitizeSVG(bodyBytes)
			bodyReader = bytes.NewReader(sanitized)
		}
	}
	if err := h.updateWorkshopCollectionFile(ctx, params.ProblemId, mediaDir, params.Name, bodyReader); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Media file updated successfully")}, nil
}

// DeleteProblemMediaFile handles DELETE /problems/{problemId}/media/{name}
func (h *CoreServer) DeleteProblemMediaFile(ctx context.Context, params corev1.DeleteProblemMediaFileParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, mediaDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Media file deleted successfully")}, nil
}

// ListProblemWorkshopSubmissions handles GET /problems/{problemId}/submissions
func (h *CoreServer) ListProblemWorkshopSubmissions(ctx context.Context, params corev1.ListProblemWorkshopSubmissionsParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, solutionDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemWorkshopSubmission handles POST /problems/{problemId}/submissions
func (h *CoreServer) CreateProblemWorkshopSubmission(ctx context.Context, req corev1.CreateProblemWorkshopSubmissionReq, params corev1.CreateProblemWorkshopSubmissionParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.createWorkshopCollectionTextFile(ctx, params.ProblemId, solutionDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Author solution file created successfully")}, nil
}

// GetProblemWorkshopSubmission handles GET /problems/{problemId}/submissions/{name}
func (h *CoreServer) GetProblemWorkshopSubmission(ctx context.Context, params corev1.GetProblemWorkshopSubmissionParams) (corev1.GetProblemWorkshopSubmissionOK, error) {
	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, solutionDir, params.Name)
	if err != nil {
		return corev1.GetProblemWorkshopSubmissionOK{}, err
	}
	return corev1.GetProblemWorkshopSubmissionOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemWorkshopSubmission handles PUT /problems/{problemId}/submissions/{name}
func (h *CoreServer) UpdateProblemWorkshopSubmission(ctx context.Context, req corev1.UpdateProblemWorkshopSubmissionReq, params corev1.UpdateProblemWorkshopSubmissionParams) (*corev1.MessageResponse, error) {
	contentBytes, err := io.ReadAll(req.Data)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "failed to read body")
	}
	if err := h.updateWorkshopCollectionTextFile(ctx, params.ProblemId, solutionDir, params.Name, string(contentBytes)); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Author solution file updated successfully")}, nil
}

// DeleteProblemWorkshopSubmission handles DELETE /problems/{problemId}/submissions/{name}
func (h *CoreServer) DeleteProblemWorkshopSubmission(ctx context.Context, params corev1.DeleteProblemWorkshopSubmissionParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, solutionDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Author solution file deleted successfully")}, nil
}

// ListProblemTests handles GET /problems/{problemId}/tests
func (h *CoreServer) ListProblemTests(ctx context.Context, params corev1.ListProblemTestsParams) (*corev1.WorkshopFileListResponse, error) {
	resp, err := h.listWorkshopCollection(ctx, params.ProblemId, testDir)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateProblemTestFile handles POST /problems/{problemId}/tests
func (h *CoreServer) CreateProblemTestFile(ctx context.Context, req corev1.CreateProblemTestFileReq, params corev1.CreateProblemTestFileParams) (*corev1.MessageResponse, error) {
	if err := h.createWorkshopCollectionFile(ctx, params.ProblemId, testDir, params.Name, req.Data); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Test file created successfully")}, nil
}

// UpdateProblemTestsConfig handles PATCH /problems/{problemId}/tests/config
func (h *CoreServer) UpdateProblemTestsConfig(ctx context.Context, req corev1.UpdateProblemTestsConfigRequest, params corev1.UpdateProblemTestsConfigParams) (*corev1.MessageResponse, error) {
	if req == nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "request body is required")
	}
	if !h.workshopUC.IsInitialized(ctx, params.ProblemId) {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	bodyBytes, err := json.Marshal(req)
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

	if err := h.workshopUC.UpdateTestsConfig(ctx, params.ProblemId, &testsMeta); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to update tests config")
	}

	return &corev1.MessageResponse{Message: corev1.NewOptString("Tests config updated successfully")}, nil
}

// GetProblemTestFile handles GET /problems/{problemId}/tests/{name}
func (h *CoreServer) GetProblemTestFile(ctx context.Context, params corev1.GetProblemTestFileParams) (corev1.GetProblemTestFileOK, error) {
	if params.Name == "tests.json" {
		testsMeta, err := h.workshopUC.GetTestsConfig(ctx, params.ProblemId)
		if err != nil {
			return corev1.GetProblemTestFileOK{}, pkg.Wrap(pkg.ErrNotFound, err, "tests config not found")
		}
		content, err := json.MarshalIndent(testsMeta, "", "  ")
		if err != nil {
			return corev1.GetProblemTestFileOK{}, pkg.Wrap(pkg.ErrInternal, err, "failed to marshal tests config")
		}
		return corev1.GetProblemTestFileOK{Data: bytes.NewReader(content)}, nil
	}

	content, err := h.getWorkshopCollectionFile(ctx, params.ProblemId, testDir, params.Name)
	if err != nil {
		return corev1.GetProblemTestFileOK{}, err
	}
	return corev1.GetProblemTestFileOK{Data: bytes.NewReader(content)}, nil
}

// UpdateProblemTestFile handles PUT /problems/{problemId}/tests/{name}
func (h *CoreServer) UpdateProblemTestFile(ctx context.Context, req corev1.UpdateProblemTestFileReq, params corev1.UpdateProblemTestFileParams) (*corev1.MessageResponse, error) {
	if err := h.updateWorkshopCollectionFile(ctx, params.ProblemId, testDir, params.Name, req.Data); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Test file updated successfully")}, nil
}

// DeleteProblemTestFile handles DELETE /problems/{problemId}/tests/{name}
func (h *CoreServer) DeleteProblemTestFile(ctx context.Context, params corev1.DeleteProblemTestFileParams) (*corev1.MessageResponse, error) {
	if err := h.deleteWorkshopCollectionFile(ctx, params.ProblemId, testDir, params.Name); err != nil {
		return nil, err
	}
	return &corev1.MessageResponse{Message: corev1.NewOptString("Test file deleted successfully")}, nil
}

// CompileProblemComponent handles POST /problems/{problemId}/workshop/components/{componentType}/compile
func (h *CoreServer) CompileProblemComponent(ctx context.Context, params corev1.CompileProblemComponentParams) (*corev1.CompileResult, error) {
	user := middleware.GetUser(ctx)

	compileReq := models.CompileComponentRequest{
		ProblemID:     params.ProblemId,
		ComponentType: string(params.ComponentType),
		UserID:        user.Id,
	}

	result, err := h.workshopUC.CompileProblemComponent(ctx, compileReq)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to compile component")
	}

	return &corev1.CompileResult{
		CompileError: stringToOptString(result.CompileError),
		CompileLog:   stringToOptString(result.CompileLog),
		FileID:       stringToOptString(result.FileID),
		SHA256:       stringToOptString(result.SHA256),
		Success:      corev1.NewOptBool(result.Success),
	}, nil
}

// GenerateTests handles POST /problems/{problemId}/workshop/tests/generate
func (h *CoreServer) GenerateTests(ctx context.Context, req *corev1.GenerateTestsReq, params corev1.GenerateTestsParams) (*corev1.GenerateTestsOK, error) {
	user := middleware.GetUser(ctx)

	testNumbers := make([]int, len(req.TestNumbers))
	copy(testNumbers, req.TestNumbers)

	var arguments [][]string
	if req.Arguments != nil {
		arguments = make([][]string, len(req.Arguments))
		copy(arguments, req.Arguments)
	}

	generateReq := models.GenerateTestsRequest{
		ProblemID:     params.ProblemId,
		GeneratorName: req.GeneratorName,
		TestNumbers:   testNumbers,
		Arguments:     arguments,
		UserID:        user.Id,
	}

	if err := h.workshopUC.GenerateTests(ctx, generateReq); err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to generate tests")
	}

	return &corev1.GenerateTestsOK{Message: corev1.NewOptString("Tests generated successfully")}, nil
}

// ValidateAllTests handles POST /problems/{problemId}/workshop/tests/validate
func (h *CoreServer) ValidateAllTests(ctx context.Context, params corev1.ValidateAllTestsParams) (*corev1.ValidationReport, error) {
	user := middleware.GetUser(ctx)

	report, err := h.workshopUC.ValidateAllTests(ctx, params.ProblemId, user.Id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to validate tests")
	}

	results := make([]corev1.TestValidationResult, len(report.Results))
	for i, r := range report.Results {
		results[i] = corev1.TestValidationResult{
			Error:      stringToOptString(r.Error),
			Message:    stringToOptString(r.Message),
			TestNumber: corev1.NewOptInt(r.TestNumber),
			Valid:      corev1.NewOptBool(r.Valid),
		}
	}

	return &corev1.ValidationReport{
		InvalidTests: corev1.NewOptInt(report.InvalidTests),
		Results:      results,
		TotalTests:   corev1.NewOptInt(report.TotalTests),
		ValidTests:   corev1.NewOptInt(report.ValidTests),
	}, nil
}

// TestSolution handles POST /problems/{problemId}/workshop/solutions/test
func (h *CoreServer) TestSolution(ctx context.Context, req *corev1.TestSolutionReq, params corev1.TestSolutionParams) (*corev1.TestReport, error) {
	user := middleware.GetUser(ctx)

	var testNumbers []int
	if req.TestNumbers != nil {
		testNumbers = make([]int, len(req.TestNumbers))
		copy(testNumbers, req.TestNumbers)
	}

	testReq := models.TestSolutionRequest{
		ProblemID:    params.ProblemId,
		SolutionPath: req.SolutionPath,
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
			Memory:     corev1.NewOptInt64(r.Memory),
			Message:    stringToOptString(r.Message),
			TestNumber: corev1.NewOptInt(r.TestNumber),
			Time:       corev1.NewOptInt64(r.Time),
			Verdict:    stringToOptString(r.Verdict),
		}
	}

	return &corev1.TestReport{
		FailedTests: corev1.NewOptInt(report.FailedTests),
		PassedTests: corev1.NewOptInt(report.PassedTests),
		Results:     results,
		TotalTests:  corev1.NewOptInt(report.TotalTests),
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
	return corev1.WorkshopFileListResponse{Files: contractFiles}, nil
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

func (h *CoreServer) toContractLimits(manifest *models.ProblemManifest) *corev1.ProblemLimits {
	var maxScoreOpt corev1.OptNilInt
	if manifest.MaxScore != nil {
		maxScoreOpt = corev1.NewOptNilInt(*manifest.MaxScore)
	}
	return &corev1.ProblemLimits{
		MaxScore:      maxScoreOpt,
		MemoryLimitMB: manifest.MemoryLimitMb,
		ProblemType:   manifest.ProblemType,
		TimeLimitMs:   manifest.TimeLimitMs,
	}
}

func (h *CoreServer) toContractStatementForLang(stmt models.Statement, languages []string, currentLang string) *corev1.ProblemStatement {
	return &corev1.ProblemStatement{
		InputFormat:  stmt.InputFormat,
		Interaction:  stringToOptString(stmt.Interaction),
		Legend:       stmt.Legend,
		Notes:        stringToOptString(stmt.Notes),
		OutputFormat: stmt.OutputFormat,
		Scoring:      stringToOptString(stmt.Scoring),
		Title:        stmt.Title,
		Languages:    languages,
		CurrentLang:  stringToOptString(currentLang),
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
			Path:        corev1.NewOptString(f.Path),
			IsDirectory: corev1.NewOptBool(f.IsDirectory),
			Size:        corev1.NewOptInt64(f.Size),
			IsMain:      corev1.NewOptBool(isMain),
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
