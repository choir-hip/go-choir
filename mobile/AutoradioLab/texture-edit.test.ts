// The Texture editor, derived from the table at the head of texture-edit.ts: one test per row (T01 to T43) and per kill (K1 to K8); the id of the row is the start of each test's name.
// The fake server answers what the real one does (docs, revisions, lifecycle, a post with a parent check and an idempotency key); the fake disk can delay or fail a write; a "hung" post lands or does not land
// and never answers, which is how a kill during a post is made. A LAUNCH is a second Editor on the same files. Nothing here reads source text.
import { beforeEach, describe, expect, test } from 'bun:test';
import { Editor, PENDING_DIR, editableDoc, textureEditInfo, textureEdit, textureEditOpen, textureFlush, wipeTextureEdit, forgetTextureEdit, type Net, type Info } from './texture-edit.ts';
import { HTTPError, SignedOutError, resetSeam, bindNative, beginSignOut, nextGeneration } from './api.ts';
import { projectBody } from './texture-body.ts';

// ---------------------------------------------------------------------------------------------------------------- the fake server
type Rev = { id: string; content: string; body: unknown; n: number };
let revs: Map<string, Rev>; let head: Record<string, string>; let traj: Record<string, string>; let keys: Map<string, string>;
let log: { method: string; path: string; body?: Record<string, unknown> }[];
let failNext: (() => Error) | null; let postMode: 'normal' | 'landHang' | 'dropHang'; let getGate: Promise<void> | null; let postGate: Promise<void> | null; let failGets: boolean; let seq: number;
const body = (t: string): unknown => ({ schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content: [{ type: 'paragraph', attrs: { id: 'p' }, content: [{ type: 'text', text: t }] }] } });
const hang = (): Promise<never> => new Promise<never>(() => undefined);
const reply = (b: unknown) => ({ body: b, serverTime: null, raw: '' });
const net: Net = { json: async (path, opts) => {
  const method = opts?.method ?? 'GET';
  log.push({ method, path, body: opts?.body as Record<string, unknown> | undefined });
  if (method === 'GET') { if (getGate) await getGate; if (failGets) throw new TypeError('network down'); }
  let m = path.match(/^\/api\/texture\/documents\/([^/]+)$/);
  if (m && method === 'GET') return reply({ document: { doc_id: m[1], current_revision_id: head[m[1]], ...(traj[m[1]] ? { trajectory_id: traj[m[1]] } : {}) } });
  m = path.match(/^\/api\/texture\/revisions\/([^/]+)$/);
  if (m) { const r = revs.get(m[1])!; return reply({ revision: { revision_id: r.id, content: r.content, body_doc: r.body, version_number: r.n } }); }
  m = path.match(/^\/api\/texture\/lifecycle\/([^/]+)$/);
  if (m) return reply({ trajectory: { lifecycle_version: 7 } });
  m = path.match(/^\/api\/texture\/documents\/([^/]+)\/revisions$/);
  if (m && method === 'POST') {
    if (postGate) await postGate;
    if (failNext) { const e = failNext(); failNext = null; throw e; }
    const b = opts!.body as Record<string, unknown>; const doc = m[1];
    const done = keys.get(String(b.idempotency_key));
    if (done) return reply({ revision: { revision_id: done, version_number: revs.get(done)!.n } });         // the same key twice is one revision
    // the real server's rule (internal/store/texture_structured_revision.go): content is the projection of body_doc, else 400. projectBody is held to the server's own function by the shared fixture (texture-body-projection.test.ts).
    if (b.body_doc && String(b.content ?? '').trim() !== '' && b.content !== projectBody(b.body_doc as never)) throw new HTTPError(400, 'HTTP 400: invalid texture revision: content must match derived body_doc projection');
    if (postMode === 'dropHang') return hang();
    if (String(b.parent_revision_id ?? '') !== head[doc]) throw new HTTPError(409, 'HTTP 409: document head changed');
    seq += 1; const id = `rev-new-${seq}`;
    revs.set(id, { id, content: String(b.content), body: b.body_doc, n: (revs.get(head[doc])?.n ?? -1) + 1 }); head[doc] = id; keys.set(String(b.idempotency_key), id);
    if (postMode === 'landHang') return hang();
    return reply({ revision: { revision_id: id, version_number: revs.get(id)!.n } });
  }
  throw new Error('unexpected ' + method + ' ' + path);
} };
const posts = (): typeof log => log.filter((l) => l.method === 'POST');

