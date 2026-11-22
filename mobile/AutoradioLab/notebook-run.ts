// The notebook page's state in shared code: what the Swift NotebookStore kept for the page (the last reads, the selected line, the refresh) and the one
// narrow native call. The model is notebook.ts; tests are notebook-run.test.ts (R1 to R7).
//
// The native call, `{op: 'notebookState'}` / `{op: 'notebookText'}` on the app module (AutoradioModule+Notebook.swift `notebookOp`), each with the reason only native code can meet:
//  - notebookState: the steer taps waiting (the more / less taps are queued by the Swift cards and sent by the Swift steer queue, which the Swift run sheet feeds) and the write of the
//    listener's topics into the notebook (a Texture canonical write through the Swift TextureEditor: red, it stays native until that editor moves). Answers `{steerQueued}`.
//  - notebookText: the notebook text just read, for the DJ's request. The request is built natively (RadioProgramme.swift) and cannot read `app:/data`.
// With no module (Android, the web) there is no call and no steer line.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, nativeInert, inertTimeout, scoped } from './api.ts';
import { picksText } from './edition-picks.ts';
import { queued as steerQueued } from './steer.ts';
import { loadDoc, readLearning, page, seamGet, type Get, type PageInfo } from './notebook.ts';

// Module state belongs to the world (api.ts `scoped`): a closed world's is unreachable and no later world joins it.
const S = scoped(() => ({ memo: null as { key: string; read: Promise<Read> } | null, pushed: null as string | null, pushing: null as string | null }));

export interface Device { state(): Promise<void>; text(text: string): Promise<void>; }
export type Env = { get: Get; device: Device | null; deviceDeadlineMs?: number };
export const DEVICE_DEADLINE_MS = 20000;
/** The page waits at most this long for the native steer count (the topics write is native's and may take longer: the page is not held for it). */
export const STATE_WAIT_MS = 4000;

export function nativeDevice(): Device | null {
  const raw = nativeInert();
  if (!raw) return null;
  const later = raw as unknown as (r: Record<string, unknown>) => Promise<unknown>;
  return {
    async state() { await later({ op: 'notebookState' }); },
    async text(text) { await later({ op: 'notebookText', text }); },
  };
}
export const liveEnv = (): Env => ({ get: seamGet, device: nativeDevice() });

/** The line after a tap: the same id again clears the selection. */
export const selectedAfter = (current: string, id: string): string => (current === id ? '' : id);

type Read = { doc: Awaited<ReturnType<typeof loadDoc>>['load']; learning: Awaited<ReturnType<typeof readLearning>>; steer: number };

const idle = (): PageInfo => page({ doc: null, learning: null, selected: '', steerQueued: 0 });
const within = <T,>(p: Promise<T>, ms: number, fallback: T): Promise<T> =>
  Promise.race([p.catch(() => fallback), inertTimeout(ms).then(() => fallback)]);

async function readAll(storage: Storage | undefined, env: Env, gen: number): Promise<Read> {
  const d = env.device, ms = env.deviceDeadlineMs ?? DEVICE_DEADLINE_MS;
  if (d) await picksText(storage).catch(() => undefined);                     // the picks reach the native copy first (the topics write reads it)
  if (d) await within(d.state(), Math.min(ms, STATE_WAIT_MS), undefined);   // first: the topics are in the notebook before it is read (when native is quick; the page is not held for it)
  const steer = await steerQueued(storage);                                   // the waiting more / less taps are shared code's (steer.ts)
  const [{ load, text }, learning] = await Promise.all([loadDoc(env.get, storage), readLearning(env.get)]);
  if (d && text !== null && text !== S().pushed && text !== S().pushing && gen === accountGeneration()) {
    // The DJ's copy: handed over in the background, never waited for. `pushed` says "native has it" only after the call succeeded, so a timeout is tried again at the next read.
    S().pushing = text;
    void (async () => {
      try { await Promise.race([d.text(text), inertTimeout(ms).then((): never => { throw new Error('deadline'); })]); if (gen === accountGeneration()) S().pushed = text; }
      catch { /* not handed over: the next read tries again */ }
      finally { if (S().pushing === text) S().pushing = null; }
    })();
  }
  return { doc: load, learning, steer };
}

/** The page's source. `tick` is the Refresh press, `handle` the account handle (the world it was asked in); a select changes only `selected` and reads nothing. Never throws. */
export async function notebookShared(a: { selected: string; tick: number | string; active: boolean; handle: string }, storage: Storage | undefined, env: Env = liveEnv()): Promise<PageInfo> {
  if (!a.active) { S().memo = null; return idle(); }
  const gen = accountGeneration();
  const key = `${a.tick}|${a.handle}|${gen}`;
  const st = S();
  if (st.memo === null || st.memo.key !== key) st.memo = { key, read: readAll(storage, env, gen) };
  const mine = st.memo;
  const r = await mine.read;
  if (gen !== accountGeneration()) return idle();                                  // the account moved while this read was out: none of it is shown
  return page({ doc: r.doc, learning: r.learning, selected: a.selected, steerQueued: r.steer });
}
