// The edition picks (the listener's chosen topics) as shared code: edition-picks.ts is the ONE reader and writer of `app:/data/edition-topics.json`.
// Ported from EditionSetupTests.swift (E6, E7, E9 and the session's finish and wipe), and the account rules the takes and Texture-editor reviews sent back
// three times each, written as a failure list BEFORE the code (every test reads the stored FILE or the page as the listener would, not a fake's call log):
//  P1 Reading: no file, an empty one, a corrupt one or one of the wrong shape is "no picks" and never throws (E7); a stored file is read again by a new process (new memory).
//  P2 Writing: every change is stored after it is made; a failed write loses nothing in memory (E6); the picks arrive for the paper one per line, in the order chosen.
//  P3 Concurrent writers: taps, typed words and speech that overlap all apply their change at write time, one after another; the FINAL FILE holds every change (no lost update).
//  P4 Accounts: (a) memory belongs to the account generation: after a sign-out the next account reads nothing; (b) a write in flight when the account changes lands nowhere (the file
//     goes again) and the next account reads nothing; (c) a change queued for account A that runs after the account moved is skipped, not applied to B; (d) a wipe whose rm fails
//     empties the file instead, and says so when it cannot do either.
//  P5 The native copy (the DJ, the saved corpus, the edition writer read it): pushed only after the file was written; emptied at a wipe; not pushed for an account that moved on;
//     a native call that never answers does not stop the next push.
//  P6 The old Swift file (Application Support/Autoradio/edition-topics.json): it carries no proof of whose it is, so a topic of it is adopted ONLY when the signed-in account's own
//     notebook lists it (GET /api/texture/edition, "What you asked for"), and what is stored is THE NOTEBOOK'S line, never the old file's string. No marker, no flag. THE TABLE
//     (docs/reference/AUTORADIO-EDITION-PICKS.md), one test per cell; "open" = the first read of this account generation on this phone:
//       old file | picks file at open | server            | change since open | action
//       absent   | any                | not asked         | -                 | nothing (c0)
//       unreadable| any               | not asked         | -                 | keep (c10a)
//       corrupt or no topics | any     | not asked         | -                 | drop (c10b)
//       topics   | there              | not asked         | -                 | drop, adds nothing (c1)
//       topics   | absent             | cannot say        | none              | keep, say so (c2)
//       topics   | absent             | cannot say        | yes               | keep; next launch the picks are there: dropped, the cost (c3)
//       topics   | absent             | answered, no match or no notebook | - | drop (c4)
//       topics   | absent             | answered, match   | none              | adopt the notebook's lines, then drop (c5)
//       topics   | absent             | answered, match   | none, write fails | keep (c6)
//       topics   | absent             | answered, match   | yes               | merge into what the listener has, then drop (c7)
//       topics   | absent             | account moved     | -                 | nothing written, keep (c8)
//       a drop that fails repeats safely (c9); no native module reads nothing (c11); a collision stores the notebook's text, not the old file's (c12);
//       the first page waits for the proof (bounded) and says so when it did not finish (c13).
//  P7 Speech: "add space and chess" is stored and answered; "done" finishes; heard words are taken from the native mailbox in order and each is answered once, even when refused.
//  P8 The page: rows follow the selection, expanding changes no pick, own topics stay as chips after a deselect (E9), the typed duplicate says so, live sections merge.
import { beforeEach, describe, expect, test } from 'bun:test';
import { accountGeneration, nextGeneration, resetSeam, bindNative } from './api.ts';
import { WIPES, wipedPaths, wipeAll, leftovers } from './account.ts';
import {
  PICKS_FILE, picksPage, picksText, toggle, expand, addTyped, clearPicks, hear, drainHeard, wipePicks, forgetPicks, legacyDone, parseStored, type Device, type Env,
} from './edition-picks.ts';
import type { Get } from './notebook.ts';
import { HTTPError } from './api.ts';

