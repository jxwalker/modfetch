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
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/sync/errgroup"

	mfarchive "github.com/jxwalker/modfetch/internal/archive"
	"github.com/jxwalker/modfetch/internal/batch"
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

const (
	defaultDownloadConnections    = 4
	defaultDownloadChunkSizeMB    = 8
	largeModelDownloadConnections = 16
	largeModelChunkSizeMB         = 64
)

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
	profile := fs.String("profile", "", "download tuning profile: default or large-model")
	connections := fs.Int("connections", 0, "parallel range requests per file (aria2-style; overrides concurrency.per_file_chunks)")
	chunkSizeMB := fs.Int("chunk-size-mb", 0, "range chunk size in MiB for this invocation")
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
	batchFileParallel := c.Concurrency.PerHostRequests
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
	if err := applyDownloadTuning(c, *profile, *connections, *chunkSizeMB); err != nil {
		return err
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
		// tryReserve returns (path, true, nil) when path is free and reserved,
		// (path, false, nil) when the path is already taken (file exists or
		// another worker reserved it), or ("", false, err) for any unexpected
		// filesystem error (e.g. permission denied on the download directory)
		// so the caller can surface a clear error instead of looping forever.
		tryReserve := func(p string) (string, bool, error) {
			_, err := os.Stat(p)
			switch {
			case err == nil:
				return p, false, nil
			case os.IsNotExist(err):
				if _, ok := reserved[p]; ok {
					return p, false, nil
				}
				reserved[p] = struct{}{}
				return p, true, nil
			default:
				return "", false, fmt.Errorf("stat %s: %w", p, err)
			}
		}
		reserveUnique := func(dir, base, versionHint string) (string, error) {
			destMu.Lock()
			defer destMu.Unlock()
			base = util.SafeFileName(base)
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			// try plain
			if p, ok, err := tryReserve(filepath.Join(dir, base)); err != nil {
				return "", err
			} else if ok {
				return p, nil
			}
			// try version
			if strings.TrimSpace(versionHint) != "" {
				if p, ok, err := tryReserve(filepath.Join(dir, fmt.Sprintf("%s (v%s)%s", name, versionHint, ext))); err != nil {
					return "", err
				} else if ok {
					return p, nil
				}
			}
			// Cap suffix attempts so a misbehaving filesystem can't hang the
			// goroutine forever even if every candidate appears taken.
			const maxSuffixAttempts = 1 << 16
			for i := 2; i < maxSuffixAttempts; i++ {
				p, ok, err := tryReserve(filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext)))
				if err != nil {
					return "", err
				}
				if ok {
					return p, nil
				}
			}
			return "", fmt.Errorf("could not reserve unique destination for %q in %s after %d attempts", base, dir, maxSuffixAttempts)
		}
		// Download tuning flags control range concurrency inside each worker.
		// Keep batch file fan-out on the original config unless explicitly set.
		parallel := batchFileParallel
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
						if isResolverURI(resolvedURL) {
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
						logMu.Lock()
						log.Infof("job %d: starting %s -> %s", it.idx, logging.SanitizeURL(candidates[0].url), destCandidate)
						logMu.Unlock()
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

	if isResolverURI(resolvedURL) {
		res, err := resolver.Resolve(ctx, resolvedURL, c)
		if err != nil {
			return err
		}
		resolvedURL = res.URL
		headers = res.Headers
	} else {
		// Attach auth headers for direct URLs when possible, but only when
		// sources.<provider>.token_env is explicitly configured. An empty
		// token_env means "do not attach auth", matching the semantics in
		// internal/resolver/{civitai,huggingface}.go and cmd/modfetch/batch_cmd.go;
		// we deliberately do not fall back to ambient CIVITAI_TOKEN / HF_TOKEN
		// here so users who left token_env unset never get unexpected
		// credentials sent to direct URLs.
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
		if !isResolverURI(resolverURI) {
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
				"resolver_uri":  logging.SanitizeURL(*url),
				"resolved_url":  logging.SanitizeURL(resolvedURL),
				"default_dest":  candDest,
				"connections":   effectiveDownloadConnections(c),
				"chunk_size_mb": effectiveChunkSizeMB(c),
				"profile":       effectiveDownloadProfile(*profile),
			})
			return nil
		}
		fmt.Printf("Plan (dry-run)\n")
		fmt.Printf("  Resolver URI: %s\n", logging.SanitizeURL(*url))
		fmt.Printf("  Resolved URL: %s\n", logging.SanitizeURL(resolvedURL))
		fmt.Printf("  Default dest: %s\n", candDest)
		fmt.Printf("  Profile: %s\n", effectiveDownloadProfile(*profile))
		fmt.Printf("  Connections: %d\n", effectiveDownloadConnections(c))
		fmt.Printf("  Chunk size: %d MiB\n", effectiveChunkSizeMB(c))
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
		if !isResolverURI(resolverURI) {
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
	if !isResolverURI(resolverURI2) {
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
	// Guard against zero/near-zero durations: float64(size)/0 yields +Inf,
	// which json.Encoder cannot serialize and silently fails. Match the
	// non-JSON branch below by reporting 0 in that edge case.
	avgBps := 0.0
	if dur > 0 {
		avgBps = float64(size) / dur
	}
	if *summaryJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"url":       logging.SanitizeURL(resolvedURL),
			"dest":      final,
			"size":      size,
			"duration":  dur,
			"avg_bps":   avgBps,
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

func applyDownloadTuning(c *config.Config, profile string, connections, chunkSizeMB int) error {
	if c == nil {
		return errors.New("nil config")
	}
	if connections < 0 {
		return errors.New("--connections must be >= 0")
	}
	if chunkSizeMB < 0 {
		return errors.New("--chunk-size-mb must be >= 0")
	}
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "default":
	case "large", "large-model":
		if c.Concurrency.PerFileChunks < largeModelDownloadConnections {
			c.Concurrency.PerFileChunks = largeModelDownloadConnections
		}
		if c.Concurrency.PerHostRequests < largeModelDownloadConnections {
			c.Concurrency.PerHostRequests = largeModelDownloadConnections
		}
		if c.Concurrency.ChunkSizeMB < largeModelChunkSizeMB {
			c.Concurrency.ChunkSizeMB = largeModelChunkSizeMB
		}
	default:
		return fmt.Errorf("unknown download profile %q (valid: default, large-model)", profile)
	}
	if connections > 0 {
		c.Concurrency.PerFileChunks = connections
		if c.Concurrency.PerHostRequests < connections {
			c.Concurrency.PerHostRequests = connections
		}
	}
	if chunkSizeMB > 0 {
		c.Concurrency.ChunkSizeMB = chunkSizeMB
	}
	return nil
}

func effectiveDownloadProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "large", "large-model":
		return "large-model"
	default:
		return "default"
	}
}

