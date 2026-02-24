package main

import (
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// newMarkdown returns a goldmark instance configured with GFM extensions
// and Chroma server-side syntax highlighting for the given theme.
func newMarkdown(theme string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle(theme),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(false),
					chromahtml.TabWidth(4),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
}

// renderMarkdown converts markdown source bytes to an HTML fragment.
func renderMarkdown(md goldmark.Markdown, source []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
