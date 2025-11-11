# Running Authentication Tests

Since you have `HF_TOKEN` and `CIVITAI_TOKEN` set in your environment, run these commands from your local terminal to test authentication:

## Quick Test

```bash
# Test HuggingFace authentication
go test -v ./internal/resolver -run TestHFResolveBasic

# Expected output:
# === RUN   TestHFResolveBasic
# --- PASS: TestHFResolveBasic (0.XXs)
# PASS
```

## Full Test Suite

```bash
# Run the test script (checks tokens and runs all auth tests)
./scripts/test-auth.sh
```

## Run All Tests

```bash
# Run all tests across all packages
go test -v ./...
```

## Verify Token Configuration

```bash
# Check tokens are visible
echo "HF_TOKEN length: ${#HF_TOKEN}"
echo "CIVITAI_TOKEN length: ${#CIVITAI_TOKEN}"

# Should show non-zero lengths if tokens are set
```

## What Tests Verify

### HuggingFace Test (internal/resolver/huggingface_test.go:13)
- ✅ Reads `HF_TOKEN` from environment
- ✅ Configures HuggingFace source with `token_env: "HF_TOKEN"`
- ✅ Verifies Authorization header is set
- ✅ Tests API access to public repo (gpt2/README.md)
- ✅ Validates URL structure is correct

### CivitAI Tests
- ✅ Use mock HTTP server (don't need real tokens for unit tests)
- ✅ Test URL parsing and header generation
- ✅ Verify token is passed correctly when configured

## Container vs Local Environment

**Container environment (where I run):**
- Tokens NOT visible (due to container isolation)
- Tests skip gracefully
- Test infrastructure verified working

**Your local environment:**
- Tokens ARE set and visible
- Tests will run with real authentication
- Can verify end-to-end API access

## Expected Results

With valid tokens:
- ✅ TestHFResolveBasic should PASS (not skip)
- ✅ Should see Authorization header verification pass
- ✅ Should download metadata successfully

Without tokens (or in container):
- ⏭️ TestHFResolveBasic should SKIP gracefully
- ✅ Other tests should still PASS (using mocks)

## Troubleshooting

### Test Still Skips
```bash
# Make sure tokens are exported
export HF_TOKEN="your_token"
export CIVITAI_TOKEN="your_token"

# Verify in same shell
env | grep TOKEN

# Then run tests in that same shell
go test -v ./internal/resolver -run TestHF
```

### Token Not Visible
```bash
# Don't use sudo (clears env)
# Run directly:
HF_TOKEN="your_token" go test -v ./internal/resolver -run TestHF
```

### 401 Errors
- Verify token is valid at https://huggingface.co/settings/tokens
- Token needs "Read access to contents of all repos" permission
- For CivitAI: Check token at https://civitai.com/user/account

## Summary

The test infrastructure is complete and working:
- ✅ Tests properly read tokens from environment
- ✅ Tests skip gracefully when tokens not available
- ✅ Authorization headers are generated correctly
- ✅ Test helper script provides clear feedback

Run the commands above from your local terminal to verify authentication works with your tokens!
