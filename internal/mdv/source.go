package mdv

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// maxRemoteMarkdownSize caps how much of a remote Markdown document is read.
const maxRemoteMarkdownSize = 10 << 20 // 10 MiB

// Source supplies the Markdown documents and assets that the server renders.
// URL paths passed to it are server-relative (e.g. "/docs/other.md").
type Source interface {
	// MainPath is the server URL path of the main document, e.g. "/README.md".
	MainPath() string
	// ReadMarkdown returns the raw Markdown for urlPath.
	ReadMarkdown(urlPath string) ([]byte, error)
	// ServeAsset serves a non-Markdown file (image, etc.) for r.URL.Path.
	ServeAsset(w http.ResponseWriter, r *http.Request)
}

// IsRemote reports whether arg is an http(s) URL rather than a local path.
func IsRemote(arg string) bool {
	u, err := url.Parse(arg)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
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

func (s *localSource) MainPath() string { return "/" + filepath.Base(s.filePath) }

func (s *localSource) ReadMarkdown(urlPath string) ([]byte, error) {
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
type remoteSource struct {
	doc      *url.URL // exact URL of the main document (query preserved)
	base     *url.URL // directory URL that relative paths resolve against
	mainPath string
	client   *http.Client
}

// NewRemoteSource returns a Source backed by an http(s) Markdown URL.
func NewRemoteSource(rawURL string) (Source, error) {
	doc, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if !IsRemote(rawURL) {
		return nil, fmt.Errorf("not an http(s) URL: %s", rawURL)
	}
	doc.Fragment = ""

	name := path.Base(doc.Path)
	if name == "/" || name == "." || strings.HasSuffix(doc.Path, "/") {
		name = "index.md"
	}
	base := doc.ResolveReference(&url.URL{Path: "./"})

	return &remoteSource{
		doc:      doc,
		base:     base,
		mainPath: "/" + name,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *remoteSource) MainPath() string { return s.mainPath }

// resolve maps a server URL path to the remote URL. The path is cleaned as an
// absolute path first, so "/../x" cannot climb above the document directory.
func (s *remoteSource) resolve(urlPath string) string {
	if urlPath == "" || urlPath == s.mainPath {
		return s.doc.String()
	}
	rel := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	return s.base.ResolveReference(&url.URL{Path: rel}).String()
}

func (s *remoteSource) ReadMarkdown(urlPath string) ([]byte, error) {
	return fetchRemote(s.client, s.resolve(urlPath))
}

func (s *remoteSource) ServeAsset(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.resolve(r.URL.Path), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("fetch error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Length", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// fetchRemote GETs rawURL and returns the body, failing on non-2xx statuses
// and on bodies larger than maxRemoteMarkdownSize.
func fetchRemote(client *http.Client, rawURL string) ([]byte, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteMarkdownSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxRemoteMarkdownSize {
		return nil, errors.New("remote document exceeds 10 MiB limit")
	}
	return body, nil
}

// WatchURL polls rawURL every interval and calls hub.Broadcast() when the
// body changes. It blocks forever; run it in a goroutine.
func WatchURL(rawURL string, interval time.Duration, hub *Hub) {
	client := &http.Client{Timeout: 30 * time.Second}
	var last [sha256.Size]byte
	var haveLast, failing bool

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		body, err := fetchRemote(client, rawURL)
		if err != nil {
			if !failing {
				log.Printf("watcher: %v", err)
				failing = true
			}
			continue
		}
		if failing {
			log.Printf("watcher: %s reachable again", rawURL)
			failing = false
		}
		sum := sha256.Sum256(body)
		if haveLast && sum != last {
			hub.Broadcast()
		}
		last, haveLast = sum, true
	}
}
