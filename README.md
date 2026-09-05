# wget

A from-scratch, dependency-free reimplementation of a useful subset of
[GNU Wget](https://www.gnu.org/software/wget/), written in Go. Everything
here uses only the standard library.

## Building

```
go build -o wget .
```

or run directly with `go run . <args>`.

## Basic usage

```
$ ./wget https://pbs.twimg.com/media/EMtmPFLWkAA8CIS.jpg
start at 2017-10-14 03:46:06
sending request, awaiting response... status 200 OK
content size: 56370 [~0.06MB]
saving file to: ./EMtmPFLWkAA8CIS.jpg
 55.05 KiB / 55.05 KiB [==================================================] 100.00% 1.24 MiB/s 0s

Downloaded [https://pbs.twimg.com/media/EMtmPFLWkAA8CIS.jpg]
finished at 2017-10-14 03:46:07
```

The progress bar (KiB/MiB transferred, percentage, ETA) only prints to an
interactive terminal; redirected or backgrounded output gets the plain
status lines instead.

## Flags

| Flag | Purpose |
| --- | --- |
| `-O=name` | Save the file under `name` instead of the one derived from the URL |
| `-P=path` | Save into `path` instead of the current directory |
| `--rate-limit=300k` / `--rate-limit=2M` | Cap download speed (accepts a plain byte count, or `k`/`M`/`G` suffixes) |
| `-B` | Start the download detached in the background; output goes to `wget-log` |
| `-i=file` | Read multiple URLs from `file`, one per line, and download them all concurrently |
| `--mirror` | Download an entire site into a folder named after its domain |
| `-R=ext1,ext2` / `--reject=ext1,ext2` | With `--mirror`, skip files whose extension matches |
| `-X=/path1,/path2` / `--exclude=/path1,/path2` | With `--mirror`, skip anything under these URL paths |
| `--convert-links` | With `--mirror`, rewrite mirrored pages' links to point at the local copies |

## Mirroring a site

```
$ ./wget --mirror --convert-links https://example.com
```

Crawls same-host pages and assets reachable through `<a href>`,
`<link href>` and `<img src>` (plus `url(...)` in CSS, including inline
`<style>` blocks), saving everything under `./example.com/`. Fetches run
through a small worker pool so a site with a few hundred pages doesn't
take minutes to crawl one request at a time.

`--reject` and `--exclude` are applied before a link is ever fetched, so
excluded paths never even touch the filesystem. `--convert-links` only
rewrites references that actually get mirrored - anything off-site or
filtered out is left pointing at the original URL, same as real wget.

## Layout

```
main.go                    CLI: flag parsing and dispatch
internal/downloader/       single-file download, progress bar, rate limiting
internal/background/       -B: detached re-exec with output to a log file
internal/batch/            -i: concurrent downloads from a URL list
internal/mirror/           --mirror: crawler, link extraction/rewriting
```
