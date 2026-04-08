package mdv

import (
	"strings"
	"testing"
)

func TestRenderMarkdownRendersMermaidBlocks(t *testing.T) {
	md := newMarkdown("github")
	source := []byte("```mermaid\ngraph TD\nA[Hello] --> B[World]\n```\n")

	rendered, err := renderMarkdown(md, source)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	body := string(rendered)
	if !strings.Contains(body, `<pre class="mermaid">graph TD`) {
		t.Fatalf("mermaid block was not rendered as mermaid HTML: %q", body)
	}
	if strings.Contains(body, `language-mermaid`) {
		t.Fatalf("mermaid block should bypass regular code highlighting: %q", body)
	}
}

func TestRenderMarkdownEscapesMermaidHTML(t *testing.T) {
	md := newMarkdown("github")
	source := []byte("```mermaid\ngraph TD\nA[<b>unsafe</b>]\n```\n")

	rendered, err := renderMarkdown(md, source)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	body := string(rendered)
	if !strings.Contains(body, "&lt;b&gt;unsafe&lt;/b&gt;") {
		t.Fatalf("mermaid source should be HTML-escaped: %q", body)
	}
}
