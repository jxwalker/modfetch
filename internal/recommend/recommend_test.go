package recommend

import (
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/discovery"
)

func TestRankPrefersHardwareFitAndTask(t *testing.T) {
	hw := HardwareProfile{RAMBytes: 32 << 30, UnifiedMemory: true}
	results := []discovery.Result{
		{
			Provider:  discovery.ProviderHuggingFace,
			ModelID:   "acme/huge-70b",
			Name:      "Huge 70B Instruct",
			FileName:  "huge-70b.Q4_K_M.gguf",
			FileType:  "gguf",
			Size:      42 << 30,
			Downloads: 900000,
			URI:       "hf://acme/huge-70b/huge-70b.Q4_K_M.gguf?rev=main",
		},
		{
			Provider:  discovery.ProviderHuggingFace,
			ModelID:   "acme/qwen-coder-7b",
			Name:      "Qwen Coder 7B Instruct",
			FileName:  "qwen-coder-7b.Q4_K_M.gguf",
			FileType:  "gguf",
			Size:      5 << 30,
			Downloads: 100000,
			URI:       "hf://acme/qwen-coder-7b/qwen-coder-7b.Q4_K_M.gguf?rev=main",
		},
	}

	ranked := Rank(results, hw, "coding")
	if len(ranked) != 2 {
		t.Fatalf("ranked len = %d, want 2", len(ranked))
	}
	if ranked[0].ModelID != "acme/qwen-coder-7b" {
		t.Fatalf("top recommendation = %s, want coder model", ranked[0].ModelID)
	}
	if ranked[0].Fit != "excellent" {
		t.Fatalf("fit = %s, want excellent", ranked[0].Fit)
	}
	if ranked[1].Fit != "too_large" {
		t.Fatalf("huge fit = %s, want too_large", ranked[1].Fit)
	}
}

func TestRankInfersQuantizationAndParams(t *testing.T) {
	ranked := Rank([]discovery.Result{{
		Provider: discovery.ProviderHuggingFace,
		ModelID:  "acme/tiny",
		Name:     "Tiny 3B",
		FilePath: "models/tiny-3b.Q5_K_M.gguf",
		FileType: "gguf",
		URI:      "hf://acme/tiny/models/tiny-3b.Q5_K_M.gguf?rev=main",
	}}, HardwareProfile{RAMBytes: 16 << 30, UnifiedMemory: true}, "chat")

	if len(ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1", len(ranked))
	}
	if ranked[0].ParameterCount != "3B" {
		t.Fatalf("params = %q, want 3B", ranked[0].ParameterCount)
	}
	if ranked[0].Quantization != "Q5_K_M" {
		t.Fatalf("quant = %q, want Q5_K_M", ranked[0].Quantization)
	}
	if !strings.Contains(ranked[0].DownloadCommand, "modfetch download --url") {
		t.Fatalf("missing download command: %q", ranked[0].DownloadCommand)
	}
	if len(ranked[0].RuntimeHints) == 0 || ranked[0].RuntimeHints[0].Runtime != "llama.cpp" {
		t.Fatalf("runtime hints = %#v, want llama.cpp first", ranked[0].RuntimeHints)
	}
}

func TestRankDemotesSplitShards(t *testing.T) {
	ranked := Rank([]discovery.Result{
		{
			Provider:  discovery.ProviderHuggingFace,
			ModelID:   "acme/coder-32b",
			Name:      "Coder 32B",
			FilePath:  "coder-32b-q4_k_m-00003-of-00003.gguf",
			FileType:  "gguf",
			Size:      4 << 30,
			Downloads: 900000,
			URI:       "hf://acme/coder-32b/coder-32b-q4_k_m-00003-of-00003.gguf?rev=main",
		},
		{
			Provider:  discovery.ProviderHuggingFace,
			ModelID:   "acme/coder-14b",
			Name:      "Coder 14B",
			FilePath:  "coder-14b-q4_k_m.gguf",
			FileType:  "gguf",
			Size:      9 << 30,
			Downloads: 100000,
			URI:       "hf://acme/coder-14b/coder-14b-q4_k_m.gguf?rev=main",
		},
	}, HardwareProfile{RAMBytes: 64 << 30, UnifiedMemory: true}, "coding")

	if len(ranked) != 2 {
		t.Fatalf("ranked len = %d, want 2", len(ranked))
	}
	if ranked[0].ModelID != "acme/coder-14b" {
		t.Fatalf("top recommendation = %s, want complete single artifact", ranked[0].ModelID)
	}
	if !strings.Contains(strings.Join(ranked[1].Reasons, " "), "multi-part shard") {
		t.Fatalf("missing shard reason: %#v", ranked[1].Reasons)
	}
}

