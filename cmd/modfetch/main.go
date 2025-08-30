package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"modfetch/internal/config"
	"modfetch/internal/downloader"
	"modfetch/internal/logging"
	"modfetch/internal/resolver"
	"modfetch/internal/state"
	"modfetch/internal/placer"
	"modfetch/internal/classifier"
	"modfetch/internal/batch"
	"modfetch/internal/metrics"
	"modfetch/internal/util"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("no command provided")
	}

	// Global flags (parsed per subcommand to avoid hard defaults)
	cmd := args[0]
	switch cmd {
	case "config":
		return handleConfig(args[1:])
	case "status":
		return handleStatus(args[1:])
	case "download":
		return handleDownload(args[1:])
	case "place":
		return handlePlace(args[1:])
	case "verify":
		return handleVerify(args[1:])
	case "tui":
		return handleTUI(args[1:])
	case "version":
		fmt.Println(version)
		return nil
case "completion":
		return handleCompletion(args[1:])
case "hostcaps":
		return handleHostCaps(args[1:])
case "clean":
		return handleClean(args[1:])
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
  version           Print version
  help              Show this help
  completion        Generate shell completion scripts (bash|zsh|fish)
  hostcaps          Manage host capability cache (list/clear)
  clean             Prune staged partials and other cached artifacts

Flags:
  --config PATH     Path to YAML config file (or MODFETCH_CONFIG env var)
  --log-level L     Log level: debug|info|warn|error (per command)
  --json            JSON log output (per command)
`))
}

func handleStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if _, err := os.Stat(*cfgPath); err != nil { return fmt.Errorf("config file not found: %s", *cfgPath) }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	_ = c // currently unused; reserved for future filters
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	rows, err := st.ListDownloads()
	if err != nil { return err }
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	for _, r := range rows {
		log.Infof("%s -> %s [%s] size=%d", logging.SanitizeURL(r.URL), r.Dest, r.Status, r.Size)
	}
	return nil
}

func handleConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand required: validate | print")
	}
	sub := args[0]
	switch sub {
	case "validate":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			log.Infof("config: valid")
			return nil
		})
	case "print":
		return configOp(args[1:], func(c *config.Config, log *logging.Logger) error {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(c)
		})
	case "wizard":
		return handleConfigWizard(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s", sub)
	}
}

func handleDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	quiet := fs.Bool("quiet", false, "suppress progress and info logs (errors only)")
	noResume := fs.Bool("no-resume", false, "do not resume; start fresh (delete any staged .part and chunk state)")
	url := fs.String("url", "", "HTTP URL to download (direct) or resolver URI (e.g. hf://owner/repo/path)")
	dest := fs.String("dest", "", "destination path (optional)")
	sha := fs.String("sha256", "", "expected SHA256 (optional)")
	batchPath := fs.String("batch", "", "YAML batch file with download jobs")
	placeFlag := fs.Bool("place", false, "place files after successful download")
	summaryJSON := fs.Bool("summary-json", false, "print a JSON summary when a download completes")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env }
	}
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if _, err := os.Stat(*cfgPath); err != nil { return fmt.Errorf("config file not found: %s", *cfgPath) }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	// quiet forces log level to error unless JSON is requested
	if *quiet && !*jsonOut { *logLevel = "error" }
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()
	ctx := context.Background()
	startWall := time.Now()
	// Metrics manager (Prometheus textfile), if enabled
	m := metrics.New(c)
	// Batch mode
	if *batchPath != "" {
		bf, err := batch.Load(*batchPath)
		if err != nil { return err }
		dl := downloader.NewAuto(c, log, st, m)
		for i, job := range bf.Jobs {
			resolvedURL := job.URI
			headers := map[string]string{}
			destCandidate := strings.TrimSpace(job.Dest)
			if strings.HasPrefix(resolvedURL, "hf://") || strings.HasPrefix(resolvedURL, "civitai://") {
				res, err := resolver.Resolve(ctx, resolvedURL, c)
				if err != nil { return fmt.Errorf("job %d resolve: %w", i, err) }
				resolvedURL = res.URL
				headers = res.Headers
				if destCandidate == "" && strings.HasPrefix(job.URI, "civitai://") && strings.TrimSpace(res.SuggestedFilename) != "" {
					if p, err := util.UniquePath(c.General.DownloadRoot, res.SuggestedFilename, res.VersionID); err == nil { destCandidate = p }
				}
			}
			final, sum, err := dl.Download(ctx, resolvedURL, destCandidate, job.SHA256, headers, false)
			if err != nil { return fmt.Errorf("job %d download: %w", i, err) }
			log.Infof("downloaded: %s (sha256=%s)", final, sum)
			var placed []string
			if job.Place || *placeFlag {
				var err2 error
				placed, err2 = placer.Place(c, final, job.Type, job.Mode)
				if err2 != nil { return fmt.Errorf("job %d place: %w", i, err2) }
				for _, p := range placed { log.Infof("placed: %s", p) }
			}
			if *summaryJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]any{
					"url":      logging.SanitizeURL(resolvedURL),
					"dest":     final,
					"sha256":   sum,
					"placed":   placed,
					"status":   "ok",
				})
			}
		}
		return nil
	}

	resolvedURL := *url
	headers := map[string]string{}
	// Support CivitAI model page URLs by translating to civitai://model/{id}[?version=]
	if strings.HasPrefix(resolvedURL, "http://") || strings.HasPrefix(resolvedURL, "https://") {
		if u, err := neturl.Parse(resolvedURL); err == nil {
			h := strings.ToLower(u.Hostname())
			if strings.HasSuffix(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
				parts := strings.Split(strings.Trim(u.Path, "/"), "/")
				if len(parts) >= 2 {
					modelID := parts[1]
					q := u.Query()
					ver := q.Get("modelVersionId")
					if ver == "" { ver = q.Get("version") }
					civ := "civitai://model/" + modelID
					if strings.TrimSpace(ver) != "" { civ += "?version=" + ver }
					if res, err := resolver.Resolve(ctx, civ, c); err == nil {
						resolvedURL = res.URL
						headers = res.Headers
					}
				}
			}
		}
	}
	if strings.HasPrefix(resolvedURL, "hf://") || strings.HasPrefix(resolvedURL, "civitai://") {
		res, err := resolver.Resolve(ctx, resolvedURL, c)
		if err != nil { return err }
		resolvedURL = res.URL
		headers = res.Headers
	} else {
		// Attach auth headers for direct URLs when possible
		if u, err := neturl.Parse(resolvedURL); err == nil {
			h := strings.ToLower(u.Hostname())
			if headers == nil { headers = map[string]string{} }
			if strings.HasSuffix(h, "civitai.com") && c.Sources.CivitAI.Enabled {
				if env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer " + tok } else { log.Warnf("CivitAI token env %s is not set; gated content will return 401. Export %s.", env, env) }
				}
			}
			if strings.HasSuffix(h, "huggingface.co") && c.Sources.HuggingFace.Enabled {
				if env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" { headers["Authorization"] = "Bearer " + tok } else { log.Warnf("Hugging Face token env %s is not set; gated repos will return 401. Export %s and accept the repo license.", env, env) }
				}
			}
		}
	}
	// Prefer chunked downloader; it will fall back to single when needed
	// Progress display (disabled for JSON or quiet)
	var stopProg func()
	if !*jsonOut && !*quiet {
		candDest := strings.TrimSpace(*dest)
		if candDest == "" && strings.HasPrefix(*url, "civitai://") {
			// If we resolved civitai:// earlier, try SuggestedFilename by resolving again cheaply
			if res2, err := resolver.Resolve(ctx, *url, c); err == nil && strings.TrimSpace(res2.SuggestedFilename) != "" {
				if p, err := util.UniquePath(c.General.DownloadRoot, res2.SuggestedFilename, res2.VersionID); err == nil { candDest = p }
			}
		}
		if candDest == "" {
			candDest = filepath.Join(c.General.DownloadRoot, filepath.Base(resolvedURL))
		}
		stopProg = startProgressLoop(ctx, st, c, resolvedURL, candDest)
	}
	// Prefer chunked downloader; it will fall back to single when needed
	dl := downloader.NewAuto(c, log, st, m)
	// If civitai:// and no explicit dest, use SuggestedFilename
	destArg := strings.TrimSpace(*dest)
	if destArg == "" && strings.HasPrefix(*url, "civitai://") {
		if res2, err := resolver.Resolve(ctx, *url, c); err == nil && strings.TrimSpace(res2.SuggestedFilename) != "" {
			if p, err := util.UniquePath(c.General.DownloadRoot, res2.SuggestedFilename, res2.VersionID); err == nil { destArg = p }
		}
	}
	final, sum, err := dl.Download(ctx, resolvedURL, destArg, *sha, headers, (*noResume) || c.General.AlwaysNoResume)
	if stopProg != nil { stopProg() }
	if err != nil { return err }
	// Final summary
	fi, _ := os.Stat(final)
	size := int64(0); if fi != nil { size = fi.Size() }
	dur := time.Since(startWall).Seconds()
	if *summaryJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"url":      logging.SanitizeURL(resolvedURL),
			"dest":     final,
			"size":     size,
			"duration": dur,
			"avg_bps":  float64(size)/dur,
			"sha256":   sum,
			"status":   "ok",
		})
	} else if !*jsonOut {
		var rate string
		if dur > 0 && size > 0 { rate = humanize.Bytes(uint64(float64(size)/dur)) + "/s" } else { rate = "-" }
	fmt.Printf("\nDownloaded: %s\nDest: %s\nSHA256: %s\nSize: %s\nDuration: %.1fs  Avg: %s\n", logging.SanitizeURL(resolvedURL), final, sum, humanize.Bytes(uint64(size)), dur, rate)
	}
	log.Infof("downloaded: %s (sha256=%s)", final, sum)
	if *placeFlag {
		placed, err := placer.Place(c, final, "", "")
		if err != nil { return err }
		for _, p := range placed { log.Infof("placed: %s", p) }
	}
	return nil
}

func handlePlace(args []string) error {
	fs := flag.NewFlagSet("place", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	filePath := fs.String("path", "", "path to file to place")
	artType := fs.String("type", "", "artifact type override (optional)")
	mode := fs.String("mode", "", "placement mode override: symlink|hardlink|copy (optional)")
	dryRun := fs.Bool("dry-run", false, "print planned destinations only; do not modify files")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if *filePath == "" { return errors.New("--path is required") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)
	if *dryRun {
		atype := *artType
		if atype == "" { atype = classifier.Detect(*filePath) }
		targets, err := placer.ComputeTargets(c, atype)
		if err != nil { return err }
		fmt.Printf("Would place %s (type=%s) to:\n", *filePath, atype)
		for _, t := range targets { fmt.Printf("  %s\n", t) }
		return nil
	}
	placed, err := placer.Place(c, *filePath, *artType, *mode)
	if err != nil { return err }
	for _, p := range placed { log.Infof("placed: %s", p) }
	return nil
}

func handleClean(args []string) error {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	days := fs.Int("days", 7, "remove .part files older than this many days (0 = remove all)")
	dryRun := fs.Bool("dry-run", false, "show what would be removed, but do not delete")
	destPath := fs.String("dest", "", "remove staged .part for this destination path (overrides days)")
	includeNext := fs.Bool("include-next-to-dest", true, "also scan download_root for *.part when stage_partials=false")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if _, err := os.Stat(*cfgPath); err != nil { return fmt.Errorf("config file not found: %s", *cfgPath) }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)

	removed := 0
	skipped := 0
	var errs []string

	// Helper to maybe remove a file
	removeFile := func(p string) {
		fi, err := os.Stat(p)
		if err != nil { errs = append(errs, fmt.Sprintf("%s: %v", p, err)); return }
		if *destPath == "" {
			// age-gated mode
			cutoff := time.Now().Add(-time.Duration(*days)*24*time.Hour)
			if *days > 0 && fi.ModTime().After(cutoff) { skipped++; return }
		}
		if *dryRun {
			log.Infof("would remove: %s (age=%s)", p, time.Since(fi.ModTime()).Round(time.Second))
			removed++
			return
		}
		if err := os.Remove(p); err != nil { errs = append(errs, fmt.Sprintf("%s: %v", p, err)); return }
		removed++
	}

	// If a specific dest was provided, remove its .part regardless of location
	if strings.TrimSpace(*destPath) != "" {
		// next-to-dest .part
		cand := *destPath
		if !strings.HasSuffix(cand, ".part") { cand = cand + ".part" }
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() { removeFile(cand) }
		// hashed in partials_root: find by base name prefix
		partsDir := c.General.PartialsRoot
		if strings.TrimSpace(partsDir) == "" {
			partsDir = filepath.Join(c.General.DownloadRoot, ".parts")
		}
		if fi, err := os.Stat(partsDir); err == nil && fi.IsDir() {
			entries, _ := os.ReadDir(partsDir)
			base := filepath.Base(*destPath)
			for _, e := range entries {
				if e.IsDir() { continue }
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
					if e.IsDir() { continue }
					name := e.Name()
					if !strings.HasSuffix(name, ".part") { continue }
					removeFile(filepath.Join(partsDir, name))
				}
			}
		}
		// 2) next-to-dest .part files when stage_partials is false or when includeNext is requested
		if *includeNext {
			root := c.General.DownloadRoot
			_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil { return nil }
				if d.IsDir() { return nil }
				if strings.HasSuffix(d.Name(), ".part") { removeFile(p) }
				return nil
			})
		}
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"removed": removed, "skipped": skipped, "errors": errs})
	}
	log.Infof("removed: %d skipped: %d", removed, skipped)
	if len(errs) > 0 { log.Warnf("errors: %v", errs) }
	return nil
}

func configOp(args []string, fn func(*config.Config, *logging.Logger) error) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		}
	}
	if *cfgPath == "" {
		return errors.New("--config is required or set MODFETCH_CONFIG")
	}
	if _, err := os.Stat(*cfgPath); err != nil { return fmt.Errorf("config file not found: %s", *cfgPath) }
	c, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	log := logging.New(*logLevel, *jsonOut)
	return fn(c, log)
}

