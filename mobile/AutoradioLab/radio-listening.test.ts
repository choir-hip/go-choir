// Listening only (radio-writer.ts `mode`): with no account, or no consent to outside AI, the radio plays the paper's own stories in the paper's order and sends nothing to a model.
// The failure list, written first:
//  L1 mode 'listening': the first ask places the paper's front stories (public routes, `pub:`), placed by the phone, in the paper's weight order, at most LISTEN_BATCH; no desk request, no story text fetch, no notice.
//  L2 the next ask skips what was told and goes on to the stories after it; with all told it begins the list again (the paper is a loop, not a dead end).
//  L3 mode 'desk' is the old behaviour: the desk is posted to; and a mode that throws is 'desk' (never a silent downgrade of a signed-in listener to the paper's order by a failed read).
//  L4 an empty front says nothing and places nothing (the opening says it has no front page).
import { beforeEach, describe, expect, test } from 'bun:test';
import { Writer, LISTEN_BATCH, type WriterDeps } from './radio-writer.ts';
import { RadioRuntime } from './radio-runtime.ts';
import type { DeckPort, DeckState } from './deck.ts';

const NOW = new Date('2023-11-14T22:00:00Z');
const row = (route: string, importance: number) => ({ route, title: `Title ${route}`, dek: `Dek of ${route}.`, importance, sources: 3, version_count: 2, updated_at: '2023-11-14T20:00:00Z' });
const EMPTY: DeckState = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };
const deck: DeckPort = { state: async () => EMPTY, play: async () => EMPTY, enqueue: async () => EMPTY, act: async () => EMPTY, ack: async () => EMPTY, wipe: async () => EMPTY };
let rt: RadioRuntime, w: Writer, posts: number, textCalls: number, fr: unknown, mode: () => Promise<'desk' | 'listening'>;
const settle = async () => { await w.settle(); await new Promise((r) => setTimeout(r, 0)); await w.settle(); };
const stories = () => rt.store.sheet.entries.filter((e) => e.kind === 'page').map((e) => e.source?.ref);
beforeEach(() => {
  rt = new RadioRuntime({ deck, now: () => NOW }); posts = 0; textCalls = 0; mode = async () => 'listening';
  fr = { stories: Array.from({ length: 12 }, (_, i) => row(`/pub/s${i}`, 20 - i)) };
  const d: WriterDeps = {
    rt, front: async () => fr, texts: async () => { textCalls += 1; return {}; }, takes: async () => ({ takes: [], kept: { ids: [], handle: '', muted: [] } }), edition: async () => '',
    post: async () => { posts += 1; return 'event: done\ndata: {}\n\n'; }, now: () => NOW, tz: () => 'UTC', requestId: () => 'r', generation: () => 1, mode: () => mode(),
  };
  w = new Writer(d);
});

describe('listening only', () => {
  test('L1 the paper\'s own stories in the paper\'s order, nothing sent to a model, nothing said', async () => {
    w.open(); await settle();
    expect(stories()).toEqual(['pub:/pub/s0', 'pub:/pub/s1', 'pub:/pub/s2', 'pub:/pub/s3'].slice(0, LISTEN_BATCH));
    expect(rt.store.sheet.entries.every((e) => e.placed_by === 'phone')).toBe(true);
    expect(posts).toBe(0); expect(textCalls).toBe(0); expect(w.notice).toBe('');
  });
  test('L2 the next ask goes on after what was told; all told begins again', async () => {
    w.open(); await settle();
    const first = stories(); for (const e of rt.store.sheet.entries) e.state = 'played'; w.listenerActed();
    w.checkAhead(); await settle();
    const refs = stories(); expect(first.length).toBe(LISTEN_BATCH); expect(refs.slice(LISTEN_BATCH, LISTEN_BATCH * 2)).toEqual(Array.from({ length: LISTEN_BATCH }, (_, i) => `pub:/pub/s${LISTEN_BATCH + i}`));
    fr = { stories: [row('/pub/only', 5)] };
    const played = () => { for (const e of rt.store.sheet.entries) e.state = 'played'; };
    played(); w.listenerActed(); w.checkAhead(); await settle(); played(); w.listenerActed(); w.checkAhead(); await settle();
    expect(stories().filter((r) => r === 'pub:/pub/only').length).toBeGreaterThan(0);          // the list had begun again
  });
  test('L3 the desk is posted to when the mode says desk, and when it cannot say', async () => {
    mode = async () => 'desk'; w.open(); await settle(); expect(posts).toBe(1);
    posts = 0; mode = async () => { throw new Error('keystate failed'); }; w.listenerActed(); w.checkAhead(); await settle(); expect(posts).toBe(1);
  });
  test('L4 an empty front places nothing and says nothing', async () => {
    fr = { stories: [] }; w.open(); await settle();
    expect(rt.store.sheet.entries.length).toBe(0); expect(w.notice).toBe('');
  });
});
