package tui

import (
	"database/sql"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/state"
)

// TestFetchAndStoreMetadataCmd_NilDB tests that metadata fetching handles nil database gracefully
func TestFetchAndStoreMetadataCmd_NilDB(t *testing.T) {
	m := &Model{
		st:  nil, // nil database
		log: logging.New("error", false),
	}

	// Should return nil command when database is nil
	cmd := m.fetchAndStoreMetadataCmd("https://example.com/model.gguf", "/path/to/model.gguf", "/path/to/model.gguf")
	if cmd != nil {
		t.Error("fetchAndStoreMetadataCmd should return nil when database is nil")
	}
}

// TestMetadataStorage tests that metadata can be stored and retrieved
func TestMetadataStorage(t *testing.T) {
	// Create in-memory test database
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &state.DB{SQL: sqlDB}

	// Initialize metadata table
	if err := db.InitMetadataTable(); err != nil {
		t.Fatalf("failed to initialize metadata table: %v", err)
	}

	// Create test metadata
	meta := &state.ModelMetadata{
		DownloadURL:  "https://huggingface.co/TheBloke/Llama-2-7B/resolve/main/model.gguf",
		Dest:         "/downloads/model.gguf",
		ModelName:    "Llama 2 7B",
		ModelID:      "TheBloke/Llama-2-7B",
		Source:       "huggingface",
		ModelType:    "LLM",
		Quantization: "Q4_K_M",
		Author:       "TheBloke",
		FileSize:     4096 * 1024 * 1024, // 4GB
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Store metadata
	err = db.UpsertMetadata(meta)
	if err != nil {
		t.Fatalf("UpsertMetadata() error = %v", err)
	}

	// Retrieve metadata
	retrieved, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	// Verify metadata
	if retrieved.ModelName != meta.ModelName {
		t.Errorf("ModelName = %q, want %q", retrieved.ModelName, meta.ModelName)
	}

	if retrieved.Source != meta.Source {
		t.Errorf("Source = %q, want %q", retrieved.Source, meta.Source)
	}

	if retrieved.ModelType != meta.ModelType {
		t.Errorf("ModelType = %q, want %q", retrieved.ModelType, meta.ModelType)
	}

	if retrieved.Quantization != meta.Quantization {
		t.Errorf("Quantization = %q, want %q", retrieved.Quantization, meta.Quantization)
	}
}

// TestMetadataFiltering tests filtering metadata by various criteria
func TestMetadataFiltering(t *testing.T) {
	// Create in-memory test database
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &state.DB{SQL: sqlDB}

	if err := db.InitMetadataTable(); err != nil {
		t.Fatalf("failed to initialize metadata table: %v", err)
	}

	// Insert test models
	models := []state.ModelMetadata{
		{
			DownloadURL: "https://huggingface.co/model1.gguf",
			ModelName:   "Llama Model",
			Source:      "huggingface",
			ModelType:   "LLM",
			Tags:        []string{"llama", "chat"},
			UserRating:  5,
			Favorite:    true,
		},
		{
			DownloadURL: "https://civitai.com/model2.safetensors",
			ModelName:   "Realistic Checkpoint",
			Source:      "civitai",
			ModelType:   "Checkpoint",
			Tags:        []string{"realistic", "photo"},
			UserRating:  4,
			Favorite:    false,
		},
		{
			DownloadURL: "https://huggingface.co/model3.safetensors",
			ModelName:   "Style LoRA",
			Source:      "huggingface",
			ModelType:   "LoRA",
			Tags:        []string{"style", "anime"},
			UserRating:  3,
			Favorite:    true,
		},
	}

	for _, m := range models {
		if err := db.UpsertMetadata(&m); err != nil {
			t.Fatalf("UpsertMetadata() error = %v", err)
		}
	}

	// Test: Filter by source
	results, err := db.ListMetadata(state.MetadataFilters{Source: "huggingface"})
	if err != nil {
		t.Fatalf("ListMetadata(source) error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListMetadata(source=huggingface) count = %d, want 2", len(results))
	}

	// Test: Filter by model type
	results, err = db.ListMetadata(state.MetadataFilters{ModelType: "LLM"})
	if err != nil {
		t.Fatalf("ListMetadata(type) error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("ListMetadata(type=LLM) count = %d, want 1", len(results))
	}

	// Test: Filter by favorites
	results, err = db.ListMetadata(state.MetadataFilters{Favorite: true})
	if err != nil {
		t.Fatalf("ListMetadata(favorite) error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListMetadata(favorite=true) count = %d, want 2", len(results))
	}

	// Test: Filter by minimum rating
	results, err = db.ListMetadata(state.MetadataFilters{MinRating: 4})
	if err != nil {
		t.Fatalf("ListMetadata(rating) error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("ListMetadata(rating>=4) count = %d, want 2", len(results))
	}

	// Test: Search
	results, err = db.SearchMetadata("Llama")
	if err != nil {
		t.Fatalf("SearchMetadata() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("SearchMetadata('Llama') count = %d, want 1", len(results))
	}
}

// TestMetadataUsageTracking tests usage statistics
func TestMetadataUsageTracking(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &state.DB{SQL: sqlDB}

	if err := db.InitMetadataTable(); err != nil {
		t.Fatalf("failed to initialize metadata table: %v", err)
	}

	meta := &state.ModelMetadata{
		DownloadURL: "https://example.com/model.gguf",
		ModelName:   "Test Model",
	}

	if err := db.UpsertMetadata(meta); err != nil {
		t.Fatalf("UpsertMetadata() error = %v", err)
	}

	// Track usage
	if err := db.UpdateMetadataUsage(meta.DownloadURL); err != nil {
		t.Fatalf("UpdateMetadataUsage() error = %v", err)
	}

	// Verify usage updated
	retrieved, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}

	if retrieved.TimesUsed != 1 {
		t.Errorf("TimesUsed = %d, want 1", retrieved.TimesUsed)
	}

	if retrieved.LastUsed == nil {
		t.Error("LastUsed should not be nil after usage update")
	}

	// Track again
	time.Sleep(1100 * time.Millisecond) // SQLite strftime has second precision
	if err := db.UpdateMetadataUsage(meta.DownloadURL); err != nil {
		t.Fatalf("UpdateMetadataUsage() second call error = %v", err)
	}

	retrieved2, err := db.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("GetMetadata() second call error = %v", err)
	}

	if retrieved2.TimesUsed != 2 {
		t.Errorf("TimesUsed = %d, want 2", retrieved2.TimesUsed)
	}

	if !retrieved2.LastUsed.After(*retrieved.LastUsed) {
		t.Error("LastUsed should be updated to later time")
	}
}

// TestModelWithMetadata tests that TUI model can work with metadata database
func TestModelWithMetadata(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	db := &state.DB{SQL: sqlDB}

	if err := db.InitMetadataTable(); err != nil {
		t.Fatalf("failed to initialize metadata table: %v", err)
	}

	// Create TUI model with metadata support
	cfg := &config.Config{Version: 1}
	cfg.General.DataRoot = "/tmp"

	m := &Model{
		cfg: cfg,
		st:  db,
		log: logging.New("error", false),
	}

	// Verify model has access to metadata storage
	if m.st == nil {
		t.Error("Model should have database reference")
	}

	// Store some metadata
	meta := &state.ModelMetadata{
		DownloadURL: "https://example.com/test.gguf",
		ModelName:   "Test Model",
		Source:      "huggingface",
	}

	if err := m.st.UpsertMetadata(meta); err != nil {
		t.Fatalf("Model should be able to store metadata: %v", err)
	}

	// Retrieve metadata
	retrieved, err := m.st.GetMetadata(meta.DownloadURL)
	if err != nil {
		t.Fatalf("Model should be able to retrieve metadata: %v", err)
	}

	if retrieved.ModelName != meta.ModelName {
		t.Errorf("Retrieved model name = %q, want %q", retrieved.ModelName, meta.ModelName)
	}
}
