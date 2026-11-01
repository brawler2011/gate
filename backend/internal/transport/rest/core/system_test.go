package core

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLanguagesLoadsRootConfig(t *testing.T) {
	root, err := filepath.Abs("../../../../..")
	require.NoError(t, err)

	for _, dir := range []string{".", root} {
		t.Run(dir, func(t *testing.T) {
			t.Chdir(dir)
			server := &CoreServer{}
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
		})
	}
}
