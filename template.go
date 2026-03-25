package main

import "html/template"

// pageTemplate is the full HTML page sent for GET /.
// It embeds the rendered markdown in #content, connects an SSE EventSource
// for live reload, and includes GitHub-style CSS.
var pageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
*, *::before, *::after { box-sizing: border-box; }

body {
  margin: 0;
  padding: 0;
  background: #ffffff;
  color: #24292f;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  font-size: 16px;
  line-height: 1.5;
}

#wrapper {
  max-width: 800px;
  margin: 0 auto;
  padding: 32px 24px 64px;
}

/* Headings */
h1, h2, h3, h4, h5, h6 {
  margin-top: 24px;
  margin-bottom: 16px;
  font-weight: 600;
  line-height: 1.25;
}
h1 { font-size: 2em; padding-bottom: .3em; border-bottom: 1px solid #d0d7de; }
h2 { font-size: 1.5em; padding-bottom: .3em; border-bottom: 1px solid #d0d7de; }
h3 { font-size: 1.25em; }
h4 { font-size: 1em; }
h5 { font-size: .875em; }
h6 { font-size: .85em; color: #57606a; }

/* Paragraph & spacing */
p { margin-top: 0; margin-bottom: 16px; }

/* Links */
a { color: #0969da; text-decoration: none; }
a:hover { text-decoration: underline; }

/* Code (inline) */
code {
  padding: .2em .4em;
  margin: 0;
  font-size: 85%;
  white-space: break-spaces;
  background-color: #afb8c133;
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
}

/* Code blocks */
pre {
  padding: 16px;
  overflow: auto;
  font-size: 85%;
  line-height: 1.45;
  background-color: #f6f8fa;
  border-radius: 6px;
  margin-top: 0;
  margin-bottom: 16px;
}
pre code {
  padding: 0;
  background: transparent;
  white-space: pre;
  font-size: 100%;
  border-radius: 0;
}

/* Chroma (syntax highlighting) */
.chroma { background: #f6f8fa; border-radius: 6px; }

/* Blockquote */
blockquote {
  margin: 0 0 16px;
  padding: 0 1em;
  color: #57606a;
  border-left: .25em solid #d0d7de;
}
blockquote > :first-child { margin-top: 0; }
blockquote > :last-child  { margin-bottom: 0; }

/* Lists */
ul, ol { padding-left: 2em; margin-top: 0; margin-bottom: 16px; }
li + li { margin-top: .25em; }
li > p  { margin-top: 16px; }

/* Task list */
input[type="checkbox"] { margin-right: .5em; }

/* Tables */
table {
  border-spacing: 0;
  border-collapse: collapse;
  display: block;
  max-width: 100%;
  overflow: auto;
  margin-bottom: 16px;
}
th, td {
  padding: 6px 13px;
  border: 1px solid #d0d7de;
}
th { font-weight: 600; background: #f6f8fa; }
tr:nth-child(2n) { background: #f6f8fa; }

/* Horizontal rule */
hr {
  height: .25em;
  padding: 0;
  margin: 24px 0;
  background-color: #d0d7de;
  border: 0;
}

/* Images */
img { max-width: 100%; height: auto; }

/* Reload indicator */
#reload-indicator {
  position: fixed;
  bottom: 16px;
  right: 16px;
  background: #2da44e;
  color: #fff;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 13px;
  opacity: 0;
  transition: opacity .3s;
  pointer-events: none;
}
#reload-indicator.show { opacity: 1; }
</style>
</head>
<body>
<div id="wrapper">
  <div id="content">{{.Content}}</div>
</div>
<div id="reload-indicator">Reloaded</div>
<script>
(function () {
  var indicator = document.getElementById('reload-indicator');
  var content   = document.getElementById('content');

  function showIndicator() {
    indicator.classList.add('show');
    setTimeout(function () { indicator.classList.remove('show'); }, 1500);
  }

  function stripCrossOrigin(root) {
    root.querySelectorAll('video,audio,source,img,track').forEach(function (el) {
      el.removeAttribute('crossorigin');
    });
  }

  stripCrossOrigin(content);

  document.addEventListener('click', function (e) {
    var a = e.target.closest('a');
    if (!a) return;
    var url;
    try { url = new URL(a.href); } catch (_) { return; }
    if (url.origin !== location.origin || !/\.md$/i.test(url.pathname)) return;
    e.preventDefault();
    fetch('/content?path=' + encodeURIComponent(url.pathname))
      .then(function (r) { return r.text(); })
      .then(function (html) { content.innerHTML = html; history.pushState(null, '', url.pathname + url.hash); });
  }, true);

  var es = new EventSource('/events');
  es.addEventListener('reload', function () {
    var scrollY = window.scrollY;
    fetch('/content?path=' + encodeURIComponent('{{.Path}}'))
      .then(function (r) { return r.text(); })
      .then(function (html) {
        content.innerHTML = html;
        stripCrossOrigin(content);
        var hash = window.location.hash;
        if (hash) {
          var target = document.querySelector(hash);
          if (target) { target.scrollIntoView(); } else { window.scrollTo(0, scrollY); }
        } else {
          window.scrollTo(0, scrollY);
        }
        showIndicator();
      })
      .catch(function (err) { console.error('mdview reload error:', err); });
  });

  es.onerror = function () {
    // Connection lost — silently retry (EventSource auto-reconnects)
    console.warn('mdview: SSE connection lost, retrying…');
  };
})();
</script>
</body>
</html>
`))

// pageData is the data passed to pageTemplate.
type pageData struct {
	Title   string
	Path    string
	Content template.HTML
}
