#!/usr/bin/env bash
set -euo pipefail

# Watts runs workflow commands from the project root containing watts.yaml.
if [[ ! -f watts.yaml || ! -f go.mod ]]; then
  echo 'lint: run from the Go project root containing watts.yaml and go.mod' >&2
  exit 1
fi

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo 'lint: golangci-lint v2 must be installed on the worker PATH' >&2
  exit 1
fi

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec golangci-lint run --config "$script_dir/../.golangci.yml" ./...
