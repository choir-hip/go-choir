// The run sheet: one ordered list of everything the listener hears, and the one reducer that applies ops to it.
// Ported one for one from the Swift reducer (RunSheet.swift). The Go twin is internal/runsheet; Go and this file read the same
// fixtures in mobile/fixtures/runsheet and must agree on every one. Pure data in, pure data out: no clock, no network, no deck.
// The wire format is JSON with snake_case keys, and these types keep the wire's names. Unknown fields are ignored; an unknown
// kind, state, op or other vocabulary word is a decode error. The reducer holds mechanical invariants only; nothing in it
// interprets meaning.
//
// Interpretations of the frozen interface (the same list is in internal/runsheet/apply.go):
//  - duplicate detection needs memory, so the sheet carries `applied_ops` (the last 512 applied op_ids); a dropped op is not remembered;
//  - dropped reasons: duplicate, not_ahead, not_on_air, not_phone, bad_transition, other_on_air (no longer produced: a takeover),
//    unknown_id, stale_version, no_previous, duplicate_id, reason_required, invalid, ahead_full;
//  - the reducer reads no clock: ended.at is whatever the op carried ("" when nothing); skip and `now` take at and last_block from op.ended;
//  - skip, previous and aired never promote an entry to on_air: only an `aired` op from the phone does;
//  - previous returns the entry on air and the latest heard or skipped entry to ahead, in place;
//  - an entry that goes on air moves to the head of the ahead entries (order: tail, on_air, ahead);
//  - heard or skipped entries beyond the most recent 50 are trimmed; the ahead list is capped at 100 only by `previous`;
//  - place needs state "ahead", a fresh id and the text its kind requires;
//  - check order: op shape, duplicate, aired's authority, reason, then the op's own rules.

export const KINDS = ['opening', 'transition', 'story', 'clip', 'take', 'episode', 'page', 'document', 'mail', 'result', 'answer'] as const;
export type Kind = (typeof KINDS)[number];
export const BYS = ['listener', 'desk', 'phone'] as const;
export type By = (typeof BYS)[number];
export const STATES = ['ahead', 'on_air', 'heard', 'skipped'] as const;
export type State = (typeof STATES)[number];
/** `failed`: the phone could not make the entry play. It is not the listener's judgement and settles nothing. */
export const OUTCOMES = ['finished', 'skipped', 'cut', 'left', 'failed'] as const;
export type Outcome = (typeof OUTCOMES)[number];
export const ROLES = ['subject', 'quoted', 'mentioned'] as const;
export type Role = (typeof ROLES)[number];
export const OPS = ['place', 'move', 'remove', 'skip', 'previous', 'clear', 'rewrite', 'aired'] as const;
export type OpName = (typeof OPS)[number];
export const ATS = ['now', 'next', 'queued', 'end', 'after'] as const;
export type At = (typeof ATS)[number];

/** Entries of these kinds carry words to speak. */
export const textRequired = (k: Kind): boolean => k === 'opening' || k === 'transition' || k === 'story' || k === 'answer' || k === 'result';
const isTail = (s: State): boolean => s === 'heard' || s === 'skipped';

export type Ended = { outcome: Outcome; at: string; last_block: number };
export type Source = { ref: string; label: string; url?: string; start?: number; end?: number; clip_id?: string };
export type Cite = { ref: string; role: Role; label: string };
export type Between = { after: string; before: string };
/** p_heard is absent when the phone holds only the claim id: the server never sends its probability or cause on the `segment` event. */
export type Forecast = { p_heard?: number; caused_by: string; claim_id: string };

export type Entry = {
  id: string; kind: Kind; placed_by: By; state: State; ended?: Ended; title: string; text?: string; source?: Source; cites?: Cite[];
  between?: Between; est_seconds: number; forecast?: Forecast;
  /** The master stream's fields (recorded, never interpreted here). */
  air_at?: string; origin?: string; revision: number;
};

export type Sheet = {
  version: number; on_air?: string; position_seconds: number; entries: Entry[];
  /** The last 512 applied op_ids, oldest first: how a repeated op_id is recognised. */
  applied_ops: string[];
  station?: string; master_version: number; clock_at?: string;
  /** The server's: present once a replay had to repair its sheet. A phone that does not hold it takes the whole sheet. */
  sheet_epoch: number;
};

export type Op = {
  op_id: string; by: By; op: OpName; at?: At; after?: string; id?: string; entry?: Entry; text?: string; state?: State; ended?: Ended;
  position_seconds?: number; base_version: number; reason?: string; origin?: string;
};

