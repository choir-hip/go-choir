// The radio runtime: the store, the director and the deck driver, in shared code. It is what RunSheetPlayer.swift was: nothing else decides what is heard. The sound is the deck
// (Deck.swift on iPhone, Kotlin on Android: docs/reference/exact/deck.md); this module hands it items and reads its journal.
//
// The loop. Shared code may be frozen while audio plays on a locked iPhone, so the deck HOLDS the next entries (every ready entry, up to the sheet bound of 100, only those whose sound needs no fetch) and plays on by
// itself. Whenever shared code runs it `pump`s: it reads the deck's state, reads the journal lines it has not read (numbered, never reused), makes the director's calls for each in
// order (an end is the entry's end; a `started` for an entry the deck played alone airs that entry on the sheet), starts what the director commands (resolving the words first: a
// story to fetch, a page, a book, a take, a file), acknowledges the lines it read, and keeps the deck's held list equal to the sheet's next entries.
//
// What stays native, and why: the sound and the facts only the audio code knows (`NativeFacts`: a turn is open, what is left of the spoken entry, the audio being played and where).
// Nothing here is stored but the sheet's own file (the store's `Storage`), which is the store's, under app:/data.
import { Director, previousRestartsCurrent, plan as planEntry, type Command, type Words } from './radio-director.ts';
import { RunSheetStore, NO_REPORT, type AirReport, type Poster, type Storage } from './runsheet-store.ts';
import { audioSpec, displayTitle, kindWord } from './runsheet-entries.ts';
import { clockLabel } from './radio-model.ts';
import { clock } from './clock.ts';
import { item as deckItem, DECK_HELD_MAX, type DeckEvent, type DeckItem, type DeckPort, type DeckState } from './deck.ts';
import { syncHeld } from './deck-ops.ts';
import { parseEpisodeKey } from './podcast-model.ts';
import { refusedHere } from './refused.ts';
import type { Entry } from './runsheet-model.ts';

/** How many next entries the deck is kept holding: a dozen is minutes of speech, and building (and handing over) a hundred in one turn went over Exact's 100 ms of CPU (`radioHold` took 145-169 ms in the proof). */
const HOLD_AHEAD = Math.min(12, DECK_HELD_MAX);

export type NativeFacts = {
  /** The listener's turn is open: the end of an entry waits for it. */
  turnOpen: boolean;
  /** What of the spoken entry is still to be said, from the start of its sentence; "" when unknown. */
  remainder: string;
  /** The audio being played now (not spoken) and where it is, for keeping the place while an answer speaks. */
  audio?: { url: string; title: string; show: string; start: number; end?: number; episode_id?: string; elapsed: number };
  position: { block: number; seconds: number };
  remote?: { elapsed: number; duration: number };
  wants: boolean;
  /** The app is in front (not backgrounded, the phone not locked): native's own fact, the one the voice already uses to give the microphone up. A host that does not say is in front (the page's visibility still gates). */
  foreground: boolean;
};
export const NO_FACTS: NativeFacts = { turnOpen: false, remainder: '', position: { block: 0, seconds: 0 }, wants: false, foreground: true };

/** What the phone fetches before the deck is told: the sound of these is not in the entry. Each answers undefined when it cannot be had. */
export type Resolvers = {
  story(route: string): Promise<Words | undefined>;
  webPage(url: string, title: string): Promise<Words | undefined>;
  book(id: string, at: number): Promise<Words | undefined>;
  /** The phone's copy of a take's audio (an address the deck can play). */
  take(id: string): Promise<{ url: string } | undefined>;
  /** The phone's copy of an audio file in Files. */
  file(address: string): Promise<{ url: string } | undefined>;
};

export type DeskState = { notice: string; writing: boolean; dry: boolean };

/** What the screens read of the queue (queue.contract's `QueueInfo`), and what the bar's overlay and the Autoradio page read besides. */
export type QueueView = {
  onAir: boolean; current: string; currentKind: string; audio: boolean; playing: boolean; paused: boolean; waiting: boolean; clip: boolean;
  keepTitle: string; canOpenEpisode: boolean; elapsed: number; spokenElapsed: number; duration: number; elapsedLabel: string; remainingLabel: string; fraction: number;
  message: string; notice: string; writing: boolean; dry: boolean;
};

export type RuntimeDeps = {
  deck: DeckPort;
  storage?: Storage; poster?: Poster; now?: () => Date; generation?: () => number;
  resolvers?: Partial<Resolvers>;
  native?: { facts(): Promise<NativeFacts> };
  /** Told after anything the screens read changed. */
  onChange?: () => void;
  /** What plays when nothing is ahead (the director's `fill`): the paper's front stories the phone holds and the sheet does not. */
  fill?: (removed: ReadonlySet<string>) => Entry[];
  /** Whether a story's words are already held (the writer's prefetch), and the fetch that holds them.
   *  A pump must not fetch: an answer that waits on the network is let go, and its deck call with it. */
  hasWords?: (route: string) => boolean;
  prefetchWords?: (route: string) => Promise<void>;
};