// ---------------------------------------------------------------------------------------------------------------- the fake disk
const files = new Map<string, Uint8Array>();
let writeHook: ((n: number) => Promise<void>) | null; let writes: number; let readError: boolean; let rmHook: (() => Promise<void>) | null = null;
const err = (code: string): Error => Object.assign(new Error(code), { code });
const storage = { fs: {
  readFile: async (p: string) => { if (readError && p.startsWith(PENDING_DIR)) throw err('EIO'); const b = files.get(p); if (!b) throw err('ENOENT'); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { writes += 1; const n = writes; if (writeHook) await writeHook(n); files.set(p, new Uint8Array(d)); },
  mkdir: async () => undefined,
  rm: async (p: string) => { if (rmHook) await rmHook(); let any = files.delete(p); for (const k of [...files.keys()]) if (k.startsWith(p + '/')) { files.delete(k); any = true; } if (!any) throw err('ENOENT'); },
  readdir: async (p: string) => { const n = [...files.keys()].filter((k) => k.startsWith(p + '/')).map((k) => k.slice(p.length + 1)).filter((x) => !x.includes('/')); if (n.length === 0) throw err('ENOENT'); return n; },
  stat: async (p: string) => { const b = files.get(p); if (!b) throw err('ENOENT'); return { size: b.length, isFile: true, isDirectory: false, modifiedMs: 0 }; },
} } as never;
const FILE = `${PENDING_DIR}/acct-1.json`;
type Entry = { text: string; from: string | null; base: string; key: string };
const journal = (file = FILE): Record<string, Entry> => (files.has(file) ? JSON.parse(new TextDecoder().decode(files.get(file)!)).docs : {});
const A = 'texture:d1', B = 'texture:d2';
const mk = (account = 'acct-1', timeoutMs?: number): Editor => new Editor({ net, storage, account, ...(timeoutMs ? { timeoutMs } : {}) });
const tick = (ms = 15): Promise<void> => new Promise((r) => setTimeout(r, ms));

beforeEach(() => {
  resetSeam(); files.clear(); log = []; failNext = null; postMode = 'normal'; getGate = null; postGate = null; failGets = false; seq = 0; keys = new Map(); traj = {};
  writeHook = null; writes = 0; readError = false; rmHook = null;
  revs = new Map([['r0', { id: 'r0', content: 'Hello world.', body: body('Hello world.'), n: 0 }]]); head = { d1: 'r0', d2: 'r0', d3: '' };
});
const other = (id: string, content: string, doc = 'd1'): void => { revs.set(id, { id, content, body: body(content), n: 5 }); head[doc] = id; };

// ---------------------------------------------------------------------------------------------------------------- the table
describe('who can edit', () => {
  test('editableDoc: the head of a texture document only', () => {
    expect(editableDoc('texture:abc')).toBe('abc');
    for (const a of ['texture:abc@r1', 'pub:story', 'texture:', '', 'mail:1']) expect(editableDoc(a)).toBeNull();
  });
});

describe('open', () => {
  test('T01 idle OPEN: the head is read, the field can be seeded from it', async () => {
    const i = await mk().open(A);
    expect(i.opened).toBe(true); expect(i.head).toBe('Hello world.'); expect(i.status).toBe('idle'); expect(i.hasKept).toBe(false); expect(files.size).toBe(0);
  });
  test('T02 OPEN offline: not opened, nothing kept, the field keeps what the page shows', async () => {
    failGets = true;
    const i = await mk().open(A);
    expect(i.opened).toBe(false); expect(i.hasKept).toBe(false); expect(i.status).toBe('idle'); expect(files.size).toBe(0);
  });
  test('T41 a head that moved since the last open is re-read, an entry is untouched, no conflict is assumed', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine', 'Hello world.');
    other('r1', 'Hello world. theirs'); const i = await e.open(A);
    expect(i.head).toBe('Hello world. theirs'); expect(i.hasKept).toBe(true); expect(i.status).not.toBe('conflict'); expect(journal().d1.text).toBe('Hello world. mine');
  });
});

describe('keystrokes', () => {
  test('T03 KEY over a known head: the entry carries base and from, and the page is dirty', async () => {
    const e = mk(); await e.open(A);
    const i = await e.edit(A, 'Hello world. a', 'Hello world.');
    expect(i.status).toBe('dirty'); expect(i.hasKept).toBe(true); expect(i.kept).toBe('Hello world. a');
    expect(journal().d1).toMatchObject({ text: 'Hello world. a', from: 'Hello world.', base: 'r0' });
  });
  test('T04 KEY before the head was read: the entry is on disk BEFORE any network answer; the apply reads the head once and settles the base', async () => {
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const e = mk(); const p = e.edit(A, 'Hello world. a', 'Hello world.');
    await tick();
    expect(journal().d1).toMatchObject({ text: 'Hello world. a', base: '' });                    // on disk, base not yet known
    release(); await p;
    expect(journal().d1.base).toBe('r0'); expect(log.filter((l) => l.method === 'GET' && l.path === '/api/texture/documents/d1').length).toBe(1);
  });
  test('T05 KEY equal to the head: no entry stays', async () => {
    const e = mk(); await e.open(A);
    const i = await e.edit(A, 'Hello world.', 'Hello world. x');
    expect(i.status).toBe('idle'); expect(i.hasKept).toBe(false); expect(journal()).toEqual({});
  });
  test('T06 a document with NO revision yet is a head: its first text has base "" and from ""; another writer\'s first revision is a conflict', async () => {
    head.d1 = '';
    const e = mk(); await e.edit(A, 'Mine', '');
    expect(journal().d1).toMatchObject({ base: '', from: '' });
    other('r1', 'Theirs');
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(posts().length).toBe(0); expect(journal().d1.text).toBe('Mine');
  });
  test('T06 ... and with nobody else the first save has no parent', async () => {
    head.d1 = '';
    const e = mk(); await e.edit(A, 'Mine', ''); const i = await e.flush(A);
    expect(i.status).toBe('saved'); expect(posts()[0].body!.parent_revision_id).toBeUndefined();
  });
  test('T07 KEY over an unsent text: base inherited, from = the text replaced; many keystrokes are one post', async () => {
    const e = mk();
    await e.edit(A, 'Hello world. a', 'Hello world.'); await e.edit(A, 'Hello world. ab', 'Hello world. a'); const i = await e.edit(A, 'Hello world. abc', 'Hello world. ab');
    expect(i.kept).toBe('Hello world. abc'); expect(journal().d1).toMatchObject({ base: 'r0', from: 'Hello world. ab' });
    await e.flush(A); expect(posts().length).toBe(1); expect(posts()[0].body!.content).toBe('Hello world. abc');
  });
  test('T37 two documents typed together: one writer, both entries land, however the writes are delayed', async () => {
    writeHook = async (n) => { if (n === 1) await tick(40); };
    const e = mk(); await Promise.all([e.edit(A, 'first', 'Hello world.'), e.edit(B, 'second', 'Hello world.')]);
    expect(Object.keys(journal()).sort()).toEqual(['d1', 'd2']);
  });
  test('T42 KEY over an older reading (the field showed f != the head): from is kept; the flush rebases when the head still is f, else it is a conflict', async () => {
    const e = mk(); await e.edit(A, 'typed', 'Hello world.'); expect(journal().d1.from).toBe('Hello world.');
    await e.flush(A); expect(posts().length).toBe(1);
    other('r2', 'moved on', 'd2');
    const e2 = mk(); await e2.edit(B, 'typed over old', 'Hello world.'); const i = await e2.flush(B);
    expect(i.status).toBe('conflict'); expect(posts().filter((p) => p.path.includes('/d2/')).length).toBe(0);
  });
});

