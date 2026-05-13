package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
)

type benchResult struct {
	Tool        string  `json:"tool"`
	Status      string  `json:"status"`
	Bytes       int64   `json:"bytes"`
	Duration    float64 `json:"duration"`
	AvgBPS      float64 `json:"avg_bps"`
	Connections int     `json:"connections,omitempty"`
	ChunkSizeMB int     `json:"chunk_size_mb,omitempty"`
	RateLimited bool    `json:"rate_limited,omitempty"`
	Error       string  `json:"error,omitempty"`
	Dest        string  `json:"dest,omitempty"`
}

type benchSummary struct {
	URL      string        `json:"url"`
	WorkDir  string        `json:"work_dir,omitempty"`
	Duration float64       `json:"duration"`
	Results  []benchResult `json:"results"`
	Winner   string        `json:"winner,omitempty"`
}

func handleBench(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "emit benchmark results as JSON")
	urlFlag := fs.String("url", "", "HTTP URL or resolver URI to benchmark")
	toolsFlag := fs.String("tools", "modfetch,aria2", "comma-separated tools: modfetch,aria2")
	durationFlag := fs.Duration("duration", 15*time.Second, "sample duration per tool")
	profile := fs.String("profile", "", "download tuning profile: auto, default, or large-model")
	connections := fs.Int("connections", 0, "parallel range requests per file")
	chunkSizeMB := fs.Int("chunk-size-mb", 0, "range chunk size in MiB")
	keep := fs.Bool("keep", false, "keep temporary benchmark downloads")
	history := fs.Bool("history", false, "list persisted transfer benchmark history")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	if *history {
		return printBenchHistory(c, *common.jsonOut)
	}
	if strings.TrimSpace(*urlFlag) == "" {
		return errors.New("--url is required")
	}
	if *durationFlag <= 0 {
		return errors.New("--duration must be > 0")
	}
	if err := applyDownloadTuning(c, *profile, *connections, *chunkSizeMB); err != nil {
		return err
	}
	resolvedURL, headers, err := resolveBenchURL(ctx, c, *urlFlag)
	if err != nil {
		return err
	}
	log := logging.New(*common.logLevel, false)
	if decision, err := maybeApplyAdaptiveDownloadTuning(ctx, c, resolvedURL, headers, *profile, *connections, *chunkSizeMB); !*common.jsonOut {
		if err != nil && profileWantsAuto(*profile) {
			log.Warnf("adaptive tuning probe failed: %v", err)
		} else if err == nil && decision != nil && decision.Applied {
			log.Infof("adaptive tuning: %s", decision.Reason)
		} else if err == nil && decision != nil && profileWantsAuto(*profile) {
			log.Infof("adaptive tuning skipped: %s", decision.Reason)
		}
	}
	workDir, err := os.MkdirTemp("", "modfetch-bench.*")
	if err != nil {
		return err
	}
	if !*keep {
		defer func() { _ = os.RemoveAll(workDir) }()
	}
	results := make([]benchResult, 0, 2)
	for _, tool := range parseBenchTools(*toolsFlag) {
		switch tool {
		case "modfetch":
			results = append(results, runModfetchBench(ctx, c, log, resolvedURL, headers, workDir, *durationFlag))
		case "aria2":
			results = append(results, runAria2Bench(ctx, c, resolvedURL, headers, workDir, *durationFlag))
		default:
			results = append(results, benchResult{Tool: tool, Status: "error", Error: "unknown tool"})
		}
	}
	summary := benchSummary{
		URL:      logging.SanitizeURL(resolvedURL),
		Duration: durationFlag.Seconds(),
		Results:  results,
		Winner:   benchWinner(results),
	}
	if *keep {
		summary.WorkDir = workDir
	}
	if err := recordBenchHistory(c, resolvedURL, results); err != nil && !*common.jsonOut {
		log.Warnf("benchmark history was not saved: %v", err)
	}
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			return err
		}
		if !hasSuccessfulBench(results) {
			return errors.New("no successful benchmarks")
		}
		return nil
	}
	fmt.Printf("Benchmark: %s\n", logging.SanitizeURL(resolvedURL))
	for _, r := range results {
		if r.Status == "error" {
			fmt.Printf("  %-8s error: %s\n", r.Tool, r.Error)
			continue
		}
		fmt.Printf("  %-8s %-9s %10s in %5.1fs  %10s/s\n",
			r.Tool, r.Status, humanize.Bytes(uint64(r.Bytes)), r.Duration, humanize.Bytes(uint64(r.AvgBPS)))
	}
	if summary.Winner != "" {
		fmt.Printf("Winner: %s\n", summary.Winner)
	}
	if *keep {
		fmt.Printf("Work dir: %s\n", workDir)
	}
	if !hasSuccessfulBench(results) {
		return errors.New("no successful benchmarks")
	}
	return nil
}

