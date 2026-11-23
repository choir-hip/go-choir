// One sign-in attempt is one state machine with an id, owned here and nowhere else (docs/reference/exact/auth-session.md has the transition table this file is). Three review rounds
// found the same fault three ways (a stale "pending" abandoning an approved key, a Cancel followed by a key write, a re-opened sheet still carrying a close flag), because the state
// was spread over a contract status, a result slot, a close flag and a native poll loop. Now every event names its attempt; an event for another attempt, or after an end, changes nothing.
//
// What is native, and why only that: the device code, the one poll request, and the Keychain write (the code and the key must not pass through shared code; the key's item is written by native
// code, account-boundary design). Three calls on the app module, each naming the attempt: `signInBegin`, `signInPoll`, `signInForget`. Native has no loop, no timer and no status of its own.
//
// The callback of the sign-in sheet is a hint and is never read (signin-sheet.ts). Closing the sheet is a hint too: it abandons nothing by itself, only a poll that STARTED AFTER the close
// and still answers "pending" does. Nothing here names a key: native answers a word.
import { sheetOutcomeNote } from './signin-sheet.ts';
import type { SheetResult } from './signin-sheet.ts';

export type Phase = 'idle' | 'begun' | 'sheet' | 'closed' | 'approved' | 'cancelled' | 'denied' | 'expired' | 'failed';
export type Answer = 'pending' | 'slow_down' | 'error' | 'approved' | 'denied' | 'expired' | 'stale' | 'invalid';
export type Attempt = { id: string; phase: Phase; userCode: string; link: string; note: string; message: string; issued: number; closedAt: number };
export type Event =
  | { t: 'begun'; id: string; userCode: string; link: string }
  | { t: 'beginFailed'; id: string; message: string }
  | { t: 'sheet'; id: string; outcome: 'returned' | 'cancelled' | 'failed'; note: string }
  | { t: 'reopen'; id: string }
  | { t: 'cancel'; id: string }
  | { t: 'issued'; id: string; n: number }
  | { t: 'answer'; id: string; n: number; answer: Answer }
  | { t: 'timeout'; id: string };

const WORDS = {
  denied: 'the sign-in was denied',
  expired: 'the code expired; sign in again',
  lost: 'the sign-in could not continue; sign in again',
};

export const isTerminal = (p: Phase): boolean => p === 'approved' || p === 'cancelled' || p === 'denied' || p === 'expired' || p === 'failed';
export const isPending = (p: Phase): boolean => p === 'begun' || p === 'sheet' || p === 'closed';

export const begin = (id: string): Attempt => ({ id, phase: 'begun', userCode: '', link: '', note: '', message: '', issued: 0, closedAt: -1 });

/** The whole table. Pure. */
export function step(a: Attempt, e: Event): Attempt {
  if (e.id !== a.id || isTerminal(a.phase) || a.phase === 'idle') return a;
  const to = (phase: Phase, more: Partial<Attempt> = {}): Attempt => ({ ...a, phase, ...more });
  switch (e.t) {
    case 'begun': return a.phase === 'begun' ? to('sheet', { userCode: e.userCode, link: e.link }) : a;
    case 'beginFailed': return a.phase === 'begun' ? to('failed', { message: e.message }) : a;
    case 'issued': return to(a.phase, { issued: Math.max(a.issued, e.n) });
    case 'cancel': return to('cancelled');
    case 'timeout': return a.phase === 'begun' ? a : to('expired', { message: WORDS.expired });
    case 'reopen': return a.phase === 'sheet' || a.phase === 'closed' ? to('sheet', { closedAt: -1, note: '' }) : a;
    case 'sheet':
      if (a.phase !== 'sheet') return a;
      if (e.outcome === 'cancelled') return to('closed', { closedAt: a.issued });
      return e.outcome === 'failed' ? to('sheet', { note: e.note }) : a;
    case 'answer':
      if (a.phase !== 'sheet' && a.phase !== 'closed') return a;
      switch (e.answer) {
        case 'approved': return to('approved');
        case 'denied': return to('denied', { message: WORDS.denied });
        case 'expired': return to('expired', { message: WORDS.expired });
        case 'stale': case 'invalid': return to('failed', { message: WORDS.lost });
        case 'pending': return a.phase === 'closed' && e.n > a.closedAt ? to('cancelled') : a;
        default: return a;                                            // slow_down, error: decide nothing
      }
  }
}

