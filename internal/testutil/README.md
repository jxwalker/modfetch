# Testing Infrastructure

This package provides testing utilities for ModFetch that work without network connectivity.

## Overview

The testutil package provides:
- Mock HTTP servers with canned responses
- In-memory SQLite database for testing
- Test fixture loading helpers
- Mock HTTP round trippers
- Temporary file/directory helpers

## Usage

### Creating a Test Database

```go
func TestSomething(t *testing.T) {
    db := testutil.TestDB(t)
    // Use db for testing
    // Automatically cleaned up when test completes
}
```

### Mock HTTP Server

```go
func TestHTTPFetcher(t *testing.T) {
    server := testutil.NewMockHTTPServer()
    defer server.Close()

    // Add canned responses
    server.AddJSONResponse("/api/models/123", 200, `{"id": 123, "name": "test"}`)

    // Use server.URL in your tests
    client := &http.Client{}
    resp, err := client.Get(server.URL + "/api/models/123")
    // ... assertions
}
```

### Loading Fixtures

```go
func TestWithFixture(t *testing.T) {
    json := testutil.LoadFixture(t, "huggingface_model.json")
    // Use fixture data in tests
}
```

### Mock Round Tripper

```go
func TestHTTPClient(t *testing.T) {
    mock := testutil.NewMockRoundTripper()
    mock.AddStringResponse("https://api.example.com/model", 200, `{"result": "ok"}`)

    client := &http.Client{Transport: mock}
    // Use client

    // Verify requests
    mock.AssertRequestMade(t, "https://api.example.com/model")
}
```

## Test Fixtures

Fixtures are stored in `testdata/fixtures/` and include:

- `huggingface_model.json` - Sample HuggingFace API response
- `civitai_model.json` - Sample CivitAI API response

Add new fixtures as needed for additional test cases.

## Running Tests

The test infrastructure is designed to work without network connectivity. All external API calls are mocked with canned responses.

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/metadata/...

# Run with verbose output
go test -v ./internal/state/...

# Run specific test
go test -run TestDB_UpsertMetadata ./internal/state/...
```

## Best Practices

1. **Always use testutil.TestDB()** for database tests - it creates an in-memory database that's automatically cleaned up
2. **Use fixtures for API responses** - keeps tests maintainable and realistic
3. **Mock HTTP transports** - don't make real network calls in tests
4. **Use t.Helper()** in utility functions - improves error reporting
5. **Clean up resources** - use t.Cleanup() for deferred cleanup

## Coverage

Current test coverage includes:

### Metadata Package
- ✅ HuggingFace fetcher with mocked API responses
- ✅ CivitAI fetcher with mocked API responses
- ✅ Model type inference from filenames and tags
- ✅ Quantization extraction from filenames
- ✅ URL pattern matching
- ✅ Error handling for API failures

### State Package
- ✅ Metadata CRUD operations (create, read, update, delete)
- ✅ Filtering by source, type, rating, tags, favorites
- ✅ Full-text search across metadata fields
- ✅ Usage tracking (times_used, last_used)
- ✅ Tag JSON serialization/deserialization
- ✅ Timestamp handling

## Future Test Coverage Needed

- [ ] Batch package tests
- [ ] Downloader package tests with mock HTTP servers
- [ ] TUI model tests (unit tests for state transitions)
- [ ] Integration tests for full download flow
- [ ] Resolver package tests
- [ ] Config loading and validation tests
- [ ] Metrics package tests
