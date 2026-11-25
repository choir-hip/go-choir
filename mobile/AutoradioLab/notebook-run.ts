// The notebook page's state in shared code: what the Swift NotebookStore kept for the page (the last reads, the selected line, the refresh). The model is notebook.ts; tests are
// notebook-run.test.ts (R1 to R7). Nothing here is native any more: the listener's topics are written into the notebook by notebook-topics.ts (it was the one native call, `notebookState`,
// because the write went through the Swift Texture editor), and the waiting steer taps are steer.ts's.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration } from './api.ts';
import { writeTopics } from './notebook-topics.ts';
import { queued as steerQueued } from './steer.ts';
import { loadDoc, readLearning, page, seamGet, type Get, type PageInfo } from './notebook.ts';

/** The topics write, as the page asks for it on every read: true when the notebook holds them (or there is nothing to write). */
export interface Writer { write(): Promise<boolean>; }
export type Env = { get: Get; writer: Writer | null; writerDeadlineMs?: number };
export const WRITER_DEADLINE_MS = 20000;
/** The page waits at most this long for the topics write (it may take longer: the page is not held for it). */
export const STATE_WAIT_MS = 4000;

export const liveEnv = (storage: Storage | undefined): Env => ({ get: seamGet, writer: { write: () => writeTopics(storage) } });

/** The line after a tap: the same id again clears the selection. */
export const selectedAfter = (current: string, id: string): string => (current === id ? '' : id);

type Read = { doc: Awaited<ReturnType<typeof loadDoc>>['load']; learning: Awaited<ReturnType<typeof readLearning>>; steer: number };
let memo: { key: string; read: Promise<Read> } | null = null;
export function forgetNotebook(): void { memo = null; }

const idle = (): PageInfo => page({ doc: null, learning: null, selected: '', steerQueued: 0 });
const within = <T,>(p: Promise<T>, ms: number, fallback: T): Promise<T> =>
  Promise.race([p.catch(() => fallback), new Promise<T>((r) => setTimeout(() => r(fallback), ms))]);

async function readAll(storage: Storage | undefined, env: Env, gen: number): Promise<Read> {
  const w = env.writer, ms = env.writerDeadlineMs ?? WRITER_DEADLINE_MS;
  if (w) await within(w.write(), Math.min(ms, STATE_WAIT_MS), false);       // first: the topics are in the notebook before it is read (when it is quick; the page is not held for it)
  const steer = await steerQueued(storage);                                   // the waiting more / less taps are shared code's (steer.ts)
  const [{ load }, learning] = await Promise.all([loadDoc(env.get, storage), readLearning(env.get)]);
  return { doc: load, learning, steer };
}

/** The page's source. `tick` is the Refresh press, `signedOutTick` a sign-out; a select changes only `selected` and reads nothing. Never throws. */
export async function notebookShared(a: { selected: string; tick: number | string; active: boolean; signedOutTick: string }, storage: Storage | undefined, env: Env = liveEnv(storage)): Promise<PageInfo> {
  if (!a.active) { memo = null; return idle(); }
  const gen = accountGeneration();
  const key = `${a.tick}|${a.signedOutTick}|${gen}`;
  if (memo === null || memo.key !== key) memo = { key, read: readAll(storage, env, gen) };
  const mine = memo;
  const r = await mine.read;
  if (gen !== accountGeneration()) return idle();                                  // the account moved while this read was out: none of it is shown
  return page({ doc: r.doc, learning: r.learning, selected: a.selected, steerQueued: r.steer });
}
