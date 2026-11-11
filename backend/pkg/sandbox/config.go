package sandbox

import (
	"fmt"
	"os"

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

// LoadConfig reads the languages configuration from a file (judge/langs.toml).
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox languages config: %w", err)
	}
	return ParseConfig(data)
}

// ParseConfig parses the languages configuration.
// Each language's "package" key is consumed by the judge image build and ignored here.
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

	return &Config{Langs: langs}, nil
}
