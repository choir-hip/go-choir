// The radio, as the app's sources and sends: the one place shared code assembles the runtime (radio-runtime.ts), its writer (radio-writer.ts) and the real deck (deck-native.ts), and answers the
// names the contracts use for it (queueView, queueAdd, queueAct, radioPageFacts, barRun, radioPlayPause, radioPlayStory, radioStart, radioListen, radioMove, radioPulse).
// What stays native and why: the deck (the audio), a few facts only the audio code knows (`radioFacts`: a turn is open, what is left of the spoken entry, the place in it), the book text (the
// shelf is native until it moves) and the voice turn's queue snapshot and server ops (`radioSet`, and the inbox in `radioFacts`): the turn engine is native and Exact has no call from native into shared code,
// so shared code PUSHES the snapshot and native hands back the server's `queue_ops` text, which the pump applies (the retrieval lane's pattern).
import { authedText, authedToFile, getJSON, nativeLater, accountGeneration, signOutBlocked, signOutUncertain, type NativeReader } from './api.ts';
import { nativeDeck } from './deck-native.ts';
import { RadioRuntime, NO_FACTS, type NativeFacts, type Resolvers } from './radio-runtime.ts';
import { Writer, ASK_DEADLINE_MS, phoneOpening, PROGRAMME_PATH } from './radio-writer.ts';
import { applyTurnOps, resetTurnLog, turnQueueJSON } from './voice-queue.ts';
import { routePoster, signedInRequester } from './runsheet-store.ts';
import { authedJSON } from './api.ts';
import { audioSpec, audio as audioEntry, book as bookEntry, filesAudio, mintId, paperStory, readAloud, webPage, estimateSeconds, heldStory, forgetLongURLs } from './runsheet-entries.ts';
import { offlineStories } from './content-run.ts';
import { loadReader } from './reader.ts';
import { sentenceTexts } from './reader-parse.ts';
import { fileOpen } from './files.ts';
import { lapYield, lap } from './lap.ts';
import { firstRunOver } from './onboarding.ts';
import { perform } from './radio-model.ts';
import { voiceFacts, respondsOf } from './voice-facts.ts';
import { htmlToText } from './mail-model.ts';
import { parseList, FRONT_LIMIT, type Story } from './paper-model.ts';
import { listing } from './paper.ts';
import { parseEpisodeKey, episodeAddress } from './podcast-model.ts';
import { loadKept, takesOf, type Kept, type Take } from './takes.ts';
import { seamNet } from './takes.ts';
import { clock, now as clockNow } from './clock.ts';
import { decodeEntry, type Entry, type Sheet } from './runsheet-model.ts';
import type { Storage as RunStorage } from './runsheet-store.ts';
import type { RadioFacts } from './radio-page.ts';

type Store = { fs: { readFile(p: string): Promise<Uint8Array>; atomicWriteFile(p: string, b: Uint8Array): Promise<void>; rm(p: string): Promise<void> } };
export const SHEET_FILE = 'app:/data/runsheet.json';

const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

/** A mutation in flight (or called for): `due` is a pulse that said the work is due and the mutation not started yet, `running` the mutation's body. Each goes stale (WORK_STALE_MS) so a continuation that never ends cannot stop the radio. */
type Flag = { phase: 'due' | 'running'; at: number };
type Radio = { writeFlag?: Flag; holdFlag?: Flag; pulses: number; beatSecond: number; gen: number; setKey: string; rt: RadioRuntime; writer: Writer; storage: Store | undefined; web: { url: string }; inbox: string[]; front: { at: number; published: Record<string, number>; versions: Record<string, number>; lines: { title: string; dek: string }[] }; tick: number; beat: number; takeList: Take[]; started: boolean; ready: boolean; readySince: number; fetchFront(): Promise<unknown>; native?: NativeReader };
let radio: Radio | undefined;
let restored: Promise<void> | undefined;