// ---- a real in-memory file system, with a gate to hold a write in flight
const files = new Map<string, Uint8Array>();
let gate: Promise<void> | null = null;
let failWrites = false;
let failRm = false;
const storage = {
  fs: {
    atomicWriteFile: async (p: string, d: Uint8Array) => { if (gate) await gate; if (failWrites) throw new Error('disk full'); files.set(p, d); },
    writeFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    rm: async (p: string) => { if (failRm) throw new Error('busy'); files.delete(p); },
    stat: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return { size: b.length }; },
    mkdir: async () => undefined,
  },
} as never;
const hold = (): (() => void) => { let r: () => void = () => undefined; gate = new Promise<void>((res) => { r = res; }); return r; };
const text = (p: string): string | undefined => { const b = files.get(p); return b ? new TextDecoder().decode(b) : undefined; };
const stored = (): string[] | undefined => { const t = text(PICKS_FILE); return t === undefined ? undefined : (JSON.parse(t) as { topics: string[] }).topics; };
const put = (p: string, t: string): void => { files.set(p, new TextEncoder().encode(t)); };

// ---- a fake phone: the old file and the native mirror and mailbox are state the test can read back
type Phone = { legacy: string | null; unreadable: boolean; dropFails: boolean; mirrored: string[][]; heard: { id: string; words: string }[]; said: { id: string; line: string; finish: boolean }[]; answers: boolean; mirrorHangs: boolean };
let phone: Phone;
const fresh = (): Phone => ({ legacy: null, unreadable: false, dropFails: false, mirrored: [], heard: [], said: [], answers: true, mirrorHangs: false });
const device = (): Device => ({
  legacy: async () => (phone.answers ? { found: phone.legacy !== null || phone.unreadable, text: phone.legacy ?? '', unreadable: phone.unreadable } : null),
  dropLegacy: async () => { if (!phone.dropFails) { phone.legacy = null; phone.unreadable = false; } return phone.legacy === null && !phone.unreadable; },
  mirror: async (t) => { if (phone.mirrorHangs) return new Promise<void>(() => undefined); phone.mirrored.push(t); },
  forget: async () => { phone.mirrored.push([]); },
  heard: async () => phone.heard.splice(0),
  said: async (id, line, finish) => { phone.said.push({ id, line, finish }); },
});
// ---- the server's notebook for the signed-in account
let server: { edition?: unknown; notebook?: string; fail?: boolean };
const get: Get = async (p) => {
  if (server.fail) throw new Error('network');
  if (p === '/api/texture/edition') return server.edition;
  if (p === '/api/texture/documents/nb1') return { document: { current_revision_id: 'r1' } };
  if (p === '/api/texture/revisions/r1') return { revision: { content: server.notebook ?? '' } };
  throw new HTTPError(404, 'HTTP 404');
};
const notebook = (...asked: string[]): string => `# My notebook\n\n## What you asked for\n${asked.map((t) => `- ${t}\n`).join('')}\n## What I learned\n- Short stories are heard through [L1 · heard 3 of 4]\n`;
const accountWith = (...asked: string[]): void => { server = { edition: { found: true, doc_id: 'nb1' }, notebook: notebook(...asked) }; };
let live: string[];
const env = (device_: Device | null = device()): Env => ({ device: device_, get, live: async () => live });
const E = (): Env => env();

beforeEach(() => { files.clear(); gate = null; failWrites = false; failRm = false; resetSeam(); bindNative(undefined); forgetPicks(); phone = fresh(); server = { edition: { found: false } }; live = []; });
const settle = async (): Promise<void> => { await legacyDone(); await new Promise((r) => setTimeout(r, 5)); };

describe('P1 reading', () => {
  test('no file, an empty one, a corrupt one or the wrong shape: no picks, never a throw (E7)', async () => {
    expect((await picksPage(storage, {}, E())).pickedCount).toBe(0);
    for (const t of ['', '{not json', '[]', '{"topics":"Space"}', '{"topics":[1,null,{}]}', 'null']) {
      forgetPicks(); put(PICKS_FILE, t);
      expect((await picksText(storage, E())).picks).toBe('');
    }
    expect(parseStored('{"topics":["Space","  ","Chess",3]}')).toEqual(['Space', 'Chess']);
  });
  test('E7: coming back (a new process) shows what was saved, the own topics as chips', async () => {
    await toggle(storage, 'Space', E()); await addTyped(storage, 'Chess', E()); await hear(storage, 'add film', E());
    forgetPicks();                                            // a restart: memory is gone, the file is not
    const page = await picksPage(storage, {}, E());
    expect(page.picks).toBe('Space\nChess\nFilm');
    expect(page.tree.find((x) => x.kind === 'yours')?.label).toBe('Chess');
    expect(page.tree.find((x) => x.label === 'Science')?.partial).toBe(true);
    expect(page.tree.find((x) => x.label === 'World')?.selected).toBe(false);
  });
});

