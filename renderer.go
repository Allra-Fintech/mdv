package main

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// newMarkdown returns a goldmark instance configured with GFM extensions
// and Chroma server-side syntax highlighting for the given theme.
func newMarkdown(theme string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			&mermaidExtender{},
			highlighting.NewHighlighting(
				highlighting.WithStyle(theme),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(false),
					chromahtml.TabWidth(4),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithASTTransformers(
				util.Prioritized(&unicodeHeadingIDTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
}

type mermaidExtender struct{}

func (e *mermaidExtender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&mermaidRenderer{}, 100),
	))
}

type mermaidRenderer struct{}

func (r *mermaidRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(gast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *mermaidRenderer) renderFencedCodeBlock(
	w util.BufWriter, source []byte, node gast.Node, entering bool,
) (gast.WalkStatus, error) {
	n := node.(*gast.FencedCodeBlock)
	if !bytes.EqualFold(n.Language(source), []byte("mermaid")) {
		return gast.WalkContinue, nil
	}

	if entering {
		_, _ = w.WriteString(`<pre class="mermaid">`)
		lines := n.Lines()
		for i := 0; i < lines.Len(); i++ {
			line := lines.At(i)
			_, _ = w.Write(util.EscapeHTML((&line).Value(source)))
		}
		_, _ = w.WriteString(`</pre>`)
	} else {
		_ = w.WriteByte('\n')
	}
	return gast.WalkSkipChildren, nil
}

// unicodeHeadingIDTransformer generates heading IDs that preserve Unicode
// letters and digits (e.g. Korean, CJK), matching GitHub's anchor format.
type unicodeHeadingIDTransformer struct{}

func (t *unicodeHeadingIDTransformer) Transform(doc *gast.Document, reader text.Reader, pc parser.Context) {
	seen := map[string]int{}
	source := reader.Source()

	gast.Walk(doc, func(node gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		heading, ok := node.(*gast.Heading)
		if !ok {
			return gast.WalkContinue, nil
		}
		raw := headingText(heading, source)
		id := slugify(raw, seen)
		heading.SetAttribute([]byte("id"), []byte(id))
		return gast.WalkContinue, nil
	})
}

// headingText extracts plain text content from a heading node.
func headingText(heading *gast.Heading, source []byte) string {
	var buf bytes.Buffer
	gast.Walk(heading, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if t, ok := n.(*gast.Text); ok {
			buf.Write(t.Segment.Value(source))
			if t.SoftLineBreak() {
				buf.WriteByte(' ')
			}
		}
		return gast.WalkContinue, nil
	})
	return buf.String()
}

// slugify converts heading text to a URL-safe anchor ID, keeping Unicode
// letters/digits and replacing spaces/hyphens with a single hyphen.
func slugify(text string, seen map[string]int) string {
	var b strings.Builder
	for _, r := range text {
		if r == ' ' || r == '-' {
			b.WriteRune('-')
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = "heading"
	}
	// Deduplicate: first occurrence is bare id, next is id-1, id-2, …
	if n, exists := seen[id]; exists {
		seen[id]++
		id = fmt.Sprintf("%s-%d", id, n+1)
	} else {
		seen[id] = 0
	}
	return id
}

// renderMarkdown converts markdown source bytes to an HTML fragment.
func renderMarkdown(md goldmark.Markdown, source []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
