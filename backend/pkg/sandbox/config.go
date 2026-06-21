package sandbox

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Language defines the compile and execute configurations for a programming language.
type Language struct {
	Type        string `yaml:"type"`
	Compile     string `yaml:"compile"`
	CodeFile    string `yaml:"code_file"`
	CompileFile string `yaml:"compile_file"`
	Execute     string `yaml:"execute"`
}

// Config wraps the langs map.
type Config struct {
	Langs map[string]Language `yaml:"langs"`
}

// LoadConfig reads the languages configuration from the specified YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox languages config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse sandbox languages config: %w", err)
	}

	return &cfg, nil
}
