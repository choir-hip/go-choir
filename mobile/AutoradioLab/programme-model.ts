// The programme the desk writes, as the phone reads it: the stretch stream (server-sent events or one JSON object per line), the server's
// block split, a segment's title and cites, the entry a segment becomes, and the writer's rules for when to ask for more. Ported from
// RadioProgramme.swift and RunSheetFeed.swift (the decisions; the network is `programme.ts`'s). Pure: bytes and plain data in, plain data out.
//
// What the phone does with a stretch: read its lines (StretchDecoder), make each segment an entry placed at the end by the desk (`entryFrom`,
// `placeOp`), and keep the sheet fed (`aheadRule`). The server validates; the phone drops only what it cannot speak.
import { cutBytes } from './runsheet-entries.ts';
import { estimateSeconds } from './runsheet-entries.ts';
import { Bounds, type Entry, type Forecast, type Op } from './runsheet-model.ts';

// Hoisted: a regex literal with a \p{...} class is compiled at each evaluation by Hermes (the device's engine), and these ran once per character;
// on the simulator that took paperFront 390 ms against a 100 ms turn. Compile once, here.
const LETTER_OR_DIGIT = /[\p{L}\p{N}]/u;
const CONTROL = /\p{Cc}/u;

// ---- segments ----

export type Cite = { ref: string; role: string; label: string };
export type ClipRef = { clip_id: string; story_ref: string; seconds: number };
export type Incoming = {
  stretch_id: string; segment_id: string; kind: string; title: string; text: string; cites: Cite[];
  /** The server's `block_count`, when sent. */
  block_count?: number; clip?: ClipRef; between?: { after: string; before: string };
  take_id?: string; forecast?: Forecast; entry_id?: string; claim_id?: string;
};

export const SEGMENT_LABELS: Record<string, string> = {
  story: 'Story', headlines: 'Headlines', update: 'Update', mail: 'Mail', podcast_note: 'Podcast', answer: 'Answer', result: 'Result', bridge: 'Bridge',
  ident: 'Station break', podcast_clip: 'Podcast clip', opening: 'Opening', transition: 'Transition', take: 'Take',
};
export const isKnownKind = (k: string): boolean => Object.prototype.hasOwnProperty.call(SEGMENT_LABELS, k);
export const kindLabel = (k: string): string => SEGMENT_LABELS[k] ?? '';

export type StretchEvent = { type: 'segment'; segment: Incoming } | { type: 'done' } | { type: 'error'; message: string };

/** A take id: `[A-Za-z0-9_-]{1,64}`. */
export const validTakeID = (s: string): boolean => /^[A-Za-z0-9_-]{1,64}$/.test(s);

// ---- the server's block split (learning engine design 12.3), mirrored ----

const isSpaceCP = (u: number): boolean =>
  (u >= 0x09 && u <= 0x0d) || u === 0x20 || u === 0x85 || u === 0xa0 || u === 0x1680 || (u >= 0x2000 && u <= 0x200a) || u === 0x2028 || u === 0x2029 || u === 0x202f || u === 0x205f || u === 0x3000;

/** Line endings to LF (a lone CR is one too), split on one or more blank lines (a line of only whitespace is blank), trim each part, drop
 *  empty parts. It works on code points, as Go's strings do. Go's `unicode.IsSpace` is what trims. */
export function splitBlocks(text: string): string[] {
  const trim = (cps: number[]): number[] => { let a = 0, b = cps.length; while (a < b && isSpaceCP(cps[a])) a++; while (b > a && isSpaceCP(cps[b - 1])) b--; return cps.slice(a, b); };
  const lf: number[] = [];
  let prevCR = false;
  for (const ch of text) {
    const u = ch.codePointAt(0) as number;
    if (u === 0x0a) { if (!prevCR) lf.push(0x0a); prevCR = false; }
    else if (u === 0x0d) { lf.push(0x0a); prevCR = true; }
    else { lf.push(u); prevCR = false; }
  }
  const out: string[] = [];
  let cur: number[][] = [], line: number[] = [];
  const flush = () => {
    const joined: number[] = [];
    cur.forEach((l, i) => { if (i > 0) joined.push(0x0a); joined.push(...l); });
    const part = trim(joined);
    if (part.length) out.push(String.fromCodePoint(...part));
    cur = [];
  };
  const endLine = () => { if (line.every(isSpaceCP)) flush(); else cur.push(line); line = []; };
  for (const u of lf) { if (u === 0x0a) endLine(); else line.push(u); }
  endLine(); flush();
  return out;
}

// ---- titles and labels ----

