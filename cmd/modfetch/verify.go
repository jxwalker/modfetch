package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

// handleVerify verifies downloaded files.
// Options:
//
//	--path PATH       verify a specific file in state
//	--all             verify all completed downloads
//	--safetensors     additionally perform a minimal .safetensors structure check
func handleVerify(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	pathFlag := fs.String("path", "", "Specific file to verify (optional)")
	all := fs.Bool("all", false, "Verify all completed downloads")
	checkST := fs.Bool("safetensors", false, "Sanity-check .safetensors structure")
	checkSTDeep := fs.Bool("safetensors-deep", false, "Deep-verify .safetensors: header coverage and offsets")
	scanDir := fs.String("scan-dir", "", "Recursively scan a directory for .safetensors and verify")
	repair := fs.Bool("repair", false, "When used with --scan-dir and --safetensors-deep: trim extra trailing bytes to declared size")
	quarantineIncomplete := fs.Bool("quarantine-incomplete", false, "When used with --scan-dir: move incomplete files to .incomplete")
	onlyErrors := fs.Bool("only-errors", false, "Show only files with errors or non-verified status")
	summary := fs.Bool("summary", false, "Print a summary of total scanned and error count")
	fixSidecar := fs.Bool("fix-sidecar", false, "Rewrite .sha256 sidecar with actual hash for verified files")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *cfgPath == "" {
		if env := os.Getenv("MODFETCH_CONFIG"); env != "" {
			*cfgPath = env
		} else {
			if h, err := os.UserHomeDir(); err == nil && h != "" {
				*cfgPath = filepath.Join(h, ".config", "modfetch", "config.yml")
			}
		}
	}
	if _, err := os.Stat(*cfgPath); err != nil {
		return fmt.Errorf("config file not found: %s", *cfgPath)
	}
	c, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	_ = logging.New(*logLevel, *jsonOut)
	// If scanning an arbitrary directory, DB is not required; open only for state-backed modes.
	var st *state.DB
	if *scanDir == "" {
		var err error
		st, err = state.Open(c)
		if err != nil {
			return err
		}
		defer st.SQL.Close()
	}

	report := map[string]any{}
	verifyOne := func(row state.DownloadRow) error {
		f, err := os.Open(row.Dest)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		actual := hex.EncodeToString(h.Sum(nil))
		status := "verified"
		if row.ExpectedSHA256 != "" && !equalFoldHex(row.ExpectedSHA256, actual) {
			status = "checksum_mismatch"
		}
		_ = st.UpsertDownload(state.DownloadRow{URL: row.URL, Dest: row.Dest, ExpectedSHA256: row.ExpectedSHA256, ActualSHA256: actual, ETag: row.ETag, LastModified: row.LastModified, Size: row.Size, Status: status})
		res := map[string]any{"path": row.Dest, "sha256": actual, "status": status, "size": row.Size}
		// Optionally rewrite sidecar if verified
		if *fixSidecar && status == "verified" {
			sc := row.Dest + ".sha256"
			content := actual + "  " + filepath.Base(row.Dest) + "\n"
			if err := os.WriteFile(sc, []byte(content), 0o644); err == nil {
				res["sidecar_written"] = true
			} else {
				res["sidecar_error"] = err.Error()
			}
		}
		if *checkST && (filepath.Ext(row.Dest) == ".safetensors" || filepath.Ext(row.Dest) == ".sft") {
			ok, herr := sanityCheckSafeTensors(row.Dest)
			res["safetensors_ok"] = ok
			if herr != nil {
				res["safetensors_error"] = herr.Error()
			}
		}
		if *checkSTDeep && (filepath.Ext(row.Dest) == ".safetensors" || filepath.Ext(row.Dest) == ".sft") {
			ok, need, derr := deepVerifySafeTensors(row.Dest)
			res["safetensors_deep_ok"] = ok
			res["safetensors_declared_size"] = need
			if derr != nil {
				res["safetensors_deep_error"] = derr.Error()
			}
		}
		report[row.Dest] = res
		return nil
	}

	if *scanDir != "" {
		// directory scan mode (no DB required)
		res, err := scanSafetensorsDir(*scanDir, *checkSTDeep, *repair, *quarantineIncomplete)
		if err != nil {
			return err
		}
		// merge results into report
		for k, v := range res {
			report[k] = v
		}
	} else if *pathFlag != "" {
		rows, err := st.ListDownloads()
		if err != nil {
			return err
		}
		var found *state.DownloadRow
		for i := range rows {
			if rows[i].Dest == *pathFlag {
				found = &rows[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("no download record for %s", *pathFlag)
		}
		if err := verifyOne(*found); err != nil {
			return err
		}
	} else if *all {
		rows, err := st.ListDownloads()
		if err != nil {
			return err
		}
		for _, r := range rows {
			if strings.EqualFold(r.Status, "complete") || strings.EqualFold(r.Status, "verified") || strings.EqualFold(r.Status, "checksum_mismatch") {
				if err := verifyOne(r); err != nil {
					return err
				}
			}
		}
	} else {
		return errors.New("use --scan-dir or --path or --all")
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	// Human-readable output with optional filtering and summary
	total := 0
	errCount := 0
	errPaths := []string{}
	isErr := func(m map[string]any) bool {
		// status not verified
		if st, ok := m["status"].(string); ok && st != "" && st != "verified" {
			return true
		}
		// safetensors basic
		if *checkST {
			if ok, okp := m["safetensors_ok"].(bool); okp && !ok {
				return true
			}
			if _, has := m["safetensors_error"]; has {
				return true
			}
		}
		// safetensors deep
		if *checkSTDeep {
			if ok, okp := m["safetensors_deep_ok"].(bool); okp && !ok {
				return true
			}
			if _, has := m["safetensors_deep_error"]; has {
				return true
			}
		}
		return false
	}
	for _, v := range report {
		m := v.(map[string]any)
		total++
		err := isErr(m)
		if err {
			errCount++
			if p, ok := m["path"].(string); ok {
				errPaths = append(errPaths, p)
			}
		}
		if *onlyErrors && !err {
			continue
		}
		path := m["path"]
		if _, hasSize := m["size"]; hasSize {
			fmt.Printf("%s size=%v sha256=%v status=%v\n", path, m["size"], m["sha256"], m["status"])
		} else {
			fmt.Printf("%s\n", path)
		}
		if *checkST {
			fmt.Printf("  safetensors_ok=%v error=%v\n", m["safetensors_ok"], m["safetensors_error"])
		}
		if *checkSTDeep {
			fmt.Printf("  safetensors_deep_ok=%v declared_size=%v error=%v", m["safetensors_deep_ok"], m["safetensors_declared_size"], m["safetensors_deep_error"])
			if rv, ok := m["repaired"]; ok {
				fmt.Printf(" repaired=%v", rv)
			}
			if qv, ok := m["quarantined"]; ok {
				fmt.Printf(" quarantined=%v", qv)
			}
			fmt.Print("\n")
		}
	}
	if *summary {
		fmt.Printf("Summary: scanned=%d errors=%d\n", total, errCount)
		if errCount > 0 {
			fmt.Println("Error paths:")
			for _, p := range errPaths {
				fmt.Printf("  %s\n", p)
			}
		}
	}
	return nil
}

// sanityCheckSafeTensors: minimal header sanity checks
func sanityCheckSafeTensors(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var hdrLen uint64
	if err := binary.Read(f, binary.LittleEndian, &hdrLen); err != nil {
		return false, err
	}
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	if hdrLen == 0 || hdrLen > uint64(fi.Size()) || hdrLen > 64*1024*1024 {
		return false, fmt.Errorf("invalid safetensors header length: %d", hdrLen)
	}
	hdr := make([]byte, hdrLen)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, err
	}
	var js any
	if err := json.Unmarshal(hdr, &js); err != nil {
		return false, err
	}
	return true, nil
}

// deepVerifySafeTensors performs coverage and offsets validation.
// Returns ok, declared_total_size, error (nil if ok)
func deepVerifySafeTensors(p string) (bool, int64, error) {
	f, err := os.Open(p)
	if err != nil {
		return false, 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return false, 0, err
	}
	if fi.Size() < 8 {
		return false, 0, fmt.Errorf("file too small: %d", fi.Size())
	}
	var hdrLen uint64
	if err := binary.Read(f, binary.LittleEndian, &hdrLen); err != nil {
		return false, 0, err
	}
	if hdrLen == 0 || hdrLen > uint64(fi.Size()-8) || hdrLen > 64*1024*1024 {
		return false, 0, fmt.Errorf("invalid safetensors header length: %d", hdrLen)
	}
	hdr := make([]byte, hdrLen)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return false, 0, err
	}
	var meta map[string]any
	if err := json.Unmarshal(hdr, &meta); err != nil {
		return false, 0, fmt.Errorf("header json: %w", err)
	}
	dataLen := fi.Size() - 8 - int64(hdrLen)
	if dataLen < 0 {
		return false, 0, fmt.Errorf("negative data length")
	}
	var maxEnd int64
	for k, v := range meta {
		lk := strings.ToLower(k)
		if lk == "metadata" || lk == "__metadata__" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		off, ok := m["data_offsets"].([]any)
		if !ok || len(off) != 2 {
			continue
		}
		start, ok1 := toInt64(off[0])
		end, ok2 := toInt64(off[1])
		if !ok1 || !ok2 {
			return false, 0, fmt.Errorf("tensor %q: invalid data_offsets", k)
		}
		if start < 0 || end < 0 || end < start {
			return false, 0, fmt.Errorf("tensor %q: invalid range %d-%d", k, start, end)
		}
		if end > dataLen {
			return false, 0, fmt.Errorf("tensor %q: data end %d beyond data length %d", k, end, dataLen)
		}
		// Optional: validate dtype/shape size if present
		if dtRaw, ok := m["dtype"].(string); ok {
			if shp, ok := m["shape"].([]any); ok {
				exp := dtypeBytes(dtRaw)
				if exp > 0 {
					var cnt int64 = 1
					for _, dim := range shp {
						d, ok := toInt64(dim)
						if !ok || d <= 0 {
							cnt = 0
							break
						}
						cnt *= d
					}
					if cnt > 0 {
						sz := end - start
						if sz != cnt*exp {
							return false, 0, fmt.Errorf("tensor %q: size %d != %d*%d", k, sz, cnt, exp)
						}
					}
				}
			}
		}
		if end > maxEnd {
			maxEnd = end
		}
	}
	declared := int64(8) + int64(hdrLen) + maxEnd
	if fi.Size() < declared {
		return false, declared, fmt.Errorf("incomplete: have=%d need=%d", fi.Size(), declared)
	}
	if fi.Size() > declared {
		return false, declared, fmt.Errorf("extra bytes: have=%d need=%d", fi.Size(), declared)
	}
	return true, declared, nil
}

func dtypeBytes(dt string) int64 {
	switch strings.ToUpper(strings.TrimSpace(dt)) {
	case "F64":
		return 8
	case "F32":
		return 4
	case "F16", "BF16":
		return 2
	case "F8", "F8_E4M3FN", "F8_E5M2":
		return 1
	case "I64":
		return 8
	case "I32":
		return 4
	case "I16":
		return 2
	case "I8", "U8", "BOOL":
		return 1
	default:
		return 0
	}
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	default:
		return 0, false
	}
}