export type Result = { op_id: string; applied: boolean; dropped?: string; version: number };

export const Dropped = {
  duplicate: 'duplicate', notAhead: 'not_ahead', notOnAir: 'not_on_air', notPhone: 'not_phone', badTransition: 'bad_transition',
  otherOnAir: 'other_on_air', unknownId: 'unknown_id', staleVersion: 'stale_version', noPrevious: 'no_previous', duplicateId: 'duplicate_id',
  reasonRequired: 'reason_required', invalid: 'invalid', aheadFull: 'ahead_full',
} as const;

/** The server's field bounds (internal/runsheet/runsheet.go): an op that breaks one is dropped there as invalid, so the phone builds none. */
export const Bounds = {
  maxTitleBytes: 300, maxTextBytes: 8 * 1024, maxReasonBytes: 160, maxTail: 50, maxAppliedOps: 512,
  maxAhead: 100, maxIDBytes: 64, maxRefBytes: 512, maxURLBytes: 2048, maxCites: 12, maxShortBytes: 160, maxTimeBytes: 40,
  maxEstSeconds: 86400, maxEntryJSON: 32 * 1024,
} as const;

const enc = new TextEncoder();
const bytes = (s: string | undefined): number => (s === undefined ? 0 : enc.encode(s).length);

export class DecodeError extends Error {}

// ---- decoding: strict vocabulary, unknown fields ignored, empty strings as absent ----

type Obj = Record<string, unknown>;
const isObj = (v: unknown): v is Obj => typeof v === 'object' && v !== null && !Array.isArray(v);
function need(o: Obj, k: string, what: string): unknown {
  if (o[k] === undefined || o[k] === null) throw new DecodeError(`${what}: missing ${k}`);
  return o[k];
}
function str(v: unknown, what: string): string { if (typeof v !== 'string') throw new DecodeError(`${what}: not a string`); return v; }
function num(v: unknown, what: string): number { if (typeof v !== 'number' || !Number.isFinite(v)) throw new DecodeError(`${what}: not a number`); return v; }
function int(v: unknown, what: string): number { const n = num(v, what); if (!Number.isInteger(n)) throw new DecodeError(`${what}: not an integer`); return n; }
function word<T extends string>(v: unknown, set: readonly T[], what: string): T {
  if (typeof v !== 'string' || !(set as readonly string[]).includes(v)) throw new DecodeError(`${what}: unknown word`);
  return v as T;
}
const optStr = (o: Obj, k: string): string | undefined => (o[k] === undefined || o[k] === null ? undefined : str(o[k], k));
const optNum = (o: Obj, k: string): number | undefined => (o[k] === undefined || o[k] === null ? undefined : num(o[k], k));
const nonEmpty = (s: string | undefined): string | undefined => (s === undefined || s === '' ? undefined : s);
const strip = <T extends Obj>(o: T): T => { for (const k of Object.keys(o)) if (o[k] === undefined) delete o[k]; return o; };

