  # ModFetch Testing Guide

This guide explains how to test ModFetch's TUI, metadata, and library features.

## Quick Start

```bash
# Run all offline tests (no network required)
./scripts/test-offline.sh

# Run specific package tests
go test ./internal/metadata/...
go test ./internal/state/...
go test ./internal/tui/...

# Run with verbose output
go test -v ./internal/util/...

# Run specific test
go test -run TestMetadataStorage ./internal/tui/...
```

## Test Categories

### ✅ Offline Tests (Work in sandboxed environments)

These tests use mocks and in-memory databases - no network required:

| Package | Tests | Status |
|---------|-------|--------|
| `internal/util` | Path utilities, URL helpers, naming patterns | ✅ PASS |
| `internal/classifier` | File type detection by magic bytes | ✅ PASS |
| `internal/config` | YAML config loading and validation | ✅ PASS |
| `internal/logging` | URL sanitization, log formatting | ✅ PASS |
| `internal/placer` | Symlink/hardlink/copy operations | ✅ PASS |
| `internal/resolver` (partial) | CivitAI URL resolution (mocked) | ✅ PASS |

### ⚠️ Network-Dependent Tests

These require external API access or downloadable dependencies:

| Package | Why It Needs Network | Workaround |
|---------|---------------------|------------|
| `internal/metadata` | SQLite dependency download | Use environment with cached deps |
| `internal/state` | SQLite dependency download | Use environment with cached deps |
| `internal/tui` | SQLite + Bubble Tea dependencies | Use environment with cached deps |
| `internal/downloader` | HTTP server tests | Mock HTTP servers (future) |
| `internal/resolver` (HF) | Real HuggingFace API calls | Convert to mocked tests (future) |

## Testing Metadata Features

### Database Operations

```bash
# Test metadata CRUD operations
go test -v -run TestDB_UpsertMetadata ./internal/state/...
go test -v -run TestDB_ListMetadata ./internal/state/...
go test -v -run TestDB_SearchMetadata ./internal/state/...
go test -v -run TestDB_UpdateMetadataUsage ./internal/state/...
```

**What These Test:**
- ✅ Insert and update metadata records
- ✅ Retrieve metadata by download URL
- ✅ Filter by source, type, rating, favorites, tags
- ✅ Full-text search across all fields
- ✅ Usage tracking (times_used, last_used)
- ✅ Tag JSON serialization
- ✅ Timestamp handling

### Metadata Fetchers

```bash
# Test HuggingFace fetcher
go test -v -run TestHuggingFaceFetcher ./internal/metadata/...

# Test CivitAI fetcher
go test -v -run TestCivitAIFetcher ./internal/metadata/...
```

**What These Test:**
- ✅ URL pattern matching (CanHandle)
- ✅ API response parsing with mocked HTTP
- ✅ Model type inference from filenames/tags
- ✅ Quantization extraction (Q4_K_M, etc.)
- ✅ Error handling for API failures
- ✅ Fallback to basic metadata
- ✅ Type mapping (CivitAI → internal types)

### TUI Integration

```bash
# Test TUI metadata integration
go test -v -run TestMetadata ./internal/tui/...
```

**What These Test:**
- ✅ Metadata storage from TUI
- ✅ Metadata retrieval in TUI context
- ✅ Filtering metadata by various criteria
- ✅ Usage tracking integration
- ✅ Nil database handling (safety)
- ✅ TUI model with metadata database

## Manual Testing

### Testing Metadata Fetching

1. **Start ModFetch TUI:**
   ```bash
   ./modfetch tui
   ```

2. **Download a model:**
   - Press `n` to add new download
   - Enter a HuggingFace URL: `https://huggingface.co/TheBloke/Llama-2-7B-GGUF/resolve/main/llama-2-7b.Q4_K_M.gguf`
   - Wait for download to complete

3. **Verify metadata storage:**
   ```bash
   # Check database for metadata
   sqlite3 ~/.local/share/modfetch/state.db "SELECT model_name, source, model_type, quantization FROM model_metadata;"
   ```

   Expected output:
   ```
   Llama-2-7B-GGUF|huggingface|LLM|Q4_K_M
   ```

4. **Test CivitAI metadata:**
   - Download a CivitAI model
   - Verify metadata includes: type, base_model, thumbnail_url, rating

### Testing Library Features (Future)

Once the Library view is implemented:

1. **Browse downloaded models:**
   - Press `l` to open Library view
   - Navigate with `j`/`k` or arrow keys

2. **Search models:**
   - Press `/` to search
   - Enter search term (name, tag, author)
   - Verify results