describe('flush', () => {
  test('T08 FLUSH, NET-OK: post over the base, saving then saved with a derived label, the entry gone', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.');
    const i = await e.flush(A);
    expect(posts().length).toBe(1); expect(posts()[0].body!.parent_revision_id).toBe('r0'); expect(posts()[0].body!.content).toBe('Hello world. a');
    expect(typeof posts()[0].body!.idempotency_key).toBe('string'); expect(posts()[0].body!.expected_lifecycle_version).toBeUndefined();
    expect(i.status).toBe('saved'); expect(i.label).toBe('Saved · version 2'); expect(journal()).toEqual({});
  });
  test('T08 a document with a trajectory is saved against its lifecycle version', async () => {
    traj.d1 = 't1'; const e = mk(); await e.edit(A, 'Hello world. y', 'Hello world.'); await e.flush(A);
    expect(posts()[0].body!.expected_lifecycle_version).toBe(7);
  });
  test('T09 FLUSH with base unknown and the head still the field\'s text: rebase, then post', async () => {
    failGets = true; const e0 = mk(); await e0.edit(A, 'typed offline', 'Hello world.'); failGets = false;      // kept with base '' (no head could be read)
    expect(journal().d1.base).toBe('');
    const e = mk(); const i = await e.flush(A);
    expect(i.status).toBe('saved'); expect(posts()[0].body!.parent_revision_id).toBe('r0');
  });
  test('T10 FLUSH when the head is somebody else\'s: a conflict, nothing posted, the text kept, no retry', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine', 'Hello world.'); other('r1', 'Hello world. theirs');
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(i.label).toContain('changed somewhere else'); expect(posts().length).toBe(0); expect(i.kept).toBe('Hello world. mine'); expect(journal().d1.text).toBe('Hello world. mine');
    await e.flush(A); expect(posts().length).toBe(0);
  });
  test('T11 NET-409 after our read: the head is read again; saved when it holds our text, else a conflict', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine', 'Hello world.');
    failNext = () => { other('r1', 'Hello world. mine'); return new HTTPError(409, 'HTTP 409'); };
    expect((await e.flush(A)).status).toBe('saved'); expect(journal()).toEqual({});
    const e2 = mk(); await e2.edit(B, 'Hello world. x', 'Hello world.');
    failNext = () => { other('r9', 'someone else', 'd2'); return new HTTPError(409, 'HTTP 409'); };
    const i = await e2.flush(B); expect(i.status).toBe('conflict'); expect(journal().d2.text).toBe('Hello world. x');
  });
  test('T12 NET-OFFLINE: offline, the text on disk, the words say so', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); failNext = () => new TypeError('network down');
    const i = await e.flush(A);
    expect(i.status).toBe('offline'); expect(i.label).toContain('kept on this phone'); expect(i.kept).toBe('Hello world. a'); expect(journal().d1.text).toBe('Hello world. a');
  });
  test('T13 NET-REFUSED: failed with a sentence, text kept', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); failNext = () => new HTTPError(403, '403');
    const i = await e.flush(A);
    expect(i.status).toBe('failed'); expect(i.label).toBe('Not saved: This device is not allowed to save here.'); expect(journal().d1.text).toBe('Hello world. a');
  });
  test('T14 NET-LETGO: nothing changes, dirty, never offline', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); failNext = () => new Error('the answer was let go before this fetch');
    const i = await e.flush(A);
    expect(i.status).toBe('dirty'); expect(journal().d1.text).toBe('Hello world. a');
  });
  test('T15 signed out (401): failed with the sign-in sentence, text kept', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); failNext = () => new SignedOutError('refused', 'refused');
    const i = await e.flush(A);
    expect(i.status).toBe('failed'); expect(i.label).toBe('Not saved: Sign in again to save.'); expect(journal().d1.text).toBe('Hello world. a');
  });
  test('T39 a hung network ends in the editor\'s own timeout; the entry was on disk before; offline', async () => {
    const e = mk('acct-1', 30); await e.edit(A, 'Hello world. a', 'Hello world.'); postMode = 'dropHang';
    const i = await e.flush(A);
    expect(i.status).toBe('offline'); expect(journal().d1.text).toBe('Hello world. a');
  });
});

