package mdv

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// maxRemoteMarkdownSize caps how much of a remote Markdown document is read.
	maxRemoteMarkdownSize = 10 << 20 // 10 MiB
	// maxRemoteAssetSize caps how much of a remote asset is proxied.
	maxRemoteAssetSize = 50 << 20 // 50 MiB
)

// Source supplies the Markdown documents and assets that the server renders.
// URL paths passed to it are server-relative (e.g. "/docs/other.md").
type Source interface {
	// MainPath is the server URL path of the main document, e.g. "/README.md".
	MainPath() string
	// ReadMarkdown returns the raw Markdown for urlPath ("" means the main document).
	ReadMarkdown(ctx context.Context, urlPath string) ([]byte, error)
	// ServeAsset serves a non-Markdown file (image, etc.) for r.URL.Path.
	ServeAsset(w http.ResponseWriter, r *http.Request)
	// AllowRawHTML reports whether raw HTML in the Markdown may be rendered as-is.
	AllowRawHTML() bool
}

// IsRemote reports whether arg is an http(s) URL rather than a local path.
func IsRemote(arg string) bool {
	u, err := url.Parse(arg)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// RedactURL returns rawURL without userinfo, query or fragment, for logs and
// error messages. Unparseable input is replaced entirely.
func RedactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid url>"
	}
	return redact(u)
}

func redact(u *url.URL) string {
	c := *u
	c.User = nil
	c.RawQuery = ""
	c.ForceQuery = false
	c.Fragment = ""
	c.RawFragment = ""
	return c.String()
}

// mainPathFor builds the server path for the main document named name,
// avoiding the server's own /content and /events routes.
func mainPathFor(name string) string {
	switch name {
	case "", "/", ".":
		name = "index.md"
	case "content", "events":
		name += ".md"
	}
	return "/" + name
}

// localSource serves a Markdown file and its sibling files from disk.
type localSource struct {
	filePath string
	baseDir  string
}

// NewLocalSource returns a Source backed by a local Markdown file.
func NewLocalSource(filePath string) Source {
	return &localSource{filePath: filePath, baseDir: filepath.Dir(filePath)}
}

func (s *localSource) MainPath() string   { return mainPathFor(filepath.Base(s.filePath)) }
func (s *localSource) AllowRawHTML() bool { return true }

func (s *localSource) ReadMarkdown(_ context.Context, urlPath string) ([]byte, error) {
	target := s.filePath
	if urlPath != "" && urlPath != s.MainPath() {
		target = filepath.Join(s.baseDir, filepath.FromSlash(urlPath))
	}
	return os.ReadFile(target)
}

func (s *localSource) ServeAsset(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir(s.baseDir)).ServeHTTP(w, r)
}

// remoteSource serves a Markdown document fetched over HTTP(S). Relative links
// and assets are resolved against the document's directory URL and proxied
// through the local server, so the browser never leaves 127.0.0.1.
//
// Remote content is untrusted: raw HTML in the Markdown is not rendered,
// proxied assets are served with a sandbox CSP, and connections to loopback,
// private and link-local addresses are refused (including via redirects)
// unless the document itself lives on such an address.
type remoteSource struct {
	doc      *url.URL // exact URL of the main document (query preserved)
	mainPath string
	client   *http.Client

	mu      sync.Mutex
	base    *url.URL // directory URL that relative paths resolve against
	baseSet bool     // base was taken from the first successful main fetch
	served  [sha256.Size]byte
}

// NewRemoteSource returns a Source backed by an http(s) Markdown URL.
func NewRemoteSource(rawURL string) (Source, error) {
	if !IsRemote(rawURL) {
		return nil, fmt.Errorf("not an http(s) URL: %s", RedactURL(rawURL))
	}
	doc, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	doc.Fragment = ""

	name := path.Base(doc.Path)
	if strings.HasSuffix(doc.Path, "/") {
		name = ""
	}

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: addressGuard(hostIsPrivate(doc.Hostname())),
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // the guard must see the real destination
	transport.DialContext = dialer.DialContext

	return &remoteSource{
		doc:      doc,
		base:     dirURL(doc),
		mainPath: mainPathFor(name),
		client: &http.Client{
			Timeout:       30 * time.Second,
			Transport:     transport,
			CheckRedirect: checkRedirect,
		},
	}, nil
}

// checkRedirect keeps Go's 10-hop limit and drops the Referer header, which
// would otherwise carry the previous URL's (possibly signed) query.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	req.Header.Del("Referer")
	return nil
}

func dirURL(u *url.URL) *url.URL {
	d := u.ResolveReference(&url.URL{Path: "./"})
	d.RawQuery = ""
	return d
}

func (s *remoteSource) MainPath() string   { return s.mainPath }
func (s *remoteSource) AllowRawHTML() bool { return false }

