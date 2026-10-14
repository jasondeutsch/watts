// Image-owned protection for tool results before they enter model context.
// Values remain in memory. This does not recognize unknown/obfuscated secrets.
import { Transform } from 'node:stream';
import { StringDecoder } from 'node:string_decoder';

export const marker = '[REDACTED]';
export function redactor(environment = process.env) {
  const names = JSON.parse(environment.WATTS_SECRET_ENV_NAMES || '[]');
  const variants = new Set();
  for (const name of names) {
    const value = environment[name];
    if (!value) continue;
    variants.add(value);
    let encoded = value;
    for (let depth = 0; depth < 3; depth++) {
      encoded = JSON.stringify(encoded).slice(1, -1);
      variants.add(encoded);
    }
    variants.add(encodeURIComponent(value));
    variants.add(encodeURIComponent(value).replaceAll('%20', '+'));
  }
  const values = [...variants].sort((a, b) => b.length - a.length);
  const pattern = values.length ? new RegExp(values.map(value => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|'), 'g') : null;
  const text = value => pattern ? String(value).replace(pattern, marker) : String(value);
  function data(value) {
    if (typeof value === 'string') return text(value);
    if (Array.isArray(value)) return value.map(data);
    if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, item]) => [text(key), data(item)]));
    return value;
  }
  function stream() {
    let pending = '';
    const decoder = new StringDecoder('utf8');
    function consume(final) {
      let safe = '';
      while (pending) {
        if (!final && values.some(value => value.length > pending.length && value.startsWith(pending))) break;
        const match = values.find(value => pending.startsWith(value));
        safe += match ? marker : pending[0];
        pending = pending.slice(match ? match.length : 1);
      }
      return safe;
    }
    return new Transform({
      transform(chunk, _encoding, done) { pending += decoder.write(chunk); done(null, consume(false)); },
      flush(done) { pending += decoder.end(); done(null, consume(true)); },
    });
  }
  return { text, data, stream };
}
