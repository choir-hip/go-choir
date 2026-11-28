// The radio, as the app's sources and sends: the one place shared code assembles the runtime (radio-runtime.ts), its writer (radio-writer.ts) and the real deck (deck-native.ts), and answers the
// names the contracts use for it (queueView, queueAdd, queueAct, radioPageFacts, barRun, radioPlayPause, radioPlayStory, radioStart, radioListen, radioMove, radioPulse).
// What stays native and why: the deck (the audio), a few facts only the audio code knows (`radioFacts`: a turn is open, what is left of the spoken entry, the place in it), the book text (the
// shelf is native until it moves) and the voice turn's queue snapshot and server ops (`radioSet`, and the inbox in `radioFacts`): the turn engine is native and Exact has no call from native into shared code,
// so shared code PUSHES the snapshot and native hands back the server's `queue_ops` text, which the pump applies (the retrieval lane's pattern).
import { authedText, authedToFile, getJSON, nativeLater, accountGeneration, sealingBlocked, signOutUncertain, type NativeReader } from './api.ts';
import { contractDeck, deckPort } from './deck-native.ts';
import { refusedHere } from './refused.ts';
import { RadioRuntime, NO_FACTS, type NativeFacts, type Resolvers } from './radio-runtime.ts';
import { Writer, ASK_DEADLINE_MS, phoneOpening, PROGRAMME_PATH, frontFill } from './radio-writer.ts';
import { applyTurnOps, resetTurnLog, turnQueueJSON } from './voice-queue.ts';
import { routePoster, signedInRequester } from './runsheet-store.ts';
import { authedJSON, isUnreadable } from './api.ts';

/** The `kind` a failed programme post carries (radio-writer.ts). A key that could not be read is `transport` ("the desk can't be reached right now", asked again), never `signedOut` ("Sign in to hear the desk"). */
export const programmeFailKind = (e: unknown): 'signedOut' | 'transport' | 'other' => {
  const s = (e as { status?: number } | null)?.status;
  return (e as Error | null)?.name === 'SignedOutError' && !isUnreadable(e) ? 'signedOut' : s === undefined ? 'transport' : 'other';
};
import { audioSpec, audio as audioEntry, book as bookEntry, filesAudio, mintId, paperStory, readAloud, webPage, estimateSeconds, heldStory, forgetLongURLs } from './runsheet-entries.ts';
import { offlineStories } from './content-run.ts';
import { getText } from './api.ts';
import { storyWords } from './story-text.ts';
import { fileOpen } from './files.ts';
import { lapYield, lap } from './lap.ts';
import { firstRunOver } from './onboarding.ts';
import { deskMode } from './consent.ts';
import { perform } from './radio-model.ts';
import { voiceFacts, respondsOf, airAddressOf } from './voice-facts.ts';
import { nowOf, runOf } from './radio-follow.ts';
import { htmlToText } from './mail-model.ts';
import { parseList, weight, FRONT_LIMIT, type Story } from './paper-model.ts';
import { listing } from './paper.ts';
import { parseEpisodeKey, episodeAddress } from './podcast-model.ts';
import { loadKept, takesOf, type Kept, type Take } from './takes.ts';
import { seamNet } from './takes.ts';
import { clock, now as clockNow } from './clock.ts';
import { decodeEntry, type Entry, type Sheet } from './runsheet-model.ts';
import type { Storage as RunStorage } from './runsheet-store.ts';
import type { RadioFacts } from './radio-page.ts';

type Store = { fs: { readFile(p: string): Promise<Uint8Array>; atomicWriteFile(p: string, b: Uint8Array): Promise<void>; rm(p: string): Promise<void>; rename?(from: string, to: string): Promise<void> } };
export const SHEET_FILE = 'app:/data/runsheet.json';
/** Where a sheet file that cannot be read is moved (kept, never deleted), so the radio can start with an empty sheet. */
export const SHEET_BAD_FILE = 'app:/data/runsheet.bad.json';
/** Failed reads of the sheet (not Exact's refusals: those are an answer let go, and pass) before the radio gives the file up and starts empty: one a second while the hold is due. */
export const RESTORE_TRIES = 5;

const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

/** A mutation in flight (or called for): `due` is a pulse that said the work is due and the mutation not started yet, `running` the mutation's body. Each goes stale (WORK_STALE_MS) so a continuation that never ends cannot stop the radio. */
type Flag = { phase: 'due' | 'running'; at: number };
type Radio = { writeFlag?: Flag; holdFlag?: Flag; restoreFails: number; restoreTold: boolean; pulses: number; beatSecond: number; gen: number; setKey: string; rt: RadioRuntime; writer: Writer; storage: Store | undefined; web: { url: string }; inbox: string[]; front: { at: number; published: Record<string, number>; versions: Record<string, number>; lines: { title: string; dek: string }[]; stories: { route: string; title: string }[] }; tick: number; beat: number; takeList: Take[]; started: boolean; ready: boolean; readySince: number; fetchFront(): Promise<unknown>; native?: NativeReader };
let radio: Radio | undefined;

/** Fresh state (tests; a sign-out wipes through `wipeRadio`). */
export function resetRadio(): void { radio = undefined; }
export async function wipeRadio(storage: Store | undefined): Promise<void> {
  const r = radio; radio = undefined;
  resetTurnLog();                                                                            // the lists sent with the old account's turns and its turn ids
  forgetLongURLs();                                                                          // the long web addresses of this account's entries (runsheet-entries.ts)
  try { if (r) { r.writer.stop(); await r.rt.signOut(); } }
  finally {
    // The package owns the native current/held items, journal and synthesized-audio cache. This is an explicit account-boundary send, not an overloaded stop; on a host without the package it is the empty state.
    await deckPort().wipe();
    // The old account's sheet file goes whatever the wipe did (a throw above must not leave it for the next radio to restore); a removal that fails is remembered and tried again before any radio restores.
    if (storage) { try { await storage.fs.rm(SHEET_FILE); sheetFileStuck = false; } catch (e) { if (!/ENOENT|no such file|not found/i.test(String((e as Error)?.message ?? e))) sheetFileStuck = true; } }
    if (storage) { try { await storage.fs.rm(SHEET_BAD_FILE); } catch { /* absent; a set-aside sheet is the old account's too */ } }
  }
}
/** The sheet file could not be removed by the last wipe: the next radio removes it again and starts EMPTY (a sheet that outlived a sign-out is not the next account's). */
let sheetFileStuck = false;