export function decodeEnded(v: unknown): Ended {
  if (!isObj(v)) throw new DecodeError('ended: not an object');
  return { outcome: word(need(v, 'outcome', 'ended'), OUTCOMES, 'ended.outcome'), at: optStr(v, 'at') ?? '', last_block: v.last_block == null ? 0 : int(v.last_block, 'last_block') };
}
function decodeSource(v: unknown): Source {
  if (!isObj(v)) throw new DecodeError('source: not an object');
  return strip({ ref: optStr(v, 'ref') ?? '', label: optStr(v, 'label') ?? '', url: optStr(v, 'url'), start: optNum(v, 'start'), end: optNum(v, 'end'), clip_id: optStr(v, 'clip_id') });
}
function decodeCite(v: unknown): Cite {
  if (!isObj(v)) throw new DecodeError('cite: not an object');
  return { ref: str(need(v, 'ref', 'cite'), 'cite.ref'), role: word(need(v, 'role', 'cite'), ROLES, 'cite.role'), label: str(need(v, 'label', 'cite'), 'cite.label') };
}
function decodeForecast(v: unknown): Forecast {
  if (!isObj(v)) throw new DecodeError('forecast: not an object');
  return strip({ p_heard: optNum(v, 'p_heard'), caused_by: optStr(v, 'caused_by') ?? '', claim_id: optStr(v, 'claim_id') ?? '' });
}
export function decodeEntry(v: unknown): Entry {
  if (!isObj(v)) throw new DecodeError('entry: not an object');
  const cs = v.cites == null ? undefined : (Array.isArray(v.cites) ? v.cites.map(decodeCite) : (() => { throw new DecodeError('cites: not a list'); })());
  let between: Between | undefined;
  if (v.between != null) {
    if (!isObj(v.between)) throw new DecodeError('between: not an object');
    between = { after: str(need(v.between, 'after', 'between'), 'between.after'), before: str(need(v.between, 'before', 'between'), 'between.before') };
  }
  return strip<Entry>({
    id: str(need(v, 'id', 'entry'), 'entry.id'), kind: word(need(v, 'kind', 'entry'), KINDS, 'entry.kind'),
    placed_by: word(need(v, 'placed_by', 'entry'), BYS, 'entry.placed_by'), state: word(need(v, 'state', 'entry'), STATES, 'entry.state'),
    ended: v.ended == null ? undefined : decodeEnded(v.ended), title: optStr(v, 'title') ?? '', text: nonEmpty(optStr(v, 'text')),
    source: v.source == null ? undefined : decodeSource(v.source), cites: cs && cs.length ? cs : undefined, between,
    est_seconds: optNum(v, 'est_seconds') ?? 0, forecast: v.forecast == null ? undefined : decodeForecast(v.forecast),
    air_at: nonEmpty(optStr(v, 'air_at')), origin: nonEmpty(optStr(v, 'origin')), revision: v.revision == null ? 0 : int(v.revision, 'revision'),
  });
}
export function decodeOp(v: unknown): Op {
  if (!isObj(v)) throw new DecodeError('op: not an object');
  return strip<Op>({
    op_id: str(need(v, 'op_id', 'op'), 'op_id'), by: word(need(v, 'by', 'op'), BYS, 'by'), op: word(need(v, 'op', 'op'), OPS, 'op'),
    at: v.at == null ? undefined : word(v.at, ATS, 'at'), after: optStr(v, 'after'), id: optStr(v, 'id'),
    entry: v.entry == null ? undefined : decodeEntry(v.entry), text: optStr(v, 'text'), state: v.state == null ? undefined : word(v.state, STATES, 'state'),
    ended: v.ended == null ? undefined : decodeEnded(v.ended), position_seconds: optNum(v, 'position_seconds'),
    base_version: v.base_version == null ? 0 : int(v.base_version, 'base_version'), reason: optStr(v, 'reason'), origin: nonEmpty(optStr(v, 'origin')),
  });
}
export function decodeSheet(v: unknown): Sheet {
  if (!isObj(v)) throw new DecodeError('sheet: not an object');
  if (v.entries != null && !Array.isArray(v.entries)) throw new DecodeError('entries: not a list');
  if (v.applied_ops != null && !Array.isArray(v.applied_ops)) throw new DecodeError('applied_ops: not a list');
  return strip<Sheet>({
    version: v.version == null ? 0 : int(v.version, 'version'), on_air: nonEmpty(optStr(v, 'on_air')),
    position_seconds: optNum(v, 'position_seconds') ?? 0, entries: ((v.entries as unknown[] | undefined) ?? []).map(decodeEntry),
    applied_ops: ((v.applied_ops as unknown[] | undefined) ?? []).map((x) => str(x, 'applied_ops')),
    station: nonEmpty(optStr(v, 'station')), master_version: v.master_version == null ? 0 : int(v.master_version, 'master_version'),
    clock_at: nonEmpty(optStr(v, 'clock_at')), sheet_epoch: v.sheet_epoch == null ? 0 : int(v.sheet_epoch, 'sheet_epoch'),
  });
}
export const emptySheet = (): Sheet => ({ version: 0, position_seconds: 0, entries: [], applied_ops: [], master_version: 0, sheet_epoch: 0 });

// ---- views: Up next, Previously and now-playing read these ----

export const onAirEntry = (s: Sheet): Entry | undefined => s.entries.find((e) => e.state === 'on_air');
export const ahead = (s: Sheet, limit = Infinity): Entry[] => s.entries.filter((e) => e.state === 'ahead').slice(0, limit);
/** The most recent first. */
export const heard = (s: Sheet, limit = Infinity): Entry[] => s.entries.filter((e) => isTail(e.state)).reverse().slice(0, limit);
export const entryById = (s: Sheet, id: string): Entry | undefined => s.entries.find((e) => e.id === id);