// Scan a directory recursively for .safetensors files and deep-verify them.
// If repair is true, trims extra bytes to declared size when encountered.
// If quarantineIncomplete is true, moves incomplete files to .incomplete.
func scanSafetensorsDir(root string, deep bool, repair bool, quarantineIncomplete bool) (map[string]any, error) {
	if root == "" {
		return nil, errors.New("scan-dir is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	walkErr := filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		low := strings.ToLower(p)
		if !(strings.HasSuffix(low, ".safetensors") || strings.HasSuffix(low, ".sft")) {
			return nil
		}
		ok, need, derr := deepVerifySafeTensors(p)
		m := map[string]any{"path": p, "safetensors_deep_ok": ok, "safetensors_declared_size": need}
		if derr != nil {
			m["safetensors_deep_error"] = derr.Error()
		}
		// Attempt repair if requested
		if deep && repair && derr != nil && strings.Contains(strings.ToLower(derr.Error()), "extra bytes") && need > 0 {
			fi, statErr := os.Stat(p)
			if statErr == nil && fi.Size() > need {
				f, oerr := os.OpenFile(p, os.O_RDWR, 0)
				if oerr == nil {
					if terr := f.Truncate(need); terr == nil {
						m["repaired"] = true
						// re-run verify
						ok2, need2, derr2 := deepVerifySafeTensors(p)
						m["safetensors_deep_ok"] = ok2
						m["safetensors_declared_size"] = need2
						if derr2 != nil {
							m["safetensors_deep_error"] = derr2.Error()
						} else {
							delete(m, "safetensors_deep_error")
						}
					}
					_ = f.Close()
				}
			}
		}
		if deep && quarantineIncomplete && derr != nil && strings.Contains(strings.ToLower(derr.Error()), "incomplete") {
			q := p + ".incomplete"
			if rerr := os.Rename(p, q); rerr == nil {
				m["quarantined"] = q
			}
		}
		out[p] = m
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

// equalFoldHex: case-insensitive hex compare
func equalFoldHex(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'F' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'F' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
