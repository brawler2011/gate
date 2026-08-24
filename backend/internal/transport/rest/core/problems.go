package core

import (
	"bytes"
	"context"
	"io"
	"strings"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/internal/domain/models"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/internal/transport/middleware"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (h *CoreServer) ListProblems(ctx context.Context, params corev1.ListProblemsParams) (*corev1.ListProblemsResponseModel, error) {
	searchStr := params.Search.Or("")

	filter := &models.ProblemsFilter{
		Page:     params.Page,
		PageSize: params.PageSize,
		Search:   searchStr,
	}

	if params.Owner.IsSet() && params.Owner.Value {
		user := middleware.GetUser(ctx)
		filter.OwnerID = &user.Id
	}

	if params.OrganizationID.IsSet() {
		orgID := params.OrganizationID.Value
		filter.OrganizationID = &orgID

		user := middleware.GetUser(ctx)
		isMember := false
		if user.Role == models.UserRoleAdmin {
			isMember = true
		} else if user.Id != uuid.Nil {
			_, err := h.organizationsUC.ListMembers(ctx, orgID, user.Id)
			if err == nil {
				isMember = true
			}
		}
		if !isMember {
			filter.Visibility = "public"
		}
	}

	if params.IsTemplate.IsSet() {
		isTmpl := params.IsTemplate.Value
		filter.IsTemplate = &isTmpl
	}

	problemsList, err := h.problemsUC.ListProblems(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := corev1.ListProblemsResponseModel{
		Problems:   make([]corev1.ProblemsListItemModel, len(problemsList.Problems)),
		Pagination: PaginationDTO(problemsList.Pagination),
	}

	for i, problem := range problemsList.Problems {
		resp.Problems[i] = ProblemsListItemDTO(problem)
	}
	return &resp, nil
}

func (h *CoreServer) ListProblemTemplates(ctx context.Context, params corev1.ListProblemTemplatesParams) ([]corev1.ProblemTemplateModel, error) {
	user := middleware.GetUser(ctx)

	var orgID uuid.UUID
	if params.OrganizationID.IsSet() {
		orgID = params.OrganizationID.Value
	} else if user.Id != uuid.Nil {
		orgs, err := h.organizationsUC.GetUserOrganizations(ctx, user.Id)
		if err == nil && len(orgs) > 0 {
			orgID = orgs[0].ID
		}
	}

	result := make([]corev1.ProblemTemplateModel, 0)

	// 1. Builtin templates
	builtinList := templates.ListBuiltinTemplates()
	for _, b := range builtinList {
		result = append(result, corev1.ProblemTemplateModel{
			ID:          b.ID,
			Title:       b.Title,
			Description: b.Description,
			ProblemType: b.ProblemType,
			IsBuiltin:   true,
		})
	}

	// 2. Organization templates
	if orgID != uuid.Nil {
		isTemplate := true
		filter := &models.ProblemsFilter{
			Page:           1,
			PageSize:       100,
			OrganizationID: &orgID,
			IsTemplate:     &isTemplate,
		}
		orgProblems, err := h.problemsUC.ListProblems(ctx, filter)
		if err == nil && orgProblems != nil {
			for _, p := range orgProblems.Problems {
				probType := "pass-fail"
				manifest, err := h.workshopUC.GetManifest(ctx, p.ID)
				if err == nil && manifest != nil && manifest.ProblemType != "" {
					probType = manifest.ProblemType
				}

				desc := "Пользовательский шаблон организации"
				if manifest != nil && manifest.Statement.Legend != "" {
					desc = manifest.Statement.Legend
				}

				result = append(result, corev1.ProblemTemplateModel{
					ID:          p.ID.String(),
					Title:       p.Title,
					Description: desc,
					ProblemType: probType,
					IsBuiltin:   false,
				})
			}
		}
	}

	return result, nil
}

func (h *CoreServer) CreateProblem(ctx context.Context, params corev1.CreateProblemParams) (*corev1.CreationResponseModel, error) {
	user := middleware.GetUser(ctx)

	if params.Title == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}

	templateIDStr := strings.TrimSpace(params.TemplateID)
	if templateIDStr == "" {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "template_id is required")
	}

	var orgID uuid.UUID
	if params.OrganizationID.IsSet() {
		orgID = params.OrganizationID.Value
	} else {
		orgs, err := h.organizationsUC.GetUserOrganizations(ctx, user.Id)
		if err != nil {
			return nil, err
		}
		if len(orgs) == 0 {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "user has no organizations")
		}
		orgID = orgs[0].ID
	}

	shortName := "problem-" + uuid.New().String()[:8]

	input := &models.CreateProblemInput{
		OrganizationID: orgID,
		OwnerID:        &user.Id,
		Title:          params.Title,
		ShortName:      shortName,
		Visibility:     models.ProblemVisibilityPrivate,
	}

	if strings.HasPrefix(templateIDStr, "builtin:") {
		zipBytes, err := templates.GetBuiltinTemplateZip(templateIDStr)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "неизвестный встроенный шаблон")
		}

		problemID, err := h.problemsUC.CreateProblem(ctx, input)
		if err != nil {
			return nil, err
		}

		_, err = h.importUC.ImportProblemPackage(ctx, bytes.NewReader(zipBytes), int64(len(zipBytes)), problemID)
		if err != nil {
			_ = h.problemsUC.DeleteProblem(ctx, problemID)
			return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to import builtin template package")
		}

		manifest, err := h.workshopUC.GetManifest(ctx, problemID)
		if err == nil {
			manifest.Statement.Title = input.Title
			_ = h.workshopUC.SaveManifest(ctx, problemID, manifest)
		}

		return &corev1.CreationResponseModel{ID: problemID}, nil
	}

	templateUUID, err := uuid.Parse(templateIDStr)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid template_id format")
	}

	templateProblem, err := h.problemsUC.GetProblemById(ctx, templateUUID)
	if err != nil {
		return nil, err
	}
	if !templateProblem.IsTemplate {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "выбранная задача не является шаблоном")
	}
	if templateProblem.OrganizationID != orgID {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "шаблон должен принадлежать той же организации")
	}

	readyPkg, err := h.publishUC.GetReadyPackage(ctx, templateUUID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "не удалось найти готовый пакет шаблона")
	}

	problemID, err := h.problemsUC.CreateProblem(ctx, input)
	if err != nil {
		return nil, err
	}

	zipReader, err := h.publishUC.DownloadPackage(ctx, templateUUID, readyPkg.PackageHash)
	if err != nil {
		_ = h.problemsUC.DeleteProblem(ctx, problemID)
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to download template package")
	}
	defer zipReader.Close()

	fileBytes, err := io.ReadAll(zipReader)
	if err != nil {
		_ = h.problemsUC.DeleteProblem(ctx, problemID)
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to read template package")
	}

	_, err = h.importUC.ImportProblemPackage(ctx, bytes.NewReader(fileBytes), int64(len(fileBytes)), problemID)
	if err != nil {
		_ = h.problemsUC.DeleteProblem(ctx, problemID)
		return nil, pkg.Wrap(pkg.ErrInternal, err, "failed to import template package")
	}

	manifest, err := h.workshopUC.GetManifest(ctx, problemID)
	if err == nil {
		manifest.Statement.Title = input.Title
		_ = h.workshopUC.SaveManifest(ctx, problemID, manifest)
	}

	return &corev1.CreationResponseModel{ID: problemID}, nil
}

