// What the phone posts from the deck's journal: the ops that put an item on air and say how it left, built from the deck's own lines with the
// deck's own times, and posted in order, acknowledged only when the server answered. The deck is the evidence (docs/reference/exact/deck.md):
// an item is heard only if the deck says sound was rendered; nothing here decides what plays.
// STATE AT THE 2026-11-19 INTEGRATION BRANCH: nothing in shared code calls this yet. The ops actually posted are the Swift RunSheetStore's, and
// the verdict it posts comes from the player's own `played` flag (RunSheetPlayer.finish), not from the journal's `rendered`; the journal's
// `rendered` is what this file reads, and the two are meant to agree. When shared code posts, the Swift poster goes in the SAME commit.
//
// Contract with the server (internal/runsheet/apply.go `aired`, internal/agentcore/api_autoradio_settle.go): heard + finished settles a forecast
// true; skipped (skipped, cut) settles it false; heard + left, skipped + failed settle nothing ("no verdict"); an `aired on_air` for an item
// while another is on air closes the other as left, which is a lost verdict, so an op never leaves before the end of the one before it; the
// server judges lateness on `ended.at`, so the deck's time goes into the op, never the time of the post; an op_id the server has seen is
// dropped as a duplicate, so a resend counts nothing twice. The fixtures in fixtures/deck are applied by the Go reducer
// (internal/runsheet/deck_fixtures_test.go) and read here.
//
// Ways this can fail, and what is done (each has a test in deck-ops.test.ts):
//  - sound that was not made counted as heard: only `rendered` with `finished` is heard.
//  - a verdict lost to a takeover: a start while another item is open is not built, and the build says so.
//  - a verdict counted twice: op ids come from the line (its number and time), so a resend is the same op.
//  - a verdict lost because the post failed: the deck's journal is acknowledged only for what the server answered.
//  - a post that never ends: a refused or unknown op is acknowledged (resending cannot help) and counted.
// Logs carry counts and ids, never the words of an entry or of a failure.

import { scoped } from './api.ts';
import { DECK_HELD_MAX, type DeckEvent, type DeckItem, type DeckPort } from './deck.ts';

/** The server's body limit and batch limit (api_autoradio_sheet.go: sheetMaxBody, sheetMaxOps). */
export const BATCH_OPS = 64;
export const BATCH_BYTES = 256 * 1024;
const MAX_REASON_BYTES = 160;
const MAX_POSITION = 86400;

export type WireOp = {
  op_id: string;
  by: 'phone';
  op: 'aired';
  id: string;
  state: 'on_air' | 'heard' | 'skipped';
  ended?: { outcome: 'finished' | 'skipped' | 'cut' | 'left' | 'failed'; at: string; last_block: number };
  position_seconds: number;
  base_version: 0;
  reason?: string;
};

export type Built = {
  ops: WireOp[];
  /** The journal number each op came from, in step with `ops`. */
  seqs: number[];
  /** The last journal number read: everything up to it is either an op above or a line that makes none. */
  through: number;
  /** "gap:<missing number>", "start_before_end:<number>", "dropped:<number of the deck's own note that the cap dropped lines>". */
  anomalies: string[];
};

const t36 = (at: string): string => {
  const ms = Date.parse(at);
  return Number.isFinite(ms) ? Math.floor(ms / 1000).toString(36) : '0';
};

/** `j<number>.<time in base 36>.<item>`, kept to the characters and the 64 an op id may have. */
export function opId(line: DeckEvent): string {
  const item = line.item.replace(/[^A-Za-z0-9_.-]/g, '-');
  return `j${line.seq}.${t36(line.at)}.${item}`.slice(0, 64);
}

const time = (at: string): string => (Number.isFinite(Date.parse(at)) && at.length <= 40 ? at : '');
const secs = (n: number): number => (Number.isFinite(n) ? Math.min(Math.max(n, 0), MAX_POSITION) : 0);
const block = (n: number): number => (Number.isFinite(n) && n > 0 ? Math.floor(n) : 0);
function reasonOf(s: string): string {
  const e = new TextEncoder();
  if (e.encode(s).length <= MAX_REASON_BYTES) return s;
  let out = '';
  for (const ch of s) { if (e.encode(out + ch).length > MAX_REASON_BYTES) break; out += ch; }
  return out;
}

