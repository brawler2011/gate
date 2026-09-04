package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of taskvalidator:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  taskvalidator [file1.md file2.md ...]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  If no files are passed, all files in '.tasks/*.md' will be validated.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		matches, _ := filepath.Glob(".tasks/*.md")
		if len(matches) == 0 {
			// Also check relative to parent directory if run from tools
			matches, _ = filepath.Glob("../../.tasks/*.md")
			for i := range matches {
				matches[i] = strings.TrimPrefix(matches[i], "../../")
			}
		}
		files = matches
	}

	if len(files) == 0 {
		fmt.Println("No task files found to validate.")
		os.Exit(0)
	}

	validator, err := NewValidator()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing validator: %v\n", err)
		os.Exit(1)
	}

	hasErrors := false
	validatedCount := 0

	for _, file := range files {
		if IsTemplateFile(file) {
			continue
		}

		targetPath := resolveFilePath(file)
		content, err := os.ReadFile(targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to read %s: %v\n", file, err)
			hasErrors = true
			continue
		}

		if err := validator.ValidateContent(file, content); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Validation error in %s:\n%v\n\n", file, err)
			hasErrors = true
		} else {
			fmt.Printf("✅ %s is valid\n", file)
			validatedCount++
		}
	}

	if hasErrors {
		fmt.Fprintf(os.Stderr, "Validation failed for one or more files.\n")
		os.Exit(1)
	}

	fmt.Printf("All %d task file(s) passed validation.\n", validatedCount)
}
