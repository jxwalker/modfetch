package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	neturl "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/sync/errgroup"

	mfarchive "github.com/jxwalker/modfetch/internal/archive"
	"github.com/jxwalker/modfetch/internal/batch"
	"github.com/jxwalker/modfetch/internal/classifier"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/metrics"
	"github.com/jxwalker/modfetch/internal/placer"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
	"github.com/jxwalker/modfetch/internal/storage"
	"github.com/jxwalker/modfetch/internal/util"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command provided")
	}

	// Global flags (parsed per subcommand to avoid hard defaults)
	cmd := args[0]
	switch cmd {
	case "config":
		return handleConfig(ctx, args[1:])
	case "status":
		return handleStatus(ctx, args[1:])
	case "download":
		return handleDownload(ctx, args[1:])
	case "place":
		return handlePlace(ctx, args[1:])
	case "verify":
		return handleVerify(ctx, args[1:])
	case "tui":
		return handleTUI(ctx, args[1:])
	case "version":
		fmt.Println(version)
		return nil
	case "completion":
		return handleCompletion(ctx, args[1:])
	case "hostcaps":
		return handleHostCaps(ctx, args[1:])
	case "clean":
		return handleClean(ctx, args[1:])
	case "dedupe":
		return handleDedupe(ctx, args[1:])
	case "batch":
		return handleBatch(ctx, args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func usage() {
	fmt.Println(strings.TrimSpace(`modfetch - robust model fetcher (skeleton)

Usage:
  modfetch <command> [flags]

Commands:
  config validate   Validate a YAML config file
  config print      Print the loaded config as JSON
  config wizard     Interactive TUI to generate a YAML config
  download          Download a file via direct URL or resolver URI (hf://, civitai://)
  status            Show download status (table or JSON)
  place             Place a file into configured app directories
  verify            Verify SHA256 of a file or all completed downloads
  tui               Open the interactive terminal dashboard
  batch import      Import URLs from a text file and produce a YAML batch
  version           Print version
  help              Show this help
  completion        Generate shell completion scripts (bash|zsh|fish)
  hostcaps          Manage host capability cache (list/clear)
  clean             Prune staged partials and other cached artifacts
  dedupe            Replace duplicate completed downloads with hardlinks or symlinks

Flags:
  --config PATH     Path to YAML config file (or MODFETCH_CONFIG env var; default: ~/.config/modfetch/config.yml)
  --log-level L     Log level: debug|info|warn|error (per command)
  --json            JSON log output (per command)
`))
}

func handleStatus(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	onlyErrors := fs.Bool("only-errors", false, "show only rows with error-like statuses (error, checksum_mismatch, verify_failed)")
	summary := fs.Bool("summary", false, "print totals and error count")
	duplicates := fs.Bool("duplicates", false, "show completed downloads with duplicate SHA256 content")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	_ = c // currently unused; reserved for future filters
	log := logging.New(*common.logLevel, *common.jsonOut)
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer func() { _ = st.SQL.Close() }()
	rows, err := st.ListDownloads()
	if err != nil {
		return err
	}
	if *duplicates {
		groups := duplicateDownloadGroups(rows)
		if *common.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(groups)
		}
		for _, group := range groups {
			log.Infof("duplicate sha256=%s count=%d", group.SHA256, len(group.Rows))
			for _, r := range group.Rows {
				log.Infof("  %s", r.Dest)
			}
		}
		if *summary {
			fmt.Printf("Summary: duplicate_groups=%d duplicate_files=%d\n", len(groups), countDuplicateFiles(groups))
		}
		return nil
	}
	if *onlyErrors {
		var filt []state.DownloadRow
		for _, r := range rows {
			ls := strings.ToLower(strings.TrimSpace(r.Status))
			if ls == "error" || ls == "checksum_mismatch" || ls == "verify_failed" {
				filt = append(filt, r)
			}
		}
		rows = filt
	}
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if *summary {
			return enc.Encode(map[string]any{"total": len(rows), "errors": countErrors(rows), "rows": rows})
		}
		return enc.Encode(rows)
	}
	for _, r := range rows {
		log.Infof("%s -> %s [%s] size=%d", logging.SanitizeURL(r.URL), r.Dest, r.Status, r.Size)
	}
	if *summary {
		fmt.Printf("Summary: total=%d errors=%d\n", len(rows), countErrors(rows))
	}
	return nil
}

type duplicateGroup struct {
	SHA256 string              `json:"sha256"`
	Rows   []state.DownloadRow `json:"rows"`
}

type dedupeResult struct {
	SHA256    string `json:"sha256"`
	Canonical string `json:"canonical"`
	Dest      string `json:"dest"`
	Action    string `json:"action"`
	Error     string `json:"error,omitempty"`
}

