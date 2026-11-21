// Activity (activity:): what the computer and the desk are doing now and recently, as shared code. It replaces the Swift ActivityFeed.swift (the
// changelog parse, the rows and their groups, the view's sections). Sources of the page:
//   the owner's changelog   GET /api/changelog?minor=1&limit=50 (signed in): {events:[{inbox_seq, level, changelog:{kind, occurred_at, summary, actor_label,
//                           subject:{doc_id, route_path}, tape_ref:{doc_id, revision_id}}}], head, seen_seq, unseen_major}
//   the radio's turns and the "now" lines  what the radio's engine holds (the voice runtime): passed in as arguments by the page, never fetched here
// The "seen" cursor (PUT /api/changelog/seen) is not moved by this page.
//
// Ways it can fail (tests in activity.test.ts; V1 to V6 are the Swift list):
//  V1 a summary is never markup or an instruction: rows carry plain text only, cut to 240 characters
//  V2 an event kind new to the phone is shown under the kind's words with underscores removed
//  V3 an event with no document or route is not tappable: its target is empty
//  V4 a bad date shows nothing about when, never "1970"
//  V5 the route missing (404) or the key refused: one plain line, and the radio's turns still show
//  V6 a route that is an absolute URL or has a scheme is never opened: only a path starting with "/" without ":" becomes `pub:<route>`
// Time arrives as an argument (epoch milliseconds); a day-old row's clock is shown in UTC (the shared code has no zone).

import { authedJSON, HTTPError, SignedOutError, currentWorld } from './api.ts';
import { parseDate } from './paper-model.ts';

export type Item = { id: string; kind: string; label: string; detail: string; group: string; ago: string; target: string; title: string; at: number; unseen: boolean };
export type View = { status: string; message: string; live: Item[]; computer: Item[]; desk: Item[]; paper: Item[]; radio: Item[]; unseen: number; blank: boolean };

const LABELS: Record<string, string> = {
  parent_used: 'Your work was used', oped_verdict: 'Op-ed verdict', update_started: 'Update started', update_progress: 'Update progress', update_finished: 'Update finished',
  update_interrupted: 'Update interrupted', revision_committed: 'Revision saved', wake_decision: 'Desk decision', oped_arrival: 'Op-ed arrived', gate_unavailable: 'Gate unavailable',
};
export const group = (kind: string): string => (kind.startsWith('update_') || kind === 'gate_unavailable' ? 'computer' : kind === 'parent_used' || kind.startsWith('oped_') ? 'paper' : 'desk');
export const label = (kind: string): string => LABELS[kind] ?? kind.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
export function target(docID: string, route: string): string {
  if (docID !== '') return `texture:${docID}`;
  if (route.startsWith('/') && !route.includes(':') && route.length > 1) return 'pub:' + route.slice(1);
  return '';
}
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
export function when(atMs: number, now: number): string {
  if (!(atMs > 86400000)) return '';
  const age = (now - atMs) / 1000;
  if (age >= 0 && age < 3600) { const m = Math.max(1, Math.floor(age / 60)); return m === 1 ? 'a minute ago' : `${m} minutes ago`; }
  if (age >= 0 && age < 86400) { const h = Math.floor(age / 3600); return h === 1 ? 'an hour ago' : `${h} hours ago`; }
  const d = new Date(atMs); const p = (n: number): string => String(n).padStart(2, '0');
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}, ${p(d.getUTCHours())}:${p(d.getUTCMinutes())}`;
}

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const s = (o: Record<string, unknown>, k: string): string => (typeof o[k] === 'string' ? (o[k] as string) : '');

/** The changelog route's answer as rows, newest first. Throws when the body is not an object. */
export function items(body: unknown, now: number): Item[] {
  if (!isObj(body)) throw new Error('changelog: not an object');
  const seen = typeof body.seen_seq === 'number' ? body.seen_seq : 0;
  const out: Item[] = [];
  for (const e of Array.isArray(body.events) ? body.events : []) {
    if (!isObj(e) || !isObj(e.changelog)) continue;
    const c = e.changelog, seq = typeof e.inbox_seq === 'number' ? e.inbox_seq : 0;
    const kind = s(c, 'kind') || 'event';
    const subject = isObj(c.subject) ? c.subject : {}, tape = isObj(c.tape_ref) ? c.tape_ref : {};
    const at = parseDate(s(c, 'occurred_at')) ?? 0;
    out.push({
      id: s(c, 'event_id') || `seq-${seq}`, kind, label: label(kind), group: group(kind), detail: [...s(c, 'summary')].slice(0, 240).join(''), title: s(c, 'actor_label'), at, ago: when(at, now),
      target: target(s(subject, 'doc_id') || s(tape, 'doc_id'), s(subject, 'route_path')), unseen: seq > seen && (s(e, 'level') || 'major') === 'major',
    });
  }
  return out.sort((a, b) => b.at - a.at);
}

export const plain = (e: unknown): string =>
  e instanceof SignedOutError ? 'Not signed in.' : e instanceof HTTPError && e.status === 401 ? "This phone's sign-in is no longer accepted."
    : e instanceof HTTPError && e.status === 404 ? 'This server has no activity feed yet.' : 'Could not load your computer\'s activity.';

export function view(events: Item[], status: string, message: string, radio: Item[], live: Item[]): View {
  const r = radio.slice(0, 12);
  return { status, message, live, radio: r, computer: events.filter((x) => x.group === 'computer'), desk: events.filter((x) => x.group === 'desk'), paper: events.filter((x) => x.group === 'paper'),
    unseen: events.filter((x) => x.unseen).length, blank: events.length === 0 && r.length === 0 && live.length === 0 && status !== 'loading' };
}

/** The page's source: the changelog with the radio's own rows as given. Never throws; a failure is `failed` and its one line. */
export async function activityInfo(a: { now: number; signedIn: boolean; radio: Item[]; live: Item[] }, get: (path: string, q: Record<string, string | number>) => Promise<unknown> = async (p, q) => (await authedJSON(p, { query: q, world: currentWorld() })).body): Promise<View> {
  if (!a.signedIn) return view([], 'idle', '', a.radio, a.live);
  try { return view(items(await get('/api/changelog', { limit: 50, minor: 1 }), a.now), 'ok', '', a.radio, a.live); }
  catch (e) { return view([], 'failed', plain(e), a.radio, a.live); }
}
