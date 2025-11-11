#!/bin/bash
# Test script to verify HuggingFace and CivitAI authentication
# Usage: HF_TOKEN=xxx CIVITAI_TOKEN=yyy ./scripts/test-auth.sh

set -e

echo "=== Testing HuggingFace and CivitAI Authentication ==="
echo ""

# Check if tokens are set
if [ -z "$HF_TOKEN" ]; then
    echo "❌ HF_TOKEN not set"
    echo "   Set it with: export HF_TOKEN='your_token_here'"
    HF_MISSING=1
else
    echo "✓ HF_TOKEN is set (${#HF_TOKEN} characters)"
fi

if [ -z "$CIVITAI_TOKEN" ]; then
    echo "❌ CIVITAI_TOKEN not set"
    echo "   Set it with: export CIVITAI_TOKEN='your_token_here'"
    CIVITAI_MISSING=1
else
    echo "✓ CIVITAI_TOKEN is set (${#CIVITAI_TOKEN} characters)"
fi

echo ""
echo "=== Running Tests ==="
echo ""

# Disable exit-on-error for tests (allow them to fail gracefully)
set +e

# Run resolver tests with tokens
export HF_TOKEN CIVITAI_TOKEN
go test -v ./internal/resolver -run TestHF || echo "HuggingFace test failed"
go test -v ./internal/resolver -run TestCivitAI || echo "CivitAI test failed"

# Run metadata tests if they exist
if go test -v ./internal/metadata -list=. 2>/dev/null | grep -q Test; then
    echo ""
    echo "=== Running Metadata Tests ==="
    go test -v ./internal/metadata || echo "Metadata tests failed"
fi

# Re-enable exit-on-error
set -e

echo ""
echo "=== Test Complete ==="

if [ -n "$HF_MISSING" ] || [ -n "$CIVITAI_MISSING" ]; then
    echo ""
    echo "⚠️  Some tokens were missing. To test with authentication:"
    echo "    export HF_TOKEN='your_hf_token'"
    echo "    export CIVITAI_TOKEN='your_civitai_token'"
    echo "    ./scripts/test-auth.sh"
fi
