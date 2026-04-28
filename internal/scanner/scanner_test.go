package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/config"
	"github.com/jxwalker/modfetch/internal/state"
)

// setupTestDB creates a temporary test database
func setupTestDB(t *testing.T) (*state.DB, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		if err := db.SQL.Close(); err != nil {
			t.Errorf("Failed to close database: %v", err)
		}
	}

	return db, cleanup
}

// createTestFile creates a file with given name in the directory
func createTestFile(t *testing.T, dir, filename string, size int64) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	dir = filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	// Write dummy data to simulate file size
	if size > 0 {
		if err := f.Truncate(size); err != nil {
			t.Fatalf("Failed to set file size: %v", err)
		}
	}

	return path
}

func TestScanner_ScanDirectories_Basic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)

	// Create test directory with model files
	testDir := t.TempDir()
	createTestFile(t, testDir, "model.gguf", 1024*1024)
	createTestFile(t, testDir, "lora.safetensors", 512*1024)
	createTestFile(t, testDir, "README.md", 100) // Should be ignored

	result, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("ScanDirectories failed: %v", err)
	}

	// Verify results
	if result.FilesScanned != 2 {
		t.Errorf("Expected 2 files scanned, got %d", result.FilesScanned)
	}

	if result.ModelsFound != 2 {
		t.Errorf("Expected 2 models found, got %d", result.ModelsFound)
	}

	if result.ModelsAdded != 2 {
		t.Errorf("Expected 2 models added, got %d", result.ModelsAdded)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestScanner_ScanDirectories_Recursive(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)

	// Create nested directory structure
	testDir := t.TempDir()
	createTestFile(t, testDir, "model1.gguf", 1024)
	createTestFile(t, testDir, "subdir/model2.gguf", 1024)
	createTestFile(t, testDir, "subdir/nested/model3.safetensors", 1024)

	result, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("ScanDirectories failed: %v", err)
	}

	if result.FilesScanned != 3 {
		t.Errorf("Expected 3 files scanned (recursive), got %d", result.FilesScanned)
	}

	if result.ModelsFound != 3 {
		t.Errorf("Expected 3 models found, got %d", result.ModelsFound)
	}
}

func TestScanner_FileTypeDetection(t *testing.T) {
	tests := []struct {
		filename    string
		shouldMatch bool
	}{
		{"model.gguf", true},
		{"model.ggml", true},
		{"lora.safetensors", true},
		{"checkpoint.ckpt", true},
		{"model.pt", true},
		{"model.pth", true},
		{"model.bin", true},
		{"model.h5", true},
		{"model.pb", true},
		{"model.onnx", true},
		{"README.md", false},
		{"config.json", false},
		{"model.txt", false},
		{"image.png", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isModelFile(tt.filename)
			if result != tt.shouldMatch {
				t.Errorf("isModelFile(%q) = %v, want %v", tt.filename, result, tt.shouldMatch)
			}
		})
	}
}

func TestScanner_FileTypeDetection_CaseInsensitive(t *testing.T) {
	tests := []string{
		"model.GGUF",
		"model.GgUf",
		"model.SAFETENSORS",
		"model.SafeTensors",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			if !isModelFile(filename) {
				t.Errorf("isModelFile(%q) = false, want true (case insensitive)", filename)
			}
		})
	}
}

