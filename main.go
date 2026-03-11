package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	port := flag.Int("port", 7777, "HTTP port (auto-increments if taken)")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser automatically")
	theme := flag.String("theme", "github", "Chroma highlight theme")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: mdview [flags] <file.md>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	filePath := flag.Arg(0)
	if _, err := os.Stat(filePath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	actualPort, err := resolvePort(*port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	hub := newHub()
	go watchFile(filePath, hub)

	mux := newServer(filePath, *theme, hub)

	addr := fmt.Sprintf("127.0.0.1:%d", actualPort)
	url := fmt.Sprintf("http://%s/%s", addr, filepath.Base(filePath))
	log.Printf("serving %s at %s", filePath, url)

	if !*noBrowser {
		// Give the server a moment to start before opening the browser
		go func() {
			time.Sleep(200 * time.Millisecond)
			openBrowser(url)
		}()
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// resolvePort tries up to 20 consecutive ports starting from start.
// It returns the first available port or an error.
func resolvePort(start int) (int, error) {
	for p := start; p < start+20; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no available port in range %d–%d", start, start+19)
}

// openBrowser opens url in the default system browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("openBrowser: %v", err)
	}
}