/** Fresh state (tests; a sign-out wipes through `wipeRadio`). */
export function resetRadio(): void { radio = undefined; restored = undefined; }
export async function wipeRadio(storage: Store | undefined): Promise<void> {
  const r = radio; radio = undefined; restored = undefined;
  resetTurnLog();                                                                            // the lists sent with the old account's turns and its turn ids
  forgetLongURLs();                                                                          // the long web addresses of this account's entries (runsheet-entries.ts)
  try { if (r) { r.writer.stop(); await r.rt.signOut(); } }
  finally {
    // The old account's sheet file goes whatever the wipe did (a throw above must not leave it for the next radio to restore); a removal that fails is remembered and tried again before any radio restores.
    if (storage) { try { await storage.fs.rm(SHEET_FILE); sheetFileStuck = false; } catch (e) { if (!/ENOENT|no such file|not found/i.test(String((e as Error)?.message ?? e))) sheetFileStuck = true; } }
  }
}
/** The sheet file could not be removed by the last wipe: the next radio removes it again and starts EMPTY (a sheet that outlived a sign-out is not the next account's). */
let sheetFileStuck = false;

function runStorage(s: Store | undefined): RunStorage | undefined {
  if (!s) return undefined;
  return {
    read: async () => { try { return new TextDecoder().decode(await s.fs.readFile(SHEET_FILE)); } catch { return undefined; } },
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

function resolvers(storage: Store | undefined): Partial<Resolvers> {
  const words = async (address: string): Promise<{ title: string; paragraphs: string[] } | undefined> => {
    const d = await loadReader(address);
    const ps = d.blocks.map((b) => sentenceTexts(b).join(' ').trim()).filter((p) => p !== '');
    return ps.length ? { title: d.title, paragraphs: ps } : undefined;
  };
  return {
    story: (route) => words('pub:' + route),
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
  const state: Radio = { pulses: 0, beatSecond: -1, gen: accountGeneration(), setKey: '', rt: undefined as never, writer: undefined as never, storage, web: { url: '' }, inbox: [], takeList: [], front: { at: 0, published: {}, versions: {}, lines: [] }, tick: 0, beat: 0, started: false, ready: false, readySince: 0, fetchFront: undefined as never, native };
  const rt = new RadioRuntime({
    deck: nativeDeck(), storage: runStorage(storage), poster: routePoster(signedInRequester(authedJSON)), generation: accountGeneration, resolvers: resolvers(storage),
    native: { facts: () => readFacts(state) },
    onChange: () => { state.tick += 1; },
  });
  state.rt = rt;
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
    state.front = { at: now / 1000, published: {}, versions: {}, lines: stories.slice(0, 10).map((s) => ({ title: s.title, dek: s.dek })) };
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
      await Promise.race([Promise.all(routes.map(async (r) => { try { const d = await loadReader('pub:' + r); lap(`texts: ${r.slice(-20)} read (${d.blocks.length} blocks)`); { const ps: string[] = []; let n = 0; for (const b of d.blocks) { const t = sentenceTexts(b).join(' '); ps.push(t); n += t.length; if (n > 24 * 1024) break; } out[r] = ps; } } catch { /* goes without text */ } })), new Promise((res) => setTimeout(res, 3000))]);
      return out;
    },
    takes: async () => {
      let takes: Take[] = []; let kept: Kept = await loadKept(storage as never);
      try { takes = takesOf((await seamNet('GET', '/api/platform/takes', { query: { limit: 50 } })).json); } catch { /* none offered */ }
      state.takeList = takes;                                      // what the page shows of a take on air (its transcript and its record)
      return { takes, kept };
    },
    edition: async () => '',
    // The offline story fill: the stories the phone holds whole (content-run.ts), as the Swift LocalStories gave them; the entry builder is runsheet-entries.ts.
    offlineFill: async () => {
      const entries = rt.store.sheet.entries;
      const told = entries.filter((e) => e.state !== 'ahead').map((e) => e.title), held = new Set(entries.map((e) => e.id));
      return (await offlineStories(storage as never, { told, held })).map((l) => heldStory(l.id, l.title, l.text, l.address));
    },
    post: async (body) => {
      try { return await authedText(PROGRAMME_PATH, { method: 'POST', body }); }
      catch (e) { const s = (e as { status?: number }).status; throw Object.assign(e as Error, { kind: (e as Error).name === 'SignedOutError' ? 'signedOut' : s === undefined ? 'transport' : 'other' }); }
    },
    now: clockNow, tz: () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', requestId: () => 'radio-' + mintId('').slice(0, 12), generation: accountGeneration,
  });
  // The writer is asked from the pulse (radioPulse awaits it): a fetch started from a hook inside some other answer, and carried on after that answer ended, is never run (Exact: "fetch() called outside an answer").
  return state;
}

