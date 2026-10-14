import { redactor } from './redact.mjs';
import { appendFileSync } from 'node:fs';
import { messageEvent, usageEvent } from './protocol.mjs';

export default function telemetry(pi) {
  const secrets = redactor();
  pi.on('message_end', (event, context) => {
    const session = context.sessionManager.getSessionId();
    const costKnown = process.env.PI_WATTS_COST_KNOWN === '1';
    for (const record of [usageEvent(event.message, session, costKnown), messageEvent(event.message, session, costKnown)]) {
      if (record) appendFileSync(process.env.WATTS_EVENTS, JSON.stringify(secrets.data(record)) + '\n', { mode: 0o600 });
    }
  });
}
