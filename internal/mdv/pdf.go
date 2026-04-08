package mdv

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// findChrome looks for a Chrome or Chromium binary on the system.
func findChrome() string {
	candidates := []string{
		// macOS
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		// Linux / PATH
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
	}
	return ""
}

// WaitForServer polls addr until the TCP port accepts connections or 5 s passes.
func WaitForServer(addr string) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// PrintToPDF uses headless Chrome to render pageURL and save a PDF to output.
func PrintToPDF(pageURL, output string) error {
	chrome := findChrome()
	if chrome == "" {
		return fmt.Errorf("Chrome or Chromium not found; install Chrome or use browser File → Print → Save as PDF")
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	cmd := exec.Command(chrome,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--print-to-pdf="+absOutput,
		"--print-to-pdf-no-header",
		"--run-all-compositor-stages-before-draw",
		"--disable-extensions",
		pageURL,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}