func effectiveDownloadConnections(c *config.Config) int {
	if c == nil || c.Concurrency.PerFileChunks <= 0 {
		return defaultDownloadConnections
	}
	connections := c.Concurrency.PerFileChunks
	if c.Concurrency.PerHostRequests > 0 && c.Concurrency.PerHostRequests < connections {
		return c.Concurrency.PerHostRequests
	}
	return connections
}

func effectiveChunkSizeMB(c *config.Config) int {
	if c == nil || c.Concurrency.ChunkSizeMB <= 0 {
		return defaultDownloadChunkSizeMB
	}
	return c.Concurrency.ChunkSizeMB
}

// resolveBatchDownloadCandidate resolves a single source URI or direct URL to
// a concrete download URL with any necessary auth headers.
func resolveBatchDownloadCandidate(ctx context.Context, c *config.Config, uri string) (string, map[string]string, error) {
	headers := map[string]string{}
	if isResolverURI(uri) {
		res, err := resolver.Resolve(ctx, uri, c)
		if err != nil {
			return "", nil, err
		}
		return res.URL, res.Headers, nil
	}
	if u, err := neturl.Parse(uri); err == nil {
		h := strings.ToLower(u.Hostname())
		// Only attach auth when token_env is explicitly configured. Empty
		// token_env means "do not attach auth"; matches the existing batch
		// import logic in cmd/modfetch/batch_cmd.go and the resolver package
		// to avoid leaking ambient CIVITAI_TOKEN / HF_TOKEN credentials.
		if hostIs(h, "civitai.com") && c.Sources.CivitAI.Enabled {
			if env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv); env != "" {
				if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
					headers["Authorization"] = "Bearer " + tok
				}
			}
		}
		if hostIs(h, "huggingface.co") && c.Sources.HuggingFace.Enabled {
			if env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv); env != "" {
				if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
					headers["Authorization"] = "Bearer " + tok
				}
			}
		}
	}
	return uri, headers, nil
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
