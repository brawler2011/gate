package core

import (
	"context"
	"path/filepath"
	"strings"

	corev1 "github.com/brawler2011/contracts/core/v1"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

func (h *CoreServer) GetLanguages(ctx context.Context, request corev1.GetLanguagesRequestObject) (corev1.GetLanguagesResponseObject, error) {
	cfg, err := sandbox.LoadConfig("languages.yaml")
	if err != nil {
		cfg, _ = sandbox.LoadConfig("../../languages.yaml")
	}

	var langItems []corev1.SupportedLanguage
	if cfg != nil && cfg.Langs != nil && len(cfg.Langs) > 0 {
		for key, l := range cfg.Langs {
			ext := strings.TrimPrefix(filepath.Ext(l.CodeFile), ".")
			if ext == "" || key == "cpp" {
				ext = key
			}
			langType := l.Type
			langItems = append(langItems, corev1.SupportedLanguage{
				Name:      key,
				Extension: ext,
				Type:      &langType,
			})
		}
	} else {
		langItems = []corev1.SupportedLanguage{
			{Name: "cpp", Extension: "cpp"},
			{Name: "python", Extension: "py"},
			{Name: "go", Extension: "go"},
			{Name: "java", Extension: "java"},
		}
	}

	return corev1.GetLanguages200JSONResponse{
		Languages: langItems,
	}, nil
}
