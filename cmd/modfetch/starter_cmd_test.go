package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStarterListPrintsBeginnerCatalog(t *testing.T) {
	out := captureStdout(t, func() {
		if err := handleStarter(context.Background(), []string{"list"}); err != nil {
			t.Fatalf("starter list: %v", err)
		}
	})
	for _, want := range []string{"gpt2-config", "gpt2-tokenizer", "modfetch starter download --id ID"} {
		if !strings.Contains(out, want) {
			t.Fatalf("starter list missing %q in:\n%s", want, out)
		}
	}
}

func TestStarterDownloadDryRunUsesDownloadPipeline(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeStarterTestConfig(t, cfgPath, tmp)

	out := captureStdout(t, func() {
		err := handleStarter(context.Background(), []string{
			"download",
			"--id", "gpt2-config",
			"--config", cfgPath,
			"--dry-run",
			"--summary-json",
			"--quiet",
		})
		if err != nil {
			t.Fatalf("starter download dry-run: %v", err)
		}
	})
	for _, want := range []string{"starter://gpt2-config", "huggingface.co/gpt2/resolve/main/config.json", "config.json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q in:\n%s", want, out)
		}
	}
}

func writeStarterTestConfig(t *testing.T, cfgPath, root string) {
	t.Helper()
	body := "version: 1\n" +
		"general:\n" +
		"  data_root: " + strconv.Quote(filepath.Join(root, "data")) + "\n" +
		"  download_root: " + strconv.Quote(filepath.Join(root, "downloads")) + "\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
