package desktop_test

import (
	"os"
	"strings"
	"testing"

	"github.com/clement-software/PRadar/internal/adapter/desktop"
)

func TestWindow_Validate(t *testing.T) {
	t.Parallel()
	valid := desktop.Window{URL: "http://127.0.0.1:52123/", Title: "PRadar", Width: 1100, Height: 800}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid window rejected: %v", err)
	}
	mutate := func(f func(w *desktop.Window)) desktop.Window { w := valid; f(&w); return w }
	for name, window := range map[string]desktop.Window{
		"no url":     mutate(func(w *desktop.Window) { w.URL = "" }),
		"no title":   mutate(func(w *desktop.Window) { w.Title = "" }),
		"too narrow": mutate(func(w *desktop.Window) { w.Width = 320 }),
		"too short":  mutate(func(w *desktop.Window) { w.Height = 200 }),
	} {
		if err := window.Validate(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

// TestShell_ExposesNothingToThePage is the executable form of the ADR-0007
// rule: the window hands the page no Go function and enables no developer
// tools. Both are properties of the code, so the code is what is checked.
func TestShell_ExposesNothingToThePage(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("desktop_darwin.go")
	if err != nil {
		t.Fatal(err)
	}
	shell := string(source)
	for _, forbidden := range []string{".Bind(", ".Eval(", ".Init(", "webview.New(true)"} {
		if strings.Contains(shell, forbidden) {
			t.Errorf("the shell must not use %s: the page gets no capability and no developer tools", forbidden)
		}
	}
	if !strings.Contains(shell, "webview.New(false)") {
		t.Error("developer tools must be disabled explicitly")
	}
	if !strings.Contains(shell, "runtime.LockOSThread()") {
		t.Error("the window must own the main thread")
	}
}
