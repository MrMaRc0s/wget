package downloader

import "fmt"

// humanSizeDecimal renders n bytes using decimal (SI) MB/GB units. This
// matches the "content size: N [~X.XXMB]" line wget prints up front,
// before anything has actually been transferred.
func humanSizeDecimal(n int64) string {
	const (
		mb = 1_000_000
		gb = 1_000_000_000
	)
	if n >= gb {
		return fmt.Sprintf("~%.2fGB", float64(n)/gb)
	}
	return fmt.Sprintf("~%.2fMB", float64(n)/mb)
}

// humanSizeBinary renders n bytes using binary KiB/MiB/GiB units, the
// convention used by the live progress bar.
func humanSizeBinary(n float64) (float64, string) {
	const (
		kib = 1024
		mib = 1024 * 1024
		gib = 1024 * 1024 * 1024
	)
	switch {
	case n >= gib:
		return n / gib, "GiB"
	case n >= mib:
		return n / mib, "MiB"
	default:
		return n / kib, "KiB"
	}
}
