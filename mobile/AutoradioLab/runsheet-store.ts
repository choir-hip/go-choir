// The phone's copy of the run sheet: the ONE source of what plays next. Ported from RunSheetStore.swift.
// Everything that changes what is heard is an op applied here by the one reducer (runsheet-model.ts): skip, the next button, a swipe on
// the now-playing card and the lock screen's next are all `skip(on air)`; "play this" is `place now`; "queue this" is `place queued`.
// Opening Mail or a book applies nothing.
//
// The phone plays from this copy without waiting for the computer. Each op the phone makes is also kept as pending, with its op_id, and
// posted to POST /api/autoradio/sheet/ops. A 404 ("not yet") or no network leaves it pending; pending ops survive a relaunch and are
// sent when the route answers (a replay of an op_id changes nothing on the server).
//
// Ways this can fail (each has a test in runsheet-store.test.ts):
//  1. A dropped op is posted or remembered as pending: only applied ops are.
//  2. The ahead list grows past 100: the phone bounds it. A listener's own placement takes the last entry's place; the desk's is refused.
//  3. A relaunch finds an entry still "on air" (the app died mid-entry): it is closed as `failed` (an app failure, never `left` and never a
//     choice), and nothing resumes by itself.
//  4. The same op is applied twice (a retry): the reducer's op_id memory drops the second.
//  5. A post answers 404 or fails: the ops stay pending, in order, and are sent again; none is lost or sent twice after an answer.
//  6. A batch is over 64 ops or 256 KB: it is sent in several posts.
//  7. A transition whose neighbours changed is hidden or removed by the phone: it is shown stale (`isStale`), never changed by the phone.
import { currentWorld } from './api.ts';
import { boundedOp, mintId, refTooLong } from './runsheet-entries.ts';
import {
  apply, decodeOp, decodeSheet, emptySheet, onAirEntry, ahead as aheadOf, heard as heardOf, entryById,
  type At, type By, type Entry, type Op, type Outcome, type Result, type Sheet, type State,
} from './runsheet-model.ts';

export const MAX_AHEAD = 100;
export const MAX_BATCH_OPS = 64;
export const MAX_BATCH_BYTES = 256 * 1024;

/** Where an entry was when it left the air: the server block being spoken (`last_block`) and the seconds into the entry. */
export type AirReport = { block: number; seconds: number };
export const NO_REPORT: AirReport = { block: 0, seconds: 0 };

export type PostOutcome =
  | { kind: 'answered'; results: Result[] }          // the route answered 200: every op of the batch is now the server's
  | { kind: 'answeredEpoch'; results: Result[]; epoch: number }   // 200 with a `sheet_epoch`: the server's sheet was repaired; take it
  | { kind: 'notYet' }                               // 404, 501, 5xx, 429: the ops stay pending
  | { kind: 'offline' };                             // no network or the computer did not answer: the ops stay pending

export interface Poster {
  post(ops: Op[], epoch: number): Promise<PostOutcome>;
  /** GET /api/autoradio/sheet: the server's whole sheet, or undefined when it cannot be had. */
  fetchSheet(): Promise<Sheet | undefined>;
}

/** Where the store keeps itself between launches: a file's text, or nothing. */
export interface Storage { read(): Promise<string | undefined>; write(text: string): Promise<void>; remove(): Promise<void>; }

export type Requester = (path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }) => Promise<{ body: unknown }>;
const statusOf = (e: unknown): number | undefined => {
  const s = (e as { status?: unknown })?.status;
  if (typeof s === 'number') return s;
  const m = /answered (\d{3})/.exec(String((e as Error)?.message ?? ''));
  return m ? Number(m[1]) : undefined;
};

