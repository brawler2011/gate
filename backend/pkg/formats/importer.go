package formats

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Import processes the package at tempSrc using the provided parser,
// and copies/writes the standardized gfmt package into tempDst.
func Import(tempSrc, tempDst string, parser Parser) error {
	plan, err := parser.Parse(tempSrc)
	if err != nil {
		return fmt.Errorf("failed to parse source package: %w", err)
	}

	if err := os.MkdirAll(tempDst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	yamlBytes, err := yaml.Marshal(plan.Problem)
	if err != nil {
		return fmt.Errorf("failed to marshal problem.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tempDst, "problem.yaml"), yamlBytes, 0600); err != nil {
		return fmt.Errorf("failed to write problem.yaml: %w", err)
	}

	for _, mapping := range plan.Mappings {
		srcPath := filepath.Join(tempSrc, mapping.SourcePath)
		dstPath := filepath.Join(tempDst, mapping.TargetPath)

		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", mapping.SourcePath, mapping.TargetPath, err)
		}
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory for %s: %w", dst, err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy content from %s to %s: %w", src, dst, err)
	}
	return nil
}