// ---- what the card shows

/** `rev` counts the machine's published changes: the card's resource is keyed on it (and on every press's answer), so the LAST change is always read. */
export type View = { phase: Phase; pending: boolean; userCode: string; browserLink: string; note: string; message: string; rev: number };

/** The link the card may offer in the system browser: https on the one granted origin, no userinfo, and only when the sheet could not open. */
export function browserLink(link: string, origin: string): string {
  try { const u = new URL(link); return u.protocol === 'https:' && u.origin === origin && u.username === '' && u.password === '' ? link : ''; } catch { return ''; }
}

export function view(a: Attempt | null, origin: string, rev = 0): View {
  if (!a) return { phase: 'idle', pending: false, userCode: '', browserLink: '', note: '', message: '', rev };
  const showing = isPending(a.phase);
  return { phase: a.phase, pending: showing, userCode: showing ? a.userCode : '', browserLink: showing && a.note !== '' ? browserLink(a.link, origin) : '', note: showing ? a.note : '', message: a.message, rev };
}

// ---- the driver: the one place the machine lives and the events come from

export type NativeRequest = { op: 'signInBegin' | 'signInPoll' | 'signInForget'; attempt: string };
export type AttemptEnv = {
  origin: string;
  later(request: NativeRequest): Promise<unknown>;
  sheet(link: string): Promise<Pick<SheetResult, 'outcome' | 'message'>>;
  sleep(ms: number): Promise<void>;
  newId(): string;
  log(line: string): void;
  /** Told the view after EVERY change (see `publish`); a test hears it, the card reads `signInView`. */
  publish?(view: View): void;
};

let cur: Attempt | null = null;
let rev = 0;
export const resetAttempt = (): void => { cur = null; rev = 0; sheetOf = null; };
export const signInView = (origin: string): View => view(cur, origin, rev);

/** THE one place that publishes: every change of the attempt (a new attempt, an accepted event) goes through here and nowhere else, the last one of an attempt included. An event the table
 * ignores changes nothing and says nothing. The card is keyed on `rev`, and every press answers with the `rev` it left behind, so the final phase is read as surely as the first. */
function publish(env: AttemptEnv): void {
  rev += 1;
  env.publish?.(view(cur, env.origin, rev));
}
function adopt(env: AttemptEnv, next: Attempt): void {
  const phase = cur?.phase;
  cur = next;
  if (next.phase !== phase) env.log(`signin: attempt ${next.id.slice(0, 6)} ${next.phase}`);
  publish(env);
}
function dispatch(env: AttemptEnv, e: Event): void {
  if (!cur) return;
  const next = step(cur, e);
  if (next === cur) return;
  if (e.t === 'issued') cur = next;                 // the poll counter is the machine's own bookkeeping, not part of what the card shows: nothing to publish
  else adopt(env, next);
}
const left = (phase: Phase): { phase: Phase; rev: number } => ({ phase, rev });
const own = (id: string): boolean => cur !== null && cur.id === id;
const ask = async (env: AttemptEnv, op: NativeRequest['op'], attempt: string): Promise<Record<string, unknown>> => {
  try { const r = await env.later({ op, attempt }); return r && typeof r === 'object' ? (r as Record<string, unknown>) : {}; } catch { return {}; }
};
const answerOf = (r: Record<string, unknown>): Answer => {
  const a = r.answer;
  return a === 'pending' || a === 'slow_down' || a === 'approved' || a === 'denied' || a === 'expired' || a === 'stale' || a === 'invalid' ? a : 'error';
};

/** Open the sheet and tell the machine how it ended. The caller keeps this promise inside its own answer: a mutation stays until its work is done (docs/reference/exact/answers-let-go.md). */
let sheetOf: string | null = null;           // the attempt whose sheet is open now: one sheet at a time
async function openSheet(env: AttemptEnv, id: string, link: string): Promise<void> {
  sheetOf = id;
  let r: Pick<SheetResult, 'outcome' | 'message'>;
  try { r = await env.sheet(link); } catch { r = { outcome: 'failed', message: sheetOutcomeNote }; }
  if (sheetOf === id) sheetOf = null;
  dispatch(env, { t: 'sheet', id, outcome: r.outcome, note: r.outcome === 'failed' ? r.message : '' });
}