/** The run sheet route on the listener's computer (signed in: pass `authedJSON`). */
export function routePoster(request: Requester): Poster {
  return {
    async post(ops, epoch) {
      try {
        const r = await request('/api/autoradio/sheet/ops', { method: 'POST', body: epoch > 0 ? { ops, sheet_epoch: epoch } : { ops } });
        const b = (r.body ?? {}) as { results?: Result[]; sheet_epoch?: number };
        const results = Array.isArray(b.results) ? b.results : [];
        if (typeof b.sheet_epoch === 'number' && b.sheet_epoch > 0) return { kind: 'answeredEpoch', results, epoch: b.sheet_epoch };
        return { kind: 'answered', results };
      } catch (e) {
        const st = statusOf(e);
        if (st === 404 || st === 501 || st === 503 || st === 429 || (st !== undefined && st >= 500)) return { kind: 'notYet' };
        if (st !== undefined && st >= 400 && st < 500) return { kind: 'answered', results: [] };   // a body the server refuses will never be accepted: not retried
        return { kind: 'offline' };
      }
    },
    async fetchSheet() {
      try { return decodeSheet((await request('/api/autoradio/sheet')).body); } catch { return undefined; }
    },
  };
}

/** Nothing the phone posts names a place on the phone: a `file://` address, the phone's home or its temporary folder. */
export function namesDevicePath(op: Op, home = '', tmp = ''): boolean {
  const s = JSON.stringify(op);
  return s.includes('file://') || (home.length > 4 && s.includes(home)) || (tmp.length > 4 && s.includes(tmp)) || s.includes('/private/var/') || s.includes('/var/mobile/');
}

export type StoreOptions = {
  storage?: Storage; poster?: Poster;
  /** The clock for `ended.at`. The reducer reads none. */
  now?: () => Date;
  /** How long to wait before sending pending ops again after the route did not take them (ms). */
  retryAfterMs?: number;
  /** Where the phone's home and temp folders are, so a path of the phone is never posted. */
  home?: string; tmp?: string;
  /** The account generation now; an op from the desk made under an older one is refused. */
  generation?: () => number;
};

export class RunSheetStore {
  sheet: Sheet = emptySheet();
  /** Ops the phone applied that the server has not answered yet, oldest first. */
  pending: Op[] = [];
  /** The server's `sheet_epoch` the phone holds (0: none): sent with every post; when the server's differs, the phone takes the server's sheet. */
  sheetEpoch = 0;
  onChange?: () => void;
  poster?: Poster;
  now: () => Date;
  retryAfterMs: number;
  /** Tests wait on this. */
  flushTask?: Promise<void>;
  private storage?: Storage;
  private flushing = false;
  private retry?: ReturnType<typeof setTimeout>;
  private home: string; private tmp: string;
  private generation: () => number;

  constructor(o: StoreOptions = {}) {
    this.storage = o.storage; this.poster = o.poster; this.now = o.now ?? (() => new Date()); this.retryAfterMs = o.retryAfterMs ?? 30000;
    this.home = o.home ?? ''; this.tmp = o.tmp ?? ''; this.generation = o.generation ?? (() => 0);
  }

  // ---- views ----
  onAir = (): Entry | undefined => onAirEntry(this.sheet);
  ahead = (limit = Infinity): Entry[] => aheadOf(this.sheet, limit);
  heard = (limit = Infinity): Entry[] => heardOf(this.sheet, limit);
  entry = (id: string): Entry | undefined => entryById(this.sheet, id);
  /** The seconds the entries ahead will take, by their estimates. */
  get aheadSeconds(): number { return this.ahead().reduce((a, e) => a + e.est_seconds, 0); }

  /** A transition was written for two neighbours. It is stale when the entry before it or the entry after it is no longer the one it was
   *  written for. The phone only shows that; the desk's next write may rewrite it. */
  isStale(e: Entry): boolean {
    if (e.kind !== 'transition' || e.state !== 'ahead' || !e.between) return false;
    const es = this.sheet.entries, i = es.findIndex((x) => x.id === e.id);
    if (i < 0) return false;
    const before = i > 0 ? es[i - 1].id : '', after = i + 1 < es.length ? es[i + 1].id : '';
    return before !== e.between.after || after !== e.between.before;
  }

  // ---- applying ----

  /** The one way the phone changes the sheet. An applied op is kept pending and posted. */
  apply(op: Op): Result { return this.run(op, false); }

