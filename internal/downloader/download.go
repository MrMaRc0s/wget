// Package downloader implements a single HTTP(S) file download with the
// status reporting, sizing, and progress feedback that wget prints for a
// plain `wget <url>` invocation.
package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

// Options configures a single download.
type Options struct {
	URL          string
	Destination  string
	RateLimit    int64 // bytes per second, 0 means unlimited
	ShowProgress bool
	Out          io.Writer
}

// Download fetches Options.URL and writes it to Options.Destination,
// printing wget-style status lines to Options.Out along the way.
func Download(opts Options) error {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	fmt.Fprintf(out, "start at %s\n", time.Now().Format(timeFormat))

	fmt.Fprint(out, "sending request, awaiting response... ")
	resp, err := http.Get(opts.URL)
	if err != nil {
		fmt.Fprintln(out)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Fprintf(out, "status %s\n", resp.Status)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with %s", resp.Status)
	}

	size := resp.ContentLength
	if size >= 0 {
		fmt.Fprintf(out, "content size: %d [%s]\n", size, humanSizeDecimal(size))
	} else {
		fmt.Fprintln(out, "content size: unknown")
	}

	fmt.Fprintf(out, "saving file to: %s\n", opts.Destination)

	if dir := filepath.Dir(opts.Destination); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("could not create directory: %w", err)
		}
	}

	file, err := os.Create(opts.Destination)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer file.Close()

	var body io.Reader = resp.Body
	if opts.RateLimit > 0 {
		body = newRateLimitedReader(body, opts.RateLimit)
	}

	if opts.ShowProgress {
		bar := newProgressBar(size, out)
		if _, err := io.Copy(file, io.TeeReader(body, bar)); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		bar.finish()
		fmt.Fprintln(out)
	} else {
		if _, err := io.Copy(file, body); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	}

	fmt.Fprintf(out, "Downloaded [%s]\n", opts.URL)
	fmt.Fprintf(out, "finished at %s\n", time.Now().Format(timeFormat))

	return nil
}