describe('a keystroke during a save, and what is derived', () => {
  test('T16 KEY while the POST is out: the new text is on disk at once; after the post the entry STAYS and the page is dirty, never saved', async () => {
    const e = mk(); await e.edit(A, 'one', 'Hello world.');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    const saving = e.flush(A); await tick();
    expect((await e.info(A)).status).toBe('saving');
    const typed = e.edit(A, 'one two', 'one'); await tick();
    expect(journal().d1.text).toBe('one two');                                                    // on disk while the post is out
    release(); await saving; const i = await typed;
    expect(i.status).toBe('dirty'); expect(i.kept).toBe('one two'); expect(journal().d1).toMatchObject({ text: 'one two', base: 'rev-new-1', from: 'one' });
    await e.flush(A); expect(posts().map((p) => [p.body!.content, p.body!.parent_revision_id])).toEqual([['one', 'r0'], ['one two', 'rev-new-1']]);
  });
  test('T40 NET-OK for t while the entry holds t2: the entry is rewritten with the new base, last = saved but the status is dirty (derived)', async () => {
    const e = mk(); await e.edit(A, 'one', 'Hello world.');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    const saving = e.flush(A); await tick(); const typed = e.edit(A, 'one two', 'one'); await tick(); release(); await saving; await typed;
    const i = await e.info(A); expect(i.status).toBe('dirty'); expect(i.label).not.toContain('Saved'); expect(journal().d1.base).toBe('rev-new-1');
  });
  test('T18 FLUSH again behind a post: nothing to send once the entry is gone', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.');
    const [x, y] = await Promise.all([e.flush(A), e.flush(A)]);
    expect(posts().length).toBe(1); expect(x.status === 'saved' || y.status === 'saved').toBe(true);
  });
  test('T19 FLUSH (the retry) after an offline attempt posts once and the entry goes', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); failNext = () => new TypeError('down'); await e.flush(A);
    const i = await e.flush(''); expect(i.waiting).toBe(0); expect(journal()).toEqual({}); expect(posts().length).toBe(2);
  });
  test('T20 KEY while offline: the entry is the new text; the status goes back to dirty (the Swift editor did), so the idle pause saves it again', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); failNext = () => new TypeError('down'); await e.flush(A);
    const i = await e.edit(A, 'Hello world. ab', 'Hello world. a');
    expect(i.status).toBe('dirty'); expect(i.kept).toBe('Hello world. ab'); expect(journal().d1.text).toBe('Hello world. ab');
  });
});

describe('conflict', () => {
  const inConflict = async (): Promise<Editor> => { const e = mk(); await e.edit(A, 'Hello world. mine', 'Hello world.'); other('r1', 'Hello world. theirs'); await e.flush(A); return e; };
  test('T21 KEY in a conflict: the text is kept, the conflict stays until the listener chooses', async () => {
    const e = await inConflict(); const i = await e.edit(A, 'Hello world. mine more', 'Hello world. mine');
    expect(i.status).toBe('conflict'); expect(journal().d1.text).toBe('Hello world. mine more');
  });
  test('T22 FLUSH in a conflict sends nothing and `waiting` does not count it', async () => {
    const e = await inConflict(); const i = await e.flush('');
    expect(i.waiting).toBe(0); expect(posts().length).toBe(0); expect((await e.flush(A)).status).toBe('conflict');
  });
  test('T23 OVERWRITE: re-based on the head as it is now and saved', async () => {
    const e = await inConflict(); const i = await e.overwrite(A);
    expect(i.status).toBe('saved'); expect(posts()[0].body!.parent_revision_id).toBe('r1'); expect(posts()[0].body!.content).toBe('Hello world. mine'); expect(journal()).toEqual({});
  });
  test('T24 DISCARD: the head wins, the entry goes, the next open re-reads', async () => {
    const e = await inConflict(); const i = await e.discard(A);
    expect(i.status).toBe('idle'); expect(i.hasKept).toBe(false); expect(journal()).toEqual({}); expect((await e.open(A)).head).toBe('Hello world. theirs');
  });
  test('T25 DISCARD with a newer KEY typed after it was asked: the newer entry is NOT deleted', async () => {
    const e = await inConflict();
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const gone = e.discard(A); await tick();                                                      // queued behind nothing but waiting on a gated read? (the door is free: this checks the order)
    const typed = e.edit(A, 'Hello world. newer', 'Hello world. theirs'); release(); await gone; const i = await typed;
    expect(i.hasKept).toBe(true); expect(i.kept).toBe('Hello world. newer'); expect(journal().d1.text).toBe('Hello world. newer');
  });
  test('T17 a DISCARD queued behind a post that ends in 409: idle at the end, not a late conflict', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine', 'Hello world.');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    failNext = () => { other('r1', 'theirs'); return new HTTPError(409, '409'); };
    const saving = e.flush(A); const gone = e.discard(A); await tick(); release(); await saving; const i = await gone;
    expect(i.status).toBe('idle'); expect(journal()).toEqual({});
  });
});

