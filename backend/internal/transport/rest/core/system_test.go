package core

import (
	"context"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/stretchr/testify/require"
)

func TestGetLanguagesReportsConfiguredLanguages(t *testing.T) {
	cfg, err := sandbox.LoadConfig("../../../../../judge/langs.toml")
	require.NoError(t, err)

	server := (&CoreServer{}).WithLanguages(cfg.Langs)
	response, err := server.GetLanguages(context.Background())
	require.NoError(t, err)
	require.Len(t, response.Languages, 4)
	for _, language := range response.Languages {
		languageType, ok := language.Type.Get()
		require.True(t, ok, "language %s must include its configured type", language.Name)
		if language.Name == "python" {
			require.Equal(t, "interpreter", languageType)
		} else {
			require.Equal(t, "compiler", languageType)
		}
	}
}
