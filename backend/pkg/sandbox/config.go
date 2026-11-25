package sandbox

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
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

// solutionArg in a [graders] setting stands for the problem's solution name.
const solutionArg = "${solution}"

// Graders are linked or run with every solution of a problem; Solution names the contestant's module or class for graders that import it.
type Graders struct {
	Files    map[string][]byte
	Solution string
}

var solutionName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

// ValidSolutionName accepts identifiers that every language takes as a module or class name.
func ValidSolutionName(name string) bool { return solutionName.MatchString(name) }

// LanguageGraders replace the language's settings for solutions with graders.
type LanguageGraders struct {
	CodeFile string `toml:"code_file"`
	Compile  string `toml:"compile"`
	// Entry is the grader that starts the program; an interpreter runs it in place of the solution.
	Entry string `toml:"entry"`
}

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
	// Graders is unset for languages that link graders through ${graders} in Compile alone.
	Graders *LanguageGraders `toml:"graders" json:"-"`
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
		if g := lang.Graders; g != nil && (g.CodeFile == "" || g.Entry == "" && !slices.Contains(strings.Fields(g.Compile), gradersArg)) {
			return nil, fmt.Errorf("language %q graders need code_file and an entry or ${graders} in compile", id)
		}
	}
	return cfg, nil
}

// graded is how a language builds a solution with graders.
type graded struct {
	codeFile, compile, entry string
}

func (l Language) graded(solution string) (graded, bool) {
	g := l.Graders
	if g == nil {
		return graded{codeFile: l.CodeFile, compile: l.Compile}, slices.Contains(strings.Fields(l.Compile), gradersArg)
	}
	uses := strings.Contains(g.CodeFile+g.Compile, solutionArg)
	if uses && !ValidSolutionName(solution) {
		return graded{}, false
	}
	r := strings.NewReplacer(solutionArg, solution)
	return graded{codeFile: r.Replace(g.CodeFile), compile: r.Replace(cmp.Or(g.Compile, l.Compile)), entry: g.Entry}, true
}

// Graded reports whether the language builds a solution with these grader files: they must hold its entry or a source it links, and none may replace the solution's own files.
func (l Language) Graded(graders []string, solution string) bool {
	g, ok := l.graded(solution)
	if !ok {
		return false
	}
	clash := func(name string) bool {
		return name == l.CodeFile || name == g.codeFile || name == cmp.Or(l.CompileFile, "solution")
	}
	if g.entry != "" && l.Type == "interpreter" && g.codeFile == l.CodeFile {
		// The entry grader already runs under that name.
		return false
	}
	linked := slices.Contains(strings.Fields(g.compile), gradersArg)
	return (g.entry == "" || slices.Contains(graders, g.entry)) && (!linked || slices.ContainsFunc(graders, l.compiles)) && !slices.ContainsFunc(graders, clash)
}

func (l Language) compiles(file string) bool {
	return slices.Contains(l.Extensions, strings.ToLower(strings.TrimPrefix(path.Ext(file), ".")))
}

// Graded lists, in catalog order, the languages that build solutions with these grader files.
func (c *Config) Graded(graders []string, solution string) []string {
	langs := []string{}
	for _, id := range c.Order {
		if c.Langs[id].Graded(graders, solution) {
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
