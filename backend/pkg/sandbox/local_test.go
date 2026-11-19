package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedDirFile(t *testing.T) {
	d := SharedDir{Local: "data/work", Sandbox: "/work"}
	f, err := d.File("data/work/cache/package-1/tree/tests/01")
	require.NoError(t, err)
	r, err := f.request()
	require.NoError(t, err)
	assert.Equal(t, "/work/cache/package-1/tree/tests/01", *r.Src)

	for _, path := range []string{"data/work/../secret", "data/other", "/etc/passwd"} {
		_, err := d.File(path)
		assert.Error(t, err, path)
	}
}
