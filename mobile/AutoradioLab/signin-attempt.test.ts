// One sign-in attempt is one state machine with an id, owned in one place (signin-attempt.ts; docs/reference/exact/auth-session.md has the transition table).
// Failure list, written before the code. Three review rounds found the same fault three ways, so every row of the table is a test, and so are the three the review found:
//  B1 a poll answer "pending" that STARTED BEFORE the sheet closed (it may predate the approval) must not abandon the attempt; only a poll started after the close may.
//  B2 after the owner's Cancel no key is written: a poll answer that arrives after it is ignored, the code is forgotten first, and native refuses to write (Swift tests).
//  B3 re-opening the sheet forgets the close: a later "pending" does not abandon the attempt.
//  T* every row of the table: the phase it goes to, and "anything after the end changes nothing".
//  I* an event for another attempt (an old sheet, a late poll answer) changes nothing, and a new sign-in forgets the old attempt first.
//  D* the driver: the order of the native calls, the number of polls, a timeout, a refused begin, a sheet that cannot open, a callback with no approval.
//  V* the view: what the card shows, a browser fallback only for https on the granted origin, words for each ending.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { begin as newAttempt, step, isTerminal, view, signInRun, signInCancel, signInReopen, signInView, resetAttempt, type Attempt, type AttemptEnv, type Event } from './signin-attempt.ts';

const ORIGIN = 'https://choir.news';
const LINK = `${ORIGIN}/device?code=ABCD-EFGH`;
const ID = 'att-1';

const open = (): Attempt => step(newAttempt(ID), { t: 'begun', id: ID, userCode: 'ABCD-EFGH', link: LINK });          // phase sheet
const issued = (a: Attempt, n = 1): Attempt => { for (let i = 0; i < n; i++) a = step(a, { t: 'issued', id: a.id, n: a.issued + 1 }); return a; };
const closed = (a: Attempt): Attempt => step(a, { t: 'sheet', id: a.id, outcome: 'cancelled', note: '' });
const ans = (a: Attempt, answer: Event extends infer E ? (E extends { t: 'answer'; answer: infer A } ? A : never) : never, n: number): Attempt => step(a, { t: 'answer', id: a.id, n, answer });

