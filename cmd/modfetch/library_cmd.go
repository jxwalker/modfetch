package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/catalog"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/scanner"
	"github.com/jxwalker/modfetch/internal/state"
)

func handleLibrary(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("library subcommand required: export, import, scan, or sync")
	}
	switch args[0] {
	case "export":
		return handleLibraryExport(ctx, args[1:])
	case "import":
		return handleLibraryImport(ctx, args[1:])
	case "scan":
		return handleLibraryScan(ctx, args[1:])
	case "sync":
		return handleLibrarySync(ctx, args[1:])
	default:
		return fmt.Errorf("unknown library subcommand: %s", args[0])
	}
}

func handleLibraryExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library export", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "")
	format := fs.String("format", "json", "catalog format: json")
	output := fs.String("output", "-", "output path, or - for stdout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if *format != "json" {
		return fmt.Errorf("unsupported export format %q", *format)
	}
	db, err := openLibraryDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	w, closeFn, err := outputWriter(*output)
	if err != nil {
		return err
	}
	defer closeFn()
	return catalog.Export(db, w)
}

func handleLibraryImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library import", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print import result as JSON")
	input := fs.String("input", "", "catalog JSON path, or - for stdin")
	dryRun := fs.Bool("dry-run", false, "report changes without writing to the library")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if *input == "" {
		return errors.New("library import requires --input")
	}
	db, err := openLibraryDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	r, closeFn, err := inputReader(*input)
	if err != nil {
		return err
	}
	defer closeFn()
	result, err := catalog.Import(db, r, catalog.ImportOptions{DryRun: *dryRun})
	if err != nil {
		return err
	}
	return printCatalogImportResult(result, *common.jsonOut)
}

func handleLibrarySync(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("library sync subcommand required: push or pull")
	}
	switch args[0] {
	case "push":
		return handleLibrarySyncPush(ctx, args[1:])
	case "pull":
		return handleLibrarySyncPull(ctx, args[1:])
	default:
		return fmt.Errorf("unknown library sync subcommand: %s", args[0])
	}
}

func handleLibrarySyncPush(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library sync push", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print sync push result as JSON")
	target := fs.String("target", "", "sync target URI or path; file://, http://, https://, and plain paths are supported")
	dryRun := fs.Bool("dry-run", false, "report without writing the target catalog")
	tokenEnv := fs.String("token-env", defaultSyncTokenEnv, "environment variable containing a bearer token for HTTP(S) sync targets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	syncTarget := strings.TrimSpace(*target)
	db, err := openLibraryDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	cat, err := catalog.Build(db)
	if err != nil {
		return err
	}
	result := librarySyncPushResult{
		Action: "push",
		Target: syncTarget,
		Models: len(cat.Models),
		DryRun: *dryRun,
	}
	if !*dryRun {
		outcome, err := writeCatalogSyncTarget(ctx, syncTarget, cat, *tokenEnv)
		if err != nil {
			return err
		}
		result.Path = outcome.Path
		result.Method = outcome.Method
		result.Status = outcome.Status
		result.Authenticated = outcome.Authenticated
	} else {
		outcome, err := describeCatalogSyncTarget(syncTarget)
		if err != nil {
			return err
		}
		result.Path = outcome.Path
		result.Method = outcome.Method
	}
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if *dryRun {
		fmt.Printf("Sync push dry-run: target=%s models=%d\n", result.DisplayTarget(), len(cat.Models))
	} else {
		fmt.Printf("Sync push complete: target=%s models=%d\n", result.DisplayTarget(), len(cat.Models))
	}
	return nil
}

func handleLibrarySyncPull(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library sync pull", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print sync pull result as JSON")
	target := fs.String("target", "", "sync target URI or path; file://, http://, https://, and plain paths are supported")
	dryRun := fs.Bool("dry-run", false, "report changes without writing to the library")
	tokenEnv := fs.String("token-env", defaultSyncTokenEnv, "environment variable containing a bearer token for HTTP(S) sync targets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	syncTarget := strings.TrimSpace(*target)
	db, err := openLibraryDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	r, closeFn, err := syncTargetReader(ctx, syncTarget, *tokenEnv)
	if err != nil {
		return err
	}
	defer closeFn()

	result, err := catalog.Import(db, r, catalog.ImportOptions{DryRun: *dryRun})
	if err != nil {
		return err
	}
	return printCatalogImportResult(result, *common.jsonOut)
}

