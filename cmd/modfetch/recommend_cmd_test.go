package main

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/jxwalker/modfetch/internal/discovery"
	"github.com/jxwalker/modfetch/internal/recommend"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestValidateGiBOverride(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{name: "unset", value: 0},
		{name: "fractional", value: 0.5},
		{name: "max", value: maxHardwareOverrideGiB},
		{name: "negative", value: -1, wantErr: true},
		{name: "too large", value: maxHardwareOverrideGiB + 1, wantErr: true},
		{name: "nan", value: math.NaN(), wantErr: true},
		{name: "-inf", value: math.Inf(-1), wantErr: true},
		{name: "inf", value: math.Inf(1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGiBOverride("--ram-gb", tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateGiBOverride(%g) err = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestRecommendHistoryListsRows(t *testing.T) {
	cfgPath := writeBenchConfig(t)
	cfg, _, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	db, err := state.Open(cfg)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	if err := db.UpsertRecommendationHistory(state.RecommendationHistoryRow{
		Task:        "chat",
		Query:       "tiny gpt2",
		Provider:    "huggingface",
		ModelID:     "owner/model",
		URI:         "hf://owner/model/model.bin?rev=main",
		Action:      "selected",
		Score:       42,
		Fit:         "excellent",
		HardwareKey: "darwin/arm64/128g/unified",
	}); err != nil {
		t.Fatalf("upsert recommendation history: %v", err)
	}
	_ = db.Close()

	var runErr error
	out := captureStdout(t, func() {
		runErr = handleRecommend(context.Background(), []string{
			"--config", cfgPath,
			"--history",
			"--json",
		})
	})
	if runErr != nil {
		t.Fatalf("recommend history: %v", runErr)
	}
	var rows []state.RecommendationHistoryRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("decode history: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0].Action != "selected" || rows[0].URI != "hf://owner/model/model.bin?rev=main" {
		t.Fatalf("history rows = %+v", rows)
	}
}

func TestRecommendationHardwareKey(t *testing.T) {
	got := recommendationHardwareKey(recommend.HardwareProfile{
		OS:            "darwin",
		Arch:          "arm64",
		RAMBytes:      127<<30 + 1,
		UnifiedMemory: true,
	})
	if got != "darwin/arm64/128g/unified" {
		t.Fatalf("hardware key = %q", got)
	}
}

func TestRecommendationFeedbackFromHistory(t *testing.T) {
	db, err := state.NewDB(t.TempDir() + "/state.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	recs := []recommend.Recommendation{
		{Index: 1, Provider: discovery.ProviderHuggingFace, ModelID: "owner/selected", URI: "hf://owner/selected/model.gguf?rev=main", Score: 100, Fit: "excellent"},
		{Index: 2, Provider: discovery.ProviderHuggingFace, ModelID: "owner/skipped", URI: "hf://owner/skipped/model.gguf?rev=main", Score: 90, Fit: "excellent"},
	}
	if err := recordRecommendationHistory(db, "coding", "qwen coder gguf", "darwin/arm64/128g/unified", recs, "selected", 1); err != nil {
		t.Fatalf("record selected history: %v", err)
	}
	if err := recordRecommendationHistory(db, "coding", "qwen coder gguf", "darwin/arm64/128g/unified", recs, "skipped", 1); err != nil {
		t.Fatalf("record skipped history: %v", err)
	}

	feedback, err := recommendationFeedback(db, "coding", "qwen coder gguf", "darwin/arm64/128g/unified")
	if err != nil {
		t.Fatalf("load feedback: %v", err)
	}
	selected := feedback[recommend.FeedbackKey("hf://owner/selected/model.gguf?rev=main")]
	skipped := feedback[recommend.FeedbackKey("hf://owner/skipped/model.gguf?rev=main")]
	if selected.Selected != 1 {
		t.Fatalf("selected feedback = %#v, want one selection", selected)
	}
	if skipped.Skipped != 1 {
		t.Fatalf("skipped feedback = %#v, want one skip", skipped)
	}
}
