// Package spec describes author packages independently of their eventual draft layout.
package spec

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/points"
)

// Syntax reports a metadata file that does not parse, keeping coded errors of custom decoders.
func Syntax(path string, err error) error {
	var coded *pkg.Error
	if errors.As(err, &coded) {
		return err
	}
	return Fail(pkg.ErrPackageSyntax, path, "", "parse %s: %v", path, err)
}

// Fail reports a package defect at path; value names the offending entry. Either may be empty.
func Fail(code *pkg.Error, path, value, format string, args ...any) error {
	params := map[string]any{}
	if path != "" {
		params["path"] = path
	}
	if value != "" {
		params["value"] = value
	}
	return pkg.Invalid(code, fmt.Sprintf(format, args...), params)
}

// Paths are slash-separated relative to the package root; names may repeat.
type Problem struct {
	Title         string
	Type          string // one of ProblemTypes
	TimeLimitMs   int
	MemoryLimitMb int
	Checker       *Program
	Validators    []Program
	Interactor    *Program
	// Tests identify generators by path.
	Generators []Program
	Solutions  []Program
	// Identifies a solution by path; empty means none.
	MainSolution string
	Library      []File
	Statements   []Statement
	Assets       []File
	// Testset is the root group of every test.
	Testset Group
	// Empty names keep standard input and output.
	InputFile, OutputFile string
	// Text test line endings are normalized from CRLF to LF during import.
	TextTests bool
}

type File struct {
	Name string
	Path string
	// Gate supplies system library contents; Path still identifies the dependency.
	System bool
	// Pinned keeps the file's text even when it is an upstream copy of a system file.
	Pinned bool
}

type Program struct {
	File
	// An absent language lets the file extension suggest a configuration.
	Language string
	// Dependencies maps include names to library paths; nil exposes the whole library by file name.
	Dependencies map[string]string
	// Standard checker sources remain in the package even when identified by testlib name.
	Standard string
	// Protocol of a checker, interactor or validator is a sandbox.Protocol; empty means testlib.
	Protocol string
	Args     []string
	// Text is the source of a program the importer writes itself instead of reading Path.
	Text string
	// Groups limits a validator to the tests of these groups; empty means every test.
	Groups []string
	// Verdicts a solution may get on a test; empty keeps it out of the build (see problemsmodel.DraftSolution).
	Verdicts []string
	// Points is the score a solution of a scoring problem is expected to get.
	Points         *points.Value
	CheckerTests   []CheckerTest
	ValidatorTests []ValidatorTest
}

// CheckerTest expects a problemsmodel.CheckerVerdicts verdict for Output.
type CheckerTest struct {
	Input, Output, Answer string
	Verdict               string
	Points                *points.Value
}

type ValidatorTest struct {
	Input string
	Valid bool
	Group string
}

// Statements are a whole-problem LaTeX or Markdown file, or LaTeX files per section.
type Statement struct {
	Language string
	// Empty uses the problem title.
	Title string
	// Charset names the encoding of the statement files; empty means detect.
	Charset  string
	TeX      string
	Markdown string
	HTML     string
	// PDF names an asset holding the whole statement.
	PDF      string
	Sections map[string]string
	// Tutorial is a LaTeX or Markdown file, told apart by extension.
	Tutorial string
}

// LanguageCode turns Polygon and PCMS2 language names into codes.
func LanguageCode(lang string) string {
	switch strings.ToLower(lang) {
	case "english":
		return "en"
	case "russian":
		return "ru"
	default:
		if len(lang) > 2 {
			return lang[:2]
		}
		return lang
	}
}

// IsImage reports a file \includegraphics can show.
func IsImage(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".pdf", ".svg":
		return true
	}
	return false
}

// AddAsset keeps the first file of each name, since statements in several languages often share images.
func (p *Problem) AddAsset(name, rel string) {
	if !slices.ContainsFunc(p.Assets, func(f File) bool { return f.Name == name }) {
		p.Assets = append(p.Assets, File{Name: name, Path: rel})
	}
}

