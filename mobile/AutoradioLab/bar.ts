// The bar's page model, in shared code (it was Swift: BarState.swift `BarTracker.barInfo` / `baseInfo` / `decorate` and the run sheet overlay in QueueView.swift).
// What stays native, and why: the FACTS. The voice engine's phase and transcript, the engine's events (holding line, revision, error: stamped with the device's clock), the
// player's own state (playing, title, the audio's place in the block) and the run sheet player's state (what is on air, held for the audio session, remote audio) are only
// known to the native audio code, so the native source `barFacts` hands them over as they are and decides nothing. Everything the bar says is decided here:
// the state, the words, the timed windows, the progress, the answer card, the overlay. The conversation's entries come from `conversationEntries` (conversation.ts's input).
//
// The only memory here is the player's message: it is new once (it holds the bar for the error window), so the first time it is seen is kept. It is keyed on
// the account generation (api.ts): a sign-out forgets it, so one account's words never hold the next account's bar. Nothing is stored in a file.
//
// Ways this can fail (each is a test in bar.test.ts, B1 to B11): see the list at the top of that file.
import { accountGeneration } from './api.ts';

export const ERROR_WINDOW = 4;      // seconds an error holds the bar
export const REVISED_WINDOW = 6;    // seconds a revision holds the bar
export const PICK_A_STORY = 'Pick a story below.';

export type BarInfo = {
  state: string; text: string; playing: boolean; talking: boolean; signedIn: boolean; handsFree: boolean; progress: number;
  question: string; answerText: string; answerLabel: string; answerState: string; answerNote: string; canRetry: boolean;
  /** Seconds until a timed state ends (0: none), so the contract asks again then and not before. */
  until: number;
};

const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);
const bool = (v: unknown): boolean => v === true;

// The player's message, as first seen: its words, when, and under which account.
let seen = { gen: -1, message: '', at: 0 };
/** Forget what the bar was holding (tests; a sign-out does it by moving the account generation). */
export function forgetBar(): void { seen = { gen: -1, message: '', at: 0 }; }

/** A plain line for an engine error, by the stage that failed. */
function plain(stage: string, message: string): string {
  if (message.includes('not signed in')) return 'Sign in to talk to your computer.';
  if (message.includes('429')) return 'Your computer is busy right now. Try again in a moment.';
  if (message.includes('network') || message.includes('timed out')) return 'Could not reach choir.news. Check your connection.';
  switch (stage) {
    case 'system1': return 'Your computer was slow to understand that; carrying on.';
    case 'desk': return 'Your computer could not act on that. Try asking again.';
    case 'voice': return 'The voice stopped. Tap play to carry on.';
    default: return 'Something went wrong with the radio.';
  }
}

/** The bar for the native facts and the conversation's entries. The clock is the facts' own `at` (seconds, the device's clock, the same one that stamps `errorAt` and `revisedAt`):
 * a data source may not read the time, and the facts are read again each half second while a timed state shows (spine.contract). `now` overrides it (tests). */