describe('the automatic writer', () => {
  test('T26 AUTO with no entry: one run op keeps, saves and leaves nothing', async () => {
    const e = mk(); expect(await e.writeAutomatic(A, 'Hello world. auto', 'Hello world.')).toBe('saved');
    expect(posts()[0].body!.content).toBe('Hello world. auto'); expect(journal()).toEqual({});
  });
  test('T27 AUTO with the listener\'s text there: busy, nothing replaced or written', async () => {
    const e = mk(); await e.edit(A, 'listener text', 'Hello world.');
    expect(await e.writeAutomatic(A, 'automatic', 'Hello world.')).toBe('busy'); expect(journal().d1.text).toBe('listener text'); expect(posts().length).toBe(0);
  });
  test('T27 ... also when the listener types in the window of the automatic write (before its check)', async () => {
    const e = mk(); const typed = e.edit(A, 'listener first', 'Hello world.');
    expect(await e.writeAutomatic(A, 'automatic', 'Hello world.')).toBe('busy'); await typed; expect(journal().d1.text).toBe('listener first');
  });
  test('T28 AUTO whose save fails drops only its own entry; a listener\'s text typed behind it stays', async () => {
    const e = mk(); failNext = () => new HTTPError(403, '403');
    const auto = e.writeAutomatic(A, 'automatic text', 'Hello world.'); await tick(); const typed = e.edit(A, 'listener typed meanwhile', 'automatic text');
    expect(await auto).toBe('failed'); await typed; expect(journal().d1.text).toBe('listener typed meanwhile');
  });
  test('T28 AUTO that fails alone leaves nothing behind', async () => {
    const e = mk(); failNext = () => new HTTPError(403, '403'); expect(await e.writeAutomatic(A, 'automatic', 'Hello world.')).toBe('failed'); expect(journal()).toEqual({});
  });
});

describe('sign-out, account change, kill', () => {
  test('T29 SIGNOUT begins: dead at once, queued ops answer nothing, the network and the writer refuse; the wipe then empties the whole folder even with a write in flight', async () => {
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.');
    writeHook = async () => { await gate; };
    const typed = e.edit(A, 'Hello world. ab', 'Hello world. a'); await tick();                    // a write is in flight
    e.kill(); expect(e.dead).toBe(true);
    release(); await typed.catch(() => undefined); await e.settleWrites();
    const gets = log.length; await e.flush(A); expect(log.length).toBe(gets); expect(posts().length).toBe(0);      // dead: the door and the network refuse
    files.set(`${PENDING_DIR}/acct-9.json`, new TextEncoder().encode('{}')); files.set(`${PENDING_DIR}/acct-1.bad`, new TextEncoder().encode('x'));
    await wipeTextureEdit(storage);
    expect([...files.keys()].filter((k) => k.startsWith(PENDING_DIR))).toEqual([]);
  });
  test('T33 the network answers after kill(): no effect', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    const saving = e.flush(A); await tick(); e.kill(); release(); await saving.catch(() => undefined);
    expect((await e.info(A)).status).not.toBe('saved');
  });
  test('T38 the same account keeps its instance when the key rotates or the id is read again', async () => {
    bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'session' ? { found: true, value: 'key-1-not-real', computerId: 'c1' } : { state: 'present' }) } as never);
    forgetTextureEdit(); await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'session' ? { found: true, value: 'key-2-rotated', computerId: 'c1' } : { state: 'present' }) } as never);
    expect((await textureEditInfo(A, storage)).kept).toBe('Hello world. a');
  });
});

describe('the module: one instance per account', () => {
  const session = (key: string, computer: string): void => bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'session' ? { found: true, value: key, computerId: computer } : { state: 'present' }) } as never);
  test('T31 no account id yet (a key but no computer): the read-only facade says so and keeps nothing', async () => {
    forgetTextureEdit(); session('key-1-not-real', '');
    const i = await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    expect(i.writable).toBe(false); expect(i.label).toContain('account'); expect(files.size).toBe(0); expect(i.hasKept).toBe(false);
  });
  test('T32 the id becomes known: an instance is built and loads its own file', async () => {
    forgetTextureEdit(); session('key-1-not-real', 'c1');
    await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    forgetTextureEdit();                                                                            // a new launch
    const i = await textureEditInfo(A, storage);
    expect(i.writable).toBe(true); expect(i.hasKept).toBe(true); expect(i.kept).toBe('Hello world. a');
  });
  test('T30 ACCOUNT: another id read at a call is treated as a sign-out: the folder is emptied, the new instance starts empty', async () => {
    forgetTextureEdit(); session('key-1-not-real', 'c1'); await textureEdit(A, 'first account text', 'Hello world.', storage);
    expect([...files.keys()].some((k) => k.startsWith(PENDING_DIR))).toBe(true);
    session('key-2-not-real', 'c2');
    const i = await textureEditInfo(A, storage);
    expect(i.hasKept).toBe(false); expect([...files.values()].map((b) => new TextDecoder().decode(b)).join('')).not.toContain('first account text');
  });
  test('T29 the sign-out hook kills the instance when beginSignOut runs', async () => {
    forgetTextureEdit(); session('key-1-not-real', 'c1'); await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    await beginSignOut(); nextGeneration();
    await wipeTextureEdit(storage);
    expect([...files.keys()].filter((k) => k.startsWith(PENDING_DIR))).toEqual([]);
    expect((await textureEditInfo(A, storage)).hasKept).toBe(false);
  });
  test('T31 the open source is read-only too without an id', async () => {
    forgetTextureEdit(); session('key-1-not-real', ''); const i = await textureEditOpen(A, storage); expect(i.writable).toBe(false);
    expect((await textureFlush(A, storage)).writable).toBe(false);
  });
});

