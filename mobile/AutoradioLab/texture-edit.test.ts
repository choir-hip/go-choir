// The Texture editor under bun test (texture-edit.ts). Ported from TextureEditorTests (Swift, deleted with the Swift); the failure list is at the top of texture-edit.ts (1..6).
// The pause in typing is the contract's now (a `task after(2000)` calling `textureFlush`), so the burst test types, checks nothing was posted, then flushes once.
//  E1 a burst of typing is one revision, parent = the head, the untouched paragraph is the original node, label "Saved · version 2".   E2 Done posts at once.   E3 a revision and a story have no editor.
//  E4 nothing changed posts nothing.   E5 offline: kept on the phone from the first keystroke, across a launch, posted once.   E6 typing while posting saves right after.
//  E8 the server's real shapes: a document with a trajectory is saved against its lifecycle (idempotency key, version, head).
//  E9 the words: refused, locked, forbidden, offline, a server error.   E10 sign-out and account switch: nothing of the old account shows, posts or is kept.
//  E14 leaving the page posts what was typed on it.   E11 the phone goes to the background: what is waiting is posted.   E12 the first answer after a launch posts what an earlier launch kept.   E13 after a save the native reader and content store are told once.
// Review round 1 (2026-11-20, SEND_BACK), the failure list for the second pass, written before the code:
//  M1 an upgrade does not strand the old build's unsent edit: the old JSON (Application Support/Autoradio/texture-pending.json, `{"<doc>": "<markdown>"}`) is read through ONE native call, merged into
//     the pending set, written durably in the new place, and only then removed from the old place; the normal save sends it.   M2 a crash between the write and the remove sends nothing twice (the next
//     launch merges again, the text is already the head's: no revision).   M3 the new place's text wins over the old file's for the same document; a damaged old file is removed, not retried forever.
//  M4 no native module (Android, tests): nothing is read.   M5 old text equal to the head is cleared, not posted.
//  R1 the native lifecycle signal (willResignActive, counted by the native side) flushes what is waiting; visibilityState is no longer the only road; an unchanged count flushes nothing.
//  K1 a 409 re-reads the head and merges the draft onto it (a different paragraph changed elsewhere): both changes are in the one revision posted with the fresh parent.
//  K2 a 409 where the same line changed both ways: nothing is posted, the text is kept with the revision it was written over, the page says plainly what happened, no 30 s retry (status conflict).
//  K3 a relaunch whose pending text was written over an older revision than the head's: merged when it can be, kept and told when it cannot; never posted over the newer version silently.
//  K4 a 409 with the head unchanged is the server's refusal: said, not looped.   K5 the app's own topic sync (the notebook posts behind the editor) moves the head: the editor's next save recovers (K1).
//  K6 the reply of a post was lost (the post landed): the retry finds the head equal to the draft, posts nothing, clears the kept copy; a trajectory document keeps one idempotency key for the same text.
//  L1 `Body.merge` throwing (a damaged body_doc) leaves `saving` released and says "failed", not "offline"; the next save works.   L2 a 6000-block document: an edit is one cheap revision; a worst-case rewrite finishes.
//  A1 a kept edit is the account's: after the generation moved nothing of it shows or posts.   A2 the kept file names the account (the key's digest); another account's file is not loaded and is removed.
//  P1 saving a draft equal to the head clears the kept copy (note 8).
// Review round 2 (2026-11-20, SEND_BACK: these are the owner's real documents). The INVARIANTS, each tested with texts the editor's field can really produce (the field holds what the owner last
// typed, and is reseeded from the head only when the editor opens; nothing writes a merge back into it):
//  I1 no keystroke after a three-way merge removes another writer's change. Every save is a three-way merge of (base = the field's text at the last successful save, ours = the field now, theirs = the
//     head): a field that lags the head can never revert it. K1 (a, b, c), K1d.
//  I2 a kept text with no recorded base (typed when the head could not be read; every line of the old build's file) is never posted over a head that is not equal to it: it becomes a kept conflict. If it
//     equals the head it is cleared. N1 (moved head), N2 (equal head), N3 (typed offline with no session), M1-M5 (the upgrade).
//  I3 a kept conflict is readable and copyable after the editor closes and after a relaunch (`info.kept`), is never overwritten by typing (a second text for the document is refused with a plain
//     sentence and stays in the field and in the ordinary kept set), and goes only by Discard. C1-C4.
//  I4 sign-out can say how many unsent edits it will remove (`info.unsent`, kept edits and conflicts, any document). U1.
//  I5 the retry is global: `textureFlush('')` retries every kept edit whatever page is in front, and the first answer after a launch retries what an earlier launch kept even when the page in front
//     is not a document (the radio page). G1, G2.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as E from './texture-edit.ts';
import { SignedOutError, HTTPError, resetSeam, nextGeneration, bindNative, type Reply } from './api.ts';
import { WORDS } from './words.ts';

