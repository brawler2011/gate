// Package syslib is the catalog of system library files: sources Gate
// vendors from upstream projects and provides to problem programs, so that a
// draft can include one at its current version (auto-update) instead of
// carrying a copy. Each project lives in its own folder with its license,
// the upstream commit (COMMIT) and the Git blob IDs of every upstream version
// of its file (upstream.txt). Update testlib with
// `task testlib:update -- <commit>`.
package syslib

import (
	"crypto/sha1" //nolint:gosec // Git blob IDs are SHA-1; they identify upstream texts, not secrets.
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strings"
)

//go:embed testlib
var vendored embed.FS

// File is a system library file.
type File struct {
	Name string
	// Extensions are the source extensions of programs that include it.
	Extensions []string
	Version    string
	// Commit is the upstream commit the text comes from.
	Commit   string
	Content  string
	upstream map[string]bool
}

var catalog = []File{
	load("testlib", "testlib.h", []string{"cpp"}, `(?m)^#define VERSION "([^"]+)"`),
}

func load(dir, name string, extensions []string, version string) File {
	read := func(file string) string {
		data, err := vendored.ReadFile(path.Join(dir, file))
		if err != nil {
			panic(err)
		}
		return string(data)
	}
	f := File{Name: name, Extensions: extensions, Commit: strings.TrimSpace(read("COMMIT")), Content: read(name), upstream: map[string]bool{}}
	if m := regexp.MustCompile(version).FindStringSubmatch(f.Content); m != nil {
		f.Version = m[1]
	}
	for id := range strings.FieldsSeq(read("upstream.txt")) {
		f.upstream[id] = true
	}
	return f
}

// Files returns the catalog.
func Files() []File { return catalog }

// Lookup returns the system library file named name.
func Lookup(name string) (*File, bool) {
	for i := range catalog {
		if catalog[i].Name == name {
			return &catalog[i], true
		}
	}
	return nil, false
}

// IsUpstream reports whether text is the file as some upstream commit has
// it, also when its line endings were converted to CRLF.
func (f *File) IsUpstream(text string) bool {
	return f.upstream[blobID(text)] || f.upstream[blobID(strings.ReplaceAll(text, "\r\n", "\n"))]
}

// blobID is the Git object ID of text stored as a blob.
func blobID(text string) string {
	h := sha1.New() //nolint:gosec // see the import
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(text))
	_, _ = h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}