const SPEAKS = new Set<Entry['kind']>(['opening', 'transition', 'story', 'answer', 'result']);
/** What the voice names a spoken entry (its segment): the entry's kind for the ones it distinguishes, a story for the rest. */
const segmentOf = (k: Entry['kind']): string => (k === 'opening' || k === 'transition' || k === 'answer' || k === 'result' || k === 'mail' ? k : 'story');

export class RadioRuntime {
  readonly store: RunSheetStore;
  readonly director: Director;
  private deck: DeckPort;
  private resolvers: Partial<Resolvers>;
  private native?: { facts(): Promise<NativeFacts> };
  private onChange?: () => void;
  private hasWords: (route: string) => boolean = () => true;                 // a host with no prefetcher (tests, the web) starts from its resolver
  private prefetchWords: (route: string) => Promise<void> = async () => undefined;
  private facts: NativeFacts = NO_FACTS;
  private deckState?: DeckState;
  /** The time the voice has run on the spoken entry on air (milliseconds): counted from the device's clock while the deck says it plays, held when it does not. A spoken entry may carry no estimate (one played from a paper page
   *  before its words are loaded), and the page still shows the time heard; `last` is the clock at the last read while playing, -1 when it was not. */
  private spoken = { id: '', ms: 0, last: -1 };
  private desk: DeskState = { notice: '', writing: false, dry: false };
  /** The items handed to the deck (played or held), by entry id: what a journal line is read against. */
  private known = new Map<string, DeckItem>();
  private lastSeq = 0;
  /** Entries the director commanded to start, not yet handed to the deck (the pump decides after reading every line whether the deck already has them). */
  private pendingStarts: Entry[] = [];
  private chain: Promise<unknown> = Promise.resolve();
  /** A sign-out moves it: work that began before it is dropped. */
  private epoch = 0;

  constructor(d: RuntimeDeps) {
    this.deck = d.deck; this.resolvers = d.resolvers ?? {}; this.native = d.native; this.onChange = d.onChange;
    this.hasWords = d.hasWords ?? this.hasWords; this.prefetchWords = d.prefetchWords ?? this.prefetchWords;
    this.store = new RunSheetStore({ storage: d.storage, poster: d.poster, now: d.now, generation: d.generation });
    this.director = new Director(this.store, {
      turnOpen: () => this.facts.turnOpen,
      spokenPaused: () => this.deckState?.why === 'paused_listener',
      spokenRemainder: () => (this.facts.remainder === '' ? undefined : this.facts.remainder),
      audioPlace: () => this.facts.audio,
      aheadChanged: () => { this.chain = this.chain.then(() => this.keepHeld()).then(() => undefined, () => undefined); },       // in the pump's chain: the answer that pumps waits for it (a native call started outside an answer is never run)
      onChange: () => this.onChange?.(),
      fill: d.fill,
    });
    this.director.onCommand = (c) => this.command(c);
  }

  // ---- the listener's verbs (the queue) ----

