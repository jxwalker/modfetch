package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jxwalker/modfetch/internal/state"
)

// MaxAPIRetries is the default number of retries for flaky external API calls
const MaxAPIRetries = 3

// TestDB creates an in-memory SQLite database for testing
func TestDB(t *testing.T) *state.DB {
	t.Helper()

	// Create in-memory database
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	db := &state.DB{SQL: sqlDB}

	// Initialize all tables
	if err := initTestSchema(db); err != nil {
		t.Fatalf("failed to initialize test schema: %v", err)
	}

	// Clean up when test completes
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	return db
}

// initTestSchema initializes all tables in the test database
func initTestSchema(db *state.DB) error {
	// Initialize downloads table
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS downloads (
			url TEXT PRIMARY KEY,
			dest TEXT NOT NULL,
			expected_sha256 TEXT,
			actual_sha256 TEXT,
			etag TEXT,
			last_modified TEXT,
			size INTEGER,
			status TEXT,
			retries INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_error TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status)`,
	}

	for _, stmt := range stmts {
		if _, err := db.SQL.Exec(stmt); err != nil {
			return fmt.Errorf("init downloads table: %w", err)
		}
	}

	// Initialize chunks table
	if err := db.InitChunksTable(); err != nil {
		return fmt.Errorf("init chunks table: %w", err)
	}

	// Initialize host caps table
	if err := db.InitHostCapsTable(); err != nil {
		return fmt.Errorf("init host caps table: %w", err)
	}

	// Initialize metadata table
	if err := db.InitMetadataTable(); err != nil {
		return fmt.Errorf("init metadata table: %w", err)
	}

	return nil
}

// LoadFixture loads a test fixture file
func LoadFixture(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}

	return string(data)
}

// TryLoadFixture attempts to load a fixture, returns empty string if not found
func TryLoadFixture(name string) string {
	path := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// TempDir creates a temporary directory for testing
func TempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "modfetch-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	return dir
}

// TempFile creates a temporary file with content
func TempFile(t *testing.T, name, content string) string {
	t.Helper()

	dir := TempDir(t)
	path := filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	return path
}

// StringPtr returns a pointer to a string (useful for optional fields)
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to an int
func IntPtr(i int) *int {
	return &i
}

// Int64Ptr returns a pointer to an int64
func Int64Ptr(i int64) *int64 {
	return &i
}

// RetryOnAPIError retries an operation when it encounters API-related errors
// This is useful for flaky external API integration tests (e.g., CivitAI, HuggingFace)
// Uses exponential backoff between retries (1s, 2s, 4s)
// Returns error if all retries fail with non-API errors, or skips test if API is unavailable
func RetryOnAPIError(t *testing.T, maxRetries int, operation func() error, operationName string) {
	t.Helper()

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = operation()

		if lastErr == nil {
			// Success!
			return
		}

		// Check if error is API-related (503, 400, Service Unavailable, Bad Request)
		errMsg := lastErr.Error()
		isAPIError := strings.Contains(errMsg, "503") ||
			strings.Contains(errMsg, "400") ||
			strings.Contains(errMsg, "Service Unavailable") ||
			strings.Contains(errMsg, "Bad Request")

		if isAPIError && attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s
			backoffDuration := time.Duration(1<<uint(attempt-1)) * time.Second
			t.Logf("%s attempt %d/%d failed with API error: %v (retrying in %v...)",
				operationName, attempt, maxRetries, lastErr, backoffDuration)
			time.Sleep(backoffDuration)
			continue
		}

		// Either non-API error or last attempt
		break
	}

	// If all retries failed with API errors, skip the test
	if lastErr != nil {
		errMsg := lastErr.Error()
		isAPIError := strings.Contains(errMsg, "503") ||
			strings.Contains(errMsg, "400") ||
			strings.Contains(errMsg, "Service Unavailable") ||
			strings.Contains(errMsg, "Bad Request")

		if isAPIError {
			t.Skipf("External API unavailable after %d attempts: %v", maxRetries, lastErr)
		}
		t.Fatalf("%s failed: %v", operationName, lastErr)
	}
}
