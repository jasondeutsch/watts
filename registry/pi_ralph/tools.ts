import { readFileSync } from 'node:fs';
import { runCommandTool } from './tools.mjs';

// Native tool exposure belongs to the image, not Watts orchestration.
export default function tools(pi) {
  const input = JSON.parse(readFileSync(process.env.WATTS_INPUT, 'utf8'));
  for (const tool of input.agent?.tools || []) {
    if (!tool.command) continue; // MCP services use Pi's native MCP client.
    pi.registerTool({
      name: tool.name,
      label: tool.name,
      description: tool.description || `Run the declared ${tool.name} command.`,
      promptSnippet: tool.description || `Run ${tool.name}.`,
      parameters: { type: 'object', properties: {}, additionalProperties: false },
      executionMode: 'sequential',
      async execute(_id, _parameters, signal) {
        const result = await runCommandTool(tool, { signal });
        return { content: [{ type: 'text', text: JSON.stringify(result) }], details: result, isError: result.status !== 'exited' || result.exit_code !== 0 };
      },
    });
  }
}
