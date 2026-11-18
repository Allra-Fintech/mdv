package mdv

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func newRemoteFixture(t *testing.T) *httptest.Server {
	t.Helper()
	files := map[string]struct{ ctype, body string }{
		"/docs/README.md": {"text/markdown", "# Remote\n\n[next](other.md)\n"},
		"/docs/other.md":  {"text/markdown", "# Other Remote\n"},
		"/docs/img.png":   {"image/png", "PNGDATA"},
		"/docs/page.html": {"text/html", "<script>alert(1)</script>"},
		"/docs/raw.md":    {"text/markdown", "<script>alert(1)</script>\n\n# Raw\n"},
		"/v2/guide.md":    {"text/markdown", "# Guide v2\n"},
		"/v2/pic.png":     {"image/png", "V2PNG"},
		"/secret.md":      {"text/markdown", "# Secret\n"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			http.Redirect(w, r, "/v2/guide.md", http.StatusFound)
			return
		case "/docs/sized.png":
			w.Write([]byte("size=" + r.URL.Query().Get("size")))
			return
		case "/hop":
			http.Redirect(w, r, "/docs/referer.md", http.StatusFound)
			return
		case "/docs/referer.md":
			w.Write([]byte("referer=" + r.Header.Get("Referer")))
			return
		case "/docs/signed.md":
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
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
	if _, err := src.ReadMarkdown(context.Background(), "/../secret.md"); err == nil {
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

func TestMainPathAvoidsServerRoutes(t *testing.T) {
	for _, name := range []string{"content", "events"} {
		src, err := NewRemoteSource("http://127.0.0.1:1/docs/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := src.MainPath(), "/"+name+".md"; got != want {
			t.Errorf("MainPath = %q, want %q", got, want)
		}
	}
}

func TestRemoteOmitsRawHTML(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/raw.md", "/raw.md")
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("remote raw HTML was rendered: %q", body)
	}
	if !strings.Contains(body, "Raw</h1>") {
		t.Fatalf("markdown not rendered: %q", body)
	}
}

func TestRemoteAssetsAreSandboxed(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/README.md", "/page.html")
	if rec.Code != http.StatusOK || rec.Body.String() != "<script>alert(1)</script>" {
		t.Fatalf("asset not served: %d %q", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Fatalf("Content-Security-Policy = %q, want sandbox", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestRemoteAssetForwardsQuery(t *testing.T) {
	fx := newRemoteFixture(t)
	rec := serveRemote(t, fx.URL+"/docs/README.md?token=main", "/sized.png?size=2x")
	if got := rec.Body.String(); got != "size=2x" {
		t.Fatalf("body = %q, want size=2x", got)
	}
}

func TestRemoteRedirectMovesBase(t *testing.T) {
	fx := newRemoteFixture(t)
	src, err := NewRemoteSource(fx.URL + "/latest")
	if err != nil {
		t.Fatal(err)
	}
	body, err := src.ReadMarkdown(context.Background(), "")
	if err != nil || !strings.Contains(string(body), "Guide v2") {
		t.Fatalf("main = %q, err = %v", body, err)
	}
	rec := httptest.NewRecorder()
	NewSourceServer(src, "github", NewHub()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pic.png", nil))
	if rec.Body.String() != "V2PNG" {
		t.Fatalf("asset not resolved against redirected directory: %d %q", rec.Code, rec.Body)
	}
}

func TestRemoteErrorsDoNotLeakQuery(t *testing.T) {
	fx := newRemoteFixture(t)
	src, err := NewRemoteSource(fx.URL + "/docs/signed.md?sig=SECRET")
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.ReadMarkdown(context.Background(), "")
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("err = %v", err)
	}

	unreachable, _ := NewRemoteSource("http://user:pw@127.0.0.1:1/doc.md?sig=SECRET")
	_, err = unreachable.ReadMarkdown(context.Background(), "")
	if err == nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "pw") {
		t.Fatalf("err = %v", err)
	}
}

func TestRedactURL(t *testing.T) {
	if got := RedactURL("https://u:p@example.com/a/b.md?sig=x#frag"); got != "https://example.com/a/b.md" {
		t.Fatalf("RedactURL = %q", got)
	}
}

func TestAddressGuard(t *testing.T) {
	strict := addressGuard(false)
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.1:80", "192.168.1.1:443", "169.254.169.254:80", "[::1]:80", "0.0.0.0:80"} {
		if err := strict("tcp", addr, nil); err == nil {
			t.Errorf("strict guard allowed %s", addr)
		}
	}
	if err := strict("tcp", net.JoinHostPort("93.184.216.34", "443"), nil); err != nil {
		t.Errorf("strict guard refused public address: %v", err)
	}
	if err := addressGuard(true)("tcp", "127.0.0.1:80", nil); err != nil {
		t.Errorf("permissive guard refused loopback: %v", err)
	}
}

func TestRemoteRedirectDropsReferer(t *testing.T) {
	fx := newRemoteFixture(t)
	src, err := NewRemoteSource(fx.URL + "/hop?sig=SECRET")
	if err != nil {
		t.Fatal(err)
	}
	body, err := src.ReadMarkdown(context.Background(), "")
	if err != nil || string(body) != "referer=" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestPollMainComparesAgainstServedVersion(t *testing.T) {
	var mu sync.Mutex
	body := "A"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Write([]byte(body))
	}))
	defer srv.Close()
	set := func(b string) { mu.Lock(); body = b; mu.Unlock() }

	s, err := NewRemoteSource(srv.URL + "/doc.md")
	if err != nil {
		t.Fatal(err)
	}
	rs := s.(*remoteSource)
	ctx := context.Background()

	rs.ReadMarkdown(ctx, "") // startup check sees A
	set("B")
	rs.ReadMarkdown(ctx, "") // page renders B
	set("A")                 // source reverts before the first poll
	if changed, err := rs.pollMain(ctx); err != nil || !changed {
		t.Fatalf("poll of A after serving B: changed = %v, err = %v; want a change", changed, err)
	}
	if changed, err := rs.pollMain(ctx); err != nil || changed {
		t.Fatalf("unchanged poll: changed = %v, err = %v; want no change", changed, err)
	}
}

func TestPollMainNotifiesAfterAnotherTabLoadedNewVersion(t *testing.T) {
	var mu sync.Mutex
	body := "A"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Write([]byte(body))
	}))
	defer srv.Close()

	s, err := NewRemoteSource(srv.URL + "/doc.md")
	if err != nil {
		t.Fatal(err)
	}
	rs := s.(*remoteSource)
	ctx := context.Background()

	rs.ReadMarkdown(ctx, "") // tab 1 renders A
	mu.Lock()
	body = "B"
	mu.Unlock()
	rs.ReadMarkdown(ctx, "") // tab 2 renders B before the next poll
	if changed, err := rs.pollMain(ctx); err != nil || !changed {
		t.Fatalf("changed = %v, err = %v; tab 1 still shows A and must be notified", changed, err)
	}
	if changed, err := rs.pollMain(ctx); err != nil || changed {
		t.Fatalf("changed = %v, err = %v; B was already announced", changed, err)
	}
}

func TestRemoteOversizedAssetWithoutLengthIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/big.bin" {
			chunk := make([]byte, 1<<20)
			for i := 0; i <= maxRemoteAssetSize>>20; i++ {
				w.Write(chunk)
				w.(http.Flusher).Flush() // forces chunked encoding: no Content-Length
			}
			return
		}
		w.Write([]byte("# doc"))
	}))
	defer srv.Close()

	rec := serveRemote(t, srv.URL+"/doc.md", "/big.bin")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestRedactErrDropsUnknownInnerError(t *testing.T) {
	err := redactErr(&url.Error{Op: "Get", URL: "https://h/x?sig=S", Err: errors.New(`bad Location "https://h/?sig=S"`)})
	if strings.Contains(err.Error(), "sig=S") {
		t.Fatalf("leaked: %v", err)
	}
}