const STRIPPED = new Set<number>([0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069, 0x200e, 0x200f, 0x061c, 0x200b, 0xfeff]);
const MAX_TITLE_BYTES = 200;
/** One clean line: controls and newlines become spaces, format characters (zero-width, bidi overrides) are dropped, runs of space collapse,
 *  capped in bytes at a character boundary. Joiners (ZWJ, ZWNJ) are part of the text and stay. */
export function oneLine(s: string, maxBytes = MAX_TITLE_BYTES): string {
  let t = '';
  for (const ch of s) {
    const u = ch.codePointAt(0) as number;
    if (CONTROL.test(ch)) t += ' ';
    else if (STRIPPED.has(u)) continue;
    else t += ch;
  }
  t = t.replace(/\s+/gu, ' ').trim();
  return cutBytes(t, maxBytes).trim();
}
/** `pub:harbour-1`, `mail:abc`: a ref, which is never shown to the listener. */
export const looksLikeRef = (s: string): boolean => /^[a-z][a-z_]{1,11}:\S+$/.test(s);
export function title(raw: string, kind: string): string {
  const t = oneLine(raw);
  // A bridge has no name of its own to show: the programme's, never the word "Bridge".
  return t === '' || looksLikeRef(t) ? (kind === 'bridge' || kind === 'transition' ? 'Autoradio' : kindLabel(kind)) : t;
}
export function label(raw: string): string { const t = oneLine(raw); return looksLikeRef(t) ? '' : t; }

// ---- the stream ----

/** Reads a stretch as it arrives: server-sent events (`event:` and `data:` lines) or one JSON object per line (the fixtures). Chunks may end
 *  anywhere. Line endings are LF, CRLF or a lone CR. Anything it cannot use is counted and skipped. */
export class StretchDecoder {
  ignoredCount = 0;
  droppedCount = 0;
  private line = '';
  private afterCR = false;
  private sseName = 'message';
  private stretchID = 'local';
  private seen = 0;
  private pendingEvent?: string;