func handleDedupe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("dedupe", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	mode := fs.String("mode", "hardlink", "dedupe mode: hardlink|symlink")
	dryRun := fs.Bool("dry-run", false, "show changes without modifying files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	log := logging.New(*common.logLevel, *common.jsonOut)
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer func() { _ = st.SQL.Close() }()
	rows, err := st.ListDownloads()
	if err != nil {
		return err
	}
	results := dedupeDuplicateGroups(duplicateDownloadGroups(rows), strings.ToLower(strings.TrimSpace(*mode)), *dryRun)
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	for _, r := range results {
		if r.Error != "" {
			log.Warnf("dedupe %s: %s", r.Dest, r.Error)
			continue
		}
		log.Infof("dedupe %s: %s -> %s", r.Action, r.Dest, r.Canonical)
	}
	return nil
}

func handleConfig(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand required: validate | print")
	}
	sub := args[0]
	switch sub {
	case "validate":
		return configValidate(args[1:])
	case "print":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(c)
		})
	case "wizard":
		return handleConfigWizard(ctx, args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", sub)
	}
}

func configValidate(args []string) error {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	strict := fs.Bool("strict", false, "reject unknown config fields")
	if err := fs.Parse(args); err != nil {
		return err
	}
	resolvedCfgPath, err := resolveConfigPath(*common.configPath)
	if err != nil {
		return err
	}
	if err := loadConfigForValidation(resolvedCfgPath, *strict); err != nil {
		return err
	}
	log := logging.New(*common.logLevel, *common.jsonOut)
	log.Infof("config: valid")
	return nil
}

func loadConfigForValidation(path string, strict bool) error {
	var err error
	if strict {
		_, err = config.LoadStrict(path)
	} else {
		_, err = config.Load(path)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", path)
		}
		return err
	}
	return nil
}

