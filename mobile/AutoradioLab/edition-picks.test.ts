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
//  P6 The old Swift file (Application Support/Autoradio/edition-topics.json): it carries no proof of whose it is, so it is adopted ONLY topic by topic, for a topic the signed-in
//     account's own notebook already lists (GET /api/texture/edition, "What you asked for"), and only when no picks are stored yet. Then it is dropped. No marker, no flag.
//     (a) a match is adopted and the old file dropped; (b) no notebook adopts nothing and drops; (c) the server unreachable decides nothing and keeps the file, the next try decides;
//     (d) another account's file: nothing of it reaches this account; (e) picks already stored: the old file is not adopted (a deselected topic never comes back), and is dropped;
//     (f) the account changes while the proof is in flight: nothing is written and the file stays; (g) a failed drop repeated adopts nothing new; (h) a corrupt old file is dropped;
//     (i) no native module (Android): nothing; (j) the typed picks of an upgrading listener who tapped within the first second are theirs: the old file adds nothing.
//  P7 Speech: "add space and chess" is stored and answered; "done" finishes; heard words are taken from the native mailbox in order and each is answered once, even when refused.
//  P8 The page: rows follow the selection, expanding changes no pick, own topics stay as chips after a deselect (E9), the typed duplicate says so, live sections merge.
import { beforeEach, describe, expect, test } from 'bun:test';
import { accountGeneration, nextGeneration, resetSeam, bindNative } from './api.ts';
import { WIPES, wipedPaths } from './account.ts';
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
type Phone = { legacy: string | null; dropFails: boolean; mirrored: string[][]; heard: { id: string; words: string }[]; said: { id: string; line: string; finish: boolean }[]; answers: boolean; mirrorHangs: boolean };
let phone: Phone;
const fresh = (): Phone => ({ legacy: null, dropFails: false, mirrored: [], heard: [], said: [], answers: true, mirrorHangs: false });
const device = (): Device => ({
  legacy: async () => (phone.answers ? { found: phone.legacy !== null, text: phone.legacy ?? '' } : null),
  dropLegacy: async () => { if (!phone.dropFails) phone.legacy = null; return phone.legacy === null; },
  mirror: async (t) => { if (phone.mirrorHangs) return new Promise<void>(() => undefined); phone.mirrored.push(t); },
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
  test('d: a wipe whose rm fails empties the file instead; when it can do neither it says so', async () => {
    await toggle(storage, 'Space', E());
    failRm = true; nextGeneration();
    await wipePicks(storage, E());
    expect(stored()).toEqual([]); expect((await picksText(storage, E())).picks).toBe('');
    await toggle(storage, 'Chess', E());                       // (a new account's tap, after the generation moved once more)
    nextGeneration(); failWrites = true;
    await expect(wipePicks(storage, E())).rejects.toThrow();
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
  test('not pushed for an account that moved on; a native call that never answers does not stop the next push', async () => {
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

describe('P6 the old Swift file: adopted by proof, topic by topic', () => {
  const old = (...t: string[]): string => JSON.stringify({ topics: t });
  test('a: a topic the account\'s own notebook lists is adopted; the others and the old file go', async () => {
    phone.legacy = old('Space', 'Chess', 'Film'); accountWith('Space', 'Film', 'Something else');
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual(['Space', 'Film']); expect(phone.legacy).toBeNull();
    expect(phone.mirrored.at(-1)).toEqual(['Space', 'Film']);
    expect((await picksText(storage, E())).picks).toBe('Space\nFilm');
  });
  test('b: an account with no notebook adopts nothing, and the old file goes (the cost: the listener picks again)', async () => {
    phone.legacy = old('Space'); server = { edition: { found: false } };
    await picksText(storage, E()); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).toBeNull();
    expect((await picksText(storage, E())).picks).toBe('');
  });
  test('c: the server cannot say: nothing is decided, the file stays, and the next try decides', async () => {
    phone.legacy = old('Space'); server = { fail: true };
    await picksText(storage, E()); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
    server = { edition: { found: true, doc_id: 'nb1' }, notebook: notebook('Space') };
    forgetPicks();                                            // the next launch
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual(['Space']); expect(phone.legacy).toBeNull();
    // a notebook answered but unreadable (no revision) is "cannot say" too
    forgetPicks(); files.clear(); phone.legacy = old('Space'); server = { edition: { found: true, doc_id: 'nb1' }, notebook: '' };
    await picksText(storage, E()); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
  });
  test('d: another account\'s file reaches nobody: A\'s topics, B\'s notebook', async () => {
    phone.legacy = old('Space', 'Chess'); accountWith('Gardening');            // B is signed in; the file was A's
    await picksText(storage, E()); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).toBeNull();
    // a topic B happens to have as well is B's own words, so it is adopted and nothing else
    forgetPicks(); phone.legacy = old('Space', 'Chess'); accountWith('chess');
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual(['Chess']);
  });
  test('e: picks already stored: the old file adds nothing (a topic the listener took off never comes back) and goes', async () => {
    put(PICKS_FILE, '{"topics":["Film"]}'); phone.legacy = old('Space', 'Film'); accountWith('Space', 'Film');
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual(['Film']); expect(phone.legacy).toBeNull();
    put(PICKS_FILE, '{"topics":[]}'); forgetPicks(); phone.legacy = old('Space'); accountWith('Space');       // took everything off: stored, empty
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual([]);
  });
  test('f: the account changes while the proof is in flight: nothing is written for the new account and the file stays', async () => {
    phone.legacy = old('Space'); accountWith('Space');
    let release: () => void = () => undefined; const hold2 = new Promise<void>((r) => { release = r; });
    const slowGet: Get = async (p, q) => { await hold2; return get(p, q); };
    await picksText(storage, { ...E(), get: slowGet });
    nextGeneration(); await wipePicks(storage, E());
    release(); await settle(); await new Promise((r) => setTimeout(r, 20));
    expect(files.has(PICKS_FILE)).toBe(false); expect(phone.legacy).not.toBeNull();
  });
  test('g: a drop that fails is safe to repeat: the second run adopts nothing new, and what the listener changed stays changed', async () => {
    phone.legacy = old('Space', 'Film'); phone.dropFails = true; accountWith('Space', 'Film');
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual(['Space', 'Film']); expect(phone.legacy).not.toBeNull();
    await toggle(storage, 'Film', E()); expect(stored()).toEqual(['Space']);
    forgetPicks();                                            // the next launch: the file is still there
    await picksText(storage, E()); await settle();
    expect(stored()).toEqual(['Space']);
    phone.dropFails = false; forgetPicks(); await picksText(storage, E()); await settle();
    expect(phone.legacy).toBeNull(); expect(stored()).toEqual(['Space']);
  });
  test('h: a corrupt old file is dropped and adopts nothing', async () => {
    phone.legacy = '{nope'; accountWith('Space');
    await picksText(storage, E()); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).toBeNull();
  });
  test('i: no native module: nothing is read, nothing is adopted', async () => {
    accountWith('Space'); phone.legacy = old('Space');
    await picksText(storage, env(null)); await settle();
    expect(stored()).toBeUndefined(); expect(phone.legacy).not.toBeNull();
  });
  test('j: the first tap before the proof arrives is the listener\'s; the old file adds nothing to it', async () => {
    phone.legacy = old('Space'); accountWith('Space');
    let release: () => void = () => undefined; const hold2 = new Promise<void>((r) => { release = r; });
    const slowGet: Get = async (p, q) => { await hold2; return get(p, q); };
    await picksText(storage, { ...E(), get: slowGet });
    await toggle(storage, 'Film', E());
    release(); await settle();
    expect(stored()).toEqual(['Film']); expect(phone.legacy).toBeNull();
  });
  test('no old file: nothing to prove, no server call', async () => {
    let asked = 0; const counting: Get = async (p, q) => { asked += 1; return get(p, q); };
    await picksText(storage, { ...E(), get: counting }); await settle();
    expect(asked).toBe(0);
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