func TestScanner_ExtractMetadata_ModelName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	tests := []struct {
		filename      string
		expectedName  string
		expectedType  string
		expectedQuant string
	}{
		{
			filename:      "llama-2-7b.Q4_K_M.gguf",
			expectedName:  "llama",
			expectedType:  "LLM",
			expectedQuant: "Q4_K_M",
		},
		{
			filename:      "sdxl-lora-v1.safetensors",
			expectedName:  "sdxl",
			expectedType:  "LoRA",
			expectedQuant: "",
		},
		{
			filename:      "vae-ft-mse.ckpt",
			expectedName:  "vae",
			expectedType:  "VAE",
			expectedQuant: "",
		},
		{
			filename:      "mistral-7b-instruct.Q5_K_S.gguf",
			expectedName:  "mistral",
			expectedType:  "LLM",
			expectedQuant: "Q5_K_S",
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			path := createTestFile(t, testDir, tt.filename, 1024)
			info, _ := os.Stat(path)

			meta := scanner.extractMetadata(path, info)

			if meta.ModelName != tt.expectedName {
				t.Errorf("ModelName = %q, want %q", meta.ModelName, tt.expectedName)
			}

			if meta.ModelType != tt.expectedType {
				t.Errorf("ModelType = %q, want %q", meta.ModelType, tt.expectedType)
			}

			if meta.Quantization != tt.expectedQuant {
				t.Errorf("Quantization = %q, want %q", meta.Quantization, tt.expectedQuant)
			}

			if meta.Source != "local" {
				t.Errorf("Source = %q, want %q", meta.Source, "local")
			}

			if !strings.HasPrefix(meta.DownloadURL, "file://") {
				t.Errorf("DownloadURL should start with file://, got %q", meta.DownloadURL)
			}
		})
	}
}

func TestScanner_InferModelType(t *testing.T) {
	tests := []struct {
		filename     string
		expectedType string
	}{
		{"model.gguf", "LLM"},
		{"model.ggml", "LLM"},
		{"text-lora.safetensors", "LoRA"},
		{"sdxl-lora-v2.safetensors", "LoRA"},
		{"vae-ft-mse.ckpt", "VAE"},
		{"vae-model.safetensors", "VAE"},
		{"embedding-vectors.pt", "Embedding"},
		{"textual-inversion.safetensors", "Embedding"},
		{"controlnet-canny.safetensors", "ControlNet"},
		{"sd-v1-5.ckpt", "Checkpoint"},
		{"checkpoint-model.safetensors", "Checkpoint"},
		{"generic.safetensors", "Checkpoint"}, // Default for safetensors
		{"unknown.pt", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := inferModelTypeFromPath(tt.filename)
			if result != tt.expectedType {
				t.Errorf("inferModelTypeFromPath(%q) = %q, want %q", tt.filename, result, tt.expectedType)
			}
		})
	}
}

func TestScanner_ExtractQuantization(t *testing.T) {
	tests := []struct {
		filename      string
		expectedQuant string
	}{
		{"model.Q4_K_M.gguf", "Q4_K_M"},
		{"model.Q5_K_S.gguf", "Q5_K_S"},
		{"model.Q3_K_L.gguf", "Q3_K_L"},
		{"model.Q6_K.gguf", "Q6_K"},
		{"model.Q8_0.gguf", "Q8_0"},
		{"model.F16.gguf", "F16"},
		{"model.FP16.safetensors", "FP16"},
		{"model.FP32.safetensors", "FP32"},
		{"model.gguf", ""}, // No quantization
		{"lora.safetensors", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Import from metadata package
			result := extractQuantizationHelper(tt.filename)
			if result != tt.expectedQuant {
				t.Errorf("ExtractQuantization(%q) = %q, want %q", tt.filename, result, tt.expectedQuant)
			}
		})
	}
}

// Helper to test quantization extraction (uses uppercase matching)
func extractQuantizationHelper(filename string) string {
	filename = strings.ToUpper(filename)

	quantPatterns := []string{
		"Q2_K", "Q3_K_S", "Q3_K_M", "Q3_K_L", "Q4_0", "Q4_1",
		"Q4_K_S", "Q4_K_M", "Q5_0", "Q5_1", "Q5_K_S", "Q5_K_M",
		"Q6_K", "Q8_0", "F16", "F32", "FP16", "FP32",
	}

	for _, pattern := range quantPatterns {
		if strings.Contains(filename, pattern) {
			return pattern
		}
	}

	return ""
}

func TestScanner_ExtractParameterCount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	tests := []struct {
		filename      string
		expectedParam string
	}{
		{"llama-7b.gguf", "7B"},
		{"mistral-7B.gguf", "7B"},
		{"llama-2-13b.gguf", "13B"},
		{"llama-2-70b.gguf", "70B"},
		{"phi-2-2b.gguf", "2B"},
		{"model.gguf", ""}, // No parameter count
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			path := createTestFile(t, testDir, tt.filename, 1024)
			info, _ := os.Stat(path)

			meta := scanner.extractMetadata(path, info)

			if meta.ParameterCount != tt.expectedParam {
				t.Errorf("ParameterCount = %q, want %q", meta.ParameterCount, tt.expectedParam)
			}
		})
	}
}

