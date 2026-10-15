import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

function run(overrides = {}) {
  const root = mkdtempSync(join(tmpdir(), 'watts-build-'));
  mkdirSync(join(root, 'bin'));
  const log = join(root, 'commands');
  writeFileSync(join(root, 'bin/docker'), `#!/bin/sh
printf '%s\\n' "$*" >> "$BUILD_LOG"
case "$*" in
 'buildx version') [ "$MISSING_BUILDX" != 1 ] ;;
 'context show') echo test-context ;;
 'context inspect '*) echo unix:///var/run/docker.sock ;;
 'version --format '*) echo linux/arm64 ;;
 'buildx inspect test-context') echo "Driver: \${TEST_DRIVER:-docker}" ;;
 'buildx bake '*) exit "\${BUILD_EXIT:-0}" ;;
 'image inspect '*) [ "$MISSING_IMAGE" != 1 ] && echo "\${IMAGE_PLATFORM:-linux/arm64}" ;;
 *) exit 99 ;;
esac
`, { mode: 0o755 });
  try {
    const result = spawnSync('bash', ['registry/build-images.sh'], {
      encoding: 'utf8',
      env: { ...process.env, PATH: `${join(root, 'bin')}:${process.env.PATH}`, BUILDX: '',
        DOCKER_DEFAULT_PLATFORM: '', DOCKER_HOST: '', WATTS_IMAGE_TAG: 'test', BUILD_LOG: log, ...overrides },
    });
    return { ...result, commands: readFileSync(log, 'utf8') };
  } finally { rmSync(root, { recursive: true, force: true }); }
}

test('builds graph in context-owned builder and verifies both local outputs', () => {
  const result = run();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.commands, /bake --builder test-context .*--load .*--set \*\.platform=linux\/arm64/);
  assert.match(result.commands, /image inspect watts\/pi-ralph:test/);
  assert.match(result.commands, /image inspect watts\/coder-golang:test/);
  assert.doesNotMatch(result.commands, /context use|buildx use|pull /);
});

test('stops on failed graph without claiming image availability', () => {
  const result = run({ BUILD_EXIT: '1' });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /Build graph failed/);
  assert.doesNotMatch(result.commands, /image inspect/);
});

for (const [name, env, diagnostic] of [
  ['different engine', { DOCKER_HOST: 'unix:///another-engine.sock' }, /Watts SDK uses/],
  ['missing plugin', { MISSING_BUILDX: '1' }, /Buildx is required/],
  ['different driver', { TEST_DRIVER: 'docker-container' }, /context-owned docker driver/],
  ['conflicting platform', { DOCKER_DEFAULT_PLATFORM: 'linux/amd64' }, /differs from the worker engine/],
  ['unexported image', { MISSING_IMAGE: '1' }, /not loaded into the engine/],
  ['wrong output platform', { IMAGE_PLATFORM: 'linux/amd64' }, /expected linux\/arm64/],
]) {
  test(`reports ${name}`, () => {
    const result = run(env);
    assert.equal(result.status, 1);
    assert.match(result.stderr, diagnostic);
  });
}
