#!/bin/bash
# build.sh — Regenerate test .wasm fixtures from the Go generator.
#
# Prerequisites: Go toolchain (no external deps like wabt needed).
# The generator produces minimal valid Wasm binaries by hand-encoding
# the binary format directly.
#
# Usage:
#   cd internal/plugin/testdata && bash build.sh
#
# For CI, pre-compiled .wasm files are committed to the repo.
# This script is for regeneration only.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

echo "Generating test .wasm fixtures..."
(cd "$PROJECT_ROOT" && go run ./internal/plugin/testdata/generate.go)
echo "Done."
