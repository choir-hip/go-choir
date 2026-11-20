// The take session, in shared code. It was TakeSession.swift's publish, refresh, delete and notice half plus TakesPage.swift, AutoradioModule+Takes.swift's `takesAct` and the Swift stores of the
// listener's own ids and muted voices (Choir/Takes.swift). What stays native is the RECORDER (TakeSession.swift, on the voice list): it holds the microphone and the radio's gate, writes the
// file, reads it with the on-device recogniser, plays it back for review and stops itself at the ceiling. It is handed the numbers (`limits()`) and answers one snapshot (`Native`):
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
import { accountGeneration, nativeLater, SignedOutError } from './api.ts';
import {
  BOUNDS, addOwn, client, cut, emptyKept, loadKept, mp4Seconds, mute as muteIn, page, removeOwn, saveKept, seamNet, sha256Hex, unmute as unmuteIn, validHandle,
  TakesRefused, TakesUnavailable, type Kept, type MyTake, type Net, type Record_, type Recorder, type TakesPageInfo,
} from './takes.ts';

export type Ended = '' | 'mic' | 'gate' | 'open' | 'short' | 'empty' | 'large' | 'ceiling';
export type Native = {
  /** idle | recording | transcribing | review */
  phase: 'idle' | 'recording' | 'transcribing' | 'review';
  /** While recording, the seconds so far; after it, the length of the take. */
  elapsed: number; seconds: number; transcript: string; previewing: boolean;
  /** Shared code is sending the take: only here so the page is asked again (the recorder's snapshot moves). It is never read as a fact (a restart of the shared code leaves it stale). */
  publishing: boolean;
  /** Why the last take ended without a recording (or `ceiling`: it stopped itself at the limit and the take is in review). Cleared by the next start. */
  ended: Ended; detail: string;
  respondsTitle: string; respondsRoute: string;
};
export const IDLE: Native = { phase: 'idle', elapsed: 0, seconds: 0, transcript: '', previewing: false, publishing: false, ended: '', detail: '', respondsTitle: '', respondsRoute: '' };
/** The numbers the recorder enforces (it stops itself at `maxSeconds`, keeps nothing under `minSeconds` or over `maxBytes`). */
export const limits = () => ({ minSeconds: BOUNDS.minSeconds, maxSeconds: BOUNDS.maxSeconds - BOUNDS.stopEarly, maxBytes: BOUNDS.maxBytes });

const PHASES = new Set(['idle', 'recording', 'transcribing', 'review']);
const ENDED = new Set(['', 'mic', 'gate', 'open', 'short', 'empty', 'large', 'ceiling']);
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? Math.max(0, v) : 0);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
/** The recorder's snapshot, read defensively: anything it does not say is idle. */
export function parseNative(v: unknown): Native {
  if (v === null || typeof v !== 'object') return { ...IDLE };
  const o = v as Record<string, unknown>;
  return {
    phase: typeof o.phase === 'string' && PHASES.has(o.phase) ? (o.phase as Native['phase']) : 'idle', elapsed: num(o.elapsed), seconds: num(o.seconds),
    transcript: str(o.transcript), previewing: o.previewing === true, publishing: o.publishing === true, ended: typeof o.ended === 'string' && ENDED.has(o.ended) ? (o.ended as Ended) : '', detail: str(o.detail),
    respondsTitle: str(o.respondsTitle), respondsRoute: str(o.respondsRoute),
  };
}

/** `carried`: the native marker says this phone's upgrade carry-over was done once (by whichever account): the old files are never read for another one. */
export type Legacy = { found: boolean; ids: string[]; handle: string; muted: string[]; carried?: boolean };
export type Device = {
  state(): Promise<Native>;
  start(l: ReturnType<typeof limits>): Promise<Native>;
  stop(): Promise<Native>;
  /** null toggles. */
  preview(on: boolean | null): Promise<Native>;
  discard(): Promise<Native>;
  audio(): Promise<Uint8Array | null>;
  /** Tell the recorder a publish is under way (or over): the snapshot moves, so the page is asked again. */
  publishing(on: boolean): Promise<Native>;
  /** The take is published: drop the file and give the radio back. */
  done(): Promise<Native>;
  steerMute(handle: string): Promise<void>;
  mirror(k: { ids: string[]; handle: string; muted: string[] }): Promise<void>;
  legacy(): Promise<Legacy | null>;
  /** The carry-over has been written: leave a durable marker native-side (not removed by the account wipe). True when it is there. */
  carried(): Promise<boolean>;
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
    start: (l) => snap({ op: 'takeStart', ...l }),
    stop: () => snap({ op: 'takeStop' }),
    preview: (on) => snap({ op: 'takePreview', mode: on === null ? 'toggle' : on ? 'on' : 'off' }),
    discard: () => snap({ op: 'takeDiscard' }),
    done: () => snap({ op: 'takeDone' }),
    publishing: (on) => snap({ op: 'takePublishing', on }),
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
      return r && typeof r.found === 'boolean' ? { found: r.found, ids: strs(r.ids), handle: str(r.handle), muted: strs(r.muted), carried: r.carried === true } : null;
    },
    async carried() { const r = (await later({ op: 'takeCarried' })) as { ok?: unknown } | null; return r?.ok === true; },
  };
}

