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
//  P6 The old Swift file (Application Support/Autoradio/edition-topics.json) is NEVER READ and never deleted by this module or its native calls: it carries no proof of whose it is, so nothing
//     is carried (an upgraded phone starts with no picks, and says so in the reference page); the native sign-out wipe still removes it. A topic is never adopted from it, whatever the notebook says.
//  P6b The page says `loaded` once the file was read for this account generation (the done sends are gated on it); a pick list that was never loaded is not "the listener chose nothing".
//  P7 Speech: "add space and chess" is stored and answered; "done" finishes; heard words are taken from the native mailbox in order and each is answered once, even when refused.
//  P8 The page: rows follow the selection, expanding changes no pick, own topics stay as chips after a deselect (E9), the typed duplicate says so, live sections merge.
import { moveAccount } from './account-phone.ts';
import { beforeEach, describe, expect, test } from 'bun:test';
import { accountGeneration, resetSeam, bindNative } from './api.ts';
import { nativeDevice } from './edition-picks.ts';
import {
  PICKS_FILE, picksPage, picksText, toggle, expand, addTyped, hear, drainHeard, forgetPicks, parseStored, type Device, type Env,
} from './edition-picks.ts';
import { readFileSync } from 'node:fs';

// ---- a real in-memory file system, with a gate to hold a write in flight
const files = new Map<string, Uint8Array>();
let gate: Promise<void> | null = null;
let failWrites = false;
let failRm = false;
let failRmOnce = false;
let failReads = false;
let enteredResolve: (() => void) | null = null;
const storage = {
  fs: {
    atomicWriteFile: async (p: string, d: Uint8Array) => { enteredResolve?.(); if (gate) await gate; if (failWrites) throw new Error('disk full'); files.set(p, d); },
    writeFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); if (failReads) throw new Error('EACCES'); return b; },
    rm: async (p: string) => { if (failRm) throw new Error('busy'); if (failRmOnce) { failRmOnce = false; throw new Error('busy'); } files.delete(p); },
    stat: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return { size: b.length }; },
    mkdir: async () => undefined,
  },
} as never;
/** Hold every write: the returned function releases them, and `.entered` resolves when a write is IN FLIGHT (no timer: the test waits for the fact, not for 20 ms). */
const hold = (): (() => void) & { entered: Promise<void> } => {
  let r: () => void = () => undefined; gate = new Promise<void>((res) => { r = res; });
  const entered = new Promise<void>((res) => { enteredResolve = res; });
  return Object.assign(() => r(), { entered });
};
const text = (p: string): string | undefined => { const b = files.get(p); return b ? new TextDecoder().decode(b) : undefined; };
const stored = (): string[] | undefined => { const t = text(PICKS_FILE); return t === undefined ? undefined : (JSON.parse(t) as { topics: string[] }).topics; };
const put = (p: string, t: string): void => { files.set(p, new TextEncoder().encode(t)); };

// ---- a fake phone: the old file and the native mirror and mailbox are state the test can read back
type Phone = { mirrored: string[][]; heard: { id: string; words: string }[]; said: { id: string; line: string; finish: boolean }[]; answers: boolean; mirrorHangs: boolean };
let phone: Phone;
const fresh = (): Phone => ({ mirrored: [], heard: [], said: [], answers: true, mirrorHangs: false });
const device = (): Device => ({
  mirror: async (t) => { if (phone.mirrorHangs) return new Promise<void>(() => undefined); phone.mirrored.push(t); },
  forget: async () => { phone.mirrored.push([]); },
  heard: async () => phone.heard.splice(0),
  said: async (id, line, finish) => { phone.said.push({ id, line, finish }); },
});
let live: string[];
const env = (device_: Device | null = device()): Env => ({ device: device_, live: async () => live });
const E = (): Env => env();

beforeEach(() => { files.clear(); gate = null; failWrites = false; failRm = false; failRmOnce = false; failReads = false; enteredResolve = null; resetSeam(); bindNative(undefined); forgetPicks(); phone = fresh(); live = []; });

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

