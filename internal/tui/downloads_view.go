package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
)

// Download table rendering and data management

func (m *Model) visibleRows() []state.DownloadRow {
	rows := m.filterRows(m.activeTab)
	rows = m.applySearch(rows)
	rows = m.applySort(rows)
	return rows
}

func (m *Model) filterRows(tab int) []state.DownloadRow {
	var out []state.DownloadRow
	for _, r := range m.rows {
		if tab == -1 {
			out = append(out, r)
			continue
		}
		ls := strings.ToLower(r.Status)
		switch tab {
		case 0:
			if ls == "pending" || ls == "planning" || ls == "hold" {
				out = append(out, r)
			}
		case 1:
			if ls == "running" {
				out = append(out, r)
			}
		case 2:
			if ls == "complete" {
				out = append(out, r)
			}
		case 3:
			if ls == "error" || ls == "checksum_mismatch" || ls == "verify_failed" {
				out = append(out, r)
			}
		}
	}
	return out
}

func (m *Model) applySearch(in []state.DownloadRow) []state.DownloadRow {
	q := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if q == "" {
		return in
	}
	var out []state.DownloadRow
	for _, r := range in {
		if strings.Contains(strings.ToLower(r.URL), q) || strings.Contains(strings.ToLower(r.Dest), q) {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) applySort(in []state.DownloadRow) []state.DownloadRow {
	if m.sortMode == "" {
		return in
	}
	out := make([]state.DownloadRow, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		ci, ti, ri, _ := m.progressFor(out[i])
		cj, tj, rj, _ := m.progressFor(out[j])
		etaI := etaSeconds(ci, ti, ri)
		etaJ := etaSeconds(cj, tj, rj)
		switch m.sortMode {
		case "speed":
			return ri > rj
		case "eta":
			if etaI == 0 && etaJ == 0 {
				return ri > rj
			}
			if etaI == 0 {
				return false
			}
			if etaJ == 0 {
				return true
			}
			return etaI < etaJ
		case "rem":
			// Sort by remaining bytes ascending (unknown totals last)
			remI := int64(1 << 62)
			remJ := int64(1 << 62)
			if ti > 0 {
				remI = ti - ci
			}
			if tj > 0 {
				remJ = tj - cj
			}
			if remI == remJ {
				// Tie-breaker by higher rate
				return ri > rj
			}
			return remI < remJ
		}
		return false
	})
	return out
}

func (m *Model) renderTable() string {
	rows := m.visibleRows()
	var sb strings.Builder
	lastLabel := "DEST"
	switch m.columnMode {
	case "url":
		lastLabel = "URL"
	case "host":
		lastLabel = "HOST"
	}
	speedLabel := "SPEED"
	etaLabel := "ETA"
	if m.sortMode == "speed" {
		speedLabel = speedLabel + "*"
	}
	if m.sortMode == "eta" {
		etaLabel = etaLabel + "*"
	}
	if m.isCompact() {
		hdr := m.th.head.Render(fmt.Sprintf("%-1s %-8s %-3s  %-16s  %-4s  %-12s  %-8s  %-s", "S", "STATUS", "RT", "PROG", "PCT", "SRC", etaLabel, lastLabel))
		if m.sortMode == "rem" {
			hdr = hdr + "  [sort: remaining]"
		}
		sb.WriteString(hdr)
	} else {
		hdr := m.th.head.Render(fmt.Sprintf("%-1s %-8s %-3s  %-16s  %-4s  %-10s  %-10s  %-12s  %-8s  %-s", "S", "STATUS", "RT", "PROG", "PCT", speedLabel, "THR", "SRC", etaLabel, lastLabel))
		if m.sortMode == "rem" {
			hdr = hdr + "  [sort: remaining]"
		}
		sb.WriteString(hdr)
	}
	sb.WriteString("\n")
	maxRows := m.h - 10
	if maxRows < 3 {
		maxRows = len(rows)
	}
	var prevGroup string
	for i, r := range rows {
		if m.groupBy == "host" {
			host := hostOf(r.URL)
			if i == 0 || host != prevGroup {
				sb.WriteString(m.th.label.Render("// "+host) + "\n")
				prevGroup = host
			}
		}
		// Progress and pct
		prog := m.renderProgress(r)
		cur, total, rate, _ := m.progressFor(r)
		pct := "--%"
		if total > 0 {
			ratio := float64(cur) / float64(total)
			if ratio < 0 {
				ratio = 0
			}
			if ratio > 1 {
				ratio = 1
			}
			pct = fmt.Sprintf("%3.0f%%", ratio*100)
		}
		// For completed jobs, force 100%
		if strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
			pct = "100%"
		}
		eta := m.etaCache[keyFor(r)]
		thr := m.renderSparkline(keyFor(r))
		sel := " "
		if m.selectedKeys[keyFor(r)] {
			sel = "*"
		}
		// status label with transient retrying overlay
		statusLabel := r.Status
		if strings.EqualFold(statusLabel, "hold") && strings.Contains(strings.ToLower(strings.TrimSpace(r.LastError)), "rate limited") {
			statusLabel = "hold(rl)"
		}
		if ts, ok := m.retrying[keyFor(r)]; ok {
			if time.Since(ts) < 4*time.Second {
				statusLabel = "retrying"
			} else {
				delete(m.retrying, keyFor(r))
			}
		}
		last := r.Dest
		switch m.columnMode {
		case "url":
			last = logging.SanitizeURL(r.URL)
		case "host":
			last = hostOf(r.URL)
		}
		src := hostOf(r.URL)
		src = truncateMiddle(src, 12)
		lw := m.lastColumnWidth(m.isCompact())
		last = truncateMiddle(last, lw)
		// Speed column value
		speedStr := humanize.Bytes(uint64(rate)) + "/s"
		if strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
			// Show average speed for completed
			if r.Size > 0 && r.UpdatedAt > 0 && r.CreatedAt > 0 && r.UpdatedAt >= r.CreatedAt {
				dur := time.Duration(r.UpdatedAt-r.CreatedAt) * time.Second
				if dur > 0 {
					avg := float64(r.Size) / dur.Seconds()
					speedStr = humanize.Bytes(uint64(avg)) + "/s"
				}
			} else {
				speedStr = "-"
			}
		}
		var line string
		if m.isCompact() {
			line = fmt.Sprintf("%-1s %-8s %-3d  %-16s  %-4s  %-12s  %-8s  %s", sel, statusLabel, r.Retries, prog, pct, src, eta, last)
		} else {
			line = fmt.Sprintf("%-1s %-8s %-3d  %-16s  %-4s  %-10s  %-10s  %-12s  %-8s  %s", sel, statusLabel, r.Retries, prog, pct, speedStr, thr, src, eta, last)
		}
		if i == m.selected {
			line = m.th.rowSelected.Render(line)
		}
		sb.WriteString(line + "\n")
		if i+1 >= maxRows {
			break
		}
	}
	if len(rows) == 0 {
		sb.WriteString(m.th.label.Render("(no items)"))
	}
	return sb.String()
}

