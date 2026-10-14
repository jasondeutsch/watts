import { redactor } from './redact.mjs';

export default function redaction(pi) {
  const secrets = redactor();
  pi.on('tool_result', event => ({
    content: secrets.data(event.content),
    details: secrets.data(event.details),
    ...(event.structuredContent === undefined ? {} : { structuredContent: secrets.data(event.structuredContent) }),
  }));
  // Also covers prior messages and nested/custom tool results at the provider boundary.
  pi.on('context', event => ({ messages: secrets.data(event.messages) }));
}