type librarySyncPushResult struct {
	Action        string `json:"action"`
	Target        string `json:"target"`
	Path          string `json:"path,omitempty"`
	Method        string `json:"method,omitempty"`
	Status        string `json:"status,omitempty"`
	Models        int    `json:"models"`
	DryRun        bool   `json:"dry_run"`
	Authenticated bool   `json:"authenticated,omitempty"`
}

func (r librarySyncPushResult) DisplayTarget() string {
	if r.Path != "" {
		return r.Path
	}
	return r.Target
}

func printCatalogImportResult(result *catalog.ImportResult, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
		if result.Conflicts > 0 {
			return fmt.Errorf("catalog import has %d conflict(s)", result.Conflicts)
		}
		return nil
	}
	fmt.Printf("Import summary: creates=%d updates=%d skips=%d conflicts=%d dry_run=%t\n",
		result.Creates, result.Updates, result.Skips, result.Conflicts, result.DryRun)
	for _, entry := range result.Entries {
		if entry.Reason != "" {
			fmt.Printf("%s %s (%s)\n", entry.Action, entry.DownloadURL, entry.Reason)
		} else {
			fmt.Printf("%s %s\n", entry.Action, entry.DownloadURL)
		}
	}
	if result.Conflicts > 0 {
		return fmt.Errorf("catalog import has %d conflict(s)", result.Conflicts)
	}
	return nil
}

type stringListFlag []string

const (
	defaultSyncTokenEnv      = "MODFETCH_SYNC_TOKEN"
	allowInsecureSyncHTTPEnv = "MODFETCH_ALLOW_INSECURE_HTTP"
)

var syncHTTPClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: checkSyncRedirect,
}

func checkSyncRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	first := via[0]
	if first.Method != http.MethodGet && first.Method != http.MethodHead {
		return errors.New("refusing to redirect non-GET sync request")
	}
	if first.Header.Get("Authorization") == "" {
		return nil
	}
	if first.URL == nil || req.URL == nil {
		return errors.New("refusing to redirect authenticated sync request without a URL")
	}
	if !sameNormalizedOrigin(first.URL, req.URL) {
		return errors.New("refusing to redirect authenticated sync request across scheme or host")
	}
	return nil
}

func sameNormalizedOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	if !strings.EqualFold(a.Scheme, b.Scheme) {
		return false
	}
	if !strings.EqualFold(a.Hostname(), b.Hostname()) {
		return false
	}
	return effectiveURLPort(a) == effectiveURLPort(b)
}

func effectiveURLPort(u *url.URL) string {
	if u == nil {
		return ""
	}
	if port := u.Port(); port != "" {
		return port
	}
	switch {
	case strings.EqualFold(u.Scheme, "http"):
		return "80"
	case strings.EqualFold(u.Scheme, "https"):
		return "443"
	default:
		return ""
	}
}

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("directory path is empty")
	}
	*f = append(*f, value)
	return nil
}

func handleLibraryScan(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("library scan", flag.ContinueOnError)
	common := addCommonConfigLogFlags(fs, "print scan result as JSON")
	workers := fs.Int("workers", 0, "parallel scanner workers (default: bounded by CPU, max 8)")
	repairStale := fs.Bool("repair-stale", false, "remove library metadata for missing files under scanned directories")
	noProgress := fs.Bool("no-progress", false, "disable progress output")
	var dirFlags stringListFlag
	fs.Var(&dirFlags, "dir", "directory to scan (repeatable; default: configured library directories)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cfg, db, err := openLibraryConfigAndDB(*common.configPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	dirs := []string(dirFlags)
	if len(dirs) == 0 {
		dirs = scanner.ConfiguredDirectories(cfg)
	}
	if len(dirs) == 0 {
		return errors.New("no directories configured to scan")
	}

	var progress scanner.ProgressFunc
	if !*common.jsonOut && !*noProgress {
		progress = func(p scanner.Progress) {
			if p.Path == "" {
				return
			}
			_, _ = fmt.Fprintf(os.Stderr, "\rScanning: files=%d added=%d skipped=%d stale_removed=%d errors=%d",
				p.FilesScanned, p.ModelsAdded, p.ModelsSkipped, p.StaleRemoved, p.Errors)
		}
	}

	sc := scanner.NewScanner(db)
	result, err := sc.ScanWithContext(ctx, dirs, scanner.Options{
		Workers:     *workers,
		RepairStale: *repairStale,
		Progress:    progress,
	})
	if progress != nil {
		_, _ = fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return err
	}
	if *common.jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(libraryScanOutputFromResult(result, dirs))
	}
	fmt.Printf("Scan summary: files=%d found=%d added=%d skipped=%d stale_checked=%d stale_removed=%d errors=%d\n",
		result.FilesScanned, result.ModelsFound, result.ModelsAdded, result.ModelsSkipped,
		result.StaleChecked, result.StaleRemoved, len(result.Errors))
	for _, err := range result.Errors {
		fmt.Printf("error: %v\n", err)
	}
	return nil
}

