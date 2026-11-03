package problems

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type PackageTemplates interface {
	GetReadyPackage(context.Context, uuid.UUID) (problemsmodel.ProblemPackage, error)
	DownloadPackage(context.Context, uuid.UUID, string) (io.ReadCloser, error)
}

func (uc *ProblemsUseCase) CreateProblem(ctx context.Context, input *problemsmodel.CreateProblemInput) (uuid.UUID, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return uuid.Nil, pkg.ErrUnauthenticated
	}
	if input.Title == "" {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "empty title")
	}
	templateID := strings.TrimSpace(input.TemplateID)
	if templateID == "" {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "template_id is required")
	}
	orgID := input.OrganizationID
	if orgID == uuid.Nil {
		organizations, err := uc.orgsRepo.GetUserOrganizations(ctx, user.Id)
		if err != nil {
			return uuid.Nil, err
		}
		if len(organizations) == 0 {
			return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "user has no organizations")
		}
		orgID = organizations[0].ID
	}
	reader, err := uc.templatePackage(ctx, orgID, templateID)
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	importer := &ProblemImportUseCase{problemsRepo: uc.repo, workspaceStorage: uc.workspace}
	workshop := NewWorkshopUseCase(uc.workspace, nil)
	version, prepareErr := uc.workspace.preparePackage(ctx, id, func(ctx context.Context) error {
		manifest, err := importer.importProblemPackageFiles(ctx, reader, id)
		if err != nil {
			return err
		}
		manifest.Statement.Title = input.Title
		return workshop.saveManifest(ctx, id, manifest)
	})
	if err := errors.Join(prepareErr, reader.Close()); err != nil {
		return uuid.Nil, err
	}
	params := &problemsmodel.CreateProblemParams{ID: id, OrganizationID: orgID, OwnerID: &user.Id, Title: input.Title, ShortName: "problem-" + id.String()[:8], Visibility: problemsmodel.ProblemVisibilityPrivate}
	err = uc.transactions.Within(ctx, func(tx ProblemTxStore) error {
		if err := tx.CreateProblem(ctx, params); err != nil {
			return err
		}
		if err := tx.CreateProblemMember(ctx, &problemsmodel.CreateProblemMemberParams{ProblemID: id, UserID: user.Id, Role: problemsmodel.ProblemRoleOwner}); err != nil {
			return err
		}
		return tx.CommitWorkspace(ctx, id, "", version.hash, version.metadata)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (uc *ProblemsUseCase) templatePackage(ctx context.Context, orgID uuid.UUID, templateID string) (io.ReadCloser, error) {
	if strings.HasPrefix(templateID, "builtin:") {
		data, err := templates.GetBuiltinTemplateZip(templateID)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "неизвестный встроенный шаблон")
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	id, err := uuid.Parse(templateID)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid template_id format")
	}
	template, err := uc.repo.GetProblemById(ctx, id)
	if err != nil {
		return nil, err
	}
	if !template.IsTemplate {
		return nil, pkg.Wrap(pkg.ErrBadInput, nil, "выбранная задача не является шаблоном")
	}
	if template.OrganizationID != orgID {
		return nil, pkg.Wrap(pkg.NoPermission, nil, "шаблон должен принадлежать той же организации")
	}
	ready, err := uc.templates.GetReadyPackage(ctx, id)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "не удалось найти готовый пакет шаблона")
	}
	return uc.templates.DownloadPackage(ctx, id, ready.PackageHash)
}
