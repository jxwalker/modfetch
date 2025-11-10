package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jxwalker/modfetch/internal/state"
)

// Benchmark tests for scanner performance validation
// These benchmarks verify the O(log n) performance claims

// BenchmarkScanDirectories_100Files benchmarks scanning 100 model files
func BenchmarkScanDirectories_100Files(b *testing.B) {
	benchmarkScanDirectories(b, 100)
}

// BenchmarkScanDirectories_1000Files benchmarks scanning 1,000 model files
func BenchmarkScanDirectories_1000Files(b *testing.B) {
	benchmarkScanDirectories(b, 1000)
}

// BenchmarkScanDirectories_10000Files benchmarks scanning 10,000 model files
func BenchmarkScanDirectories_10000Files(b *testing.B) {
	benchmarkScanDirectories(b, 10000)
}

// benchmarkScanDirectories is the common benchmark implementation
func benchmarkScanDirectories(b *testing.B, fileCount int) {
	// Setup: Create temporary database and test files
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	modelsDir := filepath.Join(tmpDir, "models")

	// Create test files
	createBenchmarkFiles(b, modelsDir, fileCount)

	// Initialize database
	db, err := state.NewDB(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.SQL.Close()

	scanner := NewScanner(db)

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		result, err := scanner.ScanDirectories([]string{modelsDir})
		if err != nil {
			b.Fatalf("Scan failed: %v", err)
		}

		// Verify scan worked
		if result.FilesScanned != fileCount {
			b.Errorf("Expected %d files scanned, got %d", fileCount, result.FilesScanned)
		}

		// On first iteration, models are added
		// On subsequent iterations, models are skipped (duplicate detection)
		if i == 0 {
			if result.ModelsAdded != fileCount {
				b.Errorf("First scan: expected %d added, got %d", fileCount, result.ModelsAdded)
			}
		} else {
			if result.ModelsSkipped != fileCount {
				b.Errorf("Subsequent scan: expected %d skipped, got %d", fileCount, result.ModelsSkipped)
			}
		}
	}
}

// BenchmarkDuplicateDetection_100Models benchmarks duplicate detection with 100 models
func BenchmarkDuplicateDetection_100Models(b *testing.B) {
	benchmarkDuplicateDetection(b, 100)
}

// BenchmarkDuplicateDetection_1000Models benchmarks duplicate detection with 1,000 models
func BenchmarkDuplicateDetection_1000Models(b *testing.B) {
	benchmarkDuplicateDetection(b, 1000)
}

// BenchmarkDuplicateDetection_10000Models benchmarks duplicate detection with 10,000 models
func BenchmarkDuplicateDetection_10000Models(b *testing.B) {
	benchmarkDuplicateDetection(b, 10000)
}

// benchmarkDuplicateDetection benchmarks the indexed duplicate detection query
func benchmarkDuplicateDetection(b *testing.B, modelCount int) {
	// Setup: Create database with existing models
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.SQL.Close()

	// Pre-populate database with models
	for i := 0; i < modelCount; i++ {
		meta := &state.ModelMetadata{
			DownloadURL: fmt.Sprintf("file:///models/model%d.gguf", i),
			Dest:        fmt.Sprintf("/models/model%d.gguf", i),
			ModelName:   fmt.Sprintf("model%d", i),
			Source:      "local",
		}
		if err := db.UpsertMetadata(meta); err != nil {
			b.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Test paths to query (mix of existing and non-existing)
	testPaths := []string{
		fmt.Sprintf("/models/model%d.gguf", modelCount/4),     // Exists
		fmt.Sprintf("/models/model%d.gguf", modelCount/2),     // Exists
		fmt.Sprintf("/models/model%d.gguf", modelCount*3/4),   // Exists
		fmt.Sprintf("/models/nonexistent%d.gguf", modelCount), // Doesn't exist
	}

	scanner := NewScanner(db)

	// Reset timer after setup
	b.ResetTimer()

	// Benchmark duplicate detection
	for i := 0; i < b.N; i++ {
		for _, path := range testPaths {
			_, _ = scanner.findExistingMetadata(path)
			// We don't care about the result, just measuring query speed
		}
	}
}

// BenchmarkFileTypeDetection benchmarks the file extension matching
func BenchmarkFileTypeDetection(b *testing.B) {
	testFiles := []string{
		"model.gguf",
		"model.safetensors",
		"model.ckpt",
		"README.md",
		"config.json",
		"model.pt",
		"model.bin",
		"very-long-filename-with-many-parts-and-metadata.Q4_K_M.gguf",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, file := range testFiles {
			_ = isModelFile(file)
		}
	}
}

// BenchmarkMetadataExtraction benchmarks the filename parsing and metadata extraction
func BenchmarkMetadataExtraction(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "llama-2-7b-chat.Q4_K_M.gguf")

	// Create test file
	f, err := os.Create(testFile)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}
	f.Close()

	info, err := os.Stat(testFile)
	if err != nil {
		b.Fatalf("Failed to stat file: %v", err)
	}

	// Create database
	dbPath := filepath.Join(tmpDir, "bench.db")
	db, err := state.NewDB(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.SQL.Close()

	scanner := NewScanner(db)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = scanner.extractMetadata(testFile, info)
	}
}

