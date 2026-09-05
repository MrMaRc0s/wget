// Package mirror implements the --mirror flag: crawl a website starting
// from a URL, saving every same-host page and asset it can reach through
// <a href>, <link href> and <img src> references (plus url(...) in CSS)
// into a folder named after the site's domain.
package mirror

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxConcurrentFetches bounds how many pages/assets are fetched at once.
// Sequential crawling is fine for a handful of pages but noticeably slow
// on larger sites, so a small worker pool is used instead - the same
// asynchronous spirit as -i's batch downloads, just bounded so a big
// site doesn't open hundreds of connections at once.
const maxConcurrentFetches = 8

// errSkip marks a reference that resolveLink deliberately will not turn
// into a URL (an anchor, mailto:, javascript:, etc).
var errSkip = errors.New("not a mirrorable link")

// Options configures a mirror run.
type Options struct {
	RootURL      string
	Reject       []string // file extensions to skip, without a leading dot
	Exclude      []string // URL path prefixes to skip, e.g. "/assets"
	ConvertLinks bool
}

// Mirror crawls a single site, tracking what has already been visited so
// nothing is fetched twice.
type Mirror struct {
	opts   Options
	client *http.Client

	rootDir  string
	baseHost string

	mu      sync.Mutex
	visited map[string]bool
}

func New(opts Options) *Mirror {
	reject := make([]string, len(opts.Reject))
	for i, r := range opts.Reject {
		reject[i] = strings.ToLower(strings.TrimPrefix(r, "."))
	}
	opts.Reject = reject

	return &Mirror{
		opts:    opts,
		client:  &http.Client{Timeout: 30 * time.Second},
		visited: make(map[string]bool),
	}
}

// Run crawls the site with a bounded pool of concurrent fetches, starting
// from the root URL and following every same-host reference discovered
// along the way, until nothing new is left to visit.
func (m *Mirror) Run() error {
	root, err := url.Parse(m.opts.RootURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if root.Scheme == "" {
		root.Scheme = "https"
	}

	// The root fetch runs alone, synchronously, before any worker starts:
	// it is what sets baseHost/rootDir (from the possibly-redirected
	// response URL), and every filtering decision downstream depends on
	// baseHost already being correct.
	m.visited[root.String()] = true
	links, err := m.fetchAndSave(root, true)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", root, err)
	}

	sem := make(chan struct{}, maxConcurrentFetches)
	var wg sync.WaitGroup

	var crawl func(u *url.URL)
	crawl = func(u *url.URL) {
		defer wg.Done()

		sem <- struct{}{}
		links, err := m.fetchAndSave(u, false)
		<-sem

		if err != nil {
			fmt.Fprintf(os.Stderr, "wget: skipping %s: %v\n", u, err)
			return
		}
		m.enqueueNew(links, &wg, crawl)
	}

	m.enqueueNew(links, &wg, crawl)
	wg.Wait()

	fmt.Printf("mirror saved to ./%s\n", m.rootDir)
	return nil
}

// enqueueNew marks each not-yet-seen link as visited and spawns a worker
// for it. Deduplication has to happen here, under the lock, rather than
// inside crawl itself - otherwise two workers discovering the same link
// at the same time could both win the race and fetch it twice.
func (m *Mirror) enqueueNew(links []*url.URL, wg *sync.WaitGroup, crawl func(*url.URL)) {
	for _, link := range links {
		key := link.String()

		m.mu.Lock()
		seen := m.visited[key]
		m.visited[key] = true
		m.mu.Unlock()

		if seen {
			continue
		}
		wg.Add(1)
		go crawl(link)
	}
}

func (m *Mirror) sameHost(u *url.URL) bool {
	return u.Host == m.baseHost
}

func (m *Mirror) isRejected(u *url.URL) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(u.Path), "."))
	if ext == "" {
		return false
	}
	for _, r := range m.opts.Reject {
		if ext == r {
			return true
		}
	}
	return false
}

func (m *Mirror) isExcluded(u *url.URL) bool {
	for _, prefix := range m.opts.Exclude {
		if prefix != "" && strings.HasPrefix(u.Path, prefix) {
			return true
		}
	}
	return false
}

// shouldMirror decides whether a discovered link is worth saving at all:
// same scheme family, same host as the site being mirrored, and not
// filtered out by --reject or --exclude.
func (m *Mirror) shouldMirror(u *url.URL) bool {
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if !m.sameHost(u) {
		return false
	}
	if m.isExcluded(u) || m.isRejected(u) {
		return false
	}
	return true
}

// fetchAndSave downloads u, writes it under the mirror's root directory,
// and returns the links discovered inside it (empty for anything that
// isn't HTML or CSS). On the very first call it also fixes the mirror's
// base host and root directory name from the (possibly redirected)
// response URL, so a root that redirects across hosts still works.
func (m *Mirror) fetchAndSave(u *url.URL, isRoot bool) ([]*url.URL, error) {
	resp, err := m.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

	if isRoot {
		m.baseHost = resp.Request.URL.Host
		m.rootDir = m.baseHost
	}

	localPath := m.localPath(u)
	fullPath := filepath.Join(m.rootDir, localPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	ext := strings.ToLower(filepath.Ext(u.Path))
	isHTML := strings.Contains(contentType, "text/html") || ext == ".html" || ext == ".htm" || ext == ""
	isCSS := strings.Contains(contentType, "text/css") || ext == ".css"

	if !isHTML && !isCSS {
		file, err := os.Create(fullPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		if _, err := io.Copy(file, resp.Body); err != nil {
			return nil, err
		}
		fmt.Printf("saved %s\n", fullPath)
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var newContent string
	var links []*url.URL
	if isHTML {
		newContent, links = m.processHTML(string(body), u, localPath)
	} else {
		newContent, links = m.processCSS(string(body), u, localPath)
	}

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return nil, err
	}
	fmt.Printf("saved %s\n", fullPath)
	return links, nil
}

// localPath maps a URL's path to a path relative to the mirror's root
// directory. A trailing slash (or empty path) becomes index.html;
// otherwise a path with no file extension is assumed to be an HTML page
// and gets one appended, since it will be saved as a static file.
func (m *Mirror) localPath(u *url.URL) string {
	p := u.Path
	if p == "" || strings.HasSuffix(p, "/") {
		p += "index.html"
	} else if filepath.Ext(p) == "" {
		p += ".html"
	}
	return strings.TrimPrefix(p, "/")
}
