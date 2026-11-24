// First run, as shared code: WHICH steps exist, their order, what each asks, and what is kept. It replaces the Swift OnboardingController's step machine
// (AutoradioModule+Onboarding.swift) and its page model. The steps, in order:
//   welcome -> setup (the voice models) -> mic (can I hear you) -> hearing (can you hear me) -> learn (read three sentences, the person's voice is learned, one more checks it) -> voice (pick a voice) -> ready -> topics -> writing -> done
//   and `skipped` (Not now), which comes back to the topics through "begin".
// What stays native, each as a FACT in `VoiceFacts` (the source `voiceSetupFacts`) or one narrow call (`later({op})`, `obOp` below), with its reason: the voice models
// and their download (VoiceSetup.swift, ModelStore: the on-device models), the microphone permission and the live check (the microphone), the sentence spoken at the
// hearing check (the speaker route), the voice at the topic screen (the engine's stages), and the page list of the iOS deck (still native there). The topics themselves
// are edition-picks.ts: the page passes the list on the done sends, as the picks lane wrote it.
//
// Kept (`app:/data/onboarding.json`, registered with the account wipe list): offered, skipped, setupDone, micRefused, topicsStarted, editionDone, voicePicked.
// In memory: the step as the listener left it this run, the ask for mobile data, the message, the edition and its status.
//
// Failure list (each line has a test in onboarding.test.ts):
//  O1 The first run walks the steps in the order above and no other; a step's button that does not belong to the step changes nothing.
//  O2 A phone off Wi-Fi is asked before anything downloads: "begin" without the yes only asks; "begin" with it starts the setup, once (a second begin while it runs is a no-op).
//  O3 Setup finishing (a native fact) moves "setup" to "mic" and keeps `setupDone`; it is not lost by a restart.
//  O4 A refused microphone is kept (`micRefused`) and said; "Talk to it" then says so and starts nothing; a silent one says so, and the topics step still works.
//  O5 "No" at the hearing check plays it again and says what to try; "Yes" goes on to the learn-your-voice step (then the voice step) and keeps `setupDone`.
//  O6 The voice step ends when the voice cards report a pick (a counter fact), not by a button here.
//  O7 Skip stops the voice and the microphone and is kept; a skipped first run that finished its setup begins again at "ready", with nothing downloaded again.
//  O8 Continue writes the edition from the list the page sent (an empty list is the general edition). It always ends: written, failed, or the deadline, and the topics stay.
//  O9 "Continue anyway" makes the general edition from nothing. A second Continue while one is writing is ignored.
//  O10 Editing after the first run (the edit page is the top page) does not move the first-run step; Done closes the page; Leave closes it without writing.
//  O11 The offer: a fresh install asks the computer first; a listener who already has an edition is not offered the topics step; no answer offers it. Once.
//  O12 The prompt card is gone once the edition is made or the first run skipped, and says where the listener left off otherwise.
//  O13 The kept file is read by shape: a corrupt, empty or wrong-typed file starts fresh and never throws; sign-out removes it and forgets the memory.
//  O14 The station may start by itself only when the first run is over (done or skipped): that fact is told to native code on every change.
//  O15 With no key the first run is over for the station (the paper plays at once); a locked or unreadable key leaves it to the first run as before.
//  O17 The learn step ends with `learnDone` (stored, or Not now) into the voice step and tells native to stop listening; `micSkip` (Not now at the microphone) goes to the voice step without opening the microphone.
//  O18 The press that starts the setup hands native the chosen option of every job (the person's pick, else the one recommended for this phone) and the voice-print model when it is not on the phone.
//  O16 The press that starts the voice setup records the person's agreement to the download (Wi-Fi, or mobile data when the phone is not on Wi-Fi).
import type { Storage } from './app.contract.d.ts';
import { complete, generalEdition, localWriter, FAILED_LINE, TIMED_OUT_LINE, type Edition, type Writer, editionLines } from './edition-writer.ts';
import { lookupEdition, seamGet, type Get } from './notebook.ts';
import { accountGeneration, nativeLater, writeCurrent } from './api.ts';
import * as pages from './pages.ts';
import { consentAgree, consentPicks } from './consent.ts';
import { picksFor, STAGE_JOBS, type OfferJob } from './offers.ts';
import { noEnrol, type Enrol } from './voicelearn.ts';
import { keyState } from './api.ts';