describe('launch: what is on disk is what comes back', () => {
  test('T34 LAUNCH with this account\'s file: hasKept, kept, the offline words until an attempt, waiting counts it', async () => {
    await mk().edit(A, 'Hello world. unsent', 'Hello world.');
    const i = await mk().info(A);                                                                  // a second instance on the same files
    expect(i.hasKept).toBe(true); expect(i.kept).toBe('Hello world. unsent'); expect(i.status).toBe('offline'); expect(i.label).toContain('kept on this phone'); expect(i.waiting).toBe(1);
  });
  test('T34 an EMPTY kept text is a kept text (everything deleted): hasKept, and it survives a launch', async () => {
    failGets = true; await mk().edit(A, '', 'Hello world.');
    expect((await mk().info(A)).hasKept).toBe(true); expect((await mk().info(A)).kept).toBe('');
  });
  test('T35 LAUNCH with an unreadable file (I/O error): left alone, nothing written over it, found when it can be read', async () => {
    await mk().edit(A, 'Hello world. unsent', 'Hello world.'); const before = new TextDecoder().decode(files.get(FILE)!);
    readError = true; const e = mk(); const i = await e.info(A); expect(i.hasKept).toBe(false);
    await e.edit(B, 'typed meanwhile', 'Hello world.'); expect(new TextDecoder().decode(files.get(FILE)!)).toBe(before);
    readError = false; expect((await e.info(A)).kept).toBe('Hello world. unsent');
    await tick(); expect(Object.keys(journal()).sort()).toEqual(['d1', 'd2']);
  });
  test('T36 LAUNCH with corrupt content: set aside as ONE .bad (replaced, not accumulated), a notice, nothing kept', async () => {
    files.set(FILE, new TextEncoder().encode('{not json')); files.set(`${PENDING_DIR}/acct-1.bad`, new TextEncoder().encode('an older bad one'));
    const i = await mk().info(A);
    expect(i.hasKept).toBe(false); expect(i.notice).toBe('Unsent text could not be read and was set aside.');
    expect(new TextDecoder().decode(files.get(`${PENDING_DIR}/acct-1.bad`)!)).toBe('{not json'); expect([...files.keys()].filter((k) => k.endsWith('.bad')).length).toBe(1);
  });
});

