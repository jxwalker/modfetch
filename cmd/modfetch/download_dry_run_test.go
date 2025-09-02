package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadDryRun_SkipsPreflightAndAttachesAuth(t *testing.T) {
	tmp := t.TempDir()
	dataRoot := filepath.Join(tmp, "data")
	dlRoot := filepath.Join(tmp, "dl")
	_ = os.MkdirAll(dataRoot, 0o755)
	_ = os.MkdirAll(dlRoot, 0o755)
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfg := "version: 1\n" +
		"general:\n" +
		"  data_root: \"" + strings.ReplaceAll(dataRoot, "\\", "\\\\") + "\"\n" +
		"  download_root: \"" + strings.ReplaceAll(dlRoot, "\\", "\\\\") + "\"\n" +
		"network:\n  disable_auth_preflight: true\n" +
		"sources:\n  huggingface:\n    enabled: true\n    token_env: \"HF_TOKEN\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil { t.Fatal(err) }
	old := os.Getenv("HF_TOKEN")
	_ = os.Setenv("HF_TOKEN", "DUMMY")
	t.Cleanup(func(){ _ = os.Setenv("HF_TOKEN", old) })

	// Capture stdout
	oldStdout := os.Stdout
	outFile, err := os.CreateTemp(tmp, "stdout-*.json")
	if err != nil { t.Fatal(err) }
	defer outFile.Close()
	os.Stdout = outFile
	t.Cleanup(func(){ os.Stdout = oldStdout })

	args := []string{"--config", cfgPath, "--url", "https://huggingface.co", "--dry-run", "--summary-json"}
	if err := handleDownload(context.Background(), args); err != nil {
		t.Fatalf("handleDownload --dry-run failed: %v", err)
	}
	// Flush and read
	_ = outFile.Sync()
	b, err := os.ReadFile(outFile.Name())
	if err != nil { t.Fatal(err) }

	var plan map[string]any
	if err := json.Unmarshal(b, &plan); err != nil {
		t.Fatalf("decode json: %v (raw=%s)", err, string(b))
	}
	if got := strings.ToLower(getString(plan, "host")); got != "huggingface.co" {
		t.Fatalf("host = %q", got)
	}
	if got := getBool(plan, "auth_attached"); !got {
		t.Fatalf("expected auth_attached true")
	}
	if got := getBool(plan, "preflight_skipped"); !got {
		t.Fatalf("expected preflight_skipped true")
	}
	def := getString(plan, "default_dest")
	if !strings.HasPrefix(def, dlRoot) {
		t.Fatalf("default_dest not under download_root: %s", def)
	}
	if _, err := os.Stat(def); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected no file created at default_dest, got err=%v", err)
	}
}

func TestDownloadDryRun_DirectURL_NoAuth(t *testing.T) {
	tmp := t.TempDir()
	dlRoot := filepath.Join(tmp, "dl")
	_ = os.MkdirAll(dlRoot, 0o755)
	cfgPath := filepath.Join(tmp, "cfg.yml")
	cfg := "version: 1\n" +
		"general:\n  data_root: \"" + strings.ReplaceAll(tmp, "\\", "\\\\") + "\"\n  download_root: \"" + strings.ReplaceAll(dlRoot, "\\", "\\\\") + "\"\n" +
		"network:\n  disable_auth_preflight: true\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil { t.Fatal(err) }

	oldStdout := os.Stdout
	outFile, err := os.CreateTemp(tmp, "stdout-*.json")
	if err != nil { t.Fatal(err) }
	defer outFile.Close()
	os.Stdout = outFile
	t.Cleanup(func(){ os.Stdout = oldStdout })

	args := []string{"--config", cfgPath, "--url", "https://example.com/file.bin", "--dry-run", "--summary-json"}
	if err := handleDownload(context.Background(), args); err != nil {
		t.Fatalf("handleDownload --dry-run failed: %v", err)
	}
	_ = outFile.Sync()
	b, err := os.ReadFile(outFile.Name())
	if err != nil { t.Fatal(err) }
	var plan map[string]any
	if err := json.Unmarshal(b, &plan); err != nil {
		t.Fatalf("decode json: %v (raw=%s)", err, string(b))
	}
	if got := strings.ToLower(getString(plan, "host")); got != "example.com" {
		t.Fatalf("host = %q", got)
	}
	if got := getBool(plan, "auth_attached"); got {
		t.Fatalf("expected auth_attached false")
	}
	if got := getBool(plan, "preflight_skipped"); !got {
		t.Fatalf("expected preflight_skipped true")
	}
	def := getString(plan, "default_dest")
	if !strings.HasPrefix(def, dlRoot) {
		t.Fatalf("default_dest not under download_root: %s", def)
	}
	if !strings.HasSuffix(strings.ToLower(def), string(os.PathSeparator)+"file.bin") {
		t.Fatalf("default_dest should end with file.bin, got %s", def)
	}
	if _, err := os.Stat(def); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected no file created at default_dest, got err=%v", err)
	}
}

// helpers
func getString(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		if s, ok2 := v.(string); ok2 { return s }
	}
	return ""
}
func getBool(m map[string]any, k string) bool {
	if v, ok := m[k]; ok {
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return strings.ToLower(t) == "true"
		}
	}
	return false
}
