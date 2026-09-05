package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wget/internal/background"
	"wget/internal/batch"
	"wget/internal/downloader"
	"wget/internal/mirror"
)

func main() {
	var (
		output       string
		outputDir    string
		rateLimit    string
		bg           bool
		inputFile    string
		mirrorSite   bool
		reject       string
		exclude      string
		convertLinks bool
	)

	flag.StringVar(&output, "O", "", "save the file under a different name")
	flag.StringVar(&outputDir, "P", ".", "directory prefix where the file will be saved")
	flag.StringVar(&rateLimit, "rate-limit", "", "limit download speed, e.g. 200k or 2M")
	flag.BoolVar(&bg, "B", false, "download in the background, output redirected to wget-log")
	flag.StringVar(&inputFile, "i", "", "file listing multiple URLs to download concurrently")
	flag.BoolVar(&mirrorSite, "mirror", false, "mirror an entire website")
	flag.StringVar(&reject, "R", "", "comma separated file suffixes to skip, e.g. jpg,gif")
	flag.StringVar(&reject, "reject", "", "comma separated file suffixes to skip, e.g. jpg,gif")
	flag.StringVar(&exclude, "X", "", "comma separated URL path prefixes to skip, e.g. /assets,/css")
	flag.StringVar(&exclude, "exclude", "", "comma separated URL path prefixes to skip, e.g. /assets,/css")
	flag.BoolVar(&convertLinks, "convert-links", false, "rewrite mirrored links for offline viewing")
	flag.Parse()

	if bg {
		if err := background.Relaunch(stripFlag(os.Args[1:], "-B")); err != nil {
			fmt.Fprintln(os.Stderr, "wget: "+err.Error())
			os.Exit(1)
		}
		return
	}

	limit, err := downloader.ParseRate(rateLimit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wget: "+err.Error())
		os.Exit(1)
	}

	if inputFile != "" {
		urls, err := readLines(inputFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wget: "+err.Error())
			os.Exit(1)
		}
		batch.DownloadAll(urls, outputDir, limit)
		return
	}

	args := flag.Args()

	if mirrorSite {
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "wget: --mirror requires a URL")
			os.Exit(1)
		}
		m := mirror.New(mirror.Options{
			RootURL:      args[0],
			Reject:       splitList(reject),
			Exclude:      splitList(exclude),
			ConvertLinks: convertLinks,
		})
		if err := m.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "wget: "+err.Error())
			os.Exit(1)
		}
		return
	}

	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "wget: missing URL")
		fmt.Fprintln(os.Stderr, "usage: wget [-O name] [-P path] [-B] [--rate-limit rate] <url>")
		fmt.Fprintln(os.Stderr, "       wget -i=<file>")
		fmt.Fprintln(os.Stderr, "       wget --mirror [-R exts] [-X paths] [--convert-links] <url>")
		os.Exit(1)
	}
	rawURL := args[0]

	name := output
	if name == "" {
		name = filenameFromURL(rawURL)
	}
	dest := joinDestination(outputDir, name)

	err = downloader.Download(downloader.Options{
		URL:          rawURL,
		Destination:  dest,
		RateLimit:    limit,
		ShowProgress: isTerminal(os.Stdout),
		Out:          os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "wget: "+err.Error())
		os.Exit(1)
	}
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "index.html"
	}
	name := filepath.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "index.html"
	}
	return name
}

// joinDestination builds the save path, expanding a leading "~" and
// keeping the "./name" form wget itself prints for the default directory.
func joinDestination(dir, name string) string {
	dir = expandHome(dir)
	if dir == "." || dir == "" {
		return "./" + name
	}
	return filepath.Join(dir, name)
}

// isTerminal reports whether f is an interactive terminal rather than a
// file, pipe, or redirect target. Downloads to a log file (as -B does)
// skip the carriage-return-driven progress bar, which only makes sense
// on a live screen.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func stripFlag(args []string, name string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == name {
			continue
		}
		out = append(out, a)
	}
	return out
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}