export const PROGRESS_FILE = 'app:/data/onboarding.json';
export const EDIT = 'onboarding:edit';
export const SETUP = 'onboarding:setup';

export type Row = { stage: string; name: string; model: string; sizeLabel: string; state: string; statusLine: string; numbers: string; progress: number; isFallback: boolean; canRetry: boolean; canFallback: boolean };
/** What the voice side reports. Only native code holds these. */
/** What voice needs from the network, job by job, as the model list gives it (native ModelOffer): see offers.ts. */
export type VoiceFacts = {
  rows: Row[]; summary: string[]; totalLabel: string; device: string; onWiFi: boolean; setupStarted: boolean; setupFinished: boolean;
  mic: string; level: number; heard: string; route: string; talking: boolean; voicePicked: number; finishAsked: number;
  offers: OfferJob[]; runStages: string[]; voiceReady: boolean; hearingReady: boolean; enrol: Enrol;
};
export const noVoice = (): VoiceFacts => ({ rows: [], summary: [], totalLabel: '', device: '', onWiFi: true, setupStarted: false, setupFinished: false, mic: 'unasked', level: 0, heard: '', route: '', talking: false, voicePicked: 0, finishAsked: 0, offers: [], runStages: [], voiceReady: false, hearingReady: false, enrol: noEnrol() });

export type Onboarding = {
  step: string; title: string; message: string; totalLabel: string; device: string; mic: string; heard: string; route: string; editionStatus: string; writingLine: string; editionError: string;
  editionTitle: string; editionIntro: string; editionAddress: string; card: string; cardBody: string; aim: string; askMobile: boolean; setupFinished: boolean; setupStarted: boolean;
  hearingOK: boolean; skipped: boolean; editionDone: boolean; level: number; rows: Row[]; summary: string[]; editionLines: string[]; talking: boolean; editing: boolean;
};

export type Progress = { offered: boolean; skipped: boolean; setupDone: boolean; micRefused: boolean; topicsStarted: boolean; editionDone: boolean; voicePicked: number };
const fresh = (): Progress => ({ offered: false, skipped: false, setupDone: false, micRefused: false, topicsStarted: false, editionDone: false, voicePicked: 0 });

/** O13: the kept progress, by shape; anything else is fresh. */
export function parseProgress(text: string | undefined): Progress {
  const p = fresh();
  if (!text) return p;
  try {
    const o = JSON.parse(text) as Record<string, unknown>;
    if (!o || typeof o !== 'object' || Array.isArray(o)) return p;
    for (const k of ['offered', 'skipped', 'setupDone', 'micRefused', 'topicsStarted', 'editionDone'] as const) if (o[k] === true) p[k] = true;
    if (typeof o.voicePicked === 'number' && Number.isFinite(o.voicePicked) && o.voicePicked >= 0) p.voicePicked = Math.floor(o.voicePicked);
  } catch { /* fresh */ }
  return p;
}

/** The step a run starts at, from what was kept (the Swift init). */
export function startStep(p: Progress): string {
  if (p.topicsStarted && !p.editionDone && !p.skipped) return 'topics';
  if (p.editionDone) return 'done';
  if (p.skipped) return 'skipped';
  if (p.setupDone) return 'ready';
  return 'welcome';
}

const TITLES: Record<string, string> = { welcome: 'Make it yours', setup: 'Set up your voice', mic: 'Can I hear you?', hearing: 'Can you hear me?', learn: 'Your voice', voice: 'Pick a voice', ready: 'Voice is ready',
  topics: 'Your edition', writing: 'Writing your edition', done: 'Your edition', skipped: 'Make it yours' };
export const NO_OFFERS_LINE = 'The voice files are not available just now. Try again in a moment.';
export const NOT_RECORDED_LINE = 'Your choice could not be saved just now, so nothing was downloaded. Try again.';
export const MIC_REFUSED_LINE = 'Microphone is off for Autoradio 1. You can turn it on in Settings.';
export const TALK_REFUSED_LINE = 'Talking to the radio needs the microphone. You can allow it in Settings, or tap topics instead.';
export const MIC_SILENT_LINE = 'I could not hear anything. The paper keeps working, and you can tap your topics instead.';
export const HEARING_AGAIN_LINE = 'Try the volume, or another speaker or headphones, then play it again.';

