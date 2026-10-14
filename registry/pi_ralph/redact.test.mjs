import { test } from 'node:test';
import assert from 'node:assert/strict';
import { redactor } from './redact.mjs';
import { runCommandTool } from './tools.mjs';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const secret = 'synthetic-"credential\\value';
const environment = { ...process.env, WATTS_SECRET_ENV_NAMES: '["FIXTURE_KEY"]', FIXTURE_KEY: secret };
test('nested/structured output and every streaming split', async () => {
  const safe = redactor(environment);
  assert.deepEqual(safe.data({ content: [{ text: secret }], details: { nested: [secret] } }), { content: [{ text: '[REDACTED]' }], details: { nested: ['[REDACTED]'] } });
  const input = `before ${secret} ${JSON.stringify(secret).slice(1, -1)} after`;
  for (let split = 0; split <= input.length; split++) {
    const stream = safe.stream(); let output = '';
    stream.on('data', chunk => { output += chunk; });
    const done = new Promise(resolve => stream.on('end', resolve));
    stream.write(input.slice(0, split)); stream.end(input.slice(split)); await done;
    assert.equal(output, 'before [REDACTED] [REDACTED] after');
  }
});
test('failing command sanitizes evidence and model-facing result before truncation', async () => {
  const directory = mkdtempSync(join(tmpdir(), 'watts-redaction-'));
  try {
    const env = { ...environment, WATTS_OUTPUTS: directory, WATTS_EVENTS: join(directory, 'events.jsonl') };
    const result = await runCommandTool({ name: 'dump', command: { argv: ['sh', '-c', 'printf "%s" "$FIXTURE_KEY"; printf "%s" "$FIXTURE_KEY" >&2; exit 9'], working_dir: directory } }, { environment: env });
    assert.equal(result.exit_code, 9);
    assert.equal(result.stdout, '[REDACTED]'); assert.equal(result.stderr, '[REDACTED]');
    assert.equal(readFileSync(join(directory, result.evidence), 'utf8').includes(secret), false);
  } finally { rmSync(directory, { recursive: true, force: true }); }
});