describe('P1b a read that fails closed', () => {
  test('a file that is there but cannot be read is NOT "no picks": nothing is loaded, nothing is written over it, and the next ask retries', async () => {
    put(PICKS_FILE, '{"topics":["Space","Chess"]}'); failReads = true;
    await expect(picksPage(storage, {}, E())).rejects.toThrow();
    expect(await hear(storage, 'done', E())).toBeUndefined();             // the voice's "done" changes nothing, so it must not write
    await expect(toggle(storage, 'Film', E())).rejects.toThrow();
    expect(stored()).toEqual(['Space', 'Chess']);
    failReads = false;
    const page = await picksPage(storage, {}, E());
    expect(page.loaded).toBe(true); expect(page.picks).toBe('Space\nChess');
  });
});

describe('P2 writing', () => {
  test('every change is stored after it is made, in the order chosen, one per line for the paper', async () => {
    await toggle(storage, 'Space', E()); expect(stored()).toEqual(['Space']);
    await addTyped(storage, 'Chess, Film', E()); expect(stored()).toEqual(['Space', 'Chess', 'Film']);
    await toggle(storage, 'space', E()); expect(stored()).toEqual(['Chess', 'Film']);
    expect((await picksText(storage, E())).picks).toBe('Chess\nFilm');
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
    await release.entered; release();
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
  test('a, no wipe yet: a moved generation alone empties what B reads from memory (the file is read fresh, and it is the wipe\'s to remove)', async () => {
    await toggle(storage, 'Space', E());
    moveAccount(); files.delete(PICKS_FILE);
    expect((await picksText(storage, E())).picks).toBe('');
  });
});

describe('P5 the native copy', () => {
  test('a first read after a restart gives the native side its copy', async () => {
    put(PICKS_FILE, '{"topics":["Space"]}');
    await picksText(storage, E());
    expect(phone.mirrored.at(-1)).toEqual(['Space']);
  });
});

describe('P6 the old Swift file is never read', () => {
  const ALLOWED = ['editionMirror', 'editionMirrorReset', 'editionHeard', 'editionSaid'];
  test('the only native calls are the voice\'s and the mirror\'s: no read of the old file, no delete of it', async () => {
    const ops: string[] = [];
    bindNative({ later: async (r: { op: string }) => { ops.push(r.op); return {}; } } as never);
    const dev = nativeDevice()!;
    const e: Env = { device: dev, live: async () => [] };
    await picksText(storage, e); await toggle(storage, 'Space', e); await hear(storage, 'add chess', e); await drainHeard(storage, e);
    moveAccount();;
    expect(ops.length).toBeGreaterThan(0);
    expect(ops.filter((o) => !ALLOWED.includes(o))).toEqual([]);
  });
  test('an upgraded phone starts with no picks, whatever an old Swift file holds (it is not even asked about); nothing is seeded', async () => {
    const oldPath = 'Application Support/Autoradio/edition-topics.json';
    put(oldPath, '{"topics":["Space","Chess"]}');               // the old build's file, on the same fake disk: this module has no way to reach it and never names it
    const page = await picksPage(storage, {}, E());
    expect(page.pickedCount).toBe(0); expect(page.picks).toBe(''); expect(stored()).toBeUndefined();
    expect(phone.mirrored.at(-1)).toEqual([]);             // read, nothing there: the list IS empty, and the native copy is told
    expect(text(oldPath)).toBe('{"topics":["Space","Chess"]}');
  });
});

describe('P6b loaded', () => {
  test('a page answered from this account\'s file says loaded; the done sends are gated on it', async () => {
    expect((await picksPage(storage, {}, E())).loaded).toBe(true);
    const c = readFileSync(new URL('./members-onboarding.contract', import.meta.url), 'utf8').split('\n');
    for (const send of ['onboardingAct("topicsDone"', 'onboardingAct("editDone"', 'onboardingAct("editionRetry"']) {
      const i = c.findIndex((l) => l.includes(send)); expect(i).toBeGreaterThan(0);
      const indent = (l: string): number => l.length - l.trimStart().length;
      let j = i - 1; while (j >= 0 && indent(c[j]) >= indent(c[i])) j--;       // the line that opens the block the send is in
      expect(c[j].trim()).toBe('if ep.loaded');
    }
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
