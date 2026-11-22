// The turn route's queue words over the run sheet (it was Swift: TurnQueueWire.swift; docs/reference/AUTORADIO-TURNS.md, "The queue"). The voice turn is native; it carries the queue
// SNAPSHOT this module makes with every turn (`queueSnapshot`) and hands back the server's `queue_ops` event whole, which `applyTurnOps` translates onto run sheet ops. The run sheet is the one list.
//   out: {version, current:{id,item,position_seconds}|null, upcoming:[{id,item,added_by}], on_air:{id,title,claim_id?,stories?}}   (first 50 entries ahead)
//   in:  {"turn_id","base_version","ops":[{"op":"play_now|play_next|enqueue|remove|move|skip|previous|clear", ...}]}
// Ways this can fail (tests in voice-queue.test.ts): an item the turn route cannot take (an address over its bound) is left out; a replayed turn changes the sheet twice (a turn's ops apply once, and
// every op carries an op_id made from it); an op the sheet refuses is dropped silently; a garbled op stops no other.
import { audioSpec, bookOf, cutBytes, displayTitle, paperRoute, paperStory, book as bookEntry, audio as audioEntry, webPage, webURL } from './runsheet-entries.ts';
import { apply as reduce, type Entry, type Op } from './runsheet-model.ts';
import { scoped } from './api.ts';
import type { RadioRuntime } from './radio-runtime.ts';

const bytes = (s: string): number => new TextEncoder().encode(s).length;
const clamp = (s: string, n: number): string => cutBytes(s, n);
const obj = (v: unknown): Record<string, unknown> | undefined => (typeof v === 'object' && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : undefined);

/** The wire item for an entry, or undefined when it has none the desk can take. */
export function wireItem(e: Entry): Record<string, unknown> | undefined {
  const title = clamp(displayTitle(e), 300);
  switch (e.kind) {
    case 'page': {
      const r = paperRoute(e);
      if (r !== undefined) return bytes(r) <= 256 ? { kind: 'story', route: r, title } : undefined;
      const u = webURL(e);
      return u !== undefined && bytes(u) <= 1024 ? { kind: 'page', url: u, title } : undefined;
    }
    case 'document': { const b = bookOf(e); return b && bytes(b.id) <= 128 ? { kind: 'document', id: b.id, title, at: b.at } : undefined; }
    case 'clip': case 'episode': {
      const a = audioSpec(e);
      if (!a || bytes(a.url) > 1024) return undefined;
      const o: Record<string, unknown> = { kind: 'audio', url: a.url, title, show: clamp(a.show, 200) };
      if (a.start !== undefined) o.start = a.start;
      if (a.end !== undefined) o.end = a.end;
      if (a.episode_id !== undefined && bytes(a.episode_id) <= 128) o.episode_id = a.episode_id;
      return o;
    }
    default: return undefined;        // spoken entries have no address to hand over
  }
}

/** The run sheet as the turn route's `queue` field (JSON text, sorted keys). */
export function turnQueueJSON(rt: RadioRuntime, positionBlock = 0, maxUpcoming = 50): string {
  const by = (b: Entry['placed_by']): string => (b === 'listener' ? 'listener' : b === 'desk' ? 'dj' : 'programme');
  const store = rt.store, air = store.onAir();
  const airItem = air ? wireItem(air) : undefined;
  const upcoming = store.ahead().flatMap((e) => { const i = wireItem(e); return i ? [{ id: e.id, item: i, added_by: by(e.placed_by) }] : []; }).slice(0, maxUpcoming);
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

const T = scoped(() => ({ log: [] as string[] }));      // the ids of the voice turns this world applied: the world's (api.ts `scoped`)
export const resetTurnLog = (): void => { T().log.length = 0; };

/** What the turn route answered (the `queue_ops` event, JSON text). The old rebase rule holds by the reducer's own rules: an op that names an entry applies only while that entry is ahead; additions
 *  always apply; `clear` and `previous` only when the sheet is still at `base_version`. */
export function applyTurnOps(rt: RadioRuntime, text: string): { applied: number; dropped: number } {
  let json: unknown;
  try { json = JSON.parse(text); } catch { return { applied: 0, dropped: 0 }; }
  const o = Array.isArray(json) ? { ops: json } : obj(json);
  if (!o) return { applied: 0, dropped: 0 };
  const turn = typeof o.turn_id === 'string' ? o.turn_id : '';
  const base = typeof o.base_version === 'number' ? o.base_version : undefined;
  const raw = Array.isArray(o.ops) ? o.ops : Array.isArray(o.queue_ops) ? o.queue_ops : [];
  if (turn !== '' && raw.length) {
    if (T().log.includes(turn)) return { applied: 0, dropped: raw.length };
    T().log.push(turn); if (T().log.length > 100) T().log.splice(0, T().log.length - 100);
  }
  const store = rt.store;
  let applied = 0, dropped = 0;
  raw.forEach((any, n) => {
    const d = obj(any);
    if (!d || typeof d.op !== 'string') { dropped += 1; return; }
    const reason = typeof d.reason === 'string' && d.reason !== '' ? d.reason.slice(0, 160) : 'the desk answered';
    const id = typeof d.id === 'string' ? d.id : undefined;
    const op_id = turn === '' ? store.mintOpId() : `t-${turn}-${n}`.slice(0, 64);
    let op: Op | undefined;
    switch (d.op) {
      case 'play_now': case 'play_next': case 'enqueue': {
        const item = obj(d.item), e = item ? entryFromWire(item) : undefined;
        if (!e) { dropped += 1; return; }
        const at = d.op === 'play_now' ? 'now' : d.op === 'play_next' ? 'next' : 'end';
        if (at === 'now') rt.director.wants = true;
        op = { op_id, by: 'desk', op: 'place', at, entry: e, reason, base_version: 0 };
        break;
      }
      case 'remove': if (!id) { dropped += 1; return; } op = { op_id, by: 'desk', op: 'remove', id, reason, base_version: 0 }; break;
      case 'move': {
        const to = typeof d.to_index === 'number' ? Math.trunc(d.to_index) : undefined;
        if (!id || to === undefined) { dropped += 1; return; }
        const ahead = store.ahead().filter((e) => e.id !== id);
        op = to <= 0 || ahead.length === 0 ? { op_id, by: 'desk', op: 'move', at: 'next', id, reason, base_version: 0 } : { op_id, by: 'desk', op: 'move', at: 'after', after: ahead[Math.min(to, ahead.length) - 1].id, id, reason, base_version: 0 };
        break;
      }
      case 'skip': {
        const air = store.onAir();
        if (!air || (id !== undefined && id !== air.id)) { dropped += 1; return; }
        op = { op_id, by: 'desk', op: 'skip', id: air.id, ended: { outcome: 'skipped', at: '', last_block: 0 }, reason, base_version: 0 };
        break;
      }
      case 'previous': case 'clear': {
        const want = typeof d.base_version === 'number' ? d.base_version : base;
        if (want !== undefined && want !== store.sheet.version) { dropped += 1; return; }
        op = { op_id, by: 'desk', op: d.op, base_version: store.sheet.version, reason };
        break;
      }
      default: dropped += 1; return;
    }
    const r = store.applyFromDesk(op);
    if (r.applied) applied += 1; else dropped += 1;
  });
  return { applied, dropped };
}
export const wouldApply = (rt: RadioRuntime, op: Op): boolean => reduce(rt.store.sheet, op).result.applied;