  /** An op that came from the desk's side (the feed, later the master stream): applied here, never posted back. */
  applyFromDesk(op: Op, generation?: number): Result {
    if (generation !== undefined && generation !== this.generation()) return { op_id: op.op_id, applied: false, dropped: 'stale_account', version: this.sheet.version };
    return this.run(op, true);
  }

  private run(input: Op, remote: boolean): Result {
    let op = boundedOp(input);            // the server's field bounds: cut here, never dropped there
    if (op.op === 'place' && op.entry && op.entry.state === 'ahead' && op.at !== 'now' && this.ahead().length >= MAX_AHEAD) {
      // The reducer does not cap the ahead list; the phone does. A listener's own placement is never refused: it takes the place of the
      // last entry. The desk's is refused.
      const last = this.ahead().at(-1);
      if (op.by !== 'listener' || !last) return { op_id: op.op_id, applied: false, dropped: 'ahead_full', version: this.sheet.version };
      this.run({ op_id: this.mintOpId(), by: 'phone', op: 'remove', id: last.id, base_version: 0, reason: "ahead_full: a listener's placement took the last place" }, false);
    }
    // A ref cannot be cut (it names the thing): an entry whose ref is over the server's bound is not placed, and the listener is told in plain words.
    if (!remote && op.op === 'place' && op.entry && refTooLong(op.entry)) return { op_id: op.op_id, applied: false, dropped: 'ref_too_long', version: this.sheet.version };
    if (!remote && namesDevicePath(op, this.home, this.tmp)) return { op_id: op.op_id, applied: false, dropped: 'local_path', version: this.sheet.version };
    if ((op.op === 'previous' || op.op === 'clear') && !remote) op = { ...op, base_version: this.sheet.version };
    const { sheet, result } = apply(this.sheet, op);
    if (!result.applied) return result;
    this.sheet = sheet;
    if (!remote) this.pending.push(op);
    void this.save();
    this.onChange?.();
    if (!remote) this.flush();
    return result;
  }

  mintOpId = (): string => mintId('p-');
  private stamp = (): string => this.now().toISOString().replace(/\.\d{3}Z$/, 'Z');

  // ---- the ops the app makes (each is one op with a fresh op_id) ----

  /** `place now` cuts the entry on air: the cut is reported with where it was cut (`ended.last_block`, `position_seconds`). */
  place(e: Entry, at: At, o: { after?: string; by?: By; reason?: string; cutting?: AirReport } = {}): Result {
    const by = o.by ?? e.placed_by;
    const op: Op = { op_id: this.mintOpId(), by, op: 'place', at, entry: e, base_version: 0 };
    if (o.after !== undefined) op.after = o.after;
    if (o.cutting) { op.ended = { outcome: 'cut', at: this.stamp(), last_block: o.cutting.block }; op.position_seconds = o.cutting.seconds; }
    const reason = by === 'desk' ? (o.reason ?? 'the desk wrote it') : o.reason;
    if (reason !== undefined) op.reason = reason;
    return this.apply(op);
  }
  /** A listener's removal is by the listener. The phone's own rules (a stale entry, an orphaned transition, a replaced opening) are `by: phone` with a reason. */
  remove(id: string, by: By = 'listener', reason?: string): Result {
    return this.apply({ op_id: this.mintOpId(), by, op: 'remove', id, base_version: 0, ...(reason !== undefined ? { reason } : {}) });
  }
  move(id: string, at: At, after?: string): Result {
    return this.apply({ op_id: this.mintOpId(), by: 'listener', op: 'move', at, id, base_version: 0, ...(after !== undefined ? { after } : {}) });
  }
  /** Skip, the next button, a swipe on the now-playing card, the lock screen's next and "skip" by voice: all this one op. Undefined when nothing is on air. */
  skipOnAir(report: AirReport = NO_REPORT, by: By = 'listener'): Result | undefined {
    const e = this.onAir();
    if (!e) return undefined;
    return this.apply({ op_id: this.mintOpId(), by, op: 'skip', id: e.id, base_version: 0, ended: { outcome: 'skipped', at: this.stamp(), last_block: report.block }, position_seconds: report.seconds });
  }
  previous(by: By = 'listener'): Result { return this.apply({ op_id: this.mintOpId(), by, op: 'previous', base_version: this.sheet.version }); }
  clearAhead(by: By = 'listener', reason?: string): Result {
    return this.apply({ op_id: this.mintOpId(), by, op: 'clear', base_version: this.sheet.version, ...(reason !== undefined ? { reason } : {}) });
  }
  /** The phone reports that an entry is now on air. */
  airedOnAir(id: string, positionSeconds = 0): Result {
    return this.apply({ op_id: this.mintOpId(), by: 'phone', op: 'aired', id, state: 'on_air', position_seconds: positionSeconds, base_version: 0 });
  }
  /** The phone reports how the entry on air ended: heard (it finished, or was left or cut) or skipped. */
  airedEnded(id: string, state: State, outcome: Outcome, report: AirReport = NO_REPORT, reason?: string): Result {
    return this.apply({
      op_id: this.mintOpId(), by: 'phone', op: 'aired', id, state, ended: { outcome, at: this.stamp(), last_block: report.block },
      position_seconds: report.seconds, base_version: 0, ...(reason !== undefined ? { reason } : {}),
    });
  }

