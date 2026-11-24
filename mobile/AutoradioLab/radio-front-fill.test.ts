// The radio never waits, through the bridge (radio-bridge.ts): a launch with nothing but the paper's front page. The opening plays, it ends, and the first front story is read next with no ask of the desk;
// the pulse hands native the paper's list as the turn's `candidates` (it was a Swift story list). Native and the network are fakes; the desk's route is never answered (a desk that is slow or down).
import { afterAll, beforeEach, describe, expect, test } from 'bun:test';
import { adoptNative } from './api.ts';
import { forgetOnboarding } from './onboarding.ts';
import { radioHold, radioPulse, getRadio, wipeRadio } from './radio-bridge.ts';

const realFetch = globalThis.fetch;
afterAll(() => { globalThis.fetch = realFetch; });

const stories = [1, 2, 3, 4].map((n) => ({ title: `Story ${n}`, dek: `Dek of story ${n}.`, route: { bare: `/p/s${n}` }, section: 'world', importance: 9 - n, updated_at: '2026-11-23T10:00:00Z', version_count: 1 }));
type Call = { op: string; name?: string; args?: Record<string, unknown>; [k: string]: unknown };
let calls: Call[], at: number, seq: number, journal: Record<string, unknown>[], current: string, held: string[];
const line = (type: string, item: string, o: Record<string, unknown> = {}): void => { seq += 1; journal.push({ seq, type, item, at: '2026-11-23T10:00:00Z', block: 0, seconds: 0, final: 0, rendered: false, end: '', command: '', reason: '', ...o }); };
const native = {
  available: true,
  later: async (r: Call) => {
    calls.push(r);
    if (r.op === 'radioFacts') return { at, ready: true, turnOpen: false, position: { block: 0, seconds: 0 }, wants: false, foreground: true, inbox: [] };
    if (r.op === 'deck') {
      const a = r.args ?? {};
      if (r.name === 'deckPlay') { current = String(a.id); line('started', current); }
      if (r.name === 'deckAct' && a.op === 'stop') current = '';
      if (r.name === 'deckAck') journal = journal.filter((l) => (l.seq as number) > Number(a.seq));
      return { why: current ? 'playing' : 'empty', item: current, block: 0, seconds: 0, duration: 0, held, seq, acked: 0, journal };
    }
    return { ok: true };
  },
};
beforeEach(async () => {
  forgetOnboarding(); await wipeRadio(undefined); adoptNative(native as never, 'test');
  calls = []; at = 1_700_000_000_000; seq = 0; journal = []; current = ''; held = [];
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const u = String(input);
    if (u.includes('/publications/front')) return new Response(JSON.stringify({ stories }), { status: 200, headers: { 'content-type': 'application/json' } });
    if (u.includes('/publications/resolve') || u.includes('/publications/')) return new Response(JSON.stringify({ publication: { title: 'Full' }, payload: { text: '# Full\n\nWords of the story.' } }), { status: 200 });
    return new Response('no desk here', { status: 503 });
  }) as typeof fetch;
});

describe('F1 a launch with a desk that never answers', () => {
  test('the opening plays, then the paper\'s first story: the radio does not wait; the pulse pushes the paper\'s list as the turn\'s candidates', async () => {
    await radioPulse(undefined, native as never);
    at += 3000;
    await radioHold(undefined, native as never);
    const r = await getRadio(undefined, native as never, false);
    const opening = r.rt.store.onAir();
    expect(opening?.kind).toBe('opening'); expect(opening?.text).toContain('Story 1');
    // the opening ends (the deck says so); nothing was written ahead
    expect(r.rt.store.ahead().filter((e) => e.placed_by !== 'phone').length).toBe(0);
    line('ended', opening!.id, { end: 'finished', rendered: true, seconds: 9 }); current = '';
    await r.rt.pump();
    for (let i = 0; i < 10; i++) await new Promise((res) => setTimeout(res, 0));
    const air = r.rt.store.onAir();
    expect(air?.kind).toBe('page'); expect(air?.placed_by).toBe('phone'); expect(air?.source?.ref).toMatch(/^pub:\/p\/s\d$/);
    const played = calls.filter((c) => c.op === 'deck' && c.name === 'deckPlay').map((c) => String((c.args as { id: string }).id));
    expect(played).toEqual([opening!.id, air!.id]);
    // the paper's list goes to native with the queue snapshot
    await radioPulse(undefined, native as never);
    const set = calls.filter((c) => c.op === 'radioSet' && c.snapshot !== undefined).at(-1) as { candidates?: { route: string; title: string }[] } | undefined;
    expect(set?.candidates?.map((c) => c.route).sort()).toEqual(['/p/s1', '/p/s2', '/p/s3', '/p/s4']);
    expect(set?.candidates?.[0].title).toMatch(/^Story \d$/);
  });
});