/** [A-Za-z0-9][A-Za-z0-9_.-]{0,63} */
export function validId(s: string): boolean {
  const b = enc.encode(s);
  if (b.length === 0 || b.length > 64) return false;
  for (let i = 0; i < b.length; i++) {
    const c = b[i];
    const alnum = (c >= 0x61 && c <= 0x7a) || (c >= 0x41 && c <= 0x5a) || (c >= 0x30 && c <= 0x39);
    if (alnum || (i > 0 && (c === 0x5f || c === 0x2e || c === 0x2d))) continue;
    return false;
  }
  return true;
}

const validEnded = (e: Ended): boolean => bytes(e.at) <= Bounds.maxTimeBytes && e.last_block >= 0;

/** The entry's size as a record carries it (the server's bound, fixture 33): the entry as compact JSON, marshalled as a JSON string (as a
 *  record body is), minus the two quotes. A plain byte costs 1; `<` `>` `&` 6; a quote or a backslash 2 more than the plain byte it
 *  escapes (a quote or backslash in the text is `\"` inside the entry and so 4 in all); U+2028 and U+2029 7. */
export function entryJSONSize(e: Entry): number {
  const wire = strip<Obj>({
    air_at: e.air_at, origin: e.origin, revision: e.revision !== 0 ? e.revision : undefined,
    id: e.id, kind: e.kind, placed_by: e.placed_by, state: e.state, ended: e.ended, title: e.title, text: e.text, source: e.source, cites: e.cites,
    between: e.between, est_seconds: e.est_seconds, forecast: e.forecast,
  });
  const inner = JSON.stringify(wire);
  let n = 0;
  for (const ch of inner) {
    const u = ch.codePointAt(0) as number;
    if (u === 0x22 || u === 0x5c) n += 2;
    else if (u === 0x3c || u === 0x3e || u === 0x26) n += 6;
    else if (u === 0x0a || u === 0x0d || u === 0x09) n += 2;
    else if (u < 0x20) n += 6;
    else if (u === 0x2028 || u === 0x2029) n += 7;
    else n += enc.encode(ch).length;
  }
  return n;
}

/** Every string, list and number of an entry is bounded (internal/runsheet validEntry); the server drops an entry that breaks one. */
export function validEntry(e: Entry): boolean {
  const B = Bounds;
  if (!validId(e.id) || bytes(e.title) > B.maxTitleBytes || bytes(e.text) > B.maxTextBytes) return false;
  if (textRequired(e.kind) && !(e.text ?? '')) return false;
  if (e.ended && !validEnded(e.ended)) return false;
  if (!(e.est_seconds >= 0 && e.est_seconds <= B.maxEstSeconds) || e.revision < 0 || bytes(e.air_at) > B.maxTimeBytes || bytes(e.origin) > B.maxShortBytes) return false;
  const s = e.source;
  if (s) {
    if (bytes(s.ref) > B.maxURLBytes || bytes(s.label) > B.maxRefBytes || bytes(s.url) > B.maxURLBytes || bytes(s.clip_id) > B.maxRefBytes) return false;
    if ((s.start ?? 0) < 0 || (s.end ?? 0) < 0) return false;
  }
  if ((e.cites?.length ?? 0) > B.maxCites) return false;
  for (const c of e.cites ?? []) if (bytes(c.ref) > B.maxURLBytes || bytes(c.label) > B.maxRefBytes) return false;
  if (e.between && (bytes(e.between.after) > B.maxIDBytes || bytes(e.between.before) > B.maxIDBytes)) return false;
  const f = e.forecast;
  if (f && (!((f.p_heard ?? 0) >= 0 && (f.p_heard ?? 0) <= 1) || bytes(f.caused_by) > B.maxShortBytes || bytes(f.claim_id) > B.maxShortBytes)) return false;
  return entryJSONSize(e) <= B.maxEntryJSON;
}

// ---- the rules ----

const firstAhead = (s: Sheet): number => {
  const oi = s.entries.findIndex((e) => e.state === 'on_air');
  return oi >= 0 ? oi + 1 : s.entries.filter((e) => isTail(e.state)).length;
};

function slot(s: Sheet, at: At, after: string | undefined): number {
  const first = firstAhead(s);
  switch (at) {
    case 'next': case 'now': return first;
    case 'end': return s.entries.length;
    case 'queued': {
      let last = -1;
      for (let i = first; i < s.entries.length; i++) if (s.entries[i].state === 'ahead' && s.entries[i].placed_by === 'listener') last = i;
      return last < 0 ? first : last + 1;
    }
    case 'after': {
      const i = s.entries.findIndex((e) => e.id === after);
      if (after && i >= 0 && (s.entries[i].state === 'ahead' || s.entries[i].state === 'on_air')) return i + 1;
      return first;
    }
  }
}