/** Start an attempt and run it to its end: begin, open the sheet, poll. Returns the phase it ended in. BEGIN while an attempt is still running (begun, sheet or closed) SUPERSEDES it: the older
 * one is cancelled and forgotten and the newest wins (a double tap; also what the contract's newest-wins mutation expects). An attempt that already ended is NOT forgotten (an approved one owns the key). */
export async function signInRun(env: AttemptEnv): Promise<{ phase: Phase; rev: number }> {
  if (cur && !isTerminal(cur.phase)) { const old = cur.id; dispatch(env, { t: 'cancel', id: old }); await ask(env, 'signInForget', old); }
  let id: string;
  try { id = env.newId(); } catch { adopt(env, step(begin('none'), { t: 'beginFailed', id: 'none', message: WORDS.lost })); return left(cur!.phase); }
  adopt(env, begin(id));
  const b = await ask(env, 'signInBegin', id);
  if (!own(id) || isTerminal(cur!.phase)) { await ask(env, 'signInForget', id); return left('cancelled'); }          // cancelled or replaced while native was beginning: its late code must not stay live
  if (b.ok !== true || typeof b.link !== 'string') { dispatch(env, { t: 'beginFailed', id, message: typeof b.message === 'string' && b.message ? b.message : WORDS.lost }); return left(cur!.phase); }
  dispatch(env, { t: 'begun', id, userCode: typeof b.userCode === 'string' ? b.userCode : '', link: b.link });
  const sheetEnd = openSheet(env, id, b.link);
  const interval = Math.max(1, typeof b.interval === 'number' ? b.interval : 5) * 1000;
  const deadline = (typeof b.expires === 'number' ? b.expires : 600) * 1000 + 5000;
  let wait = interval;
  let waited = 0;             // Exact has no clock in shared code: the time is the sum of the waits
  const poll = async (): Promise<void> => {
  while (own(id) && !isTerminal(cur!.phase)) {
    await env.sleep(wait); waited += wait;
    if (!own(id) || isTerminal(cur!.phase)) break;
    if (waited > deadline) { dispatch(env, { t: 'timeout', id }); break; }
    const n = cur!.issued + 1;
    dispatch(env, { t: 'issued', id, n });
    const answer = answerOf(await ask(env, 'signInPoll', id));
    dispatch(env, { t: 'answer', id, n, answer });
    if (answer === 'slow_down' || answer === 'error') wait += 5000;
  }
  if (own(id) && cur!.phase !== 'approved') await ask(env, 'signInForget', id);        // an end without a key: native drops the code (idempotent)
  };
  await Promise.all([poll(), sheetEnd]);                                                // the answer stays until the sheet has ended too: its call belongs to this answer
  return left(own(id) ? cur!.phase : 'cancelled');
}

/** The card's Cancel. The machine moves first (so every later answer is ignored), then native forgets the code: no key REMAINS after it (one written in the gap is deleted and revoked, one that arrives later is never written). */
export async function signInCancel(env: AttemptEnv): Promise<{ phase: Phase; rev: number }> {
  const a = cur;
  if (!a || isTerminal(a.phase)) return left(a ? a.phase : 'idle');
  dispatch(env, { t: 'cancel', id: a.id });
  await ask(env, 'signInForget', a.id);
  return left(own(a.id) ? cur!.phase : 'cancelled');
}

/** "Open the approval page": the sheet again, and the close is forgotten. The answer stays until that sheet ends. */
export async function signInReopen(env: AttemptEnv): Promise<{ phase: Phase; rev: number }> {
  const a = cur;
  if (!a || (a.phase !== 'sheet' && a.phase !== 'closed') || sheetOf === a.id) return left(a ? a.phase : 'idle');          // not while this attempt's sheet is already up
  dispatch(env, { t: 'reopen', id: a.id });
  await openSheet(env, a.id, a.link);
  return left(own(a.id) ? cur!.phase : 'cancelled');
}
