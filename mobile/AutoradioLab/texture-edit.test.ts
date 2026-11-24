// The Texture editor in shared code (texture-edit.ts). The state model is in that file's header; this is its failure list, written before the code. Each line has a test below.
//  E1  a burst of edits is one post.                                          E2  a revision, a story or nothing is not editable: no session, no network.
//  E3  an edit equal to the head posts nothing and clears what was kept.      E4  the post names the revision the text descends from (parent), the Markdown and its body_doc, an idempotency key.
//  E5  a document with a trajectory is saved against its lifecycle version.   E6  offline keeps the text on the phone; a relaunch posts it once; nothing is posted twice.
//  E7  typing while a save is in flight waits behind it, then is saved right after, on the new head.  E8  the head moved (409, or a head the kept text did not descend from): nothing is posted over it,
//      the text stays kept and readable, `conflict`, no retry; the same text as the head is not a conflict (a lost reply).
//  E9  reopening shows the kept text (`info.kept`), not the head.             E10 a refused save (400, 403) fails with a sentence and keeps the text.
//  E11 signed out: "Sign in again to save", text kept.                        E12 sign-out removes memory and file, and a save in flight afterwards keeps and posts nothing.
//  E15 the editor opens on the head the server holds NOW (open), and a text typed over it is posted over that head; a head that moved before the first keystroke, seen from an older field, is a conflict.
//  E16 a first edit made offline is kept with the text the field was seeded from; online later it is posted when the head still is that text, else it is a conflict.
//  E17 edits fired together, with no head read yet, keep the last one and read the head once.   E18 a sign-out during a head read or a file read leaves nothing, for the next account too.
//  E19 `waiting` does not count a document in conflict.   E20 an overwrite across a sign-out writes nothing; while a sign-out is under way the editor shows nothing kept.
//  E21 Done (flush) typed behind a first edit still on its way to the head is posted AFTER it, not lost.   E22 a discard queued behind a save that gets a 409 ends idle, not in conflict.
//  E23 an EMPTY document is a text, not "no seed": over a head that moved it is a conflict; offline it is posted when the head still is empty.
//  E24 the contract's way: every keystroke names what the field showed before it, so a second save after a first is not a false conflict.
//  E25 calls queued or begun across a sign-out leave nothing; a kept file written under another key is not the next account's.
//  E26 a key that cannot be read (no handle yet, a locked Keychain) or is absent keeps the unsent file, loads nothing, and writes nothing over it; it is loaded when the key can be read; a key rotated for the same account (same computer) keeps it.
//  E27 the latest keystroke is on disk BEFORE it waits: during a save in flight, during a hung link (the editor's own short timeout), and after a kill.   E28 two documents written at once leave both in the file.   E29 a let-go answer is not "offline".
//  E13 discard drops the kept text; overwrite is the listener's act: it re-bases on the head and posts.   E14 `waiting` counts the documents with kept text.
import { beforeEach, describe, expect, test } from 'bun:test';
import { Editor, editableDoc, PENDING_FILE, CONFLICT_WORDS, type Net } from './texture-edit.ts';
import { HTTPError, SignedOutError, resetSeam, nextGeneration, beginSignOut, abortSignOut, bindNative } from './api.ts';

