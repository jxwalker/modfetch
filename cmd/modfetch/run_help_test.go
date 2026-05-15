package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpHintsForGGUF(t *testing.T) {
	hints := runHelpHintsForPath("/tmp/Qwen Coder's Test.Q4_K_M.gguf")
	if len(hints) != 3 {
		t.Fatalf("hints len = %d, want 3", len(hints))
	}
	if hints[0].Runtime != "llama.cpp" || !strings.Contains(hints[0].Command, "llama-cli -m") {
		t.Fatalf("llama.cpp hint = %+v", hints[0])
	}
	if hints[1].Runtime != "Ollama" || !strings.Contains(hints[1].Command, "ollama create qwen-coder-s-test.q4_k_m") {
		t.Fatalf("ollama hint = %+v", hints[1])
	}
	if !strings.Contains(hints[1].Command, "-f -") {
		t.Fatalf("ollama hint should stream Modelfile on stdin: %s", hints[1].Command)
	}
	if !strings.Contains(hints[1].Command, "'\"'\"'") {
		t.Fatalf("shell quote did not escape apostrophe: %s", hints[1].Command)
	}
}

func TestRunHelpArtifactNameHandlesURIStyleDestinations(t *testing.T) {
	if got := runHelpArtifactName("s3://models/checkpoints/model.gguf"); got != "model.gguf" {
		t.Fatalf("s3 artifact name = %q, want model.gguf", got)
	}
	if got := runHelpArtifactName("https://example.com/models/model.safetensors?download=1"); got != "model.safetensors" {
		t.Fatalf("https artifact name = %q, want model.safetensors", got)
	}
}

func TestRunHelpHintsSuppressLocalCommandsForRemoteDestinations(t *testing.T) {
	hints := runHelpHintsForPath("s3://models/checkpoints/model.gguf")
	if len(hints) != 1 {
		t.Fatalf("remote hints len = %d, want 1", len(hints))
	}
	if hints[0].Command != "" {
		t.Fatalf("remote hint should not include local command: %+v", hints[0])
	}
	if !strings.Contains(hints[0].Note, "not directly runnable") {
		t.Fatalf("remote hint note = %q", hints[0].Note)
	}
}

func TestRunHelpHintsForSafetensorsPrioritizeFormatSignals(t *testing.T) {
	textHints := runHelpHintsForPath("/tmp/tiny-random-bert/model.safetensors")
	if len(textHints) == 0 || textHints[0].Runtime != "Transformers" {
		t.Fatalf("text safetensors first hint = %+v", textHints)
	}
	imageHints := runHelpHintsForPath("/tmp/sdxl-checkpoint.safetensors")
	if len(imageHints) == 0 || imageHints[0].Runtime != "ComfyUI" {
		t.Fatalf("image safetensors first hint = %+v", imageHints)
	}
}

func TestRunHelpHintsForUnknownExtensionReturnsEmptySlice(t *testing.T) {
	hints := runHelpHintsForPath("/tmp/model.unknown")
	if hints == nil {
		t.Fatal("unknown extension should return an empty slice, not nil")
	}
	if len(hints) != 0 {
		t.Fatalf("unknown extension hints = %+v, want empty", hints)
	}
}

func TestDownloadDryRunIncludesRunHelpJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "128")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeRunHelpTestConfig(t, cfgPath, tmp)

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleDownload(context.Background(), []string{
			"--config", cfgPath,
			"--url", ts.URL + "/model.gguf",
			"--dry-run",
			"--summary-json",
			"--run-help",
		})
	})
	if runErr != nil {
		t.Fatalf("download dry-run: %v", runErr)
	}
	var got struct {
		RunHints []runHelpHint `json:"run_hints"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode dry-run JSON: %v\n%s", err, out)
	}
	if len(got.RunHints) == 0 {
		t.Fatalf("run_hints empty in:\n%s", out)
	}
	if got.RunHints[0].Runtime != "llama.cpp" || !strings.Contains(got.RunHints[0].Command, "llama-cli") {
		t.Fatalf("first run hint = %+v", got.RunHints[0])
	}
}

func TestDownloadDryRunPrintsRunHelp(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "128")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yml")
	writeRunHelpTestConfig(t, cfgPath, tmp)

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleDownload(context.Background(), []string{
			"--config", cfgPath,
			"--url", ts.URL + "/image.safetensors",
			"--dry-run",
			"--run-help",
		})
	})
	if runErr != nil {
		t.Fatalf("download dry-run: %v", runErr)
	}
	for _, want := range []string{"Run it locally:", "ComfyUI", "modfetch place --path", "--preset comfyui"} {
		if !strings.Contains(out, want) {
			t.Fatalf("run help output missing %q in:\n%s", want, out)
		}
	}
}

func writeRunHelpTestConfig(t *testing.T, cfgPath, root string) {
	t.Helper()
	body := "version: 1\n" +
		"general:\n" +
		"  data_root: " + filepath.Join(root, "data") + "\n" +
		"  download_root: " + filepath.Join(root, "downloads") + "\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
