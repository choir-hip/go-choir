// The edition picks: the topics the listener chose for their edition. THIS module is the one reader and the one writer of `app:/data/edition-topics.json`
// (it replaces the Swift EditionSetupSession, its file in Application Support, and the topic screens' sends). The page logic (the tree, tap, type, say) is
// edition-model.ts; this file is what is kept, for whom, and how it is carried.
//
// One account owns the picks. The state, row by row, is the table in docs/reference/AUTORADIO-EDITION-PICKS.md. In short:
//  - memory (`mem`) belongs to the account generation (api.ts `accountGeneration`): a moved generation starts a new, empty memory that reads the file again;
//  - the file is written ONLY through `writeCurrent` with the generation the change began under, one change at a time (`mutate`'s queue), each change applied to the
//    state as it is at write time (no lost update), and a change queued for an account that has moved on is skipped;
//  - the native side keeps one in-memory copy (the DJ's edition text, the saved corpus and the edition writer read it): pushed after the file is written, emptied at a wipe;
//  - the old Swift file (Application Support/Autoradio/edition-topics.json) is NEVER READ here and nothing deletes it here: it says nothing about whose it is, so an upgraded
//    phone starts with no picks (the cost, in the reference page). The native sign-out wipe still removes it.
//
// Native calls, each with the reason only native code can meet (modules/apple/ModelsEntry.swift, `editionOp`): `editionMirror`/`editionMirrorReset` give the voice's native code
// (the DJ request, the on-device retrieval) its copy; `editionHeard`/`editionSaid` carry the words the voice heard at the setup screen to shared code and its one-line answer back,
// because Exact has no call from native code into shared code (native-modules.md, "Out"): shared code asks, on the screen's own timer.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, isRefusal, nativeLater, writeCurrent } from './api.ts';
import { Selection, suggestions, resolve, treeRows, key, tidy, EXPLAINER, VOICE_HINT, selectionLine, type TopicRow } from './edition-model.ts';
import { listing } from './paper.ts';

export const PICKS_FILE = 'app:/data/edition-topics.json';
export const MIRROR_DEADLINE_MS = 5000;
const DUPLICATE = 'That one is already there.';

/** The phone's native half (the app module), or null where there is none. */
export interface Device {
  /** `gen` is the generation the caller checked; native answers `ok: false` for a copy computed for an account that has since signed out (the result is false then, undefined or true otherwise). */
  mirror(topics: string[], gen: number): Promise<boolean | void>;
  /** The wipe: the native copy is emptied AND forgotten (an empty copy that was never pushed is not the listener's choice). */
  forget(): Promise<void>;
  /** The words the voice heard at the setup screen since the last call, in order. */
  heard(): Promise<{ id: string; words: string }[]>;
  said(id: string, line: string, finish: boolean): Promise<void>;
}
export type Env = { device: Device | null; live: () => Promise<string[]>; mirrorDeadlineMs?: number };

export function nativeDevice(): Device | null {
  const raw = nativeLater();
  if (!raw) return null;
  const later = raw as unknown as (r: Record<string, unknown>) => Promise<unknown>;
  const o = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});
  return {
    async mirror(topics, gen) { return o(await later({ op: 'editionMirror', topics, gen })).ok !== false; },       // stamped with the generation the caller checked: native drops a copy computed for an account that has since signed out (a late copy would be written into the next account's notebook)
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
export const liveEnv = (storage: Storage | undefined): Env => ({ device: nativeDevice(), live: liveSections(storage) });

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

type Mem = { gen: number; sel: Selection; own: string[]; expanded: Set<string>; live: string[]; loaded: Promise<void> | null; rev: number; message: string; mirrored: boolean };
let mem: Mem | null = null;
let tail: Promise<unknown> = Promise.resolve();
let mirrorTail: Promise<unknown> = Promise.resolve();

/** For tests: a new process. */
export function forgetPicks(): void { mem = null; tail = Promise.resolve(); mirrorTail = Promise.resolve(); }
/** The memory of the CURRENT account generation; a moved generation is a new, empty one (it reads the file again). */
function memFor(): Mem {
  const gen = accountGeneration();
  if (!mem || mem.gen !== gen) mem = { gen, sel: new Selection(), own: [], expanded: new Set(), live: [], loaded: null, rev: 0, message: '', mirrored: false };
  return mem;
}

async function ensure(storage: Storage | undefined, m: Mem): Promise<void> {
  if (!m.loaded) {
    m.loaded = (async () => {
      if (!storage) return;
      let t: string | undefined;
      try { t = new TextDecoder().decode(await storage.fs.readFile(PICKS_FILE)); }
      catch {
        // Absent is "no picks yet". A file that is THERE but cannot be read is not: fail closed (loaded stays false, nothing is written over it, the next ask reads again).
        let there = true; try { await storage.fs.stat(PICKS_FILE); } catch { there = false; }
        if (there) throw new Error('the edition picks could not be read');
      }
      m.sel = new Selection(parseStored(t));
      m.own = m.sel.topics.filter((x) => !suggestions().some((s) => key(s) === key(x)));
    })();
  }
  try { await m.loaded; } catch (e) { m.loaded = null; throw e; }
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
    if (gen !== accountGeneration()) return undefined;
    return { value, wrote, topics: [...m.sel.topics] };
  });
  if (!out) return undefined;
  if (out.wrote && !(await pushMirror(env, gen, out.topics)) && mem?.gen === gen) mem.mirrored = false;       // refused by native: sent again at the next open
  return { value: out.value, wrote: out.wrote };
}

