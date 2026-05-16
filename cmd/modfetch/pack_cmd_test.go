package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackListAndShow(t *testing.T) {
	out := captureStdout(t, func() {
		if err := handlePack(context.Background(), []string{"list"}); err != nil {
			t.Fatalf("pack list: %v", err)
		}
	})
	for _, want := range []string{"llm-smoke", "embedding-smoke", "modfetch pack download --id ID"} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack list missing %q in:\n%s", want, out)
		}
	}
	out = captureStdout(t, func() {
		if err := handlePack(context.Background(), []string{"show", "llm-smoke"}); err != nil {
			t.Fatalf("pack show: %v", err)
		}
	})
	for _, want := range []string{"Tiny LLM", "tokenizer.json", "hf://gpt2/tokenizer.json"} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack show missing %q in:\n%s", want, out)
		}
	}
}

func TestPackExportWritesBatchManifest(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeStarterTestConfig(t, cfgPath, tmp)
	outPath := filepath.Join(tmp, "pack.yml")

	if err := handlePack(context.Background(), []string{
		"export",
		"--config", cfgPath,
		"--id", "llm-smoke",
		"--output", outPath,
	}); err != nil {
		t.Fatalf("pack export: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(body)
	for _, want := range []string{"version: 1", "hf://gpt2/config.json", filepath.Join(tmp, "downloads", "llm-smoke", "config.json")} {
		if !strings.Contains(text, want) {
			t.Fatalf("manifest missing %q in:\n%s", want, text)
		}
	}
}

func TestPackDownloadDryRunPrintsPlan(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeStarterTestConfig(t, cfgPath, tmp)

	out := captureStdout(t, func() {
		err := handlePack(context.Background(), []string{
			"download",
			"--config", cfgPath,
			"--id", "embedding-smoke",
			"--dry-run",
		})
		if err != nil {
			t.Fatalf("pack download dry-run: %v", err)
		}
	})
	for _, want := range []string{"Pack download plan", "hf-internal-testing/tiny-random-bert", "model.safetensors"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run missing %q in:\n%s", want, out)
		}
	}
}
