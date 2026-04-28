package configwizard

import (
	"path/filepath"
	"testing"
)

func TestWizardAppliesPlacementPresets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	w := New(nil)
	setWizardInput(t, w, "placement.presets", "comfyui,ollama")
	cfg, err := w.buildConfig()
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	if cfg.Placement.Apps["comfyui"].Base != filepath.Join(home, "ComfyUI") {
		t.Fatalf("expected comfyui preset app, got %+v", cfg.Placement.Apps["comfyui"])
	}
	if cfg.Placement.Apps["ollama"].Base != filepath.Join(home, ".ollama") {
		t.Fatalf("expected ollama preset app, got %+v", cfg.Placement.Apps["ollama"])
	}
}

func TestWizardReportsUnknownPlacementPreset(t *testing.T) {
	w := New(nil)
	setWizardInput(t, w, "placement.presets", "missing")

	if _, err := w.buildConfig(); err == nil {
		t.Fatal("expected unknown preset error")
	}
}

func setWizardInput(t *testing.T, w *Wizard, placeholder, value string) {
	t.Helper()
	for i, label := range w.labels {
		if label == placeholder {
			w.inputs[i].SetValue(value)
			return
		}
	}
	t.Fatalf("wizard input %q not found", placeholder)
}
