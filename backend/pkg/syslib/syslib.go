// Package syslib provides vendored library sources and standard checkers to problem programs.
package syslib

import (
	"crypto/sha1" //nolint:gosec // Git blob IDs are SHA-1; they identify upstream texts, not secrets.
	"embed"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"strings"
)

//go:embed testlib
var vendored embed.FS

type File struct {
	Name string
	// Extensions select which programs can include the file.
	Extensions []string
	Version    string
	Commit   string
	Content  string
	upstream map[string]bool
}

var catalog = []File{
	load("testlib", "testlib.h", []string{"cpp"}, `(?m)^#define VERSION "([^"]+)"`),
}

func load(dir, name string, extensions []string, version string) File {
	read := func(file string) string {
		data, err := vendored.ReadFile(path.Join(dir, file))
		if err != nil {
			panic(err)
		}
		return string(data)
	}
	f := File{Name: name, Extensions: extensions, Commit: strings.TrimSpace(read("COMMIT")), Content: read(name), upstream: map[string]bool{}}
	if m := regexp.MustCompile(version).FindStringSubmatch(f.Content); m != nil {
		f.Version = m[1]
	}
	for id := range strings.FieldsSeq(read("upstream.txt")) {
		f.upstream[id] = true
	}
	return f
}

type Checker struct {
	Name        string
	Description string
	Content string
}

var checkerDescriptions = map[string]string{
	"acmp":       "Одно вещественное число, абсолютная погрешность 1.5e-6",
	"caseicmp":   "Одно целое число в каждом тесте вида «Case N: …»",
	"casencmp":   "Последовательность целых чисел в каждом тесте вида «Case N: …»",
	"casewcmp":   "Последовательность слов в каждом тесте вида «Case N: …»",
	"dcmp":       "Одно вещественное число, абсолютная или относительная погрешность 1e-6",
	"fcmp":       "Файлы построчно, строки совпадают в точности",
	"hcmp":       "Одно целое число произвольной длины",
	"icmp":       "Одно 32-битное целое число",
	"lcmp":       "Слова по строкам: строки совпадают как последовательности слов",
	"ncmp":       "Упорядоченная последовательность 64-битных целых чисел",
	"nyesno":     "Последовательность ответов YES/NO без учёта регистра",
	"pointscmp":  "Пример чекера с баллами: баллы — разность ответов",
	"pointsinfo": "Пример чекера с points_info",
	"rcmp":       "Одно вещественное число, абсолютная погрешность 1.5e-6",
	"rcmp4":      "Последовательность вещественных чисел, абсолютная или относительная погрешность 1e-4",
	"rcmp6":      "Последовательность вещественных чисел, абсолютная или относительная погрешность 1e-6",
	"rcmp9":      "Последовательность вещественных чисел, абсолютная или относительная погрешность 1e-9",
	"rncmp":      "Последовательность вещественных чисел, абсолютная погрешность 1.5e-5",
	"uncmp":      "Неупорядоченная последовательность 64-битных целых чисел",
	"wcmp":       "Последовательность слов (токенов)",
	"yesno":      "Один ответ YES или NO без учёта регистра",
}

var checkers = loadCheckers()

func loadCheckers() []Checker {
	entries, err := vendored.ReadDir("testlib/checkers")
	if err != nil {
		panic(err)
	}
	list := make([]Checker, 0, len(entries))
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".cpp")
		data, err := vendored.ReadFile("testlib/checkers/" + e.Name())
		if err != nil {
			panic(err)
		}
		description, ok := checkerDescriptions[name]
		if !ok || !strings.Contains(string(data), testlibInclude) {
			panic("syslib: standard checker " + name + " has no description or does not include testlib.h")
		}
		list = append(list, Checker{Name: name, Description: description, Content: string(data)})
	}
	return list
}

const testlibInclude = "#include \"testlib.h\"\n"

func Checkers() []Checker { return checkers }

func LookupChecker(name string) (*Checker, bool) {
	for i := range checkers {
		if checkers[i].Name == name {
			return &checkers[i], true
		}
	}
	return nil, false
}

// Inline system testlib.h so a draft copy cannot change a standard checker.
func (c *Checker) Standalone() string {
	testlib, _ := Lookup("testlib.h")
	return strings.Replace(c.Content, testlibInclude, testlib.Content+"\n", 1)
}

func Files() []File { return catalog }

func Lookup(name string) (*File, bool) {
	for i := range catalog {
		if catalog[i].Name == name {
			return &catalog[i], true
		}
	}
	return nil, false
}

func (f *File) IsUpstream(text string) bool {
	return f.upstream[blobID(text)] || f.upstream[blobID(strings.ReplaceAll(text, "\r\n", "\n"))]
}

func blobID(text string) string {
	h := sha1.New() //nolint:gosec // see the import
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(text))
	_, _ = h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}