func handleDownload(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs (errors only)")
	noResume := fs.Bool("no-resume", false, "do not resume; start fresh (delete any staged .part and chunk state)")
	url := fs.String("url", "", "HTTP URL to download (direct) or resolver URI (e.g. hf://owner/repo/path)")
	dest := fs.String("dest", "", "destination path (optional)")
	sha := fs.String("sha256", "", "expected SHA256 (optional)")
	shaFile := fs.String("sha256-file", "", "path to .sha256 or file containing expected hash (optional)")
	batchPath := fs.String("batch", "", "YAML batch file with download jobs")
	placeFlag := fs.Bool("place", false, "place files after successful download")
	summaryJSON := fs.Bool("summary-json", false, "print a JSON summary when a download completes")
	batchParallel := fs.Int("batch-parallel", 0, "max parallel downloads when using --batch (default: config concurrency per_host_requests)")
	dryRun := fs.Bool("dry-run", false, "plan only: resolve URL/URI and compute default destination; no download")
	forceSkip := fs.Bool("force", false, "skip SHA256 verification even if --sha256/--sha256-file is provided")
	noAuthPreflight := fs.Bool("no-auth-preflight", false, "skip auth preflight probe")
	extractFlag := fs.Bool("extract", false, "extract zip/tar/tar.gz/tgz/7z archives after download")
	extractDir := fs.String("extract-dir", "", "directory for extracted archives (default: archive path without extension)")
	quant := fs.String("quant", "", "HuggingFace quantization to download (e.g., Q4_K_M, fp16)")
	listQuants := fs.Bool("list-quants", false, "list available quantizations for HuggingFace URI and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*sha) == "" && strings.TrimSpace(*shaFile) != "" {
		v, perr := parseSHA256FromFile(*shaFile)
		if perr != nil {
			return fmt.Errorf("sha256-file: %v", perr)
		}
		*sha = v
	}
	if storage.IsS3URI(strings.TrimSpace(*dest)) && (*extractFlag || *placeFlag) {
		return errors.New("s3 destinations cannot be combined with --extract or --place")
	}
	// quiet forces log level to error unless JSON is requested
	if *quiet && !*common.jsonOut {
		*common.logLevel = "error"
	}
	log := logging.New(*common.logLevel, *common.jsonOut)
	st, err := state.Open(c)
	if err != nil {
		return err
	}
	defer func() { _ = st.SQL.Close() }()
	startWall := time.Now()
	// Metrics manager (Prometheus textfile), if enabled
	m := metrics.New(c)
	// Batch mode
	if *batchPath != "" {
		bf, err := batch.Load(*batchPath)
		if err != nil {
			return err
		}
		// Destination reservation to avoid two workers writing the same path concurrently
		var destMu sync.Mutex
		reserved := map[string]struct{}{}
		reserveSpecific := func(p string) bool {
			destMu.Lock()
			defer destMu.Unlock()
			if _, ok := reserved[p]; ok {
				return false
			}
			reserved[p] = struct{}{}
			return true
		}
		reserveUnique := func(dir, base, versionHint string) (string, error) {
			destMu.Lock()
			defer destMu.Unlock()
			base = util.SafeFileName(base)
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			// try plain
			p := filepath.Join(dir, base)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				if _, ok := reserved[p]; !ok {
					reserved[p] = struct{}{}
					return p, nil
				}
			}
			// try version
			if strings.TrimSpace(versionHint) != "" {
				cand := filepath.Join(dir, fmt.Sprintf("%s (v%s)%s", name, versionHint, ext))
				if _, err := os.Stat(cand); os.IsNotExist(err) {
					if _, ok := reserved[cand]; !ok {
						reserved[cand] = struct{}{}
						return cand, nil
					}
				}
			}
			for i := 2; ; i++ {
				cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
				if _, err := os.Stat(cand); os.IsNotExist(err) {
					if _, ok := reserved[cand]; !ok {
						reserved[cand] = struct{}{}
						return cand, nil
					}
				}
			}
		}
		parallel := c.Concurrency.PerHostRequests
		if *batchParallel > 0 {
			parallel = *batchParallel
		}
		if parallel < 1 {
			parallel = 1
		}

		type jobItem struct {
			idx int
			job batch.BatchJob
		}

		jobs := make(chan jobItem)
		g, gctx := errgroup.WithContext(ctx)
		var logMu sync.Mutex

		for i := 0; i < parallel; i++ {
			g.Go(func() error {
				dl := downloader.NewAuto(c, log, st, m)
				for {
					select {
					case <-gctx.Done():
						return gctx.Err()
					case it, ok := <-jobs:
						if !ok {
							return nil
						}
						job := it.job
						if delay, err := scheduleWindowDelay(time.Now(), job.ScheduleWindow); err != nil {
							return fmt.Errorf("job %d schedule_window: %w", it.idx, err)
						} else if delay > 0 {
							logMu.Lock()
							log.Infof("job %d: waiting %s for schedule window %s", it.idx, delay.Round(time.Second), job.ScheduleWindow)
							logMu.Unlock()
							timer := time.NewTimer(delay)
							select {
							case <-gctx.Done():
								timer.Stop()
								return gctx.Err()
							case <-timer.C:
							}
						}
						resolvedURL := job.URI
						headers := map[string]string{}
						destCandidate := strings.TrimSpace(job.Dest)
						if strings.HasPrefix(resolvedURL, "hf://") || strings.HasPrefix(resolvedURL, "civitai://") {
							res, err := resolver.Resolve(gctx, resolvedURL, c)
							if err != nil {
								return fmt.Errorf("job %d resolve: %w", it.idx, err)
							}
							resolvedURL = res.URL
							headers = res.Headers
							if destCandidate == "" && strings.HasPrefix(job.URI, "civitai://") && strings.TrimSpace(res.SuggestedFilename) != "" {
								if p, err := reserveUnique(c.General.DownloadRoot, res.SuggestedFilename, res.VersionID); err == nil {
									destCandidate = p
								} else {
									return fmt.Errorf("job %d dest reserve: %v", it.idx, err)
								}
							}
						}
						// If still no destCandidate, try probing to compute a safe filename and reserve it
						if destCandidate == "" {
							meta, _ := downloader.ProbeURL(gctx, c, resolvedURL, headers)
							base := strings.TrimSpace(meta.Filename)
							if base == "" {
								if meta.FinalURL != "" {
									base = util.URLPathBase(meta.FinalURL)
								} else {
									base = util.URLPathBase(resolvedURL)
								}
							}
							p, err := reserveUnique(c.General.DownloadRoot, base, "")
							if err != nil {
								return fmt.Errorf("job %d dest reserve: %v", it.idx, err)
							}
							destCandidate = p
						}
						// If user provided explicit dest, ensure single writer
						if job.Dest != "" {
							if ok := reserveSpecific(destCandidate); !ok {
								logMu.Lock()
								log.Warnf("skipping job %d: destination already reserved: %s", it.idx, destCandidate)
								logMu.Unlock()
								continue
							}
						}
						if storage.IsS3URI(destCandidate) && (job.Extract || *extractFlag || job.Place || *placeFlag) {
							return fmt.Errorf("job %d: s3 destinations cannot be combined with extract or place", it.idx)
						}
						// Allow global --force to skip expected SHA verification even if job specifies one
						expected := job.SHA256
						if *forceSkip {
							expected = ""
						}
						type downloadCandidate struct {
							url     string
							headers map[string]string
						}
						sources := append([]string{job.URI}, job.Mirrors...)
						candidates := make([]downloadCandidate, 0, len(sources))
						for _, source := range sources {
							source = strings.TrimSpace(source)
							if source == "" {
								continue
							}
							candidateURL, candidateHeaders, err := resolveBatchDownloadCandidate(gctx, c, source)
							if err != nil {
								logMu.Lock()
								log.Warnf("job %d: skipping source %q: %v", it.idx, logging.SanitizeURL(source), logging.SanitizeError(err))
								logMu.Unlock()
								continue
							}
							candidates = append(candidates, downloadCandidate{url: candidateURL, headers: candidateHeaders})
						}
						if len(candidates) == 0 {
							return fmt.Errorf("job %d: no valid download candidates", it.idx)
						}
						var final, sum string
						var err error
						baseNoResume := *noResume || c.General.AlwaysNoResume
						for attempt, candidate := range candidates {
							final, sum, err = dl.Download(gctx, candidate.url, destCandidate, expected, candidate.headers, baseNoResume || attempt > 0)
							if err == nil {
								if attempt > 0 {
									logMu.Lock()
									log.Infof("job %d: mirror succeeded after %d failed source(s): %s", it.idx, attempt, logging.SanitizeURL(candidate.url))
									logMu.Unlock()
								}
								break
							}
							if attempt < len(candidates)-1 {
								_ = st.DeleteDownloadsAndChunksByDest(destCandidate)
								next := logging.SanitizeURL(candidates[attempt+1].url)
								logMu.Lock()
								log.Warnf("job %d: source failed: %s (%v); trying next source: %s", it.idx, logging.SanitizeURL(candidate.url), logging.SanitizeError(err), next)
								logMu.Unlock()
							}
						}
						if err != nil {
							return fmt.Errorf("job %d download: %w", it.idx, err)
						}
						var extracted []string
						if job.Extract || *extractFlag {
							outDir := strings.TrimSpace(job.ExtractDir)
							if outDir == "" {
								outDir = strings.TrimSpace(*extractDir)
							}
							if outDir == "" {
								outDir = defaultExtractDir(final)
							}
							extracted, err = mfarchive.Extract(gctx, final, outDir)
							if err != nil {
								return fmt.Errorf("job %d extract: %w", it.idx, err)
							}
						}
						var placed []string
						if job.Place || *placeFlag {
							var err2 error
							placed, err2 = placer.Place(c, final, job.Type, job.Mode)
							if err2 != nil {
								return fmt.Errorf("job %d place: %w", it.idx, err2)
							}
						}
						logMu.Lock()
						log.Infof("downloaded: %s (sha256=%s)", final, sum)
						if len(extracted) > 0 {
							log.Infof("extracted: %d file(s)", len(extracted))
						}
						for _, p := range placed {
							log.Infof("placed: %s", p)
						}
						if *summaryJSON {
							enc := json.NewEncoder(os.Stdout)
							enc.SetIndent("", "  ")
							_ = enc.Encode(map[string]any{
								"url":       logging.SanitizeURL(resolvedURL),
								"dest":      final,
								"sha256":    sum,
								"placed":    placed,
								"extracted": extracted,
								"status":    "ok",
							})
						}
						logMu.Unlock()
					}
				}
			})
		}

		go func() {
			defer close(jobs)
			items := make([]jobItem, 0, len(bf.Jobs))
			for i, job := range bf.Jobs {
				items = append(items, jobItem{idx: i, job: job})
			}
			sort.SliceStable(items, func(i, j int) bool {
				return items[i].job.Priority > items[j].job.Priority
			})
			for _, item := range items {
				select {
				case <-gctx.Done():
					return
				case jobs <- item:
				}
			}
		}()

		if err := g.Wait(); err != nil {
			return err
		}
		return nil
	}

	resolvedURL := *url
	headers := map[string]string{}
	// Support CivitAI model page URLs by translating to civitai://model/{id}[?version=]
	if strings.HasPrefix(resolvedURL, "http://") || strings.HasPrefix(resolvedURL, "https://") {
		if u, err := neturl.Parse(resolvedURL); err == nil {
			h := strings.ToLower(u.Hostname())
			// CivitAI model page -> civitai://
			if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) >= 2 {
					modelID := parts[1]
					q := u.Query()
					ver := q.Get("modelVersionId")
					if ver == "" {
						ver = q.Get("version")
					}
					civ := "civitai://model/" + modelID
					if strings.TrimSpace(ver) != "" {
						civ += "?version=" + ver
					}
					log.Infof("normalized CivitAI page -> %s", civ)
					// Defer resolution; set resolver URI and let generic path handle it
					resolvedURL = civ
				}
			}
			// Hugging Face blob page -> hf://owner/repo/path?rev=...
			if hostIs(h, "huggingface.co") {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				// Expect /{owner}/{repo}/blob/{rev}/path...
				if len(parts) >= 5 && parts[2] == "blob" {
					owner := parts[0]
					repo := parts[1]
					rev := parts[3]
					filePath := strings.Join(parts[4:], "/")
					hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
					// Add quant parameter if specified
					if strings.TrimSpace(*quant) != "" {
						hf += "&quant=" + neturl.QueryEscape(*quant)
					}
					log.Infof("normalized HF blob -> %s", hf)
					resolvedURL = hf
				}
			}
		}
	}
	// Add quant parameter to hf:// URI if specified and not already present
	if strings.HasPrefix(resolvedURL, "hf://") && strings.TrimSpace(*quant) != "" {
		if !strings.Contains(resolvedURL, "quant=") {
			separator := "?"
			if strings.Contains(resolvedURL, "?") {
				separator = "&"
			}
			resolvedURL += separator + "quant=" + neturl.QueryEscape(*quant)
		}
	}

	// Handle --list-quants: resolve and display available quantizations
	if *listQuants {
		if !strings.HasPrefix(resolvedURL, "hf://") {
			return fmt.Errorf("--list-quants only supported for HuggingFace URIs (hf://owner/repo[/path])")
		}
		res, err := resolver.Resolve(ctx, resolvedURL, c)
		if err != nil {
			return fmt.Errorf("resolve quantizations: %w", err)
		}
		if len(res.AvailableQuantizations) == 0 {
			fmt.Println("No quantizations detected for this repository")
			return nil
		}
		fmt.Printf("Available quantizations for %s:\n\n", resolvedURL)
		for i, q := range res.AvailableQuantizations {
			selected := ""
			if q.Name == res.SelectedQuantization {
				selected = " (recommended)"
			}
			fmt.Printf("  [%2d] %-12s  %10s  %-12s%s\n",
				i+1, q.Name, humanize.Bytes(uint64(q.Size)), q.FileType, selected)
		}
		fmt.Printf("\nUse --quant=<name> to download a specific quantization\n")
		return nil
	}

	if strings.HasPrefix(resolvedURL, "hf://") || strings.HasPrefix(resolvedURL, "civitai://") {
		res, err := resolver.Resolve(ctx, resolvedURL, c)
		if err != nil {
			return err
		}
		resolvedURL = res.URL
		headers = res.Headers
	} else {
		// Attach auth headers for direct URLs when possible
		if u, err := neturl.Parse(resolvedURL); err == nil {
			h := strings.ToLower(u.Hostname())
			if hostIs(h, "civitai.com") && c.Sources.CivitAI.Enabled {
				if env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					} else {
						log.Warnf("CivitAI token env %s is not set; gated content will return 401. Export %s.", env, env)
					}
				}
			}
			if hostIs(h, "huggingface.co") && c.Sources.HuggingFace.Enabled {
				if env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					} else {
						log.Warnf("Hugging Face token env %s is not set; gated repos will return 401. Export %s and accept the repo license.", env, env)
					}
				}
			}
		}
	}
	// If dry-run, compute a default destination and print plan
	if *dryRun {
		candDest := strings.TrimSpace(*dest)
		resolverURI := resolvedURL
		if !strings.HasPrefix(resolverURI, "hf://") && !strings.HasPrefix(resolverURI, "civitai://") {
			resolverURI = *url
		}
		if candDest == "" && strings.HasPrefix(resolverURI, "civitai://") {
			if res2, err := resolver.Resolve(ctx, resolverURI, c); err == nil && strings.TrimSpace(res2.SuggestedFilename) != "" {
				if p, err := util.UniquePath(c.General.DownloadRoot, res2.SuggestedFilename, res2.VersionID); err == nil {
					candDest = p
				}
			}
		}
		if candDest == "" {
			base := util.URLPathBase(resolvedURL)
			candDest = filepath.Join(c.General.DownloadRoot, util.SafeFileName(base))
		}
		if *summaryJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{
				"resolver_uri": *url,
				"resolved_url": logging.SanitizeURL(resolvedURL),
				"default_dest": candDest,
			})
			return nil
		}
		fmt.Printf("Plan (dry-run)\n")
		fmt.Printf("  Resolver URI: %s\n", logging.SanitizeURL(*url))
		fmt.Printf("  Resolved URL: %s\n", logging.SanitizeURL(resolvedURL))
		fmt.Printf("  Default dest: %s\n", candDest)
		return nil
	}
	if !*noAuthPreflight && !c.Network.DisableAuthPreflight {
		reach, info := downloader.CheckReachable(ctx, c, resolvedURL, headers)
		if !reach {
			return fmt.Errorf("preflight failed: unreachable (%s)", info)
		}
		code := 0
		if sp := strings.Fields(info); len(sp) > 0 {
			_, _ = fmt.Sscanf(sp[0], "%d", &code)
		}
		if code == 401 || code == 403 {
			host := ""
			if u, _ := neturl.Parse(resolvedURL); u != nil {
				host = strings.ToLower(u.Hostname())
			}
			msg := info
			if strings.HasSuffix(host, "huggingface.co") {
				env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv)
				if env == "" {
					env = "HF_TOKEN"
				}
				msg = fmt.Sprintf("%s — set %s and ensure repo access/license accepted", info, env)
			} else if strings.HasSuffix(host, "civitai.com") {
				env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv)
				if env == "" {
					env = "CIVITAI_TOKEN"
				}
				msg = fmt.Sprintf("%s — set %s and ensure content is accessible", info, env)
			}
			return fmt.Errorf("preflight auth failed: %s", msg)
		}
	}
	// Prefer chunked downloader; it will fall back to single when needed
	// Progress display (disabled for JSON or quiet)
	var stopProg func()
	if !*common.jsonOut && !*quiet {
		candDest := strings.TrimSpace(*dest)
		// Determine the resolver URI (could have been normalized above)
		resolverURI := resolvedURL
		if !strings.HasPrefix(resolverURI, "hf://") && !strings.HasPrefix(resolverURI, "civitai://") {
			resolverURI = *url
		}
		if candDest == "" && strings.HasPrefix(resolverURI, "civitai://") {
			// If we resolved civitai:// earlier, try SuggestedFilename by resolving again cheaply
			if res2, err := resolver.Resolve(ctx, resolverURI, c); err == nil && strings.TrimSpace(res2.SuggestedFilename) != "" {
				if p, err := util.UniquePath(c.General.DownloadRoot, res2.SuggestedFilename, res2.VersionID); err == nil {
					candDest = p
				}
			}
		}
		if candDest == "" {
			base := util.URLPathBase(resolvedURL)
			candDest = filepath.Join(c.General.DownloadRoot, util.SafeFileName(base))
		}
		stopProg = startProgressLoop(ctx, st, c, resolvedURL, candDest)
	}
	// Prefer chunked downloader; it will fall back to single when needed
	dl := downloader.NewAuto(c, log, st, m)
	// If civitai:// and no explicit dest, use SuggestedFilename
	destArg := strings.TrimSpace(*dest)
	// Determine the resolver URI to use for civitai SuggestedFilename
	resolverURI2 := resolvedURL
	if !strings.HasPrefix(resolverURI2, "hf://") && !strings.HasPrefix(resolverURI2, "civitai://") {
		resolverURI2 = *url
	}
	if destArg == "" && strings.HasPrefix(resolverURI2, "civitai://") {
		if res2, err := resolver.Resolve(ctx, resolverURI2, c); err == nil && strings.TrimSpace(res2.SuggestedFilename) != "" {
			if p, err := util.UniquePath(c.General.DownloadRoot, res2.SuggestedFilename, res2.VersionID); err == nil {
				destArg = p
			}
		}
	}
	if storage.IsS3URI(destArg) && (*extractFlag || *placeFlag) {
		return errors.New("s3 destinations cannot be combined with --extract or --place")
	}
	expected := *sha
	if *forceSkip {
		expected = ""
	}
	final, sum, err := dl.Download(ctx, resolvedURL, destArg, expected, headers, (*noResume) || c.General.AlwaysNoResume)
	if stopProg != nil {
		stopProg()
	}
	if err != nil {
		return err
	}
	var extracted []string
	if *extractFlag {
		outDir := strings.TrimSpace(*extractDir)
		if outDir == "" {
			outDir = defaultExtractDir(final)
		}
		extracted, err = mfarchive.Extract(ctx, final, outDir)
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}
	}
	// Final summary
	fi, _ := os.Stat(final)
	if fi == nil && storage.IsS3URI(final) {
		if local, err := storage.StagingPath(c, final, resolvedURL); err == nil {
			fi, _ = os.Stat(local)
		}
	}
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	dur := time.Since(startWall).Seconds()
	if *summaryJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"url":       logging.SanitizeURL(resolvedURL),
			"dest":      final,
			"size":      size,
			"duration":  dur,
			"avg_bps":   float64(size) / dur,
			"sha256":    sum,
			"status":    "ok",
			"extracted": extracted,
		})
	} else if !*common.jsonOut && !*quiet {
		var rate string
		if dur > 0 && size > 0 {
			rate = humanize.Bytes(uint64(float64(size)/dur)) + "/s"
		} else {
			rate = "-"
		}
		fmt.Printf("\nDownloaded: %s\nDest: %s\nSHA256: %s\nSize: %s\nDuration: %.1fs  Avg: %s\n", logging.SanitizeURL(resolvedURL), final, sum, humanize.Bytes(uint64(size)), dur, rate)
	}
	log.Infof("downloaded: %s (sha256=%s)", final, sum)
	if len(extracted) > 0 {
		log.Infof("extracted: %d file(s)", len(extracted))
	}
	if *placeFlag {
		placed, err := placer.Place(c, final, "", "")
		if err != nil {
			return err
		}
		for _, p := range placed {
			log.Infof("placed: %s", p)
		}
	}
	return nil
}