func (m *Model) renderInspector() string {
	rows := m.visibleRows()
	if m.selected < 0 || m.selected >= len(rows) {
		return m.th.label.Render("No selection")
	}
	r := rows[m.selected]
	var sb strings.Builder
	sb.WriteString(m.th.head.Render("Details"))
	sb.WriteString("\n")
	sb.WriteString(m.th.label.Render("URL:"))
	sb.WriteString("\n")
	sb.WriteString(logging.SanitizeURL(r.URL))
	sb.WriteString("\n\n")
	sb.WriteString(m.th.label.Render("Dest:"))
	sb.WriteString("\n")
	sb.WriteString(r.Dest)
	sb.WriteString("\n\n")
	// Basic metrics
	cur := m.curBytesCache[keyFor(r)]
	total := m.totalCache[keyFor(r)]
	rate := m.rateCache[keyFor(r)]
	eta := m.etaCache[keyFor(r)]
	sb.WriteString(fmt.Sprintf("%s %s/%s\n", m.th.label.Render("Progress:"), humanize.Bytes(uint64(cur)), humanize.Bytes(uint64(total))))
	sb.WriteString(fmt.Sprintf("%s %s/s\n", m.th.label.Render("Speed:"), humanize.Bytes(uint64(rate))))
	sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("ETA:"), eta))
	sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Throughput:"), m.renderSparkline(keyFor(r))))
	// Retries and status (with transient retrying overlay)
	statusLabel := r.Status
	if ts, ok := m.retrying[keyFor(r)]; ok {
		if time.Since(ts) < 4*time.Second {
			statusLabel = "retrying"
		} else {
			delete(m.retrying, keyFor(r))
		}
	}
	sb.WriteString(fmt.Sprintf("%s %d\n", m.th.label.Render("Retries:"), r.Retries))
	sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Status:"), statusLabel))
	// Completed job duration and average speed
	if strings.EqualFold(strings.TrimSpace(r.Status), "complete") && r.CreatedAt > 0 && r.UpdatedAt >= r.CreatedAt {
		dur := time.Duration((r.UpdatedAt - r.CreatedAt)) * time.Second
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Duration:"), dur.String()))
		// Start/End wall times
		startAt := time.Unix(r.CreatedAt, 0).Local().Format("2006-01-02 15:04:05")
		endAt := time.Unix(r.UpdatedAt, 0).Local().Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Started:"), startAt))
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Finished:"), endAt))
		if r.Size > 0 && dur > 0 {
			avg := float64(r.Size) / dur.Seconds()
			sb.WriteString(fmt.Sprintf("%s %s/s\n", m.th.label.Render("Avg Speed:"), humanize.Bytes(uint64(avg))))
		}
	} else if strings.EqualFold(strings.TrimSpace(r.Status), "running") && r.CreatedAt > 0 {
		// Show start time for running jobs
		startAt := time.Unix(r.CreatedAt, 0).Local().Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Started:"), startAt))
	}
	// Verification details
	if strings.TrimSpace(r.ExpectedSHA256) != "" || strings.TrimSpace(r.ActualSHA256) != "" {
		sb.WriteString(m.th.label.Render("SHA256:"))
		sb.WriteString("\n")
		if strings.TrimSpace(r.ExpectedSHA256) != "" {
			sb.WriteString(fmt.Sprintf("expected: %s\n", r.ExpectedSHA256))
		}
		if strings.TrimSpace(r.ActualSHA256) != "" {
			sb.WriteString(fmt.Sprintf("actual:   %s\n", r.ActualSHA256))
		}
		if r.ExpectedSHA256 != "" && r.ActualSHA256 != "" {
			if strings.EqualFold(strings.TrimSpace(r.ExpectedSHA256), strings.TrimSpace(r.ActualSHA256)) {
				sb.WriteString(m.th.ok.Render("verified: OK") + "\n")
			} else {
				sb.WriteString(m.th.bad.Render("verified: MISMATCH") + "\n")
			}
		}
	}
	// Show reason for hold/error if available
	if strings.TrimSpace(r.LastError) != "" {
		sb.WriteString(fmt.Sprintf("%s %s\n", m.th.label.Render("Reason:"), r.LastError))
	}
	return sb.String()
}

