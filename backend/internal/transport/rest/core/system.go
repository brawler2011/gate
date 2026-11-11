package core

import (
	"context"
	"path/filepath"
	"strings"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
)

func (h *CoreServer) GetLanguages(ctx context.Context) (*corev1.SupportedLanguagesResponse, error) {
	langItems := make([]corev1.SupportedLanguage, 0, len(h.languages))
	for key, l := range h.languages {
		ext := strings.TrimPrefix(filepath.Ext(l.CodeFile), ".")
		if ext == "" || key == "cpp" {
			ext = key
		}
		langItems = append(langItems, corev1.SupportedLanguage{
			Name:      key,
			Extension: ext,
			Type:      corev1.NewOptString(l.Type),
		})
	}

	return &corev1.SupportedLanguagesResponse{
		Languages: langItems,
	}, nil
}
