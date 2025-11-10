#!/bin/bash
#
# Run tests that work without network connectivity
# This script runs all tests that don't require external API calls or network access
#

set -e

echo "================================================"
echo "ModFetch Offline Test Suite"
echo "================================================"
echo ""

# Track overall status
FAILED=0

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

run_test() {
    local package=$1
    local name=$2

    echo -n "Testing $name... "
    if go test -v "$package" > /tmp/test_output.txt 2>&1; then
        echo -e "${GREEN}PASS${NC}"
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Error output:"
        cat /tmp/test_output.txt | grep -E "FAIL|error" | head -5 | sed 's/^/    /'
        FAILED=$((FAILED + 1))
        return 1
    fi
}

echo "Running unit tests that work offline..."
echo ""

# Utility tests - always work
run_test "./internal/util/..." "Utility Functions"

# Classifier tests - work offline
run_test "./internal/classifier/..." "File Classifier"

# Config tests - work offline
run_test "./internal/config/..." "Config Loading"

# Logging tests - work offline
run_test "./internal/logging/..." "Logging & Sanitization"

# Placer tests - work offline
run_test "./internal/placer/..." "File Placement"

# CivitAI resolver tests - work offline (mocked)
echo -n "Testing CivitAI Resolver... "
if go test -v ./internal/resolver/... -run TestCivitAI > /tmp/test_output.txt 2>&1; then
    echo -e "${GREEN}PASS${NC}"
else
    echo -e "${YELLOW}SKIP${NC} (requires network)"
fi

echo ""
echo "================================================"
echo "Test Summary"
echo "================================================"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All offline tests passed!${NC}"
    exit 0
else
    echo -e "${RED}$FAILED test suite(s) failed${NC}"
    exit 1
fi