  feed(chunk: string): StretchEvent[] {
    const out: StretchEvent[] = [];
    for (const ch of chunk) {
      if (ch === '\n') {
        if (this.afterCR) { this.afterCR = false; continue; }          // the LF of a CRLF whose CR already ended the line
        out.push(...this.endLine());
      } else if (ch === '\r') { out.push(...this.endLine()); this.afterCR = true; }
      else { this.afterCR = false; this.line += ch; }
    }
    return out;
  }
  /** The stream ended: a last line with no line ending still counts. */
  finish(): StretchEvent[] {
    this.afterCR = false;
    const out = this.line === '' ? [] : this.endLine();
    out.push(...this.flushPending());
    return out;
  }
  /** An `event:` line that no `data:` line followed (a bare `event: done`): dispatched when the next line that is not a comment or its data
   *  arrives, or when the stream ends. */
  private flushPending(): StretchEvent[] {
    const p = this.pendingEvent;
    if (p === undefined) return [];
    this.pendingEvent = undefined;
    return this.dispatch(p, {});
  }
  private endLine(): StretchEvent[] {
    const l = this.line; this.line = '';
    if (l.startsWith(':')) return [];                                  // a comment says nothing about the event in progress
    let out: StretchEvent[] = [];
    if (!l.startsWith('data:')) out = this.flushPending();
    const rest = this.process(l);
    if (l.startsWith('event:')) this.pendingEvent = l.slice(6).trim();
    else if (l.startsWith('data:')) this.pendingEvent = undefined;
    return [...out, ...rest];
  }
  private static object(s: string): Record<string, unknown> | undefined {
    try { const v = JSON.parse(s); return typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : undefined; } catch { return undefined; }
  }
  private process(l: string): StretchEvent[] {
    if (l.trim() === '') { this.sseName = 'message'; return []; }
    if (l.startsWith('{')) {
      const o = StretchDecoder.object(l);
      if (!o) { this.ignoredCount++; return []; }
      return this.dispatch(typeof o.type === 'string' ? o.type : typeof o.event === 'string' ? o.event : 'segment', o);
    }
    if (l.startsWith('event:')) { this.sseName = l.slice(6).trim(); return []; }
    if (l.startsWith('data:')) {
      let d = l.slice(5); if (d.startsWith(' ')) d = d.slice(1);
      const name = this.sseName; this.sseName = 'message';
      const o = StretchDecoder.object(d);
      if (!o) { this.ignoredCount++; return []; }
      return this.dispatch(name === 'message' ? 'segment' : name, o);
    }
    if (l.startsWith('id:') || l.startsWith('retry:')) return [];
    this.ignoredCount++;                                               // not JSON, not SSE
    return [];
  }
  /** A JSON number that is a whole number: not a bool, not a fraction, not a string. */
  static wholeNumber(v: unknown): number | undefined {
    return typeof v === 'number' && Number.isFinite(v) && v === Math.round(v) && Math.abs(v) < 1e9 ? v : undefined;
  }
  private dispatch(name: string, o: Record<string, unknown>): StretchEvent[] {
    const str = (v: unknown): string | undefined => (typeof v === 'string' ? v : undefined);
    const nonEmpty = (v: unknown): string | undefined => { const s = str(v); return s ? s : undefined; };
    switch (name) {
      case 'segment': {
        const n = this.seen++;
        const kind = str(o.kind) ?? '', text = str(o.text) ?? '';
        if (!isKnownKind(kind) || text.trim() === '') { this.droppedCount++; return []; }
        let clip: ClipRef | undefined;
        const c = o.clip as Record<string, unknown> | undefined;
        if (c && typeof c === 'object' && nonEmpty(c.clip_id)) clip = { clip_id: c.clip_id as string, story_ref: str(c.story_ref) ?? '', seconds: typeof c.seconds === 'number' ? c.seconds : 0 };
        if (kind === 'podcast_clip' && !clip) { this.droppedCount++; return []; }
        // ASSUMED wire: `take` is the id, or an object with `take_id`; the forecast is `forecast{p_heard,caused_by,claim_id}` or the line's own flat fields.
        let takeID: string | undefined;
        if (typeof o.take === 'string') takeID = o.take; else if (o.take && typeof o.take === 'object') takeID = str((o.take as Record<string, unknown>).take_id);
        if (kind === 'take' && !(takeID !== undefined && validTakeID(takeID))) { this.droppedCount++; return []; }
        let forecast: Forecast | undefined;
        const fo = (o.forecast && typeof o.forecast === 'object' ? o.forecast : o) as Record<string, unknown>;
        const p = fo.p_heard;
        if (typeof p === 'number' && Number.isFinite(p) && p >= 0 && p <= 1) forecast = { p_heard: p, caused_by: str(fo.caused_by) ?? '', claim_id: str(fo.claim_id) ?? str(o.claim_id) ?? '' };
        // ASSUMED wire (learn-settle): the event carries `entry_id` and, for a content segment, `claim_id`, flat on the event.
        const entryID = nonEmpty(o.entry_id);
        const claimID = nonEmpty(o.claim_id) ?? nonEmpty((o.forecast as Record<string, unknown> | undefined)?.claim_id);
        if (kind === 'take' && !forecast) { this.droppedCount++; return []; }
        const st = nonEmpty(o.stretch_id) ?? this.stretchID;
        const id = nonEmpty(o.segment_id) ?? `${st}/${n}`;
        const cites: Cite[] = [];
        for (const ci of Array.isArray(o.cites) ? o.cites : []) {
          if (typeof ci === 'string' && ci !== '') cites.push({ ref: ci, role: '', label: '' });
          else if (ci && typeof ci === 'object' && nonEmpty((ci as Record<string, unknown>).ref)) {
            const d = ci as Record<string, unknown>;
            cites.push({ ref: d.ref as string, role: str(d.role) ?? '', label: str(d.label) ?? '' });
          }
        }
        let between: Incoming['between'];
        const b = o.between as Record<string, unknown> | undefined;
        if (b && typeof b === 'object') {
          const a = str(b.after_segment ?? b.after), z = str(b.before_segment ?? b.before);
          if (a && z) between = { after: a, before: z };
        }
        const seg: Incoming = { stretch_id: st, segment_id: id, kind, title: str(o.title) ?? '', text, cites };
        const bc = StretchDecoder.wholeNumber(o.block_count);
        if (bc !== undefined) seg.block_count = bc;
        if (kind === 'podcast_clip' && clip) seg.clip = clip;
        if (between) seg.between = between;
        if (kind === 'take' && takeID !== undefined) seg.take_id = takeID;
        if (forecast) seg.forecast = forecast;
        if (entryID) seg.entry_id = entryID;
        if (claimID) seg.claim_id = claimID;
        return [{ type: 'segment', segment: seg }];
      }
      case 'stretch': { const id = nonEmpty(o.stretch_id); if (id) this.stretchID = id; return []; }
      case 'done': return [{ type: 'done' }];
      case 'error': {
        const reason = str(o.reason) ?? '', spoken = str(o.spoken) ?? '';
        return [{ type: 'error', message: reason === '' ? (spoken === '' ? 'the stretch failed' : spoken) : (spoken === '' ? reason : `${reason}: ${spoken}`) }];
      }
      default: this.ignoredCount++; return [];                         // handoff (reserved) and anything unknown
    }
  }
}

