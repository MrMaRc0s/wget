package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wget/internal/background"
	"wget/internal/downloader"
)

func main() {
	var (
		output    string
		outputDir string
		rateLimit string
		bg        bool
	)

	flag.StringVar(&output, "O", "", "save the file under a different name")
	flag.StringVar(&outputDir, "P", ".", "directory prefix where the file will be saved")
	flag.StringVar(&rateLimit, "rate-limit", "", "limit download speed, e.g. 200k or 2M")
	flag.BoolVar(&bg, "B", false, "download in the background, output redirected to wget-log")
	flag.Parse()

	if bg {
		if err := background.Relaunch(stripFlag(os.Args[1:], "-B")); err != nil {
			fmt.Fprintln(os.Stderr, "wget: "+err.Error())
			os.Exit(1)
		}
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "wget: missing URL")
		fmt.Fprintln(os.Stderr, "usage: wget [-O name] [-P path] [--rate-limit rate] <url>")
		os.Exit(1)
	}
	rawURL := args[0]

	limit, err := downloader.ParseRate(rateLimit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wget: "+err.Error())
		os.Exit(1)
	}

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
