package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func resolveFilePath(file string) string {
	if _, err := os.Stat(file); err == nil {
		return file
	}
	alt := filepath.Join("../..", file)
	if _, err := os.Stat(alt); err == nil {
		return alt
	}
	return file
}

func main() {
	var (
		titleFlag string
		fileFlag  string
	)

	flag.StringVar(&titleFlag, "title", "", "Directly validate a PR title or commit subject string")
	flag.StringVar(&titleFlag, "subject", "", "Alias for -title")
	flag.StringVar(&fileFlag, "file", "", "Path to commit message file (e.g. .git/COMMIT_EDITMSG)")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of commitvalidator:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  commitvalidator [flags] [commit-msg-file]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Examples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  commitvalidator .git/COMMIT_EDITMSG\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  commitvalidator -title \"feat(scope): short description\"\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  git log -1 --pretty=%%B | commitvalidator -\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// 1. Direct title/subject validation (e.g. in CI for PR titles)
	if titleFlag != "" {
		if err := ValidateHeader(titleFlag); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Invalid PR title / commit subject:\n%v\n", err)
			os.Exit(1)
		}
		fmt.Println("✅ PR title is valid Conventional Commit")
		os.Exit(0)
	}

	// 2. File or Stdin validation
	filePath := fileFlag
	if filePath == "" && flag.NArg() > 0 {
		filePath = flag.Arg(0)
	}

	var content []byte
	var err error

	if filePath == "-" {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to read from stdin: %v\n", err)
			os.Exit(1)
		}
	} else if filePath != "" {
		resolvedPath := resolveFilePath(filePath)
		content, err = os.ReadFile(resolvedPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to read commit message file %s: %v\n", filePath, err)
			os.Exit(1)
		}
	} else {
		// Check if stdin is being piped
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			content, err = io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Failed to read from stdin: %v\n", err)
				os.Exit(1)
			}
		} else {
			flag.Usage()
			os.Exit(1)
		}
	}

	if err := ValidateMessage(string(content)); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Commit message validation failed:\n%v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Commit message is valid")
}