// ---- a segment becomes an entry ----

/** What the phone holds about a podcast clip, by id (the clip store). */
export type ClipInfo = { clip_id: string; show: string; audio_url: string; start: number; end: number; seconds: number; route: string; version: number; story_title: string; episode_title: string; relocatable: boolean };
/** What the phone holds about a take, by id (the list it read for this request). */
export type TakeInfo = { take_id: string; publisher: string; duration: number; title: string; responds_to?: { route: string; title: string } };
export type Lookups = { clip(id: string): ClipInfo | undefined; take(id: string): TakeInfo | undefined };

/** `d-<segment id>`, kept to the characters an id may have and 64 bytes. */
export function entryID(segmentID: string): string {
  let out = 'd-';
  for (const ch of segmentID) {
    const u = ch.codePointAt(0) as number;
    const ok = (u >= 0x30 && u <= 0x39) || (u >= 0x41 && u <= 0x5a) || (u >= 0x61 && u <= 0x7a) || u === 0x5f || u === 0x2e || u === 0x2d;
    out += ok ? ch : '-';
    if (out.length >= 64) break;
  }
  return out;
}

/** The op that places a desk entry at the end. The op id follows the entry id: a segment seen twice is one op, and the reducer drops the second. */
export function placeOp(e: Entry, cause: string): Op {
  return { op_id: ('o' + e.id).slice(0, 64), by: 'desk', op: 'place', at: 'end', entry: e, reason: cause === 'open' ? 'written to open the listening' : 'written to extend the programme', base_version: 0 };
}

/** The entry a segment becomes; undefined when it cannot be one (a clip or take the phone does not have, or no words). Old kind names map as
 *  the interface says: `ident` is `opening`, `bridge` is `transition`; `story`, `update` and `headlines` are `story`; `podcast_clip` is `clip`. */
export function entryFrom(s: Incoming, look: Lookups): Entry | undefined {
  const text = cutBytes(s.text.trim(), Bounds.maxTextBytes);
  if (text === '') return undefined;
  let kind: Entry['kind'];
  switch (s.kind) {
    case 'opening': case 'ident': kind = 'opening'; break;
    case 'transition': case 'bridge': kind = 'transition'; break;
    case 'podcast_clip': kind = 'clip'; break;
    case 'take': kind = 'take'; break;
    case 'answer': kind = 'answer'; break;
    case 'result': kind = 'result'; break;
    default: kind = 'story';                                           // story, update, headlines, mail, podcast_note
  }
  const cites: NonNullable<Entry['cites']> = [];
  for (const c of s.cites) {
    if (c.ref === '' || cites.some((x) => x.ref === c.ref)) continue;
    cites.push({ ref: c.ref, role: c.role === 'subject' || c.role === 'quoted' || c.role === 'mentioned' ? c.role : 'mentioned', label: label(c.label) });
  }
  // The listener's words: an opening is "Opening"; a transition is the station speaking between two entries and has no title of its own.
  const t = kind === 'opening' ? 'Opening' : kind === 'transition' ? '' : title(s.title, s.kind);
  // The server's own id when it sent one (it places the same entry on its sheet under it); else the phone's grammar, which is the same string.
  const id = s.entry_id ? s.entry_id.slice(0, 64) : entryID(s.segment_id);
  const e: Entry = { id, kind, placed_by: 'desk', state: 'ahead', title: t, text, est_seconds: estimateSeconds(text), revision: 0 };
  if (cites.length) e.cites = cites;
  const p = cites.find((c) => c.ref.startsWith('pub:'));
  if (p) e.source = { ref: p.ref, label: p.label };
  if (s.between) e.between = { after: entryID(s.between.after), before: entryID(s.between.before) };
  if (kind === 'clip') {
    const c = s.clip ? look.clip(s.clip.clip_id) : undefined;
    if (!s.clip || !c || !c.relocatable) return undefined;              // a clip the phone does not have, or cannot find in a served file (no fingerprint)
    e.source = { ref: 'clip:' + c.clip_id, label: c.show, url: c.audio_url, start: c.start, end: c.end, clip_id: c.clip_id };
    e.est_seconds += c.seconds;
    // The story the clip was cut for is one of the stories this entry was built on (the page's "Built on").
    const stories: [string, string][] = [];
    if (c.route !== '') stories.push([`pub:${c.route}@v${c.version}`, c.story_title]);
    if (s.clip.story_ref !== '') stories.push([s.clip.story_ref, '']);
    for (const [r, l] of stories) {
      if (!(e.cites ?? []).some((x) => x.ref === r || x.ref.split('@')[0] === r.split('@')[0])) e.cites = [...(e.cites ?? []), { ref: r, role: 'subject', label: label(l) }];
    }
    if (e.title === '' || e.title === kindLabel(s.kind)) e.title = c.episode_title === '' ? c.show : c.episode_title;
  }
  if (kind === 'take') {
    // A take the phone does not hold (not in the list it read for this request) is not played.
    const tk = s.take_id ? look.take(s.take_id) : undefined;
    if (!tk) return undefined;
    const cs = [...(e.cites ?? [])];
    if (tk.responds_to && !cs.some((c) => c.ref.startsWith('pub:' + tk.responds_to!.route))) cs.push({ ref: 'pub:' + tk.responds_to.route, role: 'mentioned', label: label(tk.responds_to.title) });
    if (cs.length) e.cites = cs; else delete e.cites;
    e.source = { ref: 'take:' + tk.take_id, label: tk.publisher };
    e.est_seconds += tk.duration;
    e.title = tk.title;
  }
  if (s.forecast) e.forecast = s.forecast;
  else if (s.claim_id) e.forecast = { caused_by: '', claim_id: s.claim_id };   // the claim the server settles by; no number the phone does not hold
  return e;
}

