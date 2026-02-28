# mdv

A CLI tool that renders Markdown files in the browser with GitHub-style formatting and live reload on file changes.

## Demo

![mdv demo](./screen.gif)

## Features

- GitHub Flavored Markdown (GFM) — tables, strikethrough, autolinks, task lists
- Server-side syntax highlighting via [Chroma](https://github.com/alecthomas/chroma) — no client-side JS
- Live reload via SSE — browser updates without full page reload, scroll position preserved
- Serves relative local assets from the Markdown directory (e.g. `![demo](./recording.gif)`)
- Auto-increments port if the default is taken (up to 20 attempts)
- Binds to `127.0.0.1` only — no LAN exposure

## Install

```bash
go install github.com/Allra-Fintech/mdv@latest
```

Or build from source:

```bash
git clone https://github.com/Allra-Fintech/mdv
cd mdv
make install        # installs to ~/.local/bin (default) as mdv
# PREFIX=/usr/local make install   # custom prefix
```

### Make targets

| Target | Description |
|--------|-------------|
| `make build` | Build `./mdv` binary in the project directory |
| `make install` | Build and install to `$PREFIX/bin` (default: `~/.local/bin`) |
| `make uninstall` | Remove installed binary |
| `make clean` | Remove local `./mdv` binary |

## Usage

```
mdv [flags] <file.md>
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--port` | int | `7777` | HTTP port (auto-increments if taken) |
| `--no-browser` | bool | `false` | Don't open browser automatically |
| `--theme` | string | `"github"` | Chroma highlight theme |

### Examples

```bash
# Open README.md in browser with live reload
mdv README.md

# Use a custom port and dark highlight theme
mdv --port 8080 --theme monokai README.md

# Print URL but don't open browser
mdv --no-browser README.md
```

## HTTP Routes

| Route | Description |
|-------|-------------|
| `GET /` | Full HTML page (template + rendered markdown) |
| `GET /content` | HTML fragment only (for SSE partial refresh) |
| `GET /events` | SSE stream for live reload |
| `GET /<asset>` | Static files from the Markdown file directory |

## Architecture

```
main.go       — CLI flag parsing, resolvePort, openBrowser, wiring
server.go     — HTTP routes: GET /, GET /content, GET /events (SSE)
renderer.go   — goldmark setup with GFM + Chroma highlighting
hub.go        — SSE broadcast hub (Register/Unregister/Broadcast)
watcher.go    — fsnotify file watcher → hub.Broadcast()
template.go   — Full HTML page template (inline CSS + SSE JS)
```

## Chroma Themes

Common themes: `github`, `github-dark`, `monokai`, `dracula`, `solarized-dark`, `vs`, `xcode`.

Full list: https://xyproto.github.io/splash/docs/