const endedFor = (op: Op, outcome: Outcome): Ended => ({ outcome, at: op.ended?.at ?? '', last_block: op.ended?.last_block ?? 0 });

function putAhead(s: Sheet, entry: Entry, at: At, after: string | undefined, op: Op): void {
  const e = { ...entry };
  if (at !== 'now') { s.entries.splice(slot(s, at, after), 0, e); return; }
  const oi = s.entries.findIndex((x) => x.state === 'on_air');
  if (oi >= 0) { s.entries[oi].state = 'skipped'; s.entries[oi].ended = endedFor(op, 'cut'); }
  e.state = 'on_air';
  delete e.ended;
  s.entries.splice(firstAhead(s), 0, e);
  s.on_air = e.id;
  s.position_seconds = 0;
}

/** An answer the phone already said: the one place that may arrive in a state other than ahead. */
const isHeardAnswer = (op: Op): boolean => !!op.entry && op.entry.kind === 'answer' && op.entry.placed_by === 'phone' && op.entry.state === 'heard';

/** Desk-placed ahead entries farthest from the air are dropped until at most `maxAhead` are ahead (never the ones `keep` names, never a
 *  listener's or the phone's); false when that cannot be done. */
function trimAhead(s: Sheet, keep: Set<string>): boolean {
  for (;;) {
    let n = 0, far = -1;
    s.entries.forEach((e, i) => { if (e.state === 'ahead') { n++; if (e.placed_by === 'desk' && !keep.has(e.id)) far = i; } });
    if (n <= Bounds.maxAhead) return true;
    if (far < 0) return false;
    s.entries.splice(far, 1);
  }
}

function performAired(s: Sheet, op: Op): string | undefined {
  const to = op.state;
  if (!op.id || !to || to === 'ahead') return Dropped.invalid;
  const i = s.entries.findIndex((e) => e.id === op.id);
  if (i < 0) return Dropped.unknownId;
  const cur = s.entries[i].state;
  if (cur === 'ahead' && to === 'on_air') {
    // Takeover: the entry on air lost its end op; close it as heard, outcome left, no time, no block.
    const oi = s.entries.findIndex((e) => e.state === 'on_air');
    if (oi >= 0) { s.entries[oi].state = 'heard'; s.entries[oi].ended = { outcome: 'left', at: '', last_block: 0 }; delete s.on_air; }
    const [e] = s.entries.splice(i, 1);
    e.state = 'on_air';
    delete e.ended;
    s.entries.splice(firstAhead(s), 0, e);
    s.on_air = e.id;
    s.position_seconds = op.position_seconds ?? 0;
  } else if (cur === 'on_air' && (to === 'heard' || to === 'skipped')) {
    s.entries[i].state = to;
    s.entries[i].ended = op.ended ?? { outcome: to === 'heard' ? 'finished' : 'skipped', at: '', last_block: 0 };
    delete s.on_air;
    s.position_seconds = 0;
  } else return Dropped.badTransition;
  return undefined;
}