describe('P2 writing', () => {
  test('every change is stored after it is made, in the order chosen, one per line for the paper', async () => {
    await toggle(storage, 'Space', E()); expect(stored()).toEqual(['Space']);
    await addTyped(storage, 'Chess, Film', E()); expect(stored()).toEqual(['Space', 'Chess', 'Film']);
    await toggle(storage, 'space', E()); expect(stored()).toEqual(['Chess', 'Film']);
    expect((await picksText(storage, E())).picks).toBe('Chess\nFilm');
    await clearPicks(storage, E()); expect(stored()).toEqual([]); expect((await picksText(storage, E())).picks).toBe('');
  });
  test('E6: a failed write loses nothing in memory, and the next good write stores all of it', async () => {
    failWrites = true;
    const page = await toggle(storage, 'Space', E());
    expect(page.picks).toBe('Space'); expect(stored()).toBeUndefined();
    failWrites = false;
    await toggle(storage, 'Chess', E());
    expect(stored()).toEqual(['Space', 'Chess']);
  });
});

describe('P3 concurrent writers', () => {
  test('overlapping taps, typed words and speech all land in the FINAL FILE, even with the writes held and released late', async () => {
    const release = hold();
    const calls = [
      toggle(storage, 'Space', E()), addTyped(storage, 'Chess', E()), hear(storage, 'add film and music', E()), toggle(storage, 'Science', E()),
      toggle(storage, 'Space', E()),                         // the same topic again: off
      addTyped(storage, 'Sourdough', E()), hear(storage, 'drop film', E()),
    ];
    await new Promise((r) => setTimeout(r, 20)); release();
    await Promise.all(calls);
    expect(stored()).toEqual(['Chess', 'Music', 'Science', 'Sourdough']);
    expect((await picksText(storage, E())).picks).toBe('Chess\nMusic\nScience\nSourdough');
  });
  test('changes apply in the order they were asked, whatever else is in flight', async () => {
    await Promise.all([toggle(storage, 'Space', E()), toggle(storage, 'Chess', E()), toggle(storage, 'Film', E())]);
    expect(stored()).toEqual(['Space', 'Chess', 'Film']);
  });
});

