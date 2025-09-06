package placer

import (
	"os"
	"path/filepath"
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
