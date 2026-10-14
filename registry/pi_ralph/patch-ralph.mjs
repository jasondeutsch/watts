// Patches are image-owned, exact, and fail the build if the pinned upstream
// implementation changes. Watts core has no knowledge of these integrations.
import { readFileSync, writeFileSync } from 'node:fs';
const directory = '/usr/local/lib/node_modules/@lnilluv/pi-ralph-loop/src';
function replace(file, before, after) {
  const path = `${directory}/${file}`;
  const source = readFileSync(path, 'utf8');
  if (!source.includes(before)) throw new Error(`Unsupported Ralph source: ${file}`);
  writeFileSync(path, source.replace(before, after));
}
replace('runner.ts', 'const registryRoots = cwd === taskDir ? [taskDir] : [taskDir, cwd];', 'const registryRoots = process.env.PI_WATTS_RALPH_REGISTRY ? [taskDir, process.env.PI_WATTS_RALPH_REGISTRY] : (cwd === taskDir ? [taskDir] : [taskDir, cwd]);');
replace('runner-rpc.ts', '["--mode", "rpc", "--no-session", "--no-extensions", "-e", extensionPath]', '["--mode", "rpc", "--no-session", "--no-extensions", "--no-context-files", "--no-approve", "--offline", "-e", extensionPath, "-e", "/opt/watts/redaction.ts", "-e", "/opt/watts/telemetry.ts", "-e", "/opt/watts/tools.ts", ...JSON.parse(process.env.PI_WATTS_SKILL_ARGS ?? "[]")]');
