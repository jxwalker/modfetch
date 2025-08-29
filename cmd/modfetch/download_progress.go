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
		// smoothing window of recent samples (bytes, time)
		type sample struct{ t time.Time; b int64 }
		var win []sample
		var lastNonZeroRate float64
		var lastNonZeroAt time.Time
		var rows []state.DownloadRow
		for {
			select {
			case <-stop:
				fmt.Fprint(os.Stderr, "\r")
				return
			case <-time.After(250 * time.Millisecond):
				// Fetch total size from downloads table
				total := int64(0)
				rows, _ = st.ListDownloads()
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
				// Rate (smoothed) and ETA
				now := time.Now()
				dt := now.Sub(lastT).Seconds()
				if dt <= 0 { dt = 1 }
				delta := completed - lastCompleted
				if delta < 0 { delta = 0 }
				lastCompleted = completed
				lastT = now
				win = append(win, sample{t: now, b: completed})
				// drop samples older than 5s
				cut := now.Add(-5 * time.Second)
				for len(win) > 1 && win[0].t.Before(cut) { win = win[1:] }
				// compute smoothed rate
				var rate float64
				if len(win) >= 2 {
					span := win[len(win)-1].t.Sub(win[0].t).Seconds()
					bytes := win[len(win)-1].b - win[0].b
					if span > 0 && bytes > 0 {
						rate = float64(bytes) / span
					}
				}
				if rate > 0 { lastNonZeroRate = rate; lastNonZeroAt = now }
				// Use last non-zero for brief stalls to avoid flicker
				if rate <= 0 && time.Since(lastNonZeroAt) < 2*time.Second {
					rate = lastNonZeroRate
				}
				eta := "-"
				if rate > 0 && total > 0 && completed < total {
					rem := float64(total-completed) / rate
					eta = fmt.Sprintf("%ds", int(rem+0.5))
				}
				// Chunk status and retries
				chunks, _ = st.ListChunks(url, dest)
				cTotal := len(chunks)
				cDone := 0
				cRun := 0
				for _, c := range chunks {
					if strings.EqualFold(c.Status, "complete") { cDone++ }
					if strings.EqualFold(c.Status, "running") { cRun++ }
				}
				// Fetch retries from downloads row
				retries := int64(0)
				rows, _ := st.ListDownloads()
				for _, r := range rows {
					if r.URL == url && r.Dest == dest { retries = r.Retries; break }
				}
				// Bar
				bar := renderBar(completed, total, 30)
				fmt.Fprintf(os.Stderr, "\r%s %6.2f%%  %8s/s  ETA %s  %s/%s  C %d/%d A %d  R %d",
					bar,
					pct(completed, total),
					ifnz(rate, "-"),
					eta,
					humanize.Bytes(uint64(completed)),
					humanize.Bytes(uint64(max64(total, completed))),
					cDone, cTotal, cRun,
					retries,
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