/** The narrow native calls, each with its reason (see the header). Null where there is no native module (the browser, Android until its package exists). */
export type Device = {
  facts(): Promise<VoiceFacts | null>;
  op(op: string, args?: Record<string, unknown>): Promise<void>;
};
export function nativeDevice(): Device | null {
  const raw = nativeLater();
  if (!raw) return null;
  const later = raw as unknown as (r: Record<string, unknown>) => Promise<unknown>;
  return {
    async facts() { const r = await later({ op: 'obFacts' }); return r && typeof r === 'object' && Array.isArray((r as VoiceFacts).rows) ? (r as VoiceFacts) : null; },
    async op(op, args = {}) { await later({ op, ...args }); },
  };
}

type Mem = { gen: number; loaded: boolean; loading?: Promise<void>; p: Progress; step: string; askMobile: boolean; message: string; hearingOK: boolean; edition: Edition | null; status: string; writingLine: string; error: string; mirrored: boolean | null; picked: number };
let mem: Mem | null = null;
/** The account generation of the first memory this process made: the launch's. */
let launchGen: number | undefined;
const blank = (): Mem => ({ gen: launchGen === undefined ? (launchGen = accountGeneration()) : accountGeneration(), loaded: false, p: fresh(), step: 'welcome', askMobile: false, message: '', hearingOK: false, edition: null, status: '', writingLine: '', error: '', mirrored: null, picked: 0 });
/** For tests: a new process. */
export function forgetOnboarding(): void { mem = null; }

export type Env = { storage: Storage | undefined; device: Device | null; get: Get; agree?: (mobile: boolean) => Promise<boolean | void>; deadlineSeconds?: number; writer?: Writer; closePage?: (address: string) => Promise<void>; openPage?: (address: string, title: string) => Promise<void> };
export const liveEnv = (storage: Storage | undefined): Env => ({ storage, device: nativeDevice(), get: seamGet,
  agree: async (mobile) => (await consentAgree('voice', mobile, storage)) && (await consentAgree('voice-rest', mobile, storage)),
  closePage: async (a) => { await pages.pagesClose(storage, a); }, openPage: async (a, t) => { await pages.pagesOpen(storage, a, t); } });

/** The progress, read once per account generation. THE CAUSE of the flip (found by reading, 2026-11-22): `loaded` used to be set BEFORE the file was read and a failed read counted as "no file", so a second
 *  caller met a memory that said loaded with the default progress (step `welcome`: not over) while the first was still reading, and a read that failed (an answer let go while it waited, `the proof's plain
 *  launch: pulse ready=true, then ready=false for good`) left that default for the life of the process. Now concurrent callers share one read, only a file that is truly absent is "no progress", and any other
 *  failure leaves the memory unloaded so the next call reads again. */
async function load(env: Env): Promise<Mem> {
  if (!mem || mem.gen !== accountGeneration()) mem = blank();          // the memory belongs to the account generation it was made in
  const m = mem;
  if (m.loaded) return m;
  if (!m.loading) {
    m.loading = (async () => {
      let text: string | undefined, failed = false;
      try { if (env.storage) text = new TextDecoder().decode(await env.storage.fs.readFile(PROGRESS_FILE)); }
      catch (e) {
        // A storage refusal carries the POSIX name in `code` (Exact js/src/storage.js:32-37): an absent file is "no progress", anything else is a failed read.
        if ((e as { code?: string } | null)?.code !== 'ENOENT') { failed = true; console.log(`onboarding: the progress file could not be read (${String((e as Error)?.message ?? e).slice(0, 120)}): not loaded, read again next time`); }
      }
      if (m.gen === accountGeneration() && !failed) { m.p = parseProgress(text); m.step = startStep(m.p); m.picked = m.p.voicePicked; m.loaded = true; }
      m.loading = undefined;
    })();
  }
  await m.loading;
  if (m.gen !== accountGeneration()) return load(env);                 // the account moved while the file was read: that text is not the new account's
  return m;
}
async function save(env: Env, m: Mem): Promise<void> {
  if (!env.storage || !m.loaded) return;                                // never write the default over progress that was not read
  await writeCurrent(m.gen, env.storage, PROGRESS_FILE, new TextEncoder().encode(JSON.stringify(m.p)));     // a refused or failed write leaves the step in memory; the next change writes again
}
/** Is the first run over (done or skipped)? The radio's start asks this itself: it must not wait for a page to ask `ob` (a plain launch has no driver whose dumps pull every resource in, and nothing else asked it). Also tells native. */
export async function firstRunOver(storage: Storage | undefined): Promise<boolean> {
  // Nobody signed in: there is no first run to finish before the radio plays the paper (listening needs no account; the voice download is a card, not a gate).
  try { if ((await keyState()).state === 'absent') return true; } catch { /* the first run decides */ }
  const env = liveEnv(storage);
  const m = await load(env);
  await mirror(env, m);
  return over(m);
}

