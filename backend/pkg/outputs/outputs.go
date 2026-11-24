// Package outputs packs what an output-only submission gives into one archive of files named by test ordinal.
package outputs

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
)

const (
	MaxBytes     = 64 << 20
	MaxFileBytes = 16 << 20
)

type File struct {
	Name string
	Data io.Reader
}

// Key is where object storage keeps a packed archive.
func Key(sha256 string) string { return "submission-outputs/" + sha256 }

// Ordinal reads a test ordinal from the digits ending a file name, before or after its extension: 01.out, output_1.txt, test.3.
func Ordinal(name string) (int, bool) {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	for _, s := range []string{strings.TrimSuffix(base, path.Ext(base)), base} {
		digits := len(s) - len(strings.TrimRight(s, "0123456789"))
		if digits == 0 || digits > 9 {
			continue
		}
		n, err := strconv.Atoi(s[len(s)-digits:])
		return n, err == nil && n > 0
	}
	return 0, false
}

func invalid(code *pkg.Error, params map[string]any, format string, args ...any) error {
	return pkg.AtField("/files", pkg.Invalid(code, fmt.Sprintf(format, args...), params))
}

// Pack takes outputs one per file or inside .zip archives and returns the archive with its SHA-256.
func Pack(files []File) ([]byte, string, error) {
	byOrdinal := map[int][]byte{}
	total := 0
	add := func(name string, data []byte) error {
		n, ok := Ordinal(name)
		switch {
		case !ok:
			return invalid(pkg.ErrFormat, map[string]any{"value": name}, "%s does not end with a test number", name)
		case byOrdinal[n] != nil:
			return invalid(pkg.ErrDuplicate, map[string]any{"value": name}, "test %d has more than one output", n)
		case len(data) > MaxFileBytes:
			return invalid(pkg.ErrTooLarge, map[string]any{"max": MaxFileBytes}, "output %s is too large", name)
		case total+len(data) > MaxBytes:
			return invalid(pkg.ErrTooLarge, map[string]any{"max": MaxBytes}, "outputs are too large")
		}
		total += len(data)
		byOrdinal[n] = data
		return nil
	}
	for _, f := range files {
		data, err := io.ReadAll(io.LimitReader(f.Data, MaxBytes+1))
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", f.Name, err)
		}
		if !strings.EqualFold(path.Ext(f.Name), ".zip") {
			if err := add(f.Name, data); err != nil {
				return nil, "", err
			}
			continue
		}
		if len(data) > MaxBytes {
			return nil, "", invalid(pkg.ErrTooLarge, map[string]any{"max": MaxBytes}, "archive %s is too large", f.Name)
		}
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, "", invalid(pkg.ErrFormat, map[string]any{"value": f.Name}, "%s is not a zip archive", f.Name)
		}
		for _, entry := range z.File {
			// Finder adds __MACOSX/ and ._ metadata to the archives it makes.
			if entry.FileInfo().IsDir() || strings.HasPrefix(path.Base(entry.Name), ".") || strings.HasPrefix(entry.Name, "__MACOSX/") {
				continue
			}
			r, err := entry.Open()
			if err != nil {
				return nil, "", invalid(pkg.ErrFormat, map[string]any{"value": entry.Name}, "cannot read %s: %v", entry.Name, err)
			}
			data, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
			r.Close()
			if err != nil {
				return nil, "", invalid(pkg.ErrFormat, map[string]any{"value": entry.Name}, "cannot read %s: %v", entry.Name, err)
			}
			if err := add(entry.Name, data); err != nil {
				return nil, "", err
			}
		}
	}
	if len(byOrdinal) == 0 {
		return nil, "", invalid(pkg.ErrRequired, nil, "no outputs submitted")
	}
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	ordinals := make([]int, 0, len(byOrdinal))
	for n := range byOrdinal {
		ordinals = append(ordinals, n)
	}
	slices.Sort(ordinals)
	for _, n := range ordinals {
		entry, err := w.CreateHeader(&zip.FileHeader{Name: strconv.Itoa(n), Method: zip.Deflate})
		if err != nil {
			return nil, "", fmt.Errorf("pack output %d: %w", n, err)
		}
		if _, err := entry.Write(byOrdinal[n]); err != nil {
			return nil, "", fmt.Errorf("pack output %d: %w", n, err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("pack outputs: %w", err)
	}
	sum := sha256.Sum256(archive.Bytes())
	return archive.Bytes(), hex.EncodeToString(sum[:]), nil
}
