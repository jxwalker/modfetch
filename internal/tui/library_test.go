package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/catalog"
	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/scanner"
	"github.com/jxwalker/modfetch/internal/state"
)

// setupTestLibrary creates a test model with library data
func setupTestLibrary(t *testing.T) (*Model, *state.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cfg := &config.Config{
		General: config.General{
			DownloadRoot: tmpDir,
		},
		Placement: config.Placement{
			Apps:    make(map[string]config.AppPlacement),
			Mapping: []config.MappingRule{},
		},
	}

	model := New(cfg, db, "test-version").(*Model)
	model.w = 120
	model.h = 40
	model.activeTab = 5 // Library tab

	cleanup := func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	}

	return model, db, cleanup
}

// createTestMetadata adds test metadata to the database
func createTestMetadata(t *testing.T, db *state.DB, count int) []state.ModelMetadata {
	t.Helper()

	var models []state.ModelMetadata

	for i := 0; i < count; i++ {
		meta := &state.ModelMetadata{
			DownloadURL:    "https://example.com/model" + string(rune(i)),
			Dest:           "/path/to/model" + string(rune(i)),
			ModelName:      "TestModel" + string(rune(i)),
			ModelID:        "test/model" + string(rune(i)),
			Source:         "huggingface",
			ModelType:      "LLM",
			Quantization:   "Q4_K_M",
			ParameterCount: "7B",
			FileSize:       int64(4 * 1024 * 1024 * 1024), // 4GB
			FileFormat:     ".gguf",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := db.UpsertMetadata(meta); err != nil {
			t.Fatalf("Failed to insert test metadata: %v", err)
		}

		models = append(models, *meta)
	}

	return models
}

func TestLibrary_RefreshLibraryData_Empty(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	model.refreshLibraryData()

	if len(model.libraryRows) != 0 {
		t.Errorf("Expected 0 library rows, got %d", len(model.libraryRows))
	}

	if model.libraryNeedsRefresh {
		t.Error("libraryNeedsRefresh should be false after refresh")
	}
}

func TestLibrary_RefreshLibraryData_WithModels(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Add test models
	createTestMetadata(t, db, 5)

	model.refreshLibraryData()

	if len(model.libraryRows) != 5 {
		t.Errorf("Expected 5 library rows, got %d", len(model.libraryRows))
	}

	if model.libraryNeedsRefresh {
		t.Error("libraryNeedsRefresh should be false after refresh")
	}
}

func TestLibrary_RefreshLibraryData_WithFilters(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Add models with different types
	meta1 := &state.ModelMetadata{
		DownloadURL: "https://huggingface.co/llmmodel.gguf",
		ModelName:   "LLMModel",
		ModelType:   "LLM",
		Source:      "huggingface",
		Dest:        "/test1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	meta2 := &state.ModelMetadata{
		DownloadURL: "https://civitai.com/loramodel.safetensors",
		ModelName:   "LoRAModel",
		ModelType:   "LoRA",
		Source:      "civitai",
		Dest:        "/test2",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.UpsertMetadata(meta1); err != nil {
		t.Fatalf("Failed to insert meta1: %v", err)
	}
	if err := db.UpsertMetadata(meta2); err != nil {
		t.Fatalf("Failed to insert meta2: %v", err)
	}

	// Test type filter
	model.libraryFilterType = "LLM"
	model.refreshLibraryData()

	if len(model.libraryRows) != 1 {
		t.Errorf("Expected 1 LLM model, got %d", len(model.libraryRows))
	}
	if model.libraryRows[0].ModelType != "LLM" {
		t.Errorf("Expected LLM type, got %s", model.libraryRows[0].ModelType)
	}

	// Test source filter
	model.libraryFilterType = ""
	model.libraryFilterSource = "civitai"
	model.refreshLibraryData()

	if len(model.libraryRows) != 1 {
		t.Errorf("Expected 1 civitai model, got %d", len(model.libraryRows))
	}
	if model.libraryRows[0].Source != "civitai" {
		t.Errorf("Expected civitai source, got %s", model.libraryRows[0].Source)
	}
}

func TestLibrary_RefreshLibraryData_WithSearch(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Add models with distinct names
	meta1 := &state.ModelMetadata{
		DownloadURL: "https://huggingface.co/llama-2-7b.gguf",
		ModelName:   "llama-2-7b",
		ModelType:   "LLM",
		Dest:        "/test1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	meta2 := &state.ModelMetadata{
		DownloadURL: "https://huggingface.co/mistral-7b.gguf",
		ModelName:   "mistral-7b",
		ModelType:   "LLM",
		Dest:        "/test2",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.UpsertMetadata(meta1); err != nil {
		t.Fatalf("Failed to insert meta1: %v", err)
	}
	if err := db.UpsertMetadata(meta2); err != nil {
		t.Fatalf("Failed to insert meta2: %v", err)
	}

	// Test search
	model.librarySearch = "llama"
	model.refreshLibraryData()

	if len(model.libraryRows) != 1 {
		t.Errorf("Expected 1 result for 'llama' search, got %d", len(model.libraryRows))
	}
	if !strings.Contains(strings.ToLower(model.libraryRows[0].ModelName), "llama") {
		t.Errorf("Search result should contain 'llama', got %s", model.libraryRows[0].ModelName)
	}
}

func TestLibrary_RenderLibrary_Empty(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	model.refreshLibraryData()
	output := model.renderLibrary()

	if !strings.Contains(output, "No models found") {
		t.Error("Empty library should show 'No models found' message")
	}

	if !strings.Contains(output, "Download some models") {
		t.Error("Empty library should show hint about downloading models")
	}
}

func TestLibrary_RenderLibrary_WithModels(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	createTestMetadata(t, db, 3)
	model.refreshLibraryData()

	output := model.renderLibrary()

	// Should contain header
	if !strings.Contains(output, "Model Library") {
		t.Error("Library view should contain header")
	}

	// Should show models
	for _, meta := range model.libraryRows {
		if !strings.Contains(output, meta.ModelName) {
			t.Errorf("Library view should contain model name %s", meta.ModelName)
		}
	}

	// Should show footer
	if !strings.Contains(output, "navigate") {
		t.Error("Library view should show navigation help")
	}
}

func TestLibrary_RenderLibrary_Pagination(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Create more models than can fit on screen
	createTestMetadata(t, db, 50)
	model.h = 20 // Small height to force pagination
	model.refreshLibraryData()

	output := model.renderLibrary()

	// Should show pagination info
	if !strings.Contains(output, "Showing") {
		t.Error("Paginated library should show 'Showing x-y of z' info")
	}

	if !strings.Contains(output, "50 models") {
		t.Error("Should show total count of 50 models")
	}
}

func TestLibrary_RenderLibrary_Selection(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	createTestMetadata(t, db, 3)
	model.refreshLibraryData()

	// Test selection at position 0
	model.librarySelected = 0
	output := model.renderLibrary()

	if !strings.Contains(output, "▶") {
		t.Error("Selected row should show cursor")
	}

	// Test selection at position 1
	model.librarySelected = 1
	output = model.renderLibrary()

	// Count cursors (should be exactly 1)
	cursorCount := strings.Count(output, "▶")
	if cursorCount != 1 {
		t.Errorf("Expected exactly 1 cursor, found %d", cursorCount)
	}
}

func TestLibrary_RenderLibrary_FilterIndicators(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	createTestMetadata(t, db, 2)
	model.refreshLibraryData()

	// Test type filter indicator
	model.libraryFilterType = "LLM"
	output := model.renderLibrary()
	if !strings.Contains(output, "Type: LLM") {
		t.Error("Should show type filter indicator")
	}

	// Test source filter indicator
	model.libraryFilterType = ""
	model.libraryFilterSource = "huggingface"
	output = model.renderLibrary()
	if !strings.Contains(output, "Source: huggingface") {
		t.Error("Should show source filter indicator")
	}

	// Test search indicator
	model.libraryFilterSource = ""
	model.librarySearch = "test"
	output = model.renderLibrary()
	if !strings.Contains(output, "Search: \"test\"") {
		t.Error("Should show search indicator")
	}

	// Test favorites indicator
	model.librarySearch = ""
	model.libraryShowFavorites = true
	output = model.renderLibrary()
	if !strings.Contains(output, "Favorites") {
		t.Error("Should show favorites indicator")
	}
}

func TestLibrary_RenderLibraryDetail(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Create detailed model metadata
	meta := &state.ModelMetadata{
		DownloadURL:    "https://huggingface.co/meta/llama-2-7b.gguf",
		ModelName:      "llama-2-7b",
		ModelID:        "meta/llama-2-7b",
		Version:        "v1.0",
		Source:         "huggingface",
		Author:         "Meta",
		License:        "MIT",
		ModelType:      "LLM",
		Architecture:   "Llama",
		ParameterCount: "7B",
		Quantization:   "Q4_K_M",
		FileSize:       int64(4 * 1024 * 1024 * 1024),
		FileFormat:     ".gguf",
		Description:    "A large language model for text generation",
		Tags:           []string{"llm", "text-generation", "meta"},
		DownloadCount:  1000,
		TimesUsed:      5,
		LastUsed:       ptrTime(time.Now()),
		UserRating:     4,
		Favorite:       true,
		UserNotes:      "Great model for general tasks",
		HomepageURL:    "https://example.com",
		RepoURL:        "https://github.com/example/llama",
		Dest:           "/path/to/model",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := db.UpsertMetadata(meta); err != nil {
		t.Fatalf("Failed to insert metadata: %v", err)
	}

	model.libraryDetailModel = meta
	model.libraryViewingDetail = true

	output := model.renderLibraryDetail()

	// Check all sections are present
	sections := []string{
		"llama-2-7b",       // Title
		"Type:",            // Basic info
		"Version:",         // Version
		"Source:",          // Source
		"Author:",          // Author
		"License:",         // License
		"Specifications",   // Specs section
		"Architecture:",    // Arch
		"Parameters:",      // Params
		"Quantization:",    // Quant
		"File Information", // File section
		"Size:",            // Size
		"Format:",          // Format
		"Location:",        // Location
		"Description",      // Desc section
		"Tags",             // Tags section
		"Usage Statistics", // Usage section
		"Downloads:",       // Download count
		"Times Used:",      // Usage count
		"User Data",        // User section
		"Rating:",          // Rating
		"Favorite",         // Favorite indicator
		"Notes:",           // User notes
		"Links",            // Links section
		"Homepage:",        // Homepage
		"Repository:",      // Repo
		"Esc to go back",   // Footer
	}

	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("Detail view should contain '%s'", section)
		}
	}
}

func TestLibrary_RenderLibraryDetail_Minimal(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Create minimal metadata (only required fields)
	meta := &state.ModelMetadata{
		ModelName: "minimal-model",
		Dest:      "/path",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	model.libraryDetailModel = meta
	model.libraryViewingDetail = true

	output := model.renderLibraryDetail()

	// Should still render without errors
	if !strings.Contains(output, "minimal-model") {
		t.Error("Detail view should show model name")
	}

	// Optional fields should not cause errors
	if strings.Contains(output, "<nil>") {
		t.Error("Detail view should not show nil values")
	}
}

func TestLibrary_UpdateLibrarySearch_Activation(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Activate search
	model.librarySearchActive = true
	model.librarySearchInput.Focus()

	// Type some text
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("llama")}
	_, _ = model.updateLibrarySearch(msg)

	if model.librarySearchInput.Value() == "" {
		t.Error("Search input should contain typed text")
	}
}

func TestLibrary_UpdateLibrarySearch_Submit(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	createTestMetadata(t, db, 3)

	// Activate search and type
	model.librarySearchActive = true
	model.librarySearchInput.Focus()
	model.librarySearchInput.SetValue("test")

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, _ = model.updateLibrarySearch(msg)

	if model.librarySearchActive {
		t.Error("Search should be deactivated after Enter")
	}

	if model.librarySearch != "test" {
		t.Errorf("Search term should be 'test', got %q", model.librarySearch)
	}

	// Note: libraryNeedsRefresh is set to true but then cleared by refreshLibraryData()
	// which is called within updateLibrarySearch, so we can't test it this way
	// Instead, verify that the search term was set correctly
}

func TestLibrary_UpdateLibrarySearch_Cancel(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Activate search and type
	model.librarySearchActive = true
	model.librarySearchInput.Focus()
	model.librarySearchInput.SetValue("test")

	// Press Esc
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, _ = model.updateLibrarySearch(msg)

	if model.librarySearchActive {
		t.Error("Search should be deactivated after Esc")
	}

	if model.librarySearch != "" {
		t.Errorf("Search term should be cleared, got %q", model.librarySearch)
	}

	// Note: libraryNeedsRefresh is set to true but then cleared by refreshLibraryData()
	// which is called within updateLibrarySearch, so we can't test it this way
	// Instead, verify that the search term was cleared correctly
}

func TestLibrary_ScanDirectoriesCmd_NoConfig(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Set config to nil
	model.cfg = nil

	cmd := model.scanDirectoriesCmd()
	msg := cmd()

	scanMsg, ok := msg.(scanCompleteMsg)
	if !ok {
		t.Fatal("Expected scanCompleteMsg")
	}

	if scanMsg.err == nil {
		t.Error("Should return error when config is not available")
	}
}

func TestLibrary_ScanDirectoriesCmd_NoDirectories(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Clear directories
	model.cfg.General.DownloadRoot = ""
	model.cfg.Placement.Apps = make(map[string]config.AppPlacement)
	model.cfg.Placement.Mapping = []config.MappingRule{}

	cmd := model.scanDirectoriesCmd()
	msg := cmd()

	scanMsg, ok := msg.(scanCompleteMsg)
	if !ok {
		t.Fatal("Expected scanCompleteMsg")
	}

	if scanMsg.err == nil {
		t.Error("Should return error when no directories configured")
	}
}

func TestLibrary_ScanDirectoriesStreamReportsProgress(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	modelPath := filepath.Join(model.cfg.General.DownloadRoot, "stream-model.gguf")
	if err := os.WriteFile(modelPath, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}

	ch := make(chan tea.Msg, 16)
	done := make(chan tea.Msg, 1)
	go func() {
		done <- model.scanDirectoriesStreamCmd(context.Background(), ch)()
	}()

	sawProgress := false
	var complete scanCompleteMsg
	for msg := range ch {
		switch msg := msg.(type) {
		case scanProgressMsg:
			if msg.progress.FilesScanned > 0 {
				sawProgress = true
			}
		case scanCompleteMsg:
			complete = msg
		}
	}
	if msg := <-done; msg != nil {
		t.Fatalf("stream command should return nil, got %T", msg)
	}
	if !sawProgress {
		t.Fatal("expected scan progress message")
	}
	if complete.err != nil {
		t.Fatalf("scan failed: %v", complete.err)
	}
	if complete.result == nil || complete.result.FilesScanned != 1 || complete.result.ModelsAdded != 1 {
		t.Fatalf("unexpected scan result: %+v", complete.result)
	}
}

func TestLibrary_ScanDirectoriesStreamStopsAfterCancellation(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := make(chan tea.Msg)
	done := make(chan tea.Msg, 1)
	go func() {
		done <- model.scanDirectoriesStreamCmd(ctx, ch)()
	}()

	streamClosed := false
	for !streamClosed {
		select {
		case _, ok := <-ch:
			streamClosed = !ok
		case <-time.After(time.Second):
			t.Fatal("scan stream did not close after cancellation")
		}
	}

	select {
	case msg := <-done:
		if msg != nil {
			t.Fatalf("stream command should return nil, got %T", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("scan stream command blocked after cancellation")
	}
}

func TestLibrary_ScanCompleteCancelsContext(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	cancelled := false
	model.libraryScanning = true
	model.libraryScanCancel = func() {
		cancelled = true
	}

	updated, cmd := model.Update(scanCompleteMsg{result: &scanner.ScanResult{}})
	if cmd != nil {
		t.Fatalf("scan completion should not return a command, got %T", cmd)
	}
	updatedModel := updated.(*Model)
	if !cancelled {
		t.Fatal("expected scan completion to call cancel")
	}
	if updatedModel.libraryScanCancel != nil {
		t.Fatal("expected scan cancel function to be cleared")
	}
	if updatedModel.libraryScanning {
		t.Fatal("expected scan state to be cleared")
	}
}

func TestLibrary_SelectionBounds(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	createTestMetadata(t, db, 5)
	model.refreshLibraryData()

	// Test selection out of bounds - should reset to 0
	model.librarySelected = 10
	model.refreshLibraryData()

	if model.librarySelected != 0 {
		t.Errorf("Selection out of bounds should reset to 0, got %d", model.librarySelected)
	}

	// Test selection at valid position
	model.librarySelected = 2
	model.refreshLibraryData()

	if model.librarySelected != 2 {
		t.Errorf("Valid selection should be preserved, got %d", model.librarySelected)
	}
}

func TestLibrary_FavoriteDisplay(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Create model with favorite
	meta := &state.ModelMetadata{
		DownloadURL: "https://example.com/favorite-model.gguf",
		ModelName:   "favorite-model",
		Favorite:    true,
		Dest:        "/test",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.UpsertMetadata(meta); err != nil {
		t.Fatalf("Failed to insert metadata: %v", err)
	}

	model.refreshLibraryData()
	output := model.renderLibrary()

	// Should show favorite star
	if !strings.Contains(output, "★") {
		t.Error("Favorite model should show star indicator")
	}
}

func TestLibrary_SourceColorCoding(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Create models from different sources
	sources := []string{"huggingface", "civitai", "local"}
	for _, source := range sources {
		meta := &state.ModelMetadata{
			DownloadURL: "https://example.com/" + source + "/model.gguf",
			ModelName:   "model-" + source,
			Source:      source,
			Dest:        "/test/" + source,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := db.UpsertMetadata(meta); err != nil {
			t.Fatalf("Failed to insert metadata: %v", err)
		}
	}

	model.refreshLibraryData()
	output := model.renderLibrary()

	// Should show all sources
	for _, source := range sources {
		if !strings.Contains(output, source) {
			t.Errorf("Library should show source: %s", source)
		}
	}
}

func TestLibrary_LongModelNameTruncation(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Create model with very long name
	longName := strings.Repeat("a", 100)
	meta := &state.ModelMetadata{
		DownloadURL: "https://example.com/longname.gguf",
		ModelName:   longName,
		Dest:        "/test",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.UpsertMetadata(meta); err != nil {
		t.Fatalf("Failed to insert metadata: %v", err)
	}

	model.refreshLibraryData()
	output := model.renderLibrary()

	// Name should be truncated with ellipsis
	if strings.Contains(output, longName) {
		t.Error("Long model name should be truncated")
	}

	if !strings.Contains(output, "...") {
		t.Error("Truncated name should show ellipsis")
	}
}

func TestLibrary_ThemeApplication(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	// Render should not panic with theme applied
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Rendering with theme should not panic: %v", r)
		}
	}()

	_ = model.renderLibrary()
}

func TestLibrary_FilterMenuCyclesFiltersAndSearch(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	models := []state.ModelMetadata{
		{
			DownloadURL: "https://example.com/llm.gguf",
			ModelName:   "llama",
			ModelType:   "LLM",
			Source:      "huggingface",
			Dest:        "/models/llm.gguf",
		},
		{
			DownloadURL: "https://example.com/lora.safetensors",
			ModelName:   "portrait",
			ModelType:   "LoRA",
			Source:      "civitai",
			Dest:        "/models/lora.safetensors",
		},
	}
	for i := range models {
		if err := db.UpsertMetadata(&models[i]); err != nil {
			t.Fatalf("seed metadata: %v", err)
		}
	}
	model.refreshLibraryData()

	model.libraryFilterMenu = true
	model.libraryFilterIndex = 1
	_, _ = model.updateLibraryFilterMenu(tea.KeyMsg{Type: tea.KeyEnter})
	if model.libraryFilterType != "LLM" {
		t.Fatalf("expected first type filter to be LLM, got %q", model.libraryFilterType)
	}
	if len(model.libraryRows) != 1 || model.libraryRows[0].ModelType != "LLM" {
		t.Fatalf("type filter did not apply: %+v", model.libraryRows)
	}

	model.libraryFilterIndex = 0
	_, _ = model.updateLibraryFilterMenu(tea.KeyMsg{Type: tea.KeyEnter})
	model.librarySearchInput.SetValue("llama")
	_, _ = model.updateLibraryFilterMenu(tea.KeyMsg{Type: tea.KeyEnter})
	if model.librarySearch != "llama" || len(model.libraryRows) != 1 {
		t.Fatalf("search filter did not apply: search=%q rows=%d", model.librarySearch, len(model.libraryRows))
	}
}

func TestLibrary_FilterMenuValuesUseUnfilteredRows(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	models := []state.ModelMetadata{
		{
			DownloadURL: "https://example.com/llm.gguf",
			ModelName:   "llama",
			ModelType:   "LLM",
			Source:      "huggingface",
			Dest:        "/models/llm.gguf",
		},
		{
			DownloadURL: "https://example.com/lora.safetensors",
			ModelName:   "portrait",
			ModelType:   "LoRA",
			Source:      "civitai",
			Dest:        "/models/lora.safetensors",
		},
	}
	for i := range models {
		if err := db.UpsertMetadata(&models[i]); err != nil {
			t.Fatalf("seed metadata: %v", err)
		}
	}

	model.libraryFilterType = "LLM"
	model.refreshLibraryData()
	if len(model.libraryRows) != 1 || model.libraryRows[0].ModelType != "LLM" {
		t.Fatalf("expected active type filter to show LLM only, got %+v", model.libraryRows)
	}

	model.libraryFilterMenu = true
	model.libraryFilterIndex = 1
	_, _ = model.updateLibraryFilterMenu(tea.KeyMsg{Type: tea.KeyEnter})

	if model.libraryFilterType != "LoRA" {
		t.Fatalf("expected type menu to cycle to LoRA, got %q", model.libraryFilterType)
	}
	if len(model.libraryRows) != 1 || model.libraryRows[0].ModelType != "LoRA" {
		t.Fatalf("expected LoRA row after cycling type filter, got %+v", model.libraryRows)
	}
}

func TestLibrary_FilterMenuClearResetsAllFilters(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	models := []state.ModelMetadata{
		{
			DownloadURL: "https://example.com/llm.gguf",
			ModelName:   "llama",
			ModelType:   "LLM",
			Source:      "huggingface",
			Dest:        "/models/llm.gguf",
			Favorite:    true,
		},
		{
			DownloadURL: "https://example.com/lora.safetensors",
			ModelName:   "portrait",
			ModelType:   "LoRA",
			Source:      "civitai",
			Dest:        "/models/lora.safetensors",
		},
	}
	for i := range models {
		if err := db.UpsertMetadata(&models[i]); err != nil {
			t.Fatalf("seed metadata: %v", err)
		}
	}

	model.librarySearch = "llama"
	model.librarySearchInput.SetValue("llama")
	model.libraryFilterType = "LoRA"
	model.libraryFilterSource = "civitai"
	model.libraryShowFavorites = true
	model.refreshLibraryData()
	if len(model.libraryRows) != 0 {
		t.Fatalf("expected combined filters to hide all rows, got %d", len(model.libraryRows))
	}

	model.libraryFilterMenu = true
	model.libraryFilterIndex = 4
	_, _ = model.updateLibraryFilterMenu(tea.KeyMsg{Type: tea.KeyEnter})

	if model.librarySearch != "" || model.librarySearchInput.Value() != "" {
		t.Fatalf("search was not cleared: search=%q input=%q", model.librarySearch, model.librarySearchInput.Value())
	}
	if model.libraryFilterType != "" || model.libraryFilterSource != "" || model.libraryShowFavorites {
		t.Fatalf("filters were not cleared: type=%q source=%q favorites=%v", model.libraryFilterType, model.libraryFilterSource, model.libraryShowFavorites)
	}
	if len(model.libraryRows) != 2 {
		t.Fatalf("expected all rows after clearing filters, got %d", len(model.libraryRows))
	}
}

func TestLibrary_SelectionPersistsAcrossFiltersAndTabs(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	selected := state.ModelMetadata{
		DownloadURL: "https://example.com/selected.gguf",
		ModelName:   "selected",
		ModelType:   "LLM",
		Source:      "huggingface",
		Dest:        "/models/selected.gguf",
	}
	hidden := state.ModelMetadata{
		DownloadURL: "https://example.com/hidden.safetensors",
		ModelName:   "hidden",
		ModelType:   "LoRA",
		Source:      "civitai",
		Dest:        "/models/hidden.safetensors",
	}
	if err := db.UpsertMetadata(&selected); err != nil {
		t.Fatalf("seed selected: %v", err)
	}
	if err := db.UpsertMetadata(&hidden); err != nil {
		t.Fatalf("seed hidden: %v", err)
	}

	model.activeTab = 4
	model.refreshLibraryData()
	for i, row := range model.libraryRows {
		if row.DownloadURL == selected.DownloadURL {
			model.librarySelected = i
			break
		}
	}
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	selectedKey := libraryKey(selected)
	if !model.librarySelectedKeys[selectedKey] {
		t.Fatal("expected selected model to be tracked")
	}

	model.libraryFilterType = "LoRA"
	model.refreshLibraryData()
	if model.librarySelectedKeys[selectedKey] != true {
		t.Fatal("selection should remain tracked while filtered out")
	}
	model.libraryFilterType = ""
	model.refreshLibraryData()
	if !model.librarySelectedKeys[selectedKey] {
		t.Fatal("selection should survive clearing filters")
	}

	model.activeTab = 0
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if model.activeTab != 4 || !model.librarySelectedKeys[selectedKey] {
		t.Fatal("library selection should survive tab navigation")
	}
}

func TestLibraryKeyUsesURLAndDest(t *testing.T) {
	first := state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		Dest:        "/models/a.gguf",
	}
	second := state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		Dest:        "/models/b.gguf",
	}

	if libraryKey(first) == libraryKey(second) {
		t.Fatal("expected same URL in different destinations to have distinct library keys")
	}
}

func TestLibrary_BulkFavoriteToggle(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	createTestMetadata(t, db, 2)
	model.activeTab = 4
	model.refreshLibraryData()
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if cmd == nil {
		t.Fatal("expected favorite command")
	}
	_, _ = model.Update(cmd())

	for _, row := range model.libraryRows {
		got, err := db.GetMetadata(row.DownloadURL)
		if err != nil {
			t.Fatalf("get metadata: %v", err)
		}
		if !got.Favorite {
			t.Fatalf("expected %s to be favorite", row.DownloadURL)
		}
	}

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if cmd == nil {
		t.Fatal("expected unfavorite command")
	}
	_, _ = model.Update(cmd())
	for _, row := range model.libraryRows {
		got, err := db.GetMetadata(row.DownloadURL)
		if err != nil {
			t.Fatalf("get metadata: %v", err)
		}
		if got.Favorite {
			t.Fatalf("expected %s to be unfavorited", row.DownloadURL)
		}
	}
}

func TestLibrary_BulkMessagePrunesActedSelectionKeys(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	model.librarySelectedKeys["acted"] = true
	model.librarySelectedKeys["kept"] = true

	_, _ = model.Update(libraryBulkMsg{
		action: "favorite",
		count:  1,
		keys:   []string{"acted"},
	})

	if model.librarySelectedKeys["acted"] {
		t.Fatal("expected acted key to be pruned from selection")
	}
	if !model.librarySelectedKeys["kept"] {
		t.Fatal("expected unrelated selection key to remain")
	}
}

func TestLibrary_RetrySkipsLocalURLs(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	cmd := model.retryLibraryRows([]state.ModelMetadata{{
		DownloadURL: "file:///models/local.gguf",
		ModelName:   "local",
		Dest:        "/models/local.gguf",
	}})
	if len(model.running) != 0 {
		t.Fatalf("local retry should not register running downloads: %+v", model.running)
	}
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg != nil {
		t.Fatalf("local retry should not start a command, got %T", msg)
	}
}

func TestLibrary_RetryCleansRunningStateOnUpsertError(t *testing.T) {
	model, db, _ := setupTestLibrary(t)

	row := state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		ModelName:   "remote",
		Dest:        "/models/model.gguf",
	}
	cmd := model.retryLibraryRows([]state.ModelMetadata{row})
	if cmd == nil {
		t.Fatal("expected retry command")
	}
	key := row.DownloadURL + "|" + row.Dest
	if _, ok := model.running[key]; !ok {
		t.Fatal("expected retry to register running state before command executes")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	raw := cmd()
	if batch, ok := raw.(tea.BatchMsg); ok && len(batch) == 1 {
		raw = batch[0]()
	}
	msg, ok := raw.(libraryBulkMsg)
	if !ok {
		t.Fatalf("expected libraryBulkMsg, got %T", raw)
	}
	if msg.err == nil || len(msg.runningKeys) != 1 || msg.runningKeys[0] != key {
		t.Fatalf("expected retry upsert error with cleanup key, got %+v", msg)
	}
	_, _ = model.Update(msg)
	if _, ok := model.running[key]; ok {
		t.Fatal("expected failed retry to clear running state")
	}
	if _, ok := model.retrying[key]; ok {
		t.Fatal("expected failed retry to clear retrying state")
	}
}

func TestLibrary_BulkExportSelectedCatalog(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()
	model.cfg.General.DataRoot = t.TempDir()

	createTestMetadata(t, db, 2)
	model.activeTab = 4
	model.refreshLibraryData()
	model.librarySelectedKeys[libraryKey(model.libraryRows[0])] = true

	msg := model.exportLibraryRowsCmd(model.selectedLibraryRows())()
	result, ok := msg.(libraryBulkMsg)
	if !ok {
		t.Fatalf("expected libraryBulkMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("export failed: %v", result.err)
	}
	if result.count != 1 {
		t.Fatalf("expected one exported entry, got %d", result.count)
	}
	data, err := os.ReadFile(result.path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var exported catalog.Catalog
	if err := json.Unmarshal(data, &exported); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if len(exported.Models) != 1 {
		t.Fatalf("expected one catalog model, got %d", len(exported.Models))
	}
}

func TestLibrary_DetailModelRebindsAfterRefresh(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	meta := state.ModelMetadata{
		DownloadURL: "https://example.com/detail.gguf",
		ModelName:   "detail",
		Dest:        "/models/detail.gguf",
	}
	if err := db.UpsertMetadata(&meta); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	model.refreshLibraryData()
	model.libraryViewingDetail = true
	model.libraryDetailModel = &model.libraryRows[0]

	meta.Favorite = true
	if err := db.UpsertMetadata(&meta); err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	model.refreshLibraryData()
	if model.libraryDetailModel == nil || !model.libraryDetailModel.Favorite {
		t.Fatalf("expected detail model to rebind to refreshed row, got %+v", model.libraryDetailModel)
	}
}

func TestLibrary_BulkVerifyUpdatesDownloadRow(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	path := filepath.Join(model.cfg.General.DownloadRoot, "model.gguf")
	if err := os.WriteFile(path, []byte("model"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}
	meta := state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		ModelName:   "model",
		Dest:        path,
	}
	if err := db.UpsertMetadata(&meta); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	model.refreshLibraryData()

	msg := model.verifyLibraryRowsCmd(model.libraryRows)()
	result, ok := msg.(libraryBulkMsg)
	if !ok {
		t.Fatalf("expected libraryBulkMsg, got %T", msg)
	}
	if result.err != nil || result.count != 1 {
		t.Fatalf("verify failed: %+v", result)
	}
	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "completed" || rows[0].ActualSHA256 == "" {
		t.Fatalf("expected completed verified row, got %+v", rows)
	}
}

func TestLibrary_DeleteStagedDataRequiresConfirmation(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	path := filepath.Join(model.cfg.General.DownloadRoot, "staged.gguf")
	if err := os.WriteFile(path, []byte("staged"), 0o644); err != nil {
		t.Fatalf("write staged: %v", err)
	}
	meta := state.ModelMetadata{
		DownloadURL: "https://example.com/staged.gguf",
		ModelName:   "staged",
		Dest:        path,
	}
	if err := db.UpsertMetadata(&meta); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	if err := db.UpsertDownload(state.DownloadRow{URL: meta.DownloadURL, Dest: meta.Dest, Status: "completed"}); err != nil {
		t.Fatalf("seed download: %v", err)
	}
	model.activeTab = 4
	model.refreshLibraryData()

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if cmd != nil {
		t.Fatal("delete should wait for confirmation")
	}
	if model.libraryConfirm == nil {
		t.Fatal("expected confirmation state")
	}

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected delete command after confirmation")
	}
	msg := cmd()
	result, ok := msg.(libraryBulkMsg)
	if !ok {
		t.Fatalf("expected libraryBulkMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("delete failed: %v", result.err)
	}
	if len(result.keys) != 1 || result.keys[0] != libraryKey(meta) {
		t.Fatalf("expected acted key in delete result, got %+v", result.keys)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected staged file to be removed, err=%v", err)
	}
	rows, err := db.ListDownloads()
	if err != nil {
		t.Fatalf("list downloads: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected download row deleted, got %+v", rows)
	}
	if _, err := db.GetMetadata(meta.DownloadURL); err != nil {
		t.Fatalf("metadata should be kept: %v", err)
	}
}

func TestLibrary_DeleteSkipsNonStagedRows(t *testing.T) {
	model, db, cleanup := setupTestLibrary(t)
	defer cleanup()

	meta := state.ModelMetadata{
		DownloadURL: "https://example.com/placed.gguf",
		ModelName:   "placed",
		Dest:        filepath.Join(t.TempDir(), "placed.gguf"),
	}
	if err := db.UpsertMetadata(&meta); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}
	model.activeTab = 4
	model.refreshLibraryData()

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if cmd != nil {
		t.Fatal("non-staged delete should not start a command")
	}
	if model.libraryConfirm != nil {
		t.Fatal("non-staged delete should not open confirmation")
	}
}

func TestLibrary_StagedPathRejectsRootAndDirectories(t *testing.T) {
	model, _, cleanup := setupTestLibrary(t)
	defer cleanup()

	root := model.cfg.General.DownloadRoot
	dir := filepath.Join(root, "nested")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("make dir: %v", err)
	}
	file := filepath.Join(root, "model.gguf")
	if err := os.WriteFile(file, []byte("model"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if model.isStagedLibraryPath(root) {
		t.Fatal("download root should not be deletable")
	}
	if model.isStagedLibraryPath(dir) {
		t.Fatal("directories under download root should not be deletable")
	}
	if !model.isStagedLibraryPath(file) {
		t.Fatal("files under download root should be staged")
	}
}

// Helper function to create pointer to time
func ptrTime(t time.Time) *time.Time {
	return &t
}
