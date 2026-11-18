package problems

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/internal/templates"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

// Idempotent replays return the stored result without uploading objects again.
func (uc *DraftUseCase) uploadImport(ctx context.Context, k model.MutationKey, plan *draftImport) (*model.MutationResult, []model.StoredObject, error) {
	replay, err := uc.Repo.BeginOperation(ctx, k)
	if err != nil || replay != nil {
		return replay, nil, err
	}
	objects := make([]model.StoredObject, len(plan.objects))
	ids := make([]uuid.UUID, len(plan.objects))
	for i, o := range plan.objects {
		f, err := os.Open(o.path)
		if err != nil {
			return nil, nil, fmt.Errorf("open: %w", err)
		}
		object, uploadErr := uc.storeObject(ctx, k.OrganizationID, o.spec, f)
		closeErr := f.Close()
		if err := errors.Join(uploadErr, closeErr); err != nil {
			return nil, nil, err
		}
		objects[i], ids[i] = object, object.ID
	}
	if err := plan.bind(ids); err != nil {
		return nil, nil, err
	}
	return nil, objects, nil
}
func (uc *DraftUseCase) Import(ctx context.Context, id uuid.UUID, expected int64, key string, reader io.Reader) (model.MutationResult, error) {
	p, err := uc.authorize(ctx, id, true)
	if err != nil {
		return model.MutationResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, uc.UploadTTL)
	defer cancel()
	plan, err := uc.parseImport(ctx, reader, id, expected)
	if err != nil {
		return model.MutationResult{}, err
	}
	defer plan.Close()
	current, err := uc.Repo.GetDraft(ctx, id)
	if err != nil {
		return model.MutationResult{}, err
	}
	plan.draft.ShortName = current.ShortName
	if strings.TrimSpace(plan.draft.Title) == "" {
		plan.draft.Title = current.Title
	}
	// The isolated import has only one root-level body file: the original archive spool.
	files, err := os.ReadDir(plan.root)
	if err != nil {
		return model.MutationResult{}, fmt.Errorf("readdir: %w", err)
	}
	var archiveDigest string
	for _, entry := range files {
		if !entry.IsDir() {
			object, err := uc.importObject(plan.root+"/"+entry.Name(), "asset", uuid.Nil)
			if err != nil {
				return model.MutationResult{}, err
			}
			archiveDigest = object.spec.SHA256
			break
		}
	}
	k, err := uc.key(p.OrganizationID, id, "import", key, struct {
		Revision int64
		Digest   string
	}{expected, archiveDigest})
	if err != nil {
		return model.MutationResult{}, err
	}
	replay, objects, err := uc.uploadImport(ctx, k, plan)
	if err != nil {
		return model.MutationResult{}, err
	}
	if replay != nil {
		return *replay, nil
	}
	return uc.Repo.FinalizeImport(ctx, k, id, expected, plan.draft, objects)
}
func (uc *DraftUseCase) CreateProblem(ctx context.Context, input *model.CreateProblemInput) (uuid.UUID, error) {
	user := accessmodel.UserFromContext(ctx)
	if user.IsGuest() {
		return uuid.Nil, pkg.ErrUnauthenticated
	}
	if input == nil || strings.TrimSpace(input.Title) == "" {
		return uuid.Nil, pkg.ErrBadInput
	}
	org := input.OrganizationID
	if org == uuid.Nil {
		organizations, err := uc.Organizations.GetUserOrganizations(ctx, user.Id)
		if err != nil {
			return uuid.Nil, err
		}
		if len(organizations) == 0 {
			return uuid.Nil, pkg.ErrBadInput
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
			return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, err, "unknown builtin template")
		}
		plan, err = uc.parseImport(ctx, bytes.NewReader(data), id, 1)
		if err != nil {
			return uuid.Nil, err
		}
		defer plan.Close()
	} else {
		tid, err := uuid.Parse(input.TemplateID)
		if err != nil {
			return uuid.Nil, pkg.ErrBadInput
		}
		template, err := uc.authorize(ctx, tid, false)
		if err != nil {
			return uuid.Nil, err
		}
		if !template.IsTemplate || template.OrganizationID != org {
			return uuid.Nil, pkg.NoPermission
		}
		ready, err := uc.Templates.GetReadyPackage(ctx, tid)
		if err != nil {
			return uuid.Nil, err
		}
		packageData, err := uc.PackageContent.Repo.GetPackageDescriptor(ctx, ready.ID)
		if err != nil {
			return uuid.Nil, err
		}
		plan = &draftImport{draft: packageData.Snapshot.Draft, objects: []importObject{}}
		plan.draft.ProblemID = id
		plan.draft.Revision = 1
	}
	plan.draft.Title = input.Title
	plan.draft.ShortName = "problem-" + id.String()[:8]
	k, err := uc.key(org, uuid.Nil, "create", input.IdempotencyKey, struct {
		Organization    uuid.UUID
		Title, Template string
	}{org, input.Title, input.TemplateID})
	if err != nil {
		return uuid.Nil, err
	}
	// Clones reuse same-organization objects; only imported bytes need uploading.
	var objects []model.StoredObject
	if len(plan.objects) > 0 {
		replay, uploaded, err := uc.uploadImport(ctx, k, plan)
		if err != nil {
			return uuid.Nil, err
		}
		if replay != nil {
			var result struct {
				ID uuid.UUID `json:"id"`
			}
			if err := json.Unmarshal(replay.Body, &result); err != nil {
				return uuid.Nil, fmt.Errorf("unmarshal: %w", err)
			}
			return result.ID, nil
		}
		objects = uploaded
	}
	result, err := uc.Repo.FinalizeCreate(ctx, k, user.Id, plan.draft, objects)
	if err != nil {
		return uuid.Nil, err
	}
	var created struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(result.Body, &created); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal: %w", err)
	}
	return created.ID, nil
}