const reply = (body: unknown): Reply => ({ body, serverTime: 0, raw: '' });
const files = new Map<string, Uint8Array>();
const fsLog: string[] = [];
const storage = { fs: {
  readFile: async (p: string) => { const f = files.get(p); if (!f) throw new Error('ENOENT'); return f; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { fsLog.push('write'); files.set(p, d); },
  rm: async (p: string) => { fsLog.push('rm'); files.delete(p); },
} } as never;

const HEAD_BODY = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', content: [
  { type: 'paragraph', attrs: { id: 'p1' }, content: [{ type: 'text', text: 'Hello world' }] },
  { type: 'paragraph', attrs: { id: 'p2' }, content: [{ type: 'text', text: 'Second' }] }] } };
type Post = { path: string; body: Record<string, unknown> };
const CONFLICT = 'document head changed; reload the latest version before saving';
/** A fake of the server's routes, with the field names the Go handlers write (internal/textureowner/texture.go). A POST whose parent is not the head is refused 409 (the server's rule, revise_edit.go). */
function server(o: { trajectory?: string; failure?: unknown; gate?: Promise<void>; lostReply?: boolean; lifeHead?: string; body?: unknown; content?: string } = {}) {
  const posts: Post[] = [], calls: string[] = [];
  const revs = new Map<string, { content: string; body: unknown; n: number }>();
  let head = { id: 'r1', content: o.content ?? 'Hello world\n\nSecond', body: (o.body ?? HEAD_BODY) as unknown, n: 0 };
  revs.set('r1', head);
  let lost = o.lostReply === true;
  const net: E.Net = { json: async (path, opts) => {
    calls.push(`${opts?.method ?? 'GET'} ${path}`);
    if (opts?.method === 'POST') {
      if (o.failure) throw o.failure;
      if (o.gate && posts.length === 0) await o.gate;
      const b = opts.body as Record<string, unknown>;
      if (!o.trajectory && b.parent_revision_id !== head.id) throw new HTTPError(409, CONFLICT);
      posts.push({ path, body: b });
      head = { id: `r${revs.size + 1}`, content: String(b.content), body: b.body_doc, n: head.n + 1 };
      revs.set(head.id, head);
      if (lost) { lost = false; throw new TypeError('Network request failed'); }                // the post landed; the answer did not
      return reply({ revision_id: head.id, doc_id: 'd1', version_number: head.n, content: head.content });
    }
    if (path === '/api/texture/documents/d1') return reply({ doc_id: 'd1', title: 'Notes', current_revision_id: head.id, current_version_number: head.n, ...(o.trajectory ? { trajectory_id: o.trajectory } : {}) });
    if (path.startsWith('/api/texture/revisions/')) { const r = revs.get(decodeURIComponent(path.split('/').pop()!)); if (!r) throw new HTTPError(404, 'no such revision'); return reply({ revision_id: path.split('/').pop(), doc_id: 'd1', version_number: r.n, content: r.content, body_doc: r.body }); }
    if (path.startsWith('/api/texture/lifecycle/')) return reply({ trajectory: { lifecycle_version: 7 }, current_document_head: { revision_id: o.trajectory ? (o.lifeHead ?? head.id) : 'life-head' } });
    throw new HTTPError(404, 'nothing');
  } };
  /** Another writer (the web, the notebook's topic sync, another device) saves a new head. */
  const other = (content: string): void => { head = { id: `r${revs.size + 1}`, content, body: null, n: head.n + 1 }; revs.set(head.id, head); };
  return { net, posts, calls, other, head: () => head, revs };
}
/** The app module's native calls this editor makes, with the old build's file in `old` (a string, or null when there is none). */
function nativeFake(old: string | null = null) {
  const log: string[] = [];
  const st = { old, resigns: 0, removeFails: false };
  const call = async (r: Record<string, unknown>): Promise<unknown> => {
    log.push(String(r.op));
    if (r.op === 'textureOldPending') return st.old === null ? { found: false } : { found: true, text: st.old };
    if (r.op === 'textureOldPendingRemove') { if (st.removeFails) throw new Error('cannot remove'); st.old = null; return { ok: true }; }
    if (r.op === 'textureResigned') return { count: st.resigns };
    return { ok: true };
  };
  return { call, log, st };
}
beforeEach(() => { resetSeam(); E.resetTextureEdit(); files.clear(); fsLog.length = 0; });
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
  test('E8 a document with a trajectory is saved against its lifecycle', async () => {
    const s = server({ trajectory: 'tr-1', lifeHead: 'life-head' }), d = { net: s.net, storage };
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
    const d = { net: s.net, storage, native: async (r: Record<string, unknown>) => { if (r.op === 'textureSaved') told.push(r); return null; } };
    await E.textureEdit(A, '\n# Harbor notes\n\nBody', d); await E.textureFlush(A, d); await E.textureFlush(A, d);
    expect(told).toEqual([{ op: 'textureSaved', doc: 'd1', title: 'Harbor notes', text: '\n# Harbor notes\n\nBody', version: 2 }]);
  });

  // ---- review round 1: the upgrade, the background, a changed head, a long document, the account
  const OLD = JSON.stringify({ d1: 'Kept by the old build\n\nSecond' });          // the old build's file, as Swift wrote it: {"<doc>": "<markdown>"}
  const newFile = (): { v: number; account: string; docs: Record<string, { text: string; base: string }> } => JSON.parse(new TextDecoder().decode(files.get(E.PENDING_FILE)!));

  test('M1 the old build\'s unsent edit is carried over: read, merged, durable, removed; over a head that is not equal to it, it is kept as a conflict and NOT posted', async () => {
    const s = server(), n = nativeFake(OLD), d = { net: s.net, storage, native: n.call };
    const i = await E.textureEditInfo(A, d);
    expect(n.log.slice(0, 2)).toEqual(['textureOldPending', 'textureOldPendingRemove']);
    expect(n.st.old).toBeNull();                                                      // the old file is gone
    expect(s.posts.length).toBe(0);                                                   // no base: never posted over the head
    expect(i.status).toBe('conflict'); expect(i.kept).toBe('Kept by the old build\n\nSecond');
    expect(newFile().conflicts.d1.text).toBe('Kept by the old build\n\nSecond'); expect(newFile().docs.d1).toBeUndefined();
    await E.flushAll(d); expect(s.posts.length).toBe(0);
  });
  test('M1b the new file is written BEFORE the old one is removed', async () => {
    const down = server({ failure: new TypeError('Network request failed') });
    const order: string[] = [];
    const n = nativeFake(OLD);
    const call = async (r: Record<string, unknown>) => { if (r.op === 'textureOldPendingRemove') order.push(`remove after ${fsLog.filter((x) => x === 'write').length} write(s); file kept: ${files.has(E.PENDING_FILE)}`); return n.call(r); };
    await E.textureEditInfo(A, { net: down.net, storage, native: call });
    expect(order).toEqual(['remove after 1 write(s); file kept: true']);
    expect(newFile().conflicts.d1.text).toBe('Kept by the old build\n\nSecond');
    expect(n.st.old).toBeNull();
  });
  test('M2 a crash between the write and the remove changes nothing the next launch', async () => {
    const s = server(), n = nativeFake(OLD); n.st.removeFails = true;
    await E.textureEditInfo(A, { net: s.net, storage, native: n.call });             // launch 1: merged and written, the remove fails (a crash)
    expect(n.st.old).not.toBeNull(); expect(newFile().conflicts.d1.text).toBe('Kept by the old build\n\nSecond');
    E.resetTextureEdit(); n.st.removeFails = false;
    await E.textureEditInfo(A, { net: s.net, storage, native: n.call });              // launch 2: the old file is merged again (nothing new), removed
    expect(s.posts.length).toBe(0); expect(n.st.old).toBeNull(); expect(newFile().conflicts.d1.text).toBe('Kept by the old build\n\nSecond');
    E.resetTextureEdit(); n.st.old = OLD;                                              // launch 3: the old file came back
    await E.textureEditInfo(A, { net: s.net, storage, native: n.call });
    expect(s.posts.length).toBe(0); expect(Object.keys(newFile().conflicts)).toEqual(['d1']);
  });
  test('M3 the new place wins for the same document; a damaged old file is removed', async () => {
    const down = server({ failure: new TypeError('x') });
    await E.textureEdit(A, 'Newer text\n\nSecond', { net: down.net, storage });
    E.resetTextureEdit();
    const s = server(), n = nativeFake(JSON.stringify({ d1: 'Older text', d2: 'Other doc' }));
    await E.textureEditInfo(A, { net: s.net, storage, native: n.call });
    expect(s.posts.map((p) => p.body.content)).toEqual(['Newer text\n\nSecond']);
    expect(newFile().docs.d2.text).toBe('Other doc');                                 // d2 has no route here (404): kept
    const bad = nativeFake('{not json'); E.resetTextureEdit(); files.clear();
    await E.textureEditInfo(A, { net: s.net, storage, native: bad.call });
    expect(bad.st.old).toBeNull();
  });
  test('M4 no native module: nothing is read, nothing breaks', async () => {
    const s = server();
    expect((await E.textureEditInfo(A, { net: s.net, storage })).status).toBe('idle'); expect(s.posts.length).toBe(0);
  });
  test('M5 old text equal to the head is cleared, not posted', async () => {
    const s = server(), n = nativeFake(JSON.stringify({ d1: 'Hello world\n\nSecond' }));
    await E.textureEditInfo(A, { net: s.net, storage, native: n.call });
    expect(s.posts.length).toBe(0); expect(files.size).toBe(0); expect(n.st.old).toBeNull();
  });
  test('R1 the native "went to the background" signal flushes what is waiting', async () => {
    const s = server(), n = nativeFake(), d = { net: s.net, storage, native: n.call };
    await E.textureEdit(A, 'Typed then locked\n\nSecond', d);
    expect((await E.textureEditInfo(A, d)).status).toBe('dirty'); expect(s.posts.length).toBe(0);      // same count: nothing flushed
    n.st.resigns = 1;
    expect((await E.textureEditInfo(A, d)).status).toBe('saved'); expect(s.posts.length).toBe(1);
    await E.textureEdit(A, 'Typed again\n\nSecond', d);
    expect((await E.textureEditInfo(A, d)).status).toBe('dirty'); expect(s.posts.length).toBe(1);     // the count did not move again
    n.st.resigns = 2;
    await E.textureEditInfo(A, d); expect(s.posts.length).toBe(2);
  });
  test('K1 a 409 merges, and the NEXT keystroke does not revert the other writer (the field is not told the merge)', async () => {
    const s = server({ content: 'one\n\ntwo\n\nthree' }), d = { net: s.net, storage };
    await E.textureEdit(A, 'ONE\n\ntwo\n\nthree', d);                                 // the field holds what the owner typed; the session base is r1
    s.other('one\n\ntwo\n\nTHREE');                                                     // another writer (the notebook's topic sync, the web) saved r2
    const i = await E.textureFlush(A, d);
    expect(s.posts.length).toBe(1); expect(s.posts[0].body.content).toBe('ONE\n\ntwo\n\nTHREE'); expect(s.posts[0].body.parent_revision_id).toBe('r2');
    expect(i.status).toBe('saved'); expect(files.size).toBe(0);
    // (b) the owner keeps typing: the field still holds "three", not the merge
    await E.textureEdit(A, 'ONE\n\ntwo!\n\nthree', d); await E.textureFlush(A, d);
    expect(s.head().content).toBe('ONE\n\ntwo!\n\nTHREE');                              // the other writer's THREE survives the keystroke
    expect(s.posts[1].body.parent_revision_id).toBe('r3');
    // (c) the owner edits the very line they edited before: no false conflict with their own earlier change
    await E.textureEdit(A, 'ONE!\n\ntwo!\n\nthree', d); await E.textureFlush(A, d);
    expect(s.head().content).toBe('ONE!\n\ntwo!\n\nTHREE'); expect(s.posts.length).toBe(3);
    // the other writer again, between two keystrokes
    s.other('ONE!\n\ntwo!\n\nTHREE\n\nfour');
    await E.textureEdit(A, 'ONE!\n\ntwo!!\n\nthree', d); const k = await E.textureFlush(A, d);
    expect(s.head().content).toBe('ONE!\n\ntwo!!\n\nTHREE\n\nfour'); expect(k.status).toBe('saved');
  });
  test('K1d a head that moved with the owner NOT typing again is not touched, and what the owner typed back to the head posts nothing', async () => {
    const s = server({ content: 'one\n\ntwo' }), d = { net: s.net, storage };
    await E.textureEdit(A, 'ONE\n\ntwo', d); await E.textureFlush(A, d);               // r2
    s.other('ONE\n\ntwo\n\nnew');                                                         // r3 by someone else
    await E.textureEdit(A, 'ONE\n\ntwo', d);                                              // the field text equals the last save: the head is theirs, nothing is ours
    await E.textureFlush(A, d);
    expect(s.head().content).toBe('ONE\n\ntwo\n\nnew'); expect(s.posts.length).toBe(1);
  });
  test('K2 the same line changed both ways: nothing is posted, the text is kept, the owner is told', async () => {
    const s = server({ content: 'one\n\ntwo' }), d = { net: s.net, storage };
    await E.textureEdit(A, 'ONE\n\ntwo', d);
    s.other('uno\n\ntwo');
    const i = await E.textureFlush(A, d);
    expect(s.posts.length).toBe(0);
    expect(i.status).toBe('conflict'); expect(i.label).toContain('changed somewhere else'); expect(i.label).toContain('kept on this phone');
    expect(newFile().conflicts.d1).toEqual({ text: 'ONE\n\ntwo', base: 'r1' }); expect(i.kept).toBe('ONE\n\ntwo');
    await E.flushAll(d); await E.textureFlush(A, d); expect(s.posts.length).toBe(0);    // no retry loop, no overwrite
    expect(s.head().content).toBe('uno\n\ntwo');
  });
  test('K3 a relaunch: pending text written over an older revision is merged, or kept and told', async () => {
    const s = server({ content: 'one\n\ntwo\n\nthree' }), d = { net: s.net, storage };
    await E.textureEdit(A, 'ONE\n\ntwo\n\nthree', d); E.resetTextureEdit();           // typed over r1, kept with base r1; the app is killed
    expect(newFile().docs.d1.base).toBe('r1');
    s.other('one\n\ntwo\n\nTHREE');
    const i = await E.textureEditInfo(A, d);                                              // the launch: merged onto r2
    expect(s.posts.map((p) => p.body.content)).toEqual(['ONE\n\ntwo\n\nTHREE']); expect(i.status).toBe('saved');
    // and a conflicting one
    E.resetTextureEdit(); files.clear();
    const c = server({ content: 'one\n\ntwo' }); const dc = { net: c.net, storage };
    await E.textureEdit(A, 'ONE\n\ntwo', dc); E.resetTextureEdit(); c.other('uno\n\ntwo');
    const j = await E.textureEditInfo(A, dc);
    expect(c.posts.length).toBe(0); expect(j.status).toBe('conflict'); expect(c.head().content).toBe('uno\n\ntwo'); expect(files.size).toBe(1);
  });
  test('K4 a 409 with the head unchanged is the server\'s refusal: said once, not looped', async () => {
    const s = server(), d = { net: s.net, storage };
    const real = s.net.json;
    const net: E.Net = { json: async (p, o) => { if (o?.method === 'POST') { s.calls.push('POST-refused'); throw new HTTPError(409, CONFLICT); } return real(p, o); } };
    await E.textureEdit(A, 'x\n\nSecond', { net, storage });
    const i = await E.textureFlush(A, { net, storage });
    expect(i.status).toBe('failed'); expect(i.label).toContain('document head changed');
    expect(s.calls.filter((c) => c === 'POST-refused').length).toBe(1);
  });
  test('K6 the reply of a post was lost: the retry posts nothing and clears the kept copy', async () => {
    const s = server({ lostReply: true }), d = { net: s.net, storage };
    await E.textureEdit(A, 'Landed\n\nSecond', d);
    expect((await E.textureFlush(A, d)).status).toBe('offline'); expect(s.posts.length).toBe(1); expect(files.size).toBe(1);
    const i = await E.textureFlush(A, d);
    expect(s.posts.length).toBe(1); expect(i.status).toBe('saved'); expect(files.size).toBe(0);
    // a trajectory document: the same text keeps its idempotency key
    const t = server({ trajectory: 'tr-9', lostReply: true }), dt = { net: t.net, storage };
    await E.textureEdit(A, 'Landed too\n\nSecond', dt); await E.textureFlush(A, dt); await E.textureFlush(A, dt);
    expect(t.posts.length).toBeGreaterThanOrEqual(1);
    if (t.posts.length > 1) expect(t.posts[1].body.idempotency_key).toBe(t.posts[0].body.idempotency_key);
  });
  test('P1 a draft equal to the head clears the kept copy', async () => {
    const s = server(), d = { net: s.net, storage };
    await E.textureEdit(A, 'Changed\n\nSecond', d);
    expect(files.size).toBe(1);
    await E.textureEdit(A, 'Hello world\n\nSecond', d);                                  // typed back to what the head says
    expect(files.size).toBe(0);
  });
  test('L1 a merge that throws releases the save and says failed, not offline', async () => {
    const bad = { schema: 'choir.texture_doc.v1', get doc(): never { throw new Error('damaged body_doc'); } };
    const s = server({ body: bad }), d = { net: s.net, storage };
    await E.textureEdit(A, 'Changed\n\nSecond', d);
    const i = await E.textureFlush(A, d);
    expect(i.status).toBe('failed'); expect(i.label).toContain('Not saved'); expect(s.posts.length).toBe(0); expect(files.size).toBe(1);
    const j = await E.textureFlush(A, d);                                                 // not stuck on "saving": it tries again
    expect(j.status).toBe('failed');
  });
  test('L2 a long document: one edit is one cheap revision; a rewrite finishes', async () => {
    const paras = Array.from({ length: 6000 }, (_, i) => `Paragraph ${i} has its own words ${i * 7919}.`);
    const nodes = paras.map((t, i) => ({ type: 'paragraph', attrs: { id: `p${i}` }, content: [{ type: 'text', text: t }] }));
    const body = { schema: 'choir.texture_doc.v1', doc: { type: 'doc', content: nodes } };
    const s = server({ body, content: paras.join('\n\n') }), d = { net: s.net, storage };
    const edited = paras.slice(); edited[3000] = 'Edited paragraph.';
    await E.textureEdit(A, edited.join('\n\n'), d);
    const t0 = performance.now(); const i = await E.textureFlush(A, d);
    expect(performance.now() - t0).toBeLessThan(500); expect(i.status).toBe('saved');
    const out = (s.posts[0].body.body_doc as { doc: { content: unknown[] } }).doc.content;
    expect(out.length).toBe(6000); expect(out[10]).toBe(nodes[10]);
    // everything rewritten, and a conflicting head over 6000 lines: finishes, says conflict or saved, never hangs
    const t1 = performance.now();
    await E.textureEdit(A, paras.map((_, k) => `Rewritten ${k} in full.`).join('\n\n'), d); await E.textureFlush(A, d);
    expect(performance.now() - t1).toBeLessThan(1500);
  });
  test('A1 after the account generation moves, a kept edit does not show or post', async () => {
    const down = server({ failure: new TypeError('x') });
    await E.textureEdit(A, 'Account one text\n\nSecond', { net: down.net, storage });
    expect((await E.textureFlush(A, { net: down.net, storage })).status).toBe('offline');
    nextGeneration();
    const up = server();
    expect((await E.textureEditInfo(A, { net: up.net, storage })).status).toBe('idle');
    await E.flushAll({ net: up.net, storage });
    expect(up.posts.length).toBe(0);
  });
  test('A2 the kept file names the account; another account\'s file is not loaded and is removed', async () => {
    const key = { v: 'key-one' };
    bindNative({ later: async (r: { op: string }) => (r.op === 'keystate' ? { state: 'present' } : r.op === 'session' ? { found: true, value: key.v, computerId: 'c1' } : null) });
    const down = server({ failure: new TypeError('x') });
    await E.textureEdit(A, 'Private text\n\nSecond', { net: down.net, storage });
    const mine = newFile().account; expect(mine).toMatch(/^[0-9a-f]{64}$/);
    key.v = 'key-two'; E.resetTextureEdit();                                              // another account signed in on this phone with no wipe in between
    const up = server();
    expect((await E.textureEditInfo(A, { net: up.net, storage })).status).toBe('idle');
    expect(up.posts.length).toBe(0); expect(files.size).toBe(0);
    key.v = 'key-one'; E.resetTextureEdit();
    await E.textureEdit(A, 'Mine again\n\nSecond', { net: down.net, storage }); E.resetTextureEdit();
    expect((await E.textureEditInfo(A, { net: up.net, storage })).status).toBe('saved');  // the same account: loaded and sent
  });

  // ---- review round 2
  const noRoute: E.Net = { json: async () => { throw new TypeError('Network request failed'); } };
  test('N1 a kept text with no base over a head that is not equal to it is a conflict: never posted, kept', async () => {
    await E.textureEdit(A, 'Typed with no route\n\nSecond', { net: noRoute, storage });              // no head could be read: no base
    expect(newFile().docs.d1.base).toBe('');
    E.resetTextureEdit();
    const s = server();
    const i = await E.textureEditInfo(A, { net: s.net, storage });                                  // the launch
    expect(s.posts.length).toBe(0); expect(i.status).toBe('conflict'); expect(i.kept).toBe('Typed with no route\n\nSecond');
    expect(newFile().conflicts.d1.text).toBe('Typed with no route\n\nSecond');
    await E.textureFlush(A, { net: s.net, storage }); await E.flushAll({ net: s.net, storage }); expect(s.posts.length).toBe(0);
  });
  test('N2 a kept text with no base that equals the head is cleared', async () => {
    await E.textureEdit(A, 'Hello world\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const s = server();
    const i = await E.textureEditInfo(A, { net: s.net, storage });
    expect(s.posts.length).toBe(0); expect(i.kept).toBe(''); expect(files.size).toBe(0);
  });
  test('N3 typing while the head cannot be read, then the route comes back: a conflict, not a post', async () => {
    const s = server();
    await E.textureEdit(A, 'Offline words\n\nSecond', { net: noRoute, storage });                  // same process, no session
    const i = await E.textureFlush(A, { net: s.net, storage });
    expect(s.posts.length).toBe(0); expect(i.status).toBe('conflict'); expect(i.kept).toBe('Offline words\n\nSecond');
  });
  test('N4 opening a document reads its head, so typing offline afterwards HAS a base and posts when the route returns', async () => {
    const s = server();
    await E.textureEditInfo(A, { net: s.net, storage });                                            // the editor's page appears: the head is read now
    await E.textureEdit(A, 'Typed on the train\n\nSecond', { net: noRoute, storage });
    expect(newFile().docs.d1.base).toBe('r1');
    const i = await E.textureFlush(A, { net: s.net, storage });
    expect(i.status).toBe('saved'); expect(s.posts.length).toBe(1);
  });
  test('C1 a kept conflict is readable after the editor closes and after a relaunch, and typing does not overwrite it', async () => {
    await E.textureEdit(A, 'Kept words\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const s = server(), d = { net: s.net, storage };
    expect((await E.textureEditInfo(A, d)).kept).toBe('Kept words\n\nSecond');
    E.resetTextureEdit();                                                                            // another relaunch: still there
    expect((await E.textureEditInfo(A, d)).kept).toBe('Kept words\n\nSecond');
    expect((await E.textureEditInfo('paper:front', d)).kept).toBe('');                               // only for its own document
    const t = await E.textureEdit(A, 'Fresh typing\n\nSecond', d);                                   // the owner opens the editor (the field holds the head) and types
    expect(t.kept).toBe('Kept words\n\nSecond'); expect(newFile().conflicts.d1.text).toBe('Kept words\n\nSecond');
    const f = await E.textureFlush(A, d);
    expect(s.posts.length).toBe(0); expect(f.status).toBe('conflict'); expect(f.label).toContain('Copy or discard');
    expect(newFile().docs.d1.text).toBe('Fresh typing\n\nSecond');                                    // the new typing is kept in the ordinary set, beside the conflict
  });
  test('C2 Discard removes the conflict only, and then the typing posts', async () => {
    await E.textureEdit(A, 'Kept words\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const s = server(), d = { net: s.net, storage };
    await E.textureEditInfo(A, d); await E.textureEdit(A, 'Fresh typing\n\nSecond', d);
    const g = await E.textureDiscard(A, d);
    expect(g.kept).toBe(''); expect(newFile().conflicts).toEqual({});
    const f = await E.textureFlush(A, d);
    expect(f.status).toBe('saved'); expect(s.posts.map((p) => p.body.content)).toEqual(['Fresh typing\n\nSecond']);
  });
  test('C3 a second text with no base, while a conflict is kept, is refused with a plain sentence and the first is untouched', async () => {
    await E.textureEdit(A, 'First kept\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const s = server(), d = { net: s.net, storage };
    await E.textureEditInfo(A, d);                                                                    // the first is a conflict now
    E.resetTextureEdit();
    await E.textureEdit(A, 'Second kept\n\nSecond', { net: noRoute, storage });                       // typed with no route, a conflict already kept (loaded from the file)
    const j = await E.textureEdit(A, 'Second kept, more\n\nSecond', { net: noRoute, storage });
    expect(newFile().conflicts.d1.text).toBe('First kept\n\nSecond');
    expect(j.kept).toBe('First kept\n\nSecond');
    await E.textureEditInfo(A, d);                                                                    // the route is back: the second is a conflict too, but there is no room: said, kept, not posted
    const f = await E.textureFlush(A, d);
    expect(s.posts.length).toBe(0); expect(f.label).toContain('Copy or discard'); expect(newFile().conflicts.d1.text).toBe('First kept\n\nSecond');
  });
  test('C4 an earlier launch\'s kept text is not overwritten by new typing when the route was down at launch', async () => {
    const s = server();
    await E.textureEditInfo(A, { net: s.net, storage }); await E.textureEdit(A, 'Unsent before the kill\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const j = await E.textureEdit(A, 'Typed after relaunch\n\nSecond', { net: noRoute, storage });     // the route is still down: the field was reseeded from the head
    expect(j.kept).toBe('Unsent before the kill\n\nSecond');                                           // shelved, not destroyed
    expect(newFile().docs.d1.text).toBe('Typed after relaunch\n\nSecond');
  });
  test('U1 the unsent count: kept edits and conflicts, for any document', async () => {
    const s = server(), d = { net: s.net, storage };
    expect((await E.textureEditInfo('paper:front', d)).unsent).toBe(0);
    await E.textureEdit(A, 'Typed\n\nSecond', { net: noRoute, storage });                              // d1 kept, no base
    await E.textureEdit('texture:d2', 'Other doc text', { net: noRoute, storage });                    // d2 kept
    const i = await E.textureEditInfo('paper:front', { net: noRoute, storage });
    expect(i.unsent).toBe(2); expect(i.waiting).toBe(2);
    E.resetTextureEdit();
    const k = await E.textureEditInfo('paper:front', d);                                               // launch: d1 becomes a conflict, d2 has no route here (404): still kept
    expect(k.unsent).toBe(2); expect(k.waiting).toBe(1);
    await E.wipeTextureEdit(storage);
    expect((await E.textureEditInfo('paper:front', d)).unsent).toBe(0);
  });
  test('G1 the retry is global: textureFlush("") posts every kept edit whatever page is in front', async () => {
    const s = server();
    await E.textureEditInfo(A, { net: s.net, storage });
    await E.textureEdit(A, 'Typed on the train\n\nSecond', { net: noRoute, storage });
    expect((await E.textureEditInfo('paper:front', { net: noRoute, storage })).waiting).toBe(1);
    const i = await E.textureFlush('', { net: s.net, storage });
    expect(s.posts.length).toBe(1); expect(i.waiting).toBe(0); expect(files.size).toBe(0);
  });
  test('G2 the launch retry runs from a page that is not a document', async () => {
    const s0 = server();
    await E.textureEditInfo(A, { net: s0.net, storage }); await E.textureEdit(A, 'Kept over r1\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const s = server();
    const i = await E.textureEditInfo('radio:', { net: s.net, storage });                              // the radio page is in front
    expect(s.posts.length).toBe(1); expect(i.waiting).toBe(0); expect(files.size).toBe(0);
  });
  test('C5 the route is back after the launch retry failed: the document opens (a session), the owner types, and the earlier kept text is shelved, not overwritten', async () => {
    const s0 = server();
    await E.textureEditInfo(A, { net: s0.net, storage }); await E.textureEdit(A, 'Kept over r1\n\nSecond', { net: noRoute, storage }); E.resetTextureEdit();
    const s = server();
    await E.textureEditInfo('radio:', { net: noRoute, storage });                                        // the launch retry could not reach the route
    await E.textureEditInfo(A, { net: s.net, storage });                                                 // the route is back; the document opens
    const t = await E.textureEdit(A, 'Typed fresh\n\nSecond', { net: s.net, storage });
    expect(t.kept).toBe('Kept over r1\n\nSecond'); expect(newFile().conflicts.d1.text).toBe('Kept over r1\n\nSecond');
  });
});
