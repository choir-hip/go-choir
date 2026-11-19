// The podcast app's pure data: text, durations, dates, addresses, keys and the decoders of what the directory and the server answer.
// Moved from Swift (PodcastFeed.swift: PodcastText, PodcastKey, PodcastDirectory; PodcastService.swift: addresses, PodcastSyncCodec,
// PodcastPlatformCodec). Nothing here reads the clock or the network.

export type Show = { feedURL: string; title: string; author: string; artworkURL: string; summary: string };
export type Episode = {
  id: string; title: string; published: number | null; duration: number | null; enclosureURL: string; enclosureType: string;
  notes: string; artworkURL: string; link: string; playable: boolean;
};
export type Feed = { show: Show; episodes: Episode[]; fetchedAt: number };

export const show = (feedURL: string, title: string, o: Partial<Show> = {}): Show => ({ feedURL, title, author: '', artworkURL: '', summary: '', ...o });

// ---- text

const LINEBREAK = new RegExp('\\r\\n|[\\n\\r\\u0085\\u2028\\u2029]');

/** Notes as plain paragraphs: block tags become line breaks, list items a bullet, other tags go, entities are read. */
export function plain(html: string, limit = 8000): string {
  let s = html;
  const rules: [RegExp, string][] = [[/<br\s*\/?>/gi, '\n'], [/<\/(p|div|h[1-6]|ul|ol|blockquote)>/gi, '\n\n'], [/<\/li>/gi, '\n'], [/<li[^>]*>/gi, '• ']];
  for (const [re, rep] of rules) s = s.replace(re, rep);
  s = s.replace(/<[^>]+>/g, '');
  s = decode(s);
  const lines = s.split(LINEBREAK).map((l) => l.split(/[ \t ]+/).filter((x) => x !== '').join(' '));
  const out: string[] = [];
  let blank = false;
  for (const l of lines) {
    if (l === '') { if (!blank && out.length > 0) out.push(''); blank = true; } else { out.push(l); blank = false; }
  }
  while (out.length > 0 && out[out.length - 1] === '') out.pop();
  const joined = out.join('\n');
  const chars = joined.length > limit ? Array.from(joined) : null;
  return chars && chars.length > limit ? chars.slice(0, limit - 1).join('') + '…' : joined;
}

import { decodeEntities } from './xml.ts';
export const decode = decodeEntities;

/** "1:02:03", "45:10", "2710", "2710.5" are seconds; anything else is null. Zero is null (a feed's "0" means unknown). */
export function duration(raw: string): number | null {
  const t = raw.trim();
  if (t === '') return null;
  const parts = t.split(':');
  if (parts.length > 3) return null;
  let total = 0;
  for (const p of parts) {
    if (!/^\d+(\.\d+)?$/.test(p)) return null;
    total = total * 60 + parseFloat(p);
  }
  return Number.isFinite(total) && total > 0 ? total : null;
}

/** "45 min", "1 h 12 min", "50 s". */
export function durationLabel(seconds: number | null | undefined): string {
  if (seconds == null || !(seconds > 0)) return '';
  const m = Math.round(seconds / 60);
  if (seconds < 60) return `${Math.floor(seconds)} s`;
  if (m < 60) return `${m} min`;
  return m % 60 === 0 ? `${m / 60} h` : `${Math.floor(m / 60)} h ${m % 60} min`;
}

/** A playhead: "12:40" or "1:02:03". */
export function clock(seconds: number): string {
  const t = Math.max(0, Math.floor(seconds));
  const two = (n: number): string => (n < 10 ? '0' + n : String(n));
  return t >= 3600 ? `${Math.floor(t / 3600)}:${two(Math.floor((t % 3600) / 60))}:${two(t % 60)}` : `${Math.floor(t / 60)}:${two(t % 60)}`;
}

const MONTHS = ['jan', 'feb', 'mar', 'apr', 'may', 'jun', 'jul', 'aug', 'sep', 'oct', 'nov', 'dec'];
const MONTH_NAMES = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const ZONES: Record<string, number> = { z: 0, gmt: 0, ut: 0, utc: 0, est: -300, edt: -240, cst: -360, cdt: -300, mst: -420, mdt: -360, pst: -480, pdt: -420 };

function offsetMinutes(z: string | undefined): number | null {
  if (z === undefined || z === '') return 0;
  const lower = z.toLowerCase();
  if (lower in ZONES) return ZONES[lower];
  const m = /^([+-])(\d{2}):?(\d{2})$/.exec(z);
  if (!m) return null;
  return (m[1] === '-' ? -1 : 1) * (parseInt(m[2], 10) * 60 + parseInt(m[3], 10));
}

