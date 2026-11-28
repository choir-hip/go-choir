// The real deck port: shared code's six generated @choir/autoradio package names. Exact's bake writes these functions from interface/deck.json and routes them to the Apple or Android implementation;
// Android has no Apple app-module bus. Native because the session, engine, media controls and durable journal are platform work; shared code still decides what plays next and what is posted.
// An absent package answers the EMPTY state. A failed send answers it too (the next pump asks again), but a failed deckState THROWS: an unreadable deck is not an empty one.
import { deckAck, deckAct, deckEnqueue, deckPlay, deckState, deckWipe } from '@choir/autoradio';
import { NO_PLACE, type DeckEvent, type DeckItem, type DeckOp, type DeckPlace, type DeckPort, type DeckState, type DeckWhy } from './deck.ts';

type PackageCall = (name: string, args: Record<string, unknown>) => Promise<unknown>;

const WHYS: readonly string[] = ['playing', 'paused_listener', 'paused_system', 'held_session', 'held_engine', 'stopped', 'empty'];
const obj = (v: unknown): Record<string, unknown> => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {});
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const num = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? v : 0);

export const EMPTY_STATE: DeckState = { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, place: NO_PLACE, held: [], seq: 0, acked: 0, journal: [] };

function line(v: unknown): DeckEvent | undefined {
  const o = obj(v);
  if (typeof o.seq !== 'number' || typeof o.type !== 'string') return undefined;
  return { seq: o.seq, type: o.type as DeckEvent['type'], item: str(o.item), at: str(o.at), block: num(o.block), seconds: num(o.seconds), final: num(o.final), rendered: o.rendered === true, end: str(o.end) as DeckEvent['end'], command: str(o.command) as DeckEvent['command'], reason: str(o.reason) };
}

function place(v: unknown): DeckPlace {
  const o = obj(v);
  return { entry: str(o.entry), digest: str(o.digest), start: num(o.start), length: num(o.length), seconds: num(o.seconds), total: num(o.total) };
}

export function readState(raw: unknown): DeckState {
  const o = obj(raw);
  const why = WHYS.includes(str(o.why)) ? (str(o.why) as DeckWhy) : 'empty';
  return {
    why, item: str(o.item), block: num(o.block), seconds: num(o.seconds), duration: num(o.duration), place: place(o.place),
    held: Array.isArray(o.held) ? o.held.filter((x): x is string => typeof x === 'string') : [],
    seq: num(o.seq), acked: num(o.acked),
    journal: Array.isArray(o.journal) ? o.journal.map(line).filter((l): l is DeckEvent => l !== undefined) : [],
  };
}

/** The deck's state as the contract read it (the package source `deckState`, spine.contract `ds`, handed to the pulse). The contract's placeholder before the first answer (`empty()`: no `why`) is
 *  undefined: not read is not an empty deck. */
export function contractDeck(raw: unknown): DeckState | undefined {
  return WHYS.includes(str(obj(raw).why)) ? readState(raw) : undefined;
}

// A *start* is dispatched and not waited for: the deck acts natively at once, and its journal and
// state reach shared code through `deckState` on the next radio tick. Waiting here held a data
// answer open across the deck's start (seconds of on-device speech, a foreground service) and the
// answer was let go, so a media-session Next could never land. A send that fails is logged; the
// state source is the truth. The deck's other operations are fast (a player pause, a journal ack, a
// wipe) and are waited for: their own answer is the state after the act, and a caller such as
// "Keep listening" must not record the act before the deck has taken it.
const packageDeck: PackageCall = async (name, args) => {
  const sent = (call: Promise<unknown>): DeckState => { call.catch((e: unknown) => console.log(`deck: ${name} failed: ${e instanceof Error ? e.message : String(e)}`)); return EMPTY_STATE; };
  switch (name) {
    case 'deckState': return deckState({ tick: num(args.tick) });
    case 'deckPlay': return sent(deckPlay(args as unknown as Parameters<typeof deckPlay>[0]));
    case 'deckEnqueue': return sent(deckEnqueue(args as unknown as Parameters<typeof deckEnqueue>[0]));
    case 'deckAct': return deckAct(args as unknown as Parameters<typeof deckAct>[0]);
    case 'deckAck': return deckAck(args as unknown as Parameters<typeof deckAck>[0]);
    case 'deckWipe': return deckWipe();
    default: throw new Error(`unknown deck package name ${name}`);
  }
};

/** The app's deck port; a test replaces it with a fake (the generated package names have no host under Bun). */
let portForTest: (() => DeckPort) | undefined;
export function setDeckPortForTest(port: (() => DeckPort) | undefined): void { portForTest = port; }
export function deckPort(): DeckPort { return portForTest ? portForTest() : nativeDeck(); }

/** The deck port over the generated package functions. The injectable call is for tests; the app uses Exact's binding. */
export function nativeDeck(call: PackageCall = packageDeck): DeckPort {
  const send = async (name: string, args: Record<string, unknown> = {}): Promise<DeckState> => {
    try { return readState(await call(name, args)); } catch (e) {
      const absent = (e as { name?: unknown } | null)?.name === 'NotSupportedError' || (e as { code?: unknown } | null)?.code === 'NotSupportedError';
      if (name === 'deckState' && !absent) throw e;
      if (absent && name === 'deckPlay') console.log('deck: play asked and the deck package is not bound here: nothing plays');
      else if (!absent) console.log(`deck: ${name} failed: ${e instanceof Error ? e.message : String(e)}`);
      return EMPTY_STATE;
    }
  };
  return {
    state: () => send('deckState', { tick: 0 }),
    play: (i) => send('deckPlay', { item: i }),
    enqueue: (i, at) => send('deckEnqueue', { item: i, at: at ?? -1 }),
    act: (op: DeckOp, value?: number) => send('deckAct', { op, value: value ?? 0 }),
    ack: (seq) => send('deckAck', { through: seq }),
    wipe: () => send('deckWipe'),
  };
}
