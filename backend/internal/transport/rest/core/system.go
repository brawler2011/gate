package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/brawler2011/gate/backend/pkg/syslib"
)

func (h *CoreServer) GetLanguages(ctx context.Context) (*corev1.SupportedLanguagesResponse, error) {
	items := make([]corev1.SupportedLanguage, 0, len(h.languages.Order))
	for _, id := range h.languages.Order {
		l := h.languages.Langs[id]
		items = append(items, corev1.SupportedLanguage{ID: id, Name: l.Name, Extensions: l.Extensions, Syntax: l.Syntax})
	}
	return &corev1.SupportedLanguagesResponse{Languages: items}, nil
}

func (h *CoreServer) GetStatementLanguages(ctx context.Context) (*corev1.StatementLanguagesResponse, error) {
	return &corev1.StatementLanguagesResponse{Languages: statement.Languages}, nil
}

func (h *CoreServer) GetSystemLibrary(ctx context.Context) (*corev1.SystemLibrary, error) {
	files := []corev1.SystemLibraryFile{}
	for _, f := range syslib.Files() {
		files = append(files, corev1.SystemLibraryFile{Name: f.Name, Extensions: f.Extensions, Version: f.Version, Commit: f.Commit, Content: f.Content})
	}
	checkers := []corev1.SystemChecker{}
	for _, c := range syslib.Checkers() {
		checkers = append(checkers, corev1.SystemChecker{Name: c.Name, Description: c.Description, Content: c.Content})
	}
	return &corev1.SystemLibrary{Files: files, Checkers: checkers}, nil
}
