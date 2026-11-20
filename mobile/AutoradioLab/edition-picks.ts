// The edition picks: the topics the listener chose for their edition. THIS module is the one reader and the one writer of `app:/data/edition-topics.json`
// (it replaces the Swift EditionSetupSession, its file in Application Support, and the topic screens' sends). The page logic (the tree, tap, type, say) is
// edition-model.ts; this file is what is kept, for whom, and how it is carried.
//
// One account owns the picks. The state, row by row, is the table in docs/reference/AUTORADIO-EDITION-PICKS.md. In short:
//  - memory (`mem`) belongs to the account generation (api.ts `accountGeneration`): a moved generation starts a new, empty memory that reads the file again;
//  - the file is written ONLY through `writeCurrent` with the generation the change began under, one change at a time (`mutate`'s queue), each change applied to the
//    state as it is at write time (no lost update), and a change queued for an account that has moved on is skipped;
//  - the native side keeps one in-memory copy (the DJ's edition text, the saved corpus and the edition writer read it): pushed after the file is written, emptied at a wipe;
//  - the old Swift file is adopted only on proof, topic by topic (`runLegacy`), never on a flag or a marker.
//
// Native calls, each with the reason only native code can meet (modules/apple/ModelsEntry.swift, `editionOp`): `editionLegacy`/`editionLegacyDrop` read and delete the
// old file, which lives in Application Support, a place shared code cannot see; `editionMirror` gives the voice's native code (the DJ request, the on-device retrieval) its copy;
// `editionHeard`/`editionSaid` carry the words the voice heard at the setup screen to shared code and its one-line answer back, because Exact has no call from native code into
// shared code (native-modules.md, "Out"): shared code asks, on the screen's own timer.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, nativeLater, writeCurrent } from './api.ts';
import { Selection, suggestions, resolve, treeRows, key, tidy, EXPLAINER, VOICE_HINT, selectionLine, type TopicRow } from './edition-model.ts';
import { lookupEdition, parse, seamGet, type Get } from './notebook.ts';
import { textureSource } from './content.ts';
import { listing } from './paper.ts';

export const PICKS_FILE = 'app:/data/edition-topics.json';
export const MIRROR_DEADLINE_MS = 5000;
const DUPLICATE = 'That one is already there.';

/** The phone's native half (the app module), or null where there is none. */
export interface Device {
  /** The old Swift file: `found` and its text; null when native did not answer. */
  legacy(): Promise<{ found: boolean; text: string; unreadable?: boolean } | null>;
  /** Delete the old file. True when native looked again and it is gone. */
  dropLegacy(): Promise<boolean>;
  mirror(topics: string[]): Promise<void>;
  /** The wipe: the native copy is emptied AND forgotten (an empty copy that was never pushed is not the listener's choice). */
  forget(): Promise<void>;
  /** The words the voice heard at the setup screen since the last call, in order. */
  heard(): Promise<{ id: string; words: string }[]>;
  said(id: string, line: string, finish: boolean): Promise<void>;
}
export type Env = { device: Device | null; get: Get; live: () => Promise<string[]>; mirrorDeadlineMs?: number; legacyWaitMs?: number };
/** How long the first page waits for the carry-over's proof before it says it is still looking (the proof goes on and the page is asked again). */
export const LEGACY_WAIT_MS = 3000;

export function nativeDevice(): Device | null {
  const raw = nativeLater();
  if (!raw) return null;
  const later = raw as unknown as (r: Record<string, unknown>) => Promise<unknown>;
  const o = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});
  return {
    async legacy() { const r = o(await later({ op: 'editionLegacy' })); return typeof r.found === 'boolean' ? { found: r.found, text: typeof r.text === 'string' ? r.text : '', unreadable: r.unreadable === true } : null; },
    async dropLegacy() { return o(await later({ op: 'editionLegacyDrop' })).gone === true; },
    async mirror(topics) { await later({ op: 'editionMirror', topics }); },
    async forget() { await later({ op: 'editionMirrorReset' }); },
    async heard() {
      const items = o(await later({ op: 'editionHeard' })).items;
      return (Array.isArray(items) ? items : []).map(o).filter((i) => typeof i.id === 'string' && typeof i.words === 'string').map((i) => ({ id: i.id as string, words: i.words as string }));
    },
    async said(id, line, finish) { await later({ op: 'editionSaid', id, line, finish }); },
  };
}

