// The Computer page of Settings (settings:computer), as shared code: what the listener's computer is doing, Wake, and Restart to update. It replaces the
// Swift ComputerModel and ChoirComputerClient (Accounts.swift) and the module's `settingsComputer`, `computerRefresh`, `computerWake` and `computerUpdate`.
// Routes (the web's UpdateChip and compute monitor): GET /api/compute/status, POST /api/compute/recovery {action: wake_current_computer},
// GET /api/computers/{id}/update, GET /api/computers/{id}/lifecycle/status, POST /api/computers/{id}/lifecycle/update {idempotency_key}.
//
// Failure list (each line has a test in computer.test.ts):
//  C1 Wake or Restart to update is tapped twice: while one is in flight the second is ignored (the phase is not idle, failed or current).
//  C2 The update route answers 409 (the computer is busy with a self-development apply or a live capsule): the view says it will update when idle; nothing
//     retries by itself and nothing says it updated.
//  C3 A restart the proxy gave up on (timeout, 502, 504, a dropped connection) is not a failure: the connection dropping while restarting is expected. The view
//     polls the update signal (`computerAct("poll")`, on the page's own timer, at most 40 times) and says "current" only when the signal clears.
//  C4 Status has no computer yet (a new account): "try again in a moment", never a crash.
//  C5 A refusal is told in plain words: signed out, a refused key, a busy server, or nothing reached.
//  C6 A refresh that answers after a later act began does not overwrite that act's view (the generation guard of the Swift model).
//  C7 Signed out: no request is sent, and the view is empty.
// Nothing here reads a clock or sets a timer: the poll is the page's `task`, and "Restart to update" sends an idempotency key made from a counter and the
// computer's epoch, not from a random number (a data source may not call `Math.random`).

import { accountGeneration, currentWorld, authedJSON, HTTPError, segment, SignedOutError, scoped } from './api.ts';

// Module state belongs to the world (api.ts `scoped`): a closed world's is unreachable and no later world joins it.
const S = scoped(() => ({ one: undefined as Computer | undefined, oneGen: -1 }));

export type ComputerView = { id: string; state: string; target: string; phase: string; updateAvailable: boolean; message: string; epoch: number; checked: boolean };
export const POLLS = 40;

const empty = (): ComputerView => ({ id: '', state: '', target: '', phase: 'idle', updateAvailable: false, message: '', epoch: 0, checked: false });

export type Snapshot = { id: string; state: string; epoch: number; target: string };
export type Call = (method: 'GET' | 'POST', path: string, body?: unknown) => Promise<unknown>;
const live: Call = async (method, path, body) => (await authedJSON(path, { method, body, world: currentWorld() })).body;

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');

export class NoComputer extends Error { constructor() { super('Your account has no computer yet; try again in a moment.'); this.name = 'NoComputer'; } }

/** What the computer is, from the status route and the two per-computer routes (the last two may fail: their parts stay empty). */
export async function snapshot(call: Call): Promise<Snapshot> {
  const status = await call('GET', '/api/compute/status');
  const list = (isObj(status) && Array.isArray(status.computers) ? status.computers : []).filter(isObj)
    .filter((c) => (str(c.kind) || 'interactive') === 'interactive' && str(c.computer_id) !== '');
  const pick = list.find((c) => c.current === true) ?? list[0];
  if (!pick) throw new NoComputer();                                                                                      // C4
  const id = str(pick.computer_id);
  const s: Snapshot = { id, state: str(pick.state) || str(pick.status), epoch: 0, target: '' };
  const enc = segment(id);
  try { const o = await call('GET', `/api/computers/${enc}/update`); if (isObj(o) && isObj(o.update_available)) s.target = str(o.update_available.target); } catch { /* the update signal is optional */ }
  try {
    const o = await call('GET', `/api/computers/${enc}/lifecycle/status`);
    if (isObj(o)) { if (str(o.state) !== '') s.state = str(o.state); s.epoch = typeof o.realization_epoch === 'number' ? Math.trunc(o.realization_epoch) : 0; }
  } catch { /* likewise */ }
  return s;
}

/** C5: the words for a failure. */
export function plain(e: unknown): string {
  if (e instanceof SignedOutError) return 'Not signed in.';
  if (e instanceof NoComputer) return e.message;
  if (e instanceof HTTPError && e.status === 401) return "This phone's sign-in is no longer accepted.";
  if (e instanceof HTTPError && e.status === 429) return 'Your computer is busy right now. Try again in a moment.';
  return 'Could not reach your computer.';
}

/** The state of one process's Computer page. */
export class Computer {
  v: ComputerView = empty();
  private gen = 0;
  private polls = 0;
  private sent = 0;
  constructor(private call: Call = live) {}

