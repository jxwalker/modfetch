package main

import (
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
//   --path PATH       verify a specific file in state
//   --all             verify all completed downloads
//   --safetensors     additionally perform a minimal .safetensors structure check
func handleVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	pathFlag := fs.String("path", "", "Specific file to verify (optional)")
	all := fs.Bool("all", false, "Verify all completed downloads")
	checkST := fs.Bool("safetensors", false, "Sanity-check .safetensors structure")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	_ = logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()

	report := map[string]any{}
	verifyOne := func(row state.DownloadRow) error {
		f, err := os.Open(row.Dest)
		if err != nil { return err }
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil { return err }
		actual := hex.EncodeToString(h.Sum(nil))
		status := "verified"
		if row.ExpectedSHA256 != "" && !equalFoldHex(row.ExpectedSHA256, actual) {
			status = "checksum_mismatch"
		}
		_ = st.UpsertDownload(state.DownloadRow{URL: row.URL, Dest: row.Dest, ExpectedSHA256: row.ExpectedSHA256, ActualSHA256: actual, ETag: row.ETag, LastModified: row.LastModified, Size: row.Size, Status: status})
		res := map[string]any{"path": row.Dest, "sha256": actual, "status": status, "size": row.Size}
		if *checkST && (filepath.Ext(row.Dest) == ".safetensors" || filepath.Ext(row.Dest) == ".sft") {
			ok, herr := sanityCheckSafeTensors(row.Dest)
			res["safetensors_ok"] = ok
			if herr != nil { res["safetensors_error"] = herr.Error() }
		}
		report[row.Dest] = res
		return nil
	}

	if *pathFlag != "" {
		rows, err := st.ListDownloads()
		if err != nil { return err }
		var found *state.DownloadRow
		for i := range rows { if rows[i].Dest == *pathFlag { found = &rows[i]; break } }
		if found == nil { return fmt.Errorf("no download record for %s", *pathFlag) }
		if err := verifyOne(*found); err != nil { return err }
	} else if *all {
		rows, err := st.ListDownloads()
		if err != nil { return err }
		for _, r := range rows {
			if strings.EqualFold(r.Status, "complete") || strings.EqualFold(r.Status, "verified") || strings.EqualFold(r.Status, "checksum_mismatch") {
				if err := verifyOne(r); err != nil { return err }
			}
		}
	} else {
		return errors.New("use --path or --all")
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout); enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	for _, v := range report {
		m := v.(map[string]any)
		fmt.Printf("%s size=%d sha256=%s status=%s\n", m["path"], m["size"], m["sha256"], m["status"])
		if *checkST { fmt.Printf("  safetensors_ok=%v error=%v\n", m["safetensors_ok"], m["safetensors_error"]) }
	}
	return nil
}

// sanityCheckSafeTensors: minimal header sanity checks
func sanityCheckSafeTensors(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil { return false, err }
	defer f.Close()
	var hdrLen uint64
	if err := binary.Read(f, binary.LittleEndian, &hdrLen); err != nil { return false, err }
	fi, err := f.Stat(); if err != nil { return false, err }
	if hdrLen == 0 || hdrLen > uint64(fi.Size()) || hdrLen > 64*1024*1024 { return false, fmt.Errorf("invalid safetensors header length: %d", hdrLen) }
	hdr := make([]byte, hdrLen)
	if _, err := io.ReadFull(f, hdr); err != nil { return false, err }
	var js any
	if err := json.Unmarshal(hdr, &js); err != nil { return false, err }
	return true, nil
}

// equalFoldHex: case-insensitive hex compare
func equalFoldHex(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if len(a) != len(b) { return false }
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'F' { ca += 32 }
		if cb >= 'A' && cb <= 'F' { cb += 32 }
		if ca != cb { return false }
	}
	return true
}

