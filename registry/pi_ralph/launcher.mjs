import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { appendFileSync, cpSync, existsSync, mkdirSync, readFileSync, readdirSync, renameSync, writeFileSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';

import { evaluateGates, recordEvent } from './tools.mjs';

const input = JSON.parse(readFileSync(process.env.WATTS_INPUT, 'utf8'));
const settingsPath = '/inputs/config/launcher.json';
const options = existsSync(settingsPath) ? JSON.parse(readFileSync(settingsPath, 'utf8')) : {};
const task = '/runtime/task';
const agentDirectory = '/runtime/pi';
const sessionDirectory = '/runtime/sessions';
const eventFile = process.env.WATTS_EVENTS;
const outputs = input.outputs || [];

function event(type, message) {
  const id = createHash('sha256').update(`${input.execution}:${type}:${message}`).digest('hex');
  appendFileSync(eventFile, JSON.stringify({ id, type, message }) + '\n', { mode: 0o600 });
}
function pathIn(root, name) {
  const target = join(root, name);
  const rel = relative(root, target);
  if (!rel || rel.startsWith('../') || rel === '..' || name.startsWith('/')) throw new Error(`Invalid task artifact: ${name}`);
  return target;
}
function ralphStatuses(root) {
  if (!existsSync(root)) return [];
  const statuses = [];
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    const path = join(root, entry.name);
    if (entry.isDirectory()) statuses.push(...ralphStatuses(path));
    else if (entry.name === 'status.json') statuses.push(JSON.parse(readFileSync(path, 'utf8')));
  }
  return statuses;
}

function publishTaskOutputs() {
  for (const name of outputs) {
    const source = pathIn(task, name);
    if (!existsSync(source)) throw new Error(`Agent did not produce ${name}`);
    const target = pathIn('/outputs/task', name);
    mkdirSync(dirname(target), { recursive: true }); cpSync(source, target);
  }
}
async function runPi(args, prompt, environment) {
  const child = spawn('pi', [...args, prompt], { cwd: process.env.WATTS_WORKSPACE, env: environment, stdio: 'inherit' });
  const term = () => child.kill('SIGTERM'), interrupt = () => child.kill('SIGINT');
  process.on('SIGTERM', term); process.on('SIGINT', interrupt);
  try {
    const exit = await new Promise((resolve, reject) => { child.on('error', reject); child.on('exit', (code, signal) => resolve({ code, signal })); });
    if (exit.code !== 0) throw new Error(`Pi exited ${exit.code ?? exit.signal}`);
  } finally {
    process.removeListener('SIGTERM', term); process.removeListener('SIGINT', interrupt);
  }
}