// ---------------------------------------------------------------------------------------------------------------- what this module keeps between answers

export type Overlay = {
  /** A line this module says (a failed publish, a server without takes); the recorder's own words come from its `ended` code. */
  notice: string;
  published: string; mine: MyTake[]; myRecord: Record_ | null; publishing: boolean;
  kept: Kept | null; carried: boolean;
  /** Every change of the kept data is queued here (mutate): two at the same time are applied one after the other, each to the data the one before left. */
  queue: Promise<unknown>;
  /** The account generation this overlay belongs to (api.ts accountGeneration); null until first used. */
  gen: number | null;
};
export const newOverlay = (): Overlay => ({ notice: '', published: '', mine: [], myRecord: null, publishing: false, kept: null, carried: false, queue: Promise.resolve(), gen: null });
export type Env = { device: Device | null; storage: Storage | undefined; net: Net };
export const liveEnv = (storage: Storage | undefined): Env => ({ device: nativeDevice(), storage, net: seamNet });

/** Everything this module holds in memory belongs to ONE account. When the generation moved (a sign-out, an account change: account.ts bumps it before the wipes) the overlay is dropped whole, no
 * carry-over is run for the rest of the process (the old files cannot be adopted by the next account), and the native programme's copy is emptied at once, even if the native wipe had failed. */
async function sync(env: Env, o: Overlay): Promise<boolean> {
  const g = accountGeneration();
  if (o.gen === g) return false;
  const had = o.gen !== null;
  Object.assign(o, newOverlay(), { gen: g, carried: had });
  if (had) {
    try { await env.device?.discard(); } catch { /* the sign-out path discards it natively before the sign-out; this is the second path */ }
    try { await env.device?.mirror(emptyKept()); } catch { /* the native wipe is the other path */ }
  }
  return had;
}
async function keptOf(env: Env, o: Overlay): Promise<Kept> { return (o.kept ??= await loadKept(env.storage)); }
/** Keep it (the write is held to the generation `gen` the data belongs to) and give the native programme its copy. The mirror follows the file: a copy is never pushed that was not written. */
async function keep(env: Env, o: Overlay, k: Kept, gen: number, mirror = true): Promise<boolean> {
  if (!(await saveKept(env.storage, k, gen))) return false;
  o.kept = k;
  if (mirror) { try { await env.device?.mirror(k); } catch { /* the programme keeps its last copy */ } }
  return true;
}
/** The ONE way the kept data changes: `f` is applied to the data as the change before it left it (changes are queued, so none is lost), and nothing is written for an account that is no longer
 * the overlay's. `kept: null` means no change. */
async function mutate<T>(env: Env, o: Overlay, f: (k: Kept) => { kept: Kept | null; out: T }, skipped: T, mirror = true): Promise<{ out: T; ok: boolean }> {
  const gen = o.gen;
  const run = async (): Promise<{ out: T; ok: boolean }> => {
    if (gen === null || o.gen !== gen || accountGeneration() !== gen) return { out: skipped, ok: false };
    const r = f(await keptOf(env, o));
    if (r.kept === null) return { out: r.out, ok: true };
    return { out: r.out, ok: await keep(env, o, r.kept, gen, mirror) };
  };
  const p = o.queue.then(run, run);
  o.queue = p.catch(() => undefined);
  return p;
}

/** The one-time carry-over of the Swift app's own ids, handle and muted voices (Application Support/Autoradio/takes-own.json, takes-muted.json) into app:/data/takes.json. It is a PHONE's upgrade,
 * not an account's: a durable native marker (`takeCarried`, beside the old files, not removed by the account wipe) says it was done, and with it no account ever reads the old files again, whatever a
 * failed unlink or a restart left there. The old files are READ and never removed (a reverted build still finds them). The marker is written only after the shared write succeeded, and made good at
 * the next process when it is missing. What is already in the new file wins (a handle) or is added to (ids, mutes). In every case the programme's copy is then made equal to the shared data.
 * Delete this, `takeLegacy`, `takeCarried` and the field once phones have moved. */
