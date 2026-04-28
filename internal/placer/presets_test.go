package placer

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestApplyPresetAddsAppsAndMappings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cfg := &config.Config{}
	if err := ApplyPreset(cfg, "comfyui"); err != nil {
		t.Fatalf("apply preset: %v", err)
	}

	app := cfg.Placement.Apps["comfyui"]
	if app.Base != filepath.Join(home, "ComfyUI") {
		t.Fatalf("unexpected comfyui base %q", app.Base)
	}
	if app.Paths["checkpoints"] != "models/checkpoints" {
		t.Fatalf("missing checkpoints path: %+v", app.Paths)
	}
	targets, err := ComputeTargets(cfg, "sd.lora")
	if err != nil {
		t.Fatalf("compute targets: %v", err)
	}
	want := filepath.Join(home, "ComfyUI", "models", "loras")
	if len(targets) != 1 || targets[0] != want {
		t.Fatalf("targets = %+v, want %q", targets, want)
	}
}

func TestApplyPresetsMergesWithoutDuplicatingTargets(t *testing.T) {
	cfg := &config.Config{}
	if err := ApplyPresets(cfg, []string{"comfyui", "comfyui"}); err != nil {
		t.Fatalf("apply presets: %v", err)
	}
	targets, err := ComputeTargets(cfg, "sd.checkpoint")
	if err != nil {
		t.Fatalf("compute targets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected one deduped target, got %+v", targets)
	}
}

func TestApplyPresetReportsUnknownName(t *testing.T) {
	err := ApplyPreset(&config.Config{}, "missing")
	if err == nil || !strings.Contains(err.Error(), "unknown placement preset") {
		t.Fatalf("expected unknown preset error, got %v", err)
	}
}

func TestParsePresetList(t *testing.T) {
	got := ParsePresetList("comfyui, ollama none\tforge")
	want := []string{"comfyui", "ollama", "forge"}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}