describe('T the transition table', () => {
  test('begun: BEGUN opens the sheet, BEGIN_FAILED fails, CANCEL cancels, sheet events wait', () => {
    const b = newAttempt(ID);
    expect(b.phase).toBe('begun');
    expect(step(b, { t: 'begun', id: ID, userCode: 'U', link: LINK }).phase).toBe('sheet');
    expect(step(b, { t: 'beginFailed', id: ID, message: 'no' })).toMatchObject({ phase: 'failed', message: 'no' });
    expect(step(b, { t: 'cancel', id: ID }).phase).toBe('cancelled');
    expect(step(b, { t: 'sheet', id: ID, outcome: 'cancelled', note: '' }).phase).toBe('begun');
  });
  test('sheet: returned stays, failed keeps the poll and notes it, cancelled closes', () => {
    const a = open();
    expect(step(a, { t: 'sheet', id: ID, outcome: 'returned', note: '' }).phase).toBe('sheet');
    expect(step(a, { t: 'sheet', id: ID, outcome: 'failed', note: 'could not open' })).toMatchObject({ phase: 'sheet', note: 'could not open' });
    expect(closed(issued(a, 2))).toMatchObject({ phase: 'closed', closedAt: 2 });
  });
  test('closed: reopen returns to sheet and forgets the close', () => {
    const c = closed(issued(open(), 2));
    expect(step(c, { t: 'reopen', id: ID })).toMatchObject({ phase: 'sheet', closedAt: -1, note: '' });
    expect(step(open(), { t: 'reopen', id: ID }).phase).toBe('sheet');
  });
  test('closed: pending abandons only for a poll issued after the close', () => {
    const c = closed(issued(open(), 2));               // two polls issued; the second may still be in flight
    expect(ans(c, 'pending', 1).phase).toBe('closed');
    expect(ans(c, 'pending', 2).phase).toBe('closed');
    expect(ans(issued(c), 'pending', 3).phase).toBe('cancelled');
  });
  test('sheet: pending, slow_down and error decide nothing; closed: slow_down and error too', () => {
    const a = issued(open());
    for (const x of ['pending', 'slow_down', 'error'] as const) expect(ans(a, x, 1).phase).toBe('sheet');
    const c = issued(closed(a));
    for (const x of ['slow_down', 'error'] as const) expect(ans(c, x, 2).phase).toBe('closed');
  });
  test('approved, denied, expired, stale, invalid end the attempt from sheet and from closed, whatever the poll number', () => {
    for (const from of [issued(open()), closed(issued(open()))]) {
      expect(ans(from, 'approved', 1).phase).toBe('approved');
      expect(ans(from, 'denied', 1)).toMatchObject({ phase: 'denied', message: 'the sign-in was denied' });
      expect(ans(from, 'expired', 1)).toMatchObject({ phase: 'expired', message: 'the code expired; sign in again' });
      expect(ans(from, 'stale', 1).phase).toBe('failed');
      expect(ans(from, 'invalid', 1).phase).toBe('failed');
      expect(step(from, { t: 'timeout', id: ID }).phase).toBe('expired');
    }
  });
  test('CANCEL ends begun, sheet and closed', () => {
    for (const from of [newAttempt(ID), open(), closed(open())]) expect(step(from, { t: 'cancel', id: ID }).phase).toBe('cancelled');
  });
  test('anything after the end changes nothing', () => {
    const ends = [step(open(), { t: 'cancel', id: ID }), ans(issued(open()), 'approved', 1), ans(issued(open()), 'denied', 1), ans(issued(open()), 'expired', 1), step(newAttempt(ID), { t: 'beginFailed', id: ID, message: 'x' })];
    const events: Event[] = [{ t: 'begun', id: ID, userCode: 'U', link: LINK }, { t: 'sheet', id: ID, outcome: 'cancelled', note: '' }, { t: 'reopen', id: ID }, { t: 'cancel', id: ID },
      { t: 'issued', id: ID, n: 9 }, { t: 'answer', id: ID, n: 9, answer: 'approved' }, { t: 'answer', id: ID, n: 9, answer: 'pending' }, { t: 'timeout', id: ID }];
    for (const e of ends) { expect(isTerminal(e.phase)).toBe(true); for (const ev of events) expect(step(e, ev)).toEqual(e); }
  });
});

describe('B the three the review found', () => {
  test('B1 a pending that started before the close cannot abandon an approved attempt; the next poll gets the key', () => {
    let a = issued(open());                       // poll 1 goes out
    a = closed(a);                                // the page approved and the sheet was swiped away; poll 1 is still in flight and will answer pending
    a = ans(a, 'pending', 1);                     // the stale answer
    expect(a.phase).toBe('closed');
    a = ans(issued(a), 'approved', 2);            // the poll issued after the close sees the approval
    expect(a.phase).toBe('approved');
  });
  test('B2 an answer after the owner\'s Cancel changes nothing, a key included', () => {
    let a = issued(open());
    a = step(a, { t: 'cancel', id: ID });
    expect(ans(a, 'approved', 1).phase).toBe('cancelled');
  });
  test('B3 reopen, then a pending: not abandoned', () => {
    let a = closed(issued(open()));
    a = step(a, { t: 'reopen', id: ID });
    expect(ans(issued(a), 'pending', 2).phase).toBe('sheet');
  });
});

describe('I events of another attempt', () => {
  test('a different id changes nothing', () => {
    const a = issued(open());
    for (const ev of [{ t: 'sheet', id: 'old', outcome: 'cancelled', note: '' }, { t: 'cancel', id: 'old' }, { t: 'answer', id: 'old', n: 1, answer: 'approved' }, { t: 'reopen', id: 'old' }, { t: 'timeout', id: 'old' }] as Event[])
      expect(step(a, ev)).toEqual(a);
  });
});

// ---- the driver, over a fake native, a fake sheet and a clock that does not wait