const delay = <T,>(ms: number, v: T): Promise<T> => new Promise((r) => setTimeout(() => r(v), ms));
/** The live section ids of today's paper, when the paper has them already (a slow paper does not hold the topic screen: it is asked again). */
const liveSections = (storage: Storage | undefined) => async (): Promise<string[]> =>
  Promise.race([listing('', storage).then((r) => r.sections.map((s) => s.id)), delay(300, [] as string[])]).catch(() => [] as string[]);
export const liveEnv = (storage: Storage | undefined): Env => ({ device: nativeDevice(), get: seamGet, live: liveSections(storage) });

// ---- what is kept

/** The topics a stored text holds; anything else (empty, corrupt, wrong shape) is none (E7). */
export function parseStored(text: string | undefined): string[] {
  if (!text) return [];
  try {
    const v = JSON.parse(text) as { topics?: unknown };
    if (!v || !Array.isArray(v.topics)) return [];
    return new Selection(v.topics.filter((t): t is string => typeof t === 'string').map(tidy).filter((t): t is string => t !== undefined)).topics;
  } catch { return []; }
}

type Mem = {
  gen: number; sel: Selection; own: string[]; expanded: Set<string>; live: string[];
  stored: boolean; existedAtOpen: boolean; loaded: Promise<void> | null; rev: number; message: string; mirrored: boolean; legacy: Promise<void> | null;
  carrying: boolean; kept: boolean;
};
let mem: Mem | null = null;
let lastLegacy: Promise<void> = Promise.resolve();
let tail: Promise<unknown> = Promise.resolve();
let mirrorTail: Promise<unknown> = Promise.resolve();

/** For tests: a new process. */
export function forgetPicks(): void { mem = null; lastLegacy = Promise.resolve(); tail = Promise.resolve(); mirrorTail = Promise.resolve(); }
/** For tests: the carry-over of the old file has finished. */
export const legacyDone = (): Promise<void> => lastLegacy;

/** The memory of the CURRENT account generation; a moved generation is a new, empty one (it reads the file again). */
function memFor(): Mem {
  const gen = accountGeneration();
  if (!mem || mem.gen !== gen) mem = { gen, sel: new Selection(), own: [], expanded: new Set(), live: [], stored: false, existedAtOpen: false, loaded: null, rev: 0, message: '', mirrored: false, legacy: null, carrying: false, kept: false };
  return mem;
}

async function ensure(storage: Storage | undefined, m: Mem): Promise<void> {
  if (!m.loaded) {
    m.loaded = (async () => {
      if (!storage) return;
      let t: string | undefined;
      try { t = new TextDecoder().decode(await storage.fs.readFile(PICKS_FILE)); m.stored = true; }
      catch { try { await storage.fs.stat(PICKS_FILE); m.stored = true; } catch { m.stored = false; } }       // unreadable but there still counts as stored: it is never overwritten by a carry-over
      m.existedAtOpen = m.stored;                                                                                    // the carry-over is decided by this, not by what the listener does later
      m.sel = new Selection(parseStored(t));
      m.own = m.sel.topics.filter((x) => !suggestions().some((s) => key(s) === key(x)));
    })();
  }
  await m.loaded;
}

const run = <T,>(fn: () => Promise<T>): Promise<T> => { const r = tail.then(fn, fn); tail = r.catch(() => undefined); return r; };

/** Every change goes through here: one at a time, applied to the state as it is NOW, written through the generation-checked write; skipped when the account moved on
 * between asking and running; undefined when it moved on before the end. After the write the native copy follows (and only then). */
async function mutate<R>(storage: Storage | undefined, env: Env, change: (m: Mem) => R): Promise<{ value: R; wrote: boolean } | undefined> {
  const gen = accountGeneration();
  const out = await run(async () => {
    if (gen !== accountGeneration()) return undefined;
    const m = memFor(); await ensure(storage, m);
    if (m.gen !== gen || mem !== m) return undefined;
    const value = change(m); m.rev += 1;
    const wrote = await writeCurrent(m.gen, storage, PICKS_FILE, new TextEncoder().encode(JSON.stringify({ topics: m.sel.topics })));
    if (wrote) m.stored = true;
    if (gen !== accountGeneration()) return undefined;
    return { value, wrote, topics: [...m.sel.topics] };
  });
  if (!out) return undefined;
  if (out.wrote) await pushMirror(env, gen, out.topics);
  return { value: out.value, wrote: out.wrote };
}