// resolve maps a server URL path to the remote URL. The path is cleaned as an
// absolute path first, so "/../x" cannot climb above the document directory.
func (s *remoteSource) resolve(urlPath string) *url.URL {
	if urlPath == "" || urlPath == s.mainPath {
		return s.doc
	}
	rel := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.base.ResolveReference(&url.URL{Path: rel})
}

func (s *remoteSource) ReadMarkdown(ctx context.Context, urlPath string) ([]byte, error) {
	target := s.resolve(urlPath)
	body, final, err := fetchRemote(ctx, s.client, target)
	if err != nil {
		return nil, err
	}
	if target == s.doc {
		s.mu.Lock()
		// Relative links resolve against where the document actually lives
		// after redirects. The base is fixed by the first successful fetch so
		// pages already shown keep resolving their assets consistently.
		if !s.baseSet {
			s.base, s.baseSet = dirURL(final), true
		}
		s.served = sha256.Sum256(body)
		s.mu.Unlock()
	}
	return body, nil
}

// pollMain fetches the main document and reports whether it differs from the
// newest version served to a page (or announced by a previous poll). A
// differing version is recorded as announced.
func (s *remoteSource) pollMain(ctx context.Context) (bool, error) {
	body, _, err := fetchRemote(ctx, s.client, s.doc)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(body)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sum == s.served {
		return false, nil
	}
	s.served = sum
	return true, nil
}

func (s *remoteSource) ServeAsset(w http.ResponseWriter, r *http.Request) {
	target := *s.resolve(r.URL.Path)
	target.RawQuery = r.URL.RawQuery
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		http.Error(w, "bad asset path", http.StatusBadRequest)
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch error: %v", redactErr(err)), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength > maxRemoteAssetSize {
		http.Error(w, "remote asset exceeds 50 MiB limit", http.StatusBadGateway)
		return
	}

	for _, h := range []string{"Content-Type", "Content-Length", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	// Remote assets are untrusted: a proxied HTML/SVG page must not run
	// script in the viewer's origin.
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, maxRemoteAssetSize))
}

// fetchRemote GETs u and returns the body and the final URL after redirects.
// It fails on non-2xx statuses and bodies larger than maxRemoteMarkdownSize.
// Errors never contain the URL's userinfo or query.
func fetchRemote(ctx context.Context, client *http.Client, u *url.URL) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("GET %s: invalid request", redact(u))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, redactErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, nil, fmt.Errorf("GET %s: %s", redact(u), resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteMarkdownSize+1))
	if err != nil {
		return nil, nil, redactErr(err)
	}
	if len(body) > maxRemoteMarkdownSize {
		return nil, nil, errors.New("remote document exceeds 10 MiB limit")
	}
	return body, resp.Request.URL, nil
}

// redactErr strips userinfo and query from the URL inside a *url.Error. The
// inner error is kept only when it is a network or context error; anything
// else (e.g. an unparseable redirect Location, which Go quotes verbatim) is
// replaced with a fixed message.
func redactErr(err error) error {
	var ue *url.Error
	if !errors.As(err, &ue) {
		return err
	}
	inner := ue.Err
	var ne net.Error
	if !errors.As(inner, &ne) && !errors.Is(inner, context.Canceled) && !errors.Is(inner, context.DeadlineExceeded) {
		inner = errors.New("request failed")
	}
	return &url.Error{Op: ue.Op, URL: RedactURL(ue.URL), Err: inner}
}

// hostIsPrivate reports whether host is (or resolves only to) a loopback,
// private, link-local or unspecified address. Lookup failures count as public.
func hostIsPrivate(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateIP(ip)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		if !isPrivateIP(a.IP) {
			return false
		}
	}
	return true
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// addressGuard returns a net.Dialer Control func that refuses non-public
// destinations unless allowPrivate is set. It runs on the resolved IP, so it
// covers redirects and DNS rebinding alike.
func addressGuard(allowPrivate bool) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if allowPrivate {
			return nil
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		if ip := net.ParseIP(host); ip == nil || isPrivateIP(ip) {
			return fmt.Errorf("refusing to connect to non-public address %s", host)
		}
		return nil
	}
}

// WatchRemote polls src's main document every interval and calls
// hub.Broadcast() when it differs from the newest version served to a page.
// It does nothing for sources that are not remote, and returns when ctx is done.
func WatchRemote(ctx context.Context, src Source, interval time.Duration, hub *Hub) {
	rs, ok := src.(*remoteSource)
	if !ok {
		return
	}
	failing := false

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		changed, err := rs.pollMain(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if !failing {
				log.Printf("watcher: %v", err)
				failing = true
			}
			continue
		}
		if failing {
			log.Printf("watcher: remote document reachable again")
			failing = false
		}
		if changed {
			hub.Broadcast()
		}
	}
}