func handlePlace(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("place", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	filePath := fs.String("path", "", "path to file to place")
	artType := fs.String("type", "", "artifact type override (optional)")
	mode := fs.String("mode", "", "placement mode override: symlink|hardlink|copy (optional)")
	dryRun := fs.Bool("dry-run", false, "print planned destinations only; do not modify files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *filePath == "" {
		return errors.New("--path is required")
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	log := logging.New(*common.logLevel, *common.jsonOut)
	if *dryRun {
		atype := *artType
		if atype == "" {
			atype = classifier.Detect(c, *filePath)
		}
		targets, err := placer.ComputeTargets(c, atype)
		if err != nil {
			return err
		}
		fmt.Printf("Would place %s (type=%s) to:\n", *filePath, atype)
		for _, t := range targets {
			fmt.Printf("  %s\n", t)
		}
		return nil
	}
	placed, err := placer.Place(c, *filePath, *artType, *mode)
	if err != nil {
		return err
	}
	for _, p := range placed {
		log.Infof("placed: %s", p)
	}
	return nil
}

func handleClean(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	days := fs.Int("days", 7, "remove .part files older than this many days (0 = remove all)")
	dryRun := fs.Bool("dry-run", false, "show what would be removed, but do not delete")
	destPath := fs.String("dest", "", "remove staged .part for this destination path (overrides days)")
	includeNext := fs.Bool("include-next-to-dest", true, "also scan download_root for *.part when stage_partials=false")
	sidecars := fs.Bool("sidecars", false, "also remove orphan .sha256 sidecar files (no matching base file)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	log := logging.New(*common.logLevel, *common.jsonOut)

	removed := 0
	skipped := 0
	sideRemoved := 0
	var errs []string

	// Helper to maybe remove a file
	removeFile := func(p string) {
		fi, err := os.Stat(p)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
			return
		}
		if *destPath == "" {
			// age-gated mode
			cutoff := time.Now().Add(-time.Duration(*days) * 24 * time.Hour)
			if *days > 0 && fi.ModTime().After(cutoff) {
				skipped++
				return
			}
		}
		if *dryRun {
			log.Infof("would remove: %s (age=%s)", p, time.Since(fi.ModTime()).Round(time.Second))
			removed++
			return
		}
		if err := os.Remove(p); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p, err))
			return
		}
		removed++
	}

	// If a specific dest was provided, remove its .part regardless of location
	if strings.TrimSpace(*destPath) != "" {
		// next-to-dest .part
		cand := *destPath
		if !strings.HasSuffix(cand, ".part") {
			cand = cand + ".part"
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			removeFile(cand)
		}
		// optional: next-to-dest .sha256 sidecar
		if *sidecars {
			sc := *destPath + ".sha256"
			if fi, err := os.Stat(sc); err == nil && !fi.IsDir() {
				if *dryRun {
					log.Infof("would remove sidecar: %s (age=%s)", sc, time.Since(fi.ModTime()).Round(time.Second))
					sideRemoved++
				} else if err := os.Remove(sc); err == nil {
					sideRemoved++
				} else {
					errs = append(errs, fmt.Sprintf("%s: %v", sc, err))
				}
			}
		}
		// hashed in partials_root: find by base name prefix
		partsDir := c.General.PartialsRoot
		if strings.TrimSpace(partsDir) == "" {
			partsDir = filepath.Join(c.General.DownloadRoot, ".parts")
		}
		if fi, err := os.Stat(partsDir); err == nil && fi.IsDir() {
			entries, _ := os.ReadDir(partsDir)
			base := filepath.Base(*destPath)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasPrefix(name, base+".") && strings.HasSuffix(name, ".part") {
					removeFile(filepath.Join(partsDir, name))
				}
			}
		}
	} else {
		// Bulk mode
		// 1) partials_root or download_root/.parts
		partsDir := c.General.PartialsRoot
		if strings.TrimSpace(partsDir) == "" {
			partsDir = filepath.Join(c.General.DownloadRoot, ".parts")
		}
		if fi, err := os.Stat(partsDir); err == nil && fi.IsDir() {
			entries, err := os.ReadDir(partsDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					name := e.Name()
					if !strings.HasSuffix(name, ".part") {
						continue
					}
					removeFile(filepath.Join(partsDir, name))
				}
			}
		}
		// 2) next-to-dest .part files when stage_partials is false or when includeNext is requested
		if *includeNext {
			root := c.General.DownloadRoot
			_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				name := d.Name()
				if strings.HasSuffix(name, ".part") {
					removeFile(p)
					return nil
				}
				if *sidecars && strings.HasSuffix(name, ".sha256") {
					base := strings.TrimSuffix(p, ".sha256")
					if _, err := os.Stat(base); os.IsNotExist(err) {
						if *dryRun {
							log.Infof("would remove orphan sidecar: %s", p)
							sideRemoved++
						} else if err := os.Remove(p); err == nil {
							sideRemoved++
						} else {
							errs = append(errs, fmt.Sprintf("%s: %v", p, err))
						}
					}
				}
				return nil
			})
		}
	}

	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"removed": removed, "skipped": skipped, "sidecars_removed": sideRemoved, "errors": errs})
	}
	log.Infof("removed: %d skipped: %d sidecars:%d", removed, skipped, sideRemoved)
	if len(errs) > 0 {
		log.Warnf("errors: %v", errs)
	}
	return nil
}