describe('P4 accounts', () => {
  test('a: after a sign-out the next account reads nothing, from memory or from the file', async () => {
    await toggle(storage, 'Space', E()); expect((await picksText(storage, E())).picks).toBe('Space');
    nextGeneration(); await wipePicks(storage, E());           // account.ts: the generation moves, then the wipes run
    expect(files.has(PICKS_FILE)).toBe(false);
    expect((await picksText(storage, E())).picks).toBe('');
    expect((await picksPage(storage, {}, E())).pickedCount).toBe(0);
  });
  test('a, no wipe yet: a moved generation alone empties what B reads from memory (the file is read fresh, and it is the wipe\'s to remove)', async () => {
    await toggle(storage, 'Space', E());
    nextGeneration(); files.delete(PICKS_FILE);
    expect((await picksText(storage, E())).picks).toBe('');
  });
  test('b: a write held in flight when the account changes lands nowhere; the next account reads nothing', async () => {
    const release = hold();
    const p = toggle(storage, 'Space', E());
    await new Promise((r) => setTimeout(r, 20));
    nextGeneration(); await wipePicks(storage, E()).catch(() => undefined);
    release(); await p; await new Promise((r) => setTimeout(r, 20));
    expect(files.has(PICKS_FILE)).toBe(false);
    expect((await picksText(storage, E())).picks).toBe('');
  });
  test('c: a change queued for A that would run after the account moved is skipped, not applied to B', async () => {
    const release = hold();
    const first = toggle(storage, 'Space', E());               // A: held in the write
    const second = toggle(storage, 'Chess', E());              // A: queued behind it
    await new Promise((r) => setTimeout(r, 20));
    nextGeneration(); await wipePicks(storage, E()).catch(() => undefined);
    release(); await Promise.all([first, second]); await new Promise((r) => setTimeout(r, 20));
    expect(files.has(PICKS_FILE)).toBe(false);
    await toggle(storage, 'Film', E());                        // B's own first tap
    expect(stored()).toEqual(['Film']);
  });
  test('d: a wipe whose rm fails empties the file instead; when it can do neither it names itself failed', async () => {
    await toggle(storage, 'Space', E());
    failRm = true; nextGeneration();
    await wipePicks(storage, E());
    expect(stored()).toEqual([]); expect((await picksText(storage, E())).picks).toBe('');
    await toggle(storage, 'Chess', E());                       // (a new account's tap, after the generation moved once more)
    nextGeneration(); failWrites = true;
    await expect(wipePicks(storage, E())).rejects.toThrow();
  });
  test('d2: with rm failing for good, the emptied file counts as cleared in the wipe bookkeeping (no full wipe at every launch)', async () => {
    await toggle(storage, 'Space', E());
    failRm = true; nextGeneration();
    const failed = await wipeAll(storage, undefined, WIPES.filter((w) => w.name === 'edition-picks'));
    expect(failed).toEqual([]);
    expect(await leftovers(storage)).toEqual([]);
    expect((await picksText(storage, E())).picks).toBe('');
    put(PICKS_FILE, '{"topics":["Space"]}');                   // a file that still holds picks IS left over
    expect(await leftovers(storage)).toEqual(['edition-topics.json']);
  });
  test('the wipe list covers the file (account.ts)', () => {
    expect(wipedPaths()).toContain(PICKS_FILE);
    expect(WIPES.some((w) => w.name === 'edition-picks')).toBe(true);
  });
  test('the real wipe list removes the file', async () => {
    await toggle(storage, 'Space', E());
    const w = WIPES.find((x) => x.name === 'edition-picks')!;
    nextGeneration(); await w.run(storage, undefined);
    expect(files.has(PICKS_FILE)).toBe(false);
  });
});

describe('P5 the native copy', () => {
  test('pushed only after the file was written, in the order written; emptied at the wipe', async () => {
    await toggle(storage, 'Space', E()); await toggle(storage, 'Chess', E());
    expect(phone.mirrored.at(-1)).toEqual(['Space', 'Chess']);
    failWrites = true; const n = phone.mirrored.length;
    await toggle(storage, 'Film', E());
    expect(phone.mirrored.length).toBe(n);                     // not written, not pushed
    failWrites = false;
    nextGeneration(); await wipePicks(storage, E());
    expect(phone.mirrored.at(-1)).toEqual([]);
  });
  test('a first read after a restart gives the native side its copy', async () => {
    put(PICKS_FILE, '{"topics":["Space"]}');
    await picksText(storage, E());
    expect(phone.mirrored.at(-1)).toEqual(['Space']);
  });
  test('a native call that never answers does not stop the next push; a write refused because the account moved is not pushed', async () => {
    phone.mirrorHangs = true;
    const hung = toggle(storage, 'Space', { ...E(), mirrorDeadlineMs: 30 });
    await hung;
    phone.mirrorHangs = false;
    await toggle(storage, 'Chess', { ...E(), mirrorDeadlineMs: 30 });
    await new Promise((r) => setTimeout(r, 80));
    expect(phone.mirrored.at(-1)).toEqual(['Space', 'Chess']);
    const g = accountGeneration();
    const release = hold(); const p = toggle(storage, 'Film', E()); await new Promise((r) => setTimeout(r, 10));
    nextGeneration(); release(); await p; await new Promise((r) => setTimeout(r, 20));
    expect(accountGeneration()).toBeGreaterThan(g);
    expect(phone.mirrored.some((t) => t.includes('Film'))).toBe(false);
  });
});

