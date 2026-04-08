package mdv

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// NewServer builds the HTTP mux with all routes registered.
func NewServer(filePath string, theme string, hub *Hub) *http.ServeMux {
	baseDir := filepath.Dir(filePath)

	mux := http.NewServeMux()

	mainPath := "/" + filepath.Base(filePath)

	// GET / — redirect to /filename.md
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, mainPath, http.StatusFound)
	})

	// GET /content — HTML fragment for SSE-triggered partial refresh.
	// Accepts optional ?path=/other.md to render a different file.
	mux.HandleFunc("GET /content", func(w http.ResponseWriter, r *http.Request) {
		target := filePath
		if p := r.URL.Query().Get("path"); p != "" {
			target = filepath.Join(baseDir, filepath.FromSlash(p))
		}
		content, err := renderFile(target, theme)
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

	// GET /<path> — render .md files as HTML pages; serve other files statically.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".md" {
			absPath := filepath.Join(baseDir, filepath.FromSlash(r.URL.Path))
			content, err := renderFile(absPath, theme)
			if err != nil {
				http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := pageTemplate.Execute(w, pageData{
				Title:   filepath.Base(absPath),
				Path:    r.URL.Path,
				Content: template.HTML(content),
			}); err != nil {
				log.Printf("template execute: %v", err)
			}
			return
		}
		http.FileServer(http.Dir(baseDir)).ServeHTTP(w, r)
	})

	return mux
}

// renderFile reads the markdown file and returns the rendered HTML fragment.
func renderFile(filePath, theme string) ([]byte, error) {
	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	md := newMarkdown(theme)
	return renderMarkdown(md, source)
}
