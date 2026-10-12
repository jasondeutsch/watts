import { test } from 'node:test';
import assert from 'node:assert/strict';
import { messageEvent, usageEvent } from './protocol.mjs';

test('cache tokens are a breakdown of all input, and unknown cost stays unknown', () => {
  const message = { role: 'assistant', usage: { input: 10, output: 3, cacheRead: 5, cacheWrite: 2, cost: { total: 0 } } };
  assert.equal(usageEvent(message).usage.input_tokens, 17);
  assert.equal(usageEvent(message).usage.output_tokens, 3);
  assert.equal(usageEvent(message).usage.cost_usd, undefined);
  assert.equal(usageEvent(message, '', true).usage.cost_usd, 0);
  assert.equal(usageEvent(message).id, usageEvent(message).id);
  assert.notEqual(usageEvent(message, 'other').id, usageEvent(message).id);
});
test('provider errors are authoritative even when the process exits zero', () => {
  assert.equal(messageEvent({ role: 'assistant', stopReason: 'error', errorMessage: 'rate limited' }).type, 'error');
  assert.equal(messageEvent({ role: 'toolResult', isError: true }), null);
  assert.equal(usageEvent({ role: 'assistant' }), null);
});
