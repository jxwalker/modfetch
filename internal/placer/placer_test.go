package placer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
)

func TestPlaceSymlinkCheckpoint(t *testing.T) {
	tmp := t.TempDir()
	// Build config mapping sd.checkpoint -> comfyui.checkpoints
	cfgPath := filepath.Join(tmp, "cfg.yml")
	checkpointDir := filepath.Join(tmp, "ComfyUI", "models", "checkpoints")
	cfgYaml := []byte("version: 1\n" +
		"general:\n" +
		"  data_root: \"" + tmp + "/data\"\n" +
		"  download_root: \"" + tmp + "/dl\"\n" +
		"  placement_mode: symlink\n" +
		"  allow_overwrite: true\n" +
		"placement:\n" +
		"  apps:\n" +
		"    comfyui:\n" +
		"      base: \"" + filepath.Join(tmp, "ComfyUI") + "\"\n" +
		"      paths:\n" +
		"        checkpoints: models/checkpoints\n" +
		"  mapping:\n" +
		"    - match: sd.checkpoint\n" +
		"      targets:\n" +
		"        - app: comfyui\n" +
		"          path_key: checkpoints\n")
	if err := os.WriteFile(cfgPath, cfgYaml, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}

	// Create a dummy safetensors file to classify as sd.checkpoint
	src := filepath.Join(tmp, "model.safetensors")
	if err := os.WriteFile(src, []byte("dummy-weights"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := Place(cfg, src, "", "")
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	dst := paths[0]
	// Expect symlink exists and points somewhere
	fi, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("dst lstat: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink at %s", dst)
	}
	// And parent directory is the configured checkpoints dir
	if filepath.Dir(dst) != checkpointDir {
		t.Fatalf("expected parent %s, got %s", checkpointDir, filepath.Dir(dst))
	}
}

func TestPlaceCopySkipsSameContentAndRejectsDifferentExistingFile(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{
		General: config.General{PlacementMode: "copy", AllowOverwrite: false},
		Placement: config.Placement{
			Apps: map[string]config.AppPlacement{
				"app": {Base: tmp, Paths: map[string]string{"models": "models"}},
			},
			Mapping: []config.MappingRule{
				{Match: "llm", Targets: []config.MappingTarget{{App: "app", PathKey: "models"}}},
			},
		},
	}
	src := filepath.Join(tmp, "src", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(src, []byte("same"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(tmp, "models", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	if err := os.WriteFile(dst, []byte("same"), 0o644); err != nil {
		t.Fatalf("write existing same: %v", err)
	}

	placed, err := Place(cfg, src, "llm", "copy")
	if err != nil {
		t.Fatalf("place same content: %v", err)
	}
	if len(placed) != 1 || placed[0] != dst {
		t.Fatalf("expected existing destination to be reported, got %+v", placed)
	}

	if err := os.WriteFile(dst, []byte("different"), 0o644); err != nil {
		t.Fatalf("write existing different: %v", err)
	}
	_, err = Place(cfg, src, "llm", "copy")
	if err == nil || !strings.Contains(err.Error(), "destination exists and differs") {
		t.Fatalf("expected differing destination error, got %v", err)
	}
}

func TestPlaceCopyOverwriteAndUnknownMode(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{
		General: config.General{PlacementMode: "copy", AllowOverwrite: true},
		Placement: config.Placement{
			Apps: map[string]config.AppPlacement{
				"app": {Base: tmp, Paths: map[string]string{"models": "models"}},
			},
			Mapping: []config.MappingRule{
				{Match: "llm", Targets: []config.MappingTarget{{App: "app", PathKey: "models"}}},
			},
		},
	}
	src := filepath.Join(tmp, "src", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(tmp, "models", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old dst: %v", err)
	}

	placed, err := Place(cfg, src, "llm", "copy")
	if err != nil {
		t.Fatalf("place overwrite copy: %v", err)
	}
	if len(placed) != 1 || placed[0] != dst {
		t.Fatalf("unexpected placed paths: %+v", placed)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("expected overwritten content, got %q", string(data))
	}

	if _, err := Place(cfg, src, "llm", "mystery"); err == nil || !strings.Contains(err.Error(), "unknown placement mode") {
		t.Fatalf("expected unknown mode error, got %v", err)
	}
}

func TestComputeTargetsReportsMappingErrors(t *testing.T) {
	cfg := &config.Config{
		Placement: config.Placement{
			Apps: map[string]config.AppPlacement{
				"app": {Base: "/tmp/app", Paths: map[string]string{"models": "models"}},
			},
			Mapping: []config.MappingRule{
				{Match: "missing-app", Targets: []config.MappingTarget{{App: "other", PathKey: "models"}}},
				{Match: "missing-key", Targets: []config.MappingTarget{{App: "app", PathKey: "other"}}},
			},
		},
	}
	if _, err := ComputeTargets(nil, "llm"); err == nil {
		t.Fatal("expected nil config error")
	}
	if _, err := ComputeTargets(cfg, "missing-app"); err == nil || !strings.Contains(err.Error(), "unknown app") {
		t.Fatalf("expected unknown app error, got %v", err)
	}
	if _, err := ComputeTargets(cfg, "missing-key"); err == nil || !strings.Contains(err.Error(), "missing path key") {
		t.Fatalf("expected missing path key error, got %v", err)
	}
}
