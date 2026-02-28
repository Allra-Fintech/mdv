package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type server struct {
	filePath string
	md       interface {
		Convert(source []byte, writer interface{ Write([]byte) (int, error) }) error
	}
	hub *Hub
}

// newServer builds the HTTP mux with all routes registered.
func newServer(filePath string, theme string, hub *Hub) *http.ServeMux {
	md := newMarkdown(theme)
	baseDir := filepath.Dir(filePath)

	mux := http.NewServeMux()

	// GET / — full HTML page
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		content, err := renderFile(filePath, theme)
		if err != nil {
			http.Error(w, fmt.Sprintf("render error: %v", err), http.StatusInternalServerError)
			return
		}
		title := filepath.Base(filePath)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := pageTemplate.Execute(w, pageData{
			Title:   title,
			Content: template.HTML(content),
		}); err != nil {
			log.Printf("template execute: %v", err)
		}
	})

	// GET /content — HTML fragment for SSE-triggered partial refresh
	mux.HandleFunc("GET /content", func(w http.ResponseWriter, r *http.Request) {
		content, err := renderFile(filePath, theme)
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

	// GET /<asset> — static files from the markdown file directory.
	// This makes relative markdown links like ./screen.gif resolve correctly.
	mux.Handle("GET /", http.FileServer(http.Dir(baseDir)))

	_ = md // md is used via renderFile closure below; suppress unused warning
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