function ended(line: DeckEvent): WireOp {
  const base = { op_id: opId(line), by: 'phone' as const, op: 'aired' as const, id: line.item, base_version: 0 as const };
  const done = (state: 'heard' | 'skipped', outcome: NonNullable<WireOp['ended']>['outcome'], last: number, pos: number, reason = ''): WireOp => ({
    ...base, state, ended: { outcome, at: time(line.at), last_block: block(last) }, position_seconds: secs(pos), ...(reason ? { reason: reasonOf(reason) } : {}),
  });
  switch (line.end) {
    case 'finished':
      // Heard only with sound. A deck that finishes with none (muted, dead engine) is "could not play", which the server settles as no verdict.
      return line.rendered ? done('heard', 'finished', line.final, line.seconds) : done('skipped', 'failed', 0, 0, 'ended with no sound rendered');
    case 'skipped': return done('skipped', 'skipped', line.block, line.seconds);
    case 'replaced': return done('skipped', 'cut', line.block, line.seconds);
    case 'stopped':
      return line.rendered ? done('heard', 'left', line.block, line.seconds) : done('skipped', 'failed', 0, 0, 'stopped before it made a sound');
    default: return done('skipped', 'failed', line.block, line.seconds, line.reason || 'could not play');
  }
}

/** The ops for the journal lines after number `after`, in order. Pure. */
export function opsFromJournal(lines: DeckEvent[], after = 0): Built {
  const ops: WireOp[] = [];
  const seqs: number[] = [];
  const anomalies: string[] = [];
  let through = after;
  let open = '';
  for (const line of lines) {
    if (line.seq <= after) continue;
    if (line.seq !== through + 1) anomalies.push(`gap:${through + 1}`);       // the first unacknowledged line too: the cap's loss starts there
    if (line.type === 'system' && line.command === 'dropped') anomalies.push(`dropped:${line.seq}`);   // the deck says it dropped unacknowledged lines
    if (line.type === 'started') {
      if (open === line.item) { through = line.seq; continue; }
      if (open !== '') { anomalies.push(`start_before_end:${line.seq}`); break; }
      open = line.item;
      ops.push({ op_id: opId(line), by: 'phone', op: 'aired', id: line.item, state: 'on_air', position_seconds: secs(line.seconds), base_version: 0 });
      seqs.push(line.seq);
    } else if (line.type === 'ended') {
      if (open === line.item) open = '';
      ops.push(ended(line));
      seqs.push(line.seq);
    }
    through = line.seq;
  }
  return { ops, seqs, through, anomalies };
}

// ---- keeping the deck's held items in the sheet's order

export type HeldPlan = { keep: number; enqueue: { at: number; item: DeckItem }[] };

/** What to do so the deck holds `wanted` (the next items, in the sheet's order) when it holds the ids `held`: keep what matches from the front,
 *  and hold the rest again in order from the first difference. At most DECK_HELD_MAX, each id once. */
export function heldPlan(held: string[], wanted: DeckItem[], max = DECK_HELD_MAX): HeldPlan {
  const want: DeckItem[] = [];
  for (const w of wanted) if (want.length < max && !want.some((x) => x.id === w.id)) want.push(w);
  let keep = 0;
  while (keep < held.length && keep < want.length && held[keep] === want[keep].id) keep++;
  return { keep, enqueue: want.slice(keep).map((item, i) => ({ at: keep + i, item })) };
}

/** Make the deck hold `wanted`. The first `enqueue` already drops what follows its place, so a trim is only for a shorter list. */
export async function syncHeld(port: DeckPort, wanted: DeckItem[]): Promise<void> {
  const s = await port.state();
  const plan = heldPlan(s.held, wanted);
  if (plan.enqueue.length === 0) {
    if (s.held.length > plan.keep) await port.act('trim', plan.keep);
    return;
  }
  for (const e of plan.enqueue) await port.enqueue(e.item, e.at);
}

// ---- posting

export type OpResult = { op_id: string; applied: boolean; dropped?: string };
export type PostOutcome =
  /** The route answered 200: every op of the batch is now the server's. */
  | { kind: 'answered'; results: OpResult[] }
  /** 200 with a `sheet_epoch` that is not the one the phone holds: the server's sheet was repaired and the phone must take it. */
  | { kind: 'answeredEpoch'; results: OpResult[]; epoch: number }
  /** 404, 501, 503, 429 or a 5xx: not there yet or busy. The ops stay in the journal. */
  | { kind: 'notYet' }
  /** No network. The ops stay in the journal. */
  | { kind: 'offline' }
  /** Another 4xx: the body will never be accepted. Not retried. */
  | { kind: 'refused' };

