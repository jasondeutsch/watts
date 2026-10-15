#!/usr/bin/env bash
# Keep build policy outside Watts' generic workload executor.
set -euo pipefail
cd "$(dirname "$0")/.."

die() { printf 'Image build: %s\n' "$*" >&2; exit 1; }
buildx() {
  if [[ -n "${BUILDX:-}" ]]; then "$BUILDX" "$@"; else docker buildx "$@"; fi
}

command -v docker >/dev/null || die 'Docker CLI is required.'
buildx version || die 'Docker Buildx is required (included in current Docker Desktop). Install/enable the CLI plugin, or set BUILDX to its executable path.'
context=$(docker context show) || die 'Cannot read Docker context.'
printf 'Docker context: %s\n' "$context"
endpoint=$(docker context inspect "$context" --format '{{.Endpoints.docker.Host}}') || die "Cannot read endpoint for context $context."
worker_endpoint=${DOCKER_HOST:-unix:///var/run/docker.sock}
[[ "$endpoint" == "$worker_endpoint" ]] || die "Context $context uses $endpoint but the Watts SDK uses $worker_endpoint. Set DOCKER_HOST to the context endpoint for both this build and wctl, or select a matching context for this invocation."
platform=$(docker version --format '{{.Server.Os}}/{{.Server.Arch}}') || die 'Cannot reach the Docker engine.'
# Select the context-owned Docker driver for this invocation only. A selected
# remote/container builder may have a different store; do not use it implicitly.
builder=$(buildx inspect "$context") || die "Cannot inspect builder for context $context."
printf '%s\n' "$builder"
[[ "$builder" =~ Driver:[[:space:]]+docker([[:space:]]|$) ]] || die 'This local build requires the context-owned docker driver; no builder defaults were changed.'
case "$platform" in
  linux/arm64|linux/amd64) ;;
  *) die "Unsupported worker engine platform: $platform (expected linux/arm64 or linux/amd64)." ;;
esac
[[ -z "${DOCKER_DEFAULT_PLATFORM:-}" || "$DOCKER_DEFAULT_PLATFORM" == "$platform" ]] || die "DOCKER_DEFAULT_PLATFORM=$DOCKER_DEFAULT_PLATFORM differs from the worker engine $platform. Unset it for a native build."
printf 'Native worker platform: %s\nBuilding registry/pi_ralph/Dockerfile -> registry/coder_golang/Dockerfile\nBase: watts-agent-base = Bake target pi-ralph (no remote Watts lookup)\n' "$platform"
if ! buildx bake --builder "$context" -f registry/docker-bake.hcl --load --progress plain --set "*.platform=$platform"; then
  die 'Build graph failed; see the named Dockerfile/step above. No derivative can finish without its Pi/Ralph dependency. Upstream image/package downloads still require network access.'
fi
for image in "watts/pi-ralph:${WATTS_IMAGE_TAG:-0.1.0}" "watts/coder-golang:${WATTS_IMAGE_TAG:-0.1.0}"; do
  actual=$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}') || die "$image was not loaded into the engine for context $context."
  [[ "$actual" == "$platform" ]] || die "$image has platform $actual; expected $platform."
  printf 'Available: %s (%s)\n' "$image" "$actual"
done
printf 'Images are local engine outputs, not published registry images. Use the same engine for wctl.\n'
