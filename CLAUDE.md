# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build binary in project directory
make build          # or: go build -o mdview .

# Install to ~/.local/bin (default)
make install

# Run
./mdv [--port 7777] [--theme github] [--no-browser] <file.md>

# Tidy dependencies
go mod tidy
```

There are no tests at this time. To verify behavior, build and run manually.

## Architecture

All code lives in a single `main` package across 6 files. The data flow is:

```
fsnotify event → watchFile() → hub.Broadcast()
                                    ↓
                         SSE clients (/events) receive "reload"
                                    ↓
                    Browser fetches /content → swaps #content innerHTML
```

**`main.go`** — Entry point. Parses flags, calls `resolvePort` (tries up to 20 consecutive ports), starts `watchFile` in a goroutine, wires up `newServer`, and optionally opens the browser after a 200ms delay.

**`hub.go`** — Thread-safe SSE broadcast hub. Clients register a `chan struct{}` and receive a non-blocking signal on every `Broadcast()` call.

**`watcher.go`** — Uses fsnotify to watch both the target file and its parent directory. Watching the directory is necessary for atomic-write editors (Vim, JetBrains) that replace the inode on save. Only `Write` and `Create` events matching the exact file path trigger a broadcast.

**`server.go`** — Registers three routes on `http.ServeMux`:
- `GET /` — renders file, executes `pageTemplate`
- `GET /content` — renders file, returns bare HTML fragment
- `GET /events` — SSE stream; registers a hub channel, streams `event: reload` messages

**`renderer.go`** — Builds a goldmark instance with GFM extensions (tables, strikethrough, autolinks, task lists) and Chroma server-side syntax highlighting. A new instance is created per request (stateless).

**`template.go`** — Single `pageTemplate` embedding GitHub-style CSS and the SSE client JS inline. The JS fetches `/content` on reload events and preserves scroll position.