  /** Play this now. What was on air is cut at where it was; what was ahead stays ahead. */
  play(e: Entry, words?: Words): void { this.director.playNow(e, words, this.report()); }
  playNext(e: Entry, words?: Words): void { this.director.playNext(e, words); }
  queue(e: Entry, words?: Words): void { this.director.queue(e, words); }
  remove(index: number): void { this.director.remove(index); }
  playUpcoming(index: number): void { this.director.playUpcoming(index); }
  clear(): void { this.director.clearUpcoming(); }
  /** Skip, next, a swipe on the card, the lock screen's next: one op. */
  skip(): void { this.director.next(this.report()); }
  /** Play or pause what is on air; with nothing on air, start what is next (or say the host should open the radio). */
  async toggle(): Promise<'paused' | 'resumed' | 'started' | 'nothing'> {
    const st = await this.deck.state(); this.deckState = st;
    if (this.store.onAir()) {
      if (st.why === 'playing') { this.pauseEpoch += 1; this.listenerPaused = true; await this.deck.act('pause'); return 'paused'; }
      this.pauseEpoch += 1; this.listenerPaused = false;
      await this.deck.act('resume'); return 'resumed';
    }
    this.director.advanceIfIdle();
    return this.store.onAir() ? 'started' : 'nothing';
  }
  /** The listener's pause or the voice's yield: the deck's transport. */
  async act(op: 'pause' | 'resume' | 'stop' | 'clearEnd'): Promise<void> { await this.deck.act(op); }
  /** The deck's state as it is now (what the page models read: a podcast's place). */
  deckNow(): Promise<DeckState> { return this.deck.state(); }
  /** Back: the start of this entry when it has gone on a little (or it is audio), else the entry before it. */
  async previous(): Promise<void> {
    const st = await this.deck.state(); this.deckState = st;
    const air = this.store.onAir();
    const how = this.director.previous({ block: st.block, seconds: st.seconds, isRemoteAudio: air !== undefined && audioSpec(air) !== undefined });
    if (how === 'seekStart') await this.deck.act('seekTo', 0);
    else if (how === 'restart' && air) { const it = this.known.get(air.id); if (it) await this.deck.play(it); }
  }
  /** "Keep listening": the clip stops being a clip; the episode goes on from where it is, past the clip's end. */
  async keepListening(): Promise<void> {
    const air = this.store.onAir(), spec = air ? audioSpec(air) : undefined;
    if (!air || !spec || !spec.is_clip || this.kept.has(air.id)) return;
    const st = await this.deck.state(); this.deckState = st;
    if (st.item !== air.id) return;
    const it = deckItem({ id: air.id, kind: 'audio', title: spec.title, show: spec.show, url: spec.url, start: 0, end: 0, seconds: st.seconds });
    this.kept.add(air.id); this.known.set(air.id, it);
    await this.deck.play(it);
    this.onChange?.();
  }
  /** "Keep listening" with no restart: the deck stops ending the clip (`clearEnd`), the entry is recorded as kept (the run sheet's view and the episode page read it) and the screens are told. Nothing when no clip is on air. */
  async keepEnd(episodeId = ''): Promise<void> {
    const air = this.store.onAir(), spec = air ? audioSpec(air) : undefined;
    if (!air || !spec || !spec.is_clip || this.kept.has(air.id)) return;
    if (episodeId !== '' && spec.episode_id !== episodeId) return;                          // the page asked about another episode than the one on air
    const st = await this.deck.state(); this.deckState = st;                               // the deck's own word first: the clip may have ended and the deck gone on to the next (its ended line not pumped yet)
    if (st.item !== air.id || st.why === 'stopped' || st.why === 'empty' || this.kept.has(air.id)) return;
    const after = await this.deck.act('clearEnd');
    if (!after || after.why === 'empty') return;                              // the deck did not take it: the clip would still end, so it is not recorded as kept
    this.kept.add(air.id);
    this.onChange?.();
  }
  isKept(id: string): boolean { return this.kept.has(id); }
  /** What the deck is speaking now: the programme's headline and, for a paper story, its route (what the Autoradio page names an untitled entry by). */
  programme(): { title: string; route: string } {
    const air = this.store.onAir(), it = air ? this.known.get(air.id) : undefined;
    return { title: it?.title ?? '', route: it?.document.startsWith('pub:') ? it.document.slice(4) : '' };
  }
  async seekBy(seconds: number): Promise<void> { await this.deck.act('seekBy', seconds); }
  async seekTo(seconds: number): Promise<void> { await this.deck.act('seekTo', seconds); }
  setDesk(d: DeskState): void { this.desk = d; this.onChange?.(); }
  /** A turn ended: an entry that ended over the listener's words is closed now. */
  async turnEnded(): Promise<void> { await this.refreshFacts(); this.director.turnEnded(this.report()); await this.pump(); }

  // ---- the screens' view ----

  view(): QueueView {
    const air = this.store.onAir(), st = this.deckState;
    const spec = air ? audioSpec(air) : undefined;
    const why = st?.why ?? 'empty';
    const duration = st?.duration ?? 0, raw = air && spec ? (st?.seconds ?? 0) : 0;
    const elapsed = duration > 0 ? Math.min(raw, duration) : raw;       // the audio's place can run a moment past its length before the deck ends it: never shown past the end
    return {
      onAir: air !== undefined, current: air ? displayTitle(air) : '', currentKind: air ? kindWord(air) : '', audio: spec !== undefined,
      playing: air !== undefined && why === 'playing', paused: air !== undefined && (why === 'paused_listener' || why === 'paused_system'), waiting: why === 'held_session',
      clip: (spec?.is_clip ?? false) && !(air && this.kept.has(air.id)), keepTitle: air && spec ? (spec.show === '' ? spec.title : `${spec.title} (${spec.show})`) : '', canOpenEpisode: spec?.episode_id !== undefined && parseEpisodeKey(spec.episode_id) !== null,
      elapsed, spokenElapsed: air && !spec && this.spoken.id === air.id ? Math.floor(this.spoken.ms / 1000) : 0, duration, elapsedLabel: air && spec ? clockLabel(elapsed) : '',
      remainingLabel: air && spec && duration > 0 ? '-' + clockLabel(Math.max(0, duration - elapsed)) : '', fraction: spec && duration > 0 ? Math.min(1, Math.max(0, elapsed / duration)) : 0,
      message: this.director.visibleMessage(), notice: this.desk.notice, writing: this.desk.writing, dry: this.desk.dry && !this.desk.writing,
    };
  }

