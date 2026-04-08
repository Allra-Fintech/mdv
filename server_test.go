package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestContentRouteRendersSiblingMarkdownFile(t *testing.T) {
	filePath := filepath.Join("testdata", "navigation", "index.md")
	mux := newServer(filePath, "github", newHub())

	req := httptest.NewRequest(http.MethodGet, "/content?path=%2Fother.md", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "<h1 id=\"other\">Other</h1>") {
		t.Fatalf("response did not render sibling markdown file: %q", body)
	}
}

func TestMarkdownRouteRendersSiblingMarkdownPage(t *testing.T) {
	filePath := filepath.Join("testdata", "navigation", "index.md")
	mux := newServer(filePath, "github", newHub())

	req := httptest.NewRequest(http.MethodGet, "/other.md", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<title>other.md</title>") {
		t.Fatalf("response did not set page title for sibling markdown file: %q", body)
	}
	if !strings.Contains(body, "<h1 id=\"other\">Other</h1>") {
		t.Fatalf("response did not render sibling markdown file: %q", body)
	}
}

func TestPageTemplateSupportsHistoryNavigation(t *testing.T) {
	var b strings.Builder
	err := pageTemplate.Execute(&b, pageData{
		Title:   "index.md",
		Path:    "/index.md",
		Content: template.HTML("<h1>Index</h1>"),
	})
	if err != nil {
		t.Fatalf("execute template: %v", err)
	}

	body := b.String()
	checks := []string{
		"var currentPath = ",
		"/index.md",
		"function loadMermaid()",
		"function renderMermaid(root)",
		"renderMermaid(content).then(function () {",
		"window.addEventListener('popstate'",
		"loadPath(currentPath, window.location.hash, { scrollY: scrollY })",
		"history.pushState({ path: pathname }, '', pathname + (hash || ''))",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("template missing %q", want)
		}
	}
}
