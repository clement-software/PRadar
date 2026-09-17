// Package workspace owns the temporary root under which pull-request content
// is materialised for one analysis and removed after every outcome.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Root is an application-owned directory; every workspace is a direct child.
type Root struct {
	dir      string
	maxBytes int64
}

// New creates the root directory when needed and returns its owner.
func New(dir string, maxBytes int64) (*Root, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	if info, err := os.Lstat(absolute); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("workspace root %s is a symlink; refusing to adopt foreign state", absolute)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create workspace root: %w", err)
	}
	return &Root{dir: absolute, maxBytes: maxBytes}, nil
}

// Dir is the owned root path.
func (r *Root) Dir() string { return r.dir }

// Materialise writes the given files into a fresh directory named name and
// returns it with a cleanup that removes everything again. File names must be
// plain names: no separators, no traversal, no symlink following.
func (r *Root) Materialise(ctx context.Context, name string, files map[string][]byte) (string, func() error, error) {
	if !safeName.MatchString(name) || strings.HasPrefix(name, ".") {
		return "", nil, fmt.Errorf("unsafe workspace name %q", name)
	}
	var total int64
	for fileName, content := range files {
		if !safeName.MatchString(fileName) || fileName == "." || fileName == ".." {
			return "", nil, fmt.Errorf("unsafe workspace file name %q", fileName)
		}
		total += int64(len(content))
	}
	if total > r.maxBytes {
		return "", nil, fmt.Errorf("workspace input of %d bytes exceeds the %d byte limit", total, r.maxBytes)
	}
	dir := filepath.Join(r.dir, name)
	cleanup := func() error { return os.RemoveAll(dir) }
	if err := os.RemoveAll(dir); err != nil {
		return "", nil, fmt.Errorf("reset workspace: %w", err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", nil, fmt.Errorf("create workspace: %w", err)
	}
	for fileName, content := range files {
		if ctx.Err() != nil {
			return "", nil, errors.Join(ctx.Err(), cleanup())
		}
		// O_EXCL guarantees a fresh regular file rather than an existing symlink target.
		f, err := os.OpenFile(filepath.Join(dir, fileName), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return "", nil, errors.Join(fmt.Errorf("create %s: %w", fileName, err), cleanup())
		}
		_, writeErr := f.Write(content)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return "", nil, errors.Join(fmt.Errorf("write %s: %w", fileName, err), cleanup())
		}
	}
	return dir, cleanup, nil
}

// Scavenge removes every child of the root left behind by a previous process.
func (r *Root) Scavenge(context.Context) error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("list workspace root: %w", err)
	}
	var errs []error
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(r.dir, entry.Name())); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
