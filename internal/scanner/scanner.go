package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
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
	StaleChecked  int
	StaleRemoved  int
	Errors        []error
}

// Progress describes the latest scanner state for CLI and TUI callers.
type Progress struct {
	Phase         string
	Path          string
	FilesScanned  int
	ModelsFound   int
	ModelsAdded   int
	ModelsSkipped int
	StaleChecked  int
	StaleRemoved  int
	Errors        int
}

// ProgressFunc receives progress snapshots during a scan.
type ProgressFunc func(Progress)

// Options controls scanner behavior.
type Options struct {
	Workers     int
	Progress    ProgressFunc
	RepairStale bool
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
	return s.ScanDirectoriesWithOptions(context.Background(), dirs, Options{})
}

// ScanDirectoriesWithOptions scans multiple directories with bounded worker
// concurrency while keeping database operations serialized.
func (s *Scanner) ScanDirectoriesWithOptions(ctx context.Context, dirs []string, opts Options) (*ScanResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dirs = normalizeScanDirs(dirs)
	result := &ScanResult{Errors: []error{}}
	if opts.RepairStale {
		s.repairStaleRecords(ctx, dirs, result, opts.Progress)
		if err := ctx.Err(); err != nil {
			return result, err
		}
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = defaultWorkerCount()
	}

	jobs := make(chan scanJob)
	candidates := make(chan scanCandidate)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				meta := s.extractMetadata(job.path, job.info)
				select {
				case candidates <- scanCandidate{path: job.path, meta: meta}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for candidate := range candidates {
			s.storeCandidate(candidate, result, opts.Progress)
		}
	}()

	var walkErrors []error
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			break
		}
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					if entry == nil {
						return err
					}
					if entry != nil && entry.IsDir() {
						return filepath.SkipDir
					}
					walkErrors = append(walkErrors, fmt.Errorf("permission denied %s: %w", path, err))
					return nil
				}
				if os.IsNotExist(err) {
					if path == dir {
						return err
					}
					walkErrors = append(walkErrors, fmt.Errorf("path disappeared %s: %w", path, err))
					return nil
				}
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() || !isModelFile(path) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				walkErrors = append(walkErrors, fmt.Errorf("stat %s: %w", path, err))
				return nil
			}
			select {
			case jobs <- scanJob{path: path, info: info}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil && err != context.Canceled {
			walkErrors = append(walkErrors, fmt.Errorf("scanning %s: %w", dir, err))
		}
	}

	close(jobs)
	wg.Wait()
	close(candidates)
	<-writerDone
	result.Errors = append(result.Errors, walkErrors...)
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
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

type scanJob struct {
	path string
	info os.FileInfo
}

type scanCandidate struct {
	path string
	meta *state.ModelMetadata
}

func defaultWorkerCount() int {
	workers := runtime.NumCPU()
	if workers < 1 {
		return 1
	}
	if workers > 8 {
		return 8
	}
	return workers
}

func normalizeScanDirs(dirs []string) []string {
	normalized := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if absDir, err := filepath.Abs(dir); err == nil {
			dir = absDir
		}
		normalized = append(normalized, filepath.Clean(dir))
	}
	return normalized
}

func (s *Scanner) storeCandidate(candidate scanCandidate, result *ScanResult, progress ProgressFunc) {
	result.FilesScanned++
	result.ModelsFound++

	existing, err := s.findExistingMetadata(candidate.path)
	if err == nil && existing != nil {
		result.ModelsSkipped++
		reportProgress(progress, "scan", candidate.path, result)
		return
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		result.Errors = append(result.Errors, fmt.Errorf("checking existing metadata for %s: %w", candidate.path, err))
		reportProgress(progress, "scan", candidate.path, result)
		return
	}

	if err := s.db.UpsertMetadata(candidate.meta); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("storing metadata for %s: %w", candidate.path, err))
		reportProgress(progress, "scan", candidate.path, result)
		return
	}

	result.ModelsAdded++
	reportProgress(progress, "scan", candidate.path, result)
}

