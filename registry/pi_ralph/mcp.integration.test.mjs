// Run inside watts/coder-golang with this directory mounted at /test and a small
// Go fixture at /workspace. No provider credentials or model calls are involved.
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import test from 'node:test';

const integration = process.env.WATTS_MCP_INTEGRATION === '1';
const { mcpConfigurations, verifyMCPs, loadMCPClient } = await import(integration ? '/opt/watts/mcp.mjs' : './mcp.mjs');
function agent(service, selection = {}) {
  return { tools: [{ name: 'service', mcp: { ref: 'service', ...selection } }], mcp_services: { service } };
}
function stdio(command = ['gopls', 'mcp'], seconds = 20) {
  return { connection: { transport: 'stdio', command: { argv: command, working_dir: '/workspace' }, timeout_seconds: seconds },
    lifecycle: 'attempt', tools: { discovery: 'all' }, requirements: { os: ['linux'] } };
}

test('native library connects to gopls, calls a real tool, and stops the server', { skip: !integration }, async () => {
  const library = await loadMCPClient(), workload = agent(stdio(), { allow_tools: ['go_workspace'], exposure: 'direct' });
  const configurations = mcpConfigurations(workload);
  const progress = [];
  await verifyMCPs(workload, configurations, process.env, (_, message) => progress.push(message), library);
  assert.match(progress[0], /ready/);
  const client = new library.McpClient({ name: 'watts-test', version: '0.1.0', roots: [{ uri: 'file:///workspace', name: 'project' }] });
  const transport = new library.StdioTransport({ command: 'gopls', args: ['mcp'], cwd: '/workspace' });
  let pid;
  try {
    await client.connect(transport);
    pid = transport.pid;
    const tools = await client.listTools();
    assert.ok(tools.some(tool => tool.name === 'go_workspace'));
    const result = await client.callTool('go_workspace', {});
    assert.notEqual(result.isError, true);
    assert.ok(result.content.length > 0);
    const nativeConfig = await import('/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/dist/core/mcp-servers.js');
    assert.equal(nativeConfig.getMcpToolExposure(configurations.service, 'go_workspace'), 'direct');
    assert.equal(nativeConfig.getMcpToolExposure(configurations.service, 'go_rename_symbol'), 'hidden');
  } finally { await client.close(); }
  assert.throws(() => process.kill(pid, 0), /ESRCH/);
});

test('readiness rejects unavailable tools and cleans up after timeout and cancellation', { skip: !integration }, async () => {
  const library = await loadMCPClient();
  const unavailable = agent(stdio(), { allow_tools: ['not_a_gopls_tool'] });
  await assert.rejects(verifyMCPs(unavailable, mcpConfigurations(unavailable), process.env, () => {}, library), /unavailable_tools/);
  for (const cancel of [false, true]) {
    let pid;
    const instrumented = { ...library, StdioTransport: class extends library.StdioTransport {
      async start() { await super.start(); pid = this.pid; }
    } };
    const slow = agent(stdio(['node', '-e', 'process.stdin.resume()'], 1));
    const pending = verifyMCPs(slow, mcpConfigurations(slow), process.env, () => {}, instrumented);
    const timer = cancel ? setTimeout(() => process.emit('SIGTERM'), 150) : null;
    try { await assert.rejects(pending, cancel ? /cancelled/ : /timeout/); }
    finally { clearTimeout(timer); }
    assert.throws(() => process.kill(pid, 0), /ESRCH/);
  }
});

test('native streamable HTTP uses bound auth and reports authentication failure', { skip: !integration }, async t => {
  const received = [];
  // Test-only protocol fixture: production negotiation uses Pi's MCP library.
  const server = createServer(async (request, response) => {
    if (request.method !== 'POST') { response.writeHead(405).end(); return; }
    received.push(request.headers.authorization);
    if (request.headers.authorization !== 'Bearer test-token') { response.writeHead(401).end('Bad token'); return; }
    let body = ''; for await (const chunk of request) body += chunk;
    const message = JSON.parse(body);
    if (!message.id) { response.writeHead(202).end(); return; }
    const result = message.method === 'initialize'
      ? { protocolVersion: message.params.protocolVersion, capabilities: { tools: {} }, serverInfo: { name: 'fixture', version: '0.1.0' } }
      : { tools: [{ name: 'lookup', inputSchema: { type: 'object' } }] };
    response.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify({ jsonrpc: '2.0', id: message.id, result }));
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise(resolve => server.close(resolve)));
  const workload = agent({ connection: { transport: 'streamable-http', url: `http://127.0.0.1:${server.address().port}/mcp`, timeout_seconds: 3 },
    lifecycle: 'external', tools: { discovery: 'allowlist', allow: ['lookup'] }, secret_headers: { Authorization: { environment: 'MCP_AUTH', prefix: 'Bearer ' } } });
  const environment = { ...process.env, MCP_AUTH: 'test-token' }, configurations = mcpConfigurations(workload, environment);
  await verifyMCPs(workload, configurations, environment, () => {});
  assert.ok(received.every(value => value === 'Bearer test-token'));
  assert.ok(!JSON.stringify(configurations).includes('test-token'));
  await assert.rejects(verifyMCPs(workload, configurations, { MCP_AUTH: 'wrong-token' }, () => {}), /authentication/);
});
