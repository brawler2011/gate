package gfmt

import (
	"fmt"

	"github.com/brawler2011/gate/backend/pkg/points"
	"gopkg.in/yaml.v3"
)

// Expectation is written as a verdict, a list of verdicts or a mapping with points: OK | [TLE, OK] | {verdicts: [WA], points: 40}.
type Expectation struct {
	Verdicts []string     `yaml:"verdicts" json:"verdicts"`
	Points   *points.Value `yaml:"points,omitempty" json:"points,omitempty"`
}

func (e *Expectation) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		e.Verdicts = []string{node.Value}
		return nil
	case yaml.SequenceNode:
		return node.Decode(&e.Verdicts) //nolint:wrapcheck // yaml errors name the line
	case yaml.MappingNode:
		type plain Expectation
		return node.Decode((*plain)(e)) //nolint:wrapcheck // yaml errors name the line
	}
	return fmt.Errorf("line %d: a solution expects a verdict, a list of verdicts or a mapping", node.Line)
}

func (e Expectation) MarshalYAML() (any, error) {
	switch {
	case e.Points != nil:
		type plain Expectation
		return plain(e), nil
	case len(e.Verdicts) == 1:
		return e.Verdicts[0], nil
	}
	return e.Verdicts, nil
}
