// The take session, in shared code. It was TakeSession.swift's publish, refresh, delete and notice half plus TakesPage.swift, AutoradioModule+Takes.swift's `takesAct` and the Swift stores of the
// listener's own ids and muted voices (Choir/Takes.swift). What stays native is the RECORDER (TakeSession.swift, on the voice list): it holds the microphone and the radio's gate, writes the
// file, reads it with the on-device recogniser, plays it back for review and stops itself at the ceiling. It is handed the numbers (LIMITS) and answers one snapshot (`Native`):
// a phase, the seconds, a transcript, and `ended`, a CODE for why a take ended without a recording. The WORDS for every code are here.
//
// The split in one line: native = a device that makes a file and a transcript; shared = whether to publish it, to whom, with what words, and everything the listener keeps.
// Native calls (the app module, ModelsEntry.swift, each with the reason only native code can meet):
//   takeState / takeStart / takeStop / takePreview / takeDiscard / takeDone   the recorder (microphone, gate, file, recogniser, player)
//   takeAudio                      the file's bytes, base64 (the file is native's: the preview plays it)
//   takeSteerMute                  the learning engine's steer queue is native (SteerQueue)
//   takeMirror / takeLegacy        the Swift programme still reads the listener's own ids and muted voices to filter the takes it offers (RadioProgramme); shared code is the owner of that
//                                  data (app:/data/takes.json) and pushes a copy; `takeLegacy` READS the old Swift files once (the carry-over) and removes nothing.
// A take in review belongs to the recorder, not to this module: a restart of the shared code finds it (`takeState`) and can publish it; a failed publish never drops it.
import type { Storage } from './app.contract.d.ts';
import { nativeLater, SignedOutError } from './api.ts';
import {
  BOUNDS, addOwn, client, cut, loadKept, mp4Seconds, mute as muteIn, page, removeOwn, saveKept, seamNet, sha256Hex, unmute as unmuteIn, validHandle,
  TakesRefused, TakesUnavailable, type Kept, type MyTake, type Net, type Record_, type Recorder, type TakesPageInfo,
} from './takes.ts';

export type Ended = '' | 'mic' | 'gate' | 'open' | 'short' | 'empty' | 'large' | 'ceiling';
export type Native = {
  /** idle | recording | transcribing | review */
  phase: 'idle' | 'recording' | 'transcribing' | 'review';
  /** While recording, the seconds so far; after it, the length of the take. */
  elapsed: number; seconds: number; bytes: number; transcript: string; previewing: boolean;
  /** Why the last take ended without a recording (or `ceiling`: it stopped itself at the limit and the take is in review). Cleared by the next start. */
  ended: Ended; detail: string;
  respondsTitle: string; respondsRoute: string;
};
export const IDLE: Native = { phase: 'idle', elapsed: 0, seconds: 0, bytes: 0, transcript: '', previewing: false, ended: '', detail: '', respondsTitle: '', respondsRoute: '' };
/** The numbers the recorder enforces (it stops itself at `maxSeconds`, keeps nothing under `minSeconds` or over `maxBytes`). */
export const LIMITS = { minSeconds: BOUNDS.minSeconds, maxSeconds: BOUNDS.maxSeconds - BOUNDS.stopEarly, maxBytes: BOUNDS.maxBytes };

const PHASES = new Set(['idle', 'recording', 'transcribing', 'review']);
const ENDED = new Set(['', 'mic', 'gate', 'open', 'short', 'empty', 'large', 'ceiling']);
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? Math.max(0, v) : 0);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
/** The recorder's snapshot, read defensively: anything it does not say is idle. */
export function parseNative(v: unknown): Native {
  if (v === null || typeof v !== 'object') return { ...IDLE };
  const o = v as Record<string, unknown>;
  return {
    phase: typeof o.phase === 'string' && PHASES.has(o.phase) ? (o.phase as Native['phase']) : 'idle', elapsed: num(o.elapsed), seconds: num(o.seconds), bytes: num(o.bytes),
    transcript: str(o.transcript), previewing: o.previewing === true, ended: typeof o.ended === 'string' && ENDED.has(o.ended) ? (o.ended as Ended) : '', detail: str(o.detail),
    respondsTitle: str(o.respondsTitle), respondsRoute: str(o.respondsRoute),
  };
}

