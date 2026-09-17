package ui_test

import (
	"strings"
	"testing"

	"github.com/clement-software/PRadar/internal/ui"
)

func TestRenderMarkdown_KeepsHostileContentInert(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		absent       []string
		present      []string
	}{
		{"raw html is dropped", "hello <script>alert(1)</script> <img src=x onerror=alert(1)>", []string{"<script", "onerror", "<img"}, []string{"hello"}},
		{"javascript links are neutralised", "[x](javascript:alert(1)) [y](JAVASCRIPT:alert(1)) [z](vbscript:x)", []string{"javascript:", "JAVASCRIPT:", "vbscript:"}, []string{"<a"}},
		{"data urls are neutralised", "![i](data:text/html;base64,PHNjcmlwdD4=) [d](data:text/html,<script>)", []string{"data:text"}, nil},
		{"safe links survive", "[pr](https://forge.example/acme/widgets/pulls/1)", nil, []string{`href="https://forge.example/acme/widgets/pulls/1"`}},
		{"html inside mermaid is escaped", "```mermaid\nflowchart LR\n A[\"<script>alert(1)</script>\"] --> B\n```", []string{"<script>"}, []string{`<pre class="mermaid">`, "&lt;script&gt;"}},
		{"mermaid directives stay text", "```mermaid\n%%{init: {'securityLevel':'loose'}}%%\nflowchart LR\n A-->B\n```", nil, []string{`<pre class="mermaid">%%{init`}},
		{"code fences are escaped", "```html\n<b onclick=x>y</b>\n```", []string{"<b onclick"}, []string{`class="language-html"`, "&lt;b onclick"}},
		{"html block is dropped", "<div onmouseover=\"alert(1)\">\n\nsafe\n\n</div>", []string{"<div", "onmouseover"}, []string{"safe"}},
		{"autolinks are safe", "see https://forge.example/x and javascript:alert(1)", []string{`href="javascript`}, []string{`href="https://forge.example/x"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := string(ui.RenderMarkdown(tc.source))
			for _, absent := range tc.absent {
				if strings.Contains(out, absent) {
					t.Errorf("output contains %q:\n%s", absent, out)
				}
			}
			for _, present := range tc.present {
				if !strings.Contains(out, present) {
					t.Errorf("output lacks %q:\n%s", present, out)
				}
			}
		})
	}
}