type Poll = { resolve: (answer: string) => void; id: string };
function world(over: { sheet?: (link: string) => Promise<{ outcome: 'returned' | 'cancelled' | 'failed'; message: string }>; begin?: () => unknown } = {}) {
  const calls: string[] = []; const polls: Poll[] = []; const logs: string[] = []; let t = 0; let n = 0;
  const sheets: { link: string; resolve: (r: { outcome: 'returned' | 'cancelled' | 'failed'; message: string }) => void }[] = [];
  const env: AttemptEnv = {
    origin: ORIGIN,
    later: (r) => {
      calls.push(`${r.op}:${r.attempt}`);
      if (r.op === 'signInBegin') return Promise.resolve(over.begin ? over.begin() : { ok: true, userCode: 'ABCD-EFGH', link: LINK, interval: 5, expires: 600 });
      if (r.op === 'signInPoll') return new Promise((res) => polls.push({ id: String(r.attempt), resolve: (a) => res({ answer: a }) }));
      return Promise.resolve({ ok: true });
    },
    sheet: (link) => (over.sheet ? over.sheet(link) : new Promise((res) => sheets.push({ link, resolve: res }))),
    sleep: async (ms) => { t += ms; },
    newId: () => `att-${++n}`,
    log: (l) => logs.push(l),
  };
  return { env, calls, polls, sheets, logs, clock: () => t };
}
/** Close every sheet still open (a person closes them; the run's answer stays until they have ended) and wait for the run. */
const done = async <T>(w: { sheets: { resolve: (r: { outcome: 'returned' | 'cancelled' | 'failed'; message: string }) => void }[] }, run: Promise<T>): Promise<T> => { for (const x of w.sheets) x.resolve({ outcome: 'returned', message: '' }); await settle(); return run; };
const settle = async (): Promise<void> => { for (let i = 0; i < 20; i++) await Promise.resolve(); await new Promise((r) => setTimeout(r, 0)); };

beforeEach(() => resetAttempt());

