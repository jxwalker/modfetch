# Authentication Test Status

## Summary

The test suite has been updated to properly handle HuggingFace and CivitAI authentication. Tests now correctly read tokens from environment variables when available.

## Current Test Status

### ✅ Tests Passing (No Auth Required)
- `internal/config` - Configuration loading and validation
- `internal/util` - Utility functions
- `internal/classifier` - Model type detection
- `internal/placer` - File placement logic
- `internal/logging` - Logging utilities
- `internal/resolver` - CivitAI resolver (uses mock server)

### ⏭️ Tests Skipped (Auth Required)
- `internal/resolver/TestHFResolveBasic` - Requires `HF_TOKEN` environment variable
- `internal/metadata/*` - Requires both `HF_TOKEN` and `CIVITAI_TOKEN`

### 🔧 Packages with Setup Issues
- `internal/state` - SQLite module download issues (DNS resolution)
- `internal/scanner` - SQLite dependency
- `internal/downloader` - SQLite dependency
- `internal/tui` - SQLite dependency
- `internal/testutil` - SQLite dependency

## How to Run Tests with Authentication

### Option 1: Run Test Script

```bash
# Set your tokens
export HF_TOKEN="hf_..."
export CIVITAI_TOKEN="..."

# Run the test script
./scripts/test-auth.sh
```

### Option 2: Run Tests Directly

```bash
# Set tokens
export HF_TOKEN="hf_..."
export CIVITAI_TOKEN="..."

# Run all tests
go test ./...

# Run specific tests
go test -v ./internal/resolver -run TestHF
go test -v ./internal/metadata
```

### Option 3: Inline Token Setting

```bash
HF_TOKEN="hf_..." CIVITAI_TOKEN="..." go test ./internal/resolver -v
```

## Token Setup

### HuggingFace Token
1. Go to https://huggingface.co/settings/tokens
2. Create a new token with "Read access to contents of all repos" permission
3. Copy the token (starts with `hf_`)
4. Set it: `export HF_TOKEN="hf_..."`

### CivitAI Token
1. Go to https://civitai.com/user/account
2. Scroll to "API Keys" section
3. Create a new API key
4. Copy the token
5. Set it: `export CIVITAI_TOKEN="..."`

## Fixes Applied

### 1. HuggingFace Test Authentication (internal/resolver/huggingface_test.go)

**Before:**
- Test made API calls without authentication
- Failed with 401 errors on CI
- Didn't read HF_TOKEN from environment

**After:**
- Test properly configures HuggingFace source in config
- Reads HF_TOKEN from environment
- Skips gracefully if token not available
- Verifies Authorization header is set when token is present

**Code changes:**
```go
// Configure HuggingFace with token_env
cfgYaml := []byte("version: 1\n" +
    "general:\n  data_root: \"" + tmp + "\"\n  download_root: \"" + tmp + "\"\n" +
    "sources:\n  huggingface:\n    enabled: true\n    token_env: \"HF_TOKEN\"\n")
```

### 2. Test Script (scripts/test-auth.sh)

Created a comprehensive test script that:
- Checks if tokens are set
- Provides clear instructions if missing
- Runs resolver tests with authentication
- Runs metadata tests if available
- Reports results clearly

## Verification Steps

To verify authentication works:

1. **Check token visibility:**
   ```bash
   echo "HF_TOKEN length: ${#HF_TOKEN}"
   echo "CIVITAI_TOKEN length: ${#CIVITAI_TOKEN}"
   ```

2. **Test HuggingFace auth:**
   ```bash
   export HF_TOKEN="your_token"
   go test -v ./internal/resolver -run TestHF
   ```

   Expected output:
   ```
   === RUN   TestHFResolveBasic
   --- PASS: TestHFResolveBasic (0.XXs)
   PASS
   ```

3. **Test CivitAI auth:**
   ```bash
   export CIVITAI_TOKEN="your_token"
   go test -v ./internal/resolver -run TestCivitAI
   ```

## Known Issues

### DNS Resolution for Go Modules
Some test packages fail to build due to DNS resolution issues when downloading the SQLite module. This is a network configuration issue, not a code issue.

**Workaround:**
- Use `go mod download` first to cache modules
- Run tests for individual packages that don't depend on SQLite
- Use `GOPROXY=direct` or `GOPROXY=https://proxy.golang.org,direct`

### Token Visibility
If you've set `HF_TOKEN` and `CIVITAI_TOKEN` but tests still skip:
1. Verify they're in your current shell: `env | grep TOKEN`
2. Make sure to export them: `export HF_TOKEN="..."`
3. Run tests in the same shell session where you set the tokens
4. Don't use `sudo` which might clear environment variables

## CI Configuration

For GitHub Actions CI, add secrets:
1. Go to repo Settings → Secrets and variables → Actions
2. Add `HF_TOKEN` with your HuggingFace token
3. Add `CIVITAI_TOKEN` with your CivitAI token

In `.github/workflows/ci.yml`, the tests will automatically use these secrets if configured.

## Next Steps

Once tokens are properly available in the test environment:
1. All resolver tests should pass
2. Metadata fetcher tests should pass
3. Integration tests can verify end-to-end API access

The test infrastructure is now ready to properly test authentication!
