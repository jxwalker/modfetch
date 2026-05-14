package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/discovery"
	"github.com/jxwalker/modfetch/internal/recommend"
	"github.com/jxwalker/modfetch/internal/state"
)

func TestRecommendFlowFiltersByRuntimeAndSize(t *testing.T) {
	recs := []recommend.Recommendation{
		{
			Name:  "small ollama",
			URI:   "hf://owner/small/model.gguf?rev=main",
			Size:  5 << 30,
			Index: 1,
			RuntimeHints: []recommend.RuntimeHint{
				{Runtime: "Ollama", PlacementPreset: "ollama"},
			},
		},
		{
			Name:  "large ollama",
			URI:   "hf://owner/large/model.gguf?rev=main",
			Size:  20 << 30,
			Index: 2,
			RuntimeHints: []recommend.RuntimeHint{
				{Runtime: "Ollama", PlacementPreset: "ollama"},
			},
		},
		{
			Name:  "image checkpoint",
			URI:   "hf://owner/image/model.safetensors?rev=main",
			Size:  5 << 30,
			Index: 3,
			RuntimeHints: []recommend.RuntimeHint{
				{Runtime: "ComfyUI", PlacementPreset: "comfyui"},
			},
		},
	}

	got := filterRecommendResults(recs, "ollama", 8<<30)
	if len(got) != 1 || got[0].Name != "small ollama" {
		t.Fatalf("filtered recommendations = %+v, want only small ollama", got)
	}
}

func TestRecommendFlowStartsSelectedRecommendationDownload(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := state.NewDB(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer func() { _ = db.Close() }()

	cfg := &config.Config{
		General: config.General{
			DataRoot:      tmpDir,
			DownloadRoot:  tmpDir,
			PlacementMode: "symlink",
		},
	}
	m := New(cfg, db, "test").(*Model)
	m.recommendFlow = recommendFlow{
		active:      true,
		step:        recommendStepResults,
		task:        "chat",
		query:       "llama instruct gguf",
		hardwareKey: "darwin/arm64/ram128g/unified",
		hardware:    recommend.HardwareProfile{OS: "darwin", Arch: "arm64", RAMBytes: 128 << 30, UnifiedMemory: true},
		results: []recommend.Recommendation{
			{
				Index:    1,
				Provider: discovery.ProviderHuggingFace,
				ModelID:  "owner/model",
				Name:     "Owner Model",
				URI:      "https://example.com/model.gguf",
				FileName: "model.gguf",
				FileType: "gguf",
				Score:    120,
				Fit:      "excellent",
			},
		},
	}

	_, cmd := m.updateRecommendFlow(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected selected recommendation to return a download command")
	}
	if m.recommendFlow.active {
		t.Fatal("recommend flow should close after starting a download")
	}

	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("download rows = %+v, want one pending row", rows)
	}
	if rows[0].URL != "https://example.com/model.gguf" || rows[0].Status != "pending" {
		t.Fatalf("download row = %+v, want pending recommendation URL", rows[0])
	}
	if _, ok := m.running["https://example.com/model.gguf|"+rows[0].Dest]; !ok {
		t.Fatalf("expected running cancellation handle for %q", rows[0].Dest)
	}

	history, err := db.RecommendationHistoryFor("chat", "llama instruct gguf", "darwin/arm64/ram128g/unified")
	if err != nil {
		t.Fatalf("recommendation history: %v", err)
	}
	if len(history) != 1 || history[0].Action != "selected" {
		t.Fatalf("history = %+v, want selected row", history)
	}
}

