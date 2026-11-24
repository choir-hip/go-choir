// Learning the person's voice, the page's side (owner, 2026-11-24: "we should learn user voice in voice onboarding"; and: everything that can be Exact is Exact). THE STATE MACHINE IS HERE: which sentence is next, how many takes there are,
// what the page shows and when it says "stored". Native gives only the capture of one sentence (the microphone, ending the sentence on a pause) and three calls on the embedding model (keep a take, build the profile in memory, compare
// the confirming sentence with it) plus the store; see AutoradioKit Voice/EnrolCapture.swift. Nothing leaves the phone. In this build the later check only LOGS its verdict per turn and rejects nothing.
//
// The profile is stored only after the confirming sentence MATCHED; it is built in memory before that and dropped when the person leaves, or the check is unsure, or the build fails.
//
// Failure list (each line has a test in voicelearn.test.ts):
//  Y1 The sentences are in a fixed order, three to learn from and one to confirm; the page shows the next, never the confirming one before the voice is built.
//  Y2 Nothing opens the microphone before the person presses Start or Read it; the first press shows the pre-prompt line first (Continue, Not now), only when the microphone has not been asked.
//  Y3 A sentence that was not heard well enough is asked again with a line saying so; nothing is kept for it.
//  Y4 "Stored" is said only after native stored the profile on a match; an unsure check or a failed build says so, drops everything in progress, and offers to learn again.
//  Y5 No model on the phone: the page says so and presses start nothing.
//  Y6 Learn again, Not now, leaving and sign-out drop everything in progress (native `obEnrolDiscard`); Forget my voice deletes the stored one.
//  Y7 A refused microphone shows the declined line; a capture that failed says so with a way to try again or leave; a take is never stuck without a Not now.
//  Y8 One native step at a time: a second look at the facts while a step runs starts none.
//  Y9 A sign-out forgets the state (the next account does not see a "stored" it never made).
import { accountGeneration, nativeLater } from './api.ts';

/** What native reports about the capture. `state`: idle | listening | done | error. */
export type Enrol = { state: string; level: number; usable: boolean; kept: number; built: boolean; ready: boolean; reason: string };
export const noEnrol = (): Enrol => ({ state: 'idle', level: 0, usable: false, kept: 0, built: false, ready: false, reason: '' });

export const NEEDED = 3;
export const SENTENCES = [
  'Good morning, this is my voice on the radio.',
  'Play me the news about the weather and the local team.',
  'Skip this one and read me the next story.',
] as const;
export const CONFIRM_SENTENCE = 'Turn the volume down a little, please.';

export const MIC_PRE_LINE = 'To talk to the radio or record a take, Autoradio 1 needs your microphone and speech recognition.';
export const MIC_OFF_LINE = 'Microphone is off for Autoradio 1. You can turn it on in Settings.';

type Phase = 'idle' | 'listening' | 'building' | 'confirm' | 'confirming' | 'stored' | 'unsure' | 'failed';
type Mem = { gen: number; phase: Phase; taken: number; again: boolean; busy: boolean; started: boolean; why: string };
let mem: Mem | null = null;
const cur = (): Mem => { if (!mem || mem.gen !== accountGeneration()) mem = { gen: accountGeneration(), phase: 'idle', taken: 0, again: false, busy: false, started: false, why: '' }; return mem; };
/** Y9: sign-out (account.ts WIPES), and for tests: a new process. */
export function forgetLearn(): void { mem = null; }
export async function wipeLearn(): Promise<void> { const later = nativeLater(); mem = null; try { await later?.({ op: 'obEnrolDiscard' }); } catch { /* native resets at its own wipe */ } }

type Later = ((r: { op: string; [k: string]: unknown }) => Promise<unknown>) | null;
type Facts = { enrol?: Enrol; mic?: string } | null | undefined;
const call = async (later: Later, op: string): Promise<Record<string, unknown>> => { try { const r = await later?.({ op }); return r && typeof r === 'object' ? (r as Record<string, unknown>) : {}; } catch { return {}; } };

export type Learn = {
  step: string; title: string; line: string; sentence: string; count: string; notice: string; level: number;
  primary: string; primaryOp: string; secondary: string; busy: boolean; stored: boolean;
};

