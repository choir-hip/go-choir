// What plays next and why: the radio's decisions over the run sheet store. Ported from RunSheetPlayer.swift (the decision half; the sound is the
// deck's). This class holds the rules; it plays nothing. When the entry the store says is on air changes, it says so through `onCommand`
// ("start this entry", "stop"); the deck driver turns a start into a DeckItem (`plan`) and reports back through `finished`, `documentEnded`,
// `failedToStart`. Nothing here reads a clock but the store's, and nothing here touches the network.
//
// Ways this can fail (each has a test in radio-director.test.ts; the numbers are RunSheetStoreTests.swift's):
//  1. Skip, next, a swipe on the card and the lock screen's next do different things: they all call `next()`, which is one `skip(on air)`.
//  2. The next after a story is the paper's neighbour: the director never reads the paper's list; only `ahead` decides.
//  3. An entry that cannot be loaded stops the radio: it ends as skipped (failed) and the next one plays. Two in a row that make no sound stop it.
//  4. A late end (a document that was replaced) closes the entry that replaced it: the end must name the document that is playing.
//  5. The listener spoke over the end of an entry: the next one waits for the turn to end (`turnEnded`).
//  7. A clip's skip leaves it playing: skipping stops the sound (a `stop` command), and the entry ends `skipped`.
//  8. Nothing ahead and something arrives later: the director starts it when the radio was playing and is not paused.
// 21. THE ONE OPENING RULE: a listener never hears two openings in a row.
// 22. A transition plays only when the entry before it was heard to its end and the entry after it is the one it was written for.
import { estimateSeconds, mintId } from './runsheet-entries.ts';
import { apply as reduce, type Entry, type Op } from './runsheet-model.ts';
import { NO_REPORT, type AirReport, type RunSheetStore } from './runsheet-store.ts';

/** "Previous" restarts the entry being played unless it has barely begun: the first block, and under this many seconds in. */
export const RESTART_AFTER_SECONDS = 3.0;
export const previousRestartsCurrent = (block: number, secondsIntoBlock: number): boolean => block > 0 || secondsIntoBlock >= RESTART_AFTER_SECONDS;
/** An opening may be placed only when none is on air and none was heard in the last half hour. */
export const OPENING_WINDOW_SECONDS = 30 * 60;
/** Seconds between tries when the session cannot be taken (the last repeats). */
export const SESSION_RETRY_DELAYS = [1, 2, 4, 8, 15] as const;
export const retryDelay = (attempt: number): number => SESSION_RETRY_DELAYS[Math.min(Math.max(attempt, 1) - 1, SESSION_RETRY_DELAYS.length - 1)];

/** How long a message that is not a failure to speak stays on the page (the bar holds a message for four). */
export const MESSAGE_SECONDS = 8;
export type Command = { kind: 'start'; entry: Entry; press: boolean } | { kind: 'stop'; gen: number };
export type Words = { title: string; paragraphs: string[] };

/** What the deck driver does to start an entry: the decision, as data. */
export type Plan =
  | { kind: 'speak'; document: string; paragraphs: string[] }
  | { kind: 'story'; route: string; title: string }                            // load a paper story in full, then speak it
  | { kind: 'webPage'; url: string; title: string }                            // fetch the page's text, then speak it
  | { kind: 'book'; id: string; at: number }                                   // read blocks of a book from `at`
  | { kind: 'take'; id: string; lead?: string }                                // fetch the take's audio (signed in); say the lead-in first when there is one
  | { kind: 'audio'; url: string; start?: number; end?: number; lead?: string; clipId?: string; title: string; show: string }
  | { kind: 'file'; address: string; title: string }                           // resolve a Files address to the phone's copy, then play it
  | { kind: 'cannot'; why: string };