export type Legacy = { found: boolean; ids: string[]; handle: string; muted: string[] };
export type Device = {
  state(): Promise<Native>;
  start(limits: typeof LIMITS): Promise<Native>;
  stop(): Promise<Native>;
  /** null toggles. */
  preview(on: boolean | null): Promise<Native>;
  discard(): Promise<Native>;
  audio(): Promise<Uint8Array | null>;
  /** The take is published: drop the file and give the radio back. */
  done(): Promise<Native>;
  steerMute(handle: string): Promise<void>;
  mirror(k: { ids: string[]; handle: string; muted: string[] }): Promise<void>;
  legacy(): Promise<Legacy | null>;
};

/** The device over the app module's `later`, or null when there is no module (Android today, the web). */
export function nativeDevice(): Device | null {
  const raw = nativeLater();
  if (!raw) return null;
  const later = raw as unknown as (r: Record<string, unknown>) => Promise<unknown>;
  const snap = async (r: Record<string, unknown>): Promise<Native> => parseNative(await later(r));
  const strs = (v: unknown): string[] => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []);
  return {
    state: () => snap({ op: 'takeState' }),
    start: (limits) => snap({ op: 'takeStart', ...limits }),
    stop: () => snap({ op: 'takeStop' }),
    preview: (on) => snap({ op: 'takePreview', mode: on === null ? 'toggle' : on ? 'on' : 'off' }),
    discard: () => snap({ op: 'takeDiscard' }),
    done: () => snap({ op: 'takeDone' }),
    async audio() {
      const r = (await later({ op: 'takeAudio' })) as { b64?: unknown } | null;
      if (!r || typeof r.b64 !== 'string' || r.b64 === '') return null;
      const s = atob(r.b64); const out = new Uint8Array(s.length);
      for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
      return out;
    },
    async steerMute(handle) { await later({ op: 'takeSteerMute', handle }); },
    async mirror(k) { await later({ op: 'takeMirror', ids: k.ids, handle: k.handle, muted: k.muted }); },
    async legacy() {
      const r = (await later({ op: 'takeLegacy' })) as Record<string, unknown> | null;
      return r && typeof r.found === 'boolean' ? { found: r.found, ids: strs(r.ids), handle: str(r.handle), muted: strs(r.muted) } : null;
    },
  };
}

// ---------------------------------------------------------------------------------------------------------------- what this module keeps between answers

export type Overlay = {
  /** A line this module says (a failed publish, a server without takes); the recorder's own words come from its `ended` code. */
  notice: string;
  published: string; mine: MyTake[]; myRecord: Record_ | null; publishing: boolean;
  kept: Kept | null; carried: boolean;
};
export const newOverlay = (): Overlay => ({ notice: '', published: '', mine: [], myRecord: null, publishing: false, kept: null, carried: false });
export type Env = { device: Device | null; storage: Storage | undefined; net: Net };
export const liveEnv = (storage: Storage | undefined): Env => ({ device: nativeDevice(), storage, net: seamNet });

async function keptOf(env: Env, o: Overlay): Promise<Kept> { return (o.kept ??= await loadKept(env.storage)); }
/** Keep it and give the native programme its copy. The mirror follows the file: a copy is never pushed that was not written. */
async function keep(env: Env, o: Overlay, k: Kept): Promise<void> {
  o.kept = k;
  if (await saveKept(env.storage, k)) { try { await env.device?.mirror(k); } catch { /* the programme keeps its last copy */ } }
}

/** The one-time carry-over of the Swift app's own ids, handle and muted voices (Application Support/Autoradio/takes-own.json, takes-muted.json) into app:/data/takes.json. The old files are READ and
 * never removed (a reverted build still finds them, and the mirror keeps them current). Marked `migrated` in the file only when the write succeeded; no answer from native is tried again at the
 * next call. What is already in the new file wins (a handle) or is added to (ids, mutes). Delete this, `takeLegacy` and the field once phones have moved. */