  // ---- posting ----

  /** Send what is pending. One post at a time; in order; the batch is at most 64 ops and 256 KB. */
  flush(): void {
    if (this.flushing || !this.poster || this.pending.length === 0) return;
    this.flushing = true;
    this.flushTask = (async () => {
      try {
        for (;;) {
          const poster = this.poster;
          if (!poster || this.pending.length === 0) return;
          const batch = this.nextBatch();
          const out = await poster.post(batch, this.sheetEpoch);
          if (out.kind === 'answered') this.acknowledge(batch, out.results);
          else if (out.kind === 'answeredEpoch') {
            this.acknowledge(batch, out.results);
            if (out.epoch !== this.sheetEpoch) await this.takeServerSheet(poster, out.epoch);
          } else { this.scheduleRetry(); return; }
        }
      } finally { this.flushing = false; }
    })();
  }

  /** The server answered these ops. They leave the pending list. An entry the server dropped (it is not on the sheet and never was: the
   *  server's bound, a replay that could not fold it, a record too large) is taken off the local sheet, quietly. A `duplicate` is the server
   *  already having it. The version is never taken from a result. */
  acknowledge(batch: Op[], results: Result[]): void {
    const ids = new Set(batch.map((o) => o.op_id));
    this.pending = this.pending.filter((o) => !ids.has(o.op_id));
    const byId = new Map(results.map((r) => [r.op_id, r]));
    let changed = false;
    for (const op of batch) {
      if (op.op !== 'place') continue;
      const r = byId.get(op.op_id), id = op.entry?.id;
      if (!r || r.applied || !id) continue;
      if (r.dropped === 'duplicate_id') {
        // The server already has this id (a kept entry, written by the desk): its copy stays, and so does ours. A place-now put ours on air
        // here, but the server only airs an entry through `aired on_air` (its takeover rule); without it the server's copy would stay ahead
        // forever and the later end op would be refused (ahead -> heard is bad_transition). It goes first among what is pending.
        if (op.at === 'now' && this.entry(id)) this.pending.unshift({ op_id: this.mintOpId(), by: 'phone', op: 'aired', id, state: 'on_air', position_seconds: 0, base_version: 0 });
        continue;
      }
      if (r.dropped === 'duplicate') continue;
      const i = this.sheet.entries.findIndex((e) => e.id === id);
      if (i < 0 || this.sheet.entries[i].state !== 'ahead') continue;
      this.sheet.entries.splice(i, 1); changed = true;
    }
    void this.save();
    if (changed) this.onChange?.();
  }

