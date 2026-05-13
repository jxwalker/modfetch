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

func TestDefaultQueryForTask(t *testing.T) {
	if got := DefaultQuery("coding"); !strings.Contains(got, "coder") {
		t.Fatalf("coding default query = %q", got)
	}
	if got := NormalizeTask("code"); got != "coding" {
		t.Fatalf("NormalizeTask(code) = %q, want coding", got)
	}
}