func (m *Model) progressFor(r state.DownloadRow) (cur int64, total int64, rate float64, eta string) {
	key := keyFor(r)
	cur = m.curBytesCache[key]
	total = m.totalCache[key]
	// Smooth rate using recent positive samples; fallback to last instantaneous
	rate = m.smoothedRate(key)
	eta = m.etaCache[key]
	// Fallback if cache not populated yet
	if total == 0 && r.Size > 0 {
		total = r.Size
	}
	return
}

// smoothedRate returns a moving average of recent positive samples (up to 5),
// falling back to the last instantaneous rate if no positive samples exist.

func (m *Model) smoothedRate(key string) float64 {
	h := m.rateHist[key]
	if len(h) == 0 {
		return m.rateCache[key]
	}
	sum := 0.0
	count := 0
	for i := len(h) - 1; i >= 0 && count < 5; i-- {
		v := h[i]
		if v > 0 {
			sum += v
			count++
		}
	}
	if count > 0 {
		return sum / float64(count)
	}
	return m.rateCache[key]
}

func (m *Model) computeCurAndTotal(r state.DownloadRow) (cur int64, total int64) {
	total = r.Size
	chunks, _ := m.st.ListChunks(r.URL, r.Dest)
	if len(chunks) > 0 {
		for _, c := range chunks {
			if strings.EqualFold(c.Status, "complete") {
				cur += c.Size
			}
		}
		return cur, total
	}
	if r.Dest != "" {
		p := downloader.StagePartPath(m.cfg, r.URL, r.Dest)
		if st, err := os.Stat(p); err == nil {
			cur = st.Size()
		} else if st, err := os.Stat(r.Dest); err == nil {
			cur = st.Size()
		}
	}
	return cur, total
}

