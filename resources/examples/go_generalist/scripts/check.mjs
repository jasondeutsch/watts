import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const [check, task] = process.argv.slice(2);
const read = name => readFileSync(join(task, name), 'utf8');
const fail = message => { throw new Error(message); };
try {
  switch (check) {
    case 'spec': {
      const spec = read('SPEC.md');
      if (!/^## Acceptance criteria\s*$/m.test(spec) || !/AC-\d+:\s*\S/m.test(spec)) fail('SPEC.md needs concrete, numbered acceptance criteria');
      if (!/^## Explicit non-goals\s*$/m.test(spec)) fail('SPEC.md needs explicit non-goals');
      if (/<name>|<0 \/ 1 \/ 2|AC-\d+:\s*$/m.test(spec)) fail('SPEC.md still contains unfilled scaffold fields');
      break;
    }
    case 'plan': {
      const plan = read('PLAN.md');
      if (!/^### US-\d+:/m.test(plan)) fail('PLAN.md needs sequenced stories with US-### identifiers');
      if (!/golangci|scripts\/lint\.sh/.test(plan)) fail('PLAN.md must include the team lint gate');
      for (const criterion of read('SPEC.md').match(/AC-\d+/g) || []) if (!plan.includes(criterion)) fail(`PLAN.md does not cover ${criterion}`);
      break;
    }
    case 'stories': {
      const stories = [...read('PLAN.md').matchAll(/^### (US-\d+):/gm)].map(match => match[1]);
      if (!stories.length) fail('PLAN.md has no stories');
      const log = read('STORY_LOG.md');
      const remaining = stories.filter(story => !new RegExp(`^DONE ${story}(?:\\s.*)?$`, 'm').test(log));
      if (remaining.length) fail(`Stories still remaining: ${remaining.join(', ')}`);
      break;
    }
    case 'verdict':
      if (!/VERDICT: (PASS|REJECT)\s*$/.test(read('review/VERDICT.md'))) fail('Review must end with VERDICT: PASS or VERDICT: REJECT');
      break;
    default: fail(`Unknown check: ${check}`);
  }
  console.log(`${check}: OK`);
} catch (error) {
  console.error(`${check}: FAILED: ${error.message}`);
  process.exitCode = 1;
}