const absent = (e: unknown): boolean => (e as { code?: unknown } | null)?.code === 'ENOENT' || /ENOENT|no such file|not found/i.test(String((e as Error)?.message ?? e));
function runStorage(s: Store | undefined): RunStorage | undefined {
  if (!s) return undefined;
  return {
    // Only an absent file is "no sheet". Any other failure (a read Exact refused because its answer was let go, a disk error) throws: the radio stays not restored and the next hold reads again.
    read: async () => { try { return new TextDecoder().decode(await s.fs.readFile(SHEET_FILE)); } catch (e) { if (absent(e)) return undefined; throw e; } },
    write: async (t) => { await s.fs.atomicWriteFile(SHEET_FILE, new TextEncoder().encode(t)); },
    remove: async () => { try { await s.fs.rm(SHEET_FILE); } catch { /* absent */ } },
  };
}

/** The native facts, over the app module's `radioFacts` op: the audio's own facts and the inbox of what native callers and the voice turn handed over. */
async function readFacts(r: { inbox: string[] } & Partial<Radio>): Promise<NativeFacts> {
  const later = nativeLater();
  if (!later) return NO_FACTS;
  try {
    const o = obj(await later({ op: 'radioFacts' }));
    if (num(o.at) > 0) clock.ms = num(o.at);
    r.ready = o.ready === true;
    if (Array.isArray(o.inbox)) for (const m of o.inbox) if (typeof m === 'string') r.inbox.push(m);
    const pos = obj(o.position), a = obj(o.audio), rem = obj(o.remote);
    return {
      turnOpen: o.turnOpen === true, remainder: str(o.remainder), position: { block: num(pos.block), seconds: num(pos.seconds) }, wants: o.wants === true,
      // A host that gives no `foreground` (Android has no native module in this repo, a host with no native package, the web) is taken as IN FRONT: shared code cannot tell otherwise, and the page's own visibility
      // (spine.contract `posKey`) still gates there. The cost of being wrong is one more read a second, never a frozen position.
      foreground: o.foreground !== false,
      audio: str(a.url) !== '' ? { url: str(a.url), title: str(a.title), show: str(a.show), start: num(a.start), end: a.end === undefined ? undefined : num(a.end), episode_id: str(a.episode_id) || undefined, elapsed: num(a.elapsed) } : undefined,
      remote: rem.duration !== undefined ? { elapsed: num(rem.elapsed), duration: num(rem.duration) } : undefined,
    };
  } catch { return NO_FACTS; }
}

/** The story words the desk's writer has already fetched, by route: the deck's start uses them, so a
 *  pump never waits on the network (an answer that waits is let go and its deck call with it). */
const wordCache = new Map<string, { title: string; paragraphs: string[] }>();
const cacheWords = (route: string, words: { title: string; paragraphs: string[] }): void => {
  wordCache.delete(route); wordCache.set(route, words);
  while (wordCache.size > 48) { const first = wordCache.keys().next().value as string | undefined; if (first === undefined) break; wordCache.delete(first); }
};

/** A story's words from its resolve bundle without parsing the bundle: one JSON.parse of a 1.4 MB
 *  bundle is over Exact's turn budget on the phone and the radio is refused (`story-text.ts`).
 *  The reader page still parses it for its blocks and citations. */
async function storyText(route: string): Promise<{ title: string; paragraphs: string[] } | undefined> {
  // The server's route (`/pub/texture/…`), not the app's address (`pub:…`): the plan hands the
  // route, and a caller that hands the address loses its scheme here.
  const path = route.startsWith('pub:') ? route.slice(4) : route;
  const held = wordCache.get(path);
  if (held) return held;
  const words = storyWords(await getText('/api/platform/publications/resolve', { route: path }));
  if (words) cacheWords(path, words);
  return words;
}

function resolvers(storage: Store | undefined): Partial<Resolvers> {
  return {
    story: (route) => storyText(route),
    webPage: async (url, title) => {
      const res = await fetch(url, { method: 'GET', headers: { accept: 'text/html' } } as RequestInit);
      if (!res.ok) return undefined;
      const text = htmlToText(await res.text()).split(/\n{2,}/).map((p) => p.replace(/\s+/g, ' ').trim()).filter((p) => p !== '');
      return text.length ? { title, paragraphs: text } : undefined;
    },
    // A book's words come with its entry (imported.ts hands them through radioHandWords): the sheet holds no way to read one by id.
    book: async () => undefined,
    take: async (id) => {
      const dest = `app:/cache/files/take-${id}/audio.m4a`;
      try { await authedToFile(`/api/platform/takes/${encodeURIComponent(id)}/audio`, dest, storage as never); return { url: dest }; } catch { return undefined; }
    },
    file: async (address) => {
      // Files audio is brought to the phone by the Files page's own open (the cache folder `app:/cache/files/<key>/<name>`); the deck turns that address into the file.
      const info = await fileOpen({ address }, storage as never);
      return info.status === 'ready' && info.local.startsWith('app:/cache/files/') ? { url: info.local } : undefined;
    },
  };
}

