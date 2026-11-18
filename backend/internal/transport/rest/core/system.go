package core

import (
	"context"

	corev1 "github.com/brawler2011/gate/backend/contracts/core/v1"
)

func (h *CoreServer) GetLanguages(ctx context.Context) (*corev1.SupportedLanguagesResponse, error) {
	items := make([]corev1.SupportedLanguage, 0, len(h.languages.Order))
	for _, id := range h.languages.Order {
		l := h.languages.Langs[id]
		items = append(items, corev1.SupportedLanguage{ID: id, Name: l.Name, Extensions: l.Extensions})
	}
	return &corev1.SupportedLanguagesResponse{Languages: items}, nil
}
