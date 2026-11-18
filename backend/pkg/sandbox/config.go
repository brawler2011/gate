package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrUnknownLanguage means a language configuration identifier is not in the
// catalog the sandbox runs with.
var ErrUnknownLanguage = errors.New("unknown language configuration")

// Language is one configuration of the catalog: a concrete compiler or
// interpreter with its options, selected by its identifier.
type Language struct {
	// Name identifies the compiler or interpreter variant and version for people.
	Name string `toml:"name" json:"name"`
	// Extensions are the source file extensions, without the dot, this
	// configuration accepts; the first is the one a new source file gets.
	Extensions []string `toml:"extensions" json:"extensions"`
	// Syntax is the Prism grammar the frontend highlights sources with; ""
	// shows them as plain text. It is not part of the compiler identity.
	Syntax      string `toml:"syntax" json:"-"`
	Type        string `toml:"type" json:"type"`
	Compile     string `toml:"compile" json:"compile"`
	CodeFile    string `toml:"code_file" json:"code_file"`
	CompileFile string `toml:"compile_file" json:"compile_file"`
	Execute     string `toml:"execute" json:"execute"`
}

// Config is the language catalog: configurations by identifier, and the
// identifiers in catalog order.
type Config struct {
	Langs map[string]Language
	// Order lists the identifiers as the catalog file does. People see the
	// configurations in this order, and a file extension suggests the first
	// configuration that accepts it.
	Order []string
}

// LoadConfig reads the language catalog from a file (judge/langs.toml).
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox languages config: %w", err)
	}
	return ParseConfig(data)
}

// ParseConfig parses the language catalog.
// Each configuration's "package" key is consumed by the judge image build and ignored here.
func ParseConfig(data []byte) (*Config, error) {
	var langs map[string]Language
	meta, err := toml.Decode(string(data), &langs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sandbox languages config: %w", err)
	}
	for _, key := range meta.Undecoded() {
		if len(key) == 2 && key[1] == "package" {
			continue
		}
		return nil, fmt.Errorf("unknown key %q in sandbox languages config", key.String())
	}
	if len(langs) == 0 {
		return nil, fmt.Errorf("sandbox languages config defines no languages")
	}
	cfg := &Config{Langs: langs}
	for _, key := range meta.Keys() {
		if len(key) == 1 {
			cfg.Order = append(cfg.Order, key[0])
		}
	}
	for _, id := range cfg.Order {
		lang := langs[id]
		if lang.Name == "" || len(lang.Extensions) == 0 || lang.CodeFile == "" || lang.Execute == "" {
			return nil, fmt.Errorf("language %q needs name, extensions, code_file and execute", id)
		}
		for _, ext := range lang.Extensions {
			if ext == "" || ext != strings.ToLower(ext) || strings.ContainsAny(ext, "./") {
				return nil, fmt.Errorf("language %q has invalid extension %q", id, ext)
			}
		}
	}
	return cfg, nil
}

// ForPath returns the configuration a source file's extension suggests: the
// first in catalog order that accepts it, or "" when none does.
func (c *Config) ForPath(file string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(file), "."))
	for _, id := range c.Order {
		if slices.Contains(c.Langs[id].Extensions, ext) {
			return id
		}
	}
	return ""
}