// ---- the writer's rules: when to ask for more ----

/** The least spoken time kept written ahead of the listener. */
export const AHEAD_FLOOR_SECONDS = 180;
/** The write latency assumed until one has been measured. */
export const UNMEASURED_LATENCY_SECONDS = 30;

/** Writing ahead is the rule: ask when the spoken time ahead is under max(twice the measured write latency, the floor), or at once when fewer
 *  than two playable entries (transitions do not count) are ahead. */
export function shouldAskMore(ahead: Entry[], latencySeconds: number): boolean {
  const playable = ahead.filter((e) => e.kind !== 'transition').length;
  const seconds = ahead.reduce((a, e) => a + e.est_seconds, 0);
  const estimate = latencySeconds > 0 ? latencySeconds : UNMEASURED_LATENCY_SECONDS;
  return playable < 2 || seconds < Math.max(2 * estimate, AHEAD_FLOOR_SECONDS);
}

/** What an ask has to say to the listener, and what it changes, once it ended. `quietOn` is the entry on air when it added nothing (not asked
 *  again until another one is). */
export type AskEnd = { added: number; received: number; dropped: number; failed: boolean; cause: string; airId?: string };
export type AskOutcome = { dry: boolean; clearQuiet: boolean; quietOn?: string; notice: 'clear' | 'say' | 'keep' };
export function askOutcome(r: AskEnd): AskOutcome {
  const dry = r.added === 0 && !r.failed;
  if (r.added > 0) return { dry, clearQuiet: true, notice: 'clear' };
  // Only a dropped OPENING (an `open` ask) is not "the desk has nothing more"; a repeat dropped from an extend is exactly that.
  const quietOn = r.cause === 'open' && r.dropped > 0 ? undefined : r.airId;
  // The desk answered but everything it wrote is already on the sheet (a repeat): that is not a failure.
  if (r.received > 0 && !r.failed) return { dry, clearQuiet: false, quietOn, notice: 'clear' };
  if (r.failed || r.cause === 'open') return { dry, clearQuiet: false, quietOn, notice: 'say' };
  return { dry, clearQuiet: false, quietOn, notice: 'keep' };
}

/** Plain words for what went wrong, or undefined when there is nothing to say (the front page has no story; a cancelled ask). */
export function plainWords(err: { kind: 'cancelled' | 'empty' | 'signedOut' | 'transport' | 'other' } | undefined): string | undefined {
  if (!err) return "The desk isn't answering right now.";
  switch (err.kind) {
    case 'cancelled': case 'empty': return undefined;
    case 'signedOut': return 'Sign in to hear the desk.';
    case 'transport': return "The desk can't be reached right now.";
    default: return "The desk isn't answering right now.";
  }
}

/** The routes of the stories the sheet already holds, from its entries' sources and cites. */
export function toldRoutes(entries: Entry[]): Set<string> {
  const out = new Set<string>();
  for (const e of entries) for (const ref of [...(e.cites ?? []).map((c) => c.ref), e.source?.ref ?? '']) if (ref.startsWith('pub:')) out.add(ref.slice(4).split('@')[0]);
  out.delete('');
  return out;
}