/** Which way an entry is started. `provided` is what the page's own text said (held for the entry: the entry itself is a pointer). */
export function plan(e: Entry, provided?: Words, spec?: { url: string; start?: number; end?: number; show: string; title: string }): Plan {
  switch (e.kind) {
    case 'opening': case 'transition': case 'story': case 'answer': case 'result':
      return { kind: 'speak', document: 'rs:' + e.id, paragraphs: [e.text ?? ''] };
    case 'page': case 'mail': {
      if (provided) return { kind: 'speak', document: 'rs:' + e.id, paragraphs: provided.paragraphs };
      const ref = e.source?.ref ?? '';
      if (e.kind === 'page' && ref.startsWith('pub:')) return { kind: 'story', route: ref.slice(4).split('@')[0], title: e.title };
      if (e.kind === 'page' && ref.startsWith('url:')) return { kind: 'webPage', url: ref.slice(4), title: e.title };
      return { kind: 'cannot', why: 'it has no words to read' };
    }
    case 'document': {
      const ref = e.source?.ref ?? '';
      return ref.startsWith('doc:') ? { kind: 'book', id: ref.slice(4), at: Math.trunc(e.source?.start ?? 0) } : { kind: 'cannot', why: 'it names no book' };
    }
    case 'take': {
      const id = e.source?.ref.startsWith('take:') ? e.source.ref.slice(5) : undefined;
      if (!id) return { kind: 'cannot', why: 'the take has no id' };
      return { kind: 'take', id, ...(e.text ? { lead: e.text } : {}) };
    }
    case 'clip': case 'episode': {
      const address = e.source?.ref.startsWith('file:') ? e.source.ref : undefined;
      if (!spec) return address ? { kind: 'file', address, title: e.title } : { kind: 'cannot', why: 'it names no audio' };
      return {
        kind: 'audio', url: spec.url, title: spec.title, show: spec.show,
        ...(spec.start !== undefined ? { start: spec.start } : {}), ...(spec.end !== undefined ? { end: spec.end } : {}),
        ...(e.kind === 'clip' && e.text ? { lead: e.text } : {}), ...(e.kind === 'clip' && e.source?.clip_id ? { clipId: e.source.clip_id } : {}),
      };
    }
  }
}

export type Hooks = {
  /** A turn (the listener's words) is open: the end of an entry waits for it. */
  turnOpen?: () => boolean;
  /** The listener paused the spoken radio: nothing starts by itself. */
  spokenPaused?: () => boolean;
  /** What of the spoken entry is still to be said, from the start of the sentence it is in. */
  spokenRemainder?: () => string | undefined;
  /** The audio being played now (not spoken) and where it is, for keeping the place while an answer speaks. */
  audioPlace?: () => { url: string; title: string; show: string; start: number; end?: number; episode_id?: string; elapsed: number } | undefined;
  /** The writer wants to know the time ahead may be short. */
  aheadChanged?: () => void;
  onChange?: () => void;
  /** An entry ended (heard or skipped): the host writes ahead. */
  onEntryEnded?: (e: Entry, played: boolean) => void;
};

export class Director {
  readonly store: RunSheetStore;
  hooks: Hooks;
  onCommand: (c: Command) => void = () => undefined;
  /** The entry the director was told to play (the entry the store has on air). */
  playingId?: string;
  playingDocument?: string;
  /** The radio is on: an entry that ends is followed by the next one. Off at launch, after a pause that stopped it, and after sign-out. */
  wants = false;
  /** A plain line about the last thing that could not play ("" when nothing is wrong). */
  message = '';
  /** When a transient message was set (device ms; 0: not transient). A message that is not a failure to speak goes by itself after MESSAGE_SECONDS: `visibleMessage`. */
  messageAt = 0;
  visibleMessage(): string {
    if (this.message !== '' && this.messageAt > 0) { const t = this.store.now().getTime(); if (t > 0 && (t - this.messageAt) / 1000 > MESSAGE_SECONDS) { this.message = ''; this.messageAt = 0; } }
    return this.message;
  }
  /** Which start this is; the deck's ends belong to the start they report. */
  generation = 0;
  /** The phone's opening of this listening, and what became of desk entries the sheet did not take. */
  currentOpeningId?: string;
  droppedFromDesk: { id: string; reason: string }[] = [];
  /** The launch started with an entry that is not an opening: the desk's live opening, when it comes, is a transition placed next. */
  openingIsTransition = false;
  /** An answer went on air over this entry: where it was, as a continuation entry. Placed `next` when the answers are over. */
  remembered?: Entry;
  /** Words the phone holds for an entry whose own words are a pointer (a page's text, a held rest). */
  provided = new Map<string, Words>();
  /** The turn ids already applied (the turn route's replay rule). */
  turnLog: string[] = [];
  private silent = 0;
  private reconciling = false;
  private again = false;
  private pressPending = false;
  private pendingEnd?: { id: string; played: boolean };

