import assert from 'node:assert/strict';
import test from 'node:test';
import { mcpConfigurations, verifyMCPs } from './mcp.mjs';

function fixture() {
  return { tools: [{ name: 'code', mcp: { ref: 'code', exposure: 'direct', allow_tools: ['search'] } }],
    mcp_services: { code: {
      connection: { transport: 'stdio', command: { argv: ['gopls', 'mcp'], working_dir: '/workspace' }, timeout_seconds: 1 },
      lifecycle: 'attempt', tools: { discovery: 'allowlist', allow: ['search', 'diagnostics'] },
      requirements: { os: ['linux'], arch: ['amd64', 'arm64'] },
      secret_environment: { API_KEY: 'MCP_KEY' },
    } } };
}

test('consumer selection restricts native exposure and keeps credentials as bindings', () => {
  const agent = fixture(), servers = mcpConfigurations(agent, { MCP_KEY: 'private-key' }, 'linux', 'x64');
  assert.deepEqual(servers.code.args, ['mcp']);
  assert.equal(servers.code.cwd, '/workspace');
  assert.deepEqual(servers.code.toolExposure, { search: 'direct', '*': 'hidden' });
  assert.equal(servers.code.exposure, 'hidden');
  assert.equal(servers.code.env.API_KEY, '${MCP_KEY}');
  assert.ok(!JSON.stringify(servers).includes('private-key'));
  assert.throws(() => mcpConfigurations(agent, {}, 'linux', 'arm64'), /authentication: missing binding/);
  assert.throws(() => mcpConfigurations(agent, { MCP_KEY: 'private-key' }, 'darwin', 'arm64'), /platform/);
});

test('readiness verifies requested tools, classifies errors, redacts secrets, and closes clients', async () => {
  const agent = fixture(), environment = { MCP_KEY: 'private-key' };
  const servers = mcpConfigurations(agent, environment, 'linux', 'arm64');
  let closed = 0, tools = [{ name: 'search' }], failure;
  class Client {
    async connect() { if (failure) throw failure; }
    async listTools() { return tools; }
    async close() { closed++; }
  }
  const library = { McpClient: Client, StdioTransport: class {} }, messages = [];
  await verifyMCPs(agent, servers, environment, (_, message) => messages.push(message), library);
  assert.match(messages[0], /ready/);
  tools = [{ name: 'other' }];
  await assert.rejects(verifyMCPs(agent, servers, environment, () => {}, library), /unavailable_tools.*search/);
  failure = Object.assign(new Error('Bad credential private-key'), { status: 401 });
  await assert.rejects(verifyMCPs(agent, servers, environment, () => {}, library), error =>
    error.message.includes('authentication') && error.message.includes('[redacted]') && !error.message.includes('private-key'));
  assert.equal(closed, 3);
});
