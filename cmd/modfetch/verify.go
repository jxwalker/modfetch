package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"modfetch/internal/config"
	"modfetch/internal/logging"
	"modfetch/internal/state"
)

func handleVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	pathFlag := fs.String("path", "", "Specific file to verify (optional)")
	all := fs.Bool("all", false, "Verify all completed downloads")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)
	st, err := state.Open(c)
	if err != nil { return err }
	defer st.SQL.Close()

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
		return st.UpsertDownload(state.DownloadRow{URL: row.URL, Dest: row.Dest, ExpectedSHA256: row.ExpectedSHA256, ActualSHA256: actual, ETag: row.ETag, LastModified: row.LastModified, Size: row.Size, Status: status})
	}

	if *pathFlag != "" {
		rows, err := st.ListDownloads()
		if err != nil { return err }
		var found bool
		for _, r := range rows {
			if r.Dest == *pathFlag {
				found = true
				if err := verifyOne(r); err != nil { return err }
				log.Infof("verified: %s", r.Dest)
				break
			}
		}
		if !found { return fmt.Errorf("no download record for %s", *pathFlag) }
		return nil
	}

	if *all {
		rows, err := st.ListDownloads()
		if err != nil { return err }
		for _, r := range rows {
			if r.Status == "complete" || r.Status == "checksum_mismatch" || r.Status == "verified" {
				if err := verifyOne(r); err != nil { return err }
				log.Infof("verified: %s", r.Dest)
			}
		}
		return nil
	}

	return errors.New("use --path or --all")
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