/** The radio of this process: made once, its sheet restored from its own file (nothing plays). */
/** Ends the turn (Exact gives one about 100 ms of CPU; Bun has no scheduler). */
const yieldTurn = (label: string): Promise<void> => lapYield(`bridge: ${label}`);

/** `read` false: the screens' reads (the queue, the bar's run, the page's facts) need what the pulse just read, not their own native call for it. Mutations and the pulse read (the device's time first). */
export async function getRadio(storage: Store | undefined, native: NativeReader | undefined, read = true): Promise<Radio> {
  if (!radio) radio = await makeRadio(storage, native);
  const r = radio;                                          // held locally: a sign-out (`wipeRadio`) clears the module's variable at any await below
  if (read) { const f = await readFacts(r); if (f !== NO_FACTS) r.rt.useFacts(f); }      // the device's time first: nothing below may run on a stale clock; the runtime keeps what was read (the pulse's peek does not read it again)
  if (radio !== r) {                                          // signed out meanwhile: this radio is the old account's. What native handed over in that read was drained from native: it goes to the radio that replaced this one, not into the one discarded (review B1-10)
    const next = await getRadio(storage, native, read); next.inbox.unshift(...r.inbox); r.inbox = []; return next;
  }
  if (!restored) {
    if (sheetFileStuck) { try { await storage?.fs.rm(SHEET_FILE); sheetFileStuck = false; } catch { /* still there: not read either */ } restored = Promise.resolve(); }
    else restored = r.rt.restore();
  }
  const done = restored;
  await done;
  return radio === r ? r : getRadio(storage, native, read);
}

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

/** What the screens ask again on, and WHICH WORK IS DUE. The pulse only reads (the deck's state, the facts) and says: it is asked again every second and an ask is let go when the next one comes
 *  (Exact: js/src/fetch.js "the answer was let go before this fetch"; host/web-js/ts-fetch.js `letGo`), so a pulse that began work with several native calls or a fetch lost it half done.
 *  The work is in mutations, which stay until they are done: `radioHold` (what the voice handed over, the radio's start, the deck's journal and held list) and `radioWrite` (the desk's ask, the sheet's post).
 *  docs/reference/exact/answers-let-go.md; radio-pulse-rule.test.ts fails when this body fetches or pumps. */