  /** The server's sheet replaces the local one (its epoch is then the phone's); what the phone has done since and the server has not answered
   *  is applied on top again, in order. */
  async takeServerSheet(poster: Poster, epoch: number): Promise<void> {
    const theirs = await poster.fetchSheet();
    if (!theirs) return;
    this.adopt(theirs, theirs.sheet_epoch > 0 ? theirs.sheet_epoch : epoch);
  }
  /** Replace the local sheet with `theirs` (a GET, or a full `sheet` stream event) and remember its epoch; pending ops are applied again on top. */
  adopt(theirs: Sheet, epoch: number): void {
    let s = theirs;
    for (const op of this.pending) s = apply(s, op).sheet;
    this.sheet = s; this.sheetEpoch = epoch;
    void this.save(); this.onChange?.();
  }

  private nextBatch(): Op[] {
    const out: Op[] = [];
    let bytes = 0;
    for (const op of this.pending) {
      const n = new TextEncoder().encode(JSON.stringify(op)).length;
      if (out.length > 0 && (out.length >= MAX_BATCH_OPS || bytes + n > MAX_BATCH_BYTES)) break;
      out.push(op); bytes += n;
    }
    return out;
  }
  private scheduleRetry(): void {
    if (this.retry) clearTimeout(this.retry);
    this.retry = setTimeout(() => this.flush(), Math.max(0, this.retryAfterMs));
  }

  // ---- sign-out ----

  /** Sign-out, after playback stopped: wait up to `timeoutMs` for the pending ops (the "left" report among them) to reach the computer while
   *  the old key is still held. What is not answered by then is dropped by `wipe()`: pending ops are never sent under a later account. */
  async settlePending(timeoutMs: number): Promise<void> {
    this.flush();
    const end = Date.now() + timeoutMs;
    while (this.pending.length > 0 && Date.now() < end) await new Promise((r) => setTimeout(r, 50));
  }
  /** Sign-out: the sheet, its heard tail, the pending ops and the epoch go, in memory and on disk; the poster goes too, so nothing of this
   *  account is posted afterwards. */
  async wipe(): Promise<void> {
    if (this.retry) clearTimeout(this.retry);
    this.retry = undefined;
    this.sheet = emptySheet(); this.pending = []; this.sheetEpoch = 0; this.poster = undefined;
    await this.storage?.remove();
    this.onChange?.();
  }

  // ---- saving ----

  save(): Promise<void> {
    if (!this.storage) return Promise.resolve();
    return this.storage.write(JSON.stringify({ sheet: this.sheet, pending: this.pending, epoch: this.sheetEpoch })).catch(() => undefined);
  }

  /** The sheet and the pending ops from the last launch. Nothing plays. An entry that was on air when the app died is closed as `failed`, by
   *  the phone, with the reason "interrupted by a restart": a crash is not the listener leaving (`left` is sign-out) and not a choice, so it
   *  settles nothing (the server records no verdict for `failed`). A crash restart never resumes work. */
  async restore(): Promise<void> {
    const text = await this.storage?.read();
    if (!text) return;
    let saved: { sheet: unknown; pending: unknown[]; epoch?: number };
    try { saved = JSON.parse(text); this.sheet = decodeSheet(saved.sheet); this.pending = (saved.pending ?? []).map(decodeOp); this.sheetEpoch = saved.epoch ?? 0; } catch { return; }
    const e = onAirEntry(this.sheet);
    if (e) this.airedEnded(e.id, 'skipped', 'failed', NO_REPORT, 'interrupted by a restart'); else void this.save();
    this.onChange?.();
    this.flush();
  }
}

// ---- the seam to the signed-in routes, and sign-out ----

/** The store's requester over the signed-in seam (`authedJSON`): pass `routePoster(signedInRequester(authedJSON))` as the poster. HTTP errors
 *  carry `status`, which `routePoster` reads; a signed-out call (no session) is "offline": the ops stay pending and nothing is lost. */
export const signedInRequester = (authedJSON: (path: string, o?: { method?: 'GET' | 'POST'; body?: unknown; world?: unknown }) => Promise<{ body: unknown }>, world?: unknown): Requester =>
  (path, opts) => authedJSON(path, { ...opts, world: world ?? currentWorld() });
