package classifier

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestDetectMagicGGUF(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "model.bin")
	if err := os.WriteFile(p, []byte("GGUF"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	got := Detect(nil, p)
	if got != "llm.gguf" {
		t.Fatalf("expected llm.gguf, got %s", got)
	}
}

func TestDetectCustomRuleOverrides(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "special.gguf")
	if err := os.WriteFile(p, []byte("GGUF"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	cfg := &config.Config{
		Classifier: config.ClassifierConfig{
			Rules: []config.ClassifierRule{{Regex: "^special", Type: "sd.lora"}},
		},
	}
	got := Detect(cfg, p)
	if got != "sd.lora" {
		t.Fatalf("expected sd.lora, got %s", got)
	}
}

func TestAnalyzeReportsAmbiguousSafetensorsConfidence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "model.safetensors")
	if err := os.WriteFile(p, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := Analyze(nil, p)
	if got.Type != "sd.checkpoint" || got.Confidence != "low" {
		t.Fatalf("expected low-confidence checkpoint, got %+v", got)
	}
}

func TestAnalyzeReportsFilenameHeuristicConfidence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "portrait-lora.safetensors")
	if err := os.WriteFile(p, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	got := Analyze(nil, p)
	if got.Type != "sd.lora" || got.Confidence != "medium" {
		t.Fatalf("expected medium-confidence LoRA, got %+v", got)
	}
}