export async function carryOver(env: Env, o: Overlay): Promise<void> {
  if (o.carried || !env.device || !env.storage) return;
  const k = await keptOf(env, o);
  if (k.migrated) { o.carried = true; return; }
  let l: Legacy | null = null;
  try { l = await env.device.legacy(); } catch { l = null; }
  if (!l) return;
  const next: Kept = {
    ids: [...new Set([...k.ids, ...l.ids])], handle: k.handle !== '' ? k.handle : validHandle(l.handle) ? l.handle : '',
    muted: [...new Set([...k.muted, ...l.muted.filter(validHandle)])], migrated: true,
  };
  if (!(await saveKept(env.storage, next))) { o.kept = null; return; }
  o.kept = next; o.carried = true;
  if (l.found) { try { await env.device.mirror(next); } catch { /* the programme keeps its last copy */ } }
}

// ---------------------------------------------------------------------------------------------------------------- words

const why = (e: unknown): string => (e instanceof TakesUnavailable ? e.message : 'the connection failed');
/** The recorder's own words, from its code. */
function nativeNotice(n: Native): string {
  switch (n.ended) {
    case 'mic': return 'The microphone is not allowed. Allow it in Settings to record a take.';
    case 'gate': return `Could not start: ${n.detail}`;
    case 'open': return 'Could not record: the microphone could not be opened.';
    case 'short': return `A take is at least ${BOUNDS.minSeconds} seconds. Nothing was kept.`;
    case 'empty': return 'Nothing was recorded.';
    case 'large': return 'That take is too large to publish. Record a shorter one.';
    case 'ceiling': break;
    default: break;
  }
  const lead = n.ended === 'ceiling' ? `Stopped at ${BOUNDS.maxSeconds} seconds, the longest a take can be.` : '';
  const blank = n.phase === 'review' && n.transcript.trim() === '' ? 'Nothing was recognised. Record it again.' : '';
  return [lead, blank].filter((x) => x !== '').join(' ');
}
const canPublish = (n: Native, o: Overlay): boolean => n.phase === 'review' && !o.publishing && n.transcript.trim() !== '';

function recorderOf(n: Native, o: Overlay): Recorder {
  return {
    state: n.phase === 'review' && o.publishing ? 'publishing' : n.phase, elapsed: n.elapsed, transcript: cut(n.transcript, BOUNDS.maxTranscriptBytes), previewing: n.previewing,
    notice: o.notice !== '' ? o.notice : nativeNotice(n), respondsTitle: n.respondsTitle, canPublish: canPublish(n, o),
  };
}

/** The page for a recorder snapshot (the native source `takesRecorder`). */
export async function pageFor(env: Env, o: Overlay, rec: unknown): Promise<TakesPageInfo> {
  const kept = await keptOf(env, o);
  return page(recorderOf(parseNative(rec), o), { mine: o.mine, kept, myRecord: o.myRecord, justPublished: o.published });
}

// ---------------------------------------------------------------------------------------------------------------- the acts

async function refresh(env: Env, o: Overlay): Promise<void> {
  await carryOver(env, o);
  const c = client(env.net);
  try {
    const m = await c.mine();
    o.mine = m;
    let k = await keptOf(env, o), changed = false;
    for (const t of m) { const next = addOwn(k, t.takeID, t.publisher); if (next.handle !== k.handle || next.ids.length !== k.ids.length) { k = next; changed = true; } }
    if (changed) await keep(env, o, k);
  } catch (e) {
    if (e instanceof SignedOutError) return;
    if (e instanceof TakesUnavailable) { o.notice = e.message; return; }
  }
  const h = (await keptOf(env, o)).handle;
  if (h !== '') { try { o.myRecord = await c.record(h); } catch { /* the last record stays */ } }
}

