// Command desktop-shell answers one question for the PRadar MVP: can a single
// Go binary host the existing loopback visualizer in a native macOS window,
// with Mermaid actually rendering, without an npm toolchain?
//
// It opens the given URL in a WKWebView window, then asks the page how many
// Mermaid diagrams rendered and reports the answer back to Go.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/webview/webview_go"
)

func main() {
	url := flag.String("url", "", "URL of a running pradar visualizer")
	probeAfter := flag.Duration("probe-after", 3*time.Second, "delay before probing the rendered page")
	closeAfter := flag.Duration("close-after", 0, "close the window automatically (0 keeps it open)")
	flag.Parse()
	if *url == "" {
		fmt.Fprintln(os.Stderr, "usage: desktop-shell --url http://127.0.0.1:PORT/")
		os.Exit(2)
	}

	view := webview.New(false)
	defer view.Destroy()
	view.SetTitle("PRadar")
	view.SetSize(1100, 800, webview.HintNone)

	// report is called by the page with what actually rendered.
	if err := view.Bind("report", func(mermaid, cards, focusable int, title, theme, background string) {
		fmt.Printf("rendered: mermaid=%d cards=%d focusable=%d theme=%s background=%s title=%q\n",
			mermaid, cards, focusable, theme, background, title)
		if *closeAfter > 0 {
			view.Terminate()
		}
	}); err != nil {
		fmt.Fprintln(os.Stderr, "bind:", err)
		os.Exit(1)
	}

	view.Navigate(*url)
	go func() {
		time.Sleep(*probeAfter)
		view.Dispatch(func() {
			view.Eval(`report(
				document.querySelectorAll('svg[id^="mermaid"], .mermaid svg').length,
				document.querySelectorAll('li.card').length,
				document.querySelectorAll('a[href], button, input, select').length,
				document.title,
				window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
				getComputedStyle(document.body).backgroundColor.replace(/[ ]/g, ''))`)
		})
	}()
	if *closeAfter > 0 {
		go func() {
			time.Sleep(*closeAfter)
			view.Dispatch(view.Terminate)
		}()
	}
	view.Run()
}
