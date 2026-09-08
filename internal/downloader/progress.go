package downloader

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const barWidth = 20

type progressBar struct {
	total   int64
	written int64
	start   time.Time
	out     io.Writer

	lastRender  time.Time
	lastWritten int64
	rendered    bool
}

func newProgressBar(total int64, out io.Writer) *progressBar {
	return &progressBar{total: total, start: time.Now(), out: out}
}

func (p *progressBar) Write(b []byte) (int, error) {
	p.written += int64(len(b))

	now := time.Now()
	if now.Sub(p.lastRender) > 80*time.Millisecond || p.written == p.total {
		p.render()
		p.lastRender = now
	}
	return len(b), nil
}

func (p *progressBar) render() {
	elapsed := time.Since(p.start).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	speed := float64(p.written) / elapsed

	var percent float64
	var filled int
	if p.total > 0 {
		percent = float64(p.written) / float64(p.total) * 100
		filled = int(float64(barWidth) * float64(p.written) / float64(p.total))
		if filled > barWidth {
			filled = barWidth
		}
	}
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)

	writtenVal, writtenUnit := humanSizeBinary(float64(p.written))
	totalVal, totalUnit := humanSizeBinary(float64(p.total))
	speedVal, speedUnit := humanSizeBinary(speed)

	eta := "--"
	if p.total > 0 && speed > 0 {
		remaining := float64(p.total-p.written) / speed
		if remaining < 0 {
			remaining = 0
		}
		eta = formatDuration(remaining)
	}

	line := fmt.Sprintf(" %6.2f %s / %6.2f %s [%s] %6.2f%% %.2f %s/s %s",
		writtenVal, writtenUnit, totalVal, totalUnit, bar, percent, speedVal, speedUnit, eta)

	fmt.Fprint(p.out, "\r\033[2K"+line)
	p.lastWritten = p.written
	p.rendered = true
}

func (p *progressBar) finish() {
	if !p.rendered || p.lastWritten != p.written {
		p.render()
	}
	fmt.Fprintln(p.out)
}

func formatDuration(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	return fmt.Sprintf("%dm%ds", m, s)
}
