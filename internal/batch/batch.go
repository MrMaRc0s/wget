// Package batch implements the -i flag: download every URL listed in a
// file concurrently.
package batch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"wget/internal/downloader"
)

type result struct {
	url  string
	name string
	err  error
}

// DownloadAll fetches every URL in urls at the same time, saving each
// into outputDir. Sizes are reported up front (one HEAD request per URL,
// also done concurrently) and "finished" lines print in completion
// order, which is what makes the concurrency visible.
func DownloadAll(urls []string, outputDir string, rateLimit int64) {
	sizes := make([]int64, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			sizes[i] = contentLength(u)
		}(i, u)
	}
	wg.Wait()
	fmt.Printf("content size: %v\n", sizes)

	results := make(chan result, len(urls))
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			name := filepath.Base(u)
			err := downloader.Download(downloader.Options{
				URL:          u,
				Destination:  filepath.Join(outputDir, name),
				RateLimit:    rateLimit,
				ShowProgress: false,
				Out:          io.Discard,
			})
			results <- result{url: u, name: name, err: err}
		}(u)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "error downloading %s: %v\n", r.url, r.err)
			continue
		}
		fmt.Printf("finished %s\n", r.name)
	}

	fmt.Printf("\nDownload finished: %v\n", urls)
}

func contentLength(u string) int64 {
	resp, err := http.Head(u)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	return resp.ContentLength
}