/** Sign-out (account.ts WIPES): the memory, the file, and the voice side. Throws when the file could not be removed and is not empty. */
export async function wipeOnboarding(storage: Storage | undefined): Promise<void> {
  mem = null;
  try { await nativeDevice()?.op('obReset'); } catch { /* the native memory resets at its own wipe */ }
  if (!storage) return;
  try { await storage.fs.rm(PROGRESS_FILE); } catch (e) { try { await storage.fs.stat(PROGRESS_FILE); } catch { return; } throw e; }
}

const over = (m: Mem): boolean => m.step === 'done' || m.step === 'skipped';
/** O14: the station's gate on the native side follows the step. */
async function mirror(env: Env, m: Mem): Promise<void> {
  const f = over(m);
  if (m.mirrored === f) return;
  m.mirrored = f;
  try { await env.device?.op('obMirror', { finished: f }); } catch { m.mirrored = null; }
}

/** Fold what native reports into the step (O3, O4, O6). Idempotent. */
async function reconcile(env: Env, m: Mem, v: VoiceFacts): Promise<void> {
  let dirty = false;
  if (m.step === 'setup' && v.setupFinished && !m.askMobile) { m.p.setupDone = true; m.step = 'mic'; dirty = true; }
  if (v.mic === 'refused' && !m.p.micRefused) { m.p.micRefused = true; dirty = true; }
  if (v.mic === 'heard' && m.p.micRefused) { m.p.micRefused = false; dirty = true; }
  if (v.voicePicked > m.picked) { m.picked = v.voicePicked; if (m.step === 'voice') m.step = 'ready'; if (m.p.voicePicked !== v.voicePicked) { m.p.voicePicked = v.voicePicked; dirty = true; } }
  if (dirty) await save(env, m);
}

function cardFor(m: Mem, v: VoiceFacts): { card: string; cardBody: string } {
  if (m.p.editionDone || m.p.skipped) return { card: '', cardBody: '' };                                                 // O12
  if (m.p.topicsStarted) return { card: 'Choose your topics', cardBody: 'Pick up where you left off. What you chose is saved.' };
  if (v.setupStarted && !v.setupFinished) return { card: 'Finish voice setup', cardBody: 'Setting up the voice on this phone takes a few minutes.' };
  return { card: 'Make your edition', cardBody: 'Pick the topics you care about, then your paper is yours.' };
}

function view(m: Mem, v: VoiceFacts, top: string): Onboarding {
  let message = m.message;
  if (m.step === 'mic') { if (v.mic === 'refused') message = MIC_REFUSED_LINE; else if (v.mic === 'silent') message = MIC_SILENT_LINE; }
  const heard = v.mic === 'heard' ? (v.heard === '' ? 'Your microphone is working.' : v.heard) : '';
  const { card, cardBody } = cardFor(m, v);
  return {
    step: m.step, title: TITLES[m.step] ?? 'Make it yours', message, totalLabel: v.totalLabel, device: v.device, mic: v.mic, heard, route: v.route, editionStatus: m.status, writingLine: m.writingLine, editionError: m.error,
    editionTitle: m.edition?.title ?? 'My edition', editionIntro: m.edition?.intro ?? '', editionAddress: EDIT, card, cardBody, aim: m.p.editionDone ? '' : 'Make it yours',
    askMobile: m.askMobile, setupFinished: v.setupFinished, setupStarted: v.setupStarted, hearingOK: m.hearingOK, skipped: m.p.skipped, editionDone: m.p.editionDone, level: v.level, rows: v.rows, summary: v.summary,
    editionLines: m.edition ? editionLines(m.edition) : [], talking: v.talking, editing: top === EDIT,
  };
}