/** Give the native side its copy, in order, each push bounded by a deadline so a native call that never answers stops nothing. A push for an account that moved on is not sent. */
function pushMirror(env: Env, gen: number, topics: string[] | null, force = false): Promise<boolean> {
  const dev = env.device; if (!dev) return Promise.resolve(true);
  const p = mirrorTail.then(async (): Promise<boolean> => {
    if (!force && gen !== accountGeneration()) return true;                  // an account that moved on: nothing to send, nothing to resend
    const r = await Promise.race([(topics === null ? dev.forget() : dev.mirror(topics, gen)).catch((e: unknown) => (isRefusal(e) ? false : undefined)), delay(env.mirrorDeadlineMs ?? MIRROR_DEADLINE_MS, undefined)]);
    return r !== false;                                                      // native's refusal is false: the copy is sent again (the next time the picks open or change)
  });
  mirrorTail = p.catch(() => undefined);
  return p.catch((e: unknown) => !isRefusal(e));                            // refused (the answer that ran this link was let go): not sent, so sent again at the next open
}

// ---- the page

export type PicksPage = { tree: TopicRow[]; pickedCount: number; selectionLine: string; explainer: string; voiceHint: string; message: string; picks: string; rev: number; loaded: boolean };

function noteOwn(m: Mem, t: string): void {
  const among = suggestions(m.live);
  if (!among.some((s) => key(s) === key(t)) && !m.own.some((s) => key(s) === key(t))) m.own.push(t);
}
function build(m: Mem): PicksPage {
  const topics = m.sel.topics;
  return {
    tree: treeRows({ liveSections: m.live, expanded: m.expanded, selection: m.sel, own: m.own }), pickedCount: topics.length, selectionLine: selectionLine(topics), explainer: EXPLAINER,
    voiceHint: VOICE_HINT, message: m.message, picks: topics.join('\n'), rev: m.rev, loaded: true,   // the file was read for this generation (`opened` awaits it): the list is the listener's, empty or not
  };
}
async function opened(storage: Storage | undefined, env: Env): Promise<Mem> {
  const m = memFor(); await ensure(storage, m);
  if (!m.mirrored) { m.mirrored = true; if (!(await pushMirror(env, m.gen, [...m.sel.topics]))) m.mirrored = false; }
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
  const live = await env.live().catch(() => []);
  if (m.gen === accountGeneration()) m.live = live;
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
/** What the reader said: the one line to say back and whether they are finished; undefined when the account moved on. */
export async function hear(storage: Storage | undefined, words: string, env: Env = liveEnv(storage)): Promise<{ line: string; finish: boolean } | undefined> {
  const r = await mutate(storage, env, (m) => { m.message = ''; const o = m.sel.apply(words, suggestions(m.live)); o.added.forEach((t) => noteOwn(m, t)); return { line: o.reply, finish: o.finish }; }).catch(() => undefined);
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

/** Forget the memory and remove the file; when it cannot be removed, empty it and remove it again; when it still stays, say so (account.ts names the store and tries again at the next launch). */
export async function wipePicks(storage: Storage | undefined, env: Env = liveEnv(storage)): Promise<void> {
  mem = null;
  const fs = storage?.fs;
  let failed = false;
  if (fs) {
    const there = async (): Promise<boolean> => { try { await fs.stat(PICKS_FILE); return true; } catch { return false; } };
    try { await fs.rm(PICKS_FILE); } catch { /* checked below */ }
    if (await there()) {
      // rm failed: empty it so the next account cannot read the picks, then remove it again; if it still stays, the store is named failed (account.ts keeps its marker and retries)
      try { await fs.atomicWriteFile(PICKS_FILE, new TextEncoder().encode('{"topics":[]}')); } catch { /* still failed below */ }
      try { await fs.rm(PICKS_FILE); } catch { /* checked below */ }
      failed = await there();
    }
  }
  await pushMirror(env, 0, null, true);          // the native wipe is the other path
  if (failed) throw new Error('the edition picks could not be cleared');
}
