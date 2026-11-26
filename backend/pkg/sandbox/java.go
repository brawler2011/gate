package sandbox

import (
	"regexp"
	"slices"
)

// classArg in a language's code_file and compile stands for the source's public top-level Java class.
const classArg = "${class}"

// defaultClass keeps programs without a public top-level class building as before.
const defaultClass = "Main"

var javaIdentifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// javaClass names the public top-level type, which javac requires to match the file name.
func javaClass(source []byte) string {
	depth, public := 0, false
	typeKeyword := false
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			end := indexFrom(source, i+2, "*/")
			i = end + 2
		case c == '"' && i+2 < len(source) && source[i+1] == '"' && source[i+2] == '"':
			i = indexFrom(source, i+3, `"""`) + 3
		case c == '"' || c == '\'':
			i++
			for i < len(source) && source[i] != c && source[i] != '\n' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '{':
			depth++
			i++
		case c == '}':
			depth--
			if depth == 0 {
				public = false
			}
			i++
		case c == ';' && depth == 0:
			public = false
			i++
		case isIdentStart(c):
			start := i
			for i < len(source) && (isIdentStart(source[i]) || source[i] >= '0' && source[i] <= '9') {
				i++
			}
			if depth != 0 {
				continue
			}
			word := string(source[start:i])
			switch {
			case typeKeyword:
				typeKeyword = false
				if public && javaIdentifier.MatchString(word) {
					return word
				}
			case word == "public":
				public = true
			case slices.Contains([]string{"class", "interface", "enum", "record"}, word):
				typeKeyword = true
			}
		default:
			i++
		}
	}
	return defaultClass
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// indexFrom returns len(s) when sep is missing, so an unterminated comment or literal ends the scan.
func indexFrom(s []byte, from int, sep string) int {
	for j := from; j+len(sep) <= len(s); j++ {
		if string(s[j:j+len(sep)]) == sep {
			return j
		}
	}
	return len(s)
}