async function makeRadio(storage: Store | undefined, native: NativeReader | undefined): Promise<Radio> {
  const state: Radio = { restoreFails: 0, restoreTold: false, pulses: 0, beatSecond: -1, gen: accountGeneration(), setKey: '', rt: undefined as never, writer: undefined as never, storage, web: { url: '' }, inbox: [], takeList: [], front: { at: 0, published: {}, versions: {}, lines: [], stories: [] }, tick: 0, beat: 0, started: false, ready: false, readySince: 0, fetchFront: undefined as never, native };
  const rt = new RadioRuntime({
    deck: deckPort(), storage: runStorage(storage), poster: routePoster(signedInRequester(authedJSON)), generation: accountGeneration, resolvers: resolvers(storage), hasWords: (route) => wordCache.has(route), prefetchWords: async (route) => { await storyText(route); },
    native: { facts: () => readFacts(state) },
    fill: (removed) => frontFill(state.front.stories, state.rt.store.sheet.entries, undefined, removed),
    onChange: () => { state.tick += 1; },
  });
  state.rt = rt;
  if (!storage) rt.markRestored();                                // no storage (a host without app storage, a test): there is no file to read
  const front = async (): Promise<unknown> => {
    let body: unknown, stories: Story[];
    try { body = (await getJSON('/api/platform/publications/front', { limit: FRONT_LIMIT })).body; await yieldTurn('front list received'); stories = parseList(body); }
    catch (e) {
      // Offline: the radio opens from the paper's last good front list (paper.ts keeps it for an offline start) and the desk is not asked; the error stands for the caller.
      stories = (await listing('', storage as never)).stories;
      if (stories.length === 0) throw e;
      body = undefined;
    }
    const now = clock.ms;
    // `stories`: what plays when nothing is ahead (the director's fill), the paper's weight first, the list's order between equals
    const byWeight = stories.map((s, i) => ({ s, i, w: weight(s, now) })).sort((a, b) => (a.w !== b.w ? b.w - a.w : a.i - b.i)).map((x) => ({ route: x.s.route, title: x.s.title }));
    state.front = { at: now / 1000, published: {}, versions: {}, lines: stories.slice(0, 10).map((s) => ({ title: s.title, dek: s.dek })), stories: byWeight };
    for (const s of stories) { if (s.updated !== null) state.front.published[s.route] = s.updated / 1000; state.front.versions[s.route] = s.versions; }
    if (body === undefined) throw new Error('offline: the last front page');
    return body;
  };
  state.fetchFront = front;
  state.writer = new Writer({
    rt, front,
    texts: async (routes) => {
      // the writer sends the first 8 KB of each: the rest of a 1.6 MB story is not read
      const out: Record<string, string[]> = {};
      await Promise.race([Promise.all(routes.map(async (r) => { try { const w = await storyText(r); if (!w) return; const ps: string[] = []; let n = 0; for (const p of w.paragraphs) { ps.push(p); n += p.length; if (n > 24 * 1024) break; } out[r] = ps; } catch { /* goes without text */ } })), new Promise((res) => setTimeout(res, 3000))]);
      return out;
    },
    takes: async () => {
      let takes: Take[] = []; let kept: Kept = await loadKept(storage as never);
      try { takes = takesOf((await seamNet('GET', '/api/platform/takes', { query: { limit: 50 } })).json); } catch { /* none offered */ }
      state.takeList = takes;                                      // what the page shows of a take on air (its transcript and its record)
      return { takes, kept };
    },
    edition: async () => '',
    mode: () => deskMode(),                                          // no account: the paper plays in its own order and nothing goes to a model (consent.ts)
    // The offline story fill: the stories the phone holds whole (content-run.ts), as the Swift LocalStories gave them; the entry builder is runsheet-entries.ts.
    offlineFill: async () => {
      const entries = rt.store.sheet.entries;
      const told = entries.filter((e) => e.state !== 'ahead').map((e) => e.title), held = new Set(entries.map((e) => e.id));
      return (await offlineStories(storage as never, { told, held })).map((l) => heldStory(l.id, l.title, l.text, l.address));
    },
    post: async (body) => {
      try { return await authedText(PROGRAMME_PATH, { method: 'POST', body }); }
      catch (e) { throw Object.assign(e as Error, { kind: programmeFailKind(e) }); }
    },
    now: clockNow, tz: () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', requestId: () => 'radio-' + mintId('').slice(0, 12), generation: accountGeneration,
  });
  // The writer is asked from the pulse (radioPulse awaits it): a fetch started from a hook inside some other answer, and carried on after that answer ended, is never run (Exact: "fetch() called outside an answer").
  return state;
}

/** Ends the turn (Exact gives one about 100 ms of CPU; Bun has no scheduler). */
const yieldTurn = (label: string): Promise<void> => lapYield(`bridge: ${label}`);

/** The radio of this process, made once. It is NOT restored here: a source (the pulse, the screens' reads) only reads, and an answer that is let go must not start the once-a-launch restore
 *  (docs/problems/radio-restore-empty-after-let-go-2026-11-25.md). Mutations take it through `workRadio`, which restores first.
 *  `read` false: the screens' reads (the queue, the bar's run, the page's facts) need what the pulse just read, not their own native call for it. Mutations and the pulse read (the device's time first). */
export async function getRadio(storage: Store | undefined, native: NativeReader | undefined, read = true): Promise<Radio> {
  if (!radio) radio = await makeRadio(storage, native);
  const r = radio;                                          // held locally: a sign-out (`wipeRadio`) clears the module's variable at any await below
  if (read) { const f = await readFacts(r); if (f !== NO_FACTS) r.rt.useFacts(f); }      // the device's time first: nothing below may run on a stale clock; the runtime keeps what was read
  if (radio !== r) {                                          // signed out meanwhile: this radio is the old account's. What native handed over in that read was drained from native: it goes to the radio that replaced this one, not into the one discarded (review B1-10)
    const next = await getRadio(storage, native, read); next.inbox.unshift(...r.inbox); r.inbox = []; return next;
  }
  return r;
}

/** The sheet from its file, once per radio, in the mutation that asks (a mutation is not let go by the next pulse). The outcome is the runtime's value (`rt.restored`); a read that fails leaves it
 *  false, and the pulse keeps saying `hold` until a hold restores it. Never a kept promise: another answer's refusal must not become this one's empty sheet. */
