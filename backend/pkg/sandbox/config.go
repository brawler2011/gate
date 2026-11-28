package sandbox

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
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

// reserved holds Python and Java keywords and literals, which neither takes as a module or class name.
var reserved = strings.Fields(`False None True and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield
	_ abstract boolean byte case catch char const default do double enum extends false final float goto implements instanceof int interface long native new null package private protected public short static strictfp super switch synchronized this throw throws transient true void volatile`)

// ValidSolutionName accepts identifiers that every language takes as a module or class name.
func ValidSolutionName(name string) bool { return solutionName.MatchString(name) && !slices.Contains(reserved, name) }

// LanguageGraders replace the language's settings for solutions with graders.
type LanguageGraders struct {
	CodeFile string `toml:"code_file" json:"code_file"`
	Compile  string `toml:"compile" json:"compile"`
	// Entry is the grader that starts the program; an interpreter runs it in place of the solution.
	Entry string `toml:"entry" json:"entry"`
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
	Graders *LanguageGraders `toml:"graders" json:"graders,omitempty"`
	// Variant names the only judge image that has the language; empty means every image.
	Variant string `toml:"variant" json:"-"`
}

// Judge image variants; the full image has every language.
const (
	VariantMinimal = "minimal"
	VariantFull    = "full"
)

type Config struct {
	Langs map[string]Language
	// Order preserves the catalog display order.
	Order []string
	// Missing holds the catalog's languages that this image lacks.
	Missing      map[string]Language
	missingOrder []string
}

// NotInstalledError is a catalog language that the running judge image lacks.
type NotInstalledError struct{ ID, Name string }

func (e *NotInstalledError) Error() string {
	return fmt.Sprintf("language %s (%s) is not installed on this server", e.ID, e.Name)
}

func (e *NotInstalledError) Is(target error) bool { return target == ErrUnknownLanguage }

// Lookup finds an installed language; the error tells an uninstalled catalog language from an unknown one.
func (c *Config) Lookup(id string) (Language, error) {
	if lang, ok := c.Langs[id]; ok {
		return lang, nil
	}
	if lang, ok := c.Missing[id]; ok {
		return Language{}, &NotInstalledError{ID: id, Name: lang.Name}
	}
	return Language{}, fmt.Errorf("%w: %s", ErrUnknownLanguage, id)
}

// LoadConfig reads a catalog for the given image variant.
func LoadConfig(path, variant string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read sandbox languages config: %w", err)
	}
	return ParseConfig(data, variant)
}

// LoadManifest reads the catalog of the judge image whose manifest is in dir.
func LoadManifest(dir string) (*Config, error) {
	variant, err := os.ReadFile(filepath.Join(dir, "variant"))
	if err != nil {
		return nil, fmt.Errorf("failed to read judge image variant: %w", err)
	}
	return LoadConfig(filepath.Join(dir, "langs.toml"), strings.TrimSpace(string(variant)))
}

// ParseConfig keeps the languages of the variant's image and ignores "package", which only the image build consumes.
func ParseConfig(data []byte, variant string) (*Config, error) {
	if variant != VariantMinimal && variant != VariantFull {
		return nil, fmt.Errorf("unknown judge image variant %q", variant)
	}
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
	cfg := &Config{Langs: map[string]Language{}, Missing: map[string]Language{}}
	for _, key := range meta.Keys() {
		if len(key) != 1 {
			continue
		}
		id := key[0]
		lang := langs[id]
		if lang.Name == "" || len(lang.Extensions) == 0 || lang.CodeFile == "" || lang.Execute == "" {
			return nil, fmt.Errorf("language %q needs name, extensions, code_file and execute", id)
		}
		if lang.Variant != "" && lang.Variant != VariantFull {
			return nil, fmt.Errorf("language %q has unknown variant %q", id, lang.Variant)
		}
		for _, ext := range lang.Extensions {
			if ext == "" || ext != strings.ToLower(ext) || strings.ContainsAny(ext, "./") {
				return nil, fmt.Errorf("language %q has invalid extension %q", id, ext)
			}
		}
		if g := lang.Graders; g != nil && (g.CodeFile == "" || g.Entry == "" && !slices.Contains(strings.Fields(g.Compile), gradersArg)) {
			return nil, fmt.Errorf("language %q graders need code_file and an entry or ${graders} in compile", id)
		}
		if lang.Variant == VariantFull && variant != VariantFull {
			cfg.Missing[id] = lang
			cfg.missingOrder = append(cfg.missingOrder, id)
			continue
		}
		cfg.Langs[id] = lang
		cfg.Order = append(cfg.Order, id)
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

// ForPath prefers installed languages but falls back to missing ones, which Lookup then reports as not installed.
func (c *Config) ForPath(file string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(file), "."))
	for _, id := range c.Order {
		if slices.Contains(c.Langs[id].Extensions, ext) {
			return id
		}
	}
	for _, id := range c.missingOrder {
		if slices.Contains(c.Missing[id].Extensions, ext) {
			return id
		}
	}
	return ""
}