  constructor(store: RunSheetStore, hooks: Hooks = {}) {
    this.store = store; this.hooks = hooks;
    store.onChange = () => this.storeChanged();
  }
  private get turnOpen(): boolean { return this.hooks.turnOpen?.() ?? false; }
  private get spokenPaused(): boolean { return this.hooks.spokenPaused?.() ?? false; }
  private changed(): void { this.hooks.onChange?.(); }
  get wantsToPlay(): boolean { return this.wants; }

  // ---- the opening rule ----

  /** An opening may be placed only when none is on air and none was heard (finished or part-heard) in the last half hour. A heard opening with
   *  no time on it counts as recent. Admission on the phone, before an op is made; the reducer and the server are not involved. */
  openingAllowed(): boolean {
    const es = this.store.sheet.entries;
    if (es.some((e) => e.kind === 'opening' && e.state === 'on_air')) return false;
    const now = this.store.now().getTime();
    for (const e of es) {
      if (e.kind !== 'opening' || (e.state !== 'heard' && e.state !== 'skipped') || e.ended?.outcome === 'failed') continue;
      const at = e.ended?.at, t = at ? Date.parse(at) : NaN;
      if (!at || !Number.isFinite(t)) return false;
      if ((now - t) / 1000 <= OPENING_WINDOW_SECONDS) return false;
    }
    return true;
  }

  /** The opening: placed by the phone, on air at once. The radio is wanted from now on. What the desk and the phone wrote for an earlier
   *  listening is not played now (it may be a day old): it is taken off. What the listener placed themselves stays. */
  open(entry: Entry, asOpening = true): void {
    this.openingIsTransition = !asOpening;
    this.wants = false;                         // nothing ahead is aired while the sheet is being cleaned: the opening goes first
    const allowed = entry.kind !== 'opening' || this.openingAllowed();
    // When the one-opening rule refuses the opening, the earlier listening was minutes ago: what it left ahead is still good, and it plays at once.
    if (allowed) for (const e of this.store.ahead()) if (e.placed_by !== 'listener') this.store.remove(e.id, 'phone', 'left from an earlier listening');
    if (allowed) { this.currentOpeningId = asOpening ? entry.id : undefined; this.store.place(entry, 'now', { by: 'phone' }); }
    else this.currentOpeningId = undefined;
    this.wants = true;
    if (!allowed) this.advanceIfIdle();         // nothing changed on the sheet, so nothing would start by itself
  }

  // ---- the listener's verbs (each is ops on the store) ----

  /** Play this now ("Play", Listen, Read it to me, Read aloud). What was on air is cut; what was ahead stays ahead. `words` are what a page's
   *  own text said, held for this entry. `report` is where the entry on air was when it was cut. */
  playNow(e: Entry, words?: Words, report?: AirReport): void {
    this.dropRemembered();
    this.wants = true;
    this.pressPending = true;                   // an explicit placement is a press: it may take the session back
    try {
      if (words) this.provided.set(e.id, words);
      const cut = this.store.onAir() ? (report ?? NO_REPORT) : undefined;
      this.placed(this.store.place(e, 'now', { cutting: cut }));
    } finally { this.pressPending = false; }
  }
  /** After the entry on air, before everything else ahead. */
  playNext(e: Entry, words?: Words): void { if (words) this.provided.set(e.id, words); this.placed(this.store.place(e, 'next')); }
  /** "Queue this": after what the listener placed before it. */
  queue(e: Entry, words?: Words): void { if (words) this.provided.set(e.id, words); this.placed(this.store.place(e, 'queued')); }
  /** An address too long for the server is not placed, and the listener is told so. */
  private placed(r: { dropped?: string }): void { if (r.dropped === 'ref_too_long') { this.message = 'That address is too long to add.'; this.messageAt = this.store.now().getTime(); this.changed(); } }
  remove(index: number): void { const a = this.store.ahead(index + 1); if (index >= 0 && index < a.length) this.store.remove(a[index].id); }
  /** Tap an item in Up next: play it now. The entries before it stay ahead. */
  playUpcoming(index: number): void {
    const a = this.store.ahead(index + 1);
    if (index < 0 || index >= a.length) return;
    this.dropRemembered();
    this.wants = true;
    this.pressPending = true;
    try { this.store.move(a[index].id, 'now'); } finally { this.pressPending = false; }
  }
  clearUpcoming(): void { this.dropRemembered(); this.store.clearAhead(); }