type Rev = { id: string; content: string; body: unknown; n: number };
let revs: Map<string, Rev>; let head: Record<string, string>; let traj: Record<string, string>; let log: { method: string; path: string; body?: Record<string, unknown> }[];
let writes = 0; let slowFirstWrite = false; let failGets = false; let getGate: Promise<void> | null = null; let postGate: Promise<void> | null = null; let readGate: Promise<void> | null = null; let getCount = 0; let failNext: (() => Error) | null; let seq = 0; let lifecycleVersion = 7;
const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (readGate) await readGate; if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { writes += 1; if (slowFirstWrite && writes === 1) await new Promise((r) => setTimeout(r, 40)); files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
} } as never;
let signedKey = 'key-a-not-real'; let signedComputer = 'c1'; let identityMode: 'normal' | 'unreadable' | 'absent' = 'normal';
const reader = { available: true, later: async (r: { op?: string }) => (r.op === 'session' ? (identityMode === 'normal' ? { found: true, value: signedKey, computerId: signedComputer } : { found: false }) : { state: identityMode === 'absent' ? 'absent' : identityMode === 'unreadable' ? 'unreadable' : 'present' }) };
const kept = (): Record<string, { text: string; base: string; key: string; seed: string | null }> => (files.has(PENDING_FILE) ? JSON.parse(new TextDecoder().decode(files.get(PENDING_FILE)!)).docs : {});
const body = (t: string): unknown => ({ schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'd' }, content: [{ type: 'paragraph', attrs: { id: 'p' }, content: [{ type: 'text', text: t }] }] } });
const net: Net = { json: async (path, opts) => {
  const method = opts?.method ?? 'GET';
  log.push({ method, path, body: opts?.body as Record<string, unknown> | undefined });
  if (method === 'GET') { getCount += 1; if (getGate) await getGate; if (failGets) throw new TypeError('network down'); }
  if (method === 'POST' && postGate) await postGate;
  if (failNext && method === 'POST') { const e = failNext(); failNext = null; throw e; }
  let m = path.match(/^\/api\/texture\/documents\/([^/]+)$/);
  if (m && method === 'GET') return { body: { document: { doc_id: m[1], current_revision_id: head[m[1]], ...(traj[m[1]] ? { trajectory_id: traj[m[1]] } : {}) } }, serverTime: null, raw: '' };
  m = path.match(/^\/api\/texture\/revisions\/([^/]+)$/);
  if (m) { const r = revs.get(m[1])!; return { body: { revision: { revision_id: r.id, content: r.content, body_doc: r.body, version_number: r.n } }, serverTime: null, raw: '' }; }
  m = path.match(/^\/api\/texture\/lifecycle\/([^/]+)$/);
  if (m) return { body: { trajectory: { lifecycle_version: lifecycleVersion }, current_document_head: { revision_id: head['d1'] } }, serverTime: null, raw: '' };
  m = path.match(/^\/api\/texture\/documents\/([^/]+)\/revisions$/);
  if (m && method === 'POST') {
    const b = opts!.body as Record<string, unknown>;
    if (b.parent_revision_id !== head[m[1]]) throw new HTTPError(409, 'HTTP 409: document head changed');
    seq += 1; const id = `rev-new-${seq}`;
    revs.set(id, { id, content: String(b.content), body: b.body_doc, n: revs.get(head[m[1]])!.n + 1 }); head[m[1]] = id;
    return { body: { revision: { revision_id: id, version_number: revs.get(id)!.n } }, serverTime: null, raw: '' };
  }
  throw new Error('unexpected ' + method + ' ' + path);
} };
const posts = (): typeof log => log.filter((l) => l.method === 'POST');
const mk = (): Editor => new Editor({ net, storage });
const A = 'texture:d1';
beforeEach(() => {
  resetSeam(); bindNative(reader as never); signedKey = 'key-a-not-real'; signedComputer = 'c1'; identityMode = 'normal'; files.clear(); log = []; failNext = null; writes = 0; slowFirstWrite = false; failGets = false; getGate = null; postGate = null; readGate = null; getCount = 0; seq = 0; traj = {};
  revs = new Map([['r0', { id: 'r0', content: 'Hello world.', body: body('Hello world.'), n: 0 }]]); head = { d1: 'r0', d2: 'r0' };
});

describe('who can edit', () => {
  test('E2 only the head of a texture document', async () => {
    expect(editableDoc('texture:abc')).toBe('abc');
    for (const a of ['texture:abc@r1', 'pub:story', 'texture:', '', 'mail:1']) expect(editableDoc(a)).toBeNull();
    const e = mk();
    const i = await e.edit('pub:story', 'x');
    expect(i.editable).toBe(false); expect(log).toEqual([]);
  });
});

