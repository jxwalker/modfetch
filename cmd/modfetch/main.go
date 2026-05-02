package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jxwalker/modfetch/internal/classifier"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/placer"
	"github.com/jxwalker/modfetch/internal/state"
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
	case "discover":
		return handleDiscover(ctx, args[1:])
	case "starter":
		return handleStarter(ctx, args[1:])
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
	case "library":
		return handleLibrary(ctx, args[1:])
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
  download          Download a file via direct URL or resolver URI (starter://, hf://, civitai://)
  discover          Search real model providers and download a selected result
  starter           List or download beginner-safe starter artifacts
  status            Show download status (table or JSON)
  place             Place a file into configured app directories
  verify            Verify SHA256 of a file or all completed downloads
  tui               Open the interactive terminal dashboard or print a state snapshot
  library export    Export model library catalog as JSON
  library import    Import model library catalog JSON
  library scan      Scan configured model directories into the library
  library sync      Push or pull a catalog through a sync target
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

func handlePlace(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("place", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	filePath := fs.String("path", "", "path to file to place")
	artType := fs.String("type", "", "artifact type override (optional)")
	mode := fs.String("mode", "", "placement mode override: symlink|hardlink|copy (optional)")
	preset := fs.String("preset", "", "placement preset(s) to apply: "+strings.Join(placer.PresetNames(), ", "))
	listPresets := fs.Bool("list-presets", false, "list available placement presets")
	dryRun := fs.Bool("dry-run", false, "print planned destinations only; do not modify files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *listPresets {
		for _, name := range placer.PresetNames() {
			p := placer.Presets()[name]
			fmt.Printf("%-14s %s\n", name, p.Description)
		}
		return nil
	}
	if *filePath == "" {
		return errors.New("--path is required")
	}
	presetNames := placer.ParsePresetList(*preset)
	c, _, err := loadConfig(*common.configPath)
	if err != nil {
		canUsePresetOnly := len(presetNames) > 0 &&
			strings.TrimSpace(*common.configPath) == "" &&
			strings.TrimSpace(os.Getenv("MODFETCH_CONFIG")) == "" &&
			errors.Is(err, os.ErrNotExist)
		if !canUsePresetOnly {
			return err
		}
		c = &config.Config{Version: 1, General: config.General{PlacementMode: "symlink"}}
	}
	if err := placer.ApplyPresets(c, presetNames); err != nil {
		return err
	}
	log := logging.New(*common.logLevel, *common.jsonOut)
	if *dryRun {
		result := classifier.Result{Type: strings.TrimSpace(*artType), Confidence: "override", Reason: "--type override"}
		if result.Type == "" {
			result = classifier.Analyze(c, *filePath)
		}
		targets, err := placer.ComputeTargets(c, result.Type)
		if err != nil {
			return err
		}
		placeMode := strings.TrimSpace(*mode)
		if placeMode == "" {
			placeMode = c.General.PlacementMode
		}
		if placeMode == "" {
			placeMode = "symlink"
		}
		if *common.jsonOut {
			planned := make([]map[string]string, 0, len(targets))
			for _, t := range targets {
				planned = append(planned, map[string]string{
					"action": "place",
					"mode":   placeMode,
					"source": *filePath,
					"target": filepath.Join(t, filepath.Base(*filePath)),
				})
			}
			if len(planned) == 0 {
				planned = append(planned, map[string]string{
					"action": "skip",
					"reason": "no mapping targets for type " + result.Type,
					"source": *filePath,
				})
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"type":       result.Type,
				"confidence": result.Confidence,
				"reason":     result.Reason,
				"preset":     presetNames,
				"planned":    planned,
			})
		}
		fmt.Printf("Would place %s\n", *filePath)
		fmt.Printf("Detected type: %s (confidence=%s; %s)\n", result.Type, result.Confidence, result.Reason)
		fmt.Printf("Placement mode: %s\n", placeMode)
		if len(targets) == 0 {
			fmt.Printf("Would skip: no mapping targets for type %s\n", result.Type)
			return nil
		}
		for _, t := range targets {
			fmt.Printf("  %s -> %s\n", placeMode, filepath.Join(t, filepath.Base(*filePath)))
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


func isResolverURI(uri string) bool {
	uri = strings.TrimSpace(uri)
	return strings.HasPrefix(uri, "starter://") || strings.HasPrefix(uri, "hf://") || strings.HasPrefix(uri, "civitai://")
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