async function restoreOf(r: Radio, storage: Store | undefined): Promise<boolean> {
  if (r.rt.restored) return true;
  if (sheetFileStuck) { try { await storage?.fs.rm(SHEET_FILE); sheetFileStuck = false; } catch { /* still there: not read either */ } r.rt.markRestored(); return true; }
  try { await r.rt.restore(); return true; }
  catch (e) {
    const why = e instanceof Error ? e.message : String(e);
    // A refusal is this answer let go, not the file: it passes, and is not counted. A read that fails as the disk's own failure is counted, and a refusal of the grant itself (`denied`, `unsupported`,
    // `bake`: storage.js `storageError`) will not pass: then, or after RESTORE_TRIES failures, the file is moved aside (kept for a person to look at) and the radio starts with an empty sheet. One line
    // when it first fails and one when it gives up, not one a second.
    const permanent = !refusedHere(e) && ['denied', 'unsupported', 'bake'].includes(String((e as { code?: unknown } | null)?.code ?? ''));
    if (!refusedHere(e)) r.restoreFails += 1;
    if (!permanent && (refusedHere(e) || r.restoreFails < RESTORE_TRIES)) {
      if (!r.restoreTold) { r.restoreTold = true; console.log(`radio: restore: the run sheet could not be read yet (${why}); the next hold reads it again`); }
      return false;
    }
    let kept = 'it could not be moved aside, and the next save replaces it';
    if (storage?.fs.rename) {
      try { await storage.fs.rename(SHEET_FILE, SHEET_BAD_FILE); kept = `it is kept as ${SHEET_BAD_FILE}`; }
      catch (m) { if (refusedHere(m)) return false; /* the next hold gives up again, as itself */ }
    }
    r.rt.markRestored();
    console.log(`radio: restore: the run sheet could not be read (${why}; ${r.restoreFails} failed read${r.restoreFails === 1 ? '' : 's'}): ${kept}, and the radio starts with an empty sheet`);
    return true;
  }
}

/** The radio for a mutation that changes the sheet or acts on it: restored first, so an act never lands on an empty sheet the file then replaces. A sheet that cannot be read yet refuses the act. */
async function workRadio(storage: Store | undefined, native: NativeReader | undefined): Promise<Radio> {
  const r = await getRadio(storage, native);
  if (!(await restoreOf(r, storage))) throw new Error('The radio is still reading its list. Try again in a moment.');
  await turnEnd();
  return radio === r ? r : workRadio(storage, native);
}

/** Ends the turn before the expensive half of a mutation: Exact allows about 100 ms of CPU per turn, and a
 *  first tap restores and parses the sheet (over 100 ms on the phone). The mutation still completes; it needs
 *  its own turn rather than being refused. */
const turnEnd = async (): Promise<void> => {
  const sc = (globalThis as { scheduler?: { yield?: () => Promise<void> } }).scheduler;
  if (sc?.yield) await sc.yield();
};

// ---- the entries the screens ask for ----

/** An entry from the words a screen sends: `kind` story (ref a route or a pub: address), page (a web address), file (an address in Files). */
export function requestEntry(kind: string, ref: string, title: string): Entry | undefined {
  switch (kind) {
    case 'story': { const r = ref.startsWith('pub:') ? ref.slice(4) : ref; return r === '' ? undefined : paperStory(r, title); }
    case 'page': return ref === '' ? undefined : webPage(ref, title);
    case 'file': return ref.startsWith('file:') ? filesAudio(ref, title === '' ? decodeURIComponent(ref.split('/').pop() ?? '').replace(/\.[^.]+$/, '') : title) : undefined;
    default: return undefined;
  }
}

/** Words for a page's own text: paragraphs split on blank lines, the headline read first. */
export function listenWords(title: string, text: string): { title: string; paragraphs: string[] } | undefined {
  const ps = text.split(/\n\s*\n/).map((p) => p.replace(/\s+/g, ' ').trim()).filter((p) => p !== '');
  if (ps.length === 0) return undefined;
  const head = title.trim();
  if (head !== '' && ps[0].toLowerCase() !== head.toLowerCase()) ps.unshift(head);
  return { title: head, paragraphs: ps };
}

// ---- the sources and sends ----

export type Ack = { ok: boolean; message: string };
const ACK: Ack = { ok: true, message: '' };
/** Exactly queue-types.contract's QueueInfo: a field the shape does not name empties the page. */
export type QueueAnswer = { current: string; currentKind: string; audio: boolean; playing: boolean; clip: boolean; keepTitle: string; canOpenEpisode: boolean; elapsed: number; duration: number; elapsedLabel: string; remainingLabel: string; fraction: number; message: string; notice: string; writing: boolean; dry: boolean; webUrl: string; webHost: string };
const host = (u: string): string => { try { return new URL(u).host; } catch { return u; } };

/** What the screens read (queue.contract's QueueInfo): the runtime's view and the page the bar opened. */
export async function queueView(storage: Store | undefined, native: NativeReader | undefined): Promise<QueueAnswer> {
  const r = await getRadio(storage, native, false);
  const v = r.rt.view();
  return { current: v.current, currentKind: v.currentKind, audio: v.audio, playing: v.audio && v.playing, clip: v.clip, keepTitle: v.keepTitle, canOpenEpisode: v.canOpenEpisode, elapsed: v.elapsed, duration: v.duration, elapsedLabel: v.elapsedLabel, remainingLabel: v.remainingLabel, fraction: v.fraction,
    message: v.message, notice: v.notice, writing: v.writing, dry: v.dry, webUrl: r.web.url, webHost: r.web.url === '' ? '' : host(r.web.url) };
}

/** What the screens ask again on, and WHICH WORK IS DUE. The pulse only reads (the facts; the deck's state is the contract's `ds`, handed in) and says: it is asked again every second and an ask is let go when the next one comes
 *  (Exact: js/src/fetch.js "the answer was let go before this fetch"; host/web-js/ts-fetch.js `letGo`), so a pulse that began work with several native calls or a fetch lost it half done.
 *  The work is in mutations, which stay until they are done: `radioHold` (the sheet's restore from its file, what the voice handed over, the radio's start, the deck's journal and held list) and `radioWrite` (the desk's ask, the sheet's post).
 *  docs/reference/exact/answers-let-go.md; radio-pulse-rule.test.ts fails when this body fetches or pumps. */