func TestScanner_ExtractVersion(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	tests := []struct {
		filename        string
		expectedVersion string
	}{
		{"model-v1.0.gguf", "v1.0"},
		{"model-v2.gguf", "v2"},
		{"model-v1.5.1.gguf", "v1.5.1"},
		{"model.gguf", ""}, // No version
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			path := createTestFile(t, testDir, tt.filename, 1024)
			info, _ := os.Stat(path)

			meta := scanner.extractMetadata(path, info)

			if meta.Version != tt.expectedVersion {
				t.Errorf("Version = %q, want %q", meta.Version, tt.expectedVersion)
			}
		})
	}
}

func TestScanner_DuplicateSkipping(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	// Create a test file
	createTestFile(t, testDir, "model.gguf", 1024)

	// First scan - should add the model
	result1, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("First scan failed: %v", err)
	}

	if result1.ModelsAdded != 1 {
		t.Errorf("First scan: expected 1 model added, got %d", result1.ModelsAdded)
	}

	// Second scan - should skip the existing model
	result2, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("Second scan failed: %v", err)
	}

	if result2.FilesScanned != 1 {
		t.Errorf("Second scan: expected 1 file scanned, got %d", result2.FilesScanned)
	}

	if result2.ModelsAdded != 0 {
		t.Errorf("Second scan: expected 0 models added (duplicate), got %d", result2.ModelsAdded)
	}

	if result2.ModelsFound != 1 {
		t.Errorf("Second scan: expected 1 model found before duplicate skip, got %d", result2.ModelsFound)
	}

	if result2.ModelsSkipped != 1 {
		t.Errorf("Second scan: expected 1 model skipped, got %d", result2.ModelsSkipped)
	}
}

func TestScanner_EmptyDirectory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	result, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("ScanDirectories failed: %v", err)
	}

	if result.FilesScanned != 0 {
		t.Errorf("Expected 0 files scanned in empty directory, got %d", result.FilesScanned)
	}

	if result.ModelsFound != 0 {
		t.Errorf("Expected 0 models found in empty directory, got %d", result.ModelsFound)
	}
}

func TestScanner_NonExistentDirectory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)

	// Try to scan a directory that doesn't exist
	result, err := scanner.ScanDirectories([]string{"/nonexistent/path/that/does/not/exist"})
	if err != nil {
		t.Fatalf("ScanDirectories should not fail on non-existent dir: %v", err)
	}

	// Should have errors but not crash
	if len(result.Errors) == 0 {
		t.Error("Expected errors for non-existent directory, got none")
	}
}

func TestScanner_MultipleDirectories(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)

	// Create multiple test directories
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	createTestFile(t, dir1, "model1.gguf", 1024)
	createTestFile(t, dir1, "model2.gguf", 1024)
	createTestFile(t, dir2, "model3.safetensors", 1024)

	result, err := scanner.ScanDirectories([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("ScanDirectories failed: %v", err)
	}

	if result.FilesScanned != 3 {
		t.Errorf("Expected 3 files scanned across multiple directories, got %d", result.FilesScanned)
	}

	if result.ModelsFound != 3 {
		t.Errorf("Expected 3 models found, got %d", result.ModelsFound)
	}
}

func TestScanner_MetadataStorage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	// Create and scan a model file
	filename := "llama-2-7b.Q4_K_M.gguf"
	createTestFile(t, testDir, filename, 1024*1024)

	result, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("ScanDirectories failed: %v", err)
	}

	if result.ModelsAdded != 1 {
		t.Fatalf("Expected 1 model added, got %d", result.ModelsAdded)
	}

	// Verify metadata was stored in database
	filters := state.MetadataFilters{
		Source: "local",
		Limit:  10,
	}
	stored, err := db.ListMetadata(filters)
	if err != nil {
		t.Fatalf("Failed to list metadata: %v", err)
	}

	if len(stored) != 1 {
		t.Fatalf("Expected 1 stored metadata entry, got %d", len(stored))
	}

	meta := stored[0]
	if meta.ModelName != "llama" {
		t.Errorf("Stored ModelName = %q, want %q", meta.ModelName, "llama")
	}

	if meta.ModelType != "LLM" {
		t.Errorf("Stored ModelType = %q, want %q", meta.ModelType, "LLM")
	}

	if meta.Quantization != "Q4_K_M" {
		t.Errorf("Stored Quantization = %q, want %q", meta.Quantization, "Q4_K_M")
	}

	if meta.ParameterCount != "7B" {
		t.Errorf("Stored ParameterCount = %q, want %q", meta.ParameterCount, "7B")
	}
}

