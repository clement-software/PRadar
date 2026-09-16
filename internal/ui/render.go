package ui

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// markdown renders untrusted Markdown with raw HTML dropped, dangerous link
// schemes removed and Mermaid fences turned into inert <pre> blocks that the
// embedded strict-mode Mermaid runtime renders client side.
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(fenceRenderer{}, 100))),
)

// RenderMarkdown converts untrusted Markdown into sanitised HTML.
func RenderMarkdown(source string) template.HTML {
	var buffer bytes.Buffer
	if err := markdown.Convert([]byte(source), &buffer); err != nil {
		return template.HTML(template.HTMLEscapeString(source)) //nolint:gosec // escaped above
	}
	return template.HTML(buffer.String()) //nolint:gosec // goldmark runs without WithUnsafe: raw HTML and unsafe URLs are dropped
}

type fenceRenderer struct{}

func (fenceRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, renderFence)
}

func renderFence(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	block := node.(*ast.FencedCodeBlock)
	if !entering {
		return ast.WalkContinue, nil
	}
	language := string(block.Language(source))
	switch language {
	case "mermaid":
		_, _ = w.WriteString(`<pre class="mermaid">`)
	case "":
		_, _ = w.WriteString(`<pre><code>`)
	default:
		_, _ = w.WriteString(`<pre><code class="language-`)
		_, _ = w.Write(util.EscapeHTML([]byte(language)))
		_, _ = w.WriteString(`">`)
	}
	lines := block.Lines()
	for i := range lines.Len() {
		line := lines.At(i)
		_, _ = w.Write(util.EscapeHTML(line.Value(source)))
	}
	if language == "mermaid" {
		_, _ = w.WriteString("</pre>\n")
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkSkipChildren, nil
}

// noJavaScriptLinks is enforced by goldmark's default (non-unsafe) renderer,
// which drops javascript:, vbscript:, file: and data: URLs. Tests pin it.
var _ = html.IsDangerousURL