export type Pulse = { tick: number; write: boolean; hold: boolean; live: boolean; beat: number; slow: boolean };
/** `deck`: the deck's state as the contract read it (spine.contract `ds`, the package source `deckState`, read once per radio tick): the pulse makes no deck call of its own. */
export async function radioPulse(storage: Store | undefined, native: NativeReader | undefined, deck?: unknown): Promise<Pulse> {
  const r = await getRadio(storage, native);
  const st = contractDeck(deck);
  if (st) r.rt.seeDeck(st);                                                // the contract's placeholder (not read yet) leaves what was last known
  // The generation this RADIO was made under, not the one the pulse happens to read: a pulse that begins after the sign-out began reads the new generation while the radio still holds the old account's sheet, and
  // would carry a stamp native takes. A sign-out under way keeps the radio's own; one that was given up (or never wiped this radio) leaves it the same account's, so it takes the current one.
  if (!sealingBlocked() && !signOutUncertain() && r.gen !== accountGeneration()) r.gen = accountGeneration();
  const gen = r.gen;
  if (!r.ready) r.ready = await firstRunOver(storage as never);          // native's `ready` is only what a page's ask of `ob` told it: the start must not wait for that (set after getRadio: reading the facts sets it from native)
  if (startDue(r) && r.readySince === 0) r.readySince = clock.ms;
  const restored = r.rt.restored;                                          // until a hold has read the sheet, nothing is pushed or written from the empty one
  // the turn route's queue: pushed to native, which sends it with every turn (one call; the next pulse sets it again if this one is let go)
  const later = nativeLater();
  // Sent when it changed (the sheet's version, its length, what is on air): the whole sheet crossed the bridge every second, paused, idle or locked, for state that had not moved (review B2-1). A failed or let-go call leaves the key, so the next pulse sends again.
  const candidates = r.front.stories.slice(0, 40);                       // the paper's list the desk may add from (the turn's `candidates`: native sends it with every turn)
  const setKey = `${gen}|${r.rt.store.sheet.version}|${r.rt.store.sheet.entries.length}|${r.rt.store.onAir()?.id ?? ''}|${candidates.length}:${candidates.map((c) => c.route).join(',')}`;
  const responds = respondsOf(r.rt.store.onAir());
  if (later && restored && setKey !== r.setKey) { try { const ans = await later({ op: 'radioSet', gen, snapshot: turnQueueJSON(r.rt, 0), facts: voiceFacts(r.rt.store.sheet.entries), respondsRoute: responds?.route ?? '', respondsTitle: responds?.title ?? '', candidates }); if ((ans as { ok?: unknown } | null)?.ok !== false) r.setKey = setKey; else console.log(`radio: native refused the radioSet push (stamped ${gen}, the account moved on); it is sent again with the next pulse's stamp`); } catch { /* the next pulse sets it */ } }       // a push native refused (ok: false: stamped for an account that moved on) is not recorded as sent: the next pulse sends it again with that pulse's stamp
  const posting = r.rt.store.canPost;                                     // pending ops and not backing off (the route's Retry-After, or 30 s doubling): the pulse does not call for a post it would not make
  // `live`: something is sounding and the app is in front (native's `foreground`: with the screen off or the app away the audio plays on and nothing here asks again), so the position is moving and the screens that show it are asked again each second (spine.contract `posKey`). `beat` moves once per pulse while it is, and not otherwise:
  // the key is the beat, so the screens are asked after THIS pulse's read of the deck (they never read a position older than the pulse that said so), and a pause or a hidden page leaves the key still.
  const live = r.rt.view().playing && r.rt.inFront();
  // The beat moves once per device SECOND, not once per pulse: the pulse is asked again whenever native touches the radio note (a state change), and each beat asks the three screens and
  // what reads them again. The position label changes once a second at the most.
  const second = Math.floor(clock.ms / 1000);
  if (live && (second !== r.beatSecond || clock.ms <= 0)) { r.beat += 1; r.beatSecond = second; }
  r.pulses += 1;
  const awaitingWords = r.rt.wantsWords();
  const writeDue = restored && !flagBusy(r, r.writeFlag) && (r.writer.wouldAsk() || posting || awaitingWords);
  if (writeDue) r.writeFlag = { phase: 'due', at: flagNow(r) };            // marked by the pulse that calls for it: a pulse computed before the mutation's body starts cannot call for a second send (the gate closing cancels the first's pending `after`)
  const holdDue = !flagBusy(r, r.holdFlag, HOLD_DUE_STALE_MS, HOLD_RUNNING_STALE_MS) && (!restored || r.rt.needsWork() || r.inbox.length > 0 || startDue(r));
  if (holdDue) r.holdFlag = { phase: 'due', at: flagNow(r) };              // marked like the write: a second hold sent before the first's body starts would replace it (the first let go mid-restore)
  const out = { tick: r.tick, write: writeDue, live, beat: r.beat, hold: holdDue, slow: false };
  out.slow = !out.live && !out.write && !out.hold && !r.writer.inFlight;          // nothing sounds in front, nothing is due: the clock asks again in five seconds, not one (what native has to say wakes it at once: the deck's seq and the radio note are keys of the pulse)
  const state = `restored=${restored} ready=${r.ready} started=${r.started} onAir=${r.rt.store.onAir() ? 'yes' : 'no'} write=${out.write} hold=${out.hold} ahead=${r.rt.store.ahead().length} inFlight=${r.writer.inFlight} lost=${r.rt.store.lostPending}`;
  if (state !== lastPulse) { lastPulse = state; console.log(`radio: pulse ${state}`); }
  return out;
}

/** The radio starts by itself when Autoradio opens (onboarding complete, no other audio on the phone), once a launch: it waits a moment for the front page it speaks from. */
const startDue = (r: Radio): boolean => !r.started && r.ready && r.rt.restored && !r.rt.store.onAir();

/** The work the pulse says is due: the voice's messages, the radio's start, the pump (starts, the journal, the held list). A mutation: it is not let go when the next pulse is asked. */
let lastPulse = '';
/** The flags are the RADIO's (a sign-out replaces the radio and its flags go with it), and they go stale: a body whose continuation never runs (an answer let go) must not stop the radio for good. `running` after 260 s (just over the
 *  240 s at which a writer abandons its ask), `due` (called for, the mutation not started: the gate closed, the send never came) after 5 s. Device time; with none known, one pulse counts as a second. */
export const WORK_STALE_MS = 260_000, DUE_STALE_MS = 5_000;
/** A hold whose answer was let go must not stop the radio for the write's 260 s: it starts audio in the
 *  background and normally finishes in well under a second, so a running flag older than this is stale. */