  // ---- the loop ----

  /** Read the deck and act on what it says. One at a time; safe to call as often as shared code runs. */
  pump(): Promise<void> {
    const run = this.chain.then(async () => { await this.pumpOnce(); if (this.pendingStarts.length > 0 && !this.inPump) await this.pumpOnce(); });
    this.chain = run.catch(() => undefined);
    // what was appended to the chain meanwhile (a stop, the held list) is waited for too
    return run.then(async () => { let c: Promise<unknown>; do { c = this.chain; await c; } while (c !== this.chain); });
  }

  private async refreshFacts(): Promise<void> { if (this.native) { try { this.facts = await this.native.facts(); } catch { /* keep the last */ } } }

  private report(): AirReport {
    const st = this.deckState;
    return st && st.item !== '' ? { block: st.block, seconds: st.seconds } : NO_REPORT;
  }

  private async pumpOnce(): Promise<void> {
    this.pumpCount += 1;
    const epoch = this.epoch;
    await RadioRuntime.turnEnd();
    await this.refreshFacts();
    let st: DeckState;
    try { st = await this.deck.state(); } catch { return; }              // the deck could not be read: it is not an empty deck. Nothing is decided from nothing; the next pump reads again
    if (epoch !== this.epoch) return;
    this.deckState = st;
    this.inPump = true;
    const lines = st.journal.filter((l) => l.seq > this.lastSeq).sort((a, b) => a.seq - b.seq);
    // the last `started` line of each item: a line is "followed by a start of X" when that is later than it (one pass, no slicing per line)
    const lastStart = new Map<string, number>();
    lines.forEach((l, i) => { if (l.type === 'started') lastStart.set(l.item, i); });
    for (let i = 0; i < lines.length; i++) this.read(lines[i], (id) => (lastStart.get(id) ?? -1) > i);
    if (lines.length > 0) {
      this.lastSeq = lines[lines.length - 1].seq;
      await this.deck.ack(this.lastSeq);
    }
    if (epoch !== this.epoch) return;
    await RadioRuntime.turnEnd();
    const starts = this.pendingStarts; this.pendingStarts = [];
    this.inPump = false;
    if (starts.length > 0) console.log(`radio: pump starts ${starts.map((e) => e.id + (this.director.playingId === e.id ? '' : ' (not playing id ' + (this.director.playingId ?? 'none') + ')')).join(', ')}; deck item ${st.item || 'none'}`);
    for (const e of starts) if (this.director.playingId === e.id) await this.startEntry(e, st);
    if (epoch !== this.epoch) return;
    await RadioRuntime.turnEnd();
    await this.keepHeld();
    this.onChange?.();
  }

  /** One journal line, in order. `startsLater` says whether a later line of the same read started an item: an entry the director is about to start that the deck already started is not started twice. */
  private read(l: DeckEvent, startsLater: (id: string) => boolean): void {
    const known = this.known.get(l.item);
    const air = this.store.onAir();
    switch (l.type) {
      case 'started': {
        // The deck played this entry by itself (it held it): the sheet airs it now, in order.
        if (known && !air && this.director.playingId === undefined && this.store.ahead(1)[0]?.id === l.item) this.store.airedOnAir(l.item);
        break;
      }
      case 'sounding': if (known && this.director.playingId === l.item) this.director.started(l.item, known.document || undefined); break;
      case 'ended': {
        if (!known || this.director.playingId !== l.item) break;
        const report: AirReport = { block: l.block, seconds: l.seconds, at: l.at };      // the deck's own time: after a freeze the end is not posted as heard at the pump's hour
        if (l.end === 'finished' && l.rendered) {
          if (this.director.playingDocument === undefined) this.director.started(l.item, known.document || undefined);
          if (known.kind === 'audio') this.director.remoteEnded(this.director.generation, report); else this.director.documentEnded(known.document, true, report);
        } else if (l.end === 'failed' || (l.end === 'finished' && !l.rendered)) this.director.failedToStart(l.item, l.reason);
        // skipped, replaced, stopped: the director caused them; nothing to report
        break;
      }
      case 'remote': this.remote(l); break;
      default: break;
    }
    // an entry the director just commanded to start that the deck has already started (a later line says so) is not started again
    this.pendingStarts = this.pendingStarts.filter((e) => !startsLater(e.id));
  }