/** RFC 822 (the feed standard, with or without weekday, seconds or numeric zone) and ISO 8601. Seconds since 1970, or null. A date with no
 *  zone is read as UTC. */
export function date(raw: string): number | null {
  const t = raw.trim();
  if (t === '') return null;
  let m = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2})(?:\.(\d+))?)?\s*(Z|[+-]\d{2}:?\d{2})?)?$/i.exec(t);
  if (m) {
    const off = offsetMinutes(m[8]);
    if (off === null) return null;
    const ms = Date.UTC(+m[1], +m[2] - 1, +m[3], +(m[4] ?? 0), +(m[5] ?? 0), +(m[6] ?? 0));
    return Number.isFinite(ms) ? ms / 1000 - off * 60 : null;
  }
  m = /^(?:[A-Za-z]{3,9},?\s+)?(\d{1,2})\s+([A-Za-z]{3,9})\s+(\d{2,4})\s+(\d{1,2}):(\d{2})(?::(\d{2}))?\s*([A-Za-z]{1,4}|[+-]\d{2}:?\d{2})?$/.exec(t);
  if (!m) return null;
  const mon = MONTHS.indexOf(m[2].slice(0, 3).toLowerCase());
  const off = offsetMinutes(m[7]);
  if (mon < 0 || off === null) return null;
  let year = +m[3];
  if (m[3].length === 2) year += year < 70 ? 2000 : 1900;
  const ms = Date.UTC(year, mon, +m[1], +m[4], +m[5], +(m[6] ?? 0));
  return Number.isFinite(ms) ? ms / 1000 - off * 60 : null;
}

/** "Today", "Yesterday", "3 days ago", then "Nov 3", then "Nov 3, 2025" for another year. `tz` is the zone's offset from UTC in minutes
 *  (the device's, passed in so a test is not the machine's). */
export function relative(time: number | null, now: number, tz = 0): string {
  if (time == null) return '';
  const day = (s: number): number => Math.floor((s + tz * 60) / 86400);
  const days = day(now) - day(time);
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  if (days > 1 && days < 7) return `${days} days ago`;
  const d = new Date((time + tz * 60) * 1000);
  const n = new Date((now + tz * 60) * 1000);
  const base = `${MONTH_NAMES[d.getUTCMonth()]} ${d.getUTCDate()}`;
  return d.getUTCFullYear() === n.getUTCFullYear() ? base : `${base}, ${d.getUTCFullYear()}`;
}

export function https(s: string): string {
  const t = s.trim();
  return t.toLowerCase().startsWith('http://') ? 'https://' + t.slice(7) : t;
}

