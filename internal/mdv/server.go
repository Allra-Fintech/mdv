package mdv

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path"
)

// NewServer builds the HTTP mux for a local Markdown file.
func NewServer(filePath string, theme string, hub *Hub) *http.ServeMux {
	return NewSourceServer(NewLocalSource(filePath), theme, hub)
}

// NewSourceServer builds the HTTP mux with all routes registered, reading
// documents and assets from src.
func NewSourceServer(src Source, theme string, hub *Hub) *http.ServeMux {
	mux := http.NewServeMux()

	mainPath := src.MainPath()

	// GET / — redirect to /filename.md
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, mainPath, http.StatusFound)
	})

	// GET /content — HTML fragment for SSE-triggered partial refresh.
	// Accepts optional ?path=/other.md to render a different file.
	mux.HandleFunc("GET /content", func(w http.ResponseWriter, r *http.Request) {
		content, err := renderSource(src, r.URL.Query().Get("path"), theme)
		if err != nil {
			http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(content)
	})

	// GET /events — SSE stream
	mux.HandleFunc("GET /events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		ch := make(chan struct{}, 1)
		hub.Register(ch)
		defer hub.Unregister(ch)

		// Send a comment to establish the connection
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				fmt.Fprintf(w, "event: reload\ndata: reload\n\n")
				flusher.Flush()
			}
		}
	})

	// GET /<path> — render .md files (and the main document) as HTML pages;
	// serve other files as assets.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == mainPath || path.Ext(r.URL.Path) == ".md" {
			content, err := renderSource(src, r.URL.Path, theme)
			if err != nil {
				http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := pageTemplate.Execute(w, pageData{
				Title:   path.Base(r.URL.Path),
				Path:    r.URL.Path,
				Content: template.HTML(content),
			}); err != nil {
				log.Printf("template execute: %v", err)
			}
			return
		}
		src.ServeAsset(w, r)
	})

	return mux
}

// renderSource reads the markdown at urlPath from src and returns the rendered
// HTML fragment. An empty urlPath means the main document.
func renderSource(src Source, urlPath, theme string) ([]byte, error) {
	source, err := src.ReadMarkdown(urlPath)
	if err != nil {
		return nil, err
	}
	md := newMarkdown(theme)
	return renderMarkdown(md, source)
}
