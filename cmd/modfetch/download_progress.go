package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	"modfetch/internal/state"
)

// startProgressLoop prints a single-line progress bar with throughput and ETA while a download is running.
// It polls the state DB for chunk completion (chunked) or file size (.part) for single-stream.
// Call the returned stop() when the download ends.
func startProgressLoop(ctx context.Context, st *state.DB, url, dest string) func() {
	stop := make(chan struct{})
	var stopped atomic.Bool

	go func() {
		var lastCompleted int64
		var lastT = time.Now()
		for {
			select {
			case <-stop:
				fmt.Print("\r")
				return
			case <-time.After(250 * time.Millisecond):
				// Fetch total size from downloads table
				total := int64(0)
				rows, _ := st.ListDownloads()
				for _, r := range rows {
					if r.URL == url && r.Dest == dest { total = r.Size; break }
				}
				// Completed bytes
				completed := int64(0)
				chunks, _ := st.ListChunks(url, dest)
				if len(chunks) > 0 {
					for _, c := range chunks {
						if strings.EqualFold(c.Status, "complete") { completed += c.Size }
					}
				} else {
					// Single: read .part size if exists, else final
					if fi, err := os.Stat(dest + ".part"); err == nil { completed = fi.Size() } else if fi, err := os.Stat(dest); err == nil { completed = fi.Size() }
				}
				// Rate and ETA
				now := time.Now()
				dt := now.Sub(lastT).Seconds()
				var rate float64
				if dt > 0 {
					rate = float64(completed-lastCompleted) / dt
				}
				lastCompleted = completed
				lastT = now
				eta := "-"
				if rate > 0 && total > 0 && completed < total {
					rem := float64(total-completed) / rate
					eta = fmt.Sprintf("%ds", int(rem+0.5))
				}
				// Bar
				bar := renderBar(completed, total, 30)
				fmt.Printf("\r%s %6.2f%%  %8s/s  ETA %s  %s/%s",
					bar,
					pct(completed, total),
					ifnz(rate, "-"),
					eta,
					humanize.Bytes(uint64(completed)),
					humanize.Bytes(uint64(max64(total, completed))),
				)
			}
		}
	}()

	return func() {
		if stopped.CompareAndSwap(false, true) {
			close(stop)
		}
	}
}

func renderBar(completed, total int64, width int) string {
	if total <= 0 { total = 1 }
	ratio := float64(completed) / float64(total)
	if ratio < 0 { ratio = 0 }
	if ratio > 1 { ratio = 1 }
	filled := int(ratio * float64(width))
	if filled > width { filled = width }
	b := strings.Repeat("=", filled)
	if filled < width { b += ">" + strings.Repeat(" ", width-filled-1) } else if filled == width {
		b = strings.Repeat("=", width)
	}
	return "[" + b + "]"
}

func pct(a, b int64) float64 {
	if b <= 0 { return 0 }
	return float64(a) * 100 / float64(b)
}

func ifnz(v float64, def string) string {
	if v <= 0 { return def }
	return humanize.Bytes(uint64(v))
}

func max64(a, b int64) int64 { if a > b { return a }; return b }

