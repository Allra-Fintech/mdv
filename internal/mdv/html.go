package mdv

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ExportHTML fetches the fully rendered page from pageURL and saves it to output.
func ExportHTML(pageURL, output string) error {
	resp, err := http.Get(pageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status fetching %s: %s", pageURL, resp.Status)
	}

	absOutput, err := filepath.Abs(output)
	if err != nil {
		return err
	}

	f, err := os.Create(absOutput)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}