export const HOLD_RUNNING_STALE_MS = 8_000;
/** A hold called for and not begun goes stale sooner than a write: the pulse that marked it may itself be let go (launch re-asks it often), and the restore and the start wait on the hold. */
export const HOLD_DUE_STALE_MS = 2_000;
const flagNow = (r: Radio): number => (clock.ms > 0 ? clock.ms : r.pulses * 1000);
const flagBusy = (r: Radio, f: Flag | undefined, dueStale = DUE_STALE_MS, runningStale = WORK_STALE_MS): boolean => f !== undefined && flagNow(r) - f.at < (f.phase === 'running' ? runningStale : dueStale);
// one radioHold at a time: a second send would take the first's place, and the first would be let go
export async function radioHold(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  const r = await getRadio(storage, native);
  const mine: Flag = { phase: 'running', at: flagNow(r) }; r.holdFlag = mine;
  try { return await holdInner(storage, native); } finally { if (r.holdFlag === mine) r.holdFlag = undefined; }
}
const EMPTY_PULSE: Pulse = { tick: 0, write: false, hold: false, live: false, beat: 0, slow: false };
async function holdInner(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  const r = await getRadio(storage, native);
  // A sign-out (wipeRadio) can run while this answer is yielded: its radio is discarded, and
  // nothing more may start on it (a deck play after the wipe would speak the old account's entry).
  const gone = (): boolean => radio !== r;
  if (!(await restoreOf(r, storage))) return { tick: r.tick, write: false, hold: false, live: false, beat: 0, slow: false };       // not read yet: the pulse says hold again
  if (gone()) return EMPTY_PULSE;

  await r.rt.pump();
  if (gone()) return EMPTY_PULSE;

  await turnEnd();
  if (gone()) return EMPTY_PULSE;
  if (!r.ready) r.ready = await firstRunOver(storage as never);
  if (gone()) return EMPTY_PULSE;

  await turnEnd();
  if (gone()) return EMPTY_PULSE;
  while (r.inbox.length > 0) { if (gone()) return EMPTY_PULSE; await handle(r, r.inbox.shift() as string); }

  await turnEnd();
  if (gone()) return EMPTY_PULSE;
  // Replay only after the turn's own messages (a `turnEnded` advances an entry that ended while the
  // listener was speaking): otherwise a finished entry is sent to the deck again from its start.
  await r.rt.replayOnAir();
  if (gone()) return EMPTY_PULSE;

  await turnEnd();
  if (gone()) return EMPTY_PULSE;
  if (startDue(r)) {
    if (r.readySince === 0) r.readySince = clock.ms;
    // The front page it speaks from is fetched first and the radio opens in this same answer once it is held, or once 2 s have passed since the first run was over (the opening then says it has no front page yet).
    // (It used to fetch in one answer and open in the next: with nothing else moving the tick, no next answer was asked, and a plain launch never started.)
    if (r.front.lines.length === 0) { try { await r.fetchFront(); } catch { /* none yet */ } }
    if (gone()) return EMPTY_PULSE;

    await turnEnd();
    if (gone()) return EMPTY_PULSE;
    if (r.front.lines.length > 0 || clock.ms - r.readySince > 2000) await openRadioOf(r);
    if (gone()) return EMPTY_PULSE;

    await turnEnd();
    if (gone()) return EMPTY_PULSE;
  }

  await turnEnd();
  if (gone()) return EMPTY_PULSE;
  await r.rt.peek();

  console.log(`radio: hold: ${r.rt.store.ahead().length} ahead on the sheet, the deck holds ${r.rt.deckLine().held}, playing ${r.rt.deckLine().item || 'nothing'}`);
  return { tick: r.tick, write: false, hold: false, live: false, beat: 0, slow: false };
}

/** The desk's ask and the sheet's post, in one answer that waits for both. */
// one radioWrite at a time, like radioHold: a mutation sent again REPLACES the first, which is let go, and the first is the one waiting for the desk's reply (its stretch then fails after the first entry it places)
export async function radioWrite(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  const r = await getRadio(storage, native);
  const mine: Flag = { phase: 'running', at: flagNow(r) }; r.writeFlag = mine;
  try { return await writeInner(storage, native); } finally { if (r.writeFlag === mine) r.writeFlag = undefined; }
}
async function writeInner(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  lap('radioWrite: start');
  const r = await workRadio(storage, native);
  lap('radioWrite: radio ready');
  await r.rt.prefetchOnAir();                                      // the words the deck's next start needs, fetched here (not in a pump)
  r.rt.store.flush();
  const asked = r.writer.checkAhead(), mine = r.writer.lastId;     // the ask THIS answer started: it waits for that one and abandons only that one
  const waits: Promise<unknown>[] = [];
  if (asked) waits.push(r.writer.settleWithin(2 * ASK_DEADLINE_MS, mine));
  if (r.rt.store.flushTask) waits.push(r.rt.store.flushTask);
  await Promise.all(waits.map((t) => t.catch(() => undefined)));
  return { tick: r.tick, write: false, hold: false, live: false, beat: 0, slow: false };
}


/** One message from native (a verb from the voice turn or a handler that has not moved): `{verb, ...}` as text. */
async function handle(r: Radio, text: string): Promise<void> {
  let m: Record<string, unknown>;
  try { m = obj(JSON.parse(text)); } catch { return; }
  switch (m.verb) {
    case 'turnOps': applyTurnOps(r.rt, str(m.json)); break;
    case 'turnEnded': await r.rt.turnEnded(); break;
    case 'stopAll': {
      await r.rt.signOut();
      const later = nativeLater();                                            // native waits for this before the key goes (ChoirApp.signOut): the wait is for the answer, not a guess
      if (later) { try { await later({ op: 'radioSet', signedOut: true }); } catch { /* native stops waiting at its deadline */ } }
      break;
    }
    case 'place': {
      const e = (() => { try { return decodeEntry(m.entry); } catch { return undefined; } })();
      if (!e) break;
      r.writer.listenerActed();                                               // a podcast or a clip added from the native side is the listener's act like every verb of the screens
      const w = obj(m.words), words = Array.isArray(w.paragraphs) ? { title: str(w.title), paragraphs: w.paragraphs.filter((p): p is string => typeof p === 'string') } : undefined;
      if (m.mode === 'now') r.rt.play(e, words); else if (m.mode === 'next') r.rt.playNext(e, words); else r.rt.queue(e, words);
      break;
    }
    default: break;
  }
}