describe('P6 the old Swift file: adopted by proof, the notebook\'s own lines', () => {
  const old = (...t: string[]): string => JSON.stringify({ topics: t });
  const launch = async (e: Env = E()): Promise<void> => { forgetPicks(); await picksText(storage, e); await settle(); };      // a launch: new memory, the first read, the proof
  test('c0: no old file: nothing to prove, no server call', async () => {
    let asked = 0; const counting: Get = async (p, q) => { asked += 1; return get(p, q); };
    await picksText(storage, { ...E(), get: counting }); await settle();
    expect(asked).toBe(0);
  });
  test('c1: picks already there at the first read: the old file adds nothing (a topic taken off never comes back), is dropped, and no server call is made', async () => {
    put(PICKS_FILE, '{"topics":["Film"]}'); phone.legacy = old('Space', 'Film'); accountWith('Space', 'Film');
    let asked = 0; const counting: Get = async (p, q) => { asked += 1; return get(p, q); };
    await picksText(storage, { ...E(), get: counting }); await settle();
    expect(stored()).toEqual(['Film']); expect(phone.legacy).toBeNull(); expect(asked).toBe(0);
    put(PICKS_FILE, '{"topics":[]}'); forgetPicks(); phone.legacy = old('Space'); accountWith('Space');       // took everything off: the file is there, empty
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual([]); expect(phone.legacy).toBeNull();
  });
  test('c2: the server cannot say: nothing is decided, the file stays, the page says so, and the next launch decides', async () => {
    phone.legacy = old('Space'); server = { fail: true };
    const page = await picksPage(storage, {}, E()); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
    expect(page.carryNote).toContain('could not be checked');
    server = { edition: { found: true, doc_id: 'nb1' }, notebook: notebook('Space') };
    await launch();
    expect(stored()).toEqual(['Space']); expect(phone.legacy).toBeNull();
    // a notebook answered but unreadable (no revision) is "cannot say" too
    files.clear(); phone.legacy = old('Space'); server = { edition: { found: true, doc_id: 'nb1' }, notebook: '' };
    await launch();
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
  });
  test('c3: offline first launch, one tap, next launch online: the picks are there, so the old file adds nothing and goes. THIS IS THE COST, stated in the reference page', async () => {
    phone.legacy = old('Space', 'Film'); server = { fail: true };
    await toggle(storage, 'Culture', E()); await settle();
    expect(stored()).toEqual(['Culture']); expect(phone.legacy).not.toBeNull();       // the same launch decides nothing
    accountWith('Space', 'Film');
    await launch();
    expect(stored()).toEqual(['Culture']); expect(phone.legacy).toBeNull();
  });
  test('c4: an account with no notebook, or a notebook with none of the topics, adopts nothing and the file goes', async () => {
    phone.legacy = old('Space'); server = { edition: { found: false } };
    await launch(); expect(stored()).toBeUndefined(); expect(phone.legacy).toBeNull(); expect((await picksText(storage, E())).picks).toBe('');
    phone.legacy = old('Space', 'Chess'); accountWith('Gardening');
    await launch(); expect(stored()).toBeUndefined(); expect(phone.legacy).toBeNull();
  });
  test('c5: a topic the account\'s own notebook lists is adopted AS THE NOTEBOOK WRITES IT, in the old file\'s order; the first page already shows it', async () => {
    phone.legacy = old('Space', 'Chess', 'Film'); accountWith('film', 'space', 'Something else');
    const page = await picksPage(storage, {}, E());
    expect(page.picks).toBe('Space\nFilm'); expect(page.carryNote).toBe(''); expect(page.pickedCount).toBe(2);
    await settle();
    expect(stored()).toEqual(['Space', 'Film']); expect(phone.legacy).toBeNull(); expect(phone.mirrored.at(-1)).toEqual(['Space', 'Film']);
  });
  test('c6: adopted but not written: the old file stays', async () => {
    phone.legacy = old('Space'); accountWith('Space'); failWrites = true;
    await launch();
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
  });
  test('c7: the listener changed picks while the proof was in flight: the proven topics are merged into what they have, then the file goes', async () => {
    phone.legacy = old('Space'); accountWith('Space');
    let release: () => void = () => undefined; const hold2 = new Promise<void>((r) => { release = r; });
    const slowGet: Get = async (p, q) => { await hold2; return get(p, q); };
    await picksText(storage, { ...E(), get: slowGet, legacyWaitMs: 5 });
    await toggle(storage, 'Film', E());
    release(); await settle();
    expect(stored()).toEqual(['Film', 'Space']); expect(phone.legacy).toBeNull();
  });
  test('c8: the account changes while the proof is in flight: nothing is written for the new account and the file stays', async () => {
    phone.legacy = old('Space'); accountWith('Space');
    let release: () => void = () => undefined; const hold2 = new Promise<void>((r) => { release = r; });
    const slowGet: Get = async (p, q) => { await hold2; return get(p, q); };
    await picksText(storage, { ...E(), get: slowGet, legacyWaitMs: 5 });
    nextGeneration(); await wipePicks(storage, E());
    release(); await settle(); await new Promise((r) => setTimeout(r, 20));
    expect(files.has(PICKS_FILE)).toBe(false); expect(phone.legacy).not.toBeNull();
  });
  test('c9: a drop that fails repeats safely: the next launch adopts nothing, and what the listener changed stays changed', async () => {
    phone.legacy = old('Space', 'Film'); phone.dropFails = true; accountWith('Space', 'Film');
    await launch();
    expect(stored()).toEqual(['Space', 'Film']); expect(phone.legacy).not.toBeNull();
    await toggle(storage, 'Film', E()); expect(stored()).toEqual(['Space']);
    await launch(); expect(stored()).toEqual(['Space']);
    phone.dropFails = false; await launch(); expect(phone.legacy).toBeNull(); expect(stored()).toEqual(['Space']);
  });
  test('c10: an old file that cannot be read stays; one that is corrupt or holds no topics goes', async () => {
    accountWith('Space'); phone.unreadable = true;
    await launch(); expect(phone.unreadable).toBe(true); expect(stored()).toBeUndefined();
    phone.unreadable = false;
    for (const t of ['{nope', '{"topics":[]}', '']) { phone.legacy = t; await launch(); expect(phone.legacy).toBeNull(); expect(stored()).toBeUndefined(); }
  });
  test('c11: no native module: nothing is read, nothing is adopted', async () => {
    accountWith('Space'); phone.legacy = old('Space');
    await launch(env(null));
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
  });
  test('c12: a collision stores THE NOTEBOOK\'S text: B asked for "C", the old file (A\'s) says "C++"; a case-differing label is B\'s too', async () => {
    phone.legacy = old('C++', 'Space!!', 'Chess', 'SPACE-X'); accountWith('C', 'space', 'chess');
    await launch();
    expect(stored()).toEqual(['C', 'Space', 'Chess']);
    expect(phone.mirrored.at(-1)).toEqual(['C', 'Space', 'Chess']);
  });
  test('c13: the first page waits for the proof, bounded; past the bound it says it is still looking, and the next ask has the picks', async () => {
    phone.legacy = old('Space'); accountWith('Space');
    let release: () => void = () => undefined; const hold2 = new Promise<void>((r) => { release = r; });
    const slowGet: Get = async (p, q) => { await hold2; return get(p, q); };
    const first = await picksPage(storage, {}, { ...E(), get: slowGet, legacyWaitMs: 20 });
    expect(first.pickedCount).toBe(0); expect(first.carrying).toBe(true); expect(first.carryNote).toContain('Looking');
    release(); await settle();
    const next = await picksPage(storage, {}, E());
    expect(next.picks).toBe('Space'); expect(next.carrying).toBe(false); expect(next.carryNote).toBe('');
    expect((await picksText(storage, E())).picks).toBe('Space');
  });
});

