// The Texture editor under bun test (texture-edit.ts). Ported from TextureEditorTests (Swift, deleted with the Swift); the failure list is at the top of texture-edit.ts (1..6).
// The pause in typing is the contract's now (a `task after(2000)` calling `textureFlush`), so the burst test types, checks nothing was posted, then flushes once.
//  E1 a burst of typing is one revision, parent = the head, the untouched paragraph is the original node, label "Saved · version 2".   E2 Done posts at once.   E3 a revision and a story have no editor.
//  E4 nothing changed posts nothing.   E5 offline: kept on the phone from the first keystroke, across a launch, posted once.   E6 typing while posting saves right after.
//  E7 demo: nothing leaves the phone, the page says Saved.   E8 the server's real shapes: a document with a trajectory is saved against its lifecycle (idempotency key, version, head).
//  E9 the words: refused, locked, forbidden, offline, a server error.   E10 sign-out and account switch: nothing of the old account shows, posts or is kept.
//  E14 leaving the page posts what was typed on it.   E11 the phone goes to the background: what is waiting is posted.   E12 the first answer after a launch posts what an earlier launch kept.   E13 after a save the native reader and content store are told once.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as E from './texture-edit.ts';
import { SignedOutError, HTTPError, resetSeam, nextGeneration, type Reply } from './api.ts';
import { WORDS } from './words.ts';

const reply = (body: unknown): Reply => ({ body, serverTime: 0, raw: '' });
const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const f = files.get(p); if (!f) throw new Error('ENOENT'); return f; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
} } as never;

const HEAD_BODY = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', content: [
  { type: 'paragraph', attrs: { id: 'p1' }, content: [{ type: 'text', text: 'Hello world' }] },
  { type: 'paragraph', attrs: { id: 'p2' }, content: [{ type: 'text', text: 'Second' }] }] } };
type Post = { path: string; body: Record<string, unknown> };
/** A fake of the server's routes, with the field names the Go handlers write (internal/textureowner/texture.go). */
function server(o: { trajectory?: string; failure?: unknown; gate?: Promise<void> } = {}) {
  const posts: Post[] = [], calls: string[] = [];
  let head = { id: 'r1', content: 'Hello world\n\nSecond', body: HEAD_BODY as unknown, n: 0 };
  const net: E.Net = { json: async (path, opts) => {
    calls.push(`${opts?.method ?? 'GET'} ${path}`);
    if (opts?.method === 'POST') {
      if (o.failure) throw o.failure;
      if (o.gate && posts.length === 0) await o.gate;
      const b = opts.body as Record<string, unknown>;
      posts.push({ path, body: b });
      head = { id: `r${posts.length + 1}`, content: String(b.content), body: b.body_doc, n: head.n + 1 };
      return reply({ revision_id: head.id, doc_id: 'd1', version_number: head.n, content: head.content });
    }
    if (path === '/api/texture/documents/d1') return reply({ doc_id: 'd1', title: 'Notes', current_revision_id: head.id, current_version_number: head.n, ...(o.trajectory ? { trajectory_id: o.trajectory } : {}) });
    if (path.startsWith('/api/texture/revisions/')) return reply({ revision_id: head.id, doc_id: 'd1', version_number: head.n, content: head.content, body_doc: head.body });
    if (path.startsWith('/api/texture/lifecycle/')) return reply({ trajectory: { lifecycle_version: 7 }, current_document_head: { revision_id: 'life-head' } });
    throw new HTTPError(404, 'nothing');
  } };
  return { net, posts, calls };
}
beforeEach(() => { resetSeam(); E.resetTextureEdit(); files.clear(); });
const A = 'texture:d1';