/** Source: the page's model. `voice` is the native facts; `top` the page on top (the edit page is `onboarding:edit`). */
export async function onboardingPage(voice: VoiceFacts | null | undefined, _tick: number, top: string, env: Env): Promise<Onboarding> {
  const m = await load(env);
  const v = { ...noVoice(), ...(voice ?? {}) };
  await reconcile(env, m, v);
  await mirror(env, m);
  return view(m, v, top);
}

async function facts(env: Env): Promise<VoiceFacts> { return { ...noVoice(), ...((await env.device?.facts().catch(() => null)) ?? {}) }; }
const dev = async (env: Env, op: string, args?: Record<string, unknown>): Promise<void> => { try { await env.device?.op(op, args); } catch { /* a native call that fails leaves the facts as they were */ } };

async function writeEdition(env: Env, m: Mem, topics: string[], edit: boolean): Promise<void> {
  if (m.status === 'writing') return;                                                                                       // O9
  if (!edit) m.step = 'writing';
  m.status = 'writing'; m.writingLine = 'Writing your edition…'; m.error = ''; m.message = '';
  const out = await complete(topics, env.writer ?? localWriter, (line) => { if (m.gen === accountGeneration()) m.writingLine = line; }, env.deadlineSeconds);
  if (m.gen !== accountGeneration()) return;                                                                                // another account now: nothing of this edition is theirs
  if (out.kind === 'written') {
    await ready(env, m, out.edition, edit);
    await dev(env, 'obNotebookTopics', { topics });                                                                        // the first lines of the notebook (native: Notebook.swift writes it)
  } else { m.status = 'failed'; m.error = out.kind === 'timedOut' ? TIMED_OUT_LINE : FAILED_LINE; }
}

async function ready(env: Env, m: Mem, e: Edition, edit: boolean): Promise<void> {
  m.edition = e; m.status = 'ready'; m.p.editionDone = true;
  if (!edit) m.step = 'done';
  await save(env, m);
  if (edit) await closeEdit(env);                                                                                           // O10
}
async function closeEdit(env: Env): Promise<void> { await env.closePage?.(EDIT); }

export type Op = 'begin' | 'retry' | 'fallback' | 'mic' | 'micNext' | 'hearing' | 'topics' | 'talk' | 'topicsDone' | 'editionRetry' | 'editDone' | 'editLeave' | 'continueAnyway' | 'skip' | 'offer' | 'finishAsked' | 'learnDone' | 'micSkip';

