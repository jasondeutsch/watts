// Protocol negotiation, discovery, timeouts, and process-group cleanup belong to
// Pi's pinned MCP library. This adapter only translates the Watts contract.
const clientLibrary = '/usr/local/lib/node_modules/@earendil-works/pi-coding-agent/node_modules/@earendil-works/pi-mcp/dist/index.js';

function assertPlatform(name, service, platform, arch) {
  const requirements = service.requirements || {};
  for (const [key, actual] of [['os', platform], ['arch', arch === 'x64' ? 'amd64' : arch]]) {
    if (requirements[key]?.length && !requirements[key].includes(actual)) {
      throw new Error(`MCP ${name}: platform: requires ${key} ${requirements[key].join(', ')}, workload is ${actual}`);
    }
  }
}

function allowedTools(service, selection) {
  return selection.allow_tools?.length ? selection.allow_tools : service.tools.discovery === 'allowlist' ? service.tools.allow : null;
}

export function mcpConfigurations(agent, environment = process.env, platform = process.platform, arch = process.arch) {
  const servers = {};
  for (const tool of agent.tools || []) {
    if (!tool.mcp) continue;
    const name = tool.mcp.ref, service = agent.mcp_services?.[name];
    if (!service) throw new Error(`MCP ${name}: configuration: missing resolved service`);
    assertPlatform(name, service, platform, arch);
    const connection = service.connection;
    const config = { timeout: connection.timeout_seconds, description: service.description, exposure: tool.mcp.exposure || 'deferred' };
    if (connection.transport === 'stdio') {
      config.type = 'stdio';
      config.command = connection.command.argv[0];
      config.args = connection.command.argv.slice(1);
      config.cwd = connection.command.working_dir || '/workspace';
      config.env = { ...service.environment };
      for (const [target, alias] of Object.entries(service.secret_environment || {})) {
        if (!environment[alias]) throw new Error(`MCP ${name}: authentication: missing binding ${alias}`);
        config.env[target] = '${' + alias + '}';
      }
    } else if (connection.transport === 'streamable-http') {
      config.type = 'streamable-http';
      config.url = connection.url;
      config.headers = { ...service.headers };
      for (const [header, binding] of Object.entries(service.secret_headers || {})) {
        if (!environment[binding.environment]) throw new Error(`MCP ${name}: authentication: missing binding ${binding.environment}`);
        config.headers[header] = (binding.prefix || '') + '${' + binding.environment + '}';
      }
    } else throw new Error(`MCP ${name}: configuration: unsupported transport ${connection.transport}`);
    const allow = allowedTools(service, tool.mcp);
    if (allow) {
      config.toolExposure = Object.fromEntries(allow.map(name => [name, config.exposure]));
      config.toolExposure['*'] = 'hidden';
      config.exposure = 'hidden';
    }
    servers[name] = config;
  }
  return servers;
}

export async function loadMCPClient() {
  return import(clientLibrary);
}

// A bounded initialize/list-tools probe fails before a model spends tokens on an
// unavailable service. It never invokes tools or launches browser OAuth flows.
// Pi owns subsequent session connections; external endpoints remain external.
export async function verifyMCPs(agent, servers, environment, progress, library) {
  if (!Object.keys(agent.mcp_services || {}).length) return;
  library ||= await loadMCPClient();
  for (const [name, service] of Object.entries(agent.mcp_services)) {
    const config = servers[name];
    if (!config) throw new Error(`MCP ${name}: configuration: service is not selected`);
    const client = new library.McpClient({ name: 'watts-readiness', version: '0.1.0', requestTimeoutMs: config.timeout * 1000,
      roots: [{ uri: 'file:///workspace', name: 'workspace' }] });
    let timer, closing, cancelled = false;
    const close = () => closing ||= client.close();
    const abort = () => { cancelled = true; void close(); };
    process.once('SIGTERM', abort); process.once('SIGINT', abort);
    try {
      const resolveBindings = values => Object.fromEntries(Object.entries(values || {}).map(([key, value]) =>
        [key, value.replace(/\$\{([A-Z_][A-Z0-9_]*)\}/g, (_, alias) => environment[alias] || '')]));
      const transport = config.type === 'stdio'
        ? new library.StdioTransport({ command: config.command, args: config.args, cwd: config.cwd, env: { ...environment, ...resolveBindings(config.env) }, inheritEnv: false, stderr: 'pipe', closeTimeoutMs: 1000 })
        : new library.StreamableHttpTransport({ url: config.url, headers: resolveBindings(config.headers), openGetStream: false,
          fetch: (url, options = {}) => fetch(url, { ...options, signal: AbortSignal.any([
            ...(options.signal ? [options.signal] : []), AbortSignal.timeout(config.timeout * 1000),
          ]) }),
        });
      const probe = async () => {
        await client.connect(transport);
        const tools = await client.listTools();
        const selection = agent.tools.find(tool => tool.mcp?.ref === name).mcp;
        const missing = (allowedTools(service, selection) || []).filter(name => !tools.some(tool => tool.name === name));
        if (missing.length) throw new Error(`unavailable_tools: server does not advertise ${missing.join(', ')}`);
        progress('progress', `MCP ${name}: ready (${tools.length} advertised tools)`);
      };
      await Promise.race([probe(), new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error('timeout: initialization/discovery deadline exceeded')), config.timeout * 1000);
      })]);
    } catch (error) {
      let detail = error.message || String(error);
      for (const alias of [...Object.values(service.secret_environment || {}), ...Object.values(service.secret_headers || {}).map(binding => binding.environment)]) {
        if (environment[alias]) detail = detail.replaceAll(environment[alias], '[redacted]');
      }
      const category = cancelled ? 'cancelled' : [401, 403].includes(error.status) ? 'authentication' : error.name === 'McpTimeoutError' ? 'timeout' : /^(timeout|unavailable_tools):/.test(detail) ? '' : 'connection';
      throw new Error(`MCP ${name}: ${category ? category + ': ' : ''}${detail}`);
    } finally {
      clearTimeout(timer);
      process.removeListener('SIGTERM', abort); process.removeListener('SIGINT', abort);
      await close();
    }
    if (cancelled) throw new Error(`MCP ${name}: cancelled: readiness interrupted`);
  }
}
