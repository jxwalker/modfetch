package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/metadata"
	"github.com/jxwalker/modfetch/internal/scanner"
	"github.com/jxwalker/modfetch/internal/state"
)

// Integration tests verify the interaction between Scanner, Database, and Library components
// These tests simulate real-world workflows from file discovery to UI display

// TestIntegration_ScanToLibraryFlow tests the complete flow:
// 1. Create model files on disk
// 2. Scanner discovers and extracts metadata
// 3. Database stores metadata
// 4. Library loads and displays models
func TestIntegration_ScanToLibraryFlow(t *testing.T) {
	// Setup: Create test environment
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	modelsDir := filepath.Join(tmpDir, "models")

	// Create test model files
	testFiles := []struct {
		path string
		size int64
	}{
		{filepath.Join(modelsDir, "llama-2-7b.Q4_K_M.gguf"), 4 * 1024 * 1024 * 1024},
		{filepath.Join(modelsDir, "mistral-7b-instruct.Q5_K_S.gguf"), 5 * 1024 * 1024 * 1024},
		{filepath.Join(modelsDir, "sdxl-base.fp16.safetensors"), 7 * 1024 * 1024 * 1024},
	}

	for _, tf := range testFiles {
		if err := os.MkdirAll(filepath.Dir(tf.path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		f, err := os.Create(tf.path)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		if err := f.Truncate(tf.size); err != nil {
			f.Close()
			t.Fatalf("Failed to set file size: %v", err)
		}
		f.Close()
	}

	// Step 1: Initialize database
	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Step 2: Run scanner
	scan := scanner.NewScanner(db)
	result, err := scan.ScanDirectories([]string{modelsDir})
	if err != nil {
		t.Fatalf("Scanner failed: %v", err)
	}

	// Verify scan results
	if result.FilesScanned != 3 {
		t.Errorf("Expected 3 files scanned, got %d", result.FilesScanned)
	}
	if result.ModelsFound != 3 {
		t.Errorf("Expected 3 models found, got %d", result.ModelsFound)
	}
	if result.ModelsAdded != 3 {
		t.Errorf("Expected 3 models added, got %d", result.ModelsAdded)
	}

	// Step 3: Load data via Library
	cfg := &config.Config{
		General: config.GeneralConfig{
			DataRoot:     tmpDir,
			DownloadRoot: modelsDir,
		},
	}

	model := New(cfg, db, "test").(*Model)
	model.w = 120
	model.h = 40
	model.activeTab = 5 // Library tab

	// Refresh library data
	model.refreshLibraryData()

	// Verify library loaded models
	if len(model.libraryRows) != 3 {
		t.Errorf("Expected 3 models in library, got %d", len(model.libraryRows))
	}

	// Step 4: Verify model data
	for i, expectedFile := range testFiles {
		baseName := filepath.Base(expectedFile.path)
		found := false

		for _, row := range model.libraryRows {
			if strings.Contains(row.Dest, baseName) {
				found = true

				// Verify metadata extraction worked
				if row.Source != "local" {
					t.Errorf("Model %d: Expected source 'local', got %s", i, row.Source)
				}
				if row.FileSize != expectedFile.size {
					t.Errorf("Model %d: Expected size %d, got %d", i, expectedFile.size, row.FileSize)
				}
				if row.Dest == "" {
					t.Errorf("Model %d: Dest path is empty", i)
				}
				break
			}
		}

		if !found {
			t.Errorf("Model not found in library: %s", baseName)
		}
	}

	// Step 5: Verify UI rendering
	output := model.renderLibrary()
	if !strings.Contains(output, "Model Library") {
		t.Error("Library view should contain header")
	}
	if !strings.Contains(output, "llama") {
		t.Error("Library should show llama model")
	}
	if !strings.Contains(output, "mistral") {
		t.Error("Library should show mistral model")
	}
	if !strings.Contains(output, "sdxl") {
		t.Error("Library should show sdxl model")
	}
}

// TestIntegration_MetadataFetchToLibrary tests:
// 1. Download with metadata fetch
// 2. Store in database
// 3. Display in library with rich metadata
func TestIntegration_MetadataFetchToLibrary(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Simulate metadata fetch from HuggingFace
	// (In real scenario, this would come from API)
	meta := &state.ModelMetadata{
		DownloadURL:    "https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/llama-2-7b.Q4_K_M.gguf",
		Dest:           filepath.Join(tmpDir, "llama-2-7b.Q4_K_M.gguf"),
		ModelName:      "Llama-2-7B-GGUF",
		ModelID:        "TheBloke/Llama-2-7B-GGUF",
		Version:        "main",
		Source:         "huggingface",
		Description:    "Llama 2 7B model quantized to Q4_K_M",
		Author:         "TheBloke",
		AuthorURL:      "https://huggingface.co/TheBloke",
		License:        "MIT",
		Tags:           []string{"llm", "text-generation", "llama"},
		ModelType:      "LLM",
		Architecture:   "Llama",
		ParameterCount: "7B",
		Quantization:   "Q4_K_M",
		FileSize:       int64(4 * 1024 * 1024 * 1024),
		FileFormat:     ".gguf",
		DownloadCount:  125000,
		HomepageURL:    "https://huggingface.co/TheBloke/Llama-2-7B-GGUF",
		RepoURL:        "https://huggingface.co/TheBloke/Llama-2-7B-GGUF",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Store metadata
	if err := db.UpsertMetadata(meta); err != nil {
		t.Fatalf("Failed to store metadata: %v", err)
	}

	// Load in library
	cfg := &config.Config{
		General: config.GeneralConfig{
			DataRoot:     tmpDir,
			DownloadRoot: tmpDir,
		},
	}

	model := New(cfg, db, "test").(*Model)
	model.activeTab = 5
	model.refreshLibraryData()

	// Verify rich metadata is available
	if len(model.libraryRows) != 1 {
		t.Fatalf("Expected 1 model in library, got %d", len(model.libraryRows))
	}

	loaded := model.libraryRows[0]

	// Verify all metadata fields
	tests := []struct {
		field    string
		expected string
		actual   string
	}{
		{"ModelName", "Llama-2-7B-GGUF", loaded.ModelName},
		{"ModelID", "TheBloke/Llama-2-7B-GGUF", loaded.ModelID},
		{"Source", "huggingface", loaded.Source},
		{"Author", "TheBloke", loaded.Author},
		{"ModelType", "LLM", loaded.ModelType},
		{"Architecture", "Llama", loaded.Architecture},
		{"ParameterCount", "7B", loaded.ParameterCount},
		{"Quantization", "Q4_K_M", loaded.Quantization},
		{"License", "MIT", loaded.License},
	}

	for _, tt := range tests {
		if tt.actual != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.field, tt.expected, tt.actual)
		}
	}

	// Verify tags
	if len(loaded.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(loaded.Tags))
	}

	// Verify detail view rendering
	model.libraryDetailModel = &loaded
	model.libraryViewingDetail = true
	output := model.renderLibraryDetail()

	detailSections := []string{
		"Llama-2-7B-GGUF",
		"Type: LLM",
		"Architecture: Llama",
		"Parameters: 7B",
		"Quantization: Q4_K_M",
		"Author: TheBloke",
		"License: MIT",
		"Tags",
		"llm",
		"text-generation",
	}

	for _, section := range detailSections {
		if !strings.Contains(output, section) {
			t.Errorf("Detail view should contain %q", section)
		}
	}
}

