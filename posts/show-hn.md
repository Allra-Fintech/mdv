# Show HN: mdv – Markdown viewer with live reload (Go, no API calls)

**Title:** Show HN: mdv – CLI Markdown viewer that renders in your browser with live reload

**URL:** https://github.com/Allra-Fintech/mdv

---

**Body (optional context comment):**

I built mdv because I found myself constantly switching between my editor and a browser tab to check how Markdown looked. grip needs GitHub credentials and sends your files to an API. glow renders in the terminal and loses fidelity. I wanted something that stays local, renders faithfully, and updates as I type.

mdv is a small Go CLI (~700 lines) that starts a local HTTP server and opens the file in your browser. When the file changes, it pushes an update over SSE and swaps only the content div — scroll position is preserved.

Features:
- GitHub Flavored Markdown (tables, task lists, strikethrough)
- Server-side syntax highlighting via Chroma (no client JS)
- Mermaid diagrams rendered in-browser on demand
- PDF export via headless Chrome
- Works offline, no rate limits

Install: `go install github.com/Allra-Fintech/mdv@latest`
Or: `brew tap Allra-Fintech/tap && brew install mdv`

Happy to hear feedback on what's missing or broken.

---

**Timing:** Post Tuesday–Thursday, 8–10am US Eastern for best visibility.