describe('D the driver', () => {
  test('D1 begin, the sheet opens with the link, a callback with no approval changes nothing, polls go on, then the key', async () => {
    const w = world();
    const run = signInRun(w.env); await settle();
    expect(w.calls[0]).toBe('signInBegin:att-1'); expect(w.sheets.length).toBe(1); expect(w.sheets[0].link).toBe(LINK);
    expect(signInView(ORIGIN)).toMatchObject({ phase: 'sheet', pending: true, userCode: 'ABCD-EFGH' });
    w.sheets[0].resolve({ outcome: 'returned', message: '' }); await settle();
    expect(signInView(ORIGIN).phase).toBe('sheet');                       // a callback with no approval: still waiting
    w.polls[0].resolve('pending'); await settle();
    expect(signInView(ORIGIN).phase).toBe('sheet'); expect(w.polls.length).toBe(2);
    w.polls[1].resolve('approved'); await settle();
    expect((await done(w, run)).phase).toBe('approved'); expect(signInView(ORIGIN)).toMatchObject({ phase: 'approved', pending: false });
    expect(w.calls.filter((c) => c.startsWith('signInForget'))).toEqual([]);
    expect(w.polls.length).toBe(2);                                         // no poll after the end
  });
  test('D2 B1 end to end: the sheet is swiped away with a poll in flight; its pending is ignored; the next poll gets the key', async () => {
    const w = world();
    const run = signInRun(w.env); await settle();
    expect(w.polls.length).toBe(1);
    w.sheets[0].resolve({ outcome: 'cancelled', message: '' }); await settle();
    expect(signInView(ORIGIN).phase).toBe('closed');
    w.polls[0].resolve('pending'); await settle();                          // started before the close
    expect(signInView(ORIGIN).phase).toBe('closed'); expect(w.polls.length).toBe(2);
    w.polls[1].resolve('approved'); await settle();
    expect((await done(w, run)).phase).toBe('approved'); expect(w.calls.some((c) => c.startsWith('signInForget'))).toBe(false);
  });
  test('D3 closed and then still pending on a poll started after the close: abandoned, and native forgets the code', async () => {
    const w = world();
    const run = signInRun(w.env); await settle();
    w.sheets[0].resolve({ outcome: 'cancelled', message: '' }); await settle();
    w.polls[0].resolve('pending'); await settle();
    w.polls[1].resolve('pending'); await settle();
    expect((await done(w, run)).phase).toBe('cancelled'); expect(w.calls).toContain('signInForget:att-1');
    expect(signInView(ORIGIN)).toMatchObject({ phase: 'cancelled', pending: false, message: '' });          // no noise
  });
  test('D4 B2 end to end: the card Cancel forgets the code first; the answer that was in flight is ignored', async () => {
    const w = world();
    const run = signInRun(w.env); await settle();
    const cancelled = signInCancel(w.env); await settle();
    expect(w.calls).toContain('signInForget:att-1');
    w.polls[0].resolve('approved'); await settle();                          // arrives after Cancel
    await cancelled;
    expect((await done(w, run)).phase).toBe('cancelled'); expect(signInView(ORIGIN).phase).toBe('cancelled');
    expect(w.calls.indexOf('signInForget:att-1')).toBeLessThan(w.calls.length);
    expect(w.polls.length).toBe(1);
  });
  test('D5 B3 end to end: close, reopen, approve; a pending after the reopen does not abandon', async () => {
    const w = world();
    const run = signInRun(w.env); await settle();
    w.sheets[0].resolve({ outcome: 'cancelled', message: '' }); await settle();
    void signInReopen(w.env); await settle();
    expect(signInView(ORIGIN).phase).toBe('sheet'); expect(w.sheets.length).toBe(2);
    w.polls[0].resolve('pending'); await settle(); w.polls[1].resolve('pending'); await settle();
    expect(signInView(ORIGIN).phase).toBe('sheet');
    w.polls[2].resolve('approved'); await settle();
    expect((await done(w, run)).phase).toBe('approved');
  });
  test('D6 a new sign-in forgets the old attempt; the old sheet and the old answers change nothing', async () => {
    const w = world();
    const first = signInRun(w.env); await settle();
    const second = signInRun(w.env); await settle();
    expect(w.calls).toContain('signInForget:att-1'); expect(w.calls).toContain('signInBegin:att-2');
    w.sheets[0].resolve({ outcome: 'cancelled', message: '' }); await settle();          // the first attempt's sheet
    expect(signInView(ORIGIN).phase).toBe('sheet');
    w.polls[0].resolve('approved'); await settle();                                       // the first attempt's late answer
    expect(signInView(ORIGIN).phase).toBe('sheet');
    w.polls[1]?.resolve('approved'); await settle();
    expect((await done(w, second)).phase).toBe('approved'); await first;
  });
  test('D7 denied and expired, with their words', async () => {
    for (const [answer, words] of [['denied', 'the sign-in was denied'], ['expired', 'the code expired; sign in again']] as const) {
      resetAttempt(); const w = world();
      const run = signInRun(w.env); await settle(); w.polls[0].resolve(answer); await settle();
      expect((await done(w, run)).phase).toBe(answer); expect(signInView(ORIGIN).message).toBe(words);
    }
  });
  test('D8 the code runs out without an answer: expired by the clock', async () => {
    const w = world({ begin: () => ({ ok: true, userCode: 'U', link: LINK, interval: 5, expires: 12 }) });
    const run = signInRun(w.env); await settle();
    for (let i = 0; i < 6 && !isTerminal(signInView(ORIGIN).phase); i++) { w.polls[i]?.resolve('pending'); await settle(); }
    expect((await done(w, run)).phase).toBe('expired');
  });
  test('D9 a refused begin fails with words; no sheet opens', async () => {
    const w = world({ begin: () => ({ ok: false, message: 'contacting choir.news failed' }) });
    expect((await signInRun(w.env)).phase).toBe('failed'); expect(w.sheets.length).toBe(0); expect(signInView(ORIGIN).message).toBe('contacting choir.news failed');
    resetAttempt(); const w2 = world({ begin: () => { throw new Error('bridge'); } });
    expect((await signInRun(w2.env)).phase).toBe('failed');
  });
  test('D10 a sheet that cannot open is a note and a browser link; the poll goes on and collects the key', async () => {
    const w = world({ sheet: async () => ({ outcome: 'failed', message: 'The sign-in window could not open.' }) });
    const run = signInRun(w.env); await settle();
    expect(signInView(ORIGIN)).toMatchObject({ phase: 'sheet', note: 'The sign-in window could not open.', browserLink: LINK });
    w.polls[0].resolve('approved'); await settle();
    expect((await done(w, run)).phase).toBe('approved');
  });
  test('D11 an error or slow_down only waits longer; a native answer of stale ends it', async () => {
    const w = world();
    const run = signInRun(w.env); await settle();
    const t0 = w.clock(); w.polls[0].resolve('slow_down'); await settle(); w.polls[1].resolve('error'); await settle();
    expect(w.clock() - t0).toBeGreaterThan(5000);
    w.polls[2].resolve('stale'); await settle();
    expect((await done(w, run)).phase).toBe('failed');
  });
  test('D12 a poll that throws is an error answer, not the end', async () => {
    const w = world(); w.env.later = ((orig) => (r: { op: string; attempt?: unknown }) => (r.op === 'signInPoll' && w.polls.length === 0 ? (w.polls.push({ id: 'x', resolve: () => undefined }), Promise.reject(new Error('net'))) : orig(r)))(w.env.later) as never;
    const run = signInRun(w.env); await settle();
    expect(signInView(ORIGIN).phase).toBe('sheet');
    w.polls[1].resolve('approved'); await settle();
    expect((await done(w, run)).phase).toBe('approved');
  });
  test('D13 nothing is logged but ids and phases', async () => {
    const w = world(); const run = signInRun(w.env); await settle(); w.polls[0].resolve('approved'); await settle(); await done(w, run);
    const all = w.logs.join('\n');
    expect(all).not.toContain('ABCD'); expect(all).not.toContain('choir.news'); expect(all).toContain('signin: attempt');
  });
});