export function barInfo(rawFacts: unknown, rawEntries: unknown, now?: number): BarInfo {
  const f = obj(rawFacts);
  now = now ?? num(f.at);
  const b: BarInfo = { state: 'idle', text: '', playing: bool(f.playing), talking: false, signedIn: bool(f.signedIn), handsFree: bool(f.handsFree), progress: 0, question: '', answerText: '', answerLabel: '', answerState: '', answerNote: '', canRetry: false, until: 0 };
  const blockCount = Math.max(0, Math.floor(num(f.blockCount)));
  if (blockCount > 0) b.progress = Math.min(1, Math.max(0, (Math.max(0, num(f.blockIndex)) + Math.max(0, num(f.blockFraction))) / blockCount));

  // The player's message is new once (and again when it went and came back, or under another account).
  const gen = accountGeneration();
  const message = str(f.message);
  if (seen.gen !== gen) seen = { gen, message: '', at: 0 };
  if (message === '') seen.message = '';
  else if (message !== seen.message && message !== PICK_A_STORY) seen = { gen, message, at: now };
  // The latest of the engine's error (or a line the app said) and the player's message holds the bar.
  let errorAt = num(f.errorAt) > 0 ? num(f.errorAt) : 0;
  let errorText = errorAt > 0 ? (str(f.errorStage) === '' ? str(f.errorMessage) : plain(str(f.errorStage), str(f.errorMessage))) : '';
  if (seen.message !== '' && seen.at >= errorAt) { errorAt = seen.at; errorText = seen.message; }
  const revisedAt = num(f.revisedAt);
  const left = (since: number, window: number): number => (since > 0 ? window - (now - since) : 0);
  const ends = [left(errorAt, ERROR_WINDOW), left(revisedAt, REVISED_WINDOW)].filter((r) => r > 0);
  b.until = ends.length ? Math.min(...ends) + 0.1 : 0;

  const phase = str(f.phase), transcript = str(f.transcript);
  if (left(errorAt, ERROR_WINDOW) > 0) { b.state = 'error'; b.text = errorText; }
  else if (phase === 'candidate') { b.state = 'listening'; b.text = transcript || 'Listening…'; }
  else if (phase === 'addressed' || phase === 'capturing') { b.state = 'talking'; b.talking = true; b.text = transcript || 'Listening…'; }
  else if (phase === 'ended') { b.state = 'heard'; b.text = transcript || 'Heard you'; }
  else if (phase === 'acknowledging' || phase === 'holding') { b.state = 'holding'; b.text = str(f.holding) || 'On it…'; }
  else if (phase === 'revised' || left(revisedAt, REVISED_WINDOW) > 0) { b.state = 'revised'; b.text = 'Revised · Show'; }
  else if (b.playing) { b.state = 'playing'; b.text = str(f.title); }
  else if (str(f.title) !== '' && blockCount > 0 && bool(f.running)) { b.state = 'paused'; b.text = str(f.title); }

  // The run sheet's overlay (what is on air, held for the audio session, remote audio shown as the radio).
  const remote = bool(f.remote), onAir = f.onAir !== false;
  if (bool(f.waiting) && !remote && b.state === 'playing') { b.state = 'paused'; b.playing = false; }
  else if (!onAir && !remote && (b.state === 'playing' || b.state === 'paused')) { b.state = 'idle'; b.text = ''; b.playing = false; b.progress = 0; }
  else if (remote && (b.state === 'idle' || b.state === 'playing' || b.state === 'paused')) {
    const p = bool(f.remotePlaying);
    b.state = p ? 'playing' : 'paused'; b.text = str(f.remoteTitle); b.playing = p; b.progress = Math.min(1, Math.max(0, num(f.remoteFraction)));
  }

  // The answer card: the latest entry that is not cancelled, unless it was dismissed.
  const list = Array.isArray(obj(rawEntries).entries) ? (obj(rawEntries).entries as unknown[]) : [];
  const dismissed = str(obj(rawEntries).dismissed);
  let e: Record<string, unknown> | undefined;
  for (let i = list.length - 1; i >= 0; i--) { const c = obj(list[i]); if (Object.keys(c).length && str(c.status) !== 'cancelled') { e = c; break; } }
  if (e && str(e.id) !== dismissed) {
    const first = str(e.first), source = str(e.firstSource), answer = str(e.answer), note = str(e.note), status = str(e.status);
    const saved = source === '' ? first : `${source}: ${first}`;
    b.question = str(e.question);
    if (status === 'failed') { b.answerState = 'failed'; b.answerNote = note; b.canRetry = true; }
    else if (status === 'answered' || status === 'answering') {
      if (answer !== '') { b.answerState = 'full'; b.answerText = answer; } else { b.answerState = 'first'; b.answerText = saved; b.answerLabel = 'From your saved stories'; }
      b.answerNote = note; b.canRetry = status === 'answered' && note !== '';
    } else if (first === '') b.answerState = 'working';
    else { b.answerState = 'first'; b.answerText = saved; b.answerLabel = 'From your saved stories'; }
  }
  return b;
}