describe('saving', () => {
  test('E1 a burst of edits is one post; E4 it carries parent, content, body_doc and a key', async () => {
    const e = mk();
    for (const t of ['Hello world. A', 'Hello world. Ab', 'Hello world. Abc']) await e.edit(A, t);
    expect(posts().length).toBe(0);
    const i = await e.flush(A);
    expect(posts().length).toBe(1);
    const b = posts()[0].body!;
    expect(b.parent_revision_id).toBe('r0'); expect(b.content).toBe('Hello world. Abc');
    expect(typeof b.idempotency_key).toBe('string'); expect((b.body_doc as { schema: string }).schema).toBe('choir.texture_doc.v1');
    expect(i.status).toBe('saved'); expect(i.label).toBe('Saved · version 2'); expect(kept()).toEqual({});
  });
  test('E3 an edit equal to the head posts nothing and clears what was kept', async () => {
    const e = mk();
    await e.edit(A, 'Hello world. x'); expect(Object.keys(kept())).toEqual(['d1']);
    const i = await e.edit(A, 'Hello world.');
    expect(i.status).toBe('idle'); expect(kept()).toEqual({});
    await e.flush(A); expect(posts().length).toBe(0);
  });
  test('E5 a document with a trajectory carries the lifecycle version', async () => {
    traj.d1 = 't1';
    const e = mk(); await e.edit(A, 'Hello world. y'); await e.flush(A);
    expect(posts()[0].body!.expected_lifecycle_version).toBe(7);
    expect(posts()[0].body!.parent_revision_id).toBe('r0');
  });
  test('E7 typing while a save is in flight is saved right after, on the new head', async () => {
    const e = mk(); await e.edit(A, 'one');
    const slow = net.json; let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    (net as { json: Net['json'] }).json = async (p, o) => { if (o?.method === 'POST') await gate; return slow(p, o); };
    const first = e.flush(A);
    await new Promise((r) => setTimeout(r, 5));
    const typed = e.edit(A, 'one two');                              // queued behind the save in flight: it is kept when the post is over, on the new head
    release(); await first; await typed; await e.flush(A);
    (net as { json: Net['json'] }).json = slow;
    expect(posts().map((p) => [p.body!.content, p.body!.parent_revision_id])).toEqual([['one', 'r0'], ['one two', 'rev-new-1']]);
    expect(kept()).toEqual({});
  });
});

describe('offline and relaunch', () => {
  test('E6 offline keeps the text; a relaunch posts it once', async () => {
    const e = mk(); await e.edit(A, 'Hello world. offline');
    failNext = () => new TypeError('network down');
    const i = await e.flush(A);
    expect(i.status).toBe('offline'); expect(kept().d1.text).toBe('Hello world. offline'); expect(kept().d1.base).toBe('r0');
    const e2 = mk();                                  // a new process: nothing in memory
    const j = await e2.info(A);
    expect(j.kept).toBe('Hello world. offline'); expect(j.waiting).toBe(1);
    const k = await e2.flush('');                     // retry whatever is waiting, on any page
    expect(k.waiting).toBe(0); expect(kept()).toEqual({});
    await e2.flush(''); expect(posts().filter((p) => p.body && !p.body.__x).length).toBe(2);   // the failed attempt and the one that landed; nothing after
  });
  test('E9 reopening shows the kept text, not the head', async () => {
    const e = mk(); await e.edit(A, 'Hello world. unsent'); failNext = () => new TypeError('down'); await e.flush(A);
    const i = await mk().info(A);
    expect(i.kept).toBe('Hello world. unsent'); expect(i.editable).toBe(true);
  });
  test('E14 waiting counts the documents with kept text', async () => {
    const e = mk(); await e.edit(A, 'x1'); await e.edit('texture:d2', 'x2');
    expect((await e.info(A)).waiting).toBe(2);
  });
});

