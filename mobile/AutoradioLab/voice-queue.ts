// The turn route's queue words over the run sheet (it was Swift: TurnQueueWire.swift; docs/reference/AUTORADIO-TURNS.md, "The queue"). The voice turn is native; it carries the queue
// SNAPSHOT this module makes with every turn (`queueSnapshot`) and hands back the server's `queue_ops` event whole, which `applyTurnOps` translates onto run sheet ops. The run sheet is the one list.
//   out: {version, current:{id,item,position_seconds}|null, upcoming:[{id,item,added_by}], on_air:{id,title,claim_id?,stories?}}   (first 50 entries ahead)
//   in:  {"turn_id","base_version","ops":[{"op":"play_now|play_next|enqueue|remove|move|skip|previous|clear", ...}]}
// Ways this can fail (tests in voice-queue.test.ts): an item the turn route cannot take (an address over its bound) is left out; a replayed turn changes the sheet twice (a turn's ops apply once, and
// every op carries an op_id made from it); an op the sheet refuses is refused loudly (counted, a line in `refused`, a line in the log); a garbled op stops no other.
import { audioSpec, bookOf, cutBytes, displayTitle, paperRoute, paperStory, book as bookEntry, audio as audioEntry, webPage, webURL } from './runsheet-entries.ts';
import { apply as reduce, type Entry, type Op } from './runsheet-model.ts';
import type { RadioRuntime } from './radio-runtime.ts';

const bytes = (s: string): number => new TextEncoder().encode(s).length;
const clamp = (s: string, n: number): string => cutBytes(s, n);
const obj = (v: unknown): Record<string, unknown> | undefined => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : undefined);

// An entry's id is sent as it is: the run sheet's reducer refuses an id outside the server's grammar (runsheet-model.ts `validId`, the Go `entryID`; voice-queue.test.ts Q11 holds them together),
// so no entry on the sheet can carry one the server would refuse the whole turn for.
/** What the listener sees as up next: the station's own transitions between entries are not on the page (radio-model.ts), so the desk is not told of them and its indexes count what is shown. */
export const visibleAhead = (rt: RadioRuntime): Entry[] => rt.store.ahead().filter((e) => e.kind !== 'transition');

/** The route a spoken entry is told by. The server's `story` item needs a route and uses it only to know a story is already queued: a programme story that tells a paper story carries that
 *  story's route (so the desk moves it rather than adding it twice); any other spoken entry carries its own address, which no candidate can equal. */
const spokenRoute = (e: Entry): string => {
  const ref = e.source?.ref;
  if (e.kind === 'story' && ref !== undefined && ref.startsWith('pub:')) { const r = ref.slice(4).split('@')[0]; if (r !== '') return r; }
  return 'programme:' + e.id;
};

/** The wire item for an entry, or undefined when it has none the server can take. The server's kinds are story (a route), page (a url), document (an id) and audio (a url); every kind the run
 *  sheet holds maps onto one: paper stories and the programme's own spoken entries (story, opening, transition, take, answer, result) are `story`; a web page is `page`; a book, a file or a message
 *  that the phone reads from its own store is `document`; a clip or episode is `audio`. What the server cannot say yet (that an entry is an opening, a take or a message; how long it is) is listed
 *  in docs/reference/AUTORADIO-TURNS.md under "The queue". */
export function wireItem(e: Entry): Record<string, unknown> | undefined {
  const title = clamp(displayTitle(e), 300);
  const ref = e.source?.ref ?? '';
  switch (e.kind) {
    case 'page': {
      const r = paperRoute(e);
      if (r !== undefined) return bytes(r) <= 256 ? { kind: 'story', route: r, title } : undefined;
      const u = webURL(e);
      return u !== undefined && bytes(u) <= 1024 ? { kind: 'page', url: u, title } : undefined;
    }
    case 'document': case 'mail': {
      const b = bookOf(e);
      if (b) return bytes(b.id) <= 128 ? { kind: 'document', id: b.id, title, at: b.at } : undefined;
      return ref !== '' && bytes(ref) <= 128 ? { kind: 'document', id: ref, title } : undefined;       // a message or a file: named by its address on the phone
    }
    case 'clip': case 'episode': {
      const a = audioSpec(e);
      if (!a) return ref !== '' && bytes(ref) <= 1024 ? { kind: 'audio', url: ref, title, show: clamp(e.source?.label ?? '', 200) } : undefined;   // a file on the phone: its address
      if (bytes(a.url) > 1024) return undefined;
      const o: Record<string, unknown> = { kind: 'audio', url: a.url, title, show: clamp(a.show, 200) };
      if (a.start !== undefined) o.start = a.start;
      if (a.end !== undefined) o.end = a.end;
      if (a.episode_id !== undefined && bytes(a.episode_id) <= 128) o.episode_id = a.episode_id;
      return o;
    }
    default: {                                   // story, opening, transition, take, answer, result: the programme's own words
      const route = spokenRoute(e);
      return bytes(route) <= 256 ? { kind: 'story', route, title } : undefined;
    }
  }
}

