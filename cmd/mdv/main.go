package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Allra-Fintech/mdv/internal/mdv"
)

var version = "dev"

func main() {
	port := flag.Int("port", 7777, "HTTP port (auto-increments if taken)")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser automatically")
	theme := flag.String("theme", "github", "Chroma highlight theme")
	pdf := flag.String("pdf", "", "Export to PDF file and exit (requires Chrome or Chromium)")
	html := flag.String("html", "", "Export to HTML file and exit")
	poll := flag.Duration("poll", 5*time.Second, "Poll interval for live reload of remote (http/https) documents; 0 disables")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: mdv [flags] <file.md | http(s)://host/doc.md>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	target := flag.Arg(0)
	hub := mdv.NewHub()

	var src mdv.Source
	if mdv.IsRemote(target) {
		s, err := mdv.NewRemoteSource(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if _, err := s.ReadMarkdown(""); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		src = s
	} else {
		if _, err := os.Stat(target); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		src = mdv.NewLocalSource(target)
	}

	actualPort, err := resolvePort(*port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	mux := mdv.NewSourceServer(src, *theme, hub)

	addr := fmt.Sprintf("127.0.0.1:%d", actualPort)
	url := fmt.Sprintf("http://%s%s", addr, src.MainPath())

	if *pdf != "" || *html != "" {
		// Export mode: start a temporary server, export, then exit.
		srv := &http.Server{Addr: addr, Handler: mux}
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("server: %v", err)
			}
		}()
		mdv.WaitForServer(addr)

		if *pdf != "" {
			log.Printf("generating PDF from %s", url)
			if err := mdv.PrintToPDF(url, *pdf); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			log.Printf("PDF saved to %s", *pdf)
		}

		if *html != "" {
			log.Printf("generating HTML from %s", url)
			if err := mdv.ExportHTML(url, *html); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			log.Printf("HTML saved to %s", *html)
		}

		os.Exit(0)
	}

	if mdv.IsRemote(target) {
		if *poll > 0 {
			go mdv.WatchURL(target, *poll, hub)
		}
	} else {
		go mdv.WatchFile(target, hub)
	}

	log.Printf("serving %s at %s", target, url)

	if !*noBrowser {
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
