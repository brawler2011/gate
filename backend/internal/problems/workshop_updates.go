package problems

import (
	"context"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

type LimitsPatch struct {
	ProblemType   *string
	TimeLimitMs   *int
	MemoryLimitMb *int
	MaxScore      *int
	MaxScoreSet   bool
}
type StatementPatch struct {
	Title, Legend, InputFormat, OutputFormat, Notes, Interaction, Scoring *string
}

func validateManifest(m *problemsmodel.ProblemManifest) error {
	if m.TimeLimitMs <= 0 || m.MemoryLimitMb <= 0 {
		return pkg.Wrap(pkg.ErrBadInput, nil, "limits must be positive")
	}
	if strings.TrimSpace(m.Statement.Title) == "" {
		return pkg.Wrap(pkg.ErrBadInput, nil, "title is required")
	}
	return nil
}

func (uc *WorkshopUseCase) UpdateLimits(ctx context.Context, id uuid.UUID, patch LimitsPatch) (*problemsmodel.ProblemManifest, error) {
	if err := uc.authorize(ctx, id, accessmodel.ActionEditProblem); err != nil {
		return nil, err
	}
	return withWorkspaceValue(ctx, uc.workspaceStorage, id, func(ctx context.Context) (*problemsmodel.ProblemManifest, error) {
		manifest, err := uc.loadManifest(ctx, id)
		if err != nil {
			return nil, err
		}
		if patch.ProblemType != nil {
			manifest.ProblemType = *patch.ProblemType
		}
		if patch.TimeLimitMs != nil {
			manifest.TimeLimitMs = *patch.TimeLimitMs
		}
		if patch.MemoryLimitMb != nil {
			manifest.MemoryLimitMb = *patch.MemoryLimitMb
		}
		if patch.MaxScoreSet {
			manifest.MaxScore = patch.MaxScore
		}
		if manifest.ProblemType != "scoring" {
			manifest.MaxScore = nil
		}
		if err := validateManifest(manifest); err != nil {
			return nil, err
		}
		if err := uc.saveManifest(ctx, id, manifest); err != nil {
			return nil, err
		}
		return manifest, nil
	})
}

func (uc *WorkshopUseCase) UpdateStatement(ctx context.Context, id uuid.UUID, language string, patch StatementPatch) (*problemsmodel.LocalizedStatement, error) {
	if err := uc.authorize(ctx, id, accessmodel.ActionEditProblem); err != nil {
		return nil, err
	}
	return withWorkspaceValue(ctx, uc.workspaceStorage, id, func(ctx context.Context) (*problemsmodel.LocalizedStatement, error) {
		current, err := uc.getStatementFiles(ctx, id, language)
		if err != nil {
			return nil, err
		}
		statement := current.Statement
		for _, field := range []struct {
			input  *string
			target *string
		}{
			{patch.Title, &statement.Title}, {patch.Legend, &statement.Legend}, {patch.InputFormat, &statement.InputFormat}, {patch.OutputFormat, &statement.OutputFormat},
			{patch.Notes, &statement.Notes}, {patch.Interaction, &statement.Interaction}, {patch.Scoring, &statement.Scoring},
		} {
			if field.input != nil {
				*field.target = *field.input
			}
		}
		if language == "en" {
			manifest, err := uc.loadManifest(ctx, id)
			if err != nil {
				return nil, err
			}
			manifest.Statement = statement
			if err := validateManifest(manifest); err != nil {
				return nil, err
			}
		}
		data := []byte(RenderStatementMarkdown(statement))
		if len(data) > 2*1024*1024 {
			return nil, pkg.Wrap(pkg.ErrPayloadTooLarge, nil, "statement size exceeds maximum allowed limit of 2MB")
		}
		if err := uc.updateProblemFileFiles(ctx, problemsmodel.UpdateFileRequest{ProblemID: id, Path: "statements/" + language + ".md", Content: data}); err != nil {
			return nil, err
		}
		localized, err := uc.getStatementFiles(ctx, id, language)
		if err != nil {
			return nil, err
		}
		localized.Statement = statement
		return localized, nil
	})
}
