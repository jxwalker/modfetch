package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/config"
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

// Helper function to create pointer to time
func ptrTime(t time.Time) *time.Time {
	return &t
}
