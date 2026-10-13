#!/usr/bin/env bash
set -euo pipefail

# Watts runs this container command from the isolated application workspace.
if [[ ! -f go.mod ]]; then
  echo 'lint: run from the Go project root containing go.mod' >&2
  exit 1
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo 'lint: golangci-lint v2 must be installed in the workload image' >&2
  exit 1
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec golangci-lint run --config "$script_dir/../.golangci.yml" ./...
