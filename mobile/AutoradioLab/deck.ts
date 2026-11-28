// The deck's interface, as shared code sees it: commands in, a state and a journal out. The deck is the small native audio surface
// under this code (Swift on iPhone, Kotlin on Android, one interface); see docs/reference/exact/deck.md for the rules and the reasons.
// These names are stable: other modules (the run sheet's driver) call them. A change to a name here is a change to
// mobile/exact-package/interface/deck.json, to deck-types.contract and to both native implementations.

/** The things the deck plays. `paragraphs` is one string, a blank line between paragraphs. */
export type DeckItem = {
  id: string;
  kind: 'speech' | 'audio';
  title: string;
  show: string;
  document: string;
  paragraphs: string;
  url: string;
  start: number;
  end: number;
  block: number;
  seconds: number;
  requiresPreviousHeard: boolean;
  /** What a speech item is, for the voice's segment naming: opening, transition, answer, result, mail or story ("" is story). */
  segment: string;
};

export type DeckWhy = 'playing' | 'paused_listener' | 'paused_system' | 'held_session' | 'held_engine' | 'stopped' | 'empty';

export type DeckEnd = 'finished' | 'skipped' | 'replaced' | 'stopped' | 'failed';

export type DeckRemote = 'play' | 'pause' | 'toggle' | 'next' | 'previous' | 'seek';
export type DeckSystem = 'interruption-began' | 'interruption-ended' | 'route-lost' | 'session-held' | 'session-back' | 'engine-lost' | 'dropped' | 'held-dry';

export type DeckEvent = {
  seq: number;
  type: 'started' | 'sounding' | 'ended' | 'remote' | 'system';
  item: string;
  /** RFC 3339, the device's clock when the deck wrote the line. */
  at: string;
  block: number;
  seconds: number;
  final: number;
  rendered: boolean;
  end: '' | DeckEnd;
  command: '' | DeckRemote | DeckSystem;
  reason: string;
};

/** Where the voice is in the entry's text, as native reports it (docs/design/sentence-follow-in-shared-code-2026-11-28.md). `entry` is the id native was given; `digest` is the length and FNV-1a of the entry's `paragraphs`
 *  string as native received it (radio-follow.ts `digestOf`); `start` and `length` are the unit being spoken, in UTF-16 code units of that string; `seconds` and `total` are media seconds in the unit and its length (0 when
 *  not known yet). An empty `entry` is no place. */
export type DeckPlace = { entry: string; digest: string; start: number; length: number; seconds: number; total: number };
export const NO_PLACE: DeckPlace = { entry: '', digest: '', start: 0, length: 0, seconds: 0, total: 0 };

export type DeckState = {
  why: DeckWhy;
  item: string;
  block: number;
  seconds: number;
  duration: number;
  place: DeckPlace;
  /** The ids of the items the deck holds after the one playing, in order. At most DECK_HELD_MAX (100). */
  held: string[];
  /** The last journal line's number. Numbers rise by one and are never reused, across a relaunch too. */
  seq: number;
  /** The highest number shared code has acknowledged; lines up to it are gone from `journal`. */
  acked: number;
  journal: DeckEvent[];
};

/** `deckAct` operations. `clearEnd`: the clip playing no longer ends (the episode goes on from where it is). `value` is the seconds for `seekTo` and `seekBy`, and for `trim` how many held items to keep (the rest are dropped). */
export type DeckOp = 'pause' | 'resume' | 'stop' | 'skip' | 'seekTo' | 'seekBy' | 'trim' | 'clearEnd';

/** The deck's generated @choir/autoradio package names (interface/deck.json); each answers the deck's state. `deckState` is also the live source the contract reads and hands to the pulse. */
export const DECK_NAMES = ['deckState', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck', 'deckWipe'] as const;

/** How many items after the one playing the deck holds, so it plays on while shared code is not running. It is every entry the sheet can have ahead (the phone's MAX_AHEAD, 100):
 *  what the deck holds is bounded by the sheet's own bound, and each item is a bounded text or an address (the server's entry bounds), a few hundred kilobytes at most. */
export const DECK_HELD_MAX = 100;

/** The port shared code drives the deck through. The real one calls the package's names; a test passes a fake. */
export type DeckPort = {
  state(): Promise<DeckState>;
  /** Play this item now, cutting what is playing (its end is "replaced"). */
  play(item: DeckItem): Promise<DeckState>;
  /**
   * Hold this item after the playing one. `at` is its place among the held items (0 is next); the held items from `at` on are dropped first,
   * so the held list is made in order. Absent or -1: at the end. An id already held is removed first. One past DECK_HELD_MAX is refused (the state's
   * `held` shows it).
   */
  enqueue(item: DeckItem, at?: number): Promise<DeckState>;
  act(op: DeckOp, value?: number): Promise<DeckState>;
  /** Journal lines up to `seq` are posted: the deck forgets them. */
  ack(seq: number): Promise<DeckState>;
  /** Account boundary: stop and forget the current item, held items, journal and synthesized-audio cache. */
  wipe(): Promise<DeckState>;
};

export const EMPTY_ITEM: DeckItem = {
  id: '', kind: 'speech', title: '', show: '', document: '', paragraphs: '', url: '', start: 0, end: 0, block: 0, seconds: 0, requiresPreviousHeard: false, segment: '',
};

/** A DeckItem with every field present, so what crosses the interface never carries a missing one. */
export function item(partial: Partial<DeckItem> & { id: string; kind: DeckItem['kind'] }): DeckItem {
  return { ...EMPTY_ITEM, ...partial };
}