/** Give the native side its copy, in order, each push bounded by a deadline so a native call that never answers stops nothing. A push for an account that moved on is not sent. */
function pushMirror(env: Env, gen: number, topics: string[] | null, force = false): Promise<void> {
  const dev = env.device; if (!dev) return Promise.resolve();
  const p = mirrorTail.then(async () => {
    if (!force && gen !== accountGeneration()) return;
    await Promise.race([(topics === null ? dev.forget() : dev.mirror(topics)).catch(() => undefined), delay(env.mirrorDeadlineMs ?? MIRROR_DEADLINE_MS, undefined)]);
  });
  mirrorTail = p.catch(() => undefined);
  return p.catch(() => undefined);
}

// ---- the page

export type PicksPage = { tree: TopicRow[]; pickedCount: number; selectionLine: string; explainer: string; voiceHint: string; message: string; picks: string; rev: number; carrying: boolean; carryNote: string };

function noteOwn(m: Mem, t: string): void {
  const among = suggestions(m.live);
  if (!among.some((s) => key(s) === key(t)) && !m.own.some((s) => key(s) === key(t))) m.own.push(t);
}
function build(m: Mem): PicksPage {
  const topics = m.sel.topics;
  return {
    tree: treeRows({ liveSections: m.live, expanded: m.expanded, selection: m.sel, own: m.own }), pickedCount: topics.length, selectionLine: selectionLine(topics), explainer: EXPLAINER,
    voiceHint: VOICE_HINT, message: m.message, picks: topics.join('\n'), rev: m.rev, carrying: m.carrying,
    carryNote: m.carrying ? 'Looking for the topics you chose before…' : m.kept ? 'Your earlier topics could not be checked yet. They are tried again when the app next starts.' : '',
  };
}
async function opened(storage: Storage | undefined, env: Env): Promise<Mem> {
  const m = memFor(); await ensure(storage, m);
  if (!m.mirrored) { m.mirrored = true; await pushMirror(env, m.gen, [...m.sel.topics]); }
  if (!m.legacy) { m.carrying = true; m.legacy = runLegacy(storage, env, m).catch(() => { m.kept = true; }).finally(() => { m.carrying = false; m.rev += 1; }); lastLegacy = m.legacy; }
  await Promise.race([m.legacy, delay(env.legacyWaitMs ?? LEGACY_WAIT_MS, undefined)]);      // the first page is not built before the proof has answered, unless it is slow
  return m;
}

/** The picks for the paper, one per line. */
export async function picksText(storage: Storage | undefined, env: Env = liveEnv(storage)): Promise<{ picks: string }> {
  const m = await opened(storage, env);
  return { picks: m.sel.topics.join('\n') };
}
export async function picksPage(storage: Storage | undefined, a: { talking?: boolean } = {}, env: Env = liveEnv(storage)): Promise<PicksPage> {
  if (a.talking) await drainHeard(storage, env);
  const m = await opened(storage, env);
  m.live = await env.live().catch(() => []);
  return build(m);
}
const pageNow = (storage: Storage | undefined, env: Env): Promise<PicksPage> => picksPage(storage, {}, env);

/** A tap. */
export async function toggle(storage: Storage | undefined, label: string, env: Env = liveEnv(storage)): Promise<PicksPage> {
  await mutate(storage, env, (m) => { m.message = ''; const among = suggestions(m.live); m.sel.toggle(label, among); if (m.sel.contains(label)) noteOwn(m, resolve(label, among) ?? label); });
  return pageNow(storage, env);
}
/** The disclosure control: not a pick, not written. */
export async function expand(storage: Storage | undefined, label: string, env: Env = liveEnv(storage)): Promise<PicksPage> {
  const m = await opened(storage, env); m.message = '';
  const k = key(label); if (m.expanded.has(k)) m.expanded.delete(k); else m.expanded.add(k);
  m.rev += 1;
  return pageNow(storage, env);
}
/** Typed words. */
export async function addTyped(storage: Storage | undefined, text: string, env: Env = liveEnv(storage)): Promise<PicksPage> {
  await mutate(storage, env, (m) => {
    const added = m.sel.addTyped(text, suggestions(m.live)); added.forEach((t) => noteOwn(m, t));
    m.message = added.length === 0 && text.trim() !== '' ? DUPLICATE : '';
  });
  return pageNow(storage, env);
}
export async function clearPicks(storage: Storage | undefined, env: Env = liveEnv(storage)): Promise<PicksPage> {
  await mutate(storage, env, (m) => { m.message = ''; m.sel = new Selection(); m.own = []; });
  return pageNow(storage, env);
}
/** What the reader said: the one line to say back and whether they are finished; undefined when the account moved on. */
export async function hear(storage: Storage | undefined, words: string, env: Env = liveEnv(storage)): Promise<{ line: string; finish: boolean } | undefined> {
  const r = await mutate(storage, env, (m) => { m.message = ''; const o = m.sel.apply(words, suggestions(m.live)); o.added.forEach((t) => noteOwn(m, t)); return { line: o.reply, finish: o.finish }; });
  return r?.value;
}

