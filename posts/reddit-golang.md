# r/golang post

**Title:** mdv – a Markdown viewer with live reload ~~I~~ (Claude) built in Go

**Body:**

Hey r/golang — I wrote a small CLI tool called mdv that renders Markdown files in your browser and live-reloads when the file changes.

The interesting bits from a Go perspective:
- Uses `fsnotify` to watch for file changes. Had to watch both the file *and* its parent directory because editors like Vim and JetBrains do atomic saves (replace the inode), so watching the file alone misses events.
- Live reload via SSE (Server-Sent Events), no WebSocket dependency. The browser gets a `reload` event, fetches the `/content` fragment, and swaps only the `#content` div — scroll position preserved.
- goldmark isn't safe for concurrent use, so a fresh instance is created per request instead of sharing a global one.
- Syntax highlighting via Chroma happens server-side, so there's zero client-side JS for that.

Demo: https://raw.githubusercontent.com/Allra-Fintech/mdv/main/screen.gif

Repo: https://github.com/Allra-Fintech/mdv

Install: `go install github.com/Allra-Fintech/mdv@latest`

To be honest, it's a pretty simple build — but it's also something I genuinely use every day. Writing docs, reviewing PRs, editing READMEs. Having it just stay open and update as I type is one of those small things that turns out to matter a lot.

Feedback welcome — especially if you know a better pattern for the atomic-save watcher issue.
