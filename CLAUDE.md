# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
make build                          # produces ./mdv
go build -o mdv ./cmd/mdv           # equivalent

# Test
make test-unit                      # go test ./...
make test-integration               # go test -tags integration ./...
make test                           # both

# Run a single test
go test ./internal/mdv/ -run TestLiveReloadOnFileChange
go test -tags integration ./internal/mdv/ -run TestLiveReloadOnFileChange

# Format / lint
make format                         # go fmt ./...
make lint                           # go vet ./...

# Tidy deps
go mod tidy
```

## Architecture

```
cmd/mdv/main.go       — flag parsing, resolvePort, openBrowser, server wiring
internal/mdv/
  hub.go              — SSE broadcast hub
  watcher.go          — fsnotify file watcher
  server.go           — HTTP routes and handlers
  renderer.go         — goldmark Markdown→HTML pipeline
  template.go         — full page HTML template (CSS + JS inline)
  pdf.go              — headless Chrome PDF export
  html.go             — HTML export (fetches the rendered page, no Chrome needed)
```

Live reload data flow:

```
fsnotify event → WatchFile() → Hub.Broadcast()
                                     ↓
                        SSE clients (/events) receive "event: reload"
                                     ↓
                   Browser fetches /content → swaps #content innerHTML
```

**Key design points worth knowing before changing things:**

- **Watcher watches both the file and its parent directory.** Atomic-write editors (Vim, JetBrains) replace the inode on save, so watching the file alone misses events. Only `Write`/`Create` events whose path matches the target file trigger a broadcast.

- **`/content` returns an HTML fragment; `GET /<file>.md` returns a full page.** The SSE JS calls `/content?path=` for partial refresh (preserving scroll), while direct navigation renders through `pageTemplate`.

- **`newMarkdown` is stateless and created per request.** goldmark isn't safe for concurrent use so a fresh instance is built each time.

- **Integration tests are behind the `integration` build tag.** Add `//go:build integration` at the top of any new integration test file. Unit tests (`go test ./...`) skip them; CI runs both suites separately.

- **Mermaid rendering is client-side only.** The server emits `<pre class="mermaid">` blocks; the browser lazily loads `mermaid.min.js` from CDN on first encounter.

## Release process

`main.version` is set at build time via `-ldflags -X main.version=...` (see `Makefile`); there's no version constant in source.

To cut a release:

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

Pushing a `v*` tag triggers `.github/workflows/release.yml`, which runs `mislav/bump-homebrew-formula-action` to update the `mdv` formula in the sibling `Allra-Fintech/homebrew-tap` repo (`Formula/mdv.rb`) — it recomputes the `url`/`sha256` from the new tag's source tarball and pushes directly (no manual PR needed). This requires the `ALLRA_GITHUB_TOKEN` org secret to have write access to `homebrew-tap`, which is not guaranteed for public repos by default — if the workflow fails on auth, check that secret's repo visibility first.

There is no separate GitHub Release step or changelog; the tag alone drives both `go install github.com/Allra-Fintech/mdv@vX.Y.Z` and the Homebrew formula bump.