func resolveBatchDownloadCandidate(ctx context.Context, c *config.Config, uri string) (string, map[string]string, error) {
	headers := map[string]string{}
	if strings.HasPrefix(uri, "hf://") || strings.HasPrefix(uri, "civitai://") {
		res, err := resolver.Resolve(ctx, uri, c)
		if err != nil {
			return "", nil, err
		}
		return res.URL, res.Headers, nil
	}
	if u, err := neturl.Parse(uri); err == nil {
		h := strings.ToLower(u.Hostname())
		if hostIs(h, "civitai.com") && c.Sources.CivitAI.Enabled {
			env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv)
			if env == "" {
				env = "CIVITAI_TOKEN"
			}
			if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
				headers["Authorization"] = "Bearer " + tok
			}
		}
		if hostIs(h, "huggingface.co") && c.Sources.HuggingFace.Enabled {
			env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv)
			if env == "" {
				env = "HF_TOKEN"
			}
			if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
				headers["Authorization"] = "Bearer " + tok
			}
		}
	}
	return uri, headers, nil
}

func configOp(args []string, fn func(*config.Config, *logging.Logger) error) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		return err
	}
	log := logging.New(*common.logLevel, *common.jsonOut)
	return fn(c, log)
}

// parseSHA256FromFile reads the first 64-hex token from a file (supports .sha256 "hash  filename" format).
func parseSHA256FromFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// first token
		var tok string
		for i := 0; i < len(line); i++ {
			if line[i] == ' ' || line[i] == '\t' {
				tok = line[:i]
				break
			}
		}
		if tok == "" {
			tok = line
		}
		tok = strings.TrimSpace(tok)
		if len(tok) == 64 && isHex(tok) {
			return strings.ToLower(tok), nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no 64-hex SHA256 found in %s", path)
}

func isHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func defaultExtractDir(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		base = base[:len(base)-len(".tar.gz")]
	case strings.HasSuffix(lower, ".tgz"):
		base = base[:len(base)-len(".tgz")]
	default:
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if strings.TrimSpace(base) == "" {
		base = "extracted"
	}
	return filepath.Join(dir, base)
}

func scheduleWindowDelay(now time.Time, window string) (time.Duration, error) {
	window = strings.TrimSpace(window)
	if window == "" {
		return 0, nil
	}
	parts := strings.Split(window, "-")
	if len(parts) != 2 {
		return 0, fmt.Errorf("expected HH:MM-HH:MM")
	}
	start, err := parseClock(parts[0])
	if err != nil {
		return 0, err
	}
	end, err := parseClock(parts[1])
	if err != nil {
		return 0, err
	}
	minute := now.Hour()*60 + now.Minute()
	if start == end {
		return 0, nil
	}
	if start < end {
		if minute >= start && minute < end {
			return 0, nil
		}
		target := time.Date(now.Year(), now.Month(), now.Day(), start/60, start%60, 0, 0, now.Location())
		if !target.After(now) {
			target = target.Add(24 * time.Hour)
		}
		return target.Sub(now), nil
	}
	if minute >= start || minute < end {
		return 0, nil
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), start/60, start%60, 0, 0, now.Location())
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target.Sub(now), nil
}