// Group scores its tests and subgroups by an Aggregations entry; only the root is unnamed.
type Group struct {
	Name          string
	Aggregate     string
	StopOnFailure bool
	Requires      []string
	// Feedback is one of FeedbackPolicies; empty means complete.
	Feedback string
	// CheckerArgs follow those of the checker or interactor on the group's own tests; ValidatorArgs those of ICPC validators.
	CheckerArgs   []string
	ValidatorArgs []string
	Tests         []Test
	Groups        []Group
}

// All yields the group and its subgroups, parents first.
func (g *Group) All() iter.Seq[*Group] {
	return func(yield func(*Group) bool) { g.walk(yield) }
}

func (g *Group) walk(yield func(*Group) bool) bool {
	if !yield(g) {
		return false
	}
	for i := range g.Groups {
		if !g.Groups[i].walk(yield) {
			return false
		}
	}
	return true
}

// Aggregations combine the scores of a group's tests and subgroups.
var Aggregations = []string{"sum", "min", "max", "avg"}

// ProblemTypes lists every problem type; an output-only problem judges submitted outputs instead of running a program.
var ProblemTypes = []string{"pass-fail", "scoring", "interactive", "output-only"}

// Scored reports whether a problem type earns points per test rather than all or nothing.
func Scored(problemType string) bool { return problemType == "scoring" || problemType == "output-only" }

// Aggregate combines scores by an Aggregations entry, empty meaning sum; avg rounds down to hundredths.
func Aggregate(kind string, scores []points.Value) points.Value {
	if len(scores) == 0 {
		return 0
	}
	var sum points.Value
	for _, s := range scores {
		sum += s
	}
	switch kind {
	case "min":
		return slices.Min(scores)
	case "max":
		return slices.Max(scores)
	case "avg":
		return sum / points.Value(len(scores))
	}
	return sum
}

// An empty Answer is generated by AnswerSolution, or by the main solution when that is empty too.
type Test struct {
	Input          string
	Generator      string
	Args           []string
	Answer         string
	AnswerSolution string
	Points         points.Value
	Sample         bool
	Pretest        bool
	// StatementInput and StatementOutput name files the statement shows instead of a sample's own.
	StatementInput, StatementOutput string
	// Positive ordinals order tests across groups; zero keeps the group order.
	Ordinal int
}

// FeedbackPolicies are Polygon's: every test, tests up to the first failure, the group's points, nothing.
var FeedbackPolicies = []string{"complete", "icpc", "points", "none"}

// Metadata reads have a separate bound from the draft quotas enforced during content import.
const MaxMetadataBytes = 4 << 20

func ReadFile(fsys fs.FS, name string) ([]byte, error) {
	f, err := fsys.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, Fail(pkg.ErrPackageLayout, name, "", "%s is missing", name)
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxMetadataBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if len(data) > MaxMetadataBytes {
		return nil, pkg.Invalid(pkg.ErrTooLarge, fmt.Sprintf("%s is larger than %d bytes", name, MaxMetadataBytes), map[string]any{"path": name, "max": MaxMetadataBytes})
	}
	return data, nil
}

// Titles of languages without a statement are dropped.
func (p *Problem) SetStatementTitle(language, title string) {
	for i := range p.Statements {
		if p.Statements[i].Language == language && title != p.Title {
			p.Statements[i].Title = title
		}
	}
}

// AddTutorial gives a tutorial to the statement in its language, or else to the first one without a tutorial,
// since the site shows a tutorial with statements in every language.
func AddTutorial(statements []Statement, language, path string) {
	has := func(s Statement) bool { return s.Tutorial != "" || s.Sections["tutorial"] != "" }
	i := slices.IndexFunc(statements, func(s Statement) bool { return s.Language == language })
	if i < 0 {
		i = slices.IndexFunc(statements, func(s Statement) bool { return !has(s) })
	}
	if i >= 0 && !has(statements[i]) {
		statements[i].Tutorial = path
	}
}