export type Pulse = { tick: number; write: boolean; hold: boolean; live: boolean; beat: number; slow: boolean };
export async function radioPulse(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  const r = await getRadio(storage, native);
  // The generation this RADIO was made under, not the one the pulse happens to read: a pulse that begins after the sign-out began reads the new generation while the radio still holds the old account's sheet, and
  // would carry a stamp native takes. A sign-out under way keeps the radio's own; one that was given up (or never wiped this radio) leaves it the same account's, so it takes the current one.
  if (!signOutBlocked() && !signOutUncertain() && r.gen !== accountGeneration()) r.gen = accountGeneration();
  const gen = r.gen;
  await r.rt.peek(true);                                                   // the facts were just read by getRadio: the deck's state is the one more read
  if (!r.ready) r.ready = await firstRunOver(storage as never);          // native's `ready` is only what a page's ask of `ob` told it: the start must not wait for that (set after peek: reading the facts sets it from native)
  if (!r.started && r.ready && !r.rt.store.onAir() && r.readySince === 0) r.readySince = clock.ms;
  // the turn route's queue: pushed to native, which sends it with every turn (one call; the next pulse sets it again if this one is let go)
  const later = nativeLater();
  // Sent when it changed (the sheet's version, its length, what is on air): the whole sheet crossed the bridge every second, paused, idle or locked, for state that had not moved (review B2-1). A failed or let-go call leaves the key, so the next pulse sends again.
  const setKey = `${gen}|${r.rt.store.sheet.version}|${r.rt.store.sheet.entries.length}|${r.rt.store.onAir()?.id ?? ''}`;
  const responds = respondsOf(r.rt.store.onAir());
  if (later && setKey !== r.setKey) { try { const ans = await later({ op: 'radioSet', gen, snapshot: turnQueueJSON(r.rt, 0), facts: voiceFacts(r.rt.store.sheet.entries), respondsRoute: responds?.route ?? '', respondsTitle: responds?.title ?? '' }); if ((ans as { ok?: unknown } | null)?.ok !== false) r.setKey = setKey; else console.log(`radio: native refused the radioSet push (stamped ${gen}, the account moved on); it is sent again with the next pulse's stamp`); } catch { /* the next pulse sets it */ } }       // a push native refused (ok: false: stamped for an account that moved on) is not recorded as sent: the next pulse sends it again with that pulse's stamp
  const posting = r.rt.store.canPost;                                     // pending ops and not backing off (the route's Retry-After, or 30 s doubling): the pulse does not call for a post it would not make
  // `live`: something is sounding and the app is in front (native's `foreground`: with the screen off or the app away the audio plays on and nothing here asks again), so the position is moving and the screens that show it are asked again each second (spine.contract `posKey`). `beat` moves once per pulse while it is, and not otherwise:
  // the key is the beat, so the screens are asked after THIS pulse's read of the deck (they never read a position older than the pulse that said so), and a pause or a hidden page leaves the key still.
  const live = r.rt.view().playing && r.rt.inFront();
  // The beat moves once per device SECOND, not once per pulse: the pulse is asked again whenever native touches the radio note (the engine's 0.4 s progress timer, a state change), and each beat asks the three screens and
  // what reads them again. The position label changes once a second at the most.
  const second = Math.floor(clock.ms / 1000);
  if (live && (second !== r.beatSecond || clock.ms <= 0)) { r.beat += 1; r.beatSecond = second; }
  r.pulses += 1;
  const writeDue = !flagBusy(r, r.writeFlag) && (r.writer.wouldAsk() || posting);
  if (writeDue) r.writeFlag = { phase: 'due', at: flagNow(r) };            // marked by the pulse that calls for it: a pulse computed before the mutation's body starts cannot call for a second send (the gate closing cancels the first's pending `after`)
  const out = { tick: r.tick, write: writeDue, live, beat: r.beat, hold: !flagBusy(r, r.holdFlag) && (r.rt.needsWork() || r.inbox.length > 0 || startDue(r)), slow: false };
  out.slow = !out.live && !out.write && !out.hold && !r.writer.inFlight;          // nothing sounds in front, nothing is due: the clock asks again in five seconds, not one (what native has to say wakes it at once: the deck's seq and the radio note are keys of the pulse)
  const state = `ready=${r.ready} started=${r.started} onAir=${r.rt.store.onAir() ? 'yes' : 'no'} write=${out.write} hold=${out.hold} ahead=${r.rt.store.ahead().length} inFlight=${r.writer.inFlight} lost=${r.rt.store.lostPending}`;
  if (state !== lastPulse) { lastPulse = state; console.log(`radio: pulse ${state}`); }
  return out;
}

/** The radio starts by itself when Autoradio opens (onboarding complete, no other audio on the phone), once a launch: it waits a moment for the front page it speaks from. */
const startDue = (r: Radio): boolean => !r.started && r.ready && !r.rt.store.onAir();

/** The work the pulse says is due: the voice's messages, the radio's start, the pump (starts, the journal, the held list). A mutation: it is not let go when the next pulse is asked. */
let lastPulse = '';
/** The flags are the RADIO's (a sign-out replaces the radio and its flags go with it), and they go stale: a body whose continuation never runs (an answer let go) must not stop the radio for good. `running` after 260 s (just over the
 *  240 s at which a writer abandons its ask), `due` (called for, the mutation not started: the gate closed, the send never came) after 5 s. Device time; with none known, one pulse counts as a second. */
