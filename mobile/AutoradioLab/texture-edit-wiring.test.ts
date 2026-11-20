// The editor's wiring, end to end at the contract level (review 2026-11-20, item 5). WHY NOT A REAL-PROOF STEP: mobile/scripts/proof-real.sh is a reads-only drive of the d@e computer on staging
// ("taps nothing that writes or spends"); an edit there is a revision written to a real document, so no editing step is added to it. What can be proved without a write: (1) the sources as the contract calls them, through app.ts `answer`, against a FAKE
// server (a stubbed `fetch` and a stubbed native reader, as api.test.ts does): typing, the pause's flush, leaving the page, the native "went to the background" count and the visibility fact. The real
// phone's lock and unlock, and a real post, are the queue's real proof with an owner-approved write (not done here).
//  (The contract's own text is not asserted here: texture-edit.test.contract proves the source is asked from a non-document page; the pause and retry are the contract's `task`s, exercised below through the sources they call.)
//  W2 through `answer`: type, see "dirty", flush as the pause's task does, see "Saved · version 2" and the one native `textureSaved`; leave the page (another address) posts; the native resign count posts;
//     the visibility fact posts; every post carries the head as its parent.
import { beforeEach, describe, expect, test } from 'bun:test';
import { answer } from './app.ts';
import { resetSeam, bindNative } from './api.ts';
import { resetTextureEdit } from './texture-edit.ts';

type Info = { status: string; label: string; address: string; editable: boolean };
const A = 'texture:d1';
let resigns = 0; let ops: string[] = []; let posts: Record<string, unknown>[] = [];
let head = { id: 'r1', content: 'Hello', n: 0 };
const native = { available: true, later: async (r: { op: string }) => {
  ops.push(r.op);
  if (r.op === 'session') return { found: true, value: 'test-key-not-real', computerId: 'c1' };
  if (r.op === 'keystate') return { state: 'present' };
  if (r.op === 'textureResigned') return { count: resigns };
  if (r.op === 'textureOldPending') return { found: false };
  return null;
} };
const json = (o: unknown) => new Response(JSON.stringify(o), { status: 200, headers: { 'content-type': 'application/json' } });
beforeEach(() => {
  resetSeam(); resetTextureEdit(); resigns = 0; ops = []; posts = []; head = { id: 'r1', content: 'Hello', n: 0 };
  bindNative(native as never);
  globalThis.fetch = (async (u: string, init: RequestInit) => {
    const path = new URL(String(u)).pathname;
    if (init.method === 'POST' && path === '/api/texture/documents/d1/revisions') {
      const b = JSON.parse(String(init.body)); posts.push(b);
      head = { id: `r${posts.length + 1}`, content: b.content, n: head.n + 1 };
      return json({ revision_id: head.id, version_number: head.n });
    }
    if (path === '/api/texture/documents/d1') return json({ doc_id: 'd1', current_revision_id: head.id });
    if (path.startsWith('/api/texture/revisions/')) return json({ revision_id: head.id, version_number: head.n, content: head.content, body_doc: null });
    return new Response('{}', { status: 404 });
  }) as never;
});
const ask = (source: string, args: unknown[]) => answer(source as never, args as never, undefined as never, undefined as never, native as never) as Promise<Info>;

describe('the editor wiring', () => {
  test('W2 typing, the pause, leaving the page and the background, through app.ts against a fake server', async () => {
    expect((await ask('textureEditInfo', [A, 'visible'])).editable).toBe(true);
    expect((await ask('textureEdit', [A, 'Hello there'])).status).toBe('dirty');
    expect((await ask('textureEditInfo', [A, 'visible'])).status).toBe('dirty'); expect(posts.length).toBe(0);      // nothing posts on its own: the contract's pause does
    const done = await ask('textureFlush', [A]);                                                                     // what `edSave` sends after 2000 ms
    expect(done.status).toBe('saved'); expect(done.label).toBe('Saved · version 2'); expect(posts.length).toBe(1);
    expect(posts[0].parent_revision_id).toBe('r1');
    expect(ops.filter((o) => o === 'textureSaved').length).toBe(1);                                                   // the Swift reader and content store are told once
    await ask('textureEdit', [A, 'Hello there again']);
    expect((await ask('textureEditInfo', ['paper:front', 'visible'])).editable).toBe(false);                         // leaving the page posts
    expect(posts.length).toBe(2);
    await ask('textureEdit', [A, 'Third']);
    resigns = 1;                                                                                                      // the native side counted a willResignActive
    expect((await ask('textureEditInfo', [A, 'visible'])).label).toBe('Saved · version 4'); expect(posts.length).toBe(3);
    await ask('textureEdit', [A, 'Fourth']);
    expect((await ask('textureEditInfo', [A, 'hidden'])).label).toBe('Saved · version 5');                           // the web's own fact still flushes
    expect(posts.map((p) => p.parent_revision_id)).toEqual(['r1', 'r2', 'r3', 'r4']);
  });
  test('G2 the launch retry and the 30 s retry run from the radio page, through app.ts', async () => {
    const files = new Map<string, Uint8Array>();
    const storage = { fs: { readFile: async (p: string) => { const f = files.get(p); if (!f) throw new Error('ENOENT'); return f; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); },
      stat: async (p: string) => { if (!files.has(p)) throw new Error('ENOENT'); return {}; } } };
    const askS = (source: string, args: unknown[]) => answer(source as never, args as never, undefined as never, storage as never, native as never) as Promise<Info>;
    await askS('textureEditInfo', [A, 'visible']);                                                                   // the document is opened: its head is read
    const real = globalThis.fetch; globalThis.fetch = (async () => { throw new TypeError('Network request failed'); }) as never;
    await askS('textureEdit', [A, 'Typed with no route']);                                                          // typed on the train: kept with its base
    resetTextureEdit(); globalThis.fetch = real;                                                                      // the app is killed; the route is back at the next launch
    const i = await askS('textureEditInfo', ['radio:', 'visible']);                                                  // the first page of the launch is the radio, not a document
    expect(posts.length).toBe(1); expect(posts[0].content).toBe('Typed with no route'); expect(i.waiting).toBe(0);
    // and what edRetryGo sends every 30 s: textureFlush("") from any page
    globalThis.fetch = (async () => { throw new TypeError('Network request failed'); }) as never;
    await askS('textureEdit', [A, 'Typed again']);
    expect((await askS('textureEditInfo', ['radio:', 'visible'])).waiting).toBe(1);                                   // leaving the page tried; the route is still down
    globalThis.fetch = real;
    const g = await askS('textureFlush', ['']);
    expect(posts.length).toBe(2); expect(g.waiting).toBe(0);
  });
});
