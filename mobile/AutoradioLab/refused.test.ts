// Exact's refusals of work whose answer is gone, word for word from the pinned Exact source (mobile/scripts/setup-exact.sh pins it; EXACT2 or ../.exact-src). Each must be a refusal for refused.ts, and
// nothing a server, a disk or the network says may be. When the pinned source is here, each string is looked up in its file: a string Exact no longer writes fails here, loudly, before a refusal starts
// counting as a failure again (docs/problems/radio-restore-empty-after-let-go-2026-11-25.md).
import { describe, expect, test } from 'bun:test';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { refusedHere } from './refused.ts';

const REFUSALS: { file: string; text: string; message: string }[] = [
  { file: 'js/src/fetch.js', text: 'the answer was let go before this fetch', message: 'the answer was let go before this fetch' },
  { file: 'js/src/prelude.js', text: 'the answer was let go before this reply; the request may already have been sent', message: 'the answer was let go before this reply; the request may already have been sent' },
  { file: 'js/src/prelude.js', text: '"the answer was let go", "AbortError"', message: 'the answer was let go' },
  { file: 'js/src/fetch.js', text: 'fetch() called outside an answer: it was never run. Start a fetch inside an answer', message: "fetch() called outside an answer: it was never run. Start a fetch inside an answer, or in an error event's listener itself" },
  { file: 'js/src/fetch.js', text: 'fetch() called from background work: it was never run. Fetch inside an answer', message: 'fetch() called from background work: it was never run. Fetch inside an answer' },
  { file: 'js/src/fetch.js', text: "fetch() called from a plain .then callback a let-go answer's request settled: it was never run", message: "fetch() called from a plain .then callback a let-go answer's request settled: it was never run. Await that result in an async function, which resumes in its own answer's turn" },
  { file: 'js/src/prelude.js', text: 'native.call() called from background work: it was never run. Call it inside an answer', message: 'native.call() called from background work: it was never run. Call it inside an answer' },
  { file: 'js/src/storage.js', text: ': it was never run. Start storage inside an answer', message: 'storage.readFile() called outside an answer: it was never run. Start storage inside an answer' },
  { file: 'js/src/storage.js', text: "the answer's storage wait failed", message: "the answer's storage wait failed" },
];
const NOT_REFUSALS = ['No such file or directory (os error 2)', 'Input/output error (os error 5)', 'denied: app:/data/runsheet.json is not granted', 'network: timeout', 'network: request failed', '/api/autoradio/sheet/ops answered 503', 'the key was refused; sign in again'];

describe('refusedHere knows every refusal Exact writes, and nothing else', () => {
  test('each refusal, bare and as api.ts wraps it ("network: ...")', () => {
    for (const r of REFUSALS) {
      expect(refusedHere(new Error(r.message)), r.message).toBe(true);
      expect(refusedHere(new Error('network: ' + r.message)), 'wrapped: ' + r.message).toBe(true);
    }
  });
  test('what a server, a disk or the network says is not a refusal', () => {
    for (const m of NOT_REFUSALS) expect(refusedHere(new Error(m)), m).toBe(false);
  });
  const root = resolve(import.meta.dir, process.env.EXACT2 ?? '../.exact-src');
  test.skipIf(!existsSync(resolve(root, 'js/src/prelude.js')))('the pinned Exact source still writes each string', () => {
    for (const r of REFUSALS) expect(readFileSync(resolve(root, r.file), 'utf8').includes(r.text), `${r.file} no longer writes: ${r.text}`).toBe(true);
  });
});
