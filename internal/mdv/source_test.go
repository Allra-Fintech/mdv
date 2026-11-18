package mdv

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRemoteFixture(t *testing.T) *httptest.Server {
	t.Helper()
	files := map[string]struct{ ctype, body string }{
		"/docs/README.md": {"text/markdown", "# Remote\n\n[next](other.md)\n"},
		"/docs/other.md":  {"text/markdown", "# Other Remote\n"},
		"/docs/img.png":   {"image/png", "PNGDATA"},
		"/secret.md":      {"text/markdown", "# Secret\n"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", f.ctype)
		w.Write([]byte(f.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serveRemote(t *testing.T, docURL, reqPath string) *httptest.ResponseRecorder {
	t.Helper()
	src, err := NewRemoteSource(docURL)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	NewSourceServer(src, "github", NewHub()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, reqPath, nil))
	return rec
}

func TestIsRemote(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/a.md": true,
		"http://example.com/a.md":  true,
		"README.md":                false,
		"./docs/a.md":              false,
		"ftp://example.com/a.md":   false,
		"https:/nohost.md":         false,
	}
	for in, want := range cases {
		if got := IsRemote(in); got != want {
			t.Errorf("IsRemote(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRemoteRendersMainDocument(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/README.md", "/README.md")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<h1 id="remote">Remote</h1>`) || !strings.Contains(body, "<title>README.md</title>") {
		t.Fatalf("main document not rendered: %q", body)
	}
}

func TestRemoteRendersSiblingViaContentRoute(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/README.md", "/content?path=%2Fother.md")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Other Remote") {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body)
	}
}

func TestRemoteProxiesAssets(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/README.md", "/img.png")
	if rec.Code != http.StatusOK || rec.Body.String() != "PNGDATA" || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("status = %d, type = %q, body = %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
}

func TestRemoteCannotEscapeDocumentDirectory(t *testing.T) {
	fx := newRemoteFixture(t)
	src, err := NewRemoteSource(fx.URL + "/docs/README.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.ReadMarkdown("/../secret.md"); err == nil {
		t.Fatal("expected /../secret.md to stay inside /docs/ and 404")
	}
}

func TestRemoteMissingDocumentIsError(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/README.md", "/missing.md")
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "404") {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body)
	}
}

func TestRemoteMainPathWithoutMarkdownExtension(t *testing.T) {
	fx := newRemoteFixture(t)
	src, err := NewRemoteSource(fx.URL + "/docs/")
	if err != nil {
		t.Fatal(err)
	}
	if got := src.MainPath(); got != "/index.md" {
		t.Fatalf("MainPath = %q, want /index.md", got)
	}
}
