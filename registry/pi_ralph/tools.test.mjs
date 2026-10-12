import assert from 'node:assert/strict';
import test from 'node:test';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { evaluateGates, runCommandTool, snapshotFiles } from './tools.mjs';
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'watts-tools-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const workspace = join(root, 'workspace'), outputs = join(root, 'outputs');
  mkdirSync(workspace); mkdirSync(outputs); mkdirSync(join(outputs, "task"));
  const environment = { ...process.env, WATTS_WORKSPACE: workspace, WATTS_OUTPUTS: outputs, WATTS_EVENTS: join(outputs, 'events.jsonl') };
  return { root, workspace, outputs, environment };
}
test('command tools retain failure output, timeout, cancellation, and evidence', async t => {
  const f = fixture(t);
  const tool = { name: 'check', command: { argv: [process.execPath, '-e', 'console.log("diagnostic"); process.exit(7)'], working_dir: f.workspace } };
  const result = await runCommandTool(tool, f);
  assert.equal(result.status, 'exited'); assert.equal(result.exit_code, 7); assert.match(result.stdout, /diagnostic/);
  assert.equal(JSON.parse(readFileSync(join(f.outputs, result.evidence))).exit_code, 7);
  const slow = { name: 'slow', command: { argv: [process.execPath, '-e', 'setTimeout(()=>{},10000)'], working_dir: f.workspace, timeout_seconds: 1 } };
  assert.equal((await runCommandTool(slow, f)).status, 'timed_out');
  const controller = new AbortController(); controller.abort();
  assert.equal((await runCommandTool(slow, { ...f, signal: controller.signal })).status, 'canceled');
  const active = new AbortController();
  const running = runCommandTool(slow, { ...f, signal: active.signal });
  setTimeout(() => active.abort(), 50);
  assert.equal((await running).status, 'canceled');
  const verbose = { name: 'verbose', command: { argv: [process.execPath, '-e', 'console.log("x".repeat(20000))'], working_dir: f.workspace } };
  const bounded = await runCommandTool(verbose, f);
  assert.equal(bounded.truncated, true); assert.equal(bounded.stdout.length, 8192);
});
test('gates inspect the final workspace, retain individual outcomes, and detect gate mutations', async t => {
  const f = fixture(t);
  const tool = { name: 'verify', command: { argv: [process.execPath, '-e', 'require("node:fs").accessSync("fixed")'], working_dir: f.workspace } };
  const input = { execution: 'run', agent: { tools: [tool], gates: [{ name: 'verified', tool: 'verify', expect: { exit_code: 0 } }] } };
  assert.equal((await evaluateGates(input, f.environment)).failures.length, 1);
  writeFileSync(join(f.workspace, 'fixed'), 'done');
  const passed = await evaluateGates(input, f.environment);
  assert.equal(passed.failures.length, 0); assert.deepEqual(passed.report.workspace, snapshotFiles(f.workspace));
  tool.command.argv = [process.execPath, '-e', 'require("node:fs").writeFileSync("unexpected-build-output","binary")'];
  assert.match((await evaluateGates(input, f.environment)).failures[0].error, /changed the candidate/);
});