export async function queueAdd(storage: Store | undefined, native: NativeReader | undefined, mode: string, kind: string, ref: string, title: string, local: string): Promise<QueueAnswer> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  const e = requestEntry(kind, ref, title);
  if (e) { if (mode === 'now') r.rt.play(e); else if (mode === 'next') r.rt.playNext(e); else r.rt.queue(e); }
  await r.rt.pump();
  return queueView(storage, native);
}

export async function queueAct(storage: Store | undefined, native: NativeReader | undefined, op: string, arg: string): Promise<QueueAnswer> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  const rt = r.rt;
  switch (op) {
    case 'toggle': await radioPlayPauseOf(r); break;
    case 'back15': await rt.seekBy(-15); break;
    case 'fwd15': await rt.seekBy(15); break;
    case 'seek': { const f = Number(arg), d = rt.view().duration; if (Number.isFinite(f) && d > 0) await rt.seekTo(Math.min(Math.max(f, 0), 1) * d); break; }
    case 'remove': { const i = Number(arg); if (Number.isInteger(i)) rt.remove(i); break; }
    case 'playAt': { const i = Number(arg); if (Number.isInteger(i)) rt.playUpcoming(i); break; }
    case 'skip': rt.skip(); break;
    case 'clear': rt.clear(); break;
    case 'keep': await rt.keepListening(); break;
    case 'openEpisode': await openEpisodeOf(r); break;
    case 'openWeb': r.web.url = arg; break;
    case 'closeWeb': r.web.url = ''; break;
    default: break;
  }
  await rt.pump();
  return queueView(storage, native);
}

/** "Open episode": the page of the episode on air, at the clip's time. Opening a page is the shell's (native `openPage`); shared code names the address. */
async function openEpisodeOf(r: Radio): Promise<void> {
  const air = r.rt.store.onAir(), spec = air ? audioSpec(air) : undefined, k = spec?.episode_id ? parseEpisodeKey(spec.episode_id) : null;
  const later = nativeLater();
  if (!spec || !k || !later) return;
  const address = episodeAddress(k.feed, k.id, { enclosure: spec.url, start: spec.start ?? null, title: spec.title, show: spec.show });
  try { await later({ op: 'openPage', address, title: 'Episode' }); } catch { /* the page did not open */ }
}

async function radioPlayPauseOf(r: Radio): Promise<void> {
  const how = await r.rt.toggle();
  if (how === 'nothing') await openRadioOf(r);
}

/** The radio opens: the phone's opening plays at once, from the front titles it holds, and the desk is asked for more. */
async function openRadioOf(r: Radio): Promise<void> {
  if (r.front.lines.length === 0) { try { await r.fetchFront(); } catch { /* the opening says it has no front page yet */ } }
  if (radio !== r) return;                                   // a sign-out ran while the front page was fetched
  const lines = r.front.lines;
  r.rt.director.open(phoneOpening(lines, clockNow(), -clockNow().getTimezoneOffset(), mintId('p-')));
  r.writer.openAhead(false);
  r.started = true;
  await r.rt.pump();
}

/** Play or queue one audio address (an episode, a clip's episode): the podcast page's Play and Add to queue. The audio is the deck's; the entry and the order are the sheet's. */
export async function radioPlayAudio(storage: Store | undefined, native: NativeReader | undefined, op: string, id: string, url: string, title: string, show: string, start: number): Promise<{ status: string; message: string }> {
  if (op === 'keep') {                                  // "Keep listening" to a clip: the clip no longer ends, the episode goes on from where it is (the deck's act, the player's own rule)
    const k = await workRadio(storage, native); k.writer.listenerActed();
    await k.rt.keepEnd(id);
    return { status: 'ok', message: '' };
  }
  if (url === '') return { status: 'error', message: 'This episode has no audio to play.' };
  const r = await workRadio(storage, native); r.writer.listenerActed();
  const e = audioEntry({ url, title: title === '' ? 'Episode' : title, show, ...(start > 0 ? { start } : {}), ...(id !== '' ? { episode_id: id } : {}) });
  if (op === 'queue') r.rt.queue(e); else r.rt.play(e);
  await r.rt.pump();
  return { status: 'ok', message: '' };
}

/** Play or queue one entry that shared code made (a clip row's Play and Queue: clip-play.ts). */
export async function radioPlayEntry(storage: Store | undefined, native: NativeReader | undefined, op: string, e: Entry): Promise<void> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  if (op === 'queue') r.rt.queue(e); else r.rt.play(e);
  await r.rt.pump();
}

/** Read an imported document aloud (or queue it): the words are the document's own, one line per block from block `at`; the entry is the sheet's book entry. */
export async function radioHandWords(storage: Store | undefined, native: NativeReader | undefined, op: string, id: string, title: string, at: number, text: string): Promise<{ ok: boolean; message: string }> {
  const paragraphs = text.split('\n').filter((p) => p !== '');
  if (id === '' || paragraphs.length === 0) return { ok: false, message: 'This document has nothing to read.' };
  const r = await workRadio(storage, native); r.writer.listenerActed();
  const e = bookEntry(id, title, at), words = { title, paragraphs };
  if (op === 'queue') { r.rt.queue(e, words); await r.rt.pump(); return { ok: true, message: 'Added to the queue.' }; }
  r.rt.play(e, words);
  await r.rt.pump();
  return { ok: true, message: '' };
}

