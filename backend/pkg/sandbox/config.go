package sandbox

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Language defines the compile and execute configurations for a programming language.
type Language struct {
	Type        string `toml:"type"`
	Compile     string `toml:"compile"`
	CodeFile    string `toml:"code_file"`
	CompileFile string `toml:"compile_file"`
	Execute     string `toml:"execute"`
}

// Config wraps the langs map.
type Config struct {
	Langs map[string]Language
}

// LoadConfig reads the languages configuration (judge/langs.toml).
// Each language's "package" key is consumed by the judge image build and ignored here.
func LoadConfig(path string) (*Config, error) {
	var langs map[string]Language
	meta, err := toml.DecodeFile(path, &langs)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox languages config: %w", err)
	}
	for _, key := range meta.Undecoded() {
		if len(key) == 2 && key[1] == "package" {
			continue
		}
		return nil, fmt.Errorf("unknown key %q in sandbox languages config", key.String())
	}
	if len(langs) == 0 {
		return nil, fmt.Errorf("sandbox languages config %s defines no languages", path)
	}

	return &Config{Langs: langs}, nil
}