/** Mutation: one button. `arg` is the option of the op (mobile, stage, ok) and `topics` the page's list (one per line) on the done sends. The answer is the page after it. */
export async function onboardingAct(op: string, arg: string, topics: string, top: string, env: Env): Promise<Onboarding> {
  const m = await load(env);
  // A step must never be taken on progress that was not read (it could not be saved, and the person's input would be dropped): say so, change nothing, and the next press reads again.
  if (!m.loaded) { const f = await facts(env); return { ...view(m, f, top), message: 'Your progress could not be read just now. Try again.' }; }
  // The page sends the done ops only once its picks are loaded (`if ep.loaded`), so an empty text is an empty list: the general edition (E4).
  const picks = topics.split('\n').map((t) => t.trim()).filter((t) => t !== '');
  switch (op as Op) {
    case 'begin': {
      if (m.p.setupDone && m.step === 'skipped') { m.step = 'ready'; m.p.skipped = false; await save(env, m); break; }          // O7
      const f = await facts(env);
      if (m.step === 'setup' && f.setupStarted && !f.setupFinished) break;                                                    // O2: running
      if (f.offers.length === 0) { m.message = NO_OFFERS_LINE; break; }                                                       // O19: nothing was shown, so nothing is fetched
      if (!f.onWiFi && arg !== 'true') { m.step = 'setup'; m.askMobile = true; m.message = ''; break; }                      // O2
      // What was shown is what is fetched (O18, O19): the chosen option of every job of the two voice groups (the person's pick, else the one recommended for this phone), each as the one stage it runs in, and the voice-print model beside them. Never the configurator, never a stage that was not shown.
      let agreed = true;
      try { agreed = (await env.agree?.(!f.onWiFi)) !== false; } catch { agreed = false; }
      if (!agreed) { m.message = NOT_RECORDED_LINE; break; }                                                                   // the agreement is on disk before anything is fetched
      m.askMobile = false; m.step = 'setup'; m.message = ''; m.p.skipped = false;
      await save(env, m);
      const picks = picksFor(f.offers, ['voice', 'voice-rest'], await consentPicks(env.storage));
      const stages = STAGE_JOBS.filter((j) => j in picks);
      const speaker = f.offers.some((j) => j.job === 'speaker' && j.options.some((o) => !o.ready));
      await dev(env, 'obSetupBegin', { only: true, stages, picks, speaker });
      break;
    }
    case 'retry': if (m.step === 'setup') await dev(env, 'obSetupRetry', { stage: arg }); break;
    case 'fallback': if (m.step === 'setup') await dev(env, 'obSetupFallback', { stage: arg }); break;
    case 'mic': if (m.step === 'mic') { m.message = ''; await dev(env, 'obMicStart'); } break;
    case 'micNext': if (m.step === 'mic') { m.step = 'hearing'; m.hearingOK = false; m.message = ''; await dev(env, 'obHearingPlay'); } break;
    case 'hearing':
      if (m.step !== 'hearing') break;
      if (arg === 'true') { m.hearingOK = true; m.step = 'learn'; m.p.setupDone = true; m.message = ''; await save(env, m); }                // O5
      else { m.message = HEARING_AGAIN_LINE; await dev(env, 'obHearingPlay'); }
      break;
    case 'learnDone': if (m.step === 'learn') { await dev(env, 'obVoiceStop'); m.step = 'voice'; m.message = ''; await save(env, m); } break;                      // learned, or not now: either way on
    case 'micSkip': if (m.step === 'mic') { await dev(env, 'obMicStop'); m.step = 'voice'; m.p.setupDone = true; m.message = ''; await save(env, m); } break;       // Not now at the microphone: nothing is asked, the tap path stays
    case 'topics': m.step = 'topics'; m.p.topicsStarted = true; m.p.skipped = false; m.message = ''; await save(env, m); break;
    case 'talk': {
      const f = await facts(env);
      if (f.talking) { await dev(env, 'obTalk', { on: false }); break; }
      if (m.p.micRefused) { m.message = TALK_REFUSED_LINE; break; }                                                           // O4
      m.message = ''; await dev(env, 'obTalk', { on: true });
      break;
    }
    case 'finishAsked':
    case 'topicsDone': if (m.step !== 'topics') break; await dev(env, 'obTalk', { on: false }); await writeEdition(env, m, picks, false); break;
    case 'editDone': await dev(env, 'obTalk', { on: false }); await writeEdition(env, m, picks, true); break;
    case 'editionRetry': await writeEdition(env, m, picks, top === EDIT); break;
    case 'editLeave': {
      await dev(env, 'obTalk', { on: false });
      m.status = m.edition || m.p.editionDone ? 'ready' : ''; m.error = '';
      await closeEdit(env);
      break;
    }
    case 'continueAnyway': await ready(env, m, generalEdition(), top === EDIT); break;
    case 'skip': await dev(env, 'obStop'); m.p.skipped = true; m.step = 'skipped'; await save(env, m); break;
    case 'offer': if (arg === 'signedOut' && m.gen !== launchGen) { console.log('onboarding: no offer: signed out after the launch'); break; } await offer(env, m); break;      // signed out, the first run is offered at launch only: after a sign-out it would be written again right after the wipe
    default: break;
  }
  const f = await facts(env);
  await reconcile(env, m, f);
  await mirror(env, m);
  return view(m, f, top);
}

/** O11: a fresh install, signed in: ask the computer first, then offer the path once as an open page. */
async function offer(env: Env, m: Mem): Promise<void> {
  if (m.p.offered || m.p.editionDone || m.p.skipped) return;
  const ed = await lookupEdition(env.get);
  if (m.gen !== accountGeneration()) return;
  if (ed?.found) { m.p.editionDone = true; m.step = 'done'; await save(env, m); return; }
  await env.openPage?.(SETUP, 'Make it yours');                       // opened first, kept offered after: an offer that never opened must be made again
  if (m.gen !== accountGeneration()) { console.log('onboarding: the offer was dropped, the account moved while the page opened'); return; }
  m.p.offered = true; await save(env, m);
  console.log('onboarding: the first run was offered as an open page');
}