export async function radioPlayPause(storage: Store | undefined, native: NativeReader | undefined): Promise<Ack> { const r = await workRadio(storage, native); r.writer.listenerActed(); await radioPlayPauseOf(r); return ACK; }
export async function radioStart(storage: Store | undefined, native: NativeReader | undefined): Promise<Ack> { const r = await workRadio(storage, native); r.writer.listenerActed(); await openRadioOf(r); return ACK; }
export async function radioPlayStory(storage: Store | undefined, native: NativeReader | undefined, route: string, title: string): Promise<Ack> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  if (route !== '') r.rt.play(paperStory(route, title));
  await turnEnd();
  await r.rt.pump();
  const air = r.rt.store.onAir();
  console.log(`radio: play story ${route === '' ? '(no route)' : route.slice(0, 60)}: on air ${air ? air.id + ' ' + air.state : 'nothing'}, ahead ${r.rt.store.ahead().length}, director playing ${r.rt.director.playingId ?? 'none'}, message "${r.rt.director.message}"`);
  return ACK;
}
export async function radioListen(storage: Store | undefined, native: NativeReader | undefined, address: string, title: string, text: string): Promise<Ack> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  const w = listenWords(title, text);
  if (w) { const e = readAloud(address, title); e.est_seconds = estimateSeconds(w.paragraphs.join(' ')); r.rt.play(e, w); }
  await r.rt.pump(); return ACK;
}
export async function radioMove(storage: Store | undefined, native: NativeReader | undefined, move: string): Promise<Ack> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  if (move === 'previous') await r.rt.previous(); else r.rt.skip();
  await r.rt.pump(); return ACK;
}

/** The gestures on the Autoradio page: reorder (one `move`), remove (a swipe), play (a tap on a row), replay (a tap on Heard). */
export async function radioPageAct(storage: Store | undefined, native: NativeReader | undefined, op: string, arg: string, to: string): Promise<Ack> {
  const r = await workRadio(storage, native); r.writer.listenerActed();
  const g = op === 'reorder' ? { kind: 'reorder' as const, id: arg, before: to === '' ? undefined : to } : op === 'remove' ? { kind: 'remove' as const, id: arg } : op === 'play' ? { kind: 'playNow' as const, id: arg } : op === 'replay' ? { kind: 'replay' as const, id: arg } : undefined;
  if (g) {
    const a = perform(r.rt.store.sheet, g, r.rt.store.mintOpId());
    if (a.kind === 'op') r.rt.store.apply(a.op);
    else if (a.kind === 'remove') r.rt.store.remove(a.id);
    else if (a.kind === 'playUpcoming') r.rt.playUpcoming(a.index);
    else if (a.kind === 'placeNow') r.rt.play(a.entry);
  }
  await r.rt.pump(); return ACK;
}

/** What the bar shows of what plays, for both phones: a pure function of the deck's state and the on-air entry (radio-follow.ts `runOf`), both arguments (the contract's `ds` and `rair`). It reads no runtime state. */
export async function barRun(_storage: Store | undefined, _native: NativeReader | undefined, deck: unknown, air: unknown): Promise<Record<string, unknown>> {
  return runOf(deck, air);
}

/** The sheet as the page's facts carry it. The facts are asked again every second while the radio sounds (spine.contract `posKey`), and the sheet can be a megabyte: it is written out when it moved
 *  (a new object from the reducer, or a version or a length that changed: the store's own splice keeps the object) and the same text is handed over while it did not (radio-position.test.ts P8). */
let sheetSeen: { sheet: Sheet; version: number; length: number; text: string } | undefined;
function sheetText(sheet: Sheet): string {
  if (sheetSeen && sheetSeen.sheet === sheet && sheetSeen.version === sheet.version && sheetSeen.length === sheet.entries.length) return sheetSeen.text;
  const text = JSON.stringify({ ...sheet, applied_ops: [] });
  sheetSeen = { sheet, version: sheet.version, length: sheet.entries.length, text };
  return text;
}

/** The entry on air as the contract needs it for the sentence follow (members-radio.contract `rair`): its id, its page's address, and what was last handed to the deck under that id. Asked on the pulse and the beat. */
export async function radioAir(storage: Store | undefined, native: NativeReader | undefined): Promise<{ id: string; address: string; handedId: string; handedText: string; title: string; audio: boolean }> {
  const r = await getRadio(storage, native, false);
  const e = r.rt.store.onAir(), v = r.rt.view();
  const h = e ? r.rt.handed(e.id) : { id: '', text: '' };
  // the title and the audio flag are the entry's own, read in the same step as its id and its handed text: the bar's run (`runOf`) is a function of this and the deck's state, nothing else
  return { id: e?.id ?? '', address: airAddressOf(e), handedId: h.id, handedText: h.text, title: e ? v.current : '', audio: e ? v.audio : false };
}

export async function radioPageFacts(storage: Store | undefined, native: NativeReader | undefined, deck: unknown, air: unknown): Promise<RadioFacts> {
  const r = await getRadio(storage, native, false);
  const v = r.rt.view();
  const now = nowOf(deck, air);
  const prog = r.rt.programme(), route = prog.route;
  const airEntry = r.rt.store.onAir(), takeRef = airEntry?.kind === 'take' && airEntry.source?.ref.startsWith('take:') ? airEntry.source.ref.slice(5) : '';
  const take = takeRef !== '' ? r.takeList.find((t) => t.takeID === takeRef) : undefined;
  const published = route !== '' ? r.front.published[route] ?? 0 : 0;
  const versions = Object.entries(r.front.versions);
  return {
    at: clock.ms / 1000, sheet: sheetText(r.rt.store.sheet), playing: v.playing, sentence: v.audio ? '' : now.sentence, progress: v.audio ? v.fraction : now.progress,
    ready: true, onAir: v.onAir, waiting: v.waiting, remoteAudio: v.audio, remoteElapsed: v.elapsed, spokenElapsed: v.spokenElapsed, remoteDuration: v.duration, wants: r.rt.director.wants, message: v.message, notice: v.notice, writing: v.writing, dry: v.dry,
    airTitle: prog.title, airRoute: route, publishedAt: published, versionRoutes: versions.map(([k]) => k), versionNumbers: versions.map(([, n]) => n),
    stale: r.rt.store.ahead().filter((e) => r.rt.store.isStale(e)).map((e) => e.id), takeTranscript: take?.transcript ?? '', takeRecord: take?.record?.summary ?? '',
  };
}

export { audioEntry, bookEntry };