try {
  if (input.version !== 1 || !input.agent) throw new Error('Pi image requires a version 1 Agent input');
  for (const directory of [task, agentDirectory, sessionDirectory, '/runtime/tmp']) mkdirSync(directory, { recursive: true });
  cpSync('/outputs/task', task, { recursive: true });
  const agent = input.agent;
  const settings = agent.settings || {};
  const skills = [...(settings.skills_dirs || [])];
  for (const skill of agent.skills || []) {
    const source = skill.source;
    skills.push(source.image_path || join('/inputs/resources', source.resource, source.path));
  }
  for (const skill of skills) if (!existsSync(skill)) throw new Error(`Missing skill directory: ${skill}`);
  const environment = {
    ...process.env, PI_WATTS_TOOL_PATH: process.env.PATH, PI_CODING_AGENT_DIR: agentDirectory, PI_OFFLINE: '1', PI_TELEMETRY: '0',
    PI_WATTS_RALPH_REGISTRY: '/runtime/ralph-registry',
    PI_WATTS_SKILL_ARGS: JSON.stringify(skills.flatMap(path => ['--skill', path])),
    PI_WATTS_COST_KNOWN: options.model_config?.cost ? '1' : '0',
  };
  writeFileSync(join(agentDirectory, 'settings.json'), JSON.stringify({ defaultProvider: agent.provider, defaultModel: agent.model, enableInstallTelemetry: false, defaultProjectTrust: 'never' }), { mode: 0o600 });
  if (settings.base_url) {
    if (!settings.api_key_env) throw new Error('Custom provider endpoint requires an auth env binding');
    const provider = { baseUrl: settings.base_url, api: settings.api || 'openai-completions', apiKey: `$${settings.api_key_env}`, models: [{ id: agent.model, name: agent.model, reasoning: true, input: ['text'], contextWindow: 128000, maxTokens: 16000, ...options.model_config }] };
    writeFileSync(join(agentDirectory, 'models.json'), JSON.stringify({ providers: { [agent.provider]: provider } }), { mode: 0o600 });
  }
  const servers = {};
  for (const name of new Set([...(agent.mcps || []), ...(agent.tools || []).filter(tool => tool.mcp).map(tool => tool.mcp.ref)])) {
    if (!settings.mcp_servers?.[name]) throw new Error(`MCP ${name} has no server configuration`);
    servers[name] = { ...settings.mcp_servers[name] };
    if (servers[name].cwd === '.') servers[name].cwd = '/workspace';
  }
  writeFileSync(join(agentDirectory, 'mcp.json'), JSON.stringify({ mcpServers: servers }), { mode: 0o600 });
  const args = ['--mode', 'json', '--print', '--offline', '--no-approve', '--no-context-files', '--no-extensions', '--provider', agent.provider, '--model', agent.model, '--session-dir', sessionDirectory, '-e', '/opt/watts/telemetry.ts', '-e', '/opt/watts/tools.ts'];
  if (settings.thinking) args.push('--thinking', settings.thinking);
  for (const skill of skills) args.push('--skill', skill);
  let prompt = agent.prompt.replaceAll('{task}', task);
  if (options.ralph) {
    const source = '/inputs/config/ralph.md';
    if (!existsSync(source)) throw new Error('Ralph requires a declared ralph.md configuration file');
    cpSync(source, join(task, 'RALPH.md'));
    if (!existsSync(join(task, 'OPEN_QUESTIONS.md'))) writeFileSync(join(task, 'OPEN_QUESTIONS.md'), '# Open questions\n\nRecord unresolved blocking questions here.\n');
    args.push('-e', '/usr/local/lib/node_modules/@lnilluv/pi-ralph-loop/src/index.ts');
    prompt = `/ralph --path ${task}`;
  } else {
    prompt += `\nTask documents are in ${task}. Application work belongs in /workspace. Write the requested task outputs to ${task}.`;
    if (input.previous) prompt += `\nPrevious stage result and feedback:\n${JSON.stringify(input.previous)}`;
  }
  // Ralph receives rework context as a private file and in its iteration prompt.
  if (input.previous) {
    writeFileSync(join(task, 'WORKFLOW_FEEDBACK.json'), JSON.stringify(input.previous));
    if (options.ralph) appendFileSync(join(task, 'RALPH.md'), `\nPrevious workflow feedback:\n${JSON.stringify(input.previous)}\n`);
  }
  const maxRepairs = agent.gate_policy?.max_repairs ?? 2;
  const basePrompt = prompt;
  let gateFeedback = '';
  for (let repair = 0; ; repair++) {
    if (repair > 0 && options.ralph) {
      const state = join(task, '.ralph-runner');
      if (existsSync(state)) {
        mkdirSync('/runtime/ralph-history', { recursive: true });
        renameSync(state, `/runtime/ralph-history/repair-${repair}`);
      }
      // Start a new private loop over the same candidate, not another workflow stage.
      cpSync('/inputs/config/ralph.md', join(task, 'RALPH.md'));
      if (input.previous) appendFileSync(join(task, 'RALPH.md'), `\nPrevious workflow feedback:\n${JSON.stringify(input.previous)}\n`);
      appendFileSync(join(task, 'RALPH.md'), `\nRequired gate repair feedback:\n${gateFeedback}\n`);
    }
    const invocationPrompt = options.ralph ? basePrompt : `${basePrompt}${gateFeedback ? `\nRequired gate repair feedback:\n${gateFeedback}` : ''}`;
    event('progress', `Starting Pi${options.ralph ? ' with Ralph' : ''} (gate repair ${repair})`);
    await runPi(args, invocationPrompt, environment);
    const errors = existsSync(eventFile) ? readFileSync(eventFile, 'utf8').split('\n').filter(Boolean).map(line => JSON.parse(line)).filter(record => record.type === 'error') : [];
    if (errors.length) throw new Error(errors[0].message);
    if (options.ralph && !ralphStatuses(task).some(status => status.status === 'complete')) throw new Error('Ralph did not complete its loop; inspect private runner evidence');
    publishTaskOutputs();
    if (!(agent.gates || []).length) break;
    const checked = await evaluateGates(input, environment);
    if (!checked.failures.length) break;
    gateFeedback = JSON.stringify(checked.failures);
    if (repair >= maxRepairs) {
      const causes = checked.failures.map(failure => failure.gate
        ? `${failure.gate}: ${failure.status}, exit ${failure.exit_code ?? 'unknown'}; evidence ${failure.evidence}`
        : failure.error).join('; ');
      throw new Error(`Required gates failed after ${repair} repair(s): ${causes}`);
    }
    writeFileSync(join(task, 'GATE_FEEDBACK.json'), gateFeedback + '\n');
    recordEvent('progress', 'Returning failed gates to Agent for repair', { repair: repair + 1, failures: checked.failures }, environment);
  }
  const events = existsSync(eventFile) ? readFileSync(eventFile, 'utf8').split('\n').filter(Boolean).map(line => JSON.parse(line)) : [];
  const error = events.find(event => event.type === 'error');
  if (error) throw new Error(error.message);
  if (!events.some(event => event.type === 'usage')) throw new Error('Pi produced no assistant-turn evidence');
  if (options.ralph && !ralphStatuses(task).some(status => status.status === 'complete')) throw new Error('Ralph did not complete its loop; inspect private runner evidence');
  let outcome = 'success', feedback;
  if (options.verdict) {
    const verdict = readFileSync(pathIn(task, options.verdict), 'utf8');
    if (!/VERDICT: (PASS|REJECT)\s*$/.test(verdict)) throw new Error('Review must end with VERDICT: PASS or VERDICT: REJECT');
    if (/VERDICT: REJECT\s*$/.test(verdict)) { outcome = 'changes-required'; feedback = verdict; }
  }
  writeFileSync(process.env.WATTS_RESULT, JSON.stringify({ outcome, feedback, artifacts: outputs }) + '\n', { mode: 0o600 });
  event('progress', 'Agent result published');
} catch (error) {
  event('error', error.message);
  console.error(`Agent workload failed: ${error.message}`);
  process.exitCode = 1;
}
