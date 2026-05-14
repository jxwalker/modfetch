package tui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/discovery"
	"github.com/jxwalker/modfetch/internal/downloader"
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

func TestRecommendFlowSizeFilterKeepsUnknownSizeResults(t *testing.T) {
	recs := []recommend.Recommendation{
		{
			Name: "unknown ollama",
			URI:  "hf://owner/unknown/model.gguf?rev=main",
			RuntimeHints: []recommend.RuntimeHint{
				{Runtime: "Ollama", PlacementPreset: "ollama"},
			},
		},
		{
			Name: "known too large",
			URI:  "hf://owner/large/model.gguf?rev=main",
			Size: 20 << 30,
			RuntimeHints: []recommend.RuntimeHint{
				{Runtime: "Ollama", PlacementPreset: "ollama"},
			},
		},
	}

	got := filterRecommendResults(recs, "ollama", 8<<30)
	if len(got) != 1 || got[0].Name != "unknown ollama" {
		t.Fatalf("filtered recommendations = %+v, want unknown-size result kept", got)
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

func TestRecommendFlowInspectionRendersRationalePlacementAndTransferPlan(t *testing.T) {
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
		Concurrency: config.Concurrency{PerFileChunks: 8, ChunkSizeMB: 64},
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
		inspect:      true,
		task:         "chat",
		query:        "llama instruct gguf",
		runtimeIndex: 2,
		results: []recommend.Recommendation{
			{
				Index:             1,
				Provider:          discovery.ProviderHuggingFace,
				ModelID:           "owner/model",
				Name:              "Owner Model",
				URI:               "https://example.com/model.gguf",
				FileName:          "model.gguf",
				FileType:          "gguf",
				Size:              7 << 30,
				EstimatedRequired: 9 << 30,
				MemoryBudget:      92 << 30,
				Score:             120,
				Fit:               "excellent",
				Reasons:           []string{"comfortable memory fit", "GGUF is ready for local llama.cpp-style runtimes"},
				RuntimeHints: []recommend.RuntimeHint{
					{Runtime: "Ollama", Reason: "GGUF can be imported with a Modelfile", PlacementPreset: "ollama", SetupCommand: "ollama create NAME -f Modelfile"},
				},
			},
		},
	}

	view := m.renderRecommendResults()
	for _, want := range []string{
		"Details",
		"rationale:",
		"setup: ollama create NAME -f Modelfile",
		"placement: configured placement target",
		"artifact: llm.gguf",
		"dry-run transfer:",
		"connections=8 chunk=64 MiB",
		"live metadata: press p",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("inspection view missing %q:\n%s", want, view)
		}
	}
}

func TestRecommendFlowProbeUsesRealHTTPMetadataAndTransferHistory(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := state.NewDB(filepath.Join(tmpDir, "state.db"))
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "4096")
		w.Header().Set("Content-Disposition", `attachment; filename="server-model.gguf"`)
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Last-Modified", "Thu, 14 May 2026 12:00:00 GMT")
		if r.Method == http.MethodHead {
			return
		}
		if r.Header.Get("Range") == "bytes=0-0" {
			w.Header().Set("Content-Range", "bytes 0-0/4096")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
			return
		}
		_, _ = w.Write(make([]byte, 4096))
	}))
	defer ts.Close()

	rawURL := ts.URL + "/model.gguf"
	if err := db.UpsertTransferHistory(state.TransferHistoryRow{
		Host:        downloader.HostFromURLForHistory(rawURL),
		Tool:        "modfetch",
		Connections: 12,
		ChunkSizeMB: 64,
		AvgBPS:      2048,
		LastStatus:  "complete",
	}); err != nil {
		t.Fatalf("upsert transfer history: %v", err)
	}

	cfg := &config.Config{
		General:     config.General{DataRoot: tmpDir, DownloadRoot: tmpDir},
		Concurrency: config.Concurrency{PerFileChunks: 4, ChunkSizeMB: 8},
	}
	m := New(cfg, db, "test").(*Model)
	m.recommendFlow = recommendFlow{
		active:  true,
		step:    recommendStepResults,
		flowID:  42,
		inspect: true,
		results: []recommend.Recommendation{
			{Index: 1, Provider: discovery.ProviderHuggingFace, Name: "probe me", URI: rawURL, FileName: "model.gguf", FileType: "gguf"},
		},
	}

	cmd := m.startRecommendProbe()
	if cmd == nil {
		t.Fatal("expected probe command")
	}
	msg, ok := cmd().(recommendInspectProbeMsg)
	if !ok {
		t.Fatalf("probe command returned %T", msg)
	}
	if msg.err != nil {
		t.Fatalf("probe command error: %v", msg.err)
	}
	m.Update(msg)

	if m.recommendFlow.probeLoading {
		t.Fatal("probe should not remain loading")
	}
	if m.recommendFlow.probeDetails.Size != 4096 || !m.recommendFlow.probeDetails.AcceptRange {
		t.Fatalf("probe details = %+v, want size and range metadata", m.recommendFlow.probeDetails)
	}
	if !m.recommendFlow.probeDetails.HasHistory {
		t.Fatalf("probe details missing transfer history: %+v", m.recommendFlow.probeDetails)
	}
	view := m.renderRecommendResults()
	for _, want := range []string{"remote size: 4.1 kB", "ranges: yes", "server filename: server-model.gguf", "prior host speed:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("probe view missing %q:\n%s", want, view)
		}
	}
}
