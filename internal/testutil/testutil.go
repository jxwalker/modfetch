package testutil

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jxwalker/modfetch/internal/state"
)

// MockHTTPServer creates a test HTTP server that serves canned responses from fixtures
type MockHTTPServer struct {
	*httptest.Server
	Responses map[string]MockResponse
}

// MockResponse represents a canned HTTP response
type MockResponse struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

// NewMockHTTPServer creates a new mock HTTP server
func NewMockHTTPServer() *MockHTTPServer {
	ms := &MockHTTPServer{
		Responses: make(map[string]MockResponse),
	}

	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Look up response by path
		key := r.URL.Path
		if r.URL.RawQuery != "" {
			key += "?" + r.URL.RawQuery
		}

		resp, ok := ms.Responses[key]
		if !ok {
			// Try without query parameters
			resp, ok = ms.Responses[r.URL.Path]
		}

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, "No mock response configured for %s", key)
			return
		}

		// Set headers
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}

		w.WriteHeader(resp.StatusCode)
		_, _ = fmt.Fprint(w, resp.Body)
	}))

	return ms
}

// AddResponse adds a canned response for a specific path
func (ms *MockHTTPServer) AddResponse(path string, response MockResponse) {
	ms.Responses[path] = response
}

// AddJSONResponse adds a JSON response for a specific path
func (ms *MockHTTPServer) AddJSONResponse(path string, statusCode int, body string) {
	ms.Responses[path] = MockResponse{
		StatusCode: statusCode,
		Body:       body,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

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

// MockRoundTripper implements http.RoundTripper for testing
type MockRoundTripper struct {
	Responses map[string]*http.Response
	Requests  []*http.Request
}

// RoundTrip implements http.RoundTripper
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.Requests = append(m.Requests, req)

	key := req.URL.String()
	resp, ok := m.Responses[key]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    req,
		}, nil
	}

	return resp, nil
}

// NewMockRoundTripper creates a new mock round tripper
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		Responses: make(map[string]*http.Response),
		Requests:  make([]*http.Request, 0),
	}
}

// AddStringResponse adds a simple string response
func (m *MockRoundTripper) AddStringResponse(url string, statusCode int, body string) {
	m.Responses[url] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// AssertRequestMade checks if a request was made to a specific URL
func (m *MockRoundTripper) AssertRequestMade(t *testing.T, url string) {
	t.Helper()

	for _, req := range m.Requests {
		if req.URL.String() == url {
			return
		}
	}

	t.Errorf("expected request to %s, but none was made", url)
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