func TestApplyFeedbackBoostsPriorSelections(t *testing.T) {
	ranked := Rank([]discovery.Result{
		{
			Provider:  discovery.ProviderHuggingFace,
			ModelID:   "acme/first",
			Name:      "First 7B Instruct",
			FileName:  "first-7b.Q4_K_M.gguf",
			FileType:  "gguf",
			Size:      5 << 30,
			Downloads: 500000,
			URI:       "hf://acme/first/first-7b.Q4_K_M.gguf?rev=main",
		},
		{
			Provider:  discovery.ProviderHuggingFace,
			ModelID:   "acme/second",
			Name:      "Second 7B Instruct",
			FileName:  "second-7b.Q4_K_M.gguf",
			FileType:  "gguf",
			Size:      5 << 30,
			Downloads: 1000,
			URI:       "hf://acme/second/second-7b.Q4_K_M.gguf?rev=main",
		},
	}, HardwareProfile{RAMBytes: 32 << 30, UnifiedMemory: true}, "chat")

	if ranked[0].ModelID != "acme/first" {
		t.Fatalf("initial top = %s, want first", ranked[0].ModelID)
	}
	ApplyFeedback(ranked, map[string]Feedback{
		FeedbackKey("hf://acme/first/first-7b.Q4_K_M.gguf?rev=main"):   {Skipped: 4},
		FeedbackKey("hf://acme/second/second-7b.Q4_K_M.gguf?rev=main"): {Selected: 3},
	})
	if ranked[0].ModelID != "acme/second" {
		t.Fatalf("feedback top = %s, want second", ranked[0].ModelID)
	}
	if !strings.Contains(strings.Join(ranked[0].Reasons, " "), "prior selection") {
		t.Fatalf("missing feedback reason: %#v", ranked[0].Reasons)
	}
}

func TestRuntimeHintsForImageSafetensors(t *testing.T) {
	ranked := Rank([]discovery.Result{{
		Provider: discovery.ProviderHuggingFace,
		ModelID:  "acme/sdxl",
		Name:     "SDXL Checkpoint",
		FileName: "sdxl.safetensors",
		FileType: "safetensors",
		Tags:     []string{"stable-diffusion", "sdxl"},
		Size:     6 << 30,
		URI:      "hf://acme/sdxl/sdxl.safetensors?rev=main",
	}}, HardwareProfile{VRAMBytes: 24 << 30}, "image")

	if len(ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1", len(ranked))
	}
	if len(ranked[0].RuntimeHints) == 0 || ranked[0].RuntimeHints[0].Runtime != "ComfyUI" {
		t.Fatalf("runtime hints = %#v, want ComfyUI first", ranked[0].RuntimeHints)
	}
}

func TestRuntimeHintsForSafetensorsAreHardwareAware(t *testing.T) {
	result := discovery.Result{
		Provider: discovery.ProviderHuggingFace,
		ModelID:  "acme/llm",
		Name:     "LLM",
		FileName: "model.safetensors",
		FileType: "safetensors",
		URI:      "hf://acme/llm/model.safetensors?rev=main",
	}

	linux := Rank([]discovery.Result{result}, HardwareProfile{OS: "linux", Arch: "amd64", VRAMBytes: 24 << 30}, "chat")
	for _, hint := range linux[0].RuntimeHints {
		if hint.Runtime == "MLX" {
			t.Fatalf("linux runtime hints include MLX: %#v", linux[0].RuntimeHints)
		}
	}

	darwin := Rank([]discovery.Result{result}, HardwareProfile{OS: "darwin", Arch: "arm64", RAMBytes: 64 << 30, UnifiedMemory: true}, "chat")
	if !hasRuntimeHint(darwin[0].RuntimeHints, "MLX") {
		t.Fatalf("darwin runtime hints = %#v, want MLX", darwin[0].RuntimeHints)
	}
}

func TestDefaultQueryForTask(t *testing.T) {
	if got := DefaultQuery("coding"); !strings.Contains(got, "coder") {
		t.Fatalf("coding default query = %q", got)
	}
	if got := NormalizeTask("code"); got != "coding" {
		t.Fatalf("NormalizeTask(code) = %q, want coding", got)
	}
}

func hasRuntimeHint(hints []RuntimeHint, runtime string) bool {
	for _, hint := range hints {
		if hint.Runtime == runtime {
			return true
		}
	}
	return false
}
