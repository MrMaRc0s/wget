package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"wget/internal/downloader"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "wget: missing URL")
		fmt.Fprintln(os.Stderr, "usage: wget <url>")
		os.Exit(1)
	}

	rawURL := args[0]
	dest := "./" + filenameFromURL(rawURL)

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
