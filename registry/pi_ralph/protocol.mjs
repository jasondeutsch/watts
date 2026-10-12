import { createHash } from 'node:crypto';

// Pi reports uncached input and cache counters separately. The common contract
// reports all input tokens, with cache counters as a breakdown of that input.
export function messageEvent(message, session = '', costKnown = false) {
  if (message?.role !== 'assistant') return null;
  const id = createHash('sha256').update(JSON.stringify({ session, message })).digest('hex');
  if (message.stopReason === 'error' || message.stopReason === 'aborted') {
    return { id, type: 'error', message: message.errorMessage || `Pi stopped: ${message.stopReason}` };
  }
  return { id, type: 'progress', message: 'Pi completed an assistant turn' };
}
export function usageEvent(message, session = '', costKnown = false) {
  if (message?.role !== 'assistant' || !message.usage) return null;
  const usage = message.usage;
  if (['error', 'aborted'].includes(message.stopReason) && !(usage.input || usage.output || usage.cacheRead || usage.cacheWrite)) return null;
  const count = name => Number.isSafeInteger(usage[name]) && usage[name] >= 0 ? usage[name] : undefined;
  const input = count('input'), read = count('cacheRead'), write = count('cacheWrite');
  const totalInput = input !== undefined && read !== undefined && write !== undefined ? input + read + write : undefined;
  const id = createHash('sha256').update(JSON.stringify({ session, message })).digest('hex');
  return {
    id: `${id}-usage`, type: 'usage', usage: {
      input_tokens: totalInput,
      output_tokens: count('output'), cache_read_tokens: read, cache_write_tokens: write,
      cost_usd: costKnown && Number.isFinite(usage.cost?.total) ? usage.cost.total : undefined,
      provider: message.provider, model: message.model,
    },
  };
}
