package problems

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (uc *DraftUseCase) uploadImport(ctx context.Context, org uuid.UUID, plan *draftImport) ([]model.StoredObject, error) {
	objects := make([]model.StoredObject, len(plan.objects))
	ids := make([]uuid.UUID, len(plan.objects))
	for i, o := range plan.objects {
		f, err := os.Open(o.path)
		if err != nil {
			return nil, fmt.Errorf("open: %w", err)
		}
		object, uploadErr := uc.storeObject(ctx, org, o.spec, f)
		closeErr := f.Close()
		if err := errors.Join(uploadErr, closeErr); err != nil {
			return nil, err
		}
		objects[i], ids[i] = object, object.ID
	}
	if err := plan.bind(ids); err != nil {
		return nil, err
	}
	return objects, nil
}
func (uc *DraftUseCase) Import(ctx context.Context, id uuid.UUID, expected int64, reader io.Reader) (model.Draft, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.Draft{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, uc.UploadTTL)
	defer cancel()
	plan, err := uc.parseImport(ctx, reader, id, expected)
	if err != nil {
		return model.Draft{}, err
	}
	defer plan.Close()
	current, err := uc.Repo.GetDraft(ctx, id)
	if err != nil {
		return model.Draft{}, err
	}
	plan.draft.ShortName = current.ShortName
	if strings.TrimSpace(plan.draft.Title) == "" {
		plan.draft.Title = current.Title
	}
	objects, err := uc.uploadImport(ctx, p.OrganizationID, plan)
	if err != nil {
		return model.Draft{}, err
	}
	if len(plan.converted) > 0 {
		slog.InfoContext(ctx, "import converted text files to UTF-8", "problem_id", id, "files", plan.converted)
	}
	return uc.Repo.FinalizeImport(ctx, id, expected, plan.draft, objects)
}
func (uc *DraftUseCase) CreateProblem(ctx context.Context, input *model.CreateProblemInput) (uuid.UUID, error) {
	user, err := accessmodel.RequireUser(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if input == nil || strings.TrimSpace(input.Title) == "" {
		return uuid.Nil, pkg.AtField("/title", pkg.Invalid(pkg.ErrRequired, "title is required", nil))
	}
	org := input.OrganizationID
	if org == uuid.Nil {
		organizations, err := uc.Organizations.GetUserOrganizations(ctx, user.Id)
		if err != nil {
			return uuid.Nil, err
		}
		if len(organizations) == 0 {
			return uuid.Nil, pkg.Wrap(pkg.ErrNotOrgMember, nil, "user has no organization")
		}
		org = organizations[0].ID
	}
	ctx, cancel := context.WithTimeout(ctx, uc.UploadTTL)
	defer cancel()
	id := uuid.New()
	var plan *draftImport
	if strings.HasPrefix(input.TemplateID, "builtin:") {
		data, err := templates.GetBuiltinTemplateZip(input.TemplateID)
		if err != nil {
			return uuid.Nil, pkg.Wrap(pkg.ErrNotFound, err, "unknown builtin template")
		}
		plan, err = uc.parseImport(ctx, bytes.NewReader(data), id, 1)
		if err != nil {
			return uuid.Nil, err
		}
		defer plan.Close()
	} else {
		tid, err := uuid.Parse(input.TemplateID)
		if err != nil {
			return uuid.Nil, pkg.Wrap(pkg.ErrNotFound, err, "unknown template")
		}
		template, err := uc.authorize(ctx, tid, false)
		if err != nil {
			return uuid.Nil, err
		}
		if template.OrganizationID != org {
			return uuid.Nil, pkg.ErrForbidden
		}
		ready, err := uc.Templates.GetReadyPackage(ctx, tid)
		if err != nil {
			return uuid.Nil, err
		}
		descriptor, err := uc.PackageContent.Repo.GetPackageDescriptor(ctx, ready.ID)
		if err != nil {
			return uuid.Nil, err
		}
		archive, _, err := uc.Store.DownloadFile(ctx, uc.Bucket, descriptor.Object.Key, nil)
		if err != nil {
			return uuid.Nil, err
		}
		plan, err = uc.parseImport(ctx, archive, id, 1)
		closeErr := archive.Close()
		if err != nil {
			return uuid.Nil, err
		}
		defer plan.Close()
		if closeErr != nil {
			return uuid.Nil, fmt.Errorf("close: %w", closeErr)
		}
	}
	plan.draft.Title = input.Title
	plan.draft.ShortName = "problem-" + id.String()[:8]
	objects, err := uc.uploadImport(ctx, org, plan)
	if err != nil {
		return uuid.Nil, err
	}
	if err := uc.Repo.FinalizeCreate(ctx, org, user.Id, plan.draft, objects); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