/** Performs `op` on this private copy; returns the dropped reason, or undefined when it applied. */
function perform(s: Sheet, op: Op): string | undefined {
  switch (op.op) {
    case 'place': {
      if (isHeardAnswer(op)) {
        const e = op.entry as Entry;
        if (op.by !== 'phone' || !validEntry(e)) return Dropped.invalid;
        if (s.entries.some((x) => x.id === e.id)) return Dropped.duplicateId;
        const h = { ...e };
        if (!h.ended) h.ended = { outcome: 'finished', at: '', last_block: 0 };
        s.entries.splice(s.entries.filter((x) => isTail(x.state)).length, 0, h);   // after the last heard or skipped entry; never on air
        return undefined;
      }
      const e = op.entry, at = op.at;
      if (!e || !validEntry(e) || e.state !== 'ahead' || !at || (at === 'after' && !op.after)) return Dropped.invalid;
      if (s.entries.some((x) => x.id === e.id)) return Dropped.duplicateId;
      putAhead(s, e, at, op.after, op);
      return undefined;
    }
    case 'move': {
      const at = op.at;
      if (!op.id || !at || (at === 'after' && !op.after)) return Dropped.invalid;
      const i = s.entries.findIndex((x) => x.id === op.id);
      if (i < 0 || s.entries[i].state !== 'ahead') return Dropped.notAhead;
      const [e] = s.entries.splice(i, 1);
      putAhead(s, e, at, op.after, op);
      return undefined;
    }
    case 'remove': {
      if (!op.id) return Dropped.invalid;
      const i = s.entries.findIndex((x) => x.id === op.id);
      if (i < 0 || s.entries[i].state !== 'ahead') return Dropped.notAhead;
      s.entries.splice(i, 1);
      return undefined;
    }
    case 'rewrite': {
      if (!op.id || !op.text || bytes(op.text) > Bounds.maxTextBytes) return Dropped.invalid;
      const i = s.entries.findIndex((x) => x.id === op.id);
      if (i < 0 || s.entries[i].state !== 'ahead') return Dropped.notAhead;
      if (entryJSONSize({ ...s.entries[i], text: op.text }) > Bounds.maxEntryJSON) return Dropped.invalid;   // after every rewrite the entry is within its bound
      s.entries[i].text = op.text;
      return undefined;
    }
    case 'skip': {
      if (!op.id) return Dropped.invalid;
      const i = s.entries.findIndex((x) => x.id === op.id);
      if (i < 0 || s.entries[i].state !== 'on_air') return Dropped.notOnAir;
      s.entries[i].state = 'skipped';
      s.entries[i].ended = endedFor(op, 'skipped');
      delete s.on_air;
      s.position_seconds = 0;
      return undefined;
    }
    case 'previous': {
      if (op.base_version !== s.version) return Dropped.staleVersion;
      let last = -1;
      s.entries.forEach((e, i) => { if (isTail(e.state)) last = i; });
      if (last < 0) return Dropped.noPrevious;
      const restored = new Set<string>();
      s.entries.forEach((e, i) => {
        if (i === last || e.state === 'on_air') { e.state = 'ahead'; delete e.ended; restored.add(e.id); }
      });
      delete s.on_air;
      s.position_seconds = 0;
      return trimAhead(s, restored) ? undefined : Dropped.aheadFull;
    }
    case 'clear':
      if (op.base_version !== s.version) return Dropped.staleVersion;
      s.entries = s.entries.filter((e) => e.state !== 'ahead');
      return undefined;
    case 'aired':
      return performAired(s, op);
  }
}

function trimTail(s: Sheet): void {
  let drop = s.entries.filter((e) => isTail(e.state)).length - Bounds.maxTail;
  if (drop <= 0) return;
  s.entries = s.entries.filter((e) => { if (drop > 0 && isTail(e.state)) { drop--; return false; } return true; });
}

/** Applies `op` and returns the new sheet and what happened. A dropped op returns the sheet unchanged (the same object). */
export function apply(sheet: Sheet, op: Op): { sheet: Sheet; result: Result } {
  const drop = (why: string) => ({ sheet, result: { op_id: op.op_id, applied: false, dropped: why, version: sheet.version } as Result });
  if (!validId(op.op_id)) return drop(Dropped.invalid);
  if (sheet.applied_ops.includes(op.op_id)) return drop(Dropped.duplicate);
  if (op.op === 'aired' && op.by !== 'phone') return drop(Dropped.notPhone);
  const reason = op.reason ?? '';
  if (bytes(reason) > Bounds.maxReasonBytes || bytes(op.id) > Bounds.maxIDBytes || bytes(op.after) > Bounds.maxIDBytes
    || bytes(op.origin) > Bounds.maxShortBytes || (op.ended !== undefined && !validEnded(op.ended))
    || (op.position_seconds !== undefined && (op.position_seconds < 0 || op.position_seconds > Bounds.maxEstSeconds))
    || (op.entry !== undefined && op.op !== 'place') || (op.text !== undefined && op.op !== 'rewrite')) return drop(Dropped.invalid);   // an op carries nothing it does not use
  if (op.by === 'desk' && reason === '') return drop(Dropped.reasonRequired);
  const out = structuredClone(sheet);
  const why = perform(out, op);
  if (why) return drop(why);
  trimTail(out);
  out.version = sheet.version + 1;
  out.applied_ops.push(op.op_id);
  if (out.applied_ops.length > Bounds.maxAppliedOps) out.applied_ops.splice(0, out.applied_ops.length - Bounds.maxAppliedOps);
  return { sheet: out, result: { op_id: op.op_id, applied: true, version: out.version } };
}

export function applyAll(sheet: Sheet, ops: Op[]): { sheet: Sheet; results: Result[] } {
  let s = sheet;
  const results: Result[] = [];
  for (const op of ops) { const r = apply(s, op); s = r.sheet; results.push(r.result); }
  return { sheet: s, results };
}