  /** A lock-screen or headset command. The deck acted on it at once (Deck.swift `remoteNext`, `remotePrevious`): the line says where the item was when it came, and this makes the sheet agree. */
  private remote(l: DeckEvent): void {
    const where: AirReport = { block: l.block, seconds: l.seconds, at: l.at };
    if (l.command === 'next') this.director.next(where);                      // one skip, cut where it was; the next entry the deck played on to is found already playing
    else if (l.command === 'previous') {
      const air = this.store.onAir();
      if (!air) { this.director.previous({ block: 0, seconds: 0, isRemoteAudio: false }); return; }
      const remoteAudio = audioSpec(air) !== undefined;
      // Audio and a spoken entry that has gone on were started again by the deck itself; only "the entry before this one" is left to do.
      if (remoteAudio || previousRestartsCurrent(l.block, l.seconds)) { this.director.previous({ block: l.block, seconds: l.seconds, isRemoteAudio: remoteAudio }); return; }
      const how = this.director.previous({ block: l.block, seconds: l.seconds, isRemoteAudio: false });
      if (how === 'restart') { const it = this.known.get(air.id); if (it) void this.deck.play(it); }
    }
  }

  // ---- commands from the director ----

  private command(c: Command): void {
    // A stop is for what was on air when it was issued. A start that came after it (the skip's replacement, which the deck's `play` puts in the place of what sounds) makes it stale: run late, behind a pump that was
    // fetching words, it would stop the very entry that had just been started (review B1-1). It runs only while the director is still at the generation it was issued in.
    if (c.kind === 'stop') { const gen = c.gen; this.chain = this.chain.then(async () => { if (this.director.generation === gen) await this.deck.act('stop'); }).then(() => undefined, () => undefined); return; }       // in the pump's chain: the answer that pumps waits for it (a native call outside an answer is never run)
    this.pendingStarts.push(c.entry);
    // Outside a pump (a verb from the screen) nothing reads the journal: pump now, which starts it.
    if (!this.inPump) void this.pump();
  }
  private inPump = false;
  /** The listener's own Pause and Play (`toggle`): counted, so a start whose words were being fetched when it was pressed can tell it was paused meanwhile and does not play over it. */
  private pauseEpoch = 0; private listenerPaused = false;
  /** What was last handed the deck for each held id (deck-ops.ts `heldPlan`): a held entry the desk has rewritten since is handed over again. */
  private heldSent = new Map<string, string>();
  /** Clips the listener kept listening to: the episode went on past the clip's end. */
  private kept = new Set<string>();
  /** Story routes and the pulse the writer last tried their words: a failed fetch waits five pulses. */
  private wordsTriedAt = new Map<string, number>();
  /** Failed fetches since the words were last held: at the bound the entry fails and the station goes on. */
  private wordsTries = new Map<string, number>();
  /** Pulse number of the last dispatched start per entry, so a start that is in flight is not sent twice. */
  private startedAt = new Map<string, number>();
  /** Entries a start is running for right now (past its first await). */
  private starting = new Set<string>();
  private pumpCount = 0;

  /** One start at a time per entry: a pump and a replay can both see the deck without it. */
  private async startEntry(e: Entry, st: DeckState): Promise<void> {
    if (this.starting.has(e.id)) return;
    this.starting.add(e.id);
    try { await this.startEntryInner(e, st); } finally { this.starting.delete(e.id); }
  }

  private async startEntryInner(e: Entry, st: DeckState): Promise<void> {
    if (st.item === e.id) return;                               // the deck is already playing it
    const gen = this.director.generation, epoch = this.epoch, pe = this.pauseEpoch;
    const p = planEntry(e, this.director.provided.get(e.id), audioSpec(e));
    let item: DeckItem | undefined, why = '';
    const speech = (id: string, title: string, document: string, paragraphs: string[]) => deckItem({ id, kind: 'speech', title, document, paragraphs: paragraphs.join('\n\n'), requiresPreviousHeard: e.kind === 'transition', segment: segmentOf(e.kind) });
    try {
      switch (p.kind) {
        case 'speak': item = speech(e.id, e.title, p.document, p.paragraphs); break;
        case 'story': {
          // The words come from the writer's cache when it has them; a miss fetches here (a pump on a
          // slow network can be let go, and `replayOnAir` sends the start again on the next hold).
          const w = await this.resolvers.story?.(p.route); if (w) item = speech(e.id, w.title || e.title, 'pub:' + p.route, w.paragraphs); else why = 'the story could not be loaded'; break;
        }
        case 'webPage': { const w = await this.resolvers.webPage?.(p.url, p.title); if (w) item = speech(e.id, w.title || e.title, 'rs:' + e.id, w.paragraphs); else why = 'the page could not be read'; break; }
        case 'book': { const w = await this.resolvers.book?.(p.id, p.at); if (w) item = speech(e.id, w.title || e.title, 'rs:' + e.id, w.paragraphs); else why = 'the book could not be read'; break; }
        case 'take': { const t = await this.resolvers.take?.(p.id); if (t) item = deckItem({ id: e.id, kind: 'audio', title: e.title, show: e.source?.label ?? '', url: t.url }); else why = 'the take could not be had'; break; }
        case 'file': { const f = await this.resolvers.file?.(p.address); if (f) item = deckItem({ id: e.id, kind: 'audio', title: p.title, show: 'Files', url: f.url }); else why = 'the file could not be found'; break; }
        case 'audio': item = deckItem({ id: e.id, kind: 'audio', title: p.title, show: p.show, url: p.url, start: p.start ?? 0, end: p.end ?? 0 }); break;
        case 'cannot': why = p.why; break;
      }
    } catch (e) { if (refusedHere(e)) return; why = 'it could not be loaded'; }   // an Exact refusal is this answer let go, not a failed story: the next hold starts it again
    // cut, skipped or signed out while the words were being fetched: never played late
    if (epoch !== this.epoch || this.director.generation !== gen || this.director.playingId !== e.id) return;
    if (!item) { console.log(`radio: ${e.id} did not start: ${why}`); this.director.failedToStart(e.id, why); return; }
    this.known.set(e.id, item);
    this.startedAt.set(e.id, this.pumpCount);
    await this.deck.play(item);
    if (this.pauseEpoch !== pe && this.listenerPaused) await this.deck.act('pause');        // the listener pressed Pause while the words were on their way: the entry is on air, and paused (Play starts it)
    await this.keepHeld();
  }

