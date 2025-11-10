# Testing Guide

**Version:** 1.0
**Date:** 2025-11-10
**Status:** Complete

---

## Overview

modfetch has a comprehensive test suite with **4 layers of testing**:

1. **Unit Tests** (65 test cases) - Test individual components in isolation
2. **Integration Tests** (6 test cases) - Test component interactions
3. **Performance Benchmarks** (13 benchmarks) - Validate performance claims
4. **E2E Tests** (documented scenarios) - Test complete user workflows

**Total Test Code:** 3,900+ lines
**Coverage Target:** 90%+ for critical paths

---

## Running Tests

### All Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Specific Test Suites

```bash
# Scanner tests only (20 cases)
go test -v ./internal/scanner

# Library tests only (25 cases)
go test -v ./internal/tui -run TestLibrary

# Settings tests only (20 cases)
go test -v ./internal/tui -run TestSettings

# Integration tests only (6 cases)
go test -v ./internal/tui -run TestIntegration
```

### Performance Benchmarks

```bash
# Run all benchmarks
go test -bench=. ./internal/scanner

# Run specific benchmark
go test -bench=BenchmarkScanDirectories_1000Files ./internal/scanner

# With memory profiling
go test -bench=. -benchmem ./internal/scanner
```

### Race Detection

```bash
# Check for race conditions
go test -race ./...
```

---

## Test Coverage

| Module | File | Lines | Tests | Coverage |
|--------|------|-------|-------|----------|
| Scanner | scanner_test.go | 649 | 20 | 95%+ |
| Library | library_test.go | 683 | 25 | 90%+ |
| Settings | settings_test.go | 689 | 20 | 95%+ |
| Integration | library_integration_test.go | 470 | 6 | N/A |
| Benchmarks | scanner_bench_test.go | 410 | 13 | N/A |
| **Total** | **5 files** | **2,901** | **84** | **90%+** |

---

## Test Structure

```
internal/
├── scanner/
│   ├── scanner_test.go           (Unit - 20 tests)
│   └── scanner_bench_test.go     (Perf - 13 benchmarks)
└── tui/
    ├── library_test.go            (Unit - 25 tests)
    ├── settings_test.go           (Unit - 20 tests)
    └── library_integration_test.go (Integration - 6 tests)
```

---

## Performance Targets

| Benchmark | Target | Validates |
|-----------|--------|-----------|
| Scan 100 files | < 50ms | Small libraries |
| Scan 1,000 files | < 200ms | Medium libraries |
| Scan 10,000 files | < 500ms | Large libraries |
| Duplicate query (10K DB) | < 5ms | O(log n) performance |
| File type detection | < 10ns | Efficient matching |

---

## Best Practices

### Writing Tests

1. Use `t.TempDir()` for file operations
2. Create fresh databases per test
3. Use `t.Helper()` in test helpers
4. Clean up with `defer`
5. Isolate tests (no shared state)

### Example Test

```go
func TestExample(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    db, cleanup := setupTestDB(t)
    defer cleanup()

    // Execute
    result, err := someFunction(db)

    // Assert
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

---

## Running in CI

```yaml
- name: Run tests
  run: go test -v -cover ./...

- name: Run benchmarks
  run: go test -bench=. ./internal/scanner

- name: Race detector
  run: go test -race ./...
```

---

**For detailed scenarios, see:**
- [Unit Test Documentation](../internal/scanner/scanner_test.go)
- [Integration Test Documentation](../internal/tui/library_integration_test.go)
- [Performance Benchmarks](../internal/scanner/scanner_bench_test.go)