func (h *CoreServer) DeleteProblem(ctx context.Context, params corev1.DeleteProblemParams) error {
	err := h.problemsUC.DeleteProblem(ctx, params.ID)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) GetProblem(ctx context.Context, params corev1.GetProblemParams) (*corev1.GetProblemResponseModel, error) {
	problem, err := h.problemsUC.GetProblemById(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	statement := h.loadProblemStatement(ctx, params.ID)
	samples := h.loadProblemSamples(ctx, params.ID)

	return &corev1.GetProblemResponseModel{Problem: *ProblemDTO(problem, statement, samples)}, nil
}

func (h *CoreServer) UpdateProblem(ctx context.Context, req *corev1.UpdateProblemRequestModel, params corev1.UpdateProblemParams) error {
	if req == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "missing request body")
	}

	// Build update params
	update := &models.ProblemUpdate{}

	// Handle title update
	if req.Title.IsSet() {
		update.Title = &req.Title.Value
	}

	// Handle visibility update
	if req.Visibility.IsSet() {
		update.Visibility = (*string)(&req.Visibility.Value)
	}

	// Handle is_template update
	if req.IsTemplate.IsSet() {
		if req.IsTemplate.Value {
			pkgs, err := h.publishUC.ListPackages(ctx, params.ID)
			if err != nil {
				return err
			}
			hasReady := false
			for _, p := range pkgs {
				if p.Status == "ready" {
					hasReady = true
					break
				}
			}
			if !hasReady {
				return pkg.Wrap(pkg.ErrBadInput, nil, "для перевода задачи в шаблон необходим хотя бы один успешно собранный пакет")
			}
		}
		update.IsTemplate = &req.IsTemplate.Value
	}

	// Note: Other fields (Legend, InputFormat, etc.) are now stored in git repos
	// and managed through the workshop/publish workflow

	err := h.problemsUC.UpdateProblem(ctx, params.ID, update)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListProblemTeams(ctx context.Context, params corev1.ListProblemTeamsParams) (*corev1.ListProblemTeamsResponseModel, error) {
	user := middleware.GetUser(ctx)

	teams, err := h.problemsUC.GetProblemTeams(ctx, params.ID, user.Id)
	if err != nil {
		return nil, err
	}

	return listProblemTeamsDTO(teams), nil
}