  // ---- what the deck holds ----

  /** Entries whose sound is in the entry: speech from its words, audio from its address. The first that is not stops the list (order is kept). */
  private holdable(): DeckItem[] {
    const out: DeckItem[] = [];
    for (const e of this.store.ahead()) {
      if (out.length >= HOLD_AHEAD) break;
      let it: DeckItem | undefined;
      if (SPEAKS.has(e.kind) && e.text) it = deckItem({ id: e.id, kind: 'speech', title: e.title, document: 'rs:' + e.id, paragraphs: e.text, requiresPreviousHeard: e.kind === 'transition', segment: segmentOf(e.kind) });
      else if ((e.kind === 'episode' || e.kind === 'clip') && !e.text && !e.source?.ref.startsWith('file:')) {
        const s = audioSpec(e);
        if (s) it = deckItem({ id: e.id, kind: 'audio', title: s.title, show: s.show, url: s.url, start: s.start ?? 0, end: s.end ?? 0 });
      }
      if (!it) break;
      out.push(it);
    }
    return out;
  }
  /** Is there pump work a pulse might be let go in the middle of: an entry to start, or a held list that is not the sheet's next entries? (A mutation then does it and stays until it is done.) */
  /** What the deck last said it holds and plays (for the console). */
  knownCount(): number { return this.known.size; }
  /** What was last handed to the deck for this entry id: its id and `paragraphs` text, "" for both when none (the sentence follow checks its digest against the deck's place, so a rewritten text lights nothing). */
  handed(id: string): { id: string; text: string } {
    const it = this.known.get(id);
    return it && it.kind === 'speech' ? { id, text: it.paragraphs } : { id: '', text: '' };
  }
  heldSentCount(): number { return this.heldSent.size; }
  deckLine(): { held: number; item: string } { return { held: this.deckState?.held.length ?? 0, item: this.deckState?.item ?? '' }; }
  /** The app is in front, as of the last read of the native facts. */
  inFront = (): boolean => this.facts.foreground;
  /** The native facts a caller has just read (the bridge's `getRadio`): kept as `refreshFacts` would keep them. */
  useFacts(f: NativeFacts): void { this.facts = f; }
  /** One read of the deck and the native facts, no more (a mutation's last look, `radioHold`): the pump is `radioHold`'s. */
  /** The route of the on-air story whose words are not held, if any. The plan (not the entry's kind)
   *  says what is a story: `plan` turns a `page` whose ref is `pub:` into a story. */
  private missingWordsRoute(): string | undefined {
    const air = this.store.onAir();
    if (!air) return undefined;
    const p = planEntry(air, this.director.provided.get(air.id), audioSpec(air));
    if (p.kind !== 'story') return undefined;
    const route = p.route.split('@')[0];
    if (this.hasWords(route)) { this.wordsTriedAt.delete(route); this.wordsTries.delete(route); return undefined; }
    return route;
  }

  /** Is a fetch for the on-air story's words due now? A failed fetch backs off five pulses, never
   *  stops: a story that could not be had once is tried again, so a transient network failure
   *  cannot leave the station silent forever. */
  private wordsDue(route: string): boolean {
    const at = this.wordsTriedAt.get(route);
    return at === undefined || this.pumpCount - at >= 5;
  }
  wantsWords(): boolean { const route = this.missingWordsRoute(); return route !== undefined && this.wordsDue(route); }

