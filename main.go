package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"wget/internal/downloader"
)

func main() {
	var (
		output    string
		outputDir string
	)

	flag.StringVar(&output, "O", "", "save the file under a different name")
	flag.StringVar(&outputDir, "P", ".", "directory prefix where the file will be saved")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "wget: missing URL")
		fmt.Fprintln(os.Stderr, "usage: wget [-O name] [-P path] <url>")
		os.Exit(1)
	}
	rawURL := args[0]

	name := output
	if name == "" {
		name = filenameFromURL(rawURL)
	}
	dest := joinDestination(outputDir, name)

	err := downloader.Download(downloader.Options{
		URL:          rawURL,
		Destination:  dest,
		ShowProgress: true,
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