export type Poster = { post(ops: WireOp[], epoch: number): Promise<PostOutcome> };

/** The HTTP answer of POST /api/autoradio/sheet/ops as the Swift RouteSheetPoster read it. `status` is null when nothing answered. */
export function outcomeFromHTTP(status: number | null, body: unknown): PostOutcome {
  if (status === null) return { kind: 'offline' };
  if (status === 404 || status === 501 || status === 503 || status === 429 || status >= 500) return { kind: 'notYet' };
  if (status < 200 || status >= 300) return { kind: 'refused' };
  const o = body && typeof body === 'object' ? (body as { results?: unknown; sheet_epoch?: unknown }) : {};
  const results = Array.isArray(o.results) ? (o.results as OpResult[]) : [];
  if (typeof o.sheet_epoch === 'number' && o.sheet_epoch > 0) return { kind: 'answeredEpoch', results, epoch: o.sheet_epoch };
  return { kind: 'answered', results };
}

export type PostOptions = { epoch?: number; maxBytes?: number; log?: (line: string) => void };
export type PostReport = {
  busy: boolean;
  /** The ops the server answered. */
  posted: number;
  /** The journal number acknowledged to the deck. */
  acked: number;
  /** Journal lines still waiting (not acknowledged). */
  pending: number;
  /** The reasons the server dropped ops, counted. */
  dropped: Record<string, number>;
  /** Ops in a batch the server refused outright. */
  refused: number;
  /** Set when the server's sheet was repaired: take it before posting more. */
  epoch?: number;
  anomalies: string[];
};

const P = scoped(() => ({ posting: false }));      // module state belongs to the world (api.ts `scoped`)

/**
 * Post the deck's unacknowledged journal, oldest first, and acknowledge what the server answered. One at a time: a second call while one
 * runs does nothing (`busy`). A batch that was not answered stops the rest and leaves its lines in the deck's journal for the next call.
 */
export async function postJournal(port: DeckPort, poster: Poster, opts: PostOptions = {}): Promise<PostReport> {
  const report: PostReport = { busy: false, posted: 0, acked: 0, pending: 0, dropped: {}, refused: 0, anomalies: [] };
  if (P().posting) return { ...report, busy: true };
  P().posting = true;
  try {
    const log = opts.log ?? (() => {});
    const maxBytes = opts.maxBytes ?? BATCH_BYTES;
    const s = await port.state();
    report.acked = s.acked;
    const built = opsFromJournal(s.journal, s.acked);
    report.anomalies = built.anomalies;
    let i = 0;
    let stopped = false;
    while (i < built.ops.length && !stopped) {
      let j = i;
      while (j < built.ops.length && j - i < BATCH_OPS && JSON.stringify({ ops: built.ops.slice(i, j + 1), sheet_epoch: opts.epoch ?? 0 }).length <= maxBytes) j++;
      if (j === i) j = i + 1; // one op alone is over the limit: send it and let the server say
      const batch = built.ops.slice(i, j);
      const out = await poster.post(batch, opts.epoch ?? 0);
      if (out.kind === 'notYet' || out.kind === 'offline') { stopped = true; break; }
      if (out.kind === 'refused') report.refused += batch.length;
      else {
        report.posted += batch.length;
        for (const r of out.results) if (r.dropped) report.dropped[r.dropped] = (report.dropped[r.dropped] ?? 0) + 1;
      }
      const upTo = j < built.ops.length ? built.seqs[j - 1] : built.through;
      await port.ack(upTo);
      report.acked = upTo;
      i = j;
      if (out.kind === 'answeredEpoch') { report.epoch = out.epoch; stopped = true; }
    }
    if (built.ops.length === 0 && built.through > s.acked) {
      await port.ack(built.through);
      report.acked = built.through;
    }
    report.pending = s.journal.filter((l) => l.seq > report.acked).length;
    const dropped = Object.entries(report.dropped).map(([k, v]) => `${k}:${v}`).join(',');
    log(`deck journal posted=${report.posted} acked=${report.acked} pending=${report.pending} refused=${report.refused} dropped=${dropped || 'none'} anomalies=${report.anomalies.join(',') || 'none'}`);
    return report;
  } finally {
    P().posting = false;
  }
}