func (m *Model) renderProgress(r state.DownloadRow) string {
	// For completed jobs, render full progress
	if strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
		return m.prog.ViewAs(1)
	}
	cur, total, _, _ := m.progressFor(r)
	if total <= 0 {
		return "--"
	}
	ratio := float64(cur) / float64(total)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return m.prog.ViewAs(ratio)
}

func (m *Model) addRateSample(key string, rate float64) {
	h := m.rateHist[key]
	h = append(h, rate)
	if len(h) > 10 {
		h = h[len(h)-10:]
	}
	m.rateHist[key] = h
}

func (m *Model) renderSparklineKey(key string) string {
	h := m.rateHist[key]
	if len(h) == 0 {
		return ""
	}
	// map rates to 8 levels; normalize by max
	max := 0.0
	for _, v := range h {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return "──────────"
	}
	levels := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sb strings.Builder
	// Oldest on the left, newest on the right (rightward growth)
	for _, v := range h {
		r := int((v/max)*float64(len(levels)-1) + 0.5)
		if r < 0 {
			r = 0
		}
		if r >= len(levels) {
			r = len(levels) - 1
		}
		sb.WriteRune(levels[r])
	}
	// pad to 10
	for sb.Len() < 10 {
		sb.WriteRune(' ')
	}
	return sb.String()
}

func (m *Model) renderSparkline(key string) string { return m.renderSparklineKey(key) }

func (m *Model) lastColumnWidth(compact bool) int {
	// Without side panels, use nearly full width minus borders
	usable := m.w - 2*2 // borders on left/right wrappers
	if usable < 40 {
		usable = 40
	}
	if compact {
		// S(1), space(1), STATUS(8), space(1), RT(3), 2sp, PROG(16), 2sp, PCT(4), 2sp, SRC(12), 2sp, ETA(8), 2sp
		consumed := 1 + 1 + 8 + 1 + 3 + 2 + 16 + 2 + 4 + 2 + 12 + 2 + 8 + 2
		lw := usable - consumed
		if lw < 10 {
			lw = 10
		}
		return lw
	}
	// non-compact consumed widths: + SPEED(10) + THR(10) before SRC
	consumed := 1 + 1 + 8 + 1 + 3 + 2 + 16 + 2 + 4 + 2 + 10 + 2 + 10 + 2 + 12 + 2 + 8 + 2
	lw := usable - consumed
	if lw < 10 {
		lw = 10
	}
	return lw
}

func (m *Model) maxRowsOnScreen() int {
	max := m.h - 10
	if max < 3 {
		return 3
	}
	return max
}