func hasSuccessfulBench(results []benchResult) bool {
	for _, result := range results {
		if result.Status != "error" {
			return true
		}
	}
	return false
}

func parseBenchTools(raw string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		tool := strings.ToLower(strings.TrimSpace(part))
		if tool == "" {
			continue
		}
		if _, ok := seen[tool]; ok {
			continue
		}
		seen[tool] = struct{}{}
		out = append(out, tool)
	}
	if len(out) == 0 {
		return []string{"modfetch"}
	}
	return out
}

func resolveBenchURL(ctx context.Context, c *config.Config, raw string) (string, map[string]string, error) {
	if isResolverURI(raw) {
		res, err := resolver.Resolve(ctx, raw, c)
		if err != nil {
			return "", nil, err
		}
		return res.URL, res.Headers, nil
	}
	headers := map[string]string{}
	headers = attachDirectProviderAuthHeaders(c, raw, headers, nil)
	return raw, headers, nil
}

func runModfetchBench(ctx context.Context, base *config.Config, log *logging.Logger, rawURL string, headers map[string]string, workDir string, duration time.Duration) benchResult {
	cfg := *base
	root := filepath.Join(workDir, "modfetch")
	cfg.General.DataRoot = filepath.Join(root, "data")
	cfg.General.DownloadRoot = filepath.Join(root, "downloads")
	cfg.General.PartialsRoot = filepath.Join(root, "partials")
	if err := os.MkdirAll(cfg.General.DownloadRoot, 0o755); err != nil {
		return benchResult{Tool: "modfetch", Status: "error", Error: err.Error()}
	}
	st, err := state.Open(&cfg)
	if err != nil {
		return benchResult{Tool: "modfetch", Status: "error", Error: err.Error()}
	}
	defer func() { _ = st.Close() }()
	dest := filepath.Join(cfg.General.DownloadRoot, "modfetch-bench.bin")
	sampleCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	start := time.Now()
	final, _, err := downloader.NewAuto(&cfg, log, st, nil).Download(sampleCtx, rawURL, dest, "", headers, true)
	elapsed := time.Since(start)
	bytes, chunkRows := completedChunkBytes(st, rawURL, dest)
	bytes = maxInt64(bytes, sampledPartBytes(&cfg, rawURL, dest, chunkRows))
	if final != "" {
		bytes = maxInt64(bytes, fileSize(final))
	}
	status := "sampled"
	errText := ""
	if err == nil {
		status = "complete"
	} else if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		status = "error"
		errText = err.Error()
	}
	connections := effectiveDownloadConnections(&cfg)
	chunkSizeMB := fixedChunkSizeMB(&cfg)
	rateLimited := false
	if history, ok, hErr := st.BestTransferHistory(downloader.HostFromURLForHistory(rawURL), "modfetch"); hErr == nil && ok {
		if history.Connections > 0 {
			connections = history.Connections
		}
		if history.ChunkSizeMB >= 0 {
			chunkSizeMB = history.ChunkSizeMB
		}
		rateLimited = history.RateLimited
	}
	return benchResult{
		Tool:        "modfetch",
		Status:      status,
		Bytes:       bytes,
		Duration:    elapsed.Seconds(),
		AvgBPS:      bytesPerSecond(bytes, elapsed),
		Connections: connections,
		ChunkSizeMB: chunkSizeMB,
		RateLimited: rateLimited,
		Error:       errText,
		Dest:        dest,
	}
}