func (s *Scanner) repairStaleRecords(ctx context.Context, dirs []string, result *ScanResult, progress ProgressFunc) {
	rows, err := s.db.ListMetadata(state.MetadataFilters{})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("listing metadata for stale repair: %w", err))
		reportProgress(progress, "repair", "", result)
		return
	}
	dirVariants := pathVariantList(dirs)
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return
		}
		if row.Dest == "" || !isModelFile(row.Dest) || !pathWithinDirVariants(row.Dest, dirVariants) {
			continue
		}
		result.StaleChecked++
		if _, err := os.Stat(row.Dest); err == nil {
			reportProgress(progress, "repair", row.Dest, result)
			continue
		} else if !os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Errorf("checking stale metadata %s: %w", row.Dest, err))
			reportProgress(progress, "repair", row.Dest, result)
			continue
		}
		if err := s.db.DeleteMetadata(row.DownloadURL); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("removing stale metadata %s: %w", row.DownloadURL, err))
			reportProgress(progress, "repair", row.Dest, result)
			continue
		}
		result.StaleRemoved++
		reportProgress(progress, "repair", row.Dest, result)
	}
}

func pathWithinDirs(path string, dirs []string) bool {
	return pathWithinDirVariants(path, pathVariantList(dirs))
}

func pathWithinDirVariants(path string, dirVariants [][]string) bool {
	candidatePaths := pathVariants(path)
	if len(candidatePaths) == 0 {
		return false
	}
	for _, dirVariantGroup := range dirVariants {
		for _, dirVariant := range dirVariantGroup {
			for _, pathVariant := range candidatePaths {
				rel, err := filepath.Rel(dirVariant, pathVariant)
				if err != nil {
					continue
				}
				if rel == "." || (rel != "" && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
					return true
				}
			}
		}
	}
	return false
}

func pathVariantList(paths []string) [][]string {
	var variants [][]string
	for _, path := range paths {
		pathVariant := pathVariants(path)
		if len(pathVariant) > 0 {
			variants = append(variants, pathVariant)
		}
	}
	return variants
}

func pathVariants(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	cleanPath := filepath.Clean(absPath)
	variants := []string{cleanPath}
	if evalPath, err := filepath.EvalSymlinks(cleanPath); err == nil {
		evalPath = filepath.Clean(evalPath)
		if evalPath != cleanPath {
			variants = append(variants, evalPath)
		}
	}
	return variants
}

func reportProgress(progress ProgressFunc, phase, path string, result *ScanResult) {
	if progress == nil {
		return
	}
	progress(Progress{
		Phase:         phase,
		Path:          path,
		FilesScanned:  result.FilesScanned,
		ModelsFound:   result.ModelsFound,
		ModelsAdded:   result.ModelsAdded,
		ModelsSkipped: result.ModelsSkipped,
		StaleChecked:  result.StaleChecked,
		StaleRemoved:  result.StaleRemoved,
		Errors:        len(result.Errors),
	})
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
		return nil, sql.ErrNoRows
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
	return s.ScanDirectoriesWithOptions(context.Background(), dirs, Options{
		Progress: func(progress Progress) {
			if progressFn != nil && progress.Path != "" && progress.Phase == "scan" {
				progressFn(progress.Path, progress.ModelsFound)
			}
		},
	})
}

// ScanWithProgressDetail scans directories and reports detailed progress.
func (s *Scanner) ScanWithProgressDetail(dirs []string, opts Options) (*ScanResult, error) {
	return s.ScanDirectoriesWithOptions(context.Background(), dirs, opts)
}

// ScanWithContext scans directories with cancellation support.
func (s *Scanner) ScanWithContext(ctx context.Context, dirs []string, opts Options) (*ScanResult, error) {
	return s.ScanDirectoriesWithOptions(ctx, dirs, opts)
}