describe('kills', () => {
  test('K1 the last keystroke only: two keystrokes, killed before the second write lands: the first is on disk', async () => {
    const e = mk(); await e.edit(A, 'first', 'Hello world.');
    let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    writeHook = async () => { await gate; };
    void e.edit(A, 'second', 'first'); await tick();                                                // the write of "second" has not landed: the process dies here
    expect((await mk().info(A)).kept).toBe('first'); release();
  });
  test('K2 killed after the journal write, before the head was read: relaunch posts it', async () => {
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const e = mk(); void e.edit(A, 'Hello world. a', 'Hello world.'); await tick(); e.kill(); getGate = null;
    const e2 = mk(); expect((await e2.info(A)).kept).toBe('Hello world. a'); const i = await e2.flush(A);
    expect(i.status).toBe('saved'); expect(posts()[0].body!.content).toBe('Hello world. a'); release();
  });
  test('K3 killed during the post: it did NOT land -> posted again with the SAME key; it DID land -> acknowledged, no second revision', async () => {
    postMode = 'dropHang'; const e = mk('acct-1', 30); await e.edit(A, 'Hello world. a', 'Hello world.'); await e.flush(A); const key = journal().d1.key; e.kill();
    postMode = 'normal'; const i = await mk().flush(A);
    expect(i.status).toBe('saved'); expect(posts().filter((p) => p.body!.idempotency_key === key).length).toBe(2); expect(revs.size).toBe(2);
    // landed:
    head.d2 = 'r0'; postMode = 'landHang'; const f = mk('acct-1', 30); await f.edit(B, 'Hello world. b', 'Hello world.'); await f.flush(B); f.kill();
    const before = revs.size; postMode = 'normal'; const j = await mk().flush(B);
    expect(j.status).toBe('saved'); expect(revs.size).toBe(before); expect(journal().d2).toBeUndefined();
  });
  test('K4a t posting, t2 typed over it, the post LANDED, killed: the head holds t = from -> rebase and post t2 (no conflict)', async () => {
    const e = mk('acct-1', 400); await e.edit(A, 'one', 'Hello world.');
    postMode = 'landHang'; const saving = e.flush(A); await tick(40);
    void e.edit(A, 'one two', 'one'); await tick(); e.kill(); await saving.catch(() => undefined);
    expect(head.d1).toBe('rev-new-1'); expect(journal().d1).toMatchObject({ text: 'one two', base: 'r0', from: 'one' });
    postMode = 'normal'; const i = await mk().flush(A);
    expect(i.status).toBe('saved'); expect(posts().slice(-1)[0].body).toMatchObject({ content: 'one two', parent_revision_id: 'rev-new-1' });
  });
  test('K4b t posting, t2 typed over it, the post did NOT land, killed: the head is still the base -> post t2 on it (no conflict against the person\'s own text)', async () => {
    const e = mk('acct-1', 400); await e.edit(A, 'one', 'Hello world.');
    postMode = 'dropHang'; const saving = e.flush(A); await tick(40);
    void e.edit(A, 'one two', 'one'); await tick(); e.kill(); await saving.catch(() => undefined);
    expect(head.d1).toBe('r0');
    postMode = 'normal'; const i = await mk().flush(A);
    expect(i.status).toBe('saved'); expect(posts().slice(-1)[0].body).toMatchObject({ content: 'one two', parent_revision_id: 'r0' });
  });
  test('K5 killed after NET-OK, before the entry was deleted: the head holds t -> acknowledged', async () => {
    const e = mk(); await e.edit(A, 'Hello world. a', 'Hello world.'); const copy = new Uint8Array(files.get(FILE)!);
    await e.flush(A); files.set(FILE, copy);                                                       // the file as it was before the delete: the kill came first
    const i = await mk().flush(A); expect(i.status).toBe('saved'); expect(posts().length).toBe(1); expect(journal()).toEqual({});
  });
  test('K6 killed with a conflict: the relaunch is dirty, the next flush finds the conflict again', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine', 'Hello world.'); other('r1', 'Hello world. theirs'); await e.flush(A); e.kill();
    const e2 = mk(); expect((await e2.info(A)).hasKept).toBe(true); expect((await e2.flush(A)).status).toBe('conflict');
  });
  test('K7 killed with the key unreadable at relaunch: the file is untouched, and the instance is built when the id can be read', async () => {
    const session = (computer: string): void => bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'session' ? { found: true, value: 'key-1-not-real', computerId: computer } : { state: 'present' }) } as never);
    forgetTextureEdit(); session('c1'); await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    forgetTextureEdit(); session('');
    const i = await textureEditInfo(A, storage); expect(i.writable).toBe(false); expect(journal(`${PENDING_DIR}/${[...files.keys()][0].split('/').pop()}`).d1.text).toBe('Hello world. a');
    session('c1'); forgetTextureEdit(); expect((await textureEditInfo(A, storage)).kept).toBe('Hello world. a');
  });
  test('K8 two documents: both entries come back', async () => {
    const e = mk(); await Promise.all([e.edit(A, 'first', 'Hello world.'), e.edit(B, 'second', 'Hello world.')]); e.kill();
    const e2 = mk(); expect((await e2.info(A)).kept).toBe('first'); expect((await e2.info(B)).kept).toBe('second');
  });
});

describe('the background', () => {
  test('T43 the app goes to the background mid-post: the request ends as offline or let-go, the entry stays on disk', async () => {
    const e = mk('acct-1', 30); await e.edit(A, 'Hello world. a', 'Hello world.'); postMode = 'dropHang';
    const i = await e.flush(A);
    expect(['offline', 'dirty']).toContain(i.status); expect(journal().d1.text).toBe('Hello world. a'); expect(i.hasKept).toBe(true);
  });
});

