#!/usr/bin/env bash
set -euo pipefail

# Build products are private execution artifacts, not application source changes.
mkdir -p /runtime/build
exec go build -o /runtime/build/ ./...
