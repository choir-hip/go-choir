// The voice turn's candidate stories (the paper's list the desk may add to the queue by voice) come from shared code: the Swift ChoirApp.loadStories asked the front
// route itself, the last non-voice server call but the Texture ones. Failure list, written before the code:
//  V1 a front list that loads is handed to the voice: the app module's `voiceFront` op, the account generation, the first 40 stories as {route, title}.
//  V2 the offline copy of the list is handed too (a voice turn offline still names the paper's stories).
//  V3 a failed list with no copy hands nothing; no native module hands nothing and never throws.
//  V4 `retrievalServe` (the voice turn's ask) makes sure the list was handed even when the paper page was never opened this launch.
import { beforeEach, describe, expect, test } from 'bun:test';
import { paperFront, forget, handVoiceFront, CACHE_FILE } from './paper.ts';
import { adoptNative, resetSeam, accountGeneration } from './api.ts';
import { retrievalServe } from './retrieval-live.ts';

type Req = { op: string; gen?: number; stories?: { route: string; title: string }[] };
let sent: Req[] = [];
const files = new Map<string, Uint8Array>();
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  },
} as never;
const st = (i: number) => ({ title: `Story ${i}`, route: { bare: `story-${i}` }, section: 'world', sections: ['world'], importance: 5 });
const front = (n: number) => ({ stories: Array.from({ length: n }, (_, i) => st(i)), sections: [{ id: 'world', label: 'World' }] });
let reply: () => Response;
beforeEach(() => {
  forget(); resetSeam(); sent = []; files.clear();
  adoptNative({ available: true, later: async (r: Req) => { sent.push(r); return { ok: true }; } } as never, 'test');
  reply = () => new Response(JSON.stringify(front(3)), { status: 200 });
  globalThis.fetch = (async () => reply()) as typeof fetch;
});
const args = { view: 'front', width: 393, tick: 0, focus: 'paper', picks: '' };
const handed = () => sent.filter((r) => r.op === 'voiceFront');

describe('the voice is handed the front list by shared code', () => {
  test('V1 a loaded list is handed, with the generation, as route and title', async () => {
    await paperFront(args, storage);
    await handVoiceFront(storage);
    const h = handed();
    expect(h.length).toBeGreaterThan(0);
    const last = h[h.length - 1];
    expect(last.gen).toBe(accountGeneration());
    expect(last.stories).toEqual([0, 1, 2].map((i) => ({ route: `story-${i}`, title: `Story ${i}` })));
  });
  test('V1b at most 40 stories', async () => {
    reply = () => new Response(JSON.stringify(front(90)), { status: 200 });
    await handVoiceFront(storage);
    expect(handed()[0].stories!.length).toBe(40);
    expect(handed()[0].stories![0].route).toBe('story-0');
  });
  test('V2 the offline copy is handed', async () => {
    await paperFront(args, storage);
    expect(files.has(CACHE_FILE)).toBe(true);
    forget(); sent = [];
    reply = () => new Response('no', { status: 500 });
    await handVoiceFront(storage);
    expect(handed()[0].stories!.length).toBe(3);
  });
  test('V3 a failed list with no copy hands nothing; no native hands nothing and does not throw', async () => {
    reply = () => new Response('no', { status: 500 });
    await handVoiceFront(storage);
    expect(handed()).toEqual([]);
    resetSeam(); forget();
    reply = () => new Response(JSON.stringify(front(3)), { status: 200 });
    await handVoiceFront(storage);
    expect(sent.length).toBe(0);
  });
  test('V4 the voice turn\'s ask hands the list even though the paper page was never opened', async () => {
    const out = await retrievalServe({ id: 'a1', words: 'what is new', pageTitle: '', pageAddress: '', now: Date.now(), facts: '{}' }, storage);
    expect(out.id).toBe('a1');
    expect(handed().length).toBe(1);
    expect(handed()[0].stories!.length).toBe(3);
  });
});