// TestIntegration_SearchFiltering tests:
// 1. Multiple models in database
// 2. Search functionality
// 3. Filter by type
// 4. Filter by source
func TestIntegration_SearchFiltering(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create diverse set of models
	models := []state.ModelMetadata{
		{
			ModelName: "llama-2-7b",
			ModelType: "LLM",
			Source:    "huggingface",
			Dest:      "/models/llama1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ModelName: "llama-2-13b",
			ModelType: "LLM",
			Source:    "huggingface",
			Dest:      "/models/llama2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ModelName: "mistral-7b",
			ModelType: "LLM",
			Source:    "huggingface",
			Dest:      "/models/mistral",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ModelName: "sdxl-lora-portrait",
			ModelType: "LoRA",
			Source:    "civitai",
			Dest:      "/models/lora1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ModelName: "sdxl-lora-style",
			ModelType: "LoRA",
			Source:    "civitai",
			Dest:      "/models/lora2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, m := range models {
		if err := db.UpsertMetadata(&m); err != nil {
			t.Fatalf("Failed to insert metadata: %v", err)
		}
	}

	cfg := &config.Config{
		General: config.GeneralConfig{
			DataRoot:     tmpDir,
			DownloadRoot: tmpDir,
		},
	}

	model := New(cfg, db, "test").(*Model)
	model.activeTab = 5

	// Test 1: No filters - should show all 5
	model.refreshLibraryData()
	if len(model.libraryRows) != 5 {
		t.Errorf("Expected 5 models without filters, got %d", len(model.libraryRows))
	}

	// Test 2: Filter by type = LLM - should show 3
	model.libraryFilterType = "LLM"
	model.refreshLibraryData()
	if len(model.libraryRows) != 3 {
		t.Errorf("Expected 3 LLM models, got %d", len(model.libraryRows))
	}
	for _, row := range model.libraryRows {
		if row.ModelType != "LLM" {
			t.Errorf("Filter by LLM failed: got %s", row.ModelType)
		}
	}

	// Test 3: Filter by source = civitai - should show 2
	model.libraryFilterType = ""
	model.libraryFilterSource = "civitai"
	model.refreshLibraryData()
	if len(model.libraryRows) != 2 {
		t.Errorf("Expected 2 civitai models, got %d", len(model.libraryRows))
	}
	for _, row := range model.libraryRows {
		if row.Source != "civitai" {
			t.Errorf("Filter by civitai failed: got %s", row.Source)
		}
	}

	// Test 4: Combined filter (type=LoRA, source=civitai) - should show 2
	model.libraryFilterType = "LoRA"
	model.libraryFilterSource = "civitai"
	model.refreshLibraryData()
	if len(model.libraryRows) != 2 {
		t.Errorf("Expected 2 LoRA+civitai models, got %d", len(model.libraryRows))
	}

	// Test 5: Search by name "llama" - should show 2
	model.libraryFilterType = ""
	model.libraryFilterSource = ""
	model.librarySearch = "llama"
	model.refreshLibraryData()
	if len(model.libraryRows) != 2 {
		t.Errorf("Expected 2 models matching 'llama', got %d", len(model.libraryRows))
	}
	for _, row := range model.libraryRows {
		if !strings.Contains(strings.ToLower(row.ModelName), "llama") {
			t.Errorf("Search for 'llama' returned non-matching model: %s", row.ModelName)
		}
	}

	// Test 6: Search by name "sdxl" - should show 2
	model.librarySearch = "sdxl"
	model.refreshLibraryData()
	if len(model.libraryRows) != 2 {
		t.Errorf("Expected 2 models matching 'sdxl', got %d", len(model.libraryRows))
	}
}

// TestIntegration_DuplicateDetection tests:
// 1. Scan directory
// 2. Re-scan same directory
// 3. Verify duplicates are skipped
func TestIntegration_DuplicateDetection(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	modelsDir := filepath.Join(tmpDir, "models")

	// Create test file
	testFile := filepath.Join(modelsDir, "model.gguf")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	f.Close()

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	scan := scanner.NewScanner(db)

	// First scan
	result1, err := scan.ScanDirectories([]string{modelsDir})
	if err != nil {
		t.Fatalf("First scan failed: %v", err)
	}

	if result1.FilesScanned != 1 {
		t.Errorf("First scan: expected 1 file scanned, got %d", result1.FilesScanned)
	}
	if result1.ModelsAdded != 1 {
		t.Errorf("First scan: expected 1 model added, got %d", result1.ModelsAdded)
	}
	if result1.ModelsSkipped != 0 {
		t.Errorf("First scan: expected 0 skipped, got %d", result1.ModelsSkipped)
	}

	// Second scan (same directory)
	result2, err := scan.ScanDirectories([]string{modelsDir})
	if err != nil {
		t.Fatalf("Second scan failed: %v", err)
	}

	if result2.FilesScanned != 1 {
		t.Errorf("Second scan: expected 1 file scanned, got %d", result2.FilesScanned)
	}
	if result2.ModelsAdded != 0 {
		t.Errorf("Second scan: expected 0 models added (duplicate), got %d", result2.ModelsAdded)
	}
	if result2.ModelsSkipped != 1 {
		t.Errorf("Second scan: expected 1 skipped (duplicate), got %d", result2.ModelsSkipped)
	}

	// Verify only 1 model in database
	filters := state.MetadataFilters{Limit: 100}
	stored, err := db.ListMetadata(filters)
	if err != nil {
		t.Fatalf("Failed to list metadata: %v", err)
	}

	if len(stored) != 1 {
		t.Errorf("Expected 1 model in database after 2 scans, got %d", len(stored))
	}
}

// TestIntegration_FavoriteManagement tests:
// 1. Mark model as favorite
// 2. Verify stored in database
// 3. Verify displayed in library
// 4. Verify filter by favorites works
func TestIntegration_FavoriteManagement(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test models
	models := []state.ModelMetadata{
		{
			ModelName: "favorite-model",
			Favorite:  true,
			Dest:      "/models/fav",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ModelName: "normal-model",
			Favorite:  false,
			Dest:      "/models/normal",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, m := range models {
		if err := db.UpsertMetadata(&m); err != nil {
			t.Fatalf("Failed to insert metadata: %v", err)
		}
	}

	cfg := &config.Config{
		General: config.GeneralConfig{
			DataRoot:     tmpDir,
			DownloadRoot: tmpDir,
		},
	}

	model := New(cfg, db, "test").(*Model)
	model.activeTab = 5

	// Test 1: Load all models
	model.refreshLibraryData()
	if len(model.libraryRows) != 2 {
		t.Errorf("Expected 2 models, got %d", len(model.libraryRows))
	}

	// Test 2: Verify favorite indicator in output
	output := model.renderLibrary()
	if !strings.Contains(output, "★") {
		t.Error("Favorite model should show star indicator")
	}

	// Test 3: Filter by favorites
	model.libraryShowFavorites = true
	model.refreshLibraryData()
	if len(model.libraryRows) != 1 {
		t.Errorf("Expected 1 favorite model, got %d", len(model.libraryRows))
	}
	if !model.libraryRows[0].Favorite {
		t.Error("Filtered model should be marked as favorite")
	}
}

// TestIntegration_MetadataRegistry tests the metadata fetcher integration
func TestIntegration_MetadataRegistry(t *testing.T) {
	// Create metadata registry
	registry := metadata.NewRegistry()

	// Test HuggingFace URL detection
	hfURL := "https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/model.gguf"
	if !registry.CanHandle(hfURL) {
		t.Error("Registry should handle HuggingFace URLs")
	}

	// Test CivitAI URL detection
	civURL := "https://civitai.com/api/download/models/123456"
	if !registry.CanHandle(civURL) {
		t.Error("Registry should handle CivitAI URLs")
	}

	// Test direct URL (should use default fetcher)
	directURL := "https://example.com/model.gguf"
	ctx := context.Background()
	meta, err := registry.FetchMetadata(ctx, directURL)
	if err != nil {
		t.Fatalf("Should handle direct URLs: %v", err)
	}
	if meta.Source != "direct" {
		t.Errorf("Expected source 'direct', got %s", meta.Source)
	}
}
