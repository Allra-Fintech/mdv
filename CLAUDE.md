# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build binary in project directory
make build          # or: go build -o mdv ./cmd/mdv

# Install to ~/.local/bin (default)
make install

# Run
./mdv [--port 7777] [--theme github] [--no-browser] <file.md>

# Run unit tests
make test-unit      # or: go test ./...

# Run integration tests (live reload, PDF, routing)
make test-integration  # or: go test -tags integration ./...

# Run all tests
make test

# Format code
make format         # or: go fmt ./...

# Lint
make lint           # or: go vet ./...

# Tidy dependencies
go mod tidy
```

## Architecture

Code is split into `cmd/mdv/` (entry point) and `internal/mdv/` (core logic). The data flow is:

```
fsnotify event → watchFile() → hub.Broadcast()
                                    ↓
                         SSE clients (/events) receive "reload"
                                    ↓
                    Browser fetches /content → swaps #content innerHTML
```

**`cmd/mdv/main.go`** — Entry point. Parses flags, calls `resolvePort` (tries up to 20 consecutive ports), starts `WatchFile` in a goroutine, wires up `NewServer`, and optionally opens the browser after a 200ms delay.

**`internal/mdv/hub.go`** — Thread-safe SSE broadcast hub. Clients register a `chan struct{}` and receive a non-blocking signal on every `Broadcast()` call.

**`internal/mdv/watcher.go`** — Uses fsnotify to watch both the target file and its parent directory. Watching the directory is necessary for atomic-write editors (Vim, JetBrains) that replace the inode on save. Only `Write` and `Create` events matching the exact file path trigger a broadcast.

**`internal/mdv/server.go`** — Registers three routes on `http.ServeMux`:
- `GET /` — renders file, executes `pageTemplate`
- `GET /content` — renders file, returns bare HTML fragment
- `GET /events` — SSE stream; registers a hub channel, streams `event: reload` messages

**`internal/mdv/renderer.go`** — Builds a goldmark instance with GFM extensions (tables, strikethrough, autolinks, task lists) and Chroma server-side syntax highlighting. A new instance is created per request (stateless).

**`internal/mdv/template.go`** — Single `pageTemplate` embedding GitHub-style CSS and the SSE client JS inline. The JS fetches `/content` on reload events and preserves scroll position.

**`internal/mdv/pdf.go`** — Headless Chrome PDF export via `--print-to-pdf`.
