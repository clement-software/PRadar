package workspace_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clement-software/PRadar/internal/adapter/workspace"
)

func TestWorkspace_CleansAfterSuccessFailureAndRestart(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root, err := workspace.New(filepath.Join(base, "root"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	dir, cleanup, err := root.Materialise(t.Context(), "job-1-1", map[string][]byte{"PULL_REQUEST.md": []byte("# x"), "changes.diff": []byte("diff")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, root.Dir()) {
		t.Fatalf("workspace %s escapes root %s", dir, root.Dir())
	}
	if content, err := os.ReadFile(filepath.Join(dir, "changes.diff")); err != nil || string(content) != "diff" {
		t.Fatalf("materialised content = %q, %v", content, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("workspace must be removed after cleanup")
	}

	// Simulated restart: an orphan left by a crashed process is scavenged.
	orphan := filepath.Join(root.Dir(), "job-9-1")
	if err := os.MkdirAll(filepath.Join(orphan, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	restarted, err := workspace.New(root.Dir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Scavenge(t.Context()); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(root.Dir()); len(entries) != 0 {
		t.Fatalf("scavenge left %d entries", len(entries))
	}

	// Cancellation during materialisation leaves nothing behind.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := root.Materialise(ctx, "job-2-1", map[string][]byte{"a": nil, "b": nil}); err == nil {
		t.Fatal("cancelled materialisation must fail")
	}
	if entries, _ := os.ReadDir(root.Dir()); len(entries) != 0 {
		t.Fatalf("cancelled materialisation left %d entries", len(entries))
	}
}

func TestWorkspace_RejectsMaliciousNamesSymlinksAndOversizedInput(t *testing.T) {
	t.Parallel()
	root, err := workspace.New(filepath.Join(t.TempDir(), "root"), 16)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape", "a/b", ".hidden", "", "job\x00"} {
		if _, _, err := root.Materialise(t.Context(), name, nil); err == nil {
			t.Errorf("workspace name %q accepted", name)
		}
	}
	for _, file := range []string{"../../etc/passwd", "dir/file", "..", ".git"} {
		if _, _, err := root.Materialise(t.Context(), "job", map[string][]byte{file: []byte("x")}); err == nil {
			t.Errorf("file name %q accepted", file)
		}
	}
	if _, _, err := root.Materialise(t.Context(), "job", map[string][]byte{"big": []byte(strings.Repeat("x", 17))}); err == nil {
		t.Error("oversized input accepted")
	}
	// A pre-existing symlink at the workspace path is replaced, never followed.
	victim := t.TempDir()
	if err := os.Symlink(victim, filepath.Join(root.Dir(), "job-3-1")); err != nil {
		t.Fatal(err)
	}
	dir, cleanup, err := root.Materialise(t.Context(), "job-3-1", map[string][]byte{"PULL_REQUEST.md": []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(dir); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("workspace must be a real directory")
	}
	if _, err := os.Stat(filepath.Join(victim, "PULL_REQUEST.md")); !os.IsNotExist(err) {
		t.Fatal("content was written through the symlink")
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(victim); len(entries) != 0 {
		t.Fatal("cleanup must not touch the symlink target")
	}
}

func TestWorkspace_RefusesSymlinkedRoot(t *testing.T) {
	t.Parallel()
	victim := t.TempDir()
	if err := os.WriteFile(filepath.Join(victim, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.New(link, 1<<20); err == nil {
		t.Fatal("a symlinked root must be refused before any scavenging")
	}
	if _, err := os.Stat(filepath.Join(victim, "keep")); err != nil {
		t.Fatal("the symlink target must be untouched")
	}
}