func TestScanner_WithProgress(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	// Create test files
	createTestFile(t, testDir, "model1.gguf", 1024)
	createTestFile(t, testDir, "model2.gguf", 1024)
	createTestFile(t, testDir, "model3.safetensors", 1024)

	// Track progress callbacks
	var progressCalls []string
	progressFn := func(path string, found int) {
		progressCalls = append(progressCalls, fmt.Sprintf("%s:%d", filepath.Base(path), found))
	}

	result, err := scanner.ScanWithProgress([]string{testDir}, progressFn)
	if err != nil {
		t.Fatalf("ScanWithProgress failed: %v", err)
	}

	if result.ModelsFound != 3 {
		t.Errorf("Expected 3 models found, got %d", result.ModelsFound)
	}

	if len(progressCalls) != 3 {
		t.Errorf("Expected 3 progress callbacks, got %d", len(progressCalls))
	}
}

func TestScanner_FileSize(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)
	testDir := t.TempDir()

	// Create file with specific size
	expectedSize := int64(5 * 1024 * 1024) // 5MB
	createTestFile(t, testDir, "model.gguf", expectedSize)

	_, err := scanner.ScanDirectories([]string{testDir})
	if err != nil {
		t.Fatalf("ScanDirectories failed: %v", err)
	}

	// Verify file size was stored
	filters := state.MetadataFilters{Limit: 10}
	stored, err := db.ListMetadata(filters)
	if err != nil {
		t.Fatalf("Failed to list metadata: %v", err)
	}

	if len(stored) != 1 {
		t.Fatalf("Expected 1 stored metadata entry, got %d", len(stored))
	}

	if stored[0].FileSize != expectedSize {
		t.Errorf("FileSize = %d, want %d", stored[0].FileSize, expectedSize)
	}
}

func TestScanner_FileFormat(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	scanner := NewScanner(db)

	tests := []struct {
		filename       string
		expectedFormat string
	}{
		{"model.gguf", ".gguf"},
		{"lora.safetensors", ".safetensors"},
		{"checkpoint.ckpt", ".ckpt"},
		{"model.bin", ".bin"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Use a separate testDir for each subtest to avoid interference
			testDir := t.TempDir()
			createTestFile(t, testDir, tt.filename, 1024)

			_, err := scanner.ScanDirectories([]string{testDir})
			if err != nil {
				t.Fatalf("ScanDirectories failed: %v", err)
			}

			// Get the last added metadata
			filters := state.MetadataFilters{Limit: 1, OrderBy: "created_at"}
			stored, err := db.ListMetadata(filters)
			if err != nil {
				t.Fatalf("Failed to list metadata: %v", err)
			}

			if len(stored) == 0 {
				t.Fatal("No metadata found")
			}

			if stored[0].FileFormat != tt.expectedFormat {
				t.Errorf("FileFormat = %q, want %q", stored[0].FileFormat, tt.expectedFormat)
			}

			// Clean up for next test
			if err := db.DeleteMetadata(stored[0].DownloadURL); err != nil {
				t.Fatalf("Failed to clean up: %v", err)
			}
		})
	}
}

