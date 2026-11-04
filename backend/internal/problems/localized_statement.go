package problems

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	accessmodel "github.com/brawler2011/gate/backend/internal/access/model"
	problemsmodel "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func (uc *WorkshopUseCase) GetStatement(ctx context.Context, problemID uuid.UUID, language string) (*problemsmodel.LocalizedStatement, error) {
	if err := uc.authorize(ctx, problemID, accessmodel.ActionViewProblem); err != nil {
		return nil, err
	}

	return withWorkspaceValue(ctx, uc.workspaceStorage, problemID, func(ctx context.Context) (*problemsmodel.LocalizedStatement, error) {
		return uc.getStatementFiles(ctx, problemID, language)
	})
}
func (uc *WorkshopUseCase) getStatementFiles(ctx context.Context, problemID uuid.UUID, language string) (*problemsmodel.LocalizedStatement, error) {
	initialized, err := uc.isInitialized(ctx, problemID)
	if err != nil {
		return nil, err
	}
	if !initialized {
		return nil, pkg.Wrap(pkg.ErrNotFound, nil, "workshop not initialized")
	}

	manifest, err := uc.loadManifest(ctx, problemID)
	if err != nil {
		return nil, err
	}

	lang := language

	// 1. Get list of available languages from statements/ folder
	var languages []string
	if files, err := uc.workspaceStorage.ListFiles(ctx, problemID, "statements"); err == nil {
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
	var stmt problemsmodel.Statement
	filePath := fmt.Sprintf("statements/%s.md", lang)
	fileData, err := uc.workspaceStorage.ReadFile(ctx, problemID, filePath)
	if err == nil {
		stmt = ParseStatementMarkdown(string(fileData))
		// If title tag is empty, fallback to the manifest title
		if lang == "en" || strings.TrimSpace(stmt.Title) == "" {
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

	return &problemsmodel.LocalizedStatement{Statement: stmt, Languages: languages, Language: lang}, nil
}