export async function carryOver(env: Env, o: Overlay): Promise<void> {
  const dev = env.device;
  if (o.carried || !dev || !env.storage) return;
  const k = await keptOf(env, o);
  if (k.migrated) {
    try { await dev.carried(); } catch { /* tried again at the next process */ }
    try { await dev.mirror(k); } catch { /* the programme keeps its last copy */ }
    o.carried = true; return;
  }
  let l: Legacy | null = null;
  try { l = await dev.legacy(); } catch { l = null; }
  if (!l) return;
  if (l.carried) {          // done once on this phone: this account starts from nothing, and the programme's copy is made to say so
    try { await dev.mirror(k); } catch { /* the programme keeps its last copy */ }
    o.carried = true; return;
  }
  const old = l;
  const r = await mutate(env, o, (cur) => (cur.migrated ? { kept: null, out: true } : {
    kept: { ids: [...new Set([...cur.ids, ...old.ids])], handle: cur.handle !== '' ? cur.handle : validHandle(old.handle) ? old.handle : '',
      muted: [...new Set([...cur.muted, ...old.muted.filter(validHandle)])], migrated: true }, out: true }), false, old.found);
  if (!r.ok) return;
  o.carried = true;
  try { await dev.carried(); } catch { /* made good at the next process */ }
}

// ---------------------------------------------------------------------------------------------------------------- words

const why = (e: unknown): string => (e instanceof TakesUnavailable || e instanceof TakesRefused ? e.message.replace(/\.$/, '') : 'the connection failed');
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
const reviewable = (n: Native): boolean => n.phase === 'review' && n.transcript.trim() !== '';
const canPublish = (n: Native, o: Overlay): boolean => reviewable(n) && !o.publishing;

function recorderOf(n: Native, o: Overlay): Recorder {
  return {
    state: n.phase === 'review' && o.publishing ? 'publishing' : n.phase, elapsed: n.elapsed, transcript: cut(n.transcript, BOUNDS.maxTranscriptBytes), previewing: n.previewing,
    notice: o.notice !== '' ? o.notice : nativeNotice(n), respondsTitle: n.respondsTitle, canPublish: canPublish(n, o),
  };
}

/** The page for a recorder snapshot (the native source `takesRecorder`). */
export async function pageFor(env: Env, o: Overlay, rec: unknown): Promise<TakesPageInfo> {
  const reset = await sync(env, o);
  const kept = await keptOf(env, o);
  // the snapshot the page was asked with may predate the discard of the old account's take: read it again
  const snap = reset && env.device ? await env.device.state() : rec;
  return page(recorderOf(parseNative(snap), o), { mine: o.mine, kept, myRecord: o.myRecord, justPublished: o.published });
}

// ---------------------------------------------------------------------------------------------------------------- the acts

async function refresh(env: Env, o: Overlay): Promise<void> {
  await carryOver(env, o);
  const c = client(env.net);
  try {
    const m = await c.mine();
    o.mine = m; o.notice = '';
    await mutate(env, o, (k) => {
      let n = k;
      for (const t of m) n = addOwn(n, t.takeID, t.publisher);
      return { kept: n.handle !== k.handle || n.ids.length !== k.ids.length ? n : null, out: undefined };
    }, undefined);
  } catch (e) {
    if (e instanceof SignedOutError) return;
    if (e instanceof TakesUnavailable || e instanceof TakesRefused) { o.notice = e.message; return; }
  }
  const h = (await keptOf(env, o)).handle;
  if (h !== '') { try { o.myRecord = await c.record(h); } catch { /* the last record stays */ } }
}