describe('V the view', () => {
  test('V1 the browser fallback is https on the granted origin and nothing else', async () => {
    for (const [link, ok] of [[LINK, true], ['http://choir.news/device?code=A', false], ['https://choir.news.evil.example/d', false], ['javascript:alert(1)', false], ['news.choir.autoradio.lab:/x', false], ['https://u@choir.news/d', false], ['', false]] as [string, boolean][]) {
      resetAttempt(); const w = world({ begin: () => ({ ok: true, userCode: 'U', link, interval: 5, expires: 600 }), sheet: async () => ({ outcome: 'failed', message: 'x' }) });
      void signInRun(w.env); await settle();
      expect(signInView(ORIGIN).browserLink).toBe(ok ? link : '');
    }
  });
  test('V2 with no attempt the view is idle and empty', () => {
    expect(signInView(ORIGIN)).toEqual({ phase: 'idle', pending: false, userCode: '', browserLink: '', note: '', message: '' });
  });
  test('V3 view() is pure over an attempt: pending covers begun, sheet and closed only', () => {
    expect(view(newAttempt(ID), ORIGIN).pending).toBe(true);
    expect(view(closed(open()), ORIGIN).pending).toBe(true);
    expect(view(step(open(), { t: 'cancel', id: ID }), ORIGIN).pending).toBe(false);
  });
});

describe('S the module is shared code only', () => {
  test('S1 it names no key, store or storage', () => {
    const src = readFileSync(new URL('./signin-attempt.ts', import.meta.url), 'utf8').split('\n').filter((l) => !l.trim().startsWith('//')).join('\n');
    expect(src).not.toMatch(/\bapi_key\b|keychain|atomicWriteFile|storage\.|device_code|\bsecret\b/i);
  });
});

describe('C the contract only renders and presses', () => {
  const read = (f: string): string => readFileSync(new URL(`./${f}`, import.meta.url), 'utf8');
  const contracts = ['account.contract', 'app.contract', 'onboarding.contract', 'settings.contract'].map(read).join('\n');
  test('C1 no native sign-in sends, no second copy of the status: the three presses are the machine\'s', () => {
    const account = read('account.contract');
    expect(contracts).not.toMatch(/choirSignIn|choirCancelSignIn|choirOpenApproval|signInEnded|sheetOutcome|sheetNote/);
    expect(contracts).not.toMatch(/au\.(status|userCode|verificationURL|message)\b/);
    expect(account).toMatch(/action signIn\n    send signedRun = signInRun\(\)/);
    expect(account).toMatch(/action cancelSignIn\n    send signedCancel = signInCancel\(\)/);
    expect(account).toMatch(/action openApproval\n    send signedReopen = signInReopen\(\)/);
    expect(account).toMatch(/resource siv = signInView\(sivTick\)/);
  });
  test('C2 the browser fallback is the view\'s validated link, which is empty (no link at all) unless https on the granted origin', () => {
    expect(read('onboarding.contract')).toMatch(/if siv\.note != ""[\s\S]*href=siv\.browserLink/);
  });
  test('C3 every source the contract sends is answered by app.ts, once', () => {
    const app = read('app.ts');
    for (const n of ['signInRun', 'signInCancel', 'signInReopen', 'signInView']) expect((app.match(new RegExp(`^  ${n}:`, 'gm')) ?? []).length).toBe(1);
  });
});
