// A model of Exact's answer ownership for `bun test` (Bun has no Exact runtime). Every rule here is the pinned prelude's (mobile/.exact-src/js/src/prelude.js, time.js); the line is beside it.
// Used by settle-once.test.ts. Lives in a folder of its own so the scans of the app's modules (account.test.ts S4, radio-pulse-rule.test.ts, shared-promise-gate.test.ts) do not read it.
//
//  R1 each step of an async function resumes under the answer that called it (`__exact_async_context`, prelude.js:230-241): AsyncLocalStorage carries it the same way.
//  R2 a timer belongs to the answer that arms it; arming one for an answer that was let go or has replied throws (`timerOwner`, prelude.js:1352-1366).
//  R3 letting an answer go rejects its fetches and native calls in flight (prelude.js:1017-1034), refuses new ones (fetch.js `call.letGo`), and DROPS its timers: they never fire and what awaits them stays
//     pending (`timers.drop`, prelude.js:1041, time.js:234-238). An answer's reply drops its timers too (prelude.js:688).
//  R4 an answer that awaits work it does not own fails "the answer is pending on nothing" once no timer of its own is armed and no fetch, native call or storage step is in flight in the module
//     (`settleResult`, prelude.js:719-752; another answer's armed timer does not count there, so it does not here).
import { AsyncLocalStorage } from 'node:async_hooks';

export type Call = { id: string; letGo: boolean; replied: boolean; timers: Map<ReturnType<typeof setTimeout>, true>; waiting: Set<(e: Error) => void> };
const calls = new AsyncLocalStorage<Call>();
const realSet = globalThis.setTimeout; const realClear = globalThis.clearTimeout;
let made: Call[] = [];
let inFlight = 0;
/** Every timer armed: the answer that armed it and its delay (the test reads it). */
export let armedBy: { id: string; ms: number }[] = [];

export const current = (): Call | undefined => calls.getStore();
export const newCall = (id: string): Call => { const c: Call = { id, letGo: false, replied: false, timers: new Map(), waiting: new Set() }; made.push(c); return c; };
const dropTimers = (c: Call): void => { for (const t of c.timers.keys()) realClear(t); c.timers.clear(); };
/** R3. */
export function letGo(c: Call): void {
  c.letGo = true; dropTimers(c);
  for (const w of c.waiting) w(new Error('the answer was let go before this reply; the request may already have been sent'));
  c.waiting.clear();
}

/** Run `f` as answer `c`; settles like the runner: its value, its failure, or R4. */
export function ask<T>(c: Call, f: () => Promise<T>): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    let done = false;
    const end = (): void => { done = true; c.replied = true; dropTimers(c); };
    calls.run(c, () => f()).then((v) => { if (!done) { end(); resolve(v); } }, (e) => { if (!done) { end(); reject(e); } });
    const watch = (): void => {
      if (done || c.letGo) return;
      if (c.timers.size === 0 && inFlight === 0) { end(); reject(new Error('the answer is pending on nothing: it awaits a promise that no fetch or storage step in flight in this module will settle')); return; }
      realSet(watch, 2);
    };
    realSet(watch, 2);
  });
}

/** A host operation (a native call, a fetch) made by the current answer: refused if it was let go, rejected if it is let go while the operation is out (R3). */
export function hostOp<T>(work: () => Promise<T>): Promise<T> {
  const c = calls.getStore();
  if (c?.letGo || c?.replied) return Promise.reject(new Error('the answer was let go before this fetch'));
  inFlight += 1;
  return new Promise<T>((resolve, reject) => {
    let open = true;
    const close = (): void => { if (open) { open = false; inFlight -= 1; c?.waiting.delete(fail); } };
    const fail = (e: Error): void => { close(); reject(e); };
    c?.waiting.add(fail);
    work().then((v) => { if (open) { close(); resolve(v); } }, (e) => { if (open) { close(); reject(e); } });
  });
}

/** Install R2 and R3's timers; `uninstall` ends every answer's timers. */
export function install(): void {
  made = []; inFlight = 0; armedBy = [];
  globalThis.setTimeout = ((fn: (...a: unknown[]) => void, ms?: number, ...a: unknown[]) => {
    const c = calls.getStore();
    if (c) armedBy.push({ id: c.id, ms: ms ?? 0 });
    if (c && (c.letGo || c.replied)) throw new Error('setTimeout() is unavailable here: a timer belongs to the answer that arms it, and none is running');
    const t = realSet(() => { c?.timers.delete(t); fn(...a); }, ms);
    c?.timers.set(t, true);
    return t;
  }) as typeof setTimeout;
  globalThis.clearTimeout = ((t: ReturnType<typeof setTimeout>) => { for (const c of made) c.timers.delete(t); realClear(t); }) as typeof clearTimeout;
}
export function uninstall(): void { for (const c of made) dropTimers(c); made = []; globalThis.setTimeout = realSet; globalThis.clearTimeout = realClear; }
/** Real time passing, outside any answer. */
export const tick = async (n = 20): Promise<void> => { for (let i = 0; i < n; i++) await new Promise((r) => realSet(r, 1)); };