/** The page, from the state and what native reports. Pure. */
export function learnView(m: { phase: string; taken: number; again: boolean; started: boolean; why: string }, f: { enrol: Enrol; mic: string }): Learn {
  const e = f.enrol, n = Math.min(m.taken, NEEDED);
  const base: Learn = { step: 'intro', title: 'Teach the radio your voice', line: '', sentence: '', count: '', notice: '', level: e.level, primary: '', primaryOp: '', secondary: '', busy: false, stored: false };
  if (m.phase === 'stored') return { ...base, step: 'stored', title: 'Your voice is stored', line: 'Kept on this phone only, never uploaded. You can forget it or learn it again in Settings.', stored: true, primary: 'Done', primaryOp: 'close' };       // Y4
  if (!e.ready) return { ...base, step: 'noModel', line: 'The voice model is not on this phone yet. It comes with the downloads, in Settings.', secondary: 'Not now' };                                  // Y5
  if (f.mic === 'refused') return { ...base, step: 'refused', line: MIC_OFF_LINE, secondary: 'Not now' };                                                                                    // Y7
  const reading = (op: string): Learn => ({ ...base, step: 'reading', title: 'Read this aloud', sentence: SENTENCES[Math.min(n, SENTENCES.length - 1)], count: `Sentence ${n + 1} of ${NEEDED}`, notice: m.again ? 'I did not hear that. Read it again.' : '', primary: 'Read it', primaryOp: op, secondary: 'Not now' });
  switch (m.phase) {
    case 'listening': return { ...reading(''), line: 'Listening. Stop at the end of the sentence.', primary: '', busy: true };                                                                    // Y7: Not now stays
    case 'building': return { ...base, step: 'building', title: 'Learning your voice', line: 'One moment.', busy: true };
    case 'confirm': return { ...base, step: 'confirm', title: 'One more, to check it', sentence: CONFIRM_SENTENCE, line: 'Read this sentence, and the radio will say whether it knows you.', notice: m.again ? 'I did not hear that. Read it again.' : '', primary: 'Read it', primaryOp: 'read', secondary: 'Not now' };
    case 'confirming': return { ...base, step: 'confirming', title: 'One more, to check it', sentence: CONFIRM_SENTENCE, line: 'Listening.', busy: true, secondary: 'Not now' };
    case 'unsure': return { ...base, step: 'unsure', title: 'Not sure that was you', line: 'The radio could not tell. Learn your voice again, in a quiet place.', primary: 'Learn again', primaryOp: 'again', secondary: 'Not now' };
    case 'failed': return { ...base, step: 'failed', title: 'That did not work', line: m.why || 'The three sentences did not sound like one voice, or were too quiet. Try again in a quiet place.', primary: 'Learn again', primaryOp: 'again', secondary: 'Not now' };
    default:
      if (n === 0 && !m.again && !m.started) return { ...base, line: `Read ${NEEDED} short sentences out loud, about ten seconds in all. Kept on this phone only, never uploaded.${f.mic === 'unasked' ? ' ' + MIC_PRE_LINE : ''}`, primary: f.mic === 'unasked' ? 'Continue' : 'Start', primaryOp: 'start', secondary: 'Not now' };
      return reading('read');                                                                                                                                                                 // Y1, Y3
  }
}

/** What a take that ended did, once, in order (Y3, Y4, Y8). Native has already closed the microphone. */
async function advance(m: Mem, f: { enrol: Enrol }, later: Later): Promise<void> {
  if (m.busy || !later) return;
  const s = f.enrol.state;
  if ((m.phase === 'listening' || m.phase === 'confirming') && s === 'error') { m.why = f.enrol.reason || 'The microphone could not be used.'; m.phase = 'failed'; await call(later, 'obEnrolDiscard'); return; }
  if (!((m.phase === 'listening' || m.phase === 'confirming') && s === 'done')) return;
  m.busy = true;
  try {
    const confirming = m.phase === 'confirming';
    if (!f.enrol.usable) { m.again = true; m.phase = confirming ? 'confirm' : 'idle'; await call(later, 'obEnrolDiscardTake'); return; }                            // Y3
    m.again = false;
    if (confirming) {
      const r = await call(later, 'obEnrolCheck');
      if (r.call === 'match' && (await call(later, 'obEnrolStore')).ok === true) { m.phase = 'stored'; return; }                                                                         // Y4
      await call(later, 'obEnrolDiscard'); m.phase = 'unsure'; m.taken = 0; return;
    }
    const kept = (await call(later, 'obEnrolKeep')).kept;
    m.taken = typeof kept === 'number' ? kept : m.taken + 1;
    if (m.taken < NEEDED) { m.phase = 'idle'; return; }
    m.phase = 'building';
    if ((await call(later, 'obEnrolBuild')).ok === true) m.phase = 'confirm';
    else { await call(later, 'obEnrolDiscard'); m.phase = 'failed'; m.why = ''; m.taken = 0; }                                                                                            // Y4
  } finally { m.busy = false; }
}

/** Source: the page from the native facts. Looking at the facts is also what moves the machine on when a take has ended. */
export async function learnPage(f: Facts, later: Later = nativeLater() as Later): Promise<Learn> {
  const m = cur(); const facts = { enrol: { ...noEnrol(), ...(f?.enrol ?? {}) }, mic: f?.mic ?? 'unasked' };
  await advance(m, facts, later);
  return learnView(m, facts);
}

/** One press. `start` and `again` drop anything in progress and listen to the first sentence; `read` listens to the next; `rewind` opens the page fresh (Settings, Learn again); `forget` deletes the stored voice; `close` leaves. Y2, Y5, Y6, Y7. */
export async function learnAct(op: string, f: Facts, later: Later = nativeLater() as Later): Promise<Learn> {
  const m = cur(); const facts = { enrol: { ...noEnrol(), ...(f?.enrol ?? {}) }, mic: f?.mic ?? 'unasked' };
  const v = learnView(m, facts);
  if (later && !m.busy) {
    if (op === 'rewind') { Object.assign(m, { phase: 'idle', taken: 0, again: false, started: false, why: '' }); await call(later, 'obEnrolDiscard'); }
    else if (op === 'forget') { Object.assign(m, { phase: 'idle', taken: 0, again: false, started: false, why: '' }); await call(later, 'obVoiceForget'); }
    else if (op === 'close') { if (m.phase !== 'stored') Object.assign(m, { phase: 'idle', taken: 0, again: false, started: false, why: '' }); await call(later, 'obEnrolDiscard'); }
    else if ((op === 'start' && v.primaryOp === 'start') || (op === 'again' && v.primaryOp === 'again')) {
      Object.assign(m, { phase: 'listening', taken: 0, again: false, started: true, why: '' }); await call(later, 'obEnrolDiscard'); await call(later, 'obEnrolBegin');
    } else if (op === 'read' && v.primaryOp === 'read') { m.phase = m.phase === 'confirm' ? 'confirming' : 'listening'; await call(later, 'obEnrolBegin'); }
  }
  return learnView(m, facts);
}
