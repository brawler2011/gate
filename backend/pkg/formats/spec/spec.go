// Package spec describes a problem package read from an author format.
//
// A Problem says what a package holds and what each file is for; it does not
// prescribe where files go. Format readers fill it, and the importer alone
// decides the draft layout.
package spec

import (
	"fmt"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/points"
)

// Problem is a package read from an author format. File paths are slash
// separated and relative to the package root; a path identifies a file, while
// a name is only what the file is called in the problem and may repeat.
type Problem struct {
	Title         string
	Type          string // "pass-fail", "scoring" or "interactive"
	TimeLimitMs   int
	MemoryLimitMb int
	Checker       *Program
	Validator     *Program
	Interactor    *Program
	// Generators are referenced by tests through their Path.
	Generators []Program
	Solutions  []Program
	// MainSolution is the Path of the solution that generates answers, or "".
	MainSolution string
	// Library holds source files that programs include, such as testlib.h.
	Library    []File
	Statements []Statement
	Assets     []File
	Groups     []Group
	// TextTests says test files are text whose line endings may be CRLF, as
	// in packages prepared on Windows; the importer converts them to LF.
	TextTests bool
}

// File is a package file and the name it keeps in the problem.
type File struct {
	Name string
	Path string
}

// Program is a source file compiled into a checker, validator, interactor,
// generator or solution.
type Program struct {
	File
	// Dependencies maps a name the program includes to the Path of a Library
	// file. Nil makes the whole library available under the file names.
	Dependencies map[string]string
}

// Statement is the Markdown statement in one language.
type Statement struct {
	Language string
	Markdown string
}

// Group is a subtask: tests scored together under one policy.
type Group struct {
	Name      string
	Policy    string // "complete" or "each"
	DependsOn []string
	Tests     []Test
}

// Test takes its input from a package file (Input) or from a run of the
// generator at path Generator with Args. An empty Answer means the main
// solution produces it.
type Test struct {
	Input     string
	Generator string
	Args      []string
	Answer    string
	Points    points.Value
	Sample    bool
}

// KeepInteractiveInputsPrivate unmarks the samples of an interactive problem.
// Gate publishes sample inputs, but in Polygon, PCMS2 and ICPC the input of an
// interactive test is what the interactor reads, such as the hidden answer.
func (p *Problem) KeepInteractiveInputsPrivate() {
	if p.Type != "interactive" {
		return
	}
	for i := range p.Groups {
		for j := range p.Groups[i].Tests {
			p.Groups[i].Tests[j].Sample = false
		}
	}
}

// ParseCommand splits a generator command line into argv. Only quoting and
// escaping are understood; a trailing "> $" (output redirection in Polygon and
// gfmt recipes) is dropped, and any other shell syntax is rejected because
// generators run with plain argv, never through a shell.
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
			return nil, fmt.Errorf("shell syntax is not supported in generator command %q", command)
		default:
			current.WriteRune(r)
		}
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated argument in generator command %q", command)
	}
	if started {
		args = append(args, current.String())
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("empty generator command")
	}
	return args, nil
}

// IsSourceFile reports whether path has the extension of a program source in
// a language any judge may support. The importer checks the actual toolchain.
func IsSourceFile(path string) bool {
	switch strings.ToLower(extension(path)) {
	case ".cpp", ".cc", ".cxx", ".c", ".py", ".go", ".java", ".kt", ".pas", ".dpr", ".rs", ".cs", ".js":
		return true
	}
	return false
}

// IsHeaderFile reports whether path is a C or C++ header.
func IsHeaderFile(path string) bool {
	switch strings.ToLower(extension(path)) {
	case ".h", ".hpp", ".hh", ".hxx":
		return true
	}
	return false
}

// Stem returns the file name of path without directory and extension.
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