describe('the editor', () => {
  test('E1 a burst of typing is one revision, posted when flushed', async () => {
    const s = server(), d = { net: s.net, storage };
    for (const t of ['Hello world!', 'Hello world!!', 'Hello world!!!\n\nSecond']) await E.textureEdit(A, t, d);
    expect((await E.textureEditInfo(A, d)).status).toBe('dirty');
    expect(s.posts.length).toBe(0);
    const i = await E.textureFlush(A, d);
    expect(s.posts.length).toBe(1);
    expect(s.posts[0].path).toBe('/api/texture/documents/d1/revisions');
    expect(s.posts[0].body.content).toBe('Hello world!!!\n\nSecond');
    expect(s.posts[0].body.parent_revision_id).toBe('r1');
    expect(i.label).toBe('Saved · version 2');
    const nodes = (s.posts[0].body.body_doc as { doc: { content: { attrs: { id: string } }[] } }).doc.content;
    expect(nodes.length).toBe(2); expect(nodes[1].attrs.id).toBe('p2');
    expect(files.size).toBe(0);                                                        // posted: nothing is kept
  });
  test('E2 Done posts at once', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit(A, 'Changed\n\nSecond', d);
    expect((await E.textureFlush(A, d)).status).toBe('saved'); expect(s.posts.length).toBe(1);
  });
  test('E3 a revision and a story have no editor', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit('texture:d1@r0', 'x', d); await E.textureEdit('pub:story', 'x', d); await E.flushAll(d);
    expect(s.posts.length).toBe(0); expect(s.calls).toEqual([]);
    expect((await E.textureEditInfo('texture:d1@r0', d)).editable).toBe(false);
    expect((await E.textureEditInfo('pub:story', d)).editable).toBe(false);
    expect((await E.textureEditInfo(A, d)).editable).toBe(true);
    expect(E.editableDoc('texture:')).toBeNull(); expect(E.editableDoc('texture:abc')).toBe('abc');
  });
  test('E4 nothing changed posts nothing', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit(A, 'Hello world\n\nSecond', d); await E.textureFlush(A, d);
    expect(s.posts.length).toBe(0); expect(files.size).toBe(0);
  });
  test('E5 offline edits are kept from the first keystroke, across a launch, and posted once', async () => {
    const down = server({ failure: new TypeError('Network request failed') });
    await E.textureEdit(A, 'Kept offline\n\nSecond', { net: down.net, storage });
    expect(files.size).toBe(1);
    const i = await E.textureFlush(A, { net: down.net, storage });
    expect(i.status).toBe('offline'); expect(i.label).toContain('Offline');
    // a relaunch: nothing in memory, the file is read, the route answers, it is posted once
    E.resetTextureEdit();
    const up = server();
    expect((await E.textureEditInfo(A, { net: down.net, storage })).status).toBe('offline');       // while the route is still down: kept
    E.resetTextureEdit();
    const j = await E.textureEditInfo(A, { net: up.net, storage });                               // the first answer after a launch posts it (E12)
    expect(up.posts.length).toBe(1); expect(up.posts[0].body.content).toBe('Kept offline\n\nSecond'); expect(j.status).toBe('saved');
    await E.flushAll({ net: up.net, storage });
    expect(up.posts.length).toBe(1);                                                            // not twice
    expect(files.size).toBe(0);
  });
  test('E5b a head that cannot be read keeps the text and says offline', async () => {
    const net: E.Net = { json: async () => { throw new TypeError('offline'); } };
    const i = await E.textureEdit(A, 'typed with no route', { net, storage });
    expect(i.status).toBe('offline'); expect(i.label).toBe('Edits kept on this phone'); expect(files.size).toBe(1);
  });
  test('E6 typing while it is posting saves right after', async () => {
    let open: () => void = () => undefined;
    const s = server({ gate: new Promise<void>((r) => { open = r; }) }), d = { net: s.net, storage };
    await E.textureEdit(A, 'one\n\nSecond', d);
    const first = E.textureFlush(A, d);                                                         // posting, held at the gate
    await new Promise((r) => setTimeout(r, 20));
    await E.textureEdit(A, 'one two\n\nSecond', d);
    open(); await first;
    expect(s.posts.map((p) => p.body.content)).toEqual(['one\n\nSecond', 'one two\n\nSecond']);
  });
  test('E7 demo posts nothing and says Saved', async () => {
    const s = server();
    const i0 = await E.textureEdit('texture:demo-notes', '# Harbor notes\n\nEdited in the demo.', { net: s.net, storage, demo: true });
    expect(i0.status).toBe('dirty');
    const i = await E.textureFlush('texture:demo-notes', { net: s.net, storage, demo: true });
    expect(s.calls).toEqual([]); expect(i.label.startsWith('Saved · version')).toBe(true);
  });
  test('E8 a document with a trajectory is saved against its lifecycle', async () => {
    const s = server({ trajectory: 'tr-1' }), d = { net: s.net, storage };
    await E.textureEdit(A, 'Changed\n\nSecond', d); await E.textureFlush(A, d);
    const b = s.posts[0].body;
    expect(b.expected_lifecycle_version).toBe(7); expect(b.parent_revision_id).toBe('life-head');
    expect(String(b.idempotency_key)).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    expect(s.calls).toContain('GET /api/texture/lifecycle/tr-1');
    const plain = server(); await E.textureEdit('texture:d1', 'x\n\nSecond', { net: plain.net });
  });
  test('E9 the words', async () => {
    const say = async (failure: unknown) => { E.resetTextureEdit(); const s = server({ failure }); await E.textureEdit(A, 'Different\n\nSecond', { net: s.net, storage }); return E.textureFlush(A, { net: s.net, storage }); };
    expect((await say(new SignedOutError('x', 'refused'))).label).toBe('Not saved: Sign in again to save.');
    expect((await say(new SignedOutError('x', 'locked'))).label).toBe(`Not saved: ${WORDS.locked}`);
    expect((await say(new HTTPError(403, '403: the key lacks the scope'))).label).toBe('Not saved: This device is not allowed to save here.');
    expect((await say(new HTTPError(400, 'bad body'))).label).toBe('Not saved: Could not save. bad body');
    expect((await say(new HTTPError(503, 'busy'))).status).toBe('offline');
    expect((await say(new HTTPError(429, '429: rate limited'))).status).toBe('offline');
    expect((await say(new Error('timeout'))).status).toBe('offline');
  });
  test('E10 sign-out and an account switch keep nothing of the old account', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit(A, 'Account one text\n\nSecond', d);
    expect(files.size).toBe(1);
    await E.wipeTextureEdit(storage);
    expect(files.size).toBe(0);
    expect((await E.textureEditInfo(A, d)).status).toBe('idle');                                // nothing shows
    // an account change without a wipe call: the generation moved, the old session is not used
    await E.textureEdit(A, 'Account two\n\nSecond', d);
    nextGeneration();
    const before = s.calls.length;
    await E.textureFlush(A, d);                                                                  // a new generation: the old session posts nothing to the new account
    expect(s.calls.slice(before).some((c) => c.startsWith('POST'))).toBe(false);
    expect(s.posts.every((p) => p.body.content !== 'Account one text\n\nSecond')).toBe(true);
    // a save in flight when the account changes keeps and posts nothing after
    let open: () => void = () => undefined;
    const slow = server({ gate: new Promise<void>((r) => { open = r; }) });
    E.resetTextureEdit(); resetSeam();
    await E.textureEdit(A, 'Late text\n\nSecond', { net: slow.net, storage });
    const inflight = E.textureFlush(A, { net: slow.net, storage });
    await new Promise((r) => setTimeout(r, 20));
    nextGeneration(); await E.wipeTextureEdit(storage); open(); await inflight;
    expect(files.size).toBe(0);
    expect((await E.textureEditInfo(A, { net: slow.net, storage })).label).not.toContain('Saved');
  });
  test('E11 the background posts what is waiting', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit(A, 'Typed then hidden\n\nSecond', d);
    const i = await E.textureEditInfo(A, d, true);
    expect(s.posts.length).toBe(1); expect(i.status).toBe('saved');
  });
  test('E14 leaving the page posts what was typed on it', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit(A, 'Typed then left\n\nSecond', d);
    expect((await E.textureEditInfo(A, d)).status).toBe('dirty');
    expect((await E.textureEditInfo('paper:front', d)).editable).toBe(false);
    expect(s.posts.length).toBe(1);
  });
  test('E13 after a save the native side is told once, with the heading as the title', async () => {
    const s = server(); const told: Record<string, unknown>[] = [];
    const d = { net: s.net, storage, native: async (r: Record<string, unknown>) => { told.push(r); } };
    await E.textureEdit(A, '\n# Harbor notes\n\nBody', d); await E.textureFlush(A, d); await E.textureFlush(A, d);
    expect(told).toEqual([{ op: 'textureSaved', doc: 'd1', title: 'Harbor notes', text: '\n# Harbor notes\n\nBody', version: 2 }]);
  });
});