describe('P7 speech', () => {
  test('a spoken edit is stored and answered; "done" finishes; a question changes nothing', async () => {
    const r = await hear(storage, 'add space and chess', E());
    expect(r).toEqual({ line: 'Added Space and Chess.', finish: false }); expect(stored()).toEqual(['Space', 'Chess']);
    expect((await hear(storage, 'drop chess', E()))?.line).toBe('Took off Chess.'); expect(stored()).toEqual(['Space']);
    expect((await hear(storage, 'what is this for?', E()))?.line).toContain('Tell me a topic'); expect(stored()).toEqual(['Space']);
    expect(await hear(storage, 'continue', E())).toEqual({ line: 'Okay.', finish: true });
  });
  test('heard words are taken from the mailbox in order and each is answered once', async () => {
    phone.heard = [{ id: 'a', words: 'add space' }, { id: 'b', words: 'add chess' }, { id: 'c', words: 'not space' }, { id: 'd', words: 'done' }];
    await drainHeard(storage, E());
    expect(phone.said).toEqual([{ id: 'a', line: 'Added Space.', finish: false }, { id: 'b', line: 'Added Chess.', finish: false }, { id: 'c', line: 'Took off Space.', finish: false }, { id: 'd', line: 'Okay.', finish: true }]);
    expect(stored()).toEqual(['Chess']);
    await drainHeard(storage, E()); expect(phone.said.length).toBe(4);
  });
  test('words heard for an account that has moved on are answered empty and change nothing', async () => {
    const release = hold();
    phone.heard = [{ id: 'a', words: 'add space' }];
    const p = drainHeard(storage, E());
    await new Promise((r) => setTimeout(r, 20));
    nextGeneration(); await wipePicks(storage, E()).catch(() => undefined);
    release(); await p;
    expect(phone.said).toEqual([{ id: 'a', line: '', finish: false }]);
    expect(files.has(PICKS_FILE)).toBe(false);
  });
  test('a mailbox the native side cannot answer is no edit', async () => {
    const dead: Device = { ...device(), heard: async () => { throw new Error('no'); } };
    await drainHeard(storage, env(dead)); expect(stored()).toBeUndefined();
  });
});