export const WORK_STALE_MS = 260_000, DUE_STALE_MS = 5_000;
const flagNow = (r: Radio): number => (clock.ms > 0 ? clock.ms : r.pulses * 1000);
const flagBusy = (r: Radio, f: Flag | undefined): boolean => f !== undefined && flagNow(r) - f.at < (f.phase === 'running' ? WORK_STALE_MS : DUE_STALE_MS);
// one radioHold at a time: a second send would take the first's place, and the first would be let go
export async function radioHold(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  const r = await getRadio(storage, native);
  const mine: Flag = { phase: 'running', at: flagNow(r) }; r.holdFlag = mine;
  try { return await holdInner(storage, native); } finally { if (r.holdFlag === mine) r.holdFlag = undefined; }
}
async function holdInner(storage: Store | undefined, native: NativeReader | undefined): Promise<Pulse> {
  const r = await getRadio(storage, native);
  await r.rt.pump();
  if (!r.ready) r.ready = await firstRunOver(storage as never);
  while (r.inbox.length > 0) await handle(r, r.inbox.shift() as string);
  if (startDue(r)) {
    if (r.readySince === 0) r.readySince = clock.ms;
    // The front page it speaks from is fetched first and the radio opens in this same answer once it is held, or once 2 s have passed since the first run was over (the opening then says it has no front page yet).
    // (It used to fetch in one answer and open in the next: with nothing else moving the tick, no next answer was asked, and a plain launch never started.)
    if (r.front.lines.length === 0) { try { await r.fetchFront(); } catch { /* none yet */ } }
    if (r.front.lines.length > 0 || clock.ms - r.readySince > 2000) await openRadioOf(r);
  }
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
  const r = await getRadio(storage, native);
  lap('radioWrite: radio ready');
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
    case 'openWeb': r.web.url = str(m.url); break;
    case 'closeWeb': r.web.url = ''; break;
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
  const r = await getRadio(storage, native); r.writer.listenerActed();
  const e = requestEntry(kind, ref, title);
  if (e) { if (mode === 'now') r.rt.play(e); else if (mode === 'next') r.rt.playNext(e); else r.rt.queue(e); }
  await r.rt.pump();
  return queueView(storage, native);
}

export async function queueAct(storage: Store | undefined, native: NativeReader | undefined, op: string, arg: string): Promise<QueueAnswer> {
  const r = await getRadio(storage, native); r.writer.listenerActed();
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
  const lines = r.front.lines;
  r.rt.director.open(phoneOpening(lines, clockNow(), -clockNow().getTimezoneOffset(), mintId('p-')));
  r.writer.openAhead(false);
  r.started = true;
  await r.rt.pump();
}

/** Play or queue one audio address (an episode, a clip's episode): the podcast page's Play and Add to queue. The audio is the deck's; the entry and the order are the sheet's. */
export async function radioPlayAudio(storage: Store | undefined, native: NativeReader | undefined, op: string, id: string, url: string, title: string, show: string, start: number): Promise<{ status: string; message: string }> {
  if (op === 'keep') {                                  // "Keep listening" to a clip: the clip no longer ends, the episode goes on from where it is (the deck's act, the player's own rule)
    const k = await getRadio(storage, native); k.writer.listenerActed();
    await k.rt.act('clearEnd');
    return { status: 'ok', message: '' };
  }
  if (url === '') return { status: 'error', message: 'This episode has no audio to play.' };
  const r = await getRadio(storage, native); r.writer.listenerActed();
  const e = audioEntry({ url, title: title === '' ? 'Episode' : title, show, ...(start > 0 ? { start } : {}), ...(id !== '' ? { episode_id: id } : {}) });
  if (op === 'queue') r.rt.queue(e); else r.rt.play(e);
  await r.rt.pump();
  return { status: 'ok', message: '' };
}

/** Play or queue one entry that shared code made (a clip row's Play and Queue: clip-play.ts). */
export async function radioPlayEntry(storage: Store | undefined, native: NativeReader | undefined, op: string, e: Entry): Promise<void> {
  const r = await getRadio(storage, native); r.writer.listenerActed();
  if (op === 'queue') r.rt.queue(e); else r.rt.play(e);
  await r.rt.pump();
}

/** Read an imported document aloud (or queue it): the words are the document's own, one line per block from block `at`; the entry is the sheet's book entry. */
export async function radioHandWords(storage: Store | undefined, native: NativeReader | undefined, op: string, id: string, title: string, at: number, text: string): Promise<{ ok: boolean; message: string }> {
  const paragraphs = text.split('\n').filter((p) => p !== '');
  if (id === '' || paragraphs.length === 0) return { ok: false, message: 'This document has nothing to read.' };
  const r = await getRadio(storage, native); r.writer.listenerActed();
  const e = bookEntry(id, title, at), words = { title, paragraphs };
  if (op === 'queue') { r.rt.queue(e, words); await r.rt.pump(); return { ok: true, message: 'Added to the queue.' }; }
  r.rt.play(e, words);
  await r.rt.pump();
  return { ok: true, message: '' };
}

