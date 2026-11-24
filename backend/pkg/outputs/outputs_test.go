package outputs

import (
	"archive/zip"
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, text := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(text))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return b.Bytes()
}

func unpack(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	got := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		got[f.Name] = string(data)
	}
	return got
}

func TestPackNamesOutputsByTest(t *testing.T) {
	archive := zipOf(t, map[string]string{"out/03.out": "three", "__MACOSX/out/._03.out": "junk", "out/": "", ".DS_Store": "junk"})
	packed, sum, err := Pack([]File{
		{Name: "output_001.txt", Data: strings.NewReader("one")},
		{Name: "answers.ZIP", Data: bytes.NewReader(archive)},
		{Name: "test.2", Data: strings.NewReader("two")},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"1": "one", "2": "two", "3": "three"}, unpack(t, packed))
	again, sum2, err := Pack([]File{{Name: "2", Data: strings.NewReader("two")}, {Name: "3.out", Data: strings.NewReader("three")}, {Name: "1.out", Data: strings.NewReader("one")}})
	require.NoError(t, err)
	assert.Equal(t, sum, sum2, "the archive must not depend on how outputs were given")
	assert.Equal(t, packed, again)
}

func manyEntries(t *testing.T) []byte {
	t.Helper()
	files := map[string]string{}
	for i := 1; i <= MaxFiles+1; i++ {
		files[strconv.Itoa(i)] = ""
	}
	return zipOf(t, files)
}

func TestPackRejects(t *testing.T) {
	for name, files := range map[string][]File{
		"no number": {{Name: "answer.txt", Data: strings.NewReader("x")}},
		"zero":      {{Name: "0.out", Data: strings.NewReader("x")}},
		"duplicate": {{Name: "1.out", Data: strings.NewReader("x")}, {Name: "01.txt", Data: strings.NewReader("y")}},
		"not a zip": {{Name: "all.zip", Data: strings.NewReader("x")}},
		"nothing":   {},
		"too large": {{Name: "1.out", Data: bytes.NewReader(make([]byte, MaxFileBytes+1))}},
		"too many":  {{Name: "many.zip", Data: bytes.NewReader(manyEntries(t))}},
	} {
		_, _, err := Pack(files)
		var e *pkg.Error
		assert.ErrorAs(t, err, &e, name)
	}
}