  view(): ComputerView { return { ...this.v }; }
  private idle(): boolean { return this.v.phase === 'idle' || this.v.phase === 'failed' || this.v.phase === 'current'; }

  async refresh(): Promise<ComputerView> {
    const g = ++this.gen;
    try {
      const s = await snapshot(this.call);
      if (g === this.gen) {
        const failed = this.v.phase === 'failed';
        this.v = { ...this.v, id: s.id, state: s.state, epoch: s.epoch, target: s.target, updateAvailable: s.target !== '', checked: true, ...(failed ? { phase: 'idle', message: '' } : {}) };
      }
    } catch (e) { if (g === this.gen) this.v = { ...this.v, phase: 'failed', message: plain(e), checked: true }; }
    return this.view();
  }

  async wake(): Promise<ComputerView> {
    if (!this.idle()) return this.view();                                                                                  // C1
    this.v = { ...this.v, phase: 'waking', message: 'Waking your computer…' };
    try { await this.call('POST', '/api/compute/recovery', { action: 'wake_current_computer' }); this.v = { ...this.v, phase: 'idle', message: 'Your computer was asked to wake.' }; }
    catch (e) { this.v = { ...this.v, phase: 'failed', message: plain(e) }; return this.view(); }
    return this.refresh();
  }

  async restartToUpdate(): Promise<ComputerView> {
    if (!this.idle()) return this.view();                                                                                  // C1
    this.v = { ...this.v, phase: 'updating', message: 'Restarting to update…' };
    try {
      let id = this.v.id;
      if (id === '') id = (await snapshot(this.call)).id;
      this.sent += 1;
      const r = await this.call('POST', `/api/computers/${segment(id)}/lifecycle/update`, { idempotency_key: `autoradio-${id}-${this.v.epoch}-${this.sent}` });
      if (isObj(r) && r.outcome === 'current') this.v = { ...this.v, phase: 'current', message: 'Already up to date.', updateAvailable: false };
      else { this.polls = POLLS; this.v = { ...this.v, id }; }                                                              // started: the page polls
    } catch (e) {
      if (e instanceof HTTPError && e.status === 409) {                                                                    // C2
        this.v = { ...this.v, phase: 'paused', message: e.message === '' ? 'Your computer is busy and will update when it is idle.' : `Your computer is busy (${[...e.message].slice(0, 80).join('')}) and will update when it is idle.` };
      } else if (e instanceof HTTPError && (e.status === 502 || e.status === 504)) { this.polls = POLLS; }                  // C3: the act does not stop
      else if (e instanceof SignedOutError || (e instanceof HTTPError && (e.status === 401 || e.status === 429 || e.status < 500))) this.v = { ...this.v, phase: 'failed', message: plain(e) };
      else this.polls = POLLS;                                                                                              // a dropped connection or a timeout (C3)
    }
    return this.view();
  }

  /** One look at the update signal while `phase` is `updating` (C3). Anything else: the view as it is. */
  async poll(): Promise<ComputerView> {
    if (this.v.phase !== 'updating' || this.polls <= 0) return this.view();
    this.polls -= 1;
    try {
      const s = await snapshot(this.call);
      if (s.target === '') { this.polls = 0; this.v = { ...this.v, phase: 'current', message: 'Updated. Your computer restarted.', updateAvailable: false, target: '', state: s.state, epoch: s.epoch }; return this.view(); }
    } catch { /* the computer is restarting: look again */ }
    if (this.polls <= 0) this.v = { ...this.v, phase: 'idle', message: 'Still restarting. Check again in a minute.' };
    return this.view();
  }
}

/** The one Computer of this process (sources and acts share it) for the CURRENT account generation: a moved generation is a new Computer, so a late answer lands in the old one and is never shown. */
export const computer = (): Computer => {
  const g = accountGeneration();
  const st = S();
  if (!st.one || st.oneGen !== g) { st.one = new Computer(); st.oneGen = g; }
  return st.one;
};
/** For tests: a new process. */
export function forgetComputer(c?: Computer): void { S().one = c; S().oneGen = accountGeneration(); }

/** Source: the page's view; the first read of a signed-in phone asks the computer (C7: a signed-out phone sends nothing). */
export async function settingsComputer(args: { signedIn: boolean; tick: number }): Promise<ComputerView> {
  const c = computer();
  if (!args.signedIn) return empty();
  if (!c.v.checked && c.v.phase !== 'failed') return c.refresh();
  return c.view();
}

/** Mutation: refresh | wake | update | poll. The answer is the view after it. */
export async function computerAct(op: string, signedIn = true): Promise<ComputerView> {
  const c = computer();
  if (!signedIn) return empty();
  switch (op) {
    case 'refresh': return c.refresh();
    case 'wake': return c.wake();
    case 'update': return c.restartToUpdate();
    case 'poll': return c.poll();
    default: return c.view();
  }
}