  /** Skip, the next button, a swipe on the now-playing card, the lock screen's next, and "next" by voice: one op, `skip(on air)`. With nothing
   *  on air it starts what is next. */
  next(report: AirReport = NO_REPORT): void {
    this.dropRemembered();
    this.wants = true;
    if (this.store.onAir()) this.store.skipOnAir(report); else this.advanceIfIdle();
  }
  /** Back: the start of this entry when it has gone on a little (or it is audio), else the entry before it, aired again. Returns what the deck
   *  should do when the answer is not an op: restart the entry, or seek the audio to its start. */
  previous(pos: { block: number; seconds: number; isRemoteAudio: boolean }): 'restart' | 'seekStart' | 'previous' | 'none' {
    this.dropRemembered();
    this.wants = true;
    if (pos.isRemoteAudio && this.store.onAir()) return 'seekStart';
    if (this.store.onAir() && previousRestartsCurrent(pos.block, pos.seconds)) return 'restart';
    if (!this.store.previous().applied && this.store.onAir()) return 'restart';       // nothing before it: this one again
    if (!this.store.onAir()) { this.advanceIfIdle(); return 'none'; }
    return 'previous';
  }

  /** Sign-out: what was on air stops and nothing carries over to the next account. Sign-out is not a judgement of the entry: it leaves the air as
   *  `left`, by the phone, where it was. An entry that never sounded was not heard: `skipped` with `failed`. */
  stopAll(report: AirReport = NO_REPORT, neverSounded = false): void {
    this.dropRemembered();
    this.wants = false;
    const e = this.store.onAir();
    if (e) {
      if (neverSounded) this.store.airedEnded(e.id, 'skipped', 'failed', report, 'signed out before it made a sound');
      else this.store.airedEnded(e.id, 'heard', 'left', report);
    }
    this.store.clearAhead('phone', 'signed out');
    this.playingId = undefined; this.playingDocument = undefined;
    this.onCommand({ kind: 'stop', gen: this.generation });
  }

  /** The radio is on and nothing is on air: start what is next. */
  advanceIfIdle(): void {
    if (this.store.onAir() || this.playingId !== undefined) return;
    this.wants = true;                           // pressing Play (or the station) is the want: what ends is followed by the next, with no press
    this.advance();
  }
  private advance(): void {
    const first = this.nextToAir();
    if (!first) { this.hooks.aheadChanged?.(); return; }
    this.store.airedOnAir(first.id);             // the store's change starts it
  }

  /** The first entry ahead that should be aired. A transition is written for the two entries it sits between: it plays only when the entry just
   *  before it was heard to its end (not skipped past) and the entry after it is the one it was written for; otherwise it is dropped. */
  nextToAir(): Entry | undefined {
    for (;;) {
      const first = this.store.ahead(1)[0];
      if (!first) return undefined;
      if (first.kind !== 'transition') return first;
      const ahead = this.store.ahead(2);
      const prev = this.store.heard(1)[0];
      const after = ahead.length > 1 ? ahead[1] : undefined;
      const ok = prev?.state === 'heard' && (first.between ? prev.id === first.between.after && after?.id === first.between.before : true) && after !== undefined;
      if (ok) return first;
      this.droppedFromDesk.push({ id: first.id, reason: 'transition_orphaned' });
      this.store.remove(first.id, 'phone', 'transition_orphaned');
    }
  }