  /** Fetch the on-air story's words outside a pump (a pump that waits on the network is let go, and
   *  its deck call with it). The write asks each pulse while it is due; a failure waits five pulses. */
  async prefetchOnAir(): Promise<void> {
    const route = this.missingWordsRoute();
    if (route === undefined || !this.wordsDue(route)) return;
    this.wordsTriedAt.set(route, this.pumpCount);
    try { await this.prefetchWords(route); } catch { /* counted below */ }
    if (!this.hasWords(route)) this.wordsTries.set(route, (this.wordsTries.get(route) ?? 0) + 1);
  }

  /** On air but not in the deck (a send let go, or words that were not held): start it from the held words. */
  async replayOnAir(): Promise<void> {
    const air = this.store.onAir();
    if (!air || this.director.playingId !== air.id) return;
    if (this.facts.turnOpen) return;                       // the listener is speaking: an entry that ended waits for the turn, and so does its replay
    if (this.inPump || this.starting.has(air.id) || this.pendingStarts.some((e) => e.id === air.id)) return;   // a pump is already starting it (a screen verb's own pump)
    const epoch = this.epoch;
    // The deck's state NOW (not the pump's read before it started anything): if the start landed,
    // its item is this entry; only a start that never reached the deck is sent again.
    let st: DeckState;
    try { st = await this.deck.state(); } catch { return; }
    if (epoch !== this.epoch || this.director.playingId !== air.id || this.store.onAir()?.id !== air.id) return;
    if (this.inPump || this.starting.has(air.id) || this.pendingStarts.some((e) => e.id === air.id)) return;   // the pump that was just asked has it
    if (st.item === air.id) return;
    // The deck's own last word wins: an entry it stopped, the listener paused, or that ended or
    // failed since the pump read its journal is not played again.
    if (st.why === 'stopped') return;
    if (st.journal.some((l) => l.item === air.id && l.seq > this.lastSeq && (l.type === 'ended' || l.type === 'system'))) return;   // ended, failed, stopped or replaced: the deck's own word stands
    // Words that are not held are fetched here (the hold), not left to the write: the write backs
    // off after a failure, and this is the only path that starts an entry the pump let go.
    if (this.missingWordsRoute() !== undefined) {
      await this.prefetchOnAir();
      if (epoch !== this.epoch) return;
      const route = this.missingWordsRoute();
      if (route !== undefined) {
        // Twelve failed fetches (about a minute with the five-pulse backoff): the story cannot be
        // had, so it is failed and the station goes on rather than staying silent forever.
        if ((this.wordsTries.get(route) ?? 0) >= 12) { this.wordsTries.delete(route); this.wordsTriedAt.delete(route); this.director.failedToStart(air.id, 'the story could not be loaded'); }
        return;
      }
    }
    // A start this process dispatched a moment ago is in flight (deckPlay is not waited for): the
    // deck's next state will show it, and sending it twice would restart the spoken item.
    if (this.pumpCount - (this.startedAt.get(air.id) ?? -1_000) < 3) return;
    await this.startEntry(air, st);
  }

  async peek(): Promise<void> { await this.refreshFacts(); try { this.deckState = await this.deck.state(); } catch { /* the next pulse reads again */ } this.countSpoken(); }
  /** The deck's state as the contract read it (the package source `deckState`, handed to the pulse): the pulse makes no deck call of its own. */
  seeDeck(st: DeckState): void { this.deckState = st; this.countSpoken(); }
  private countSpoken(): void {
    const air = this.store.onAir(), st = this.deckState, now = clock.ms;
    if (!air || audioSpec(air)) { this.spoken = { id: '', ms: 0, last: -1 }; return; }
    if (this.spoken.id !== air.id) this.spoken = { id: air.id, ms: 0, last: -1 };
    const playing = st?.item === air.id && st.why === 'playing';
    if (playing && this.spoken.last >= 0 && now > this.spoken.last) this.spoken.ms += now - this.spoken.last;
    this.spoken.last = playing ? now : -1;
  }
  needsWork(): boolean {
    if (this.pendingStarts.length > 0) return true;
    const air = this.store.onAir();
    const why = this.deckState?.why;
    if (air && this.deckState && this.deckState.item !== air.id && why !== 'stopped' && !this.facts.turnOpen) return true;   // on air, not in the deck: the start must be sent again (an open turn is the turn's own business)
    if (this.deckState?.journal.some((l) => l.seq > this.lastSeq)) return true;               // lines the deck wrote that the pump has not read
    const wanted = this.director.wants || this.store.onAir() ? this.holdable() : [];
    const held = this.deckState?.held ?? [];
    return wanted.length !== held.length || wanted.some((w, i) => w.id !== held[i]);
  }
  // Why the held list is native (a reason only native code can meet): on iOS shared code runs in the background only while audio is SOUNDING with the audio background mode, and may freeze in a gap, while a
  // block is synthesised or after a pause (docs/reference/exact/native-modules.md "Open questions", Exact docs/contract-for-agents.md:4929); on Android it freezes when the activity stops. The deck must
  // therefore hold the next entries itself (up to Deck.heldMax, only those whose sound needs no fetch) and play on alone; this keeps that list full while shared code runs. Unrun on a locked phone.
  private async keepHeld(): Promise<void> {
    const epoch = this.epoch;
    const wanted = this.director.wants || this.store.onAir() ? this.holdable() : [];
    if (epoch !== this.epoch) return;
    for (const w of wanted) this.known.set(w.id, w);
    for (const id of [...this.known.keys()]) if (!this.store.entry(id)) this.known.delete(id);        // what left the sheet is forgotten (each held up to 8 KB of words)
    for (const id of [...this.heldSent.keys()]) if (!this.store.entry(id)) this.heldSent.delete(id);   // and so is what was handed to the deck for it (whole items as JSON)
    try { await syncHeld(this.deck, wanted, this.heldSent); } catch { /* the deck did not answer: the next pump holds again */ }
  }