describe('a head that moved', () => {
  test('E8 409: nothing is posted over it, the text stays, no retry', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    revs.set('r1', { id: 'r1', content: 'Hello world. theirs', body: body('theirs'), n: 1 }); head.d1 = 'r1';   // another writer
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(i.label).toBe(CONFLICT_WORDS); expect(i.kept).toBe('Hello world. mine');
    expect(posts().length).toBe(0);                                  // the stale base is seen before any post (the kept text descends from r0)
    expect(kept().d1.text).toBe('Hello world. mine');
    await e.flush(A); expect(posts().length).toBe(0);                // no retry
  });
  test('E8b a 409 from the server after our head read is also a conflict', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    failNext = () => { revs.set('r1', { id: 'r1', content: 'other', body: body('other'), n: 1 }); head.d1 = 'r1'; return new HTTPError(409, 'HTTP 409: document head changed'); };
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(posts().length).toBe(1); expect(kept().d1.text).toBe('Hello world. mine');
  });
  test('E8c the head already holds our text (a reply that was lost): saved, nothing kept', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    failNext = () => { revs.set('r1', { id: 'r1', content: 'Hello world. mine', body: body('m'), n: 1 }); head.d1 = 'r1'; return new HTTPError(409, 'HTTP 409'); };
    const i = await e.flush(A);
    expect(i.status).toBe('saved'); expect(kept()).toEqual({});
  });
  test('E13 discard drops it; overwrite re-bases on the head and posts, on the listener\'s act', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    revs.set('r1', { id: 'r1', content: 'theirs', body: body('t'), n: 1 }); head.d1 = 'r1';
    await e.flush(A);
    const d = await e.discard(A); expect(d.status).toBe('idle'); expect(d.kept).toBe(''); expect(kept()).toEqual({});
    await e.edit(A, 'again'); await e.flush(A);                       // a fresh session reads the new head
    expect(posts().length).toBe(1); expect(posts()[0].body!.parent_revision_id).toBe('r1');
    // overwrite
    await e.edit(A, 'third');
    revs.set('r2', { id: 'r2', content: 'moved again', body: body('m'), n: 5 }); head.d1 = 'r2';
    const c = await e.flush(A); expect(c.status).toBe('conflict');
    const o = await e.overwrite(A); expect(o.status).toBe('saved');
    expect(posts()[posts().length - 1].body!.parent_revision_id).toBe('r2'); expect(kept()).toEqual({});
  });
});

describe('refusals and the account', () => {
  test('E10 a refused save fails with a sentence and keeps the text', async () => {
    const e = mk(); await e.edit(A, 'Hello world. z'); failNext = () => new HTTPError(403, '403');
    const i = await e.flush(A);
    expect(i.status).toBe('failed'); expect(i.label).toBe('Not saved: This device is not allowed to save here.'); expect(kept().d1.text).toBe('Hello world. z');
  });
  test('E11 signed out says so and keeps the text', async () => {
    const e = mk(); await e.edit(A, 'Hello world. z'); failNext = () => new SignedOutError('refused', 'refused');
    const i = await e.flush(A);
    expect(i.status).toBe('failed'); expect(i.label).toBe('Not saved: Sign in again to save.'); expect(kept().d1.text).toBe('Hello world. z');
  });
  test('E12 sign-out removes everything, and a save in flight afterwards keeps nothing', async () => {
    const e = mk(); await e.edit(A, 'Hello world. z');
    const slow = net.json; let release: () => void = () => undefined; const gate = new Promise<void>((r) => { release = r; });
    (net as { json: Net['json'] }).json = async (p, o) => { if (o?.method === 'POST') await gate; return slow(p, o); };
    const p = e.flush(A); await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); await e.wipe();
    release(); await p;
    (net as { json: Net['json'] }).json = slow;
    expect(kept()).toEqual({}); expect((await e.info(A)).kept).toBe(''); expect((await e.info(A)).waiting).toBe(0);
  });
});