func (h *CoreServer) CreateProblemTeam(ctx context.Context, params corev1.CreateProblemTeamParams) error {
	user := middleware.GetUser(ctx)

	permission := models.ProblemPermissionRead
	if params.Permission.IsSet() && params.Permission.Value != "" {
		permission = models.ProblemPermission(params.Permission.Value)
	}

	err := h.problemsUC.AddProblemTeam(ctx, params.ID, params.TeamID, user.Id, permission)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateProblemTeam(ctx context.Context, params corev1.UpdateProblemTeamParams) error {
	user := middleware.GetUser(ctx)

	err := h.problemsUC.UpdateProblemTeamPermission(ctx, params.ID, params.TeamID, user.Id, models.ProblemPermission(params.Permission))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteProblemTeam(ctx context.Context, params corev1.DeleteProblemTeamParams) error {
	user := middleware.GetUser(ctx)

	err := h.problemsUC.RemoveProblemTeam(ctx, params.ID, params.TeamID, user.Id)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) ListProblemMembers(ctx context.Context, params corev1.ListProblemMembersParams) (*corev1.ListProblemMembersResponseModel, error) {
	user := middleware.GetUser(ctx)

	members, err := h.problemsUC.ListProblemMembers(ctx, params.ID, user.Id)
	if err != nil {
		return nil, err
	}

	total := safeInt32(len(members))
	page := params.Page
	return listProblemMembersDTO(members, page, total), nil
}

func (h *CoreServer) CreateProblemMember(ctx context.Context, params corev1.CreateProblemMemberParams) error {
	user := middleware.GetUser(ctx)

	role := models.ProblemRoleViewer
	if params.Role.IsSet() && params.Role.Value != "" {
		role = models.ProblemRole(params.Role.Value)
	}

	err := h.problemsUC.CreateProblemMember(ctx, params.ID, params.UserID, user.Id, role)
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) UpdateProblemMember(ctx context.Context, params corev1.UpdateProblemMemberParams) error {
	user := middleware.GetUser(ctx)

	err := h.problemsUC.UpdateProblemMemberRole(ctx, params.ID, params.UserID, user.Id, models.ProblemRole(params.Role))
	if err != nil {
		return err
	}

	return nil
}

func (h *CoreServer) DeleteProblemMember(ctx context.Context, params corev1.DeleteProblemMemberParams) error {
	user := middleware.GetUser(ctx)

	err := h.problemsUC.RemoveProblemMember(ctx, params.ID, params.UserID, user.Id)
	if err != nil {
		return err
	}

	return nil
}