// BenchmarkScanWithProgress benchmarks scanning with progress callbacks
func BenchmarkScanWithProgress(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	modelsDir := filepath.Join(tmpDir, "models")

	// Create 100 test files
	createBenchmarkFiles(b, modelsDir, 100)

	db, err := state.NewDB(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.SQL.Close()

	scanner := NewScanner(db)

	// Progress callback that does minimal work
	progressFn := func(path string, found int) {
		// Minimal callback to measure overhead
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := scanner.ScanWithProgress([]string{modelsDir}, progressFn)
		if err != nil {
			b.Fatalf("Scan failed: %v", err)
		}
	}
}

// BenchmarkInferModelType benchmarks model type inference
func BenchmarkInferModelType(b *testing.B) {
	testFiles := []string{
		"model.gguf",
		"text-lora.safetensors",
		"sdxl-lora-v2.safetensors",
		"vae-ft-mse.ckpt",
		"embedding-vectors.pt",
		"controlnet-canny.safetensors",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, file := range testFiles {
			_ = inferModelTypeFromPath(file)
		}
	}
}

// BenchmarkExtractNameAndVersion benchmarks name and version extraction
func BenchmarkExtractNameAndVersion(b *testing.B) {
	testFilenames := []string{
		"llama-2-7b",
		"mistral-v1.0-Q4_K_M",
		"sdxl-base-1.0-fp16",
		"model-name-with-many-parts-v2.5-7B-Q5_K_S",
	}

	meta := &state.ModelMetadata{}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, filename := range testFilenames {
			extractNameAndVersion(filename, meta)
		}
	}
}

// BenchmarkDatabaseUpsert benchmarks database insertion performance
func BenchmarkDatabaseUpsert(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.SQL.Close()

	// Pre-create metadata objects
	metas := make([]*state.ModelMetadata, b.N)
	for i := 0; i < b.N; i++ {
		metas[i] = &state.ModelMetadata{
			DownloadURL: fmt.Sprintf("file:///models/model%d.gguf", i),
			Dest:        fmt.Sprintf("/models/model%d.gguf", i),
			ModelName:   fmt.Sprintf("model%d", i),
			Source:      "local",
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := db.UpsertMetadata(metas[i]); err != nil {
			b.Fatalf("Upsert failed: %v", err)
		}
	}
}

// BenchmarkDatabaseQuery benchmarks database query performance with indexes
func BenchmarkDatabaseQuery(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	db, err := state.NewDB(dbPath)
	if err != nil {
		b.Fatalf("Failed to create database: %v", err)
	}
	defer db.SQL.Close()

	// Pre-populate with 10,000 models
	for i := 0; i < 10000; i++ {
		meta := &state.ModelMetadata{
			DownloadURL: fmt.Sprintf("file:///models/model%d.gguf", i),
			Dest:        fmt.Sprintf("/models/model%d.gguf", i),
			ModelName:   fmt.Sprintf("model%d", i),
			Source:      "local",
		}
		if err := db.UpsertMetadata(meta); err != nil {
			b.Fatalf("Setup failed: %v", err)
		}
	}

	// Query paths (mix of existing and non-existing)
	queryPaths := []string{
		"/models/model0.gguf",
		"/models/model5000.gguf",
		"/models/model9999.gguf",
		"/models/nonexistent.gguf",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, path := range queryPaths {
			_, _ = db.GetMetadataByDest(path)
		}
	}
}

// Helper: Create benchmark test files
func createBenchmarkFiles(b *testing.B, dir string, count int) {
	b.Helper()

	if err := os.MkdirAll(dir, 0755); err != nil {
		b.Fatalf("Failed to create directory: %v", err)
	}

	// Create files with various names and extensions
	extensions := []string{".gguf", ".safetensors", ".ckpt"}
	quantizations := []string{"Q4_K_M", "Q5_K_S", "Q8_0", "FP16"}
	models := []string{"llama", "mistral", "sdxl", "gpt"}

	for i := 0; i < count; i++ {
		ext := extensions[i%len(extensions)]
		quant := quantizations[i%len(quantizations)]
		model := models[i%len(models)]

		filename := fmt.Sprintf("%s-%d.%s%s", model, i, quant, ext)
		path := filepath.Join(dir, filename)

		f, err := os.Create(path)
		if err != nil {
			b.Fatalf("Failed to create file: %v", err)
		}
		f.Close()
	}
}

// BenchmarkCompleteWorkflow benchmarks the complete scan workflow
func BenchmarkCompleteWorkflow(b *testing.B) {
	// This benchmark measures the end-to-end performance:
	// 1. Walk directory
	// 2. Check file types
	// 3. Query database for duplicates
	// 4. Extract metadata
	// 5. Store in database

	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")
	modelsDir := filepath.Join(tmpDir, "models")

	// Create 1000 test files
	createBenchmarkFiles(b, modelsDir, 1000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create fresh database for each iteration
		iterDBPath := filepath.Join(tmpDir, fmt.Sprintf("bench%d.db", i))
		db, err := state.NewDB(iterDBPath)
		if err != nil {
			b.Fatalf("Failed to create database: %v", err)
		}

		scanner := NewScanner(db)
		result, err := scanner.ScanDirectories([]string{modelsDir})
		if err != nil {
			b.Fatalf("Scan failed: %v", err)
		}

		if result.FilesScanned != 1000 {
			b.Errorf("Expected 1000 files scanned, got %d", result.FilesScanned)
		}

		db.SQL.Close()
		os.Remove(iterDBPath)
	}
}
