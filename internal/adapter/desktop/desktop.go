// Package desktop shows the PRadar interface in a native window over the
// application's own loopback server, as ADR-0007 decided.
//
// The window is a shell: it owns the frame, the title and the size, and
// navigates to the owned origin. It exposes no Go function to the page, so
// the page's only capability is the HTTP surface the application already
// serves, and it enables no developer tools.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// ErrUnsupported reports a platform without a native window. PRadar targets
// macOS; elsewhere the interface is reachable in a browser.
var ErrUnsupported = errors.New("a native window is only available on macOS")

// Window describes the window to open.
type Window struct {
	// URL is the owned loopback origin the window navigates to.
	URL string
	// Title names the window.
	Title string
	// Width and Height are the restorable size, in points.
	Width, Height int
}

// Validate rejects a window the application does not own.
func (w Window) Validate() error {
	switch {
	case w.URL == "":
		return errors.New("window URL is required")
	case w.Title == "":
		return errors.New("window title is required")
	case w.Width < 480 || w.Height < 360:
		return fmt.Errorf("window size %dx%d is too small to read an analysis", w.Width, w.Height)
	}
	return nil
}

// OpenInBrowser hands a link to the user's browser instead of loading it in
// the window, so an analysis link to Forgejo never navigates the application
// away from its own origin. The caller validates the URL.
func OpenInBrowser(url string) error {
	if runtime.GOOS != "darwin" {
		return ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// A fixed executable and one argument, with no shell.
	if err := exec.CommandContext(ctx, "/usr/bin/open", "--", url).Run(); err != nil { //nolint:gosec // fixed executable, no shell
		return fmt.Errorf("open %s in the browser: %w", url, err)
	}
	return nil
}