describe('P8 the page', () => {
  test('rows follow the selection; expanding is not a pick; the explanation and the count are the page\'s', async () => {
    const closed = await picksPage(storage, {}, E());
    expect(closed.tree.length).toBe(10); expect(closed.selectionLine).toContain('general edition'); expect(closed.explainer).toContain('any time'); expect(closed.voiceHint).toContain('space and chess');
    const open = await expand(storage, 'Culture', E());
    expect(open.tree.length).toBe(17); expect(open.pickedCount).toBe(0); expect(files.has(PICKS_FILE)).toBe(false);
    const picked = await toggle(storage, 'Film', E());
    expect(picked.tree.length).toBe(17); expect(picked.tree.find((x) => x.label === 'Culture')?.partial).toBe(true); expect(picked.selectionLine).toContain('Film');
    expect((await expand(storage, 'culture', E())).tree.length).toBe(10);
  });
  test('E9: a deselected own topic stays as a chip until the next visit', async () => {
    await addTyped(storage, 'Chess', E()); const page = await toggle(storage, 'Chess', E());
    expect(page.pickedCount).toBe(0);
    const chip = page.tree.find((x) => x.label === 'Chess'); expect(chip?.kind).toBe('yours'); expect(chip?.selected).toBe(false);
    forgetPicks(); expect((await picksPage(storage, {}, E())).tree.find((x) => x.label === 'Chess')).toBeUndefined();
  });
  test('a typed duplicate says so; typing nothing says nothing', async () => {
    await addTyped(storage, 'Chess', E());
    expect((await addTyped(storage, 'chess', E())).message).toBe('That one is already there.');
    expect((await addTyped(storage, '  ', E())).message).toBe('');
    expect((await addTyped(storage, 'Space', E())).message).toBe('');
  });
  test('live sections of today\'s paper merge in as one more chip', async () => {
    live = ['housing_market', 'world'];
    const page = await picksPage(storage, {}, E());
    expect(page.tree.at(-1)?.label).toBe('Housing market'); expect(page.tree.length).toBe(11);
    expect((await toggle(storage, 'Housing market', E())).picks).toBe('Housing market');
  });
  test('the answer carries no key, and the page names a rev that moves with every change', async () => {
    const a = await picksPage(storage, {}, E()); const b = await toggle(storage, 'Space', E());
    expect(b.rev).toBeGreaterThan(a.rev); expect(JSON.stringify(b)).not.toMatch(/choir_sk_/);
  });
});
