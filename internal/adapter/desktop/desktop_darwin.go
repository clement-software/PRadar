package desktop

import (
	"context"
	"runtime"

	webview "github.com/webview/webview_go"
)

// Show opens the window and blocks until the user closes it or ctx ends.
// It must run on the main goroutine: macOS requires its window to live on the
// main thread.
func Show(ctx context.Context, window Window) error {
	if err := window.Validate(); err != nil {
		return err
	}
	runtime.LockOSThread()
	// false: no developer tools, in every build.
	view := webview.New(false)
	defer view.Destroy()
	view.SetTitle(window.Title)
	view.SetSize(window.Width, window.Height, webview.HintNone)
	view.Navigate(window.URL)

	// Shutting the application down closes the window; closing the window ends
	// Run, which returns to the caller.
	stop := context.AfterFunc(ctx, func() { view.Dispatch(view.Terminate) })
	defer stop()
	view.Run()
	return nil
}
