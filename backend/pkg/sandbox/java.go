package sandbox

import (
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"
)

// classArg in a language's code_file and compile stands for the source's public top-level Java class.
const classArg = "${class}"

// defaultClass keeps programs without a public top-level class building as before.
const defaultClass = "Main"

// javac translates Unicode escapes before it reads tokens.
var unicodeEscape = regexp.MustCompile(`\\u+([0-9a-fA-F]{4})`)

// javaClass names the public top-level type, which javac requires to match the file name.
func javaClass(source []byte) string {
	source = unicodeEscape.ReplaceAllFunc(source, func(m []byte) []byte {
		s, err := strconv.Unquote(`"\u` + string(m[len(m)-4:]) + `"`)
		if err != nil {
			return m
		}
		return []byte(s)
	})
	triple := func(j int) bool {
		return j+2 < len(source) && source[j] == '"' && source[j+1] == '"' && source[j+2] == '"'
	}
	depth, parens, public, typeKeyword := 0, 0, false, false
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' && source[i] != '\r' {
				i++
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			i = indexFrom(source, i+2, "*/") + 2
		case triple(i):
			for i += 3; i < len(source) && !triple(i); i++ {
				if source[i] == '\\' {
					i++
				}
			}
			i += 3
		case c == '"' || c == '\'':
			i++
			for i < len(source) && source[i] != c && source[i] != '\n' && source[i] != '\r' {
				if source[i] == '\\' {
					i++
				}
				i++
			}
			i++
		case c == '(':
			parens++
			i++
		case c == ')':
			parens--
			i++
		case parens > 0:
			// Annotation arguments hold expressions and array braces, not declarations.
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
				if public && utf8.ValidString(word) {
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
