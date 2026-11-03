package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestArchitectureImports(t *testing.T) {
	applications := map[string]bool{
		"identity": true, "organizations": true, "contests": true, "problems": true,
		"submissions": true, "judging": true, "notifications": true, "blogs": true,
		"access": true, "events": true,
	}
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(path), "/")
		module := parts[1]
		if module == "domain" || module == "usecase" || module == "worker" {
			t.Errorf("obsolete application package: %s", path)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, spec := range file.Imports {
			dependency, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("read import in %s: %w", path, err)
			}
			forbidden := strings.Contains(dependency, "/backend/contracts/") || strings.Contains(dependency, "/internal/transport")
			switch {
			case applications[module]:
				forbidden = forbidden || strings.Contains(dependency, "/internal/postgres") ||
					strings.Contains(dependency, "/internal/repository") || strings.Contains(dependency, "/internal/messaging") ||
					strings.Contains(dependency, "/sqlc") || strings.Contains(dependency, "github.com/jackc/") || strings.Contains(dependency, "github.com/nats-io/")
			case module == "transport":
				forbidden = strings.Contains(dependency, "/internal/postgres") || strings.Contains(dependency, "/sqlc") || strings.Contains(dependency, "github.com/jackc/") || dependency == "database/sql"
			case module != "postgres":
				continue
			}
			if forbidden {
				t.Errorf("%s imports forbidden dependency %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