// SetExamples reads dir/example.NN and example.NN.a, the statement versions of the samples in order as Polygon and PCMS2 write them,
// keeping those that differ from the test files; files gives a sample's input and answer paths.
func (p *Problem) SetExamples(fsys fs.FS, dir string, files func(Test) (input, answer string)) {
	var samples []*Test
	for g := range p.Testset.All() {
		for j := range g.Tests {
			if g.Tests[j].Sample {
				samples = append(samples, &g.Tests[j])
			}
		}
	}
	slices.SortStableFunc(samples, func(a, b *Test) int { return a.Ordinal - b.Ordinal })
	for n, t := range samples {
		input, answer := files(*t)
		example := path.Join(dir, fmt.Sprintf("example.%02d", n+1))
		// The interactor's files are never what the participant sees.
		if p.Type == "interactive" || differs(fsys, example, input) {
			t.StatementInput = regularFile(fsys, example)
		}
		if p.Type == "interactive" || differs(fsys, example+".a", answer) {
			t.StatementOutput = regularFile(fsys, example+".a")
		}
	}
}

// ExampleDir is the first language folder in root holding example.01.
func ExampleDir(fsys fs.FS, root string) string {
	languages, _ := fs.ReadDir(fsys, root)
	for _, language := range languages {
		if dir := path.Join(root, language.Name()); language.IsDir() && regularFile(fsys, path.Join(dir, "example.01")) != "" {
			return dir
		}
	}
	return ""
}

func regularFile(fsys fs.FS, name string) string {
	if info, err := fs.Stat(fsys, name); err == nil && info.Mode().IsRegular() {
		return name
	}
	return ""
}

// differs reports an example whose text is not that of the test file, which may be absent.
func differs(fsys fs.FS, example, test string) bool {
	a, err := fs.ReadFile(fsys, example)
	if err != nil {
		return false
	}
	b, err := fs.ReadFile(fsys, test)
	normalize := func(data []byte) string {
		return strings.TrimRightFunc(strings.ReplaceAll(string(data), "\r\n", "\n"), unicode.IsSpace)
	}
	return err != nil || normalize(a) != normalize(b)
}

// Generators run as argv, never through a shell; only quoting and escaping are supported.
// Polygon/gfmt output redirection (> $) is stripped; other shell syntax is rejected.
func ParseCommand(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	command = strings.TrimSpace(strings.TrimSuffix(command, "> $"))
	args := []string{}
	var current strings.Builder
	var quote rune
	escaped, started := false, false
	for _, r := range command {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
			continue
		case strings.ContainsRune("|;&><`$\n\r", r):
			return nil, Fail(pkg.ErrUnsupported, "", command, "shell syntax is not supported in generator command %q", command)
		default:
			current.WriteRune(r)
		}
		started = true
	}
	if quote != 0 || escaped {
		return nil, Fail(pkg.ErrPackageSyntax, "", command, "unterminated argument in generator command %q", command)
	}
	if started {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, Fail(pkg.ErrRequired, "", "", "empty generator command")
	}
	return args, nil
}

// FormatCommand quotes argv so that ParseCommand returns it unchanged.
func FormatCommand(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		if arg != "" && !strings.ContainsFunc(arg, func(r rune) bool { return !isPlain(r) }) {
			quoted[i] = arg
		} else {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		}
	}
	return strings.Join(quoted, " ")
}

func isPlain(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.,:=+/@%", r)
}

func IsSourceFile(path string) bool {
	switch strings.ToLower(extension(path)) {
	case ".cpp", ".cc", ".cxx", ".c", ".py", ".go", ".java", ".kt", ".pas", ".dpr", ".rs", ".cs", ".js":
		return true
	}
	return false
}

func IsHeaderFile(path string) bool {
	switch strings.ToLower(extension(path)) {
	case ".h", ".hpp", ".hh", ".hxx":
		return true
	}
	return false
}

func Stem(path string) string {
	base := path[strings.LastIndex(path, "/")+1:]
	return strings.TrimSuffix(base, extension(base))
}

func extension(path string) string {
	base := path[strings.LastIndex(path, "/")+1:]
	if i := strings.LastIndex(base, "."); i > 0 {
		return base[i:]
	}
	return ""
}