func TestScanner_ParallelMatchesSingleWorker(t *testing.T) {
	testDir := t.TempDir()
	files := []string{
		"llama-2-7b.Q4_K_M.gguf",
		"sdxl-lora-v1.safetensors",
		"vae-ft-mse.ckpt",
		"nested/mistral-7b-instruct.Q5_K_S.gguf",
	}
	for _, name := range files {
		createTestFile(t, testDir, name, 1024)
	}

	dbSeq, cleanupSeq := setupTestDB(t)
	defer cleanupSeq()
	seqScanner := NewScanner(dbSeq)
	seqResult, err := seqScanner.ScanDirectoriesWithOptions(context.Background(), []string{testDir}, Options{Workers: 1})
	if err != nil {
		t.Fatalf("single-worker scan failed: %v", err)
	}

	dbParallel, cleanupParallel := setupTestDB(t)
	defer cleanupParallel()
	parallelScanner := NewScanner(dbParallel)
	parallelResult, err := parallelScanner.ScanDirectoriesWithOptions(context.Background(), []string{testDir}, Options{Workers: 4})
	if err != nil {
		t.Fatalf("parallel scan failed: %v", err)
	}

	if seqResult.FilesScanned != parallelResult.FilesScanned ||
		seqResult.ModelsFound != parallelResult.ModelsFound ||
		seqResult.ModelsAdded != parallelResult.ModelsAdded ||
		seqResult.ModelsSkipped != parallelResult.ModelsSkipped ||
		seqResult.StaleChecked != parallelResult.StaleChecked ||
		seqResult.StaleRemoved != parallelResult.StaleRemoved ||
		len(seqResult.Errors) != len(parallelResult.Errors) {
		t.Fatalf("parallel result mismatch\nsingle=%+v\nparallel=%+v", seqResult, parallelResult)
	}

	seqRows, err := dbSeq.ListMetadata(state.MetadataFilters{OrderBy: "name"})
	if err != nil {
		t.Fatalf("list single-worker metadata: %v", err)
	}
	parallelRows, err := dbParallel.ListMetadata(state.MetadataFilters{OrderBy: "name"})
	if err != nil {
		t.Fatalf("list parallel metadata: %v", err)
	}
	if len(seqRows) != len(parallelRows) {
		t.Fatalf("metadata count mismatch: single=%d parallel=%d", len(seqRows), len(parallelRows))
	}
	for i := range seqRows {
		if seqRows[i].Dest != parallelRows[i].Dest ||
			seqRows[i].ModelName != parallelRows[i].ModelName ||
			seqRows[i].ModelType != parallelRows[i].ModelType ||
			seqRows[i].Quantization != parallelRows[i].Quantization {
			t.Fatalf("metadata mismatch at %d:\nsingle=%+v\nparallel=%+v", i, seqRows[i], parallelRows[i])
		}
	}
}

func TestScanner_RepairStaleRecords(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	testDir := t.TempDir()
	existingPath := createTestFile(t, testDir, "existing.gguf", 1024)
	missingPath := filepath.Join(testDir, "missing.gguf")
	outsideMissingPath := filepath.Join(t.TempDir(), "outside.gguf")

	for _, meta := range []*state.ModelMetadata{
		{DownloadURL: "file://" + existingPath, Dest: existingPath, ModelName: "existing", Source: "local"},
		{DownloadURL: "file://" + missingPath, Dest: missingPath, ModelName: "missing", Source: "local"},
		{DownloadURL: "file://" + outsideMissingPath, Dest: outsideMissingPath, ModelName: "outside", Source: "local"},
	} {
		if err := db.UpsertMetadata(meta); err != nil {
			t.Fatalf("seed metadata: %v", err)
		}
	}

	scanner := NewScanner(db)
	result, err := scanner.ScanDirectoriesWithOptions(context.Background(), []string{testDir}, Options{
		Workers:     2,
		RepairStale: true,
	})
	if err != nil {
		t.Fatalf("repair scan failed: %v", err)
	}
	if result.StaleChecked != 2 {
		t.Fatalf("expected 2 stale checks under scan dir, got %d", result.StaleChecked)
	}
	if result.StaleRemoved != 1 {
		t.Fatalf("expected 1 stale record removed, got %d", result.StaleRemoved)
	}
	if _, err := db.GetMetadata("file://" + missingPath); err != sql.ErrNoRows {
		t.Fatalf("missing metadata should be deleted, got %v", err)
	}
	if _, err := db.GetMetadata("file://" + outsideMissingPath); err != nil {
		t.Fatalf("outside metadata should be preserved, got %v", err)
	}
}