/** The lists that were sent, by the sheet's version (the snapshot's `version`, which the server answers as `base_version`): the entry ids of `upcoming`, in the order the server counted them. A `move`'s
 *  `to_index` is a place in THAT list, not in what the sheet shows when the reply arrives (an entry with no wire item, the 50 bound, an entry that ended or was added in the 5 to 10 s of a turn). */
const sentLists: { version: number; ids: string[] }[] = [];
const rememberSent = (version: number, ids: string[]): void => {
  const i = sentLists.findIndex((l) => l.version === version);
  if (i >= 0) sentLists.splice(i, 1);
  sentLists.push({ version, ids }); if (sentLists.length > 16) sentLists.shift();
};

/** The run sheet as the turn route's `queue` field (JSON text, sorted keys): what is on air and what is up next, as the listener sees them, in order. */
export function turnQueueJSON(rt: RadioRuntime, positionBlock = 0, maxUpcoming = 50): string {
  const by = (b: Entry['placed_by']): string => (b === 'listener' ? 'listener' : b === 'desk' ? 'dj' : 'programme');
  const store = rt.store, air = store.onAir();
  const airItem = air ? wireItem(air) : undefined;
  const upcoming = visibleAhead(rt).flatMap((e) => { const i = wireItem(e); return i ? [{ id: e.id, item: i, added_by: by(e.placed_by) }] : []; }).slice(0, maxUpcoming);
  rememberSent(store.sheet.version, upcoming.map((u) => u.id));
  const o: Record<string, unknown> = { version: store.sheet.version, current: air && airItem ? { id: air.id, item: airItem, position_seconds: Math.min(Math.max(positionBlock, 0), 10_000_000) } : null, upcoming };
  if (air) {
    const a: Record<string, unknown> = { id: air.id, title: clamp(displayTitle(air), 300) };
    if (air.forecast?.claim_id) a.claim_id = air.forecast.claim_id;
    const stories: string[] = [];
    for (const c of air.cites ?? []) if (c.ref.startsWith('pub:')) { const r = c.ref.slice(4).split('@')[0]; if (r !== '' && !stories.includes(r)) stories.push(r); }
    if (stories.length) a.stories = stories.slice(0, 12);
    o.on_air = a;
  }
  return JSON.stringify(sortKeys(o));
}
function sortKeys(v: unknown): unknown {
  if (Array.isArray(v)) return v.map(sortKeys);
  const o = obj(v);
  return o ? Object.fromEntries(Object.keys(o).sort().map((k) => [k, sortKeys(o[k])])) : v;
}

function entryFromWire(item: Record<string, unknown>): Entry | undefined {
  const title = typeof item.title === 'string' ? item.title : '';
  switch (item.kind) {
    case 'story': return typeof item.route === 'string' && item.route !== '' ? paperStory(item.route, title, 'desk') : undefined;
    case 'page': return typeof item.url === 'string' && item.url !== '' ? webPage(item.url, title, 'desk') : undefined;
    case 'document': return typeof item.id === 'string' ? bookEntry(item.id, title, typeof item.at === 'number' ? Math.trunc(item.at) : 0, 'desk') : undefined;
    case 'audio': {
      if (typeof item.url !== 'string' || item.url === '') return undefined;
      const a: Parameters<typeof audioEntry>[0] = { url: item.url, title, show: typeof item.show === 'string' ? item.show : '' };
      if (typeof item.start === 'number') a.start = item.start;
      if (typeof item.end === 'number') a.end = item.end;
      if (typeof item.episode_id === 'string') a.episode_id = item.episode_id;
      return audioEntry(a, undefined, 'desk');
    }
    default: return undefined;
  }
}

const turnLog: string[] = [];
export const resetTurnLog = (): void => { turnLog.length = 0; sentLists.length = 0; };

/** What the turn route answered (the `queue_ops` event, JSON text). The old rebase rule holds by the reducer's own rules: an op that names an entry applies only while that entry is ahead; additions
 *  always apply; `clear` and `previous` only when the sheet is still at `base_version`. */