func parseClock(s string) (int, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid clock time %q", strings.TrimSpace(s))
	}
	return t.Hour()*60 + t.Minute(), nil
}

func duplicateDownloadGroups(rows []state.DownloadRow) []duplicateGroup {
	bySHA := map[string][]state.DownloadRow{}
	for _, r := range rows {
		sha := strings.ToLower(strings.TrimSpace(r.ActualSHA256))
		if sha == "" || !strings.EqualFold(strings.TrimSpace(r.Status), "complete") {
			continue
		}
		bySHA[sha] = append(bySHA[sha], r)
	}
	groups := make([]duplicateGroup, 0)
	for sha, rs := range bySHA {
		if len(rs) < 2 {
			continue
		}
		sort.Slice(rs, func(i, j int) bool {
			return rs[i].Dest < rs[j].Dest
		})
		groups = append(groups, duplicateGroup{SHA256: sha, Rows: rs})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].SHA256 < groups[j].SHA256
	})
	return groups
}

func dedupeDuplicateGroups(groups []duplicateGroup, mode string, dryRun bool) []dedupeResult {
	if mode == "" {
		mode = "hardlink"
	}
	var results []dedupeResult
	for _, group := range groups {
		canonical, canonInfo, err := chooseCanonicalDuplicate(group)
		if err != nil {
			for _, row := range group.Rows {
				results = append(results, dedupeResult{SHA256: group.SHA256, Dest: row.Dest, Action: "skipped", Error: err.Error()})
			}
			continue
		}
		for _, row := range group.Rows {
			if row.Dest == canonical {
				results = append(results, dedupeResult{SHA256: group.SHA256, Canonical: canonical, Dest: row.Dest, Action: "canonical"})
				continue
			}
			results = append(results, dedupeOne(row.Dest, group.SHA256, canonical, canonInfo, mode, dryRun))
		}
	}
	return results
}

