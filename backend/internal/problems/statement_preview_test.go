package problems

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
)

// A preview carries its statement in the request, so its language is checked before typesetting loads the language file.
func TestPreviewRejectsLanguageWithoutFile(t *testing.T) {
	_, err := (&DraftUseCase{}).PreviewStatementPDF(context.Background(), uuid.New(), model.DraftStatement{Language: "fr", Legend: "Text."})
	require.ErrorIs(t, err, pkg.ErrInvalid)
}