export type TurnOpsResult = { applied: number; dropped: number; /** One plain line for each op that did not apply: which op, which entry, why. Also logged. */ refused: string[] };
export function applyTurnOps(rt: RadioRuntime, text: string): TurnOpsResult {
  let json: unknown;
  try { json = JSON.parse(text); } catch { return { applied: 0, dropped: 0, refused: [] }; }
  const o = Array.isArray(json) ? { ops: json } : obj(json);
  if (!o) return { applied: 0, dropped: 0, refused: [] };
  const turn = typeof o.turn_id === 'string' ? o.turn_id : '';
  const base = typeof o.base_version === 'number' ? o.base_version : undefined;
  const raw = Array.isArray(o.ops) ? o.ops : Array.isArray(o.queue_ops) ? o.queue_ops : [];
  if (turn !== '' && raw.length) {
    if (turnLog.includes(turn)) { const why = `turn ${turn} was replayed; its ${raw.length} op(s) applied once already`; console.log(`radio: turn ops refused: ${why}`); return { applied: 0, dropped: raw.length, refused: [why] }; }
    turnLog.push(turn); if (turnLog.length > 100) turnLog.splice(0, turnLog.length - 100);
  }
  const store = rt.store;
  let applied = 0, dropped = 0;
  const refused: string[] = [];
  // An op that does not apply is said, never dropped quietly: the server spoke the change before it sent the op, so the phone's log is where "I said I moved it" and "it did not move" meet.
  const refuse = (what: string, why: string): void => { dropped += 1; const line = `${what}: ${why}`; refused.push(line); console.log(`radio: turn ops refused (turn ${turn || '-'}): ${line}`); };
  raw.forEach((any, n) => {
    const d = obj(any);
    if (!d || typeof d.op !== 'string') { refuse(`op ${n}`, 'not an op'); return; }
    const named = `${d.op}${typeof d.id === 'string' ? ' ' + d.id : ''}`;
    const reason = typeof d.reason === 'string' && d.reason !== '' ? d.reason.slice(0, 160) : 'the desk answered';
    const id = typeof d.id === 'string' ? d.id : undefined;
    const op_id = turn === '' ? store.mintOpId() : `t-${turn}-${n}`.slice(0, 64);
    let op: Op | undefined;
    switch (d.op) {
      case 'play_now': case 'play_next': case 'enqueue': {
        const item = obj(d.item), e = item ? entryFromWire(item) : undefined;
        if (!e) { refuse(named, 'the item is not one the phone can place'); return; }
        const at = d.op === 'play_now' ? 'now' : d.op === 'play_next' ? 'next' : 'end';
        if (at === 'now') rt.director.wants = true;
        op = { op_id, by: 'desk', op: 'place', at, entry: e, reason, base_version: 0 };
        break;
      }
      case 'remove': if (!id) { refuse(named, 'it names no entry'); return; } op = { op_id, by: 'desk', op: 'remove', id, reason, base_version: 0 }; break;
      case 'move': {
        const to = typeof d.to_index === 'number' ? Math.trunc(d.to_index) : undefined;
        if (!id || to === undefined) { refuse(named, 'a move needs an entry and a place'); return; }
        // The places are those of the list SENT with this turn (the snapshot of `base_version`; the latest sent when the op names none), so what was left out of it, cut at 50, ended or added since cannot shift the place.
        const want = typeof d.base_version === 'number' ? d.base_version : base;
        const list = want === undefined ? sentLists[sentLists.length - 1] : sentLists.find((l) => l.version === want);
        if (!list) { refuse(named, `the list the desk was shown (version ${want ?? 'latest'}) is not one this phone sent`); return; }
        if (!list.ids.includes(id)) { refuse(named, 'it was not in the list the desk was shown'); return; }
        const others = list.ids.filter((x) => x !== id), now = new Set(store.ahead().map((e) => e.id));
        let anchor: string | undefined;                                      // the entry it goes after: the nearest one at or before that place that is still ahead (one that has aired is behind it)
        for (let k = Math.min(Math.max(to, 0), others.length) - 1; k >= 0; k--) if (now.has(others[k])) { anchor = others[k]; break; }
        op = anchor === undefined ? { op_id, by: 'desk', op: 'move', at: 'next', id, reason, base_version: 0 } : { op_id, by: 'desk', op: 'move', at: 'after', after: anchor, id, reason, base_version: 0 };
        break;
      }
      case 'skip': {
        const air = store.onAir();
        if (!air) { refuse(named, 'nothing is on air'); return; }
        if (id !== undefined && id !== air.id) { refuse(named, `it is not what is on air (${air.id})`); return; }
        op = { op_id, by: 'desk', op: 'skip', id: air.id, ended: { outcome: 'skipped', at: '', last_block: 0 }, reason, base_version: 0 };
        break;
      }
      case 'previous': case 'clear': {
        const want = typeof d.base_version === 'number' ? d.base_version : base;
        if (want !== undefined && want !== store.sheet.version) { refuse(named, `the queue changed since the desk saw it (version ${want}, now ${store.sheet.version})`); return; }
        op = { op_id, by: 'desk', op: d.op, base_version: store.sheet.version, reason };
        break;
      }
      default: refuse(named, 'not an op the phone knows'); return;
    }
    const r = store.applyFromDesk(op);
    if (r.applied) applied += 1; else refuse(named, r.dropped ?? 'the run sheet refused it');
  });
  return { applied, dropped, refused };
}
export const wouldApply = (rt: RadioRuntime, op: Op): boolean => reduce(rt.store.sheet, op).result.applied;