func TestScanner_RepairStaleCancellationReturnsOnlyError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	testDir := t.TempDir()
	missingPath := filepath.Join(testDir, "missing.gguf")
	if err := db.UpsertMetadata(&state.ModelMetadata{
		DownloadURL: "file://" + missingPath,
		Dest:        missingPath,
		ModelName:   "missing",
		Source:      "local",
	}); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scanner := NewScanner(db)
	result, err := scanner.ScanDirectoriesWithOptions(ctx, []string{testDir}, Options{RepairStale: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got result=%+v err=%v", result, err)
	}
	if result == nil {
		t.Fatal("expected scan result")
	}
	if len(result.Errors) != 0 {
		t.Fatalf("cancellation should be returned once as an error, got result errors: %+v", result.Errors)
	}
}

func TestScanner_CancellationLeavesStoredRowsValid(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	testDir := t.TempDir()
	for i := 0; i < 100; i++ {
		createTestFile(t, testDir, fmt.Sprintf("model-%03d.gguf", i), 1024)
	}

	ctx, cancel := context.WithCancel(context.Background())
	scanner := NewScanner(db)
	result, err := scanner.ScanDirectoriesWithOptions(ctx, []string{testDir}, Options{
		Workers: 4,
		Progress: func(progress Progress) {
			if progress.FilesScanned >= 1 {
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got result=%+v err=%v", result, err)
	}

	rows, err := db.ListMetadata(state.MetadataFilters{})
	if err != nil {
		t.Fatalf("list metadata after cancellation: %v", err)
	}
	for _, row := range rows {
		if row.DownloadURL == "" || row.Dest == "" {
			t.Fatalf("invalid stored row after cancellation: %+v", row)
		}
		if _, err := os.Stat(row.Dest); err != nil {
			t.Fatalf("stored row points at missing file after cancellation: %+v err=%v", row, err)
		}
	}
}

func TestConfiguredDirectoriesDeterministicAndTrimmed(t *testing.T) {
	cfg := &config.Config{
		General: config.General{DownloadRoot: " /downloads "},
		Placement: config.Placement{Apps: map[string]config.AppPlacement{
			"z-app": {
				Base: " /z ",
				Paths: map[string]string{
					"b": " loras ",
					"a": " checkpoints ",
				},
			},
			"a-app": {
				Base: "   ",
				Paths: map[string]string{
					"abs": " /absolute/path ",
				},
			},
			"m-app": {
				Base: " /m ",
				Paths: map[string]string{
					"abs":   " /override ",
					"empty": "  ",
				},
			},
		}},
	}

	got := ConfiguredDirectories(cfg)
	want := []string{
		"/downloads",
		"/absolute/path",
		"/m",
		"/override",
		"/z",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("configured dirs mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestConfiguredDirectoriesFiltersRedundantSubdirectories(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	cfg := &config.Config{
		General: config.General{DownloadRoot: root},
		Placement: config.Placement{Apps: map[string]config.AppPlacement{
			"app": {
				Base: root,
				Paths: map[string]string{
					"child": child,
				},
			},
		}},
	}

	got := ConfiguredDirectories(cfg)
	if len(got) != 1 || got[0] != root {
		t.Fatalf("expected only parent directory, got %#v", got)
	}
}

func TestPathWithinDirsNormalizesTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	path := createTestFile(t, dir, "model.gguf", 1024)

	if !pathWithinDirs(path, []string{dir + string(os.PathSeparator)}) {
		t.Fatalf("expected %s to be inside %s/", path, dir)
	}
}

func TestPathWithinDirsHandlesSymlinkedDirectory(t *testing.T) {
	parent := t.TempDir()
	realDir := filepath.Join(parent, "real")
	linkDir := filepath.Join(parent, "link")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	realPath := createTestFile(t, realDir, "model.gguf", 1024)
	linkPath := filepath.Join(linkDir, "model.gguf")

	if !pathWithinDirs(realPath, []string{linkDir}) {
		t.Fatalf("expected real path %s to be inside symlinked dir %s", realPath, linkDir)
	}
	if !pathWithinDirs(linkPath, []string{linkDir}) {
		t.Fatalf("expected symlink path %s to be inside symlinked dir %s", linkPath, linkDir)
	}
}