type libraryScanOutput struct {
	Directories   []string `json:"directories"`
	FilesScanned  int      `json:"files_scanned"`
	ModelsFound   int      `json:"models_found"`
	ModelsAdded   int      `json:"models_added"`
	ModelsSkipped int      `json:"models_skipped"`
	StaleChecked  int      `json:"stale_checked"`
	StaleRemoved  int      `json:"stale_removed"`
	Errors        []string `json:"errors,omitempty"`
}

func libraryScanOutputFromResult(result *scanner.ScanResult, dirs []string) libraryScanOutput {
	output := libraryScanOutput{Directories: dirs}
	if result == nil {
		return output
	}
	output.FilesScanned = result.FilesScanned
	output.ModelsFound = result.ModelsFound
	output.ModelsAdded = result.ModelsAdded
	output.ModelsSkipped = result.ModelsSkipped
	output.StaleChecked = result.StaleChecked
	output.StaleRemoved = result.StaleRemoved
	for _, err := range result.Errors {
		output.Errors = append(output.Errors, err.Error())
	}
	return output
}

func openLibraryDB(configPath string) (*state.DB, error) {
	_, db, err := openLibraryConfigAndDB(configPath)
	return db, err
}

func openLibraryConfigAndDB(configPath string) (*config.Config, *state.DB, error) {
	cfg, _, err := loadConfig(configPath)
	if err != nil {
		return nil, nil, err
	}
	db, err := state.Open(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, db, nil
}

func outputWriter(path string) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

func syncTargetReader(ctx context.Context, target, tokenEnv string) (io.Reader, func(), error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, func() {}, errors.New("library sync requires --target")
	}
	if !isURITarget(target) || strings.HasPrefix(strings.ToLower(target), "file:") {
		path, err := fileSyncTargetPath(target)
		if err != nil {
			return nil, func() {}, err
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, func() {}, err
		}
		return f, func() { _ = f.Close() }, nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return nil, func() {}, fmt.Errorf("parse sync target: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, func() {}, fmt.Errorf("unsupported sync target scheme %q; pull supports file://, http://, and https://", u.Scheme)
	}
	if u.Fragment != "" {
		return nil, func() {}, errors.New("HTTP sync target must not include a fragment")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create sync request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if _, err := addBearerAuthFromEnv(req, tokenEnv); err != nil {
		return nil, func() {}, err
	}
	resp, err := syncHTTPClient.Do(req)
	if err != nil {
		return nil, func() {}, fmt.Errorf("fetch sync target: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		statusErr := httpStatusError("fetch sync target", resp)
		_ = resp.Body.Close()
		return nil, func() {}, statusErr
	}
	return resp.Body, func() { _ = resp.Body.Close() }, nil
}

type catalogSyncWriteOutcome struct {
	Path          string
	Method        string
	Status        string
	Authenticated bool
}

func writeCatalogSyncTarget(ctx context.Context, target string, cat *catalog.Catalog, tokenEnv string) (catalogSyncWriteOutcome, error) {
	target = strings.TrimSpace(target)
	outcome, err := describeCatalogSyncTarget(target)
	if err != nil {
		return outcome, err
	}
	if outcome.Method == "file" {
		if err := writeCatalogFile(outcome.Path, cat); err != nil {
			return outcome, err
		}
		return outcome, nil
	}

	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	if err := enc.Encode(cat); err != nil {
		return outcome, fmt.Errorf("encode catalog: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, &body)
	if err != nil {
		return outcome, fmt.Errorf("create sync push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	authenticated, err := addBearerAuthFromEnv(req, tokenEnv)
	if err != nil {
		return outcome, err
	}
	outcome.Authenticated = authenticated
	resp, err := syncHTTPClient.Do(req)
	if err != nil {
		return outcome, fmt.Errorf("push sync target: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	outcome.Status = resp.Status
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return outcome, httpStatusError("push sync target", resp)
	}
	return outcome, nil
}

func describeCatalogSyncTarget(target string) (catalogSyncWriteOutcome, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return catalogSyncWriteOutcome{}, errors.New("library sync requires --target")
	}
	if !isURITarget(target) || strings.HasPrefix(strings.ToLower(target), "file:") {
		path, err := fileSyncTargetPath(target)
		if err != nil {
			return catalogSyncWriteOutcome{}, err
		}
		return catalogSyncWriteOutcome{Path: path, Method: "file"}, nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return catalogSyncWriteOutcome{}, fmt.Errorf("parse sync target: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return catalogSyncWriteOutcome{}, fmt.Errorf("unsupported sync target scheme %q; push supports file://, http://, and https://", u.Scheme)
	}
	if u.Fragment != "" {
		return catalogSyncWriteOutcome{}, errors.New("HTTP sync target must not include a fragment")
	}
	return catalogSyncWriteOutcome{Method: http.MethodPut}, nil
}

func addBearerAuthFromEnv(req *http.Request, tokenEnv string) (bool, error) {
	tokenEnv = strings.TrimSpace(tokenEnv)
	if tokenEnv == "" {
		return false, nil
	}
	token := strings.TrimSpace(os.Getenv(tokenEnv))
	if token == "" {
		if tokenEnv == defaultSyncTokenEnv {
			return false, nil
		}
		return false, fmt.Errorf("sync token environment variable %s is not set", tokenEnv)
	}
	if req.URL == nil || req.URL.Scheme != "https" {
		if strings.TrimSpace(os.Getenv(allowInsecureSyncHTTPEnv)) != "1" {
			return false, fmt.Errorf("refusing to send bearer auth to a non-HTTPS sync target; set %s=1 to allow local or otherwise trusted HTTP sync", allowInsecureSyncHTTPEnv)
		}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return true, nil
}

func httpStatusError(action string, resp *http.Response) error {
	detailBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	detail := strings.TrimSpace(string(detailBytes))
	if detail == "" {
		return fmt.Errorf("%s: HTTP %s", action, resp.Status)
	}
	return fmt.Errorf("%s: HTTP %s: %s", action, resp.Status, detail)
}

func fileSyncTargetPath(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("library sync requires --target")
	}
	if !isURITarget(target) && !strings.HasPrefix(strings.ToLower(target), "file:") {
		return filepath.Clean(target), nil
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("parse sync target: %w", err)
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported sync target scheme %q; only file:// is supported", u.Scheme)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("file sync target must not include query or fragment")
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", fmt.Errorf("unsupported file sync target host %q", u.Host)
	}
	targetPath := u.EscapedPath()
	if targetPath == "" && u.Opaque != "" {
		targetPath = u.Opaque
	}
	if targetPath == "" {
		return "", errors.New("file sync target path is empty")
	}
	decodedPath, err := url.PathUnescape(targetPath)
	if err != nil {
		return "", fmt.Errorf("decode file sync target path: %w", err)
	}
	return filepath.Clean(filepath.FromSlash(decodedPath)), nil
}

func isURITarget(target string) bool {
	return strings.Contains(target, "://")
}

func writeCatalogFile(path string, cat *catalog.Catalog) error {
	if path == "" {
		return errors.New("catalog target path is empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create catalog target directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".modfetch-catalog-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp catalog: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(catalogTargetMode(path)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set catalog permissions: %w", err)
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cat); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write catalog: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close catalog: %w", err)
	}
	if err := replaceCatalogFile(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func catalogTargetMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode().Perm()
	}
	return 0o644
}

func replaceCatalogFile(tmpPath, path string) error {
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return fmt.Errorf("replace catalog target: %w", err)
	} else if _, statErr := os.Stat(path); statErr != nil {
		return fmt.Errorf("replace catalog target: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove existing catalog target: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace catalog target after removing existing file: %w", err)
	}
	return nil
}

func inputReader(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}