describe('opening on the head, and the first keystroke', () => {
  test('E15 open reads the head now; the text typed over it is posted over that head, with the other writer\'s words in it', async () => {
    revs.set('r1', { id: 'r1', content: 'Hello world. Their paragraph.', body: body('Hello world. Their paragraph.'), n: 1 }); head.d1 = 'r1';   // moved after the reader drew r0
    const e = mk();
    const o = await e.open(A);
    expect(o.opened).toBe(true); expect(o.head).toBe('Hello world. Their paragraph.'); expect(o.kept).toBe('');
    await e.edit(A, o.head + ' Mine.', o.head); await e.flush(A);
    expect(posts().length).toBe(1); expect(posts()[0].body!.parent_revision_id).toBe('r1'); expect(posts()[0].body!.content).toBe('Hello world. Their paragraph. Mine.');
  });
  test('E15 a field seeded from an older reading (no open) over a head that moved is a conflict: nothing posted, the text kept', async () => {
    revs.set('r1', { id: 'r1', content: 'Hello world. Their paragraph.', body: body('t'), n: 1 }); head.d1 = 'r1';
    const e = mk();
    await e.edit(A, 'Hello world. typed', 'Hello world.');           // the field showed r0's text
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(posts().length).toBe(0); expect(i.kept).toBe('Hello world. typed'); expect(kept().d1.text).toBe('Hello world. typed');
  });
  test('E15 open with a kept text answers it, and the head', async () => {
    const e = mk(); await e.edit(A, 'Hello world. unsent'); failNext = () => new TypeError('down'); await e.flush(A);
    const o = await mk().open(A);
    expect(o.kept).toBe('Hello world. unsent'); expect(o.head).toBe('Hello world.');
  });
  test('E15 open offline says it did not open, and the editor still takes text', async () => {
    failGets = true;
    const e = mk(); const o = await e.open(A);
    expect(o.opened).toBe(false); expect(o.editable).toBe(true);
  });
  test('E16 a first edit made offline is kept with its seed; online later it is posted over the head when that still is the seed', async () => {
    failGets = true;
    const e = mk(); const i = await e.edit(A, 'Hello world. offline', 'Hello world.');
    expect(i.kept).toBe('Hello world. offline'); expect(kept().d1).toMatchObject({ text: 'Hello world. offline', base: '', seed: 'Hello world.' });
    failGets = false;
    const e2 = mk(); await e2.flush('');
    expect(posts().length).toBe(1); expect(posts()[0].body!.parent_revision_id).toBe('r0'); expect(kept()).toEqual({});
  });
  test('E16 ... and is a conflict, with nothing posted, when the head is something else', async () => {
    failGets = true;
    await mk().edit(A, 'Hello world. offline', 'Hello world.');
    revs.set('r1', { id: 'r1', content: 'Something new.', body: body('n'), n: 1 }); head.d1 = 'r1'; failGets = false;
    const e2 = mk(); const i = await e2.flush(A);
    expect(i.status).toBe('conflict'); expect(posts().length).toBe(0); expect(kept().d1.text).toBe('Hello world. offline');
  });
  test('E17 edits fired together keep the last one and read the head once', async () => {
    const e = mk();
    await Promise.all([e.edit(A, 'a'), e.edit(A, 'ab'), e.edit(A, 'abc')]);
    expect(kept().d1.text).toBe('abc');
    expect(log.filter((l) => l.method === 'GET' && l.path === '/api/texture/documents/d1').length).toBe(1);
  });
});

describe('the account moves under the editor', () => {
  test('E18 a sign-out during the head read leaves nothing on the disk', async () => {
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const e = mk(); const p = e.edit(A, 'Hello world. a');
    await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); await e.wipe();
    release(); await p;
    expect(kept()).toEqual({}); expect((await e.info(A)).kept).toBe('');
  });
  test('E18 a sign-out during the read of the kept file leaves nothing in memory for the next account', async () => {
    await mk().edit(A, 'Hello world. a'); failNext = null;
    let release: () => void = () => undefined; readGate = new Promise<void>((r) => { release = r; });
    const e2 = mk(); const p = e2.info(A);
    await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); await e2.wipe();
    release(); await p;
    readGate = null;
    expect((await e2.info(A)).kept).toBe(''); expect((await e2.info(A)).waiting).toBe(0);
  });
  test('E18 an edit that passed its checks but persists after the sign-out writes nothing', async () => {
    const e = mk(); await e.edit(A, 'Hello world. first');
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const e3 = mk(); const p = e3.edit('texture:d2', 'x');
    await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); await e3.wipe();
    release(); await p;
    expect(kept().d2).toBeUndefined();
  });
  test('E19 waiting does not count a document in conflict', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    revs.set('r1', { id: 'r1', content: 'theirs', body: body('t'), n: 1 }); head.d1 = 'r1';
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(i.waiting).toBe(0); expect((await e.info(A)).kept).toBe('Hello world. mine');
  });
  test('E20 an overwrite across a sign-out writes and posts nothing', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    revs.set('r1', { id: 'r1', content: 'theirs', body: body('t'), n: 1 }); head.d1 = 'r1';
    await e.flush(A);
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const p = e.overwrite(A);
    await new Promise((r) => setTimeout(r, 5));
    nextGeneration(); await e.wipe();
    release(); await p;
    expect(kept()).toEqual({}); expect(posts().length).toBe(0);
  });
  test('E20 while a sign-out is under way the editor shows nothing kept, and shows it again if the sign-out is given up', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    await beginSignOut();
    expect((await e.info(A)).kept).toBe(''); expect((await e.info(A)).waiting).toBe(0);
    abortSignOut();
    expect((await e.info(A)).kept).toBe('Hello world. mine');
  });
});