func runAria2Bench(ctx context.Context, cfg *config.Config, rawURL string, headers map[string]string, workDir string, duration time.Duration) benchResult {
	aria2Path, err := exec.LookPath("aria2c")
	if err != nil {
		return benchResult{Tool: "aria2", Status: "error", Error: "aria2c not found on PATH"}
	}
	if hasSensitiveHeaders(headers) {
		return benchResult{Tool: "aria2", Status: "error", Error: "aria2 benchmark skipped: sensitive headers cannot be passed safely to aria2c"}
	}
	dir := filepath.Join(workDir, "aria2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return benchResult{Tool: "aria2", Status: "error", Error: err.Error()}
	}
	destName := "aria2-bench.bin"
	connections := effectiveDownloadConnections(cfg)
	chunkSize := fixedChunkSizeMB(cfg)
	if chunkSize <= 0 {
		chunkSize = autoChunkSizeMaxMB
	}
	args := []string{
		"--allow-overwrite=true",
		"--auto-file-renaming=false",
		"--continue=false",
		"--console-log-level=warn",
		"--summary-interval=0",
		"--dir", dir,
		"--out", destName,
		"--max-connection-per-server", strconv.Itoa(connections),
		"--split", strconv.Itoa(connections),
		"--min-split-size", fmt.Sprintf("%dM", chunkSize),
	}
	for k, v := range headers {
		args = append(args, "--header", k+": "+v)
	}
	args = append(args, rawURL)
	sampleCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(sampleCtx, aria2Path, args...)
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	dest := filepath.Join(dir, destName)
	bytes := fileSize(dest)
	status := "sampled"
	errText := ""
	if err == nil {
		status = "complete"
	} else if sampleCtx.Err() == nil {
		status = "error"
		errText = strings.TrimSpace(string(out))
		if errText == "" {
			errText = err.Error()
		}
	}
	return benchResult{
		Tool:        "aria2",
		Status:      status,
		Bytes:       bytes,
		Duration:    elapsed.Seconds(),
		AvgBPS:      bytesPerSecond(bytes, elapsed),
		Connections: connections,
		ChunkSizeMB: chunkSize,
		Error:       errText,
		Dest:        dest,
	}
}

func hasSensitiveHeaders(headers map[string]string) bool {
	for name, value := range headers {
		if strings.TrimSpace(value) == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "authorization", "proxy-authorization", "cookie", "x-api-key":
			return true
		}
	}
	return false
}

func completedChunkBytes(st *state.DB, rawURL, dest string) (int64, int) {
	if st == nil {
		return 0, 0
	}
	chunks, err := st.ListChunks(rawURL, dest)
	if err != nil {
		return 0, 0
	}
	var total int64
	for _, chunk := range chunks {
		if chunk.Status == "complete" {
			total += chunk.Size
		}
	}
	return total, len(chunks)
}

func fileSize(path string) int64 {
	if path == "" {
		return 0
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func sampledPartBytes(cfg *config.Config, rawURL, dest string, chunkRows int) int64 {
	part := downloader.StagePartPath(cfg, rawURL, dest)
	if chunkRows > 0 {
		return allocatedFileBytes(part)
	}
	return fileSize(part)
}

func allocatedFileBytes(path string) int64 {
	if path == "" {
		return 0
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Blocks > 0 {
		return st.Blocks * 512
	}
	return fi.Size()
}

func bytesPerSecond(bytes int64, elapsed time.Duration) float64 {
	if bytes <= 0 || elapsed <= 0 {
		return 0
	}
	return float64(bytes) / elapsed.Seconds()
}

func benchWinner(results []benchResult) string {
	var winner string
	var best float64
	for _, result := range results {
		if result.Status == "error" || result.AvgBPS <= best {
			continue
		}
		best = result.AvgBPS
		winner = result.Tool
	}
	return winner
}

func fixedChunkSizeMB(c *config.Config) int {
	if mode, size := effectiveChunkSize(c); mode == "fixed" {
		return size
	}
	return 0
}

func recordBenchHistory(c *config.Config, rawURL string, results []benchResult) error {
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	host := downloader.HostFromURLForHistory(rawURL)
	if host == "" {
		return nil
	}
	for _, result := range results {
		if result.Status == "error" {
			continue
		}
		status := result.Status
		if status == "" {
			status = "unknown"
		}
		if err := st.UpsertTransferHistory(state.TransferHistoryRow{
			Host:        host,
			Tool:        result.Tool,
			Connections: result.Connections,
			ChunkSizeMB: result.ChunkSizeMB,
			AvgBPS:      result.AvgBPS,
			RateLimited: result.RateLimited,
			LastStatus:  status,
			LastError:   result.Error,
		}); err != nil {
			return err
		}
	}
	return nil
}

func printBenchHistory(c *config.Config, jsonOut bool) error {
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	rows, err := st.ListTransferHistory()
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	for _, row := range rows {
		rateLimited := ""
		if row.RateLimited {
			rateLimited = "\trate_limited=true"
		}
		fmt.Printf("%s\t%s\tconnections=%d\tchunk=%dMiB\tavg=%s/s\tsamples=%d\tstatus=%s%s\n",
			row.Host, row.Tool, row.Connections, row.ChunkSizeMB, humanize.Bytes(uint64(row.AvgBPS)), row.Samples, row.LastStatus, rateLimited)
	}
	return nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
