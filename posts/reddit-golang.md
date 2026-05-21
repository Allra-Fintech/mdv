# r/golang post

**Title:** mdv – a Markdown viewer with live reload I built in Go

**Body:**

Hey r/golang — I wrote a small CLI tool called mdv that renders Markdown files in your browser and live-reloads when the file changes.

The interesting bits from a Go perspective:
- Uses `fsnotify` to watch for file changes. Had to watch both the file *and* its parent directory because editors like Vim and JetBrains do atomic saves (replace the inode), so watching the file alone misses events.
- Live reload via SSE (Server-Sent Events), no WebSocket dependency. The browser gets a `reload` event, fetches the `/content` fragment, and swaps only the `#content` div — scroll position preserved.
- goldmark isn't safe for concurrent use, so a fresh instance is created per request instead of sharing a global one.
- Syntax highlighting via Chroma happens server-side, so there's zero client-side JS for that.

Repo: https://github.com/Allra-Fintech/mdv

Install: `go install github.com/Allra-Fintech/mdv@latest`

Feedback welcome — especially if you know a better pattern for the atomic-save watcher issue.