  // ---- restart and sign-out ----

  /** The sheet has been read from its file (or there was none): kept as a value, never as a promise another answer awaits (docs/problems/radio-restore-empty-after-let-go-2026-11-25.md). */
  get restored(): boolean { return this.isRestored; }
  private isRestored = false;
  /** Restored with nothing read (the bridge removed a sheet file a sign-out left behind). */
  markRestored(): void { this.isRestored = true; }
  /** The sheet from the runtime's own file. Nothing plays. A read Exact refused THROWS and leaves it not restored (the caller restores again later): it is never taken as an empty sheet.
   *  Two restores may overlap (two mutations): each reads, and only the first to finish takes what it read. A sign-out while it read (`epoch` moved) drops what it read: the old account's sheet is
   *  never taken, nor written back after the wipe removed its file. */
  /** Ends the turn before the sheet is parsed: the parse of a large sheet is over Exact's 100 ms turn
   *  budget on the phone (the first restore was refused at 108 ms), and the mutation must complete. */
  private static async turnEnd(): Promise<void> {
    const sc = (globalThis as { scheduler?: { yield?: () => Promise<void> } }).scheduler;
    if (sc?.yield) await sc.yield();
  }

  async restore(): Promise<void> {
    if (this.isRestored) return;
    const epoch = this.epoch;
    let st: DeckState | undefined;
    try { st = await this.deck.state(); } catch (e) { if (refusedHere(e)) throw e; /* no deck: the entry on air is closed as interrupted */ }
    const text = await this.store.readSaved();
    if (this.isRestored || epoch !== this.epoch) return;
    this.isRestored = true;
    if (text !== '') await RadioRuntime.turnEnd();
    this.store.restoreFrom(text, (store) => { if (st) this.recover(store, st); });
    if (st && this.lastSeq > st.acked) { try { await this.deck.ack(this.lastSeq); } catch { /* acknowledged at the next pump */ } }
    await RadioRuntime.turnEnd();
  }

  /** At launch: the deck's journal outlived the process, and it says what happened while shared code was not running. An entry the deck began and finished is aired and ended on the sheet
   *  with the deck's own verdict and time before anything closes the on-air entry as interrupted; so a finished entry does not play again, and a verdict heard is kept. */
  private recover(store: RunSheetStore, st: DeckState): void {
    const lines = st.journal.filter((l) => l.seq > this.lastSeq).sort((a, b) => a.seq - b.seq);
    for (const l of lines) {
      if (l.type === 'started') {
        if (!store.onAir() && store.ahead(1)[0]?.id === l.item) store.airedOnAir(l.item);
      } else if (l.type === 'ended') {
        const air = store.onAir();
        if (!air || air.id !== l.item) continue;
        const report: AirReport = { block: l.block, seconds: l.seconds, at: l.at };
        if (l.end === 'finished' && l.rendered) store.airedEnded(l.item, 'heard', 'finished', report);
        else if (l.end === 'failed' && l.reason !== 'interrupted by a restart') store.airedEnded(l.item, 'skipped', 'failed', NO_REPORT, l.reason);
      }
    }
    if (lines.length > 0) this.lastSeq = lines[lines.length - 1].seq;
  }

  /** Sign-out: what was on air stops, the deck forgets, the sheet and its file go. A line for the old account that arrives later does nothing. */
  async signOut(): Promise<void> {
    this.epoch += 1;
    this.director.stopAll(this.report());
    this.pendingStarts = []; this.known.clear(); this.kept.clear();
    try { await this.deck.act('stop'); } catch { /* the deck is gone */ }
    if (this.store.poster) await this.store.settlePending(3000);          // the `left` report reaches the computer while the old key is still held; what is not answered by then is dropped, never sent under a later account
    await this.store.wipe();
    this.facts = NO_FACTS; this.deckState = undefined; this.heldSent.clear(); this.spoken = { id: '', ms: 0, last: -1 };
    this.onChange?.();
  }
}
