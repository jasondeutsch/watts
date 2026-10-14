// Runtime-neutral command/gate mechanics owned by this workload image.
import { redactor } from './redact.mjs';
import { spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { appendFileSync, lstatSync, mkdirSync, readFileSync, readdirSync, renameSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const outputLimit = 8192;
export function recordEvent(type, message, details, environment = process.env) {
  appendFileSync(environment.WATTS_EVENTS, JSON.stringify(redactor(environment).data({ id: randomUUID(), type, message, details })) + '\n', { mode: 0o600 });
}
export function snapshotFiles(root) {
  const files = {};
  function walk(directory, prefix = '') {
    for (const name of readdirSync(directory).sort()) {
      const path = join(directory, name), relative = prefix ? `${prefix}/${name}` : name;
      const stat = lstatSync(path);
      if (stat.isSymbolicLink()) throw new Error(`Workspace symlink ${relative} is unsupported`);
      if (stat.isDirectory()) { walk(path, relative); continue; }
      if (!stat.isFile() || stat.size > 64 * 1024 * 1024) throw new Error(`Unsupported workspace file ${relative}`);
      files[relative] = { sha256: createHash('sha256').update(readFileSync(path)).digest('hex'), mode: stat.mode & 0o777 };
    }
  }
  walk(root);
  return Object.fromEntries(Object.entries(files).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0));
}
function writeEvidence(path, data) {
  const temporary = `${path}.${randomUUID()}.tmp`;
  writeFileSync(temporary, JSON.stringify(data) + '\n', { mode: 0o600 });
  renameSync(temporary, path);
}
export async function runCommandTool(tool, { signal, environment = process.env } = {}) {
  const secrets = redactor(environment);
  const command = tool.command;
  if (!command) throw new Error(`Tool ${tool.name} is not a command tool`);
  const directory = command.working_dir || '/workspace';
  const timeout = (command.timeout_seconds || 300) * 1000;
  const id = randomUUID(), evidence = `tools/${id}.json`;
  mkdirSync(join(environment.WATTS_OUTPUTS, 'tools'), { recursive: true });
  const result = { tool: tool.name, argv: command.argv, working_dir: directory, status: 'error', exit_code: null, stdout: '', stderr: '', truncated: false, evidence };
  const started = Date.now();
  recordEvent('tool', `Starting tool ${tool.name}`, { tool: tool.name, evidence }, environment);
  if (signal?.aborted) result.status = 'canceled';
  else await new Promise(resolve => {
    // Preserve the image tool PATH; Pi's private binary directory must not
    // silently replace commands used by declared tools and authoritative gates.
    const env = { ...environment, PATH: environment.PI_WATTS_TOOL_PATH || environment.PATH };
    const child = spawn(command.argv[0], command.argv.slice(1), { cwd: directory, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
    let stopping = false, escalation;
    function stop(status) {
      if (stopping) return;
      stopping = true; result.status = status;
      try { process.kill(-child.pid, 'SIGTERM'); } catch {}
      escalation = setTimeout(() => { try { process.kill(-child.pid, 'SIGKILL'); } catch {} }, 1000);
    }
    const abort = () => stop('canceled');
    const timer = setTimeout(() => stop('timed_out'), timeout);
    signal?.addEventListener('abort', abort, { once: true });
    function collect(field, data) {
      const text = data.toString();
      if (result[field].length + text.length > outputLimit) result.truncated = true;
      result[field] = (result[field] + text).slice(-outputLimit);
    }
    const stdout = secrets.stream(), stderr = secrets.stream();
    child.stdout.pipe(stdout).on('data', data => collect('stdout', data));
    child.stderr.pipe(stderr).on('data', data => collect('stderr', data));
    const drained = Promise.all([stdout, stderr].map(stream => new Promise(resolve => stream.on('end', resolve))));
    child.on('error', error => { result.stderr = secrets.text(error.message); });
    child.on('close', async code => {
      await drained;
      clearTimeout(timer); clearTimeout(escalation); signal?.removeEventListener('abort', abort);
      if (!stopping && code !== null) { result.status = 'exited'; result.exit_code = code; }
      resolve();
    });
  });
  result.duration_ms = Date.now() - started;
  writeEvidence(join(environment.WATTS_OUTPUTS, evidence), secrets.data(result));
  recordEvent('tool', `Tool ${tool.name}: ${result.status}${result.exit_code === null ? '' : ` (${result.exit_code})`}`, { tool: tool.name, status: result.status, exit_code: result.exit_code, evidence }, environment);
  return secrets.data(result);
}
export async function evaluateGates(input, environment = process.env) {
  const tools = new Map((input.agent.tools || []).map(tool => [tool.name, tool]));
  const before = snapshotFiles(environment.WATTS_WORKSPACE);
  const outputsBefore = snapshotFiles(join(environment.WATTS_OUTPUTS, "task"));
  const results = [], failures = [];
  for (const gate of input.agent.gates || []) {
    const call = await runCommandTool(tools.get(gate.tool), { environment });
    const passed = call.status === 'exited' && call.exit_code === gate.expect.exit_code;
    results.push({ name: gate.name, tool: gate.tool, status: passed ? 'passed' : 'failed', exit_code: call.exit_code, evidence: call.evidence });
    recordEvent('gate', `Gate ${gate.name}: ${passed ? 'passed' : 'failed'}`, { gate: gate.name, tool: gate.tool, evidence: call.evidence }, environment);
    if (!passed) failures.push({ gate: gate.name, expected_exit_code: gate.expect.exit_code, ...call });
  }
  const after = snapshotFiles(environment.WATTS_WORKSPACE);
  if (JSON.stringify(before) !== JSON.stringify(after)) failures.push({ error: 'Gate commands changed the candidate workspace. Put build products and caches under /runtime and rerun every gate.' });
  const taskOutputs = snapshotFiles(join(environment.WATTS_OUTPUTS, 'task'));
  if (JSON.stringify(outputsBefore) !== JSON.stringify(taskOutputs)) failures.push({ error: 'Gate commands changed published task outputs; rerun gates against stable outputs.' });
  const report = { version: 1, execution: input.execution, workspace: after, task_outputs: taskOutputs, results };
  writeEvidence(join(environment.WATTS_OUTPUTS, 'gates.json'), report);
  return { report, failures };
}
