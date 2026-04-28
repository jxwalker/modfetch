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
	w.inputs[3].SetValue("comfyui,ollama")
	cfg := w.buildConfig()

	if cfg.Placement.Apps["comfyui"].Base != filepath.Join(home, "ComfyUI") {
		t.Fatalf("expected comfyui preset app, got %+v", cfg.Placement.Apps["comfyui"])
	}
	if cfg.Placement.Apps["ollama"].Base != filepath.Join(home, ".ollama") {
		t.Fatalf("expected ollama preset app, got %+v", cfg.Placement.Apps["ollama"])
	}
}
