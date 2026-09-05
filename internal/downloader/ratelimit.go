package downloader

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ParseRate parses a human-friendly rate string such as "200k" or "2M"
// into bytes per second. An empty string means no limit.
func ParseRate(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	multiplier := int64(1)
	numPart := s
	switch s[len(s)-1] {
	case 'k', 'K':
		multiplier = 1024
		numPart = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		numPart = s[:len(s)-1]
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
		numPart = s[:len(s)-1]
	}

	value, err := strconv.ParseFloat(numPart, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid rate limit %q", s)
	}

	return int64(value * float64(multiplier)), nil
}

// rateLimitedReader wraps an io.Reader so that its cumulative average
// throughput, measured from the first read, never exceeds limit bytes
// per second.
//
// An earlier version reset a byte counter every rolling second and slept
// once that counter crossed the limit. That let each window overshoot by
// however much the last read exceeded the threshold (bounded by one
// buffer size), and since the next window started its count back at
// zero, the overshoot was never paid back — measured throughput settled
// around 6-7% above the requested rate. Tracking total bytes against
// total elapsed time instead means any overshoot pushes the next sleep
// out further, so the average converges on the target rather than
// drifting past it.
type rateLimitedReader struct {
	r     io.Reader
	limit int64

	start time.Time
	sent  int64
}

func newRateLimitedReader(r io.Reader, limit int64) *rateLimitedReader {
	return &rateLimitedReader{r: r, limit: limit, start: time.Now()}
}

func (rl *rateLimitedReader) Read(p []byte) (int, error) {
	// Keep individual reads small so throttling reacts quickly instead of
	// letting a large buffer's worth of data through in one burst.
	chunk := rl.limit / 4
	if chunk < 1024 {
		chunk = 1024
	}
	if int64(len(p)) > chunk {
		p = p[:chunk]
	}

	n, err := rl.r.Read(p)
	if n > 0 {
		rl.sent += int64(n)
		idealElapsed := time.Duration(float64(rl.sent) / float64(rl.limit) * float64(time.Second))
		if actual := time.Since(rl.start); idealElapsed > actual {
			time.Sleep(idealElapsed - actual)
		}
	}
	return n, err
}
