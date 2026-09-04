package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxHeaderLength defines the maximum character length for the commit subject/header.
	MaxHeaderLength = 65
)

// AllowedTypes contains valid Conventional Commit types.
var AllowedTypes = map[string]struct{}{
	"feat":     {},
	"fix":      {},
	"docs":     {},
	"style":    {},
	"refactor": {},
	"perf":     {},
	"test":     {},
	"build":    {},
	"ci":       {},
	"chore":    {},
	"revert":   {},
}

var (
	// conventionalCommitRegex checks the header structure: type(scope)!: subject
	conventionalCommitRegex = regexp.MustCompile(`^([a-z0-9]+)(?:\(([a-z0-9_./-]+)\))?(!)?:\s*(.*)$`)
	// looseColonRegex detects cases like "feat:no-space" or "feat(scope):no-space"
	looseColonRegex = regexp.MustCompile(`^([a-z0-9]+)(?:\([a-z0-9_./-]+\))?(!)?:[^\s]`)
)

// isIgnoredMessage returns true for merge/revert/special commits that should skip validation.
func isIgnoredMessage(header string) bool {
	trimmed := strings.TrimSpace(header)
	if strings.HasPrefix(trimmed, "Merge ") ||
		strings.HasPrefix(trimmed, "Revert \"") ||
		strings.HasPrefix(trimmed, "Apply suggestions from code review") ||
		strings.HasPrefix(trimmed, "Initial commit") ||
		strings.HasPrefix(trimmed, "squash! ") ||
		strings.HasPrefix(trimmed, "fixup! ") {
		return true
	}
	return false
}

// CleanCommitMessage removes git comments (#) and trailing whitespace.
func CleanCommitMessage(raw string) []string {
	var lines []string
	rawLines := strings.Split(raw, "\n")
	for _, line := range rawLines {
		trimmed := strings.TrimRight(line, "\r\t ")
		if strings.HasPrefix(strings.TrimSpace(trimmed), "#") {
			continue
		}
		lines = append(lines, trimmed)
	}

	// Remove trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// ValidateHeader validates a single commit header or PR title.
func ValidateHeader(header string) error {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return fmt.Errorf("commit message header cannot be empty")
	}

	if isIgnoredMessage(trimmed) {
		return nil
	}

	runeCount := utf8.RuneCountInString(trimmed)
	if runeCount > MaxHeaderLength {
		return fmt.Errorf("header exceeds %d characters (%d characters):\n  %q\nTip: Keep the header under %d chars and move details into the commit body or PR description",
			MaxHeaderLength, runeCount, trimmed, MaxHeaderLength)
	}

	if looseColonRegex.MatchString(trimmed) {
		return fmt.Errorf("missing space after colon in header:\n  %q\nExpected format: <type>(<scope>): <subject>", trimmed)
	}

	matches := conventionalCommitRegex.FindStringSubmatch(trimmed)
	if matches == nil {
		return fmt.Errorf("invalid header format:\n  %q\nExpected Conventional Commits format: <type>(<scope>): <subject>\nAllowed types: %s",
			trimmed, formatAllowedTypes())
	}

	commitType := matches[1]
	subject := strings.TrimSpace(matches[4])

	if _, ok := AllowedTypes[commitType]; !ok {
		return fmt.Errorf("unknown commit type %q in header:\n  %q\nAllowed types: %s",
			commitType, trimmed, formatAllowedTypes())
	}

	if subject == "" {
		return fmt.Errorf("commit subject description cannot be empty:\n  %q", trimmed)
	}

	if strings.HasSuffix(subject, ".") {
		return fmt.Errorf("header subject must not end with a period '.':\n  %q", trimmed)
	}

	return nil
}

// ValidateMessage validates a full commit message (header + optional body).
func ValidateMessage(rawMessage string) error {
	lines := CleanCommitMessage(rawMessage)
	if len(lines) == 0 {
		return fmt.Errorf("commit message cannot be empty")
	}

	header := lines[0]
	if err := ValidateHeader(header); err != nil {
		return err
	}

	if isIgnoredMessage(header) {
		return nil
	}

	if len(lines) > 1 {
		if strings.TrimSpace(lines[1]) != "" {
			return fmt.Errorf("second line of commit message must be empty (separate header from body with a blank line):\n  Line 1: %s\n  Line 2: %s",
				header, lines[1])
		}
	}

	return nil
}

func formatAllowedTypes() string {
	types := []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert"}
	return strings.Join(types, ", ")
}