  /** A desk op about to be applied. Where the phone's opening is the same job as the desk's: before it began, the desk's replaces it; while it is
   *  on air it finishes the block it is on and the desk's follows it; once it was heard (or skipped) the desk's is not a second greeting and is
   *  dropped. Only for the opening write (`cause: open`). Returns the op to apply, or undefined when it is dropped. */
  admitDesk(op: Op, cause: string): Op | undefined {
    // A desk that tells a story already on the sheet (heard, on air or ahead) is not heard twice: dropped, and the reason kept.
    if (op.op === 'place' && op.entry && op.entry.kind === 'story') {
      const e = op.entry;
      if (e.title !== '' && this.store.sheet.entries.some((x) => x.kind === 'story' && x.id !== e.id && x.title.toLowerCase() === e.title.toLowerCase())) {
        this.droppedFromDesk.push({ id: e.id, reason: 'title_already_on_sheet' });
        return undefined;
      }
    }
    if (this.openingIsTransition && cause === 'open' && op.op === 'place' && op.entry?.kind === 'opening') {
      this.openingIsTransition = false;
      return { ...op, at: 'next', entry: { ...op.entry, kind: 'transition', title: '' , between: undefined } as Entry };
    }
    if (cause !== 'open' || op.op !== 'place' || !op.entry || op.entry.kind !== 'opening' || op.entry.placed_by !== 'desk') return op;
    const e = { ...op.entry };
    // A phone opening ahead and not yet on air is replaced by the desk's.
    const phone = this.currentOpeningId ? this.store.entry(this.currentOpeningId) : undefined;
    if (phone && phone.state === 'ahead') {
      this.store.remove(phone.id, 'phone', 'opening_replaced_by_desk');
      e.title = 'Opening';
      return { ...op, at: 'next', entry: e };
    }
    // Otherwise one opening per start: none on air, none heard lately. The line is dropped; what follows it is admitted as usual.
    if (!this.openingAllowed()) { this.droppedFromDesk.push({ id: e.id, reason: 'opening_one_per_start' }); return undefined; }
    e.title = 'Opening';
    return { ...op, entry: e };
  }
  /** Admit and apply a desk op in one step, as the writer does. */
  applyFromDesk(op: Op, cause: string): ReturnType<RunSheetStore['applyFromDesk']> | undefined {
    const admitted = this.admitDesk(op, cause);
    return admitted ? this.store.applyFromDesk(admitted) : undefined;
  }

  // ---- reacting to the store ----

  private storeChanged(): void {
    if (this.reconciling) { this.again = true; return; }
    this.reconciling = true;
    try { do { this.again = false; this.reconcile(); } while (this.again); } finally { this.reconciling = false; }
    this.changed();
    this.hooks.aheadChanged?.();
  }

  /** An answer is about to take the air from an entry that was cut: keep that entry's place. */
  private rememberPlace(oldId: string | undefined, nextKind: Entry['kind'] | undefined): void {
    if (nextKind !== 'answer' || this.remembered || !oldId) return;
    const old = this.store.entry(oldId);
    if (!old || old.kind === 'answer' || old.kind === 'transition' || old.ended?.outcome !== 'cut') return;
    const place = this.hooks.audioPlace?.();
    const rest = this.hooks.spokenRemainder?.();
    const fromPhone = old.kind === 'mail' || old.kind === 'document' || !!old.source && /^(mail|doc|file):/.test(old.source.ref);
    if (place && this.playingDocument === undefined) {
      const url = place.url;
      this.remembered = { id: mintId('p-'), kind: old.kind === 'clip' ? 'clip' : 'episode', placed_by: 'phone', state: 'ahead', title: old.title, est_seconds: 0, revision: 0,
        source: { ref: old.source?.ref ?? 'episode:x', label: place.show, url, start: Math.max(place.start, place.elapsed) } };
    } else if (fromPhone && rest) {
      // A message, book or file: the rest of the passage is held here. The continuation names the same address with no words.
      const id = 'c-' + mintId('').slice(0, 24);
      this.remembered = { id, kind: old.kind === 'mail' ? 'mail' : 'page', placed_by: 'phone', state: 'ahead', title: old.title, est_seconds: estimateSeconds(rest), revision: 0, source: old.source };
      this.provided.set(id, { title: old.title, paragraphs: [rest] });
    } else if (rest) {
      this.remembered = { id: 'c-' + mintId('').slice(0, 24), kind: 'story', placed_by: 'phone', state: 'ahead', title: old.title, text: rest, est_seconds: estimateSeconds(rest), revision: 0, source: old.source, cites: old.cites };
    }
  }
  /** The listener asked for something else: what was kept is not wanted back. */
  dropRemembered(): void { this.remembered = undefined; }

