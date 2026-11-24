package gfmt

import (
	"path/filepath"
	"strings"
)

// FormatVersion is written by Gate; author packages may omit it.
const FormatVersion = "2.0"

func TestOutputNames(name string) []string {
	var candidates []string
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	if ext == ".out" || ext == ".ans" || ext == ".a" || ext == ".output" {
		candidates = append(candidates, name)
	}
	if stem != "" {
		candidates = append(candidates, stem+".out", stem+".ans", stem+".a", stem+".output")
	}
	candidates = append(candidates, name+".out", name+".ans", name+".a", name+".output")

	return candidates
}

func TestInputNames(name string) []string {
	candidates := []string{name}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem != "" && stem != name {
		candidates = append(candidates, stem+".in", stem+".input")
	} else {
		candidates = append(candidates, name+".in", name+".input")
	}

	return candidates
}
