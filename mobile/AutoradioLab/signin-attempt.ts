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
    case 'timeout': return to('expired', { message: WORDS.expired });
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

export type View = { phase: Phase; pending: boolean; userCode: string; browserLink: string; note: string; message: string };

/** The link the card may offer in the system browser: https on the one granted origin, no userinfo, and only when the sheet could not open. */
export function browserLink(link: string, origin: string): string {
  try { const u = new URL(link); return u.protocol === 'https:' && u.origin === origin && u.username === '' && u.password === '' ? link : ''; } catch { return ''; }
}

export function view(a: Attempt | null, origin: string): View {
  if (!a) return { phase: 'idle', pending: false, userCode: '', browserLink: '', note: '', message: '' };
  const showing = isPending(a.phase);
  return { phase: a.phase, pending: showing, userCode: showing ? a.userCode : '', browserLink: showing && a.note !== '' ? browserLink(a.link, origin) : '', note: showing ? a.note : '', message: a.message };
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
};

let cur: Attempt | null = null;
export const resetAttempt = (): void => { cur = null; };
export const signInView = (origin: string): View => view(cur, origin);

function dispatch(env: AttemptEnv, e: Event): void {
  if (!cur) return;
  const before = cur.phase;
  cur = step(cur, e);
  if (cur.phase !== before && e.id === cur.id) env.log(`signin: attempt ${cur.id.slice(0, 6)} ${cur.phase}`);
}
const own = (id: string): boolean => cur !== null && cur.id === id;
const ask = async (env: AttemptEnv, op: NativeRequest['op'], attempt: string): Promise<Record<string, unknown>> => {
  try { const r = await env.later({ op, attempt }); return r && typeof r === 'object' ? (r as Record<string, unknown>) : {}; } catch { return {}; }
};
const answerOf = (r: Record<string, unknown>): Answer => {
  const a = r.answer;
  return a === 'pending' || a === 'slow_down' || a === 'approved' || a === 'denied' || a === 'expired' || a === 'stale' || a === 'invalid' ? a : 'error';
};

/** Open the sheet and tell the machine how it ended. The caller keeps this promise inside its own answer: a mutation stays until its work is done (docs/reference/exact/answers-let-go.md). */
async function openSheet(env: AttemptEnv, id: string, link: string): Promise<void> {
  let r: Pick<SheetResult, 'outcome' | 'message'>;
  try { r = await env.sheet(link); } catch { r = { outcome: 'failed', message: sheetOutcomeNote }; }
  dispatch(env, { t: 'sheet', id, outcome: r.outcome, note: r.outcome === 'failed' ? r.message : '' });
}

/** Start an attempt and run it to its end: begin, open the sheet, poll. Returns the phase it ended in. A new run forgets the one before. */
export async function signInRun(env: AttemptEnv): Promise<{ phase: Phase }> {
  if (cur && !isTerminal(cur.phase)) { const old = cur.id; dispatch(env, { t: 'cancel', id: old }); await ask(env, 'signInForget', old); }
  let id: string;
  try { id = env.newId(); } catch { cur = step(begin('none'), { t: 'beginFailed', id: 'none', message: WORDS.lost }); return { phase: cur.phase }; }
  cur = begin(id);
  env.log(`signin: attempt ${id.slice(0, 6)} begun`);
  const b = await ask(env, 'signInBegin', id);
  if (!own(id)) return { phase: 'cancelled' };
  if (b.ok !== true || typeof b.link !== 'string') { dispatch(env, { t: 'beginFailed', id, message: typeof b.message === 'string' && b.message ? b.message : WORDS.lost }); return { phase: cur!.phase }; }
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
  return { phase: own(id) ? cur!.phase : 'cancelled' };
}

/** The card's Cancel. The code is forgotten FIRST, then the machine moves: no key is written after it. */
export async function signInCancel(env: AttemptEnv): Promise<{ phase: Phase }> {
  const a = cur;
  if (!a || isTerminal(a.phase)) return { phase: a ? a.phase : 'idle' };
  dispatch(env, { t: 'cancel', id: a.id });
  await ask(env, 'signInForget', a.id);
  return { phase: own(a.id) ? cur!.phase : 'cancelled' };
}

/** "Open the approval page": the sheet again, and the close is forgotten. The answer stays until that sheet ends. */
export async function signInReopen(env: AttemptEnv): Promise<{ phase: Phase }> {
  const a = cur;
  if (!a || (a.phase !== 'sheet' && a.phase !== 'closed')) return { phase: a ? a.phase : 'idle' };
  dispatch(env, { t: 'reopen', id: a.id });
  await openSheet(env, a.id, a.link);
  return { phase: own(a.id) ? cur!.phase : 'cancelled' };
}
