//go:build integration

package mdv

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitForReloadEvent(t *testing.T, body *bufio.Scanner) <-chan struct{} {
	t.Helper()

	reloadCh := make(chan struct{}, 1)
	go func() {
		for body.Scan() {
			if body.Text() == "event: reload" {
				reloadCh <- struct{}{}
				return
			}
		}
	}()
	return reloadCh
}

func openEventsStream(t *testing.T, url string) (*http.Response, <-chan struct{}) {
	t.Helper()

	resp, err := http.Get(url + "/events")
	if err != nil {
		t.Fatal(err)
	}

	reloadCh := waitForReloadEvent(t, bufio.NewScanner(resp.Body))
	time.Sleep(50 * time.Millisecond) // let SSE connection establish
	return resp, reloadCh
}

// TestLiveReloadOnFileChange verifies that modifying the watched file triggers
// an SSE "event: reload" on the /events stream.
func TestLiveReloadOnFileChange(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(filePath, []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}

	hub := NewHub()
	go WatchFile(filePath, hub)
	time.Sleep(50 * time.Millisecond) // let watcher initialize

	mux := NewServer(filePath, "github", hub)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, reloadCh := openEventsStream(t, srv.URL)
	defer resp.Body.Close()

	if err := os.WriteFile(filePath, []byte("# Updated"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCh:
		// success
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: no reload event received after file change")
	}
}

// TestLiveReloadOnAtomicReplace verifies that replacing the watched file via
// rename still triggers a reload, which matches how many editors save files.
func TestLiveReloadOnAtomicReplace(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(filePath, []byte("# Hello"), 0644); err != nil {
		t.Fatal(err)
	}

	hub := NewHub()
	go WatchFile(filePath, hub)
	time.Sleep(50 * time.Millisecond) // let watcher initialize

	mux := NewServer(filePath, "github", hub)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, reloadCh := openEventsStream(t, srv.URL)
	defer resp.Body.Close()

	tmpPath := filepath.Join(tmpDir, "test.md.tmp")
	if err := os.WriteFile(tmpPath, []byte("# Updated"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		t.Fatal(err)
	}

	select {
	case <-reloadCh:
		// success
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: no reload event received after atomic file replace")
	}
}

// TestRootRedirectsToFile verifies that GET / redirects to /filename.md.
func TestRootRedirectsToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "index.md")
	if err := os.WriteFile(filePath, []byte("# Index"), 0644); err != nil {
		t.Fatal(err)
	}

	mux := NewServer(filePath, "github", NewHub())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/index.md" {
		t.Fatalf("Location = %q, want %q", loc, "/index.md")
	}
}

// TestStaticFileServing verifies that non-markdown files are served as-is.
func TestStaticFileServing(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "doc.md")
	if err := os.WriteFile(filePath, []byte("# Doc"), 0644); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(tmpDir, "image.png")
	pngBytes := []byte("\x89PNG\r\n\x1a\n") // minimal PNG header
	if err := os.WriteFile(imgPath, pngBytes, 0644); err != nil {
		t.Fatal(err)
	}

	mux := NewServer(filePath, "github", NewHub())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/image.png")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("Content-Type = %q, want image/png", ct)
	}
}

// TestPDFExport verifies PDF generation. Skipped if Chrome/Chromium is not found.
func TestPDFExport(t *testing.T) {
	if findChrome() == "" {
		t.Skip("Chrome/Chromium not found; skipping PDF export test")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "doc.md")
	if err := os.WriteFile(filePath, []byte("# PDF Test\n\nHello world."), 0644); err != nil {
		t.Fatal(err)
	}

	hub := NewHub()
	mux := NewServer(filePath, "github", hub)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	outPath := filepath.Join(tmpDir, "out.pdf")
	pageURL := srv.URL + "/doc.md"

	if err := PrintToPDF(pageURL, outPath); err != nil {
		t.Fatalf("PrintToPDF: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("PDF file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("PDF file is empty")
	}
}