describe('the review of 2026-11-24 (seat 1, exact-editor)', () => {
  const NEW = 'texture:d3';
  const OWNER = 'Hello\nThis is my first time using texture editing on autoradio 1';
  test("W1 the owner's text on a new document (no revision, no body_doc) is saved: the server accepts it (it was 400 content must match derived body_doc projection)", async () => {
    const e = mk(); await e.open(NEW);
    await e.edit(NEW, OWNER, ''); const i = await e.flush(NEW);
    expect(i.status).toBe('saved'); expect(posts().length).toBe(1);
    expect(posts()[0].body!.content).toBe(OWNER); expect(posts()[0].path).toBe('/api/texture/documents/d3/revisions'); expect(posts()[0].body!.parent_revision_id).toBeUndefined();
    expect(revs.get(head.d3)!.content).toBe(OWNER);
    const top = ((posts()[0].body!.body_doc as { doc: { content: { content: { type: string }[] }[] } }).doc.content);
    expect(top.length).toBe(1); expect(top[0].content.map((x) => x.type)).toEqual(['text', 'hard_break', 'text']);
  });
  test('W2 an Enter at the end, then more typing: saved, and the next keystroke is no conflict with our own save', async () => {
    const e = mk(); await e.open(NEW);
    await e.edit(NEW, 'Hello\n', ''); expect((await e.flush(NEW)).status).toBe('saved');
    await e.edit(NEW, 'Hello\nThere', 'Hello\n'); const i = await e.flush(NEW);
    expect(i.status).toBe('saved'); expect(revs.get(head.d3)!.content).toBe('Hello\nThere');
  });
  test('W3 marks in the text: saved, and the next keystroke is no conflict (the server keeps the text without the marks)', async () => {
    const e = mk(); await e.open(NEW);
    await e.edit(NEW, 'some **bold** words', ''); expect((await e.flush(NEW)).status).toBe('saved');
    expect(revs.get(head.d3)!.content).toBe('some bold words');
    await e.edit(NEW, 'some **bold** words here', 'some **bold** words'); const i = await e.flush(NEW);
    expect(i.status).toBe('saved');
  });
  test('R9a the posted body keeps the id of an unchanged node (the body merge)', async () => {
    const e = mk(); await e.edit(A, 'Hello world. more', 'Hello world.'); await e.flush(A);
    const doc = (posts()[0].body!.body_doc as { doc: { content: { attrs: { id: string } }[] } }).doc;
    expect(doc.content[0].attrs.id).toBe('p');
  });
  test('R9b an address that is not the head of a document edits, flushes and discards nothing', async () => {
    const e = mk();
    for (const a of ['texture:d1@r0', 'pub:story']) {
      const i = await e.edit(a, 'x', null); await e.flush(a); await e.discard(a); await e.overwrite(a);
      expect(i.editable).toBe(false); expect(i.status).toBe('idle');
    }
    expect(posts().length).toBe(0); expect(files.size).toBe(0);
  });
  const session = (key: string, computer: string): void => bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'session' ? { found: true, value: key, computerId: computer } : { state: 'present' }) } as never);
  test('R3 `waiting` counts only text a retry can send: a draft being typed is not waiting, an offline one is', async () => {
    const e = mk(); const typing = await e.edit(A, 'Hello world. a', 'Hello world.');
    expect(typing.waiting).toBe(0);
    expect((await e.flush('')).waiting).toBe(0); expect(posts().length).toBe(0);                // the retry does not post a draft that was never tried
    failNext = () => new TypeError('down'); await e.flush(A);
    expect((await e.info(A)).waiting).toBe(1);
    expect((await e.flush('')).waiting).toBe(0); expect(posts().length).toBe(2);
  });
  test('R4 a failed automatic write leaves no conflict and no status behind: the listener can still edit that document', async () => {
    const e = mk();
    expect(await e.writeAutomatic(A, 'auto text', 'text the head never had')).toBe('failed');
    const i = await e.info(A);
    expect(i.status).not.toBe('conflict'); expect(i.hasKept).toBe(false);
    await e.edit(A, 'Hello world. mine', 'Hello world.'); const saved = await e.flush(A);
    expect(saved.status).toBe('saved'); expect(posts().length).toBe(1);
  });
  test('R6 the 409 that finds our own text already there carries a keystroke typed during the post into the entry (base, from)', async () => {
    const e = mk(); await e.edit(A, 'Hello world. t1', 'Hello world.');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    const f = e.flush(A); await tick();
    other('r1', 'Hello world. t1'); failNext = () => new HTTPError(409, 'HTTP 409: document head changed');
    const k2 = e.edit(A, 'Hello world. t2', 'Hello world. t1'); const k3 = e.edit(A, 'Hello world. t3', 'Hello world. t2'); await tick();
    release(); await Promise.all([f, k2, k3]);
    expect(journal().d1).toMatchObject({ text: 'Hello world. t3', base: 'r1', from: 'Hello world. t1' });
    expect((await e.flush(A)).status).toBe('saved');
  });
  test('R2 the wipe waits for the write of an instance the sign-out already killed', async () => {
    forgetTextureEdit(); session('key-1-not-real', 'c1'); await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    writeHook = async () => { await tick(80); };
    const p = textureEdit(A, 'Hello world. ab', 'Hello world. a', storage); await tick(30);
    await beginSignOut(); nextGeneration();
    await wipeTextureEdit(storage);
    await p.catch(() => undefined); await tick(120);                                             // whatever was in flight has landed by now
    expect([...files.keys()].filter((k) => k.startsWith(PENDING_DIR))).toEqual([]);
  });
  test('R2b the wipe waits for the set-aside copy of an unreadable file that a killed instance was writing', async () => {
    forgetTextureEdit(); session('key-1-not-real', 'c1'); await textureEdit(A, 'Hello world. a', 'Hello world.', storage);
    const file = [...files.keys()].find((k) => k.startsWith(PENDING_DIR))!;
    files.set(file, new TextEncoder().encode('not json at all'));
    forgetTextureEdit();
    writeHook = async () => { await tick(80); };
    const p = textureEditInfo(A, storage); await tick(30);                                       // load() is writing the .bad copy
    await beginSignOut(); nextGeneration();
    await wipeTextureEdit(storage);
    await p.catch(() => undefined); await tick(120);
    expect([...files.keys()].filter((k) => k.startsWith(PENDING_DIR))).toEqual([]);
  });
  test('R5 two calls meeting an account change do not undo each other: the new account keeps what it writes', async () => {
    forgetTextureEdit(); session('key-1-not-real', 'c1'); await textureEdit(A, 'old account text', 'Hello world.', storage);
    session('key-2-not-real', 'c2'); rmHook = async () => { await tick(30); };
    await Promise.all([textureEditInfo(A, storage), textureEdit(A, 'new account text', 'Hello world.', storage)]);
    rmHook = null;
    const kept = [...files.keys()].filter((k) => k.startsWith(PENDING_DIR));
    expect(kept.length).toBe(1); expect(JSON.stringify(journal(kept[0]))).toContain('new account text');
    expect(JSON.stringify([...files.values()].map((v) => new TextDecoder().decode(v)))).not.toContain('old account text');
  });
});

// keep the type import used
export type { Info };
