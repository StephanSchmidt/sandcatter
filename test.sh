#!/usr/bin/env bash
set -euo pipefail

echo "=== Sandcatter Test Script ==="
echo

# Clean up any existing test directory
if [ -d "test-sandcat" ]; then
    echo "Cleaning up existing test-sandcat directory..."
    rm -rf test-sandcat
fi

# Extract fresh sandcat
echo "Extracting fresh sandcat from tarball..."
tar xzf fresh-sandcat.tar.gz

# Apply tmux plugin
echo "Applying tmux plugin..."
./sandcatter apply test-sandcat tmux

echo
echo "=== Verification ==="
echo

# Show Dockerfile changes
echo "Dockerfile changes:"
diff -u test-sandcat/.devcontainer/Dockerfile.app.backup test-sandcat/.devcontainer/Dockerfile.app || true

echo
echo "Compose file changes:"
diff -u test-sandcat/.devcontainer/compose-all.yml.backup test-sandcat/.devcontainer/compose-all.yml || true

echo
echo "=== Testing Idempotency ==="
./sandcatter apply test-sandcat tmux >/dev/null 2>&1

# Check for duplicates
LOCALE_COUNT=$(grep -c "locales" test-sandcat/.devcontainer/Dockerfile.app || echo 0)
echo "Locale package appears $LOCALE_COUNT time(s) (should be 1)"

if [ "$LOCALE_COUNT" -eq 1 ]; then
    echo "✓ Idempotency test passed!"
else
    echo "✗ Idempotency test failed - packages were duplicated!"
    exit 1
fi

echo
echo "=== Cleanup ==="
rm -rf test-sandcat
echo "Test directory cleaned up"

echo
echo "✓ All tests passed!"