describe('one queue per document', () => {
  test('E21 a Done typed behind a first edit that is still reading the head is posted after it, not lost', async () => {
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const e = mk(); const typed = e.edit(A, 'Hello world. typed', 'Hello world.'); const done = e.flush(A);
    await new Promise((r) => setTimeout(r, 5)); release();
    await typed; const i = await done;
    expect(posts().length).toBe(1); expect(posts()[0].body!.content).toBe('Hello world. typed'); expect(i.status).toBe('saved'); expect(kept()).toEqual({});
  });
  test('E22 a discard queued behind a save that gets a 409 ends idle: the late conflict is not recorded over it', async () => {
    const e = mk(); await e.edit(A, 'Hello world. mine');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    failNext = () => { revs.set('r1', { id: 'r1', content: 'theirs', body: body('t'), n: 1 }); head.d1 = 'r1'; return new HTTPError(409, 'HTTP 409'); };
    const saving = e.flush(A); const gone = e.discard(A);
    await new Promise((r) => setTimeout(r, 5)); release();
    await saving; const i = await gone;
    expect(i.status).toBe('idle'); expect(i.kept).toBe(''); expect(kept()).toEqual({});
  });
  test('E24 the contract\'s way: each keystroke names what the field showed before it; a second save after a first is not a conflict', async () => {
    const e = mk();
    await e.edit(A, 'Hello world. a', 'Hello world.'); await e.flush(A);
    const i = await e.edit(A, 'Hello world. ab', 'Hello world. a'); expect(i.status).toBe('dirty');
    const j = await e.flush(A);
    expect(j.status).toBe('saved'); expect(posts().map((p) => [p.body!.content, p.body!.parent_revision_id])).toEqual([['Hello world. a', 'r0'], ['Hello world. ab', 'rev-new-1']]);
  });
});

describe('an empty document is a text, not "no seed"', () => {
  test('E23 empty, the head moved before the first keystroke: a conflict, nothing posted', async () => {
    revs.set('r0', { id: 'r0', content: '', body: body(''), n: 0 });
    revs.set('r1', { id: 'r1', content: 'Their paragraph', body: body('t'), n: 1 }); head.d1 = 'r1';
    const e = mk(); await e.edit(A, 'My paragraph', '');                  // the field showed the empty text it was drawn from
    const i = await e.flush(A);
    expect(i.status).toBe('conflict'); expect(posts().length).toBe(0); expect(kept().d1.text).toBe('My paragraph');
  });
  test('E23 empty, offline: posted when the head still is empty (not a false conflict)', async () => {
    revs.set('r0', { id: 'r0', content: '', body: body(''), n: 0 });
    failGets = true; await mk().edit(A, 'My note', ''); failGets = false;
    expect(kept().d1).toMatchObject({ base: '', seed: '' });
    const i = await mk().flush(A);
    expect(i.status).toBe('saved'); expect(posts().length).toBe(1); expect(posts()[0].body!.parent_revision_id).toBe('r0');
  });
});