export async function radioPlayPause(storage: Store | undefined, native: NativeReader | undefined): Promise<Ack> { const r = await getRadio(storage, native); r.writer.listenerActed(); await radioPlayPauseOf(r); return ACK; }
export async function radioStart(storage: Store | undefined, native: NativeReader | undefined): Promise<Ack> { const r = await getRadio(storage, native); r.writer.listenerActed(); await openRadioOf(r); return ACK; }
export async function radioPlayStory(storage: Store | undefined, native: NativeReader | undefined, route: string, title: string): Promise<Ack> {
  const r = await getRadio(storage, native); r.writer.listenerActed();
  if (route !== '') r.rt.play(paperStory(route, title));
  await r.rt.pump();
  const air = r.rt.store.onAir();
  console.log(`radio: play story ${route === '' ? '(no route)' : route.slice(0, 60)}: on air ${air ? air.id + ' ' + air.state : 'nothing'}, ahead ${r.rt.store.ahead().length}, director playing ${r.rt.director.playingId ?? 'none'}, message "${r.rt.director.message}"`);
  return ACK;
}
export async function radioListen(storage: Store | undefined, native: NativeReader | undefined, address: string, title: string, text: string): Promise<Ack> {
  const r = await getRadio(storage, native); r.writer.listenerActed();
  const w = listenWords(title, text);
  if (w) { const e = readAloud(address, title); e.est_seconds = estimateSeconds(w.paragraphs.join(' ')); r.rt.play(e, w); }
  await r.rt.pump(); return ACK;
}
export async function radioMove(storage: Store | undefined, native: NativeReader | undefined, move: string): Promise<Ack> {
  const r = await getRadio(storage, native); r.writer.listenerActed();
  if (move === 'previous') await r.rt.previous(); else r.rt.skip();
  await r.rt.pump(); return ACK;
}

/** The gestures on the Autoradio page: reorder (one `move`), remove (a swipe), play (a tap on a row), replay (a tap on Heard). */
export async function radioPageAct(storage: Store | undefined, native: NativeReader | undefined, op: string, arg: string, to: string): Promise<Ack> {
  const r = await getRadio(storage, native); r.writer.listenerActed();
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

/** The run sheet's own facts for the bar's overlay (bar.ts) and the Autoradio page (radio-page.ts). */
export async function barRun(storage: Store | undefined, native: NativeReader | undefined): Promise<Record<string, unknown>> {
  const r = await getRadio(storage, native, false);
  const v = r.rt.view();
  return { ready: true, onAir: v.onAir, waiting: v.waiting, remote: v.audio, remotePlaying: v.audio && v.playing, remoteTitle: v.current, remoteFraction: v.fraction };
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

export async function radioPageFacts(storage: Store | undefined, native: NativeReader | undefined): Promise<RadioFacts> {
  const r = await getRadio(storage, native, false);
  const v = r.rt.view(), later = nativeLater();
  let sentences: string[] = [], sentenceIndex = 0, progress = 0;
  if (later) { try { const o = obj(await later({ op: 'radioFacts', peek: true })); sentences = Array.isArray(o.sentences) ? o.sentences.filter((x): x is string => typeof x === 'string') : []; sentenceIndex = num(o.sentenceIndex); progress = num(o.progress); } catch { /* none */ } }
  const prog = r.rt.programme(), route = prog.route;
  const airEntry = r.rt.store.onAir(), takeRef = airEntry?.kind === 'take' && airEntry.source?.ref.startsWith('take:') ? airEntry.source.ref.slice(5) : '';
  const take = takeRef !== '' ? r.takeList.find((t) => t.takeID === takeRef) : undefined;
  const published = route !== '' ? r.front.published[route] ?? 0 : 0;
  const versions = Object.entries(r.front.versions);
  return {
    at: clock.ms / 1000, sheet: sheetText(r.rt.store.sheet), playing: v.playing, sentences, sentenceIndex, progress: v.audio ? v.fraction : progress,
    ready: true, onAir: v.onAir, waiting: v.waiting, remoteAudio: v.audio, remoteElapsed: v.elapsed, spokenElapsed: v.spokenElapsed, remoteDuration: v.duration, wants: r.rt.director.wants, message: v.message, notice: v.notice, writing: v.writing, dry: v.dry,
    airTitle: prog.title, airRoute: route, publishedAt: published, versionRoutes: versions.map(([k]) => k), versionNumbers: versions.map(([, n]) => n),
    stale: r.rt.store.ahead().filter((e) => r.rt.store.isStale(e)).map((e) => e.id), takeTranscript: take?.transcript ?? '', takeRecord: take?.record?.summary ?? '',
  };
}

export { audioEntry, bookEntry };
