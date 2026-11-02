package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/malpou/unusedsqlc"
	"golang.org/x/tools/go/packages"
)

const queriesPackage = "github.com/brawler2011/gate/backend/internal/repository/pg/sqlc"

func main() {
	dir := flag.String("dir", ".", "repository directory")
	flag.Parse()
	if err := run(*dir); err != nil {
		slog.Error("SQL query check failed", "error", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	config, err := unusedsqlc.LoadSQLCConfig(dir)
	if err != nil {
		return fmt.Errorf("load sqlc config: %w", err)
	}
	parser := unusedsqlc.NewQueryParser(unusedsqlc.DefaultConfig())
	var queries []*unusedsqlc.SQLQuery
	for _, pattern := range config.GetQueryFiles(dir) {
		files, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("expand query path: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no SQL files match %q", pattern)
		}
		for _, file := range files {
			parsed, err := parser.ParseFile(file)
			if err != nil {
				return fmt.Errorf("parse SQL queries: %w", err)
			}
			queries = append(queries, parsed...)
		}
	}
	if len(queries) == 0 {
		return fmt.Errorf("no sqlc queries found")
	}

	pkgs, err := packages.Load(&packages.Config{
		Dir:  dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps,
	}, "./backend/...")
	if err != nil {
		return fmt.Errorf("load production packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("production packages contain errors")
	}

	used := make(map[string]bool)
	for _, pkg := range pkgs {
		if strings.Contains(pkg.PkgPath, "/tests/") {
			continue
		}
		for _, file := range pkg.Syntax {
			if ast.IsGenerated(file) {
				continue
			}
			ast.Inspect(file, func(node ast.Node) bool {
				ident, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				fn, ok := pkg.TypesInfo.Uses[ident].(*types.Func)
				if ok && fn.Pkg() != nil && fn.Pkg().Path() == queriesPackage && fn.Type().(*types.Signature).Recv() != nil {
					used[fn.Name()] = true
				}
				return true
			})
		}
	}

	unused := 0
	for _, query := range queries {
		if !used[query.Name] {
			slog.Error("unused sqlc query", "file", query.Filename, "line", query.Line, "query", query.Name)
			unused++
		}
	}
	if unused > 0 {
		return fmt.Errorf("%d unused sqlc queries", unused)
	}
	return nil
}