/** The address when it is an http or https URL with a host, else "". */
export function webURL(s: string): string {
  const t = s.trim();
  const m = /^(https?):\/\/([^\s/?#]+)/i.exec(t);
  return m && !/^[:@]/.test(m[2]) ? t : '';
}

// ---- keys and addresses

/** A path segment, percent-encoded as the Swift PathSegment was: everything but A-Z a-z 0-9 - . _ ~ (UTF-8 bytes). */
export function segment(s: string): string {
  if (s === '.') return '%2E';
  if (s === '..') return '%2E%2E';
  return encodeURIComponent(s).replace(/[!'()*]/g, (c) => '%' + c.charCodeAt(0).toString(16).toUpperCase());
}
function unsegment(s: string): string | null { try { return decodeURIComponent(s); } catch { return null; } }

/** A feed address as one identity: trimmed, scheme and host lower-cased, no fragment, no trailing slash. */
export function feedKey(s: string): string {
  let t = s.trim();
  const m = /^([A-Za-z][A-Za-z0-9+.-]*):\/\/([^/?#]*)(.*)$/.exec(t);
  if (m) {
    const noFragment = m[3].replace(/#.*$/, '');
    t = m[1].toLowerCase() + '://' + m[2].toLowerCase() + noFragment;
  }
  while (t.endsWith('/')) t = t.slice(0, -1);
  return t;
}

/** One episode as one string: the feed and the guid. The deck's item id for a podcast play, and the key of its resume position. */
export function episodeKey(feed: string, id: string): string { return 'pod:' + segment(feed) + '/' + segment(id); }
export function parseEpisodeKey(key: string): { feed: string; id: string } | null {
  if (!key.startsWith('pod:')) return null;
  const rest = key.slice(4);
  const slash = rest.indexOf('/');
  if (slash < 0) return null;
  const f = unsegment(rest.slice(0, slash)), i = unsegment(rest.slice(slash + 1));
  return f && i ? { feed: f, id: i } : null;
}

export const HOME = 'podcasts:';
export const showAddress = (feed: string): string => 'podcast:show/' + segment(feed);

/** What an episode page may carry from a clip: the audio the clip played, where it began, and the words it had. Part of the address, so the
 *  page opens the same way after a relaunch. */
export type Hint = { enclosure: string; start: number | null; title: string; show: string };

export function episodeAddress(feed: string, id: string, hint?: Partial<Hint>): string {
  let a = 'podcast:episode/' + segment(feed) + '/' + segment(id);
  const q: string[] = [];
  if (hint?.enclosure) q.push('audio=' + segment(hint.enclosure));
  if (hint?.start != null && hint.start > 0) q.push('at=' + hint.start);
  if (hint?.title) q.push('title=' + segment(hint.title));
  if (hint?.show) q.push('show=' + segment(hint.show));
  if (q.length > 0) a += '?' + q.join('&');
  return a;
}

export function parseShow(address: string): string | null {
  if (!address.startsWith('podcast:show/')) return null;
  const f = unsegment(address.slice(13));
  return f ? f : null;
}

export function parseEpisode(address: string): { feed: string; id: string; hint: Hint | null } | null {
  if (!address.startsWith('podcast:episode/')) return null;
  let rest = address.slice(16);
  let hint: Hint | null = null;
  const qm = rest.indexOf('?');
  if (qm >= 0) {
    const h: Hint = { enclosure: '', start: null, title: '', show: '' };
    for (const pair of rest.slice(qm + 1).split('&')) {
      const eq = pair.indexOf('=');
      if (eq < 0) continue;
      const k = pair.slice(0, eq), v = unsegment(pair.slice(eq + 1)) ?? '';
      if (k === 'audio') h.enclosure = webURL(v);
      else if (k === 'at') { const n = parseFloat(v); h.start = Number.isFinite(n) && n >= 0 ? n : null; }
      else if (k === 'title') h.title = v;
      else if (k === 'show') h.show = v;
    }
    hint = h;
    rest = rest.slice(0, qm);
  }
  const slash = rest.indexOf('/');
  if (slash < 0) return null;
  const f = unsegment(rest.slice(0, slash)), i = unsegment(rest.slice(slash + 1));
  return f && i ? { feed: f, id: i, hint } : null;
}

// ---- the public directory (Apple's, no key) and the computer's lists

const unknown = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const oneLine = (s: string, limit: number): string => plain(s, limit).replace(/\n/g, ' ');

export function searchURL(query: string, limit = 15): string {
  return 'https://itunes.apple.com/search?media=podcast&entity=podcast&term=' + encodeURIComponent(query) + '&limit=' + limit;
}
export const TOP_URL = 'https://rss.marketingtools.apple.com/api/v2/us/podcasts/top/50/podcasts.json';
export const lookupURL = (ids: string[]): string => 'https://itunes.apple.com/lookup?entity=podcast&id=' + ids.join(',');

/** Shows from a search or lookup answer, in order, without those that have no feed or title or repeat a feed. Throws on a body that is not one. */
export function decodeSearch(body: unknown): Show[] {
  if (!unknown(body)) throw new Error('not a JSON object');
  const list = body.results;
  if (list === undefined || list === null) return [];
  if (!Array.isArray(list)) throw new Error('results is not a list');
  const seen = new Set<string>();
  const out: Show[] = [];
  for (const r of list) {
    if (!unknown(r)) continue;
    const feed = webURL(str(r.feedUrl));
    const title = oneLine(str(r.collectionName) || str(r.trackName), 200);
    if (feed === '' || title === '' || seen.has(feedKey(feed))) continue;
    seen.add(feedKey(feed));
    const art = str(r.artworkUrl600) || str(r.artworkUrl100) || str(r.artworkUrl60);
    out.push(show(feed, title, { author: str(r.artistName), artworkURL: https(webURL(art)) }));
  }
  return out;
}

/** The chart's ids and names for shows whose genre is News, at most `limit`. The chart has no feed URLs: `lookupURL` adds them. */
export function decodeChartNews(body: unknown, limit = 8): { id: string; name: string }[] {
  if (!unknown(body) || !unknown(body.feed)) throw new Error('not a chart');
  const list = Array.isArray(body.feed.results) ? body.feed.results : [];
  const out: { id: string; name: string }[] = [];
  for (const r of list) {
    if (!unknown(r)) continue;
    const genres = (Array.isArray(r.genres) ? r.genres : []).map((g) => (unknown(g) ? str(g.name) : '')).filter((x) => x !== '');
    if (!genres.some((g) => g.toLowerCase().includes('news')) || typeof r.id !== 'string' || typeof r.name !== 'string') continue;
    out.push({ id: r.id, name: r.name });
    if (out.length === limit) break;
  }
  return out;
}

/** The server's `{subscriptions:[{feed_url,title,author,artwork_url,...}]}`. Throws on a body that is not one. */
export function decodeSubscriptions(body: unknown): Show[] {
  if (!unknown(body)) throw new Error('not a JSON object');
  const list = body.subscriptions;
  if (list === undefined || list === null) return [];
  if (!Array.isArray(list)) throw new Error('subscriptions is not a list');
  const out: Show[] = [];
  const seen = new Set<string>();
  for (const r of list) {
    if (!unknown(r)) continue;
    const feed = webURL(str(r.feed_url));
    if (feed === '' || seen.has(feedKey(feed))) continue;
    seen.add(feedKey(feed));
    let title = str(r.title);
    if (title === '' || title.toLowerCase().startsWith('http')) title = (unknown(r.content_item) ? str(r.content_item.title) : '') || title;
    out.push(show(feed, title === '' ? feed : title, { author: str(r.author), artworkURL: https(webURL(str(r.artwork_url))) }));
  }
  return out;
}

/** Shows other readers follow: `{shows:[...]}` (provisional; `feeds` is read too). Anything else is an empty list. */
export function decodePopular(body: unknown): Show[] {
  if (!unknown(body)) return [];
  const list = body.feeds ?? body.shows;
  if (!Array.isArray(list)) return [];
  const out: Show[] = [];
  const seen = new Set<string>();
  for (const r of list) {
    if (!unknown(r)) continue;
    const feed = webURL(str(r.feed_url));
    const title = str(r.title).trim();
    if (feed === '' || title === '' || seen.has(feedKey(feed))) continue;
    seen.add(feedKey(feed));
    out.push(show(feed, title, { author: str(r.author), artworkURL: https(webURL(str(r.artwork_url))) }));
  }
  return out;
}

export type Transcript = { durationSeconds: number | null; source: string; blocks: { start: number; end: number; text: string }[] };

/** `{episode:{duration_seconds},source,blocks:[{start,end,text}]}`; blocks without text or a start are dropped; no block is no transcript. */
export function decodeTranscript(body: unknown): Transcript | null {
  if (!unknown(body) || !Array.isArray(body.blocks)) return null;
  const blocks: Transcript['blocks'] = [];
  for (const b of body.blocks) {
    if (!unknown(b)) continue;
    const text = str(b.text).trim();
    if (text === '' || typeof b.start !== 'number' || !(b.start >= 0)) continue;
    blocks.push({ start: b.start, end: typeof b.end === 'number' ? b.end : b.start, text });
  }
  if (blocks.length === 0) return null;
  const ep = unknown(body.episode) ? body.episode.duration_seconds : undefined;
  return { durationSeconds: typeof ep === 'number' ? ep : null, source: str(body.source), blocks };
}

/** The wire's bounds: at most 50 feeds, http(s) URLs with no credentials of at most 2048 bytes, titles of at most 200 bytes. */
export function reportBody(feeds: Show[], nowPlaying: { feedURL: string; episodeGuid: string; audioURL: string } | null): unknown {
  const bytes = (s: string): number => new TextEncoder().encode(s).length;
  const cut = (s: string, n: number): string => { let t = s; while (bytes(t) > n) t = t.slice(0, -1); return t; };
  const ok = (u: string): boolean => bytes(u) <= 2048 && webURL(u) !== '' && !/^https?:\/\/[^/]*@/i.test(u);
  const o: Record<string, unknown> = { feeds: feeds.filter((s) => ok(s.feedURL)).slice(0, 50).map((s) => ({ feed_url: s.feedURL, title: cut(s.title, 200) })) };
  if (nowPlaying && webURL(nowPlaying.feedURL) !== '' && webURL(nowPlaying.audioURL) !== '') {
    o.now_playing = { feed_url: nowPlaying.feedURL, episode_guid: nowPlaying.episodeGuid, audio_url: nowPlaying.audioURL };
  }
  return o;
}

/** A transcript's times are good for this file when its length is within 3 s of the transcribed one's. */
export function durationsAgree(file: number | null | undefined, transcribed: number | null | undefined): boolean {
  return file != null && transcribed != null && Math.abs(file - transcribed) <= 3;
}
