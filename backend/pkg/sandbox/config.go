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

var (
	ErrUnknownLanguage    = errors.New("unknown language configuration")
	ErrGradersUnsupported = errors.New("language does not compile with these graders")
)

// gradersArg in a compile command stands for the grader sources of a problem with graders, and for nothing otherwise.
const gradersArg = "${graders}"

type Language struct {
	Name string `toml:"name" json:"name"`
	// Extensions omit the dot; the first is used for new source files.
	Extensions []string `toml:"extensions" json:"extensions"`
	// Syntax selects a Prism grammar; empty means plain text and does not affect compiler identity.
	Syntax      string `toml:"syntax" json:"-"`
	Type        string `toml:"type" json:"type"`
	Compile     string `toml:"compile" json:"compile"`
	CodeFile    string `toml:"code_file" json:"code_file"`
	CompileFile string `toml:"compile_file" json:"compile_file"`
	Execute     string `toml:"execute" json:"execute"`
}

type Config struct {
	Langs map[string]Language
	// Order preserves the catalog display order.
	Order []string
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox languages config: %w", err)
	}
	return ParseConfig(data)
}

// ParseConfig ignores "package", which only the judge image build consumes.
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

// Graded reports whether the language links a solution with these grader files: one of them must be its source.
func (l Language) Graded(graders []string) bool {
	return slices.Contains(strings.Fields(l.Compile), gradersArg) && slices.ContainsFunc(graders, l.compiles)
}

func (l Language) compiles(file string) bool {
	return slices.Contains(l.Extensions, strings.ToLower(strings.TrimPrefix(path.Ext(file), ".")))
}

// Graded lists, in catalog order, the languages that link solutions with these grader files.
func (c *Config) Graded(graders []string) []string {
	langs := []string{}
	for _, id := range c.Order {
		if c.Langs[id].Graded(graders) {
			langs = append(langs, id)
		}
	}
	return langs
}

func (c *Config) ForPath(file string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(file), "."))
	for _, id := range c.Order {
		if slices.Contains(c.Langs[id].Extensions, ext) {
			return id
		}
	}
	return ""
}
