package judging

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"golang.org/x/sync/singleflight"
)

// CompileFunc compiles a package program and writes the result into dir: the
// primary file as "entry" and each dependency under "deps/".
type CompileFunc func(ctx context.Context, pkg *LoadedPackage, program gfmt.Program, dir string) error

// ProgramCache keeps the checker and interactor of each package compiled by
// this worker's judge image on local disk, so a program is compiled once per
// package and image rather than for every submission. Packages are immutable,
// so entries are never invalidated.
//
// An entry is <Root>/<package ID>/<image digest>/<role>. It is compiled in a
// temporary directory next to it and renamed into place, so a present entry is
// always complete; a crash leaves only a temporary directory, which is never read.
type ProgramCache struct {
	Root    string
	Digest  string
	Compile CompileFunc
	group   singleflight.Group
}

// Get returns the directory holding the compiled program of role.
func (c *ProgramCache) Get(ctx context.Context, pkg *LoadedPackage, role string) (string, error) {
	program, ok := pkg.Format.Problem.Programs[role]
	if !ok {
		return "", fmt.Errorf("package has no %s", role)
	}
	packageDir := filepath.Join(c.Root, pkg.Descriptor.ID.String())
	dir := filepath.Join(packageDir, strings.TrimPrefix(c.Digest, "sha256:"), role)
	_, err, _ := c.group.Do(dir, func() (any, error) {
		if _, err := os.Stat(dir); err == nil {
			return nil, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("stat: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
		tmp, err := os.MkdirTemp(packageDir, ".tmp-")
		if err != nil {
			return nil, fmt.Errorf("mkdirtemp: %w", err)
		}
		if err := c.Compile(ctx, pkg, program, tmp); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, fmt.Errorf("compile %s: %w", role, err)
		}
		if err := os.Rename(tmp, dir); err != nil {
			_ = os.RemoveAll(tmp)
			return nil, fmt.Errorf("rename: %w", err)
		}
		return nil, nil
	})
	return dir, err //nolint:wrapcheck // the errors are this function's own, already wrapped
}