async function publish(env: Env, o: Overlay): Promise<void> {
  const dev = env.device;
  if (!dev || o.publishing) return;
  const n = await dev.state();
  if (!canPublish(n, o)) return;
  await carryOver(env, o);
  o.publishing = true; o.notice = ''; o.published = '';
  try {
    await dev.preview(false);                  // stops the playback, and tells the page that the take is being published
    const audio = await dev.audio();
    if (!audio) { o.notice = 'Could not publish: the recording is gone. Your recording is kept; try again.'; return; }
    // The server reads the length from the file's own header and holds the claim to within 2 s of it: claim what the file says.
    const header = mp4Seconds(audio);
    const claimed = Math.min(Math.max(header !== null && Number.isFinite(header) && header > 0 ? header : n.seconds, BOUNDS.minSeconds), BOUNDS.maxSeconds);
    const responds = n.respondsRoute !== '' ? { route: n.respondsRoute, title: n.respondsTitle } : null;
    const boundary = 'choir-take-' + (await sha256Hex(audio)).slice(0, 24);       // no randomness in shared code: the audio's own hash names the parts
    let t;
    try { t = await client(env.net).publish({ audio, transcript: cut(n.transcript, BOUNDS.maxTranscriptBytes), duration: claimed, respondsTo: responds }, boundary); }
    catch (e) {
      // The recording is kept (the recorder still holds it): nothing is lost when the server is not there yet or refuses.
      if (e instanceof SignedOutError) o.notice = 'Sign in to publish a take.';
      else if (e instanceof TakesUnavailable) o.notice = 'Takes are not available yet. Your recording is kept; publish it again later.';
      else if (e instanceof TakesRefused) o.notice = `${e.message} Your recording is kept.`;
      else o.notice = `Could not publish: ${why(e)}. Your recording is kept; try again.`;
      console.log(`takes: publish failed (${e instanceof Error ? e.name : 'error'})`);
      return;
    }
    await keep(env, o, addOwn(await keptOf(env, o), t.takeID, t.publisher));
    o.published = t.publisher;
    await dev.done();
    console.log(`takes: published id=${t.takeID}`);
  } finally { o.publishing = false; }
  await refresh(env, o);
}

async function del(env: Env, o: Overlay, id: string): Promise<void> {
  await carryOver(env, o);
  try {
    await client(env.net).remove(id);
    await keep(env, o, removeOwn(await keptOf(env, o), id));
    o.mine = o.mine.filter((t) => t.takeID !== id); o.notice = '';
  } catch (e) { o.notice = e instanceof SignedOutError ? 'Sign in to delete a take.' : `Could not delete: ${why(e)}.`; }
}

/** One press on the Takes section (or the on-air Take control, the radio's mute). Every answer is the page, read after the act. */
export async function act(env: Env, o: Overlay, op: string, arg: string): Promise<TakesPageInfo> {
  const dev = env.device;
  switch (op) {
    case 'record': case 'rerecord':
      o.notice = ''; o.published = '';
      if (!dev) { o.notice = 'Recording is not available on this phone yet.'; break; }
      await dev.start(LIMITS);
      break;
    case 'stop': await dev?.stop(); break;
    case 'preview': await dev?.preview(null); break;
    case 'discard': o.notice = ''; await dev?.discard(); break;
    case 'publish': await publish(env, o); break;
    case 'delete': await del(env, o, arg); break;
    case 'mute': {
      await carryOver(env, o);
      const r = muteIn(await keptOf(env, o), arg);
      if (r.added) {
        await keep(env, o, r.kept);
        try { await dev?.steerMute(arg); } catch { /* the steer queue keeps what it has */ }
      }
      break;
    }
    case 'unmute': {
      await carryOver(env, o);
      const r = unmuteIn(await keptOf(env, o), arg);
      if (r.had) await keep(env, o, r.kept);
      break;
    }
    case 'refresh': await refresh(env, o); break;
    default: break;
  }
  return pageFor(env, o, dev ? await dev.state() : IDLE);
}

