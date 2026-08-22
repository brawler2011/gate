package templates

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuiltinTemplates(t *testing.T) {
	list := ListBuiltinTemplates()
	require.Len(t, list, 3)

	for _, tmpl := range list {
		t.Run(tmpl.ID, func(t *testing.T) {
			zipBytes, err := GetBuiltinTemplateZip(tmpl.ID)
			require.NoError(t, err)
			require.NotEmpty(t, zipBytes)

			zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
			require.NoError(t, err)
			require.NotEmpty(t, zr.File)

			var hasProblemYaml bool
			for _, f := range zr.File {
				if f.Name == "problem.yaml" {
					hasProblemYaml = true
					break
				}
			}
			require.True(t, hasProblemYaml, "template must have problem.yaml")
		})
	}
}