async function publish(env: Env, o: Overlay): Promise<void> {
  const dev = env.device;
  if (!dev || o.publishing) return;
  o.publishing = true;                           // before the first await: two presses in the same window are one publish
  const gen = accountGeneration();
  let told = false, sent = false;
  try {
    const n = await dev.state();
    if (!reviewable(n)) return;
    await carryOver(env, o);
    o.notice = ''; o.published = '';
    told = true; await dev.publishing(true);     // the recorder's snapshot moves: the page is asked again and shows "publishing" with nothing to press
    await dev.preview(false);                    // stops the playback
    const audio = await dev.audio();
    if (!audio) { o.notice = 'Could not publish: the recording is gone. Your recording is kept; try again.'; return; }
    // The server reads the length from the file's own header and holds the claim to within 2 s of it: claim what the file says.
    const header = mp4Seconds(audio);
    const claimed = Math.min(Math.max(header !== null && Number.isFinite(header) && header > 0 ? header : n.seconds, BOUNDS.minSeconds), BOUNDS.maxSeconds);
    const responds = n.respondsRoute !== '' ? { route: n.respondsRoute, title: n.respondsTitle } : null;
    const digest = await sha256Hex(audio);
    const boundary = 'choir-take-' + digest.slice(0, 24);       // no randomness in shared code: the audio's own hash names the parts
    let t;
    try { t = await client(env.net).publish({ audio, sha256: digest, transcript: cut(n.transcript, BOUNDS.maxTranscriptBytes), duration: claimed, respondsTo: responds }, boundary); }
    catch (e) {
      // The recording is kept (the recorder still holds it): nothing is lost when the server is not there yet or refuses.
      if (e instanceof SignedOutError) o.notice = 'Sign in to publish a take.';
      else if (e instanceof TakesUnavailable) o.notice = 'Takes are not available yet. Your recording is kept; publish it again later.';
      else if (e instanceof TakesRefused) o.notice = `${e.message} Your recording is kept.`;
      else o.notice = `Could not publish: ${why(e)}. Your recording is kept; try again.`;
      console.log(`takes: publish failed (${e instanceof Error ? e.name : 'error'})`);
      return;
    }
    if (accountGeneration() !== gen) return;     // another account is in: nothing of this publish is kept here
    await mutate(env, o, (k) => ({ kept: addOwn(k, t.takeID, t.publisher), out: undefined }), undefined);
    o.published = t.publisher;
    try { await dev.done(); } catch { try { await dev.discard(); } catch { /* the recorder keeps the file; the server derives the id from the audio, so a second publish is the same take */ } }
    console.log(`takes: published id=${t.takeID}`);
    sent = true;
  } finally {
    o.publishing = false;
    if (told) { try { await dev.publishing(false); } catch { /* the next snapshot says it */ } }
  }
  if (sent) await refresh(env, o);
}

async function del(env: Env, o: Overlay, id: string): Promise<void> {
  await carryOver(env, o);
  try {
    await client(env.net).remove(id);
    await mutate(env, o, (k) => ({ kept: removeOwn(k, id), out: undefined }), undefined);
    o.mine = o.mine.filter((t) => t.takeID !== id); o.notice = '';
  } catch (e) { o.notice = e instanceof SignedOutError ? 'Sign in to delete a take.' : `Could not delete: ${why(e)}.`; }
}

const RECORDER_OPS = new Set(['record', 'rerecord', 'stop', 'preview', 'discard']);
/** One press on the Takes section (or the on-air Take control, the radio's mute). Every answer is the page, read after the act. */
export async function act(env: Env, o: Overlay, op: string, arg: string): Promise<TakesPageInfo> {
  await sync(env, o);
  const dev = env.device;
  // While a take is being sent the recorder is not touched: a discard would delete the file under the upload and a re-record would be cancelled by the publish's own `done`.
  if (o.publishing && RECORDER_OPS.has(op)) return pageFor(env, o, dev ? await dev.state() : IDLE);
  switch (op) {
    case 'record': case 'rerecord':
      o.notice = ''; o.published = '';
      if (!dev) { o.notice = 'Recording is not available on this phone yet.'; break; }
      await dev.start(limits());
      break;
    case 'stop': await dev?.stop(); break;
    case 'preview': await dev?.preview(null); break;
    case 'discard': o.notice = ''; await dev?.discard(); break;
    case 'publish': await publish(env, o); break;
    case 'delete': await del(env, o, arg); break;
    case 'mute': {
      await carryOver(env, o);
      const r = await mutate(env, o, (k) => { const x = muteIn(k, arg); return { kept: x.added ? x.kept : null, out: x.added }; }, false);
      if (r.out) { try { await dev?.steerMute(arg); } catch { /* the steer queue keeps what it has */ } }
      break;
    }
    case 'unmute': {
      await carryOver(env, o);
      await mutate(env, o, (k) => { const x = unmuteIn(k, arg); return { kept: x.had ? x.kept : null, out: x.had }; }, false);
      break;
    }
    case 'refresh': await refresh(env, o); break;
    default: break;
  }
  return pageFor(env, o, dev ? await dev.state() : IDLE);
}