func chooseCanonicalDuplicate(group duplicateGroup) (string, os.FileInfo, error) {
	for _, row := range group.Rows {
		info, err := os.Stat(row.Dest)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			return row.Dest, info, nil
		}
	}
	return "", nil, fmt.Errorf("no existing regular canonical file for sha256=%s", group.SHA256)
}

func dedupeOne(dest, sha, canonical string, canonInfo os.FileInfo, mode string, dryRun bool) dedupeResult {
	res := dedupeResult{SHA256: sha, Canonical: canonical, Dest: dest}
	info, err := os.Stat(dest)
	if err != nil {
		res.Action = "skipped"
		res.Error = err.Error()
		return res
	}
	if !info.Mode().IsRegular() {
		res.Action = "skipped"
		res.Error = "destination is not a regular file"
		return res
	}
	if os.SameFile(canonInfo, info) {
		res.Action = "already_linked"
		return res
	}
	actual, err := util.HashFileSHA256(dest)
	if err != nil {
		res.Action = "skipped"
		res.Error = err.Error()
		return res
	}
	if !strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(sha)) {
		res.Action = "skipped"
		res.Error = fmt.Sprintf("sha256 mismatch: state=%s actual=%s", sha, actual)
		return res
	}
	switch mode {
	case "hardlink", "symlink":
	default:
		res.Action = "skipped"
		res.Error = fmt.Sprintf("unknown dedupe mode: %s", mode)
		return res
	}
	if dryRun {
		res.Action = "would_" + mode
		return res
	}
	if err := replaceWithLink(dest, canonical, mode); err != nil {
		res.Action = "skipped"
		res.Error = err.Error()
		return res
	}
	res.Action = mode
	return res
}

func replaceWithLink(dest, canonical, mode string) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".modfetch-dedupe-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	switch mode {
	case "hardlink":
		if err := os.Link(canonical, tmpPath); err != nil {
			return err
		}
	case "symlink":
		target := canonical
		if rel, err := filepath.Rel(dir, canonical); err == nil && rel != "" && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			target = rel
		}
		if err := os.Symlink(target, tmpPath); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func countDuplicateFiles(groups []duplicateGroup) int {
	total := 0
	for _, group := range groups {
		total += len(group.Rows)
	}
	return total
}

func countErrors(rows []state.DownloadRow) int {
	cnt := 0
	for _, r := range rows {
		ls := strings.ToLower(strings.TrimSpace(r.Status))
		if ls == "error" || ls == "checksum_mismatch" || ls == "verify_failed" {
			cnt++
		}
	}
	return cnt
}
