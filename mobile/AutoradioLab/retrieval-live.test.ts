// The live search pass may download its model only on the search group's own agreement (review, 2026-11-24, finding 2): a voice press must leave it false.
import { beforeEach, expect, test } from 'bun:test';
import { live } from './retrieval-live.ts';
import { consentAgree, forgetConsent } from './consent.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
  stat: async (p: string) => { if (!files.has(p)) throw Object.assign(new Error('ENOENT'), { code: 'ENOENT' }); return {}; },
} } as never;
beforeEach(() => { files.clear(); forgetConsent(); live.storage = storage; });

test('R1 the search pass downloads only after the search group was agreed to, not after any other group', async () => {
  expect(await live.mayDownload()).toEqual({ ok: false, mobile: false });
  await consentAgree('voice', true, storage); await consentAgree('voice-rest', false, storage);
  expect(await live.mayDownload()).toEqual({ ok: false, mobile: false });
  await consentAgree('search', false, storage);
  expect(await live.mayDownload()).toEqual({ ok: true, mobile: false });
});

// The Test button of the search model runs in shared code, where a data source may not read the clock (the bake refuses Date.now()): the time is the one the device told us (clock.ts), passed in as the ask's `now`.
test('R2 the search trial asks with the device clock as its time and never reads the clock itself', async () => {
  const { testSearch } = await import('./search-kick.ts'); const { clock } = await import('./clock.ts'); (await import('./api.ts')).resetSeam();         // no native side: the corpus only
  const asked: number[] = []; const real = live.serve.bind(live);
  (live as { serve: unknown }).serve = async (a: { now: number }) => { asked.push(a.now); return { text: '', items: [{}, {}], record: {}, brief: '' }; };
  const realNow = Date.now; Date.now = () => { throw new Error('the clock was read'); };
  clock.ms = 424242;
  try { expect(await testSearch('anything', storage)).toContain('2 stories and documents'); } finally { Date.now = realNow; (live as { serve: unknown }).serve = real; clock.ms = 0; }
  expect(asked).toEqual([424242]);
});

// The search Test must exercise the embedding model itself and the real corpus wiring (review 2026-11-24): the runtime's storage is the page's, the native embedder is bound, and the query is embedded.
test('R3 the search trial wires the runtime to the page\'s storage and embeds the query with the native model', async () => {
  const { testSearch } = await import('./search-kick.ts'); const { bindNative, resetSeam } = await import('./api.ts');
  const asked: unknown[] = []; resetSeam();
  bindNative({ later: async (r) => { asked.push(r); if (r.op === 'embedState') return { present: true, loaded: true }; if (r.op === 'embed') return { vectors: [[0.1, 0.2, 0.3]] }; return {}; } });
  const real = live.serve.bind(live); live.storage = undefined; live.native = null;
  (live as { serve: unknown }).serve = async () => ({ text: '', items: [{}], record: {}, brief: '' });
  try {
    const line = await testSearch('anything', storage);
    expect(live.storage).toBe(storage);
    expect(asked.some((r) => (r as { op?: string }).op === 'embed')).toBe(true);
    expect(line).toContain('3 numbers');                                                  // the model answered: its vector's size is in the line
  } finally { (live as { serve: unknown }).serve = real; resetSeam(); }
});

// Seat 2 round 6, S1 and N1: the model is ensured (loaded, never downloaded) before it is asked; an absent model is said to be absent, not "did not answer"; a trial whose account left runs nothing further.
test('R4 the search trial loads the embedder before it embeds, and never downloads it', async () => {
  const { testSearch } = await import('./search-kick.ts'); const { bindNative, resetSeam } = await import('./api.ts');
  const ops: Record<string, unknown>[] = []; resetSeam();
  bindNative({ later: async (r) => { ops.push(r); if (r.op === 'embedState') return { present: true, loaded: false }; if (r.op === 'embedEnsure') return { ok: true }; if (r.op === 'embed') return { vectors: [[1, 2]] }; return {}; } });
  const real = live.serve.bind(live); (live as { serve: unknown }).serve = async () => ({ text: '', items: [], record: {}, brief: '' });
  try {
    const line = await testSearch('q', storage);
    const names = ops.map((o) => o.op); expect(names.indexOf('embedEnsure')).toBeGreaterThanOrEqual(0); expect(names.indexOf('embedEnsure')).toBeLessThan(names.indexOf('embed'));
    expect(ops.find((o) => o.op === 'embedEnsure')).toMatchObject({ download: false });
    expect(line).toContain('2 numbers');
  } finally { (live as { serve: unknown }).serve = real; resetSeam(); }
});
test('R4b an embedder that is not on the phone is said to be absent and nothing is embedded; one that will not load says so', async () => {
  const { testSearch } = await import('./search-kick.ts'); const { bindNative, resetSeam } = await import('./api.ts');
  for (const [present, ok, want] of [[false, true, 'not on this phone'], [true, false, 'did not load']] as const) {
    const ops: string[] = []; resetSeam();
    bindNative({ later: async (r) => { ops.push(String(r.op)); if (r.op === 'embedState') return { present, loaded: false }; if (r.op === 'embedEnsure') return { ok }; return { vectors: [[1]] }; } });
    const line = await testSearch('q', storage); expect(line).toContain(want); expect(ops).not.toContain('embed');
  }
  resetSeam();
});
test('R4c a trial whose account left while it ran says nothing and asks no further', async () => {
  const { testSearch } = await import('./search-kick.ts'); const { bindNative, resetSeam, nextGeneration } = await import('./api.ts');
  const ops: string[] = []; resetSeam();
  bindNative({ later: async (r) => { ops.push(String(r.op)); if (r.op === 'embedState') return { present: true, loaded: false }; if (r.op === 'embedEnsure') { nextGeneration(); return { ok: true }; } return { vectors: [[1]] }; } });
  const real = live.serve.bind(live); let served = 0; (live as { serve: unknown }).serve = async () => { served += 1; return { text: '', items: [], record: {}, brief: '' }; };
  try { expect(await testSearch('q', storage)).toBe(''); expect(ops).not.toContain('embed'); expect(served).toBe(0); } finally { (live as { serve: unknown }).serve = real; resetSeam(); }
});
