# r/commandline post

**Title:** mdv – open a Markdown file in your browser with live reload, syntax highlighting, and Mermaid diagrams

**Body:**

Just released mdv, a small Go CLI for previewing Markdown files.

```
mdv README.md
```

Opens the file in your browser. Edit and save — the browser updates instantly without a full page refresh (scroll position preserved). Works entirely locally, no API calls.

**What it does:**
- GitHub Flavored Markdown (tables, task lists, strikethrough)
- Syntax highlighting for code blocks (Chroma, server-side)
- Mermaid diagrams from fenced `mermaid` blocks
- PDF export: `mdv --pdf output.pdf README.md`
- Serves local images and assets relative to the file

**Install:**
```bash
go install github.com/Allra-Fintech/mdv@latest
# or
brew tap Allra-Fintech/tap && brew install mdv
```

Repo: https://github.com/Allra-Fintech/mdv

vs the alternatives: grip sends your files to GitHub's API (needs a token, doesn't work offline). glow renders in the terminal. mdv renders in your browser, locally.

To be honest, it's not a complex tool — but I use it every day. Writing docs, editing READMEs, reviewing Markdown before committing. It just quietly stays open and updates as I type.
