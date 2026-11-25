package icpc

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"gopkg.in/yaml.v3"
)

// Grading is a testdata.yaml, or the grading section of problem.yaml; nil fields inherit.
type Grading struct {
	OnReject    *string  `yaml:"on_reject"`
	Grading     *string  `yaml:"grading"`
	GraderFlags *string  `yaml:"grader_flags"`
	InputFlags  any      `yaml:"input_validator_flags"`
	OutputFlags any      `yaml:"output_validator_flags"`
	AcceptScore *float64 `yaml:"accept_score"`
	RejectScore *float64 `yaml:"reject_score"`
}

// settings are the inherited values of a test data group.
type settings struct {
	onReject, grading, graderFlags string
	inputFlags, outputFlags        any
	accept, reject                 float64
}

var defaults = settings{onReject: "break", grading: "default", accept: 1}

func (s settings) with(g Grading) settings {
	if g.OnReject != nil {
		s.onReject = *g.OnReject
	}
	if g.Grading != nil {
		s.grading = *g.Grading
	}
	if g.GraderFlags != nil {
		s.graderFlags = *g.GraderFlags
	}
	if g.InputFlags != nil {
		s.inputFlags = g.InputFlags
	}
	if g.OutputFlags != nil {
		s.outputFlags = g.OutputFlags
	}
	if g.AcceptScore != nil {
		s.accept = *g.AcceptScore
	}
	if g.RejectScore != nil {
		s.reject = *g.RejectScore
	}
	return s
}

// testdata reads the tree under data/, the root group being data/ itself, as the ICPC default grader scores it.
type testdata struct {
	fsys    fs.FS
	scoring bool
	// legacy is problem.yaml grading, which overrides every group.
	legacy Grading
	// tests numbers the tests in the order ICPC uses them.
	tests int
	// ignoreSample scores data/ by secret alone.
	ignoreSample bool
}

func (t *testdata) read() (spec.Group, error) {
	// problem.yaml grading names on_reject values as problemtools translates them.
	if v := t.legacy.OnReject; v != nil {
		if alias, ok := map[string]string{"first_error": "break", "grade": "continue"}[*v]; ok {
			t.legacy.OnReject = &alias
		}
	}
	root, _, err := t.group("data", defaults, true)
	return root, err
}

// Tests and subgroups come in the order of their base names, as ICPC uses them.
func (t *testdata) group(dir string, parent settings, isRoot bool) (spec.Group, bool, error) {
	entries, err := fs.ReadDir(t.fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return spec.Group{}, false, nil
	}
	if err != nil {
		return spec.Group{}, false, fmt.Errorf("read %s: %w", dir, err)
	}
	s := parent
	if config := path.Join(dir, "testdata.yaml"); regular(t.fsys, config) {
		data, err := spec.ReadFile(t.fsys, config)
		if err != nil {
			return spec.Group{}, false, err
		}
		var g Grading
		if err := yaml.Unmarshal(data, &g); err != nil {
			return spec.Group{}, false, spec.Syntax(config, err)
		}
		s = s.with(g)
	}
	s = s.with(t.legacy)
	name := strings.TrimPrefix(strings.TrimPrefix(dir, "data"), "/")
	group := spec.Group{Name: name}
	sample := name == "sample" || strings.HasPrefix(name, "sample/")
	if group.ValidatorArgs, err = flags(dir, "input_validator_flags", s.inputFlags); err != nil {
		return group, false, err
	}
	if group.CheckerArgs, err = flags(dir, "output_validator_flags", s.outputFlags); err != nil {
		return group, false, err
	}
	accept, err := t.grade(dir, s, &group, isRoot, !isRoot && parent.onReject == "break")
	if err != nil {
		return group, false, err
	}
	key := func(e fs.DirEntry) string {
		if e.IsDir() {
			return e.Name()
		}
		return strings.TrimSuffix(e.Name(), ".in")
	}
	slices.SortStableFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(key(a), key(b)) })
	found := false
	for _, e := range entries {
		full := path.Join(dir, e.Name())
		if e.IsDir() {
			child, ok, err := t.group(full, s, false)
			if err != nil {
				return group, false, err
			}
			if ok {
				group.Groups = append(group.Groups, child)
				found = true
			}
			continue
		}
		stem, ok := strings.CutSuffix(e.Name(), ".in")
		if !e.Type().IsRegular() || !ok {
			continue
		}
		t.tests++
		test := spec.Test{Input: full, Sample: sample, Points: accept, Ordinal: t.tests}
		if t.ignoreSample && sample {
			test.Points = 0
		}
		if answer := path.Join(dir, stem+".ans"); regular(t.fsys, answer) {
			test.Answer = answer
		}
		group.Tests = append(group.Tests, test)
		found = true
	}
	return group, found, nil
}

// grade maps the default grader settings of a group of a scoring problem to its accept score.
func (t *testdata) grade(dir string, s settings, group *spec.Group, isRoot, parentBreaks bool) (points.Value, error) {
	if !t.scoring {
		return 0, nil
	}
	unsupported := func(format string, args ...any) error {
		return spec.Fail(pkg.ErrUnsupported, dir, "", "%s: "+format, append([]any{dir}, args...)...)
	}
	switch {
	case s.grading != "default":
		return 0, unsupported("grading %q is not supported", s.grading)
	case s.onReject != "break" && s.onReject != "continue":
		return 0, unsupported("on_reject %q is not supported", s.onReject)
	case s.reject != 0:
		return 0, unsupported("reject_score other than 0 is not supported")
	}
	group.StopOnFailure = s.onReject == "break"
	group.Aggregate = "sum"
	for _, flag := range strings.Fields(s.graderFlags) {
		switch flag {
		case "sum", "min", "max", "avg":
			group.Aggregate = flag
		case "worst_error", "no_errors", "first_error":
		// The flag applies to data/ only, though its subgroups inherit it.
		case "ignore_sample":
			t.ignoreSample = t.ignoreSample || isRoot
		// They decide only whether the group counts as accepted, which Gate's scores ignore unless the parent stops on rejection.
		case "accept_if_any_accepted", "always_accept":
			if parentBreaks {
				return 0, unsupported("grader flag %q under on_reject: break is not supported", flag)
			}
		default:
			return 0, unsupported("grader flag %q is not supported", flag)
		}
	}
	// With the sample worth nothing, the sum of data/ is the secret score.
	if isRoot && t.ignoreSample {
		group.Aggregate = "sum"
	}
	text := strconv.FormatFloat(s.accept, 'f', -1, 64)
	accept, err := points.Parse(text)
	if err != nil {
		return 0, spec.Fail(pkg.ErrFormat, dir, text, "%s: invalid accept_score %s: %v", dir, text, err)
	}
	return accept, nil
}

// flags reads validator flags given as a string or a sequence; the per-validator map form is not supported.
func flags(dir, key string, value any) ([]string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return strings.Fields(v), nil
	case []any:
		var result []string
		for _, arg := range v {
			s, ok := arg.(string)
			if !ok {
				s = fmt.Sprint(arg)
			}
			result = append(result, s)
		}
		return result, nil
	default:
		return nil, spec.Fail(pkg.ErrUnsupported, dir, key, "%s: %s per validator is not supported", dir, key)
	}
}