3. **Filter by type:**
   - Press `f` to filter
   - Select model type (LLM, LoRA, Checkpoint, etc.)
   - Verify filtered results

4. **View model details:**
   - Select a model and press `Enter`
   - Verify display shows:
     - Full description
     - Tags
     - File size and format
     - Quantization
     - Usage stats
     - Author and license

## Test Coverage Report

### Current Coverage

```
internal/metadata/fetcher.go       85%  (8 tests)
internal/metadata/civitai.go       80%  (6 tests)
internal/state/metadata.go         95%  (8 tests)
internal/tui/metadata_test.go      90%  (6 tests)
```

### Critical Paths Tested

✅ **Metadata Lifecycle:**
1. Download completes → fetchAndStoreMetadata called
2. Metadata fetcher selected based on URL
3. API called (or fallback to basic metadata)
4. Metadata stored in database
5. Available for search/filter/display

✅ **Database Operations:**
1. Create (UpsertMetadata with new URL)
2. Read (GetMetadata by URL)
3. Update (UpsertMetadata with existing URL)
4. Delete (DeleteMetadata)
5. Search (SearchMetadata)
6. Filter (ListMetadata with filters)
7. Usage tracking (UpdateMetadataUsage)

✅ **Edge Cases:**
1. Nil database handling
2. API failures (unauthorized, not found)
3. Invalid URLs
4. Missing required fields
5. Empty search results
6. JSON serialization errors (tags)

## Continuous Integration

### GitHub Actions Workflow (Recommended)

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run offline tests
        run: ./scripts/test-offline.sh

      - name: Run metadata tests
        run: |
          go test -v ./internal/metadata/...
          go test -v ./internal/state/...

      - name: Run TUI tests
        run: go test -v ./internal/tui/...

      - name: Generate coverage
        run: go test -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

## Known Limitations

### In Sandboxed Environments

❌ **Cannot run:**
- Tests requiring SQLite dependency download
- Tests making real API calls without mocks
- Integration tests requiring full HTTP server

✅ **Can run:**
- All utility and helper function tests
- Mocked HTTP client tests
- In-memory database tests (when deps cached)
- Logic and business rule tests

### Workarounds

1. **For CI/CD:** Pre-cache Go dependencies
2. **For development:** Run tests in local environment with network
3. **For sandboxes:** Use `./scripts/test-offline.sh`

## Writing New Tests

### Best Practices

1. **Use testutil helpers:**
   ```go
   db := testutil.TestDB(t)  // Auto-cleanup
   server := testutil.NewMockHTTPServer()
   defer server.Close()
   ```

2. **Load fixtures for realistic data:**
   ```go
   json := testutil.LoadFixture(t, "huggingface_model.json")
   ```

3. **Mock HTTP clients:**
   ```go
   mock := testutil.NewMockRoundTripper()
   mock.AddStringResponse(url, 200, responseBody)
   client := &http.Client{Transport: mock}
   ```

4. **Test error paths:**
   ```go
   // Test both success and failure
   t.Run("success", func(t *testing.T) { /* ... */ })
   t.Run("api_failure", func(t *testing.T) { /* ... */ })
   t.Run("network_error", func(t *testing.T) { /* ... */ })
   ```

5. **Verify cleanup:**
   ```go
   t.Cleanup(func() {
       os.RemoveAll(tempDir)
       db.Close()
   })
   ```

## Troubleshooting

### "dial tcp: lookup storage.googleapis.com: no such host"

**Problem:** Go is trying to download dependencies but network is unavailable.

**Solutions:**
- Use environment with network access
- Run only offline tests: `./scripts/test-offline.sh`
- Use `go mod vendor` to vendor dependencies

### "setup failed"

**Problem:** Test package dependencies missing.

**Solutions:**
- Ensure all dependencies in go.mod
- Run `go mod download` with network access
- Check go.sum is committed

### Tests pass locally but fail in CI

**Problem:** Environment differences (filesystem, permissions, etc.)

**Solutions:**
- Use in-memory databases for tests
- Mock file operations
- Avoid hardcoded paths
- Use `t.TempDir()` for temporary files

## Future Test Coverage Needed

- [ ] Integration tests for full download → metadata flow
- [ ] TUI Library view navigation tests
- [ ] TUI Search and filter interaction tests
- [ ] Batch download with metadata tests
- [ ] Settings panel tests
- [ ] Token validation tests
- [ ] VPN detection tests for CivitAI
- [ ] Performance tests (large metadata databases)
- [ ] Concurrent access tests
