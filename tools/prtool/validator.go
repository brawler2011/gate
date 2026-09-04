package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxHeaderLength defines the maximum character length for the PR title.
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
	conventionalCommitRegex = regexp.MustCompile(`^([a-z0-9]+)(?:\(([A-Za-z0-9_./-]+)\))?(!)?:\s*(.*)$`)
	looseColonRegex         = regexp.MustCompile(`^([a-z0-9]+)(?:\([a-z0-9_./-]+\))?(!)?:[^\s]`)
)

// ValidatePRTitle checks if the title conforms to Conventional Commits and <= 65 characters.
func ValidatePRTitle(title string) error {
	header := strings.TrimSpace(title)
	if header == "" {
		return fmt.Errorf("PR title cannot be empty")
	}

	charCount := utf8.RuneCountInString(header)
	if charCount > MaxHeaderLength {
		return fmt.Errorf("PR title exceeds maximum length of %d characters (got %d chars):\n  %s", MaxHeaderLength, charCount, header)
	}

	if looseColonRegex.MatchString(header) {
		return fmt.Errorf("missing space after colon in PR title:\n  %s", header)
	}

	matches := conventionalCommitRegex.FindStringSubmatch(header)
	if len(matches) == 0 {
		return fmt.Errorf("PR title does not match Conventional Commits format '<type>(<scope>): <subject>':\n  %s", header)
	}

	commitType := matches[1]
	scope := matches[2]
	subject := strings.TrimSpace(matches[4])

	if _, ok := AllowedTypes[commitType]; !ok {
		return fmt.Errorf("unknown commit type '%s'. Allowed types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert", commitType)
	}

	if scope != "" && strings.ToLower(scope) != scope {
		return fmt.Errorf("scope '%s' must be lowercase", scope)
	}

	if subject == "" {
		return fmt.Errorf("PR title must have a non-empty subject after type/scope")
	}

	if strings.HasSuffix(subject, ".") {
		return fmt.Errorf("PR title subject must not end with a period ('.'):\n  %s", header)
	}

	return nil
}
