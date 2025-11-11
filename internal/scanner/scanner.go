package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jxwalker/modfetch/internal/metadata"
	"github.com/jxwalker/modfetch/internal/state"
)

// Scanner scans directories for model files and populates metadata
type Scanner struct {
	db *state.DB
}

// NewScanner creates a new directory scanner
func NewScanner(db *state.DB) *Scanner {
	return &Scanner{db: db}
}

// ScanResult contains information about a scan operation
type ScanResult struct {
	FilesScanned  int
	ModelsFound   int
	ModelsAdded   int
	ModelsSkipped int
	Errors        []error
}

// ModelFileExtensions are file extensions we recognize as model files
var ModelFileExtensions = []string{
	".gguf",
	".ggml",
	".safetensors",
	".ckpt",
	".pt",
	".pth",
	".bin",
	".h5",
	".pb",
	".onnx",
}

// ScanDirectories scans multiple directories for model files
func (s *Scanner) ScanDirectories(dirs []string) (*ScanResult, error) {
	result := &ScanResult{
		Errors: []error{},
	}

	for _, dir := range dirs {
		if err := s.scanDirectory(dir, result); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("scanning %s: %w", dir, err))
		}
	}

	return result, nil
}

// scanDirectory recursively scans a single directory
func (s *Scanner) scanDirectory(dir string, result *ScanResult) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access due to permissions
			if os.IsPermission(err) {
				return filepath.SkipDir
			}
			// Return other errors (like non-existent paths) so they're captured
			if os.IsNotExist(err) {
				return err
			}
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if this is a model file
		if !isModelFile(path) {
			return nil
		}

		result.FilesScanned++

		// Check if we already have metadata for this file
		existing, err := s.findExistingMetadata(path)
		if err == nil && existing != nil {
			result.ModelsSkipped++
			return nil
		}

		// Extract metadata from file
		meta := s.extractMetadata(path, info)

		// Try to enrich metadata if we can determine the source
		// (This would require the file to have source URL in extended attributes or similar)

		result.ModelsFound++

		// Store in database
		if err := s.db.UpsertMetadata(meta); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("storing metadata for %s: %w", path, err))
			return nil
		}

		result.ModelsAdded++
		return nil
	})
}

// isModelFile checks if a file is a recognized model file
func isModelFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, modelExt := range ModelFileExtensions {
		if ext == modelExt {
			return true
		}
	}
	return false
}

// findExistingMetadata checks if we already have metadata for this file
// This uses an indexed query for O(log n) performance instead of O(n)
func (s *Scanner) findExistingMetadata(path string) (*state.ModelMetadata, error) {
	// Use direct indexed query by dest path
	meta, err := s.db.GetMetadataByDest(path)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, fmt.Errorf("not found")
	}
	return meta, nil
}

// extractMetadata extracts metadata from file path and name
func (s *Scanner) extractMetadata(path string, info os.FileInfo) *state.ModelMetadata {
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	meta := &state.ModelMetadata{
		DownloadURL: "file://" + path, // Use file:// URL for local files
		Dest:        path,
		ModelName:   nameWithoutExt,
		Source:      "local",
		FileSize:    info.Size(),
		FileFormat:  ext,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Infer model type from filename and extension
	meta.ModelType = inferModelTypeFromPath(filename)

	// Extract quantization from filename
	meta.Quantization = metadata.ExtractQuantization(filename)

	// Try to extract model name and version
	extractNameAndVersion(nameWithoutExt, meta)

	return meta
}

// inferModelTypeFromPath infers model type from file path/name
func inferModelTypeFromPath(filename string) string {
	lower := strings.ToLower(filename)

	// Check extension first
	if strings.HasSuffix(lower, ".gguf") || strings.HasSuffix(lower, ".ggml") {
		return "LLM"
	}

	// Check filename patterns
	if strings.Contains(lower, "lora") {
		return "LoRA"
	}
	if strings.Contains(lower, "vae") {
		return "VAE"
	}
	if strings.Contains(lower, "embedding") || strings.Contains(lower, "textual") {
		return "Embedding"
	}
	if strings.Contains(lower, "controlnet") {
		return "ControlNet"
	}
	if strings.HasSuffix(lower, ".safetensors") || strings.HasSuffix(lower, ".ckpt") {
		// Could be checkpoint, LoRA, or embedding - check for indicators
		if strings.Contains(lower, "checkpoint") || strings.Contains(lower, "model") {
			return "Checkpoint"
		}
		// Default safetensors to Checkpoint
		return "Checkpoint"
	}

	return "Unknown"
}

// extractNameAndVersion tries to extract model name and version from filename
func extractNameAndVersion(filename string, meta *state.ModelMetadata) {
	// Common patterns:
	// model-name-v1.0-Q4_K_M
	// model_name_fp16
	// ModelName-7B-GGUF

	// First extract version using regex to capture full version strings like v1.0, v1.5.1
	versionRegex := regexp.MustCompile(`(?i)[_-]?(v\d+(?:\.\d+)*)[_-]?`)
	if matches := versionRegex.FindStringSubmatch(filename); len(matches) > 1 {
		meta.Version = matches[1]
	}

	// Split by delimiters for other metadata
	parts := strings.FieldsFunc(filename, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})

	if len(parts) == 0 {
		return
	}

	// First part is usually the model name
	meta.ModelName = parts[0]

	// Look for other metadata in parts
	for _, part := range parts {
		lower := strings.ToLower(part)

		// Parameter count patterns (7B, 13B, 70B, etc.)
		if strings.HasSuffix(lower, "b") && len(part) <= 4 {
			meta.ParameterCount = strings.ToUpper(part)
		}

		// Precision indicators
		if lower == "fp16" || lower == "fp32" || lower == "fp8" {
			meta.Quantization = strings.ToUpper(part)
		}
	}
}

// ScanWithProgress scans directories and calls progress callback
func (s *Scanner) ScanWithProgress(dirs []string, progressFn func(path string, found int)) (*ScanResult, error) {
	result := &ScanResult{
		Errors: []error{},
	}

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return filepath.SkipDir
				}
				return nil
			}

			if info.IsDir() {
				return nil
			}

			if !isModelFile(path) {
				return nil
			}

			result.FilesScanned++

			// Report progress
			if progressFn != nil {
				progressFn(path, result.ModelsFound)
			}

			// Check if exists
			existing, err := s.findExistingMetadata(path)
			if err == nil && existing != nil {
				result.ModelsSkipped++
				return nil
			}

			// Extract and store metadata
			meta := s.extractMetadata(path, info)
			result.ModelsFound++

			if err := s.db.UpsertMetadata(meta); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("storing %s: %w", path, err))
				return nil
			}

			result.ModelsAdded++
			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("walking %s: %w", dir, err))
		}
	}

	return result, nil
}
