package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugifyContestTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "English title with spaces",
			title:    "Spring Championship 2026",
			expected: "spring-championship-2026",
		},
		{
			name:     "Russian Cyrillic title",
			title:    "Отборочный раунд 1",
			expected: "otborochnyj-raund-1",
		},
		{
			name:     "Russian Cyrillic with special chars",
			title:    "Весенний кубок: Дивизион А & Б",
			expected: "vesennij-kubok-divizion-a-b",
		},
		{
			name:     "Punctuation and consecutive hyphens",
			title:    "---Hello... World!!! ---",
			expected: "hello-world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slugifyContestTitle(tt.title)
			assert.Equal(t, tt.expected, result)
			err := validateContestLogin(result)
			require.NoError(t, err)
		})
	}
}

func TestValidateContestLogin(t *testing.T) {
	validLogins := []string{
		"round-1",
		"spring-cup-2026",
		"div-a",
		"contest123",
	}

	for _, login := range validLogins {
		err := validateContestLogin(login)
		require.NoError(t, err, "expected login '%s' to be valid", login)
	}

	invalidLogins := []string{
		"ab",                     // too short (< 3)
		"-starting-hyphen",       // starts with hyphen
		"ending-hyphen-",         // ends with hyphen
		"invalid--double-hyphen", // double hyphen
		"UPPERCASE",              // uppercase
		"with spaces",            // spaces
	}

	for _, login := range invalidLogins {
		err := validateContestLogin(login)
		require.Error(t, err, "expected login '%s' to be invalid", login)
	}
}