describe('the account moves under a queued call', () => {
  test('E25 two edits queued before a sign-out, and one begun during it: nothing is written, before or after the wipe', async () => {
    const e = mk(); await e.open(A);
    let release: () => void = () => undefined; getGate = new Promise<void>((r) => { release = r; });
    const p1 = e.edit(A, 'x1'); const p2 = e.edit(A, 'x12');
    await new Promise((r) => setTimeout(r, 5));
    await beginSignOut();
    const p3 = e.edit(A, 'x123');                                         // begun while the sign-out is under way
    await e.wipe();
    release(); await Promise.all([p1, p2, p3]);
    const p4 = await e.edit(A, 'x1234');                                  // after the wipe, before a new account: still the old generation's blocked state
    expect(kept()).toEqual({}); expect(files.has(PENDING_FILE)).toBe(false); void p4;
  });
  test('E25 a kept file written under one key is not the next key\'s kept text', async () => {
    await mk().edit(A, 'Hello world. a'); expect(kept().d1.text).toBe('Hello world. a');
    signedKey = 'key-b-not-real'; signedComputer = 'c2';                       // another account: another computer
    const i = await mk().info(A);
    expect(i.kept).toBe(''); expect(i.waiting).toBe(0); expect(files.has(PENDING_FILE)).toBe(false);
  });
});

describe('what a person typed is never lost to a key that cannot be read, or to a slow network', () => {
  test('E26 an unreadable or absent key keeps the file, loads nothing and writes nothing over it; it is loaded when the key can be read', async () => {
    await mk().edit(A, 'Hello world. unsent');
    const before = new TextDecoder().decode(files.get(PENDING_FILE)!);
    for (const mode of ['unreadable', 'absent'] as const) {
      identityMode = mode;
      const e2 = mk(); const i = await e2.info(A);
      expect(i.kept).toBe(''); expect(files.has(PENDING_FILE)).toBe(true);
      await e2.edit(A, 'typed while the key cannot be read');
      expect(new TextDecoder().decode(files.get(PENDING_FILE)!)).toBe(before);               // not written over
    }
    identityMode = 'normal';
    const e3 = mk(); expect((await e3.info(A)).kept).toBe('Hello world. unsent');
  });
  test('E26 what is typed while the key cannot be read reaches the disk when it can, on top of what was kept', async () => {
    await mk().edit(A, 'Hello world. old'); identityMode = 'unreadable';
    const e = mk(); await e.edit('texture:d2', 'second');
    identityMode = 'normal';
    await e.info(A);                                                     // the key reads again: the file is read, then written with both
    await new Promise((r) => setTimeout(r, 20));
    expect(Object.keys(kept()).sort()).toEqual(['d1', 'd2']);
  });
  test('E26 a key rotated for the same account (same computer) keeps the unsent text', async () => {
    await mk().edit(A, 'Hello world. a');
    signedKey = 'key-a-rotated-not-real';
    expect((await mk().info(A)).kept).toBe('Hello world. a');
  });
  test('E27 the latest keystroke is on disk while a save is still in flight, and a relaunch finds it', async () => {
    const e = mk(); await e.edit(A, 'one');
    let release: () => void = () => undefined; postGate = new Promise<void>((r) => { release = r; });
    const saving = e.flush(A); await new Promise((r) => setTimeout(r, 10));
    const typed = e.edit(A, 'one two three'); await new Promise((r) => setTimeout(r, 20));
    expect(kept().d1.text).toBe('one two three');                                           // before the post is over: this is the state an app kill would leave
    expect((await mk().info(A)).kept).toBe('one two three');
    release(); await saving; await typed;
  });
  test('E27 a link that answers nothing ends in the editor\'s own timeout; the text is on disk and the status says offline', async () => {
    getGate = new Promise<void>(() => undefined);
    const e = new Editor({ net, storage, timeoutMs: 30 });
    const i = await e.edit(A, 'typed on a dead link', 'Hello world.');
    expect(kept().d1.text).toBe('typed on a dead link'); expect(i.status).toBe('offline');
  });
  test('E28 two documents written at once leave both in the file, however the writes land', async () => {
    slowFirstWrite = true;
    const e = mk();
    await Promise.all([e.edit(A, 'first'), e.edit('texture:d2', 'second')]);
    expect(Object.keys(kept()).sort()).toEqual(['d1', 'd2']);
  });
  test('E29 a reply let go is not "offline": the text stays, the status stays editing', async () => {
    const e = mk(); await e.edit(A, 'Hello world. x');
    failNext = () => new Error('the answer was let go before this fetch');
    const i = await e.flush(A);
    expect(i.status).toBe('dirty'); expect(kept().d1.text).toBe('Hello world. x');
  });
});
