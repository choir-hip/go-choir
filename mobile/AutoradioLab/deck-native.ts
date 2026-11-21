// The real deck port: shared code's five deck names over the app module's ONE `deck` op (modules/apple/ModelsEntry.swift -> AutoradioModule+Deck.swift `deckCall`).
// Why it is native and why it is a single op: the deck is the audio (the session, the engine, the lock screen, interruptions), so only native code can play an item and keep
// the journal that survives a kill; shared code cannot call a package's names from a function (only a contract can), so it reaches them through the module's `later` like the
// key and the mail cipher. The interface is item-shaped and says nothing of AVPlayer: Exact's own `audio` element could play a remote item behind the same five names (docs/reference/exact/deck.md).
// A call that fails or an absent module answers the EMPTY state, never a throw: shared code goes on, and the next pump asks again.
import { nativeLater } from './api.ts';
import type { DeckEvent, DeckItem, DeckOp, DeckPort, DeckState, DeckWhy } from './deck.ts';

type Later = (request: { op: string; [key: string]: unknown }) => Promise<unknown>;

const WHYS: readonly string[] = ['playing', 'paused_listener', 'paused_system', 'held_session', 'held_engine', 'stopped', 'empty'];
const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

export const EMPTY_STATE: DeckState = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] };

function line(v: unknown): DeckEvent | undefined {
  const o = obj(v);
  if (typeof o.seq !== 'number' || typeof o.type !== 'string') return undefined;
  return { seq: o.seq, type: o.type as DeckEvent['type'], item: str(o.item), at: str(o.at), block: num(o.block), seconds: num(o.seconds), final: num(o.final), rendered: o.rendered === true, end: str(o.end) as DeckEvent['end'], command: str(o.command) as DeckEvent['command'], reason: str(o.reason) };
}

export function readState(raw: unknown): DeckState {
  const o = obj(raw);
  const why = WHYS.includes(str(o.why)) ? (str(o.why) as DeckWhy) : 'empty';
  return {
    why, item: str(o.item), block: num(o.block), seconds: num(o.seconds), duration: num(o.duration),
    held: Array.isArray(o.held) ? o.held.filter((x): x is string => typeof x === 'string') : [],
    seq: num(o.seq), acked: num(o.acked),
    journal: Array.isArray(o.journal) ? o.journal.map(line).filter((l): l is DeckEvent => l !== undefined) : [],
  };
}

/** The deck port over the app module. `later` is read at each call (the module can bind late): `nativeLater` by default. */
export function nativeDeck(later: () => Later | null = nativeLater as () => Later | null): DeckPort {
  const call = async (name: string, args: Record<string, unknown> = {}): Promise<DeckState> => {
    const l = later();
    if (!l) return EMPTY_STATE;
    try { return readState(await l({ op: 'deck', name, args })); } catch { return EMPTY_STATE; }
  };
  const fields = (i: DeckItem): Record<string, unknown> => ({ ...i });
  return {
    state: () => call('deckState'),
    play: (i) => call('deckPlay', fields(i)),
    enqueue: (i, at) => call('deckEnqueue', { ...fields(i), at: at ?? -1 }),
    act: (op: DeckOp, value?: number) => call('deckAct', { op, value: value ?? 0 }),
    ack: (seq) => call('deckAck', { through: seq }),
  };
}
