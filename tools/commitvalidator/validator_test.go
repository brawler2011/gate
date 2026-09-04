package main

import (
	"strings"
	"testing"
)

func TestValidateHeader(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple commit",
			header:  "feat: add commit validator",
			wantErr: false,
		},
		{
			name:    "valid commit with scope",
			header:  "feat(tooling): add lefthook validator",
			wantErr: false,
		},
		{
			name:    "valid commit with nested scope",
			header:  "refactor(contracts/core): split handler types",
			wantErr: false,
		},
		{
			name:    "valid commit with breaking change marker",
			header:  "feat(api)!: remove deprecated endpoints",
			wantErr: false,
		},
		{
			name:    "valid header at exactly 65 chars",
			header:  "feat(tooling): 12345678901234567890123456789012345678901234567890", // len: 15 + 50 = 65
			wantErr: false,
		},
		{
			name:    "invalid header exceeding 65 chars",
			header:  "feat(tooling): add lefthook pre-commit and markdown task validator", // 66 chars
			wantErr: true,
			errMsg:  "header exceeds 65 characters",
		},
		{
			name:    "invalid trailing period",
			header:  "fix(backend): fix nil pointer dereference.",
			wantErr: true,
			errMsg:  "must not end with a period",
		},
		{
			name:    "invalid missing space after colon",
			header:  "fix(ci):fix frontend build",
			wantErr: true,
			errMsg:  "missing space after colon",
		},
		{
			name:    "invalid unknown type",
			header:  "custom(scope): update something",
			wantErr: true,
			errMsg:  "unknown commit type",
		},
		{
			name:    "invalid empty subject",
			header:  "feat(scope): ",
			wantErr: true,
			errMsg:  "commit subject description cannot be empty",
		},
		{
			name:    "valid ignored merge branch commit",
			header:  "Merge branch 'main' of github.com:brawler2011/gate",
			wantErr: false,
		},
		{
			name:    "valid ignored PR merge commit",
			header:  "Merge pull request #1 from brawler2011/refactor/oapi-ogen-migration",
			wantErr: false,
		},
		{
			name:    "valid ignored revert commit",
			header:  `Revert "feat(api): breaking change"`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeader(tt.header)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateHeader(%q) error = %v, wantErr %v", tt.header, err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateHeader(%q) error message %q does not contain %q", tt.header, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid multi-line commit with blank line 2",
			msg: `feat(tooling): add commit validator

- Add tools/commitvalidator
- Configure lefthook commit-msg
- Add PR title CI check`,
			wantErr: false,
		},
		{
			name: "invalid multi-line commit with missing blank line 2",
			msg: `feat(tooling): add commit validator
- Missing blank line here`,
			wantErr: true,
			errMsg:  "second line of commit message must be empty",
		},
		{
			name: "valid commit message with git comments and trailing whitespace",
			msg: `feat(backend): support pagination query

# Please enter the commit message for your changes. Lines starting
# with '#' will be ignored.
`,
			wantErr: false,
		},
		{
			name:    "invalid empty message",
			msg:     "# Only comments\n# in this commit\n",
			wantErr: true,
			errMsg:  "commit message cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidateMessage() error message %q does not contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}