  private reconcile(): void {
    const air = this.store.onAir();
    if (air) {
      if (air.id === this.playingId) return;
      this.rememberPlace(this.playingId, air.kind);
      this.playingId = air.id; this.playingDocument = undefined; this.pendingEnd = undefined;
      const press = this.pressPending;
      this.generation += 1;
      this.onCommand({ kind: 'start', entry: air, press });
      return;
    }
    if (this.playingId !== undefined) {          // what was on air was skipped, cut or taken off: its sound stops
      this.rememberPlace(this.playingId, this.nextToAir()?.kind);
      this.playingId = undefined; this.playingDocument = undefined; this.generation += 1; this.pendingEnd = undefined;
      this.onCommand({ kind: 'stop', gen: this.generation });
    }
    // The answers are over (nothing on air, the next is not an answer): what they cut goes back on air, from the start of its sentence.
    if (this.remembered && this.playingId === undefined && this.nextToAir()?.kind !== 'answer') {
      const r = this.remembered;
      this.remembered = undefined;
      this.store.place(r, 'next', { by: 'phone', reason: 'back to where it was before the answer' });
      this.again = true;
      return;
    }
    if (this.wants && !this.spokenPaused && !this.turnOpen) {
      const first = this.nextToAir();
      if (first) { this.store.airedOnAir(first.id); this.again = true; }
    }
  }

  // ---- reports from the deck driver ----

  /** The start of the playing entry made its document (the deck's id for what it is speaking). */
  started(entryId: string, document?: string): void { if (entryId === this.playingId) { this.playingDocument = document; this.silent = 0; this.message = ''; this.messageAt = 0; this.changed(); } }

  /** What cannot play is skipped, once, and the radio goes on. Two in a row that made no sound stop it. */
  failedToStart(id: string, why: string): void {
    this.silent += 1;
    if (this.silent >= 2 && this.wants) { this.wants = false; if (this.message === '') { this.message = 'The radio could not play the next entries. Tap play to try again.'; this.messageAt = 0; } }
    this.finish(id, false, undefined, 'failed to start: ' + (why || this.message));
  }

  /** The entry ended (or never made a sound): the phone reports it, and the store's change starts the next one. */
  finish(id: string, played: boolean, report: AirReport = NO_REPORT, _why = ''): void {
    if (this.playingId !== id) return;
    this.playingId = undefined; this.playingDocument = undefined; this.pendingEnd = undefined;
    // A natural end carries the true final block and position; a failure is `failed` by the phone, never a skip.
    if (played) this.store.airedEnded(id, 'heard', 'finished', report);
    else this.store.airedEnded(id, 'skipped', 'failed');
    this.provided.delete(id);
    const e = this.store.entry(id);
    if (e) this.hooks.onEntryEnded?.(e, played);
  }

  /** The spoken document that was playing ran out of blocks. `anyPlayed`: a block made sound. A late end of a replaced document closes nothing. */
  documentEnded(document: string, anyPlayed: boolean, report: AirReport = NO_REPORT, failure = ''): void {
    const eid = this.playingId;
    if (eid === undefined || document !== this.playingDocument) return;
    if (this.turnOpen) { this.pendingEnd = { id: document, played: anyPlayed }; return; }
    if (anyPlayed) this.silent = 0;
    else {
      this.silent += 1;
      this.message = `Can’t speak: ${failure || 'the voice made no sound'}`; this.messageAt = 0;
      if (this.silent >= 2) this.wants = false;
    }
    this.finish(eid, anyPlayed, report);
  }

  /** A turn ended: an entry that ended over the listener's words is closed now, and the next one starts. */
  turnEnded(report: AirReport = NO_REPORT): void {
    if (this.pendingEnd) { const p = this.pendingEnd; this.pendingEnd = undefined; this.documentEnded(p.id, p.played, report); }
    else if (this.wants && !this.store.onAir()) this.advanceIfIdle();
  }

  /** Remote audio reached its end, or the clip's end. `generation` is the start it belongs to. */
  remoteEnded(generation: number, report: AirReport = NO_REPORT): void {
    if (generation !== this.generation || this.playingId === undefined) return;
    this.finish(this.playingId, true, report);
  }
}

// ---- pure helpers the pages and the writer read ----

/** Applies ops to a copy of the sheet without touching the store: what the page asks "would this be accepted". */
export const wouldApply = (store: RunSheetStore, op: Op): boolean => reduce(store.sheet, op).result.applied;