/** The words the voice heard, applied in order; each is answered once (an empty line when the account moved on or nothing could be done). */
export async function drainHeard(storage: Storage | undefined, env: Env = liveEnv(storage)): Promise<void> {
  const dev = env.device; if (!dev) return;
  const gen = accountGeneration();
  let items: { id: string; words: string }[] = [];
  try { items = await dev.heard(); } catch { return; }
  for (const it of items) {
    const r = gen === accountGeneration() ? await hear(storage, it.words, env).catch(() => undefined) : undefined;
    try { await dev.said(it.id, r?.line ?? '', r?.finish ?? false); } catch { /* the voice's own deadline answers */ }
  }
}

// ---- the sign-out

/** Forget the memory and remove the file; when it cannot be removed, empty it; when it cannot be emptied, say so (account.ts names the store and tries again at the next launch). */
export async function wipePicks(storage: Storage | undefined, env: Env = liveEnv(storage)): Promise<void> {
  mem = null;
  const fs = storage?.fs;
  let failed = false;
  if (fs) {
    try { await fs.rm(PICKS_FILE); } catch { /* checked below */ }
    let there = true; try { await fs.stat(PICKS_FILE); } catch { there = false; }
    if (there) { try { await fs.atomicWriteFile(PICKS_FILE, new TextEncoder().encode('{"topics":[]}')); } catch { failed = true; } }
  }
  await pushMirror(env, 0, null, true);          // the native wipe is the other path
  if (failed) throw new Error('the edition picks could not be cleared');
}

// ---- the old Swift file: adopted by proof

/** The topics the signed-in account's own notebook lists under "What you asked for": key -> the notebook's own line (the first one with that key). Null when the server cannot say; an
 * account with no notebook is the empty map. */
async function askedLines(get: Get): Promise<Map<string, string> | null> {
  const ed = await lookupEdition(get);
  if (ed === null) return null;
  if (!ed.found) return new Map();
  let source: string | null = null;
  try { source = await textureSource(ed.docID, get); } catch { source = null; }
  if (source === null) return null;
  const out = new Map<string, string>();
  for (const l of parse(source).sections.filter((x) => x.key === 'asked').flatMap((x) => x.lines)) {
    const t = tidy(l.text); const k = t === undefined ? '' : key(t);
    if (t !== undefined && k !== '' && !out.has(k)) out.set(k, t);
  }
  return out;
}

/** The old file says nothing about whose it is, and the native delete at sign-out can fail without a sign. So no guard here infers ownership from the phone's history. The table of
 * cells (docs/reference/AUTORADIO-EDITION-PICKS.md, tests c0 to c13) is decided by FACTS read now: the old file (absent / unreadable / no topics / topics), whether the picks file existed
 * when this generation was first read (`existedAtOpen`), and what the signed-in account's own notebook answers. What is adopted is the NOTEBOOK'S line for each topic of the old file whose
 * key it has, never the old file's string (an old "C++" is not B's "C"). A proof that cannot be asked keeps the file; the same evidence decides at the next launch. */
async function runLegacy(storage: Storage | undefined, env: Env, m: Mem): Promise<void> {
  const dev = env.device; if (!dev) return;
  const old = await dev.legacy().catch(() => null);
  if (!old || !old.found || m.gen !== accountGeneration()) return;
  if (old.unreadable) { m.kept = true; return; }
  const topics = parseStored(old.text);
  if (topics.length > 0 && !m.existedAtOpen) {
    const asked = await askedLines(env.get).catch(() => null);
    if (asked === null) { m.kept = true; return; }
    if (m.gen !== accountGeneration()) return;
    const matched: string[] = [];
    for (const t of topics) { const line = asked.get(key(t)); if (line !== undefined && !matched.includes(line)) matched.push(line); }
    if (matched.length > 0) {
      // merged into what the listener has now (they may have changed picks while this ran; an invisible old topic cannot have been taken off)
      const r = await mutate(storage, env, (x) => { x.sel = new Selection([...x.sel.topics, ...matched]); x.own = x.sel.topics.filter((t) => !suggestions(x.live).some((s) => key(s) === key(t))); });
      if (!r) return;
      if (!r.wrote) { m.kept = true; return; }               // adopted but not written: nothing is decided, the file stays
    }
  }
  if (m.gen === accountGeneration()) await dev.dropLegacy().catch(() => false);
}