func TestRecommendFlowUsesConfiguredPlacementTargetAsDestination(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := state.NewDB(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer func() { _ = db.Close() }()

	cfg := &config.Config{
		General: config.General{
			DataRoot:      tmpDir,
			DownloadRoot:  filepath.Join(tmpDir, "downloads"),
			PlacementMode: "symlink",
		},
		Placement: config.Placement{
			Apps: map[string]config.AppPlacement{
				"ollama": {
					Base: filepath.Join(tmpDir, "ollama"),
					Paths: map[string]string{
						"models": "models",
					},
				},
			},
			Mapping: []config.MappingRule{
				{Match: "llm.gguf", Targets: []config.MappingTarget{{App: "ollama", PathKey: "models"}}},
			},
		},
	}
	m := New(cfg, db, "test").(*Model)
	m.recommendFlow = recommendFlow{
		active:       true,
		step:         recommendStepResults,
		task:         "chat",
		query:        "llama instruct gguf",
		hardwareKey:  "darwin/arm64/ram128g/unified",
		hardware:     recommend.HardwareProfile{OS: "darwin", Arch: "arm64", RAMBytes: 128 << 30, UnifiedMemory: true},
		runtimeIndex: 2, // Ollama
		results: []recommend.Recommendation{
			{
				Index:    1,
				Provider: discovery.ProviderHuggingFace,
				ModelID:  "owner/model",
				Name:     "Owner Model",
				URI:      "https://example.com/model.gguf",
				FileName: "model.gguf",
				FileType: "gguf",
				Score:    120,
				Fit:      "excellent",
				RuntimeHints: []recommend.RuntimeHint{
					{Runtime: "Ollama", PlacementPreset: "ollama"},
				},
			},
		},
	}

	_, cmd := m.updateRecommendFlow(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected selected recommendation to return a download command")
	}
	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	wantDest := filepath.Join(tmpDir, "ollama", "models", "model.gguf")
	if len(rows) != 1 || rows[0].Dest != wantDest {
		t.Fatalf("download rows = %+v, want dest %q", rows, wantDest)
	}
	if m.autoPlace["https://example.com/model.gguf|"+wantDest] {
		t.Fatal("recommend placement target should use the target destination without a second place pass")
	}
}

func TestRecommendFlowQueryStepKeepsTextEditingKeys(t *testing.T) {
	input := textinput.New()
	input.SetValue("llama")
	m := &Model{recommendFlow: recommendFlow{active: true, step: recommendStepQuery, input: input}}
	m.recommendFlow.input.Focus()

	m.updateRecommendFlow(tea.KeyMsg{Type: tea.KeyLeft})
	if m.recommendFlow.step != recommendStepQuery {
		t.Fatalf("left should stay in query input, got step %v", m.recommendFlow.step)
	}

	m.updateRecommendFlow(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.recommendFlow.step != recommendStepSize {
		t.Fatalf("shift+tab should navigate back to size step, got %v", m.recommendFlow.step)
	}
}

func TestRecommendFlowEscCancelsInFlightSearch(t *testing.T) {
	cancelled := false
	m := &Model{recommendFlow: recommendFlow{
		active:  true,
		loading: true,
		cancel:  func() { cancelled = true },
	}}

	m.updateRecommendFlow(tea.KeyMsg{Type: tea.KeyEsc})

	if !cancelled {
		t.Fatal("expected esc to cancel in-flight recommendation search")
	}
	if m.recommendFlow.active {
		t.Fatal("expected esc to close recommendation flow")
	}
}

func TestRecommendResultsIgnoreStaleFlow(t *testing.T) {
	m := &Model{recommendFlow: recommendFlow{
		active:  true,
		loading: true,
		flowID:  2,
	}}

	m.Update(recommendResultsMsg{
		flowID: 1,
		recommendations: []recommend.Recommendation{{
			Index: 1,
			Name:  "stale",
			URI:   "https://example.com/stale.gguf",
		}},
	})

	if !m.recommendFlow.loading {
		t.Fatal("stale result should not mutate the active flow")
	}
	if len(m.recommendFlow.results) != 0 {
		t.Fatalf("stale result populated results: %+v", m.recommendFlow.results)
	}
}

func TestRecommendFlowRenderTaskStep(t *testing.T) {
	m := &Model{th: defaultTheme()}
	m.startRecommendFlow()

	view := m.renderRecommendFlow()
	if !strings.Contains(view, "Recommend Models") || !strings.Contains(view, "Chat / assistant") {
		t.Fatalf("recommend flow view missing expected labels:\n%s", view)
	}
}
