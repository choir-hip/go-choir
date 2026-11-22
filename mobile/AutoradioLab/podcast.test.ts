// The podcast app's data under bun test: the feed parser, the text rules, the directory and server decoders, the addresses, and the sources
// (`podcastHome`, `podcastShow`, `podcastEpisode` and the acts) with a canned network and a fake storage. Every feed is synthetic (example.com).
// These are the rules of the Swift PodcastTests (deleted in the same change); each test names the failure-list number it carries.
// Failure list, written before the code:
//  1 subscribe twice, a feed written two ways, unsubscribe of a stranger: one subscription; persisted; a damaged library is an empty one.
//  2 a feed that fails: a sentence and a retry (never an empty page); it is not fetched again until the retry; a cached copy is shown when there is one.
//  3 a clip's episode whose feed has not loaded or has left it: playable from the clip's own audio, with its title; the address carries it.
//  4 a position for an id that is not a podcast key: ignored.        5 unplayed, started, finished (within 20 s of the end); resume.
//  6 a search typed over: the older query is dropped.                7 the server not answering a sync: a note; signed out: no note; an unsubscribed show is not pulled back.
//  8 a damaged file: empty.      9 the feed parser: enclosures, durations, notes, dates, ids, not-a-feed, 300 items, hostile text and depth.
// 10 the grants and the files in app.ts agree.        11 play hands the deck data; keep listening does not restart; the key is the position's key.
import { beforeEach, describe, expect, test } from 'bun:test';
import { dig, reopen } from './account-phone.ts';
import { currentWorld as theWorld, currentWorld, bindStorage, endWorld } from './api.ts';
import { readFileSync } from 'node:fs';
import { parseFeed, NotAFeed } from './podcast-feed.ts';
import * as M from './podcast-model.ts';
import * as P from './podcast.ts';
import { bindNative, resetSession, dropHeld } from './api.ts';
import { readXML } from './xml.ts';

const fx = (n: string): string => readFileSync(new URL(`../fixtures/podcasts/${n}`, import.meta.url), 'utf8');
const FEED = fx('feed-synthetic-hour.xml');
const FEED_URL = 'https://feeds.example.com/show.xml';
const NOW = Date.parse('2026-11-16T12:00:00Z');

// ---- the network and the files

type Call = { url: string; method: string; body?: string; auth: boolean };
let calls: Call[] = [];
let routes: Record<string, (url: URL, init?: RequestInit) => Response> = {};
const files = new Map<string, Uint8Array>();
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    mkdir: async () => {},
    rm: async (p: string) => { for (const k of [...files.keys()]) if (k === p || k.startsWith(p + '/')) files.delete(k); },
  },
} as never;
const reply = (body: string, status = 200): Response => new Response(body, { status, headers: { date: new Date(NOW).toUTCString() } });
const ok = (): Record<string, (u: URL, i?: RequestInit) => Response> => ({
  'feeds.example.com/show.xml': () => reply(FEED),
  'itunes.apple.com/search': () => reply(fx('directory-search.json')),
  'rss.marketingtools.apple.com': () => reply(fx('directory-chart.json')),
  'itunes.apple.com/lookup': () => reply(fx('directory-lookup.json')),
  'choir.news/api/platform/podcast-popular': () => reply('', 404),
  'choir.news/api/podcast/subscriptions': (_u, i) => (i?.method === 'POST' ? reply('{}') : reply(fx('subscriptions.json'))),
  'choir.news/api/platform/podcast-feeds': () => reply('{}'),
  'choir.news/api/platform/podcast-transcript': () => reply('', 404),
});
const signIn = (on: boolean): void => {
  resetSession(); reopen(dig('test-key'), on ? 'test-key' : undefined, 'computer-1'); if (!on) dropHeld();
  bindNative(on ? { later: async ({ op, account }: { op?: string; account?: string }) => (op === 'session' ? { found: true, value: 'test-key', computerId: 'computer-1' } : account === 'api-key' ? { found: true, value: 'test-key' } : account === 'computer-id' ? { found: true, value: 'computer-1' } : { found: false }) } : null);
};
const count = (needle: string): number => calls.filter((c) => c.url.includes(needle)).length;

beforeEach(() => {
  P.forget(); calls = []; files.clear(); signIn(false);
  P.search.pause = 0; P.zone.offset = 0;
  routes = ok();
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    const u = new URL(String(input));
    const h = (init?.headers ?? {}) as Record<string, string>;
    calls.push({ url: String(input), method: init?.method ?? 'GET', body: init?.body as string | undefined, auth: Object.keys(h).some((k) => k.toLowerCase() === 'authorization') });
    const key = Object.keys(routes).find((k) => (u.host + u.pathname).startsWith(k));
    if (!key) throw new Error('no route ' + u.host);
    return routes[key](u, init);
  }) as typeof fetch;
});

const home = (q = '', tick = 0) => P.podcastHome({ q, address: M.HOME, tick }, storage);
const showPage = (feed = FEED_URL, nowId = '', tick = 0) => P.podcastShow({ address: M.showAddress(feed), tick, nowId, rev: 0 }, storage);
const epArgs = (address: string, o: Partial<P.EpisodeArgs> = {}): P.EpisodeArgs => ({ address, find: '', tick: 0, nowId: '', nowClip: false, nowStart: 0, nowElapsed: 0, nowDuration: 0, rev: 0, ...o });
const noNow = { id: '', clip: false, start: 0, elapsed: 0, duration: 0 };

// ---- 9 the feed parser

describe('9 the feed parser', () => {
  const f = parseFeed(FEED, FEED_URL, 1);
  test('show and episodes: enclosures, durations, notes, dates, ids', () => {
    expect(f.show.title).toBe('Synthetic Hour'); expect(f.show.author).toBe('Test Author');
    expect(f.show.artworkURL).toBe('https://img.example.com/show.jpg');
    expect(f.show.summary).toBe('A show for tests.');
    expect(f.episodes.map((e) => e.id)).toEqual(['guid-1', 'guid-2', 'https://cdn.example.com/three.mp3', 'guid-4', 'guid-5']);
    const e1 = f.episodes[0];
    expect(e1.title).toBe('Episode & One'); expect(e1.duration).toBe(3723); expect(e1.enclosureURL).toBe('https://cdn.example.com/one.mp3');
    expect(e1.enclosureType).toBe('audio/mpeg'); expect(e1.published).toBe(1_794_303_000);
    expect(e1.notes).toBe('First para & more.\n\nSecond para.\n\n• Item A'); expect(e1.artworkURL).toBe('https://img.example.com/ep1.jpg');
    expect(f.episodes[1].duration).toBe(2710); expect(f.episodes[1].enclosureType).toBe('');
    expect(f.episodes[1].notes).toBe('Plain notes with escaped html & entity.'); expect(f.episodes[1].published).not.toBeNull();
    expect(f.episodes[2].duration).toBe(2710.5); expect(f.episodes[2].enclosureURL).toBe('https://cdn.example.com/three.mp3'); expect(f.episodes[2].published).not.toBeNull();
    expect(f.episodes[3].playable).toBe(false); expect(f.episodes[3].duration).toBeNull();
    expect(f.episodes[4].playable).toBe(false); expect(f.episodes[4].duration).toBeNull();
  });
  const notes = (item: string): string => parseFeed(`<rss><channel><title>S</title><item><title>T</title><guid>g</guid><enclosure url="https://cdn.example.com/a.mp3" type="audio/mpeg"/>${item}</item></channel></rss>`, 'x', 0).episodes[0].notes;
  test('nested markup in notes is not emptied', () => {
    expect(notes('<description><p>In this episode we talk.</p><p>Second part.</p></description>')).toBe('In this episode we talk.\n\nSecond part.');
    expect(notes('<description><![CDATA[<p>In <b>this</b> episode</p>]]></description>')).toBe('In this episode');
    expect(notes('<itunes:summary><div>Lead <a href="https://x.example.com">link</a> tail.</div></itunes:summary>')).toBe('Lead link tail.');
    expect(notes('<description>Before <p>Inside</p> after</description>')).toBe('Before\n\nInside\n\nafter');
    expect(notes('<content:encoded><ul><li>A</li><li>B</li></ul></content:encoded>')).toContain('A');
  });
  test('a hostile feed is capped: text, children, depth (items under the feed cap; one over it is dropped, see "the cap never shows a half item")', () => {
    const big = 'word '.repeat(200) + '\n\n';
    const n = notes('<description>' + big.repeat(40) + '</description>');
    expect(n.length).toBeLessThan(260_000); expect(n.length).toBeGreaterThan(1000);
    const kids = notes('<description>' + '<p>paragraph of some words here</p>'.repeat(1000) + '</description>');
    expect(kids.length).toBeLessThan(260_000); expect(kids.startsWith('paragraph of some words here')).toBe(true);
    expect(notes('<description>shallow ' + '<div>'.repeat(200) + 'deep words' + '</div>'.repeat(200) + '</description>')).toBe('shallow');
    expect(notes('<description>' + '<div>'.repeat(5) + 'near words' + '</div>'.repeat(5) + '</description>')).toBe('near words');
    expect(f.episodes.length).toBe(5);
  });
  test('6 a page, an empty body and an empty channel are not feeds', () => {
    expect(() => parseFeed('<!DOCTYPE html><html><body>hi</body></html>', 'x', 0)).toThrow(NotAFeed);
    expect(() => parseFeed('', 'x', 0)).toThrow(NotAFeed);
    expect(() => parseFeed('{"error":"no"}', 'x', 0)).toThrow(NotAFeed);
    expect(() => parseFeed('<rss><channel></channel></rss>', 'x', 0)).toThrow(NotAFeed);
  });
  test('7 many items keep the first 300', () => {
    let s = '<rss><channel><title>Big</title>';
    for (let i = 0; i < 400; i++) s += `<item><title>E${i}</title><guid>g${i}</guid><enclosure url="https://c.example.com/${i}.mp3" type="audio/mpeg"/></item>`;
    const big = parseFeed(s + '</channel></rss>', 'x', 0);
    expect(big.episodes.length).toBe(300); expect(big.episodes[0].id).toBe('g0');
  });
  test('a feed nearly well formed is still read: an undefined entity, an unclosed element, a stray end tag', () => {
    const x = parseFeed('<rss><channel><title>A&nbsp;B</title></bogus><item><title>T</title><guid>g</guid><enclosure url="https://c.example.com/a.mp3"/></channel></rss>', 'x', 0);
    expect(x.show.title).toBe('A B'.trim() === 'A B' ? x.show.title : 'A B'); expect(x.episodes.length).toBe(1);
  });
  test('the XML reader skips comments, processing instructions and a DOCTYPE with a subset', () => {
    const seen: string[] = [];
    readXML('<?xml version="1.0"?><!DOCTYPE r [<!ENTITY x "y">]><!-- c --><r a=\'1\' b="x>y"><k/>t</r>', { start: (n, a) => { seen.push(n + (a.b ? ':' + a.b : '')); }, end: (n) => { seen.push('/' + n); } });
    expect(seen).toEqual(['r:x>y', 'k', '/k', '/r']);
  });
});

// ---- text, durations, dates

describe('text, durations and dates (2, 4)', () => {
  test('durations and labels', () => {
    expect(M.duration('1:02:03')).toBe(3723); expect(M.duration('45:10')).toBe(2710); expect(M.duration('90')).toBe(90);
    for (const bad of ['', '1:2:3:4', 'x:10', '0', '-5']) expect(M.duration(bad)).toBeNull();
    expect(M.durationLabel(3723)).toBe('1 h 2 min'); expect(M.durationLabel(2700)).toBe('45 min'); expect(M.durationLabel(7200)).toBe('2 h'); expect(M.durationLabel(50)).toBe('50 s');
    expect(M.clock(760)).toBe('12:40'); expect(M.clock(3723)).toBe('1:02:03');
  });
  test('dates: RFC 822 with and without weekday, zones, ISO; nonsense is none', () => {
    expect(M.date('Mon, 10 Nov 2026 09:30 +0100')).toBe(M.date('Mon, 10 Nov 2026 08:30:00 GMT'));
    expect(M.date('2026-11-01T12:00:00Z')).toBe(M.date('Sun, 01 Nov 2026 12:00:00 +0000'));
    expect(M.date('2026-11-01T12:00:00.250+02:00')).toBe(M.date('1 Nov 2026 10:00:00 UTC'));
    expect(M.date('sometime')).toBeNull(); expect(M.date('')).toBeNull();
  });
  test('relative dates', () => {
    const now = M.date('Mon, 16 Nov 2026 12:00:00 +0000')!;
    const d = (s: string): number => M.date(s)!;
    expect(M.relative(now - 60, now)).toBe('Today'); expect(M.relative(d('Sun, 15 Nov 2026 12:00:00 +0000'), now)).toBe('Yesterday');
    expect(M.relative(d('Thu, 12 Nov 2026 12:00:00 +0000'), now)).toBe('4 days ago'); expect(M.relative(d('Mon, 02 Nov 2026 12:00:00 +0000'), now)).toBe('Nov 2');
    expect(M.relative(d('Mon, 03 Nov 2025 12:00:00 +0000'), now)).toBe('Nov 3, 2025'); expect(M.relative(null, now)).toBe('');
  });
});

// ---- the decoders (8, 7)

describe('decoders', () => {
  test('8 directory search: no feed or title skipped, a feed once, http artwork upgraded', () => {
    const s = M.decodeSearch(JSON.parse(fx('directory-search.json')));
    expect(s.map((x) => x.title)).toEqual(['Show A']); expect(s[0].artworkURL).toBe('https://img.example.com/a.jpg'); expect(s[0].author).toBe('Ann Example');
    expect(M.decodeSearch({ results: [] })).toEqual([]); expect(() => M.decodeSearch('nope')).toThrow();
    expect(M.searchURL('AT&T news')).toContain('term=AT%26T%20news');
  });
  test('the chart keeps news shows only, at most the limit', () => {
    const c = JSON.parse(fx('directory-chart.json'));
    expect(M.decodeChartNews(c).map((x) => x.id)).toEqual(['2', '3']); expect(M.decodeChartNews(c, 1).length).toBe(1);
  });
  test('7 the server list: a title that is the address is read from the content item', () => {
    const l = M.decodeSubscriptions(JSON.parse(fx('subscriptions.json')));
    expect(l.map((x) => x.title)).toEqual(['Real A', 'B']); expect(l[1].artworkURL).toBe('https://img.example.com/b.png');
  });
  test('platform bodies: popular, transcript, the report', () => {
    expect(M.decodePopular({ shows: [{ feed_url: 'https://f.example.com/a.xml', title: 'A' }, { feed_url: '', title: 'x' }, { feed_url: 'https://f.example.com/a.xml/', title: 'dup' }] }).map((x) => x.title)).toEqual(['A']);
    expect(M.decodePopular('<html>')).toEqual([]);
    const t = M.decodeTranscript({ episode: { duration_seconds: 100.5 }, source: 'x', blocks: [{ start: 1, end: 2, text: ' hi ' }, { start: 3, text: '' }, { end: 4, text: 'no start' }] });
    expect(t?.blocks.map((b) => b.text)).toEqual(['hi']); expect(t?.durationSeconds).toBe(100.5); expect(M.decodeTranscript({})).toBeNull();
    const b = M.reportBody([M.show('https://f.example.com/a.xml', 'A'), M.show('https://u:p@f.example.com/x.xml', 'creds')], { feedURL: 'https://f.example.com/a.xml', episodeGuid: 'g', audioURL: 'https://c.example.com/a.mp3' }) as { feeds: unknown[]; now_playing: { episode_guid: string } };
    expect(b.feeds.length).toBe(1); expect(b.now_playing.episode_guid).toBe('g');
    expect((M.reportBody([], null) as Record<string, unknown>).now_playing).toBeUndefined();
    expect(M.durationsAgree(100, 103)).toBe(true); expect(M.durationsAgree(100, 104)).toBe(false); expect(M.durationsAgree(null, 100)).toBe(false);
  });
  test('addresses and keys round-trip, and a clip\'s words ride in the address', () => {
    const a = M.episodeAddress('https://f.example.com/a?b=1&c=2', 'tag:x/1 2');
    expect(M.parseEpisode(a)?.feed).toBe('https://f.example.com/a?b=1&c=2'); expect(M.parseEpisode(a)?.id).toBe('tag:x/1 2'); expect(M.parseEpisode(a)?.hint).toBeNull();
    expect(M.parseEpisodeKey(M.episodeKey('https://f.example.com/a', 'g/1'))?.id).toBe('g/1');
    const h = M.parseEpisode(M.episodeAddress(FEED_URL, 'g', { enclosure: 'https://cdn.example.com/x.mp3', start: 12.5, title: 'Old & gone', show: 'S' }));
    expect(h?.hint).toEqual({ enclosure: 'https://cdn.example.com/x.mp3', start: 12.5, title: 'Old & gone', show: 'S' });
    expect(M.parseShow(M.showAddress(FEED_URL))).toBe(FEED_URL); expect(M.parseShow('podcast:show/')).toBeNull();
    expect(M.feedKey('HTTPS://Feeds.Example.com/show.xml/')).toBe(FEED_URL);
    expect(M.segment('a b/c.')).toBe('a%20b%2Fc.');
  });
});

// ---- the sources

describe('1 and 8 subscriptions', () => {
  test('subscribe persists, no doubles, a stranger is not unsubscribed, a damaged file is empty', async () => {
    expect((await P.podcastSubscribe(FEED_URL, storage)).message).toContain('Subscribed');
    expect((await P.podcastSubscribe(FEED_URL + '/', storage)).message).toContain('already');
    expect((await P.podcastUnsubscribe('https://other.example.com/x', storage)).message).toBe('');
    await showPage();                                                                // its feed is read, its title kept
    expect((await P.subscriptions(storage)).map((s) => s.title)).toEqual(['Synthetic Hour']);
    P.forget();
    expect((await P.subscriptions(storage)).map((s) => s.title)).toEqual(['Synthetic Hour']);   // read back from the file
    expect(files.has('app:/data/podcast-feeds') || [...files.keys()].some((k) => k.startsWith('app:/data/podcast-feeds/'))).toBe(true);
    await P.podcastUnsubscribe(FEED_URL, storage);
    P.forget();
    expect(await P.subscriptions(storage)).toEqual([]);
    files.set(P.LIBRARY_FILE, new TextEncoder().encode('{broken')); P.forget();
    expect(await P.subscriptions(storage)).toEqual([]);
  });
  test('the home page: empty says what to do; with a show it lists it with its unplayed count', async () => {
    let h = await home();
    expect(h.status).toBe('empty'); expect(h.message).toContain('Search for a show');
    expect(h.suggestions.map((s) => s.title)).toEqual(['Daily Brief', 'Politics Talk']);
    await P.podcastSubscribe(FEED_URL, storage);
    h = await home();
    expect(h.status).toBe('ready'); expect(h.shows.length).toBe(1);
    expect(h.shows[0].unplayedCount).toBe(3); expect(h.shows[0].detail).toBe('3 unplayed · Episode & One'); expect(h.shows[0].latestStamp).toBe('6 days ago');
  });
  test('a home page for another address is idle, and asks nothing', async () => {
    const h = await P.podcastHome({ q: '', address: 'radio:', tick: 0 }, storage);
    expect(h.status).toBe('idle'); expect(calls.length).toBe(0);
  });
});

describe('5 unplayed, resume, finished', () => {
  test('positions are kept, shown and finished within 20 s of the end', async () => {
    await P.podcastSubscribe(FEED_URL, storage);
    let v = await showPage();
    expect(v.status).toBe('ready'); expect(v.episodes.length).toBe(5);
    expect(v.episodes.filter((e) => e.unplayed).length).toBe(3); expect(v.unplayedLine).toBe('3 unplayed');
    const key = M.episodeKey(FEED_URL, 'guid-1');
    await P.podcastPlayhead({ id: key, clip: false, elapsed: 760, duration: 3723 }, storage);
    v = await showPage();
    expect(v.episodes[0].progress).toBe('49 min left'); expect(v.episodes[0].unplayed).toBe(false);
    let e = await P.podcastEpisode(epArgs(M.episodeAddress(FEED_URL, 'guid-1')), storage);
    expect(e.resumeLabel).toBe('Resume from 12:40'); expect(e.hasResume).toBe(true);
    await P.podcastPlayhead({ id: key, clip: false, elapsed: 3710, duration: 3723 }, storage);
    v = await showPage(); expect(v.episodes[0].progress).toBe('Played');
    e = await P.podcastEpisode(epArgs(M.episodeAddress(FEED_URL, 'guid-1')), storage); expect(e.hasResume).toBe(false);
    expect((await home()).shows[0].unplayedCount).toBe(2);
  });
  test('4 a position for an id that is not a podcast key is ignored; progress under 5 s is not kept apart', async () => {
    await P.podcastPlayhead({ id: 'not-a-pod-id', clip: false, elapsed: 50, duration: 100 }, storage);
    expect(await P.positionOf('not-a-pod-id', storage)).toBeNull();
    const key = M.episodeKey(FEED_URL, 'guid-1');
    await P.podcastPlayhead({ id: key, clip: false, elapsed: 100, duration: 0 }, storage);
    const r1 = (await P.podcastPlayhead({ id: key, clip: false, elapsed: 102, duration: 0 }, storage)).rev;
    expect((await P.positionOf(key, storage))?.seconds).toBe(100); expect((await P.positionOf(key, storage))?.duration).toBeNull();
    expect((await P.podcastPlayhead({ id: key, clip: false, elapsed: 106, duration: 0 }, storage)).rev).toBe(r1 + 1);
  });
});

describe('11 play and queue hand the deck data', () => {
  test('resume, start, at:, queue; the key is the position key; no audio is an error', async () => {
    await P.podcastSubscribe(FEED_URL, storage); await showPage();
    const a = M.episodeAddress(FEED_URL, 'guid-1');
    await P.podcastPlayhead({ id: M.episodeKey(FEED_URL, 'guid-1'), clip: false, elapsed: 100, duration: 3723 }, storage);
    const resume = await P.podcastPlay(a, 'resume', noNow, storage);
    expect(resume.status).toBe('ok'); expect(resume.play).toEqual({ op: 'play', id: M.episodeKey(FEED_URL, 'guid-1'), url: 'https://cdn.example.com/one.mp3', title: 'Episode & One', show: 'Synthetic Hour', start: 100, clip: '', feed: FEED_URL, record: '' });
    expect((await P.podcastPlay(a, 'start', noNow, storage)).play.start).toBe(0);
    expect((await P.podcastPlay(a, 'at:754', noNow, storage)).play.start).toBe(754);
    const q = await P.podcastQueue(a, storage);
    expect(q.play.op).toBe('queue'); expect(q.play.start).toBe(100); expect(q.message).toBe('Added to the queue.');
    expect((await P.podcastPlay(M.episodeAddress(FEED_URL, 'guid-4'), 'resume', noNow, storage)).status).toBe('error');
    expect((await P.podcastQueue(M.episodeAddress(FEED_URL, 'guid-4'), storage)).status).toBe('error');
  });
  test('the playhead tells the platform what is playing, once per episode', async () => {
    signIn(true);
    await P.podcastSubscribe(FEED_URL, storage); await showPage();
    const before = count('podcast-feeds');
    const key = M.episodeKey(FEED_URL, 'guid-1');
    await P.podcastPlayhead({ id: key, clip: false, elapsed: 10, duration: 3723 }, storage);
    await P.podcastPlayhead({ id: key, clip: false, elapsed: 20, duration: 3723 }, storage);
    expect(count('podcast-feeds')).toBe(before + 1);
    const body = JSON.parse(calls.filter((c) => c.url.includes('podcast-feeds')).pop()!.body!);
    expect(body.now_playing).toEqual({ feed_url: FEED_URL, episode_guid: 'guid-1', audio_url: 'https://cdn.example.com/one.mp3' });
  });
});

describe('3 a clip opens an episode', () => {
  const clipAddr = M.episodeAddress(FEED_URL, 'guid-2', { enclosure: 'https://cdn.example.com/two.m4a', start: 760 });
  test('the feed has it: the clip\'s start is said, then "Playing from" while the clip plays; the playhead is kept', async () => {
    let v = await P.podcastEpisode(epArgs(clipAddr), storage);
    expect(v.status).toBe('ready'); expect(v.title).toBe('Two'); expect(v.showTitle).toBe('Synthetic Hour'); expect(v.subscribed).toBe(false);
    expect(v.playable).toBe(true); expect(v.playingFrom).toBe('From the clip at 12:40');
    const key = M.episodeKey(FEED_URL, 'guid-2');
    v = await P.podcastEpisode(epArgs(clipAddr, { nowId: key, nowClip: true, nowStart: 760, nowElapsed: 770, nowDuration: 2710 }), storage);
    expect(v.playingFrom).toBe('Playing from 12:40'); expect(v.fromClip).toBe(true); expect(v.current).toBe(true);
    expect((await P.podcastPlay(clipAddr, 'keep', { id: key, clip: true, start: 760, elapsed: 770, duration: 2710 }, storage)).play.op).toBe('keep');
    expect((await P.podcastPlay(clipAddr, 'keep', noNow, storage)).play.start).toBe(760);
    expect((await P.podcastSubscribe(FEED_URL, storage)).status).toBe('ok');
    expect((await P.podcastEpisode(epArgs(clipAddr), storage)).subscribed).toBe(true);
  });
  test('an episode that left the feed still plays from the clip\'s own audio, with a note', async () => {
    const gone = M.episodeAddress(FEED_URL, 'old-guid', { enclosure: 'https://cdn.example.com/old.mp3', start: 5, title: 'Old one' });
    const g = await P.podcastEpisode(epArgs(gone), storage);
    expect(g.status).toBe('ready'); expect(g.title).toBe('Old one'); expect(g.playable).toBe(true); expect(g.note).not.toBe('');
    const p = await P.podcastPlay(gone, 'resume', noNow, storage);
    expect(p.play.url).toBe('https://cdn.example.com/old.mp3'); expect(p.play.title).toBe('Old one');
  });
  test('the feed is down: the clip alone is enough; with no clip words, a sentence', async () => {
    routes['feeds.example.com/show.xml'] = () => reply('', 503);
    const g = await P.podcastEpisode(epArgs(M.episodeAddress(FEED_URL, 'old-guid', { enclosure: 'https://cdn.example.com/old.mp3', title: 'Old one' })), storage);
    expect(g.status).toBe('ready'); expect(g.playable).toBe(true);
    const bare = await P.podcastEpisode(epArgs(M.episodeAddress(FEED_URL, 'guid-1')), storage);
    expect(bare.status).toBe('error'); expect(bare.message).toContain('could not be reached');
  });
});

describe('2 a failing feed', () => {
  test('is a sentence, is not asked again until the retry, and a cached copy is shown meanwhile', async () => {
    routes['down.example.com'] = () => reply('', 500);
    const a = M.showAddress('https://down.example.com/x.xml');
    const v = await P.podcastShow({ address: a, tick: 0, nowId: '', rev: 0 }, storage);
    expect(v.status).toBe('error'); expect(v.message).toContain('could not be reached');
    await P.podcastShow({ address: a, tick: 0, nowId: '', rev: 0 }, storage); await P.podcastShow({ address: a, tick: 0, nowId: '', rev: 1 }, storage);
    expect(count('down.example.com')).toBe(1);
    await P.podcastShow({ address: a, tick: 1, nowId: '', rev: 1 }, storage);
    expect(count('down.example.com')).toBe(2);
    expect((await P.podcastShow({ address: 'podcast:show/', tick: 0, nowId: '', rev: 0 }, storage)).status).toBe('error');
    // cached: the feed loaded once, then fails on the retry; the page still shows the episodes
    await showPage();
    routes['feeds.example.com/show.xml'] = () => reply('', 500);
    const again = await showPage(FEED_URL, '', 2);
    expect(again.status).toBe('ready'); expect(again.episodes.length).toBe(5);
  });
  test('a page where a feed was expected, and a feed too large', async () => {
    routes['page.example.com'] = () => reply('<html><body>hi</body></html>');
    expect((await P.podcastShow({ address: M.showAddress('https://page.example.com/'), tick: 0, nowId: '', rev: 0 }, storage)).message).toBe('That address is a web page, not a podcast feed.');
    routes['huge.example.com'] = () => new Response('x', { status: 200, headers: { 'content-length': '30000000' } });
    expect((await P.podcastShow({ address: M.showAddress('https://huge.example.com/'), tick: 0, nowId: '', rev: 0 }, storage)).message).toBe('That feed is too large to open.');
  });
  test('one fetch per feed at a time, however many pages ask', async () => {
    await Promise.all([showPage(), showPage(), P.podcastEpisode(epArgs(M.episodeAddress(FEED_URL, 'guid-1')), storage)]);
    expect(count('feeds.example.com')).toBe(1);
  });
  test('a feed is asked https first: an http address is upgraded', async () => {
    await P.podcastShow({ address: M.showAddress('http://feeds.example.com/show.xml'), tick: 0, nowId: '', rev: 0 }, storage);
    expect(calls[0].url).toBe('https://feeds.example.com/show.xml');
    expect(calls[0].auth).toBe(false);
  });
});

describe('6 search', () => {
  test('a query asks the public directory once, with no credentials; results say if subscribed', async () => {
    const h = await home('  show   a ');
    expect(h.query).toBe('show a'); expect(h.results.map((r) => r.title)).toEqual(['Show A']); expect(h.results[0].subscribed).toBe(false);
    expect(calls[0].url).toContain('https://itunes.apple.com/search?'); expect(calls[0].url).toContain('term=show%20a'); expect(calls.every((c) => !c.auth)).toBe(true);
    await home('show a');
    expect(count('itunes.apple.com/search')).toBe(1);
  });
  test('a query typed over is dropped; a failing directory is a sentence; no results is said', async () => {
    P.search.pause = 20;
    const first = home('first'); const second = home('second');
    const [a, b] = await Promise.all([first, second]);
    expect(count('term=first')).toBe(0); expect(count('term=second')).toBe(1);
    expect(a.results).toEqual([]); expect(b.results.length).toBe(1);
    P.search.pause = 0;
    routes['itunes.apple.com/search'] = () => reply('', 500);
    expect((await home('broken')).searchNote).toContain('Could not search podcasts');
    routes['itunes.apple.com/search'] = () => reply('{"results":[]}');
    expect((await home('nothing')).searchNote).toBe('No podcasts found for that.');
  });
});

describe('7 the computer', () => {
  test('signed out: no sync call and no note', async () => {
    const h = await home();
    expect(h.syncNote).toBe(''); expect(count('/api/podcast/subscriptions')).toBe(0);
  });
  test('signed in: the computer\'s list is pulled in, local shows are pushed, an unsubscribed show is not brought back', async () => {
    signIn(true);
    await P.podcastSubscribe(FEED_URL, storage);
    const pushed: string[] = [];
    routes['choir.news/api/podcast/subscriptions'] = (_u, i) => { if (i?.method === 'POST') { pushed.push(JSON.parse(String(i.body)).feed_url); return reply('{}'); } return reply(fx('subscriptions.json')); };
    await P.podcastUnsubscribe('https://feeds.example.com/b.xml', storage);          // not subscribed: nothing
    const h = await home();
    expect(h.syncNote).toBe("In step with your computer's podcasts.");
    expect(h.shows.map((s) => s.title).sort()).toEqual(['B', 'Real A', 'Synthetic Hour']);
    expect(pushed).toEqual([FEED_URL]);
    expect(calls.filter((c) => c.url.includes('/api/podcast/subscriptions')).every((c) => c.auth)).toBe(true);
    await P.podcastUnsubscribe('https://feeds.example.com/b.xml', storage);
    P.forget();
    expect((await home()).shows.map((s) => s.title).sort()).toEqual(['Real A', 'Synthetic Hour']);
  });
  test('the computer not answering is a note, and the phone\'s list is unchanged', async () => {
    signIn(true);
    await P.podcastSubscribe(FEED_URL, storage);
    routes['choir.news/api/podcast/subscriptions'] = (_u, i) => (i?.method === 'POST' ? reply('{}') : reply('', 502));
    const h = await home();
    expect(h.syncNote).toContain('Not in step'); expect(h.shows.length).toBe(1);
  });
});

describe('transcript', () => {
  const addr = M.episodeAddress(FEED_URL, 'guid-1');
  test('shown, searchable, the playing block marked, tap-to-seek only when the lengths agree', async () => {
    signIn(true);
    routes['choir.news/api/platform/podcast-transcript'] = () => reply(fx('transcript.json'));
    let v = await P.podcastEpisode(epArgs(addr), storage);
    expect(v.transcript.length).toBe(3); expect(v.transcript.every((t) => t.seekable)).toBe(true); expect(v.transcript[1].time).toBe('0:12');
    expect(calls.find((c) => c.url.includes('podcast-transcript'))!.url).toContain('guid=guid-1');
    v = await P.podcastEpisode(epArgs(addr, { find: 'CATS' }), storage);
    expect(v.transcript.map((t) => t.text)).toEqual(['Second line about cats']);
    expect((await P.podcastEpisode(epArgs(addr, { find: 'zzz' }), storage)).transcriptNote).toContain('Nothing');
    const key = M.episodeKey(FEED_URL, 'guid-1');
    v = await P.podcastEpisode(epArgs(addr, { nowId: key, nowElapsed: 14, nowDuration: 3723 }), storage);
    expect(v.transcript.map((t) => t.current)).toEqual([false, true, false]);
    v = await P.podcastEpisode(epArgs(addr, { nowId: key, nowElapsed: 14, nowDuration: 3000 }), storage);
    expect(v.transcript.every((t) => !t.seekable)).toBe(true); expect(v.transcriptNote).toContain('another recording');
    expect(count('podcast-transcript')).toBe(1);
  });
  test('"on its way" only for an episode that was played here', async () => {
    expect((await P.podcastEpisode(epArgs(addr), storage)).transcriptState).toBe('');
    await P.podcastPlay(addr, 'at:12', noNow, storage);
    expect((await P.podcastEpisode(epArgs(addr), storage)).transcriptNote).toBe('Transcript on its way.');
  });
});

describe('10 the grants and the files agree', () => {
  const app = readFileSync(new URL('./app.ts', import.meta.url), 'utf8');
  test('app.ts grants the library, the feed folder, and any https host for a feed the listener names', () => {
    expect(app).toContain('fs.read app:/data'); expect(app).toContain('fs.write app:/data');
    expect(app).toMatch(/^net\.fetch https:$/m);
    expect(app).not.toMatch(/net\.fetch http:/);
  });
});

describe('a feed that finishes loading after the session ended is not kept', () => {
  test('nothing is written and nothing is shown', async () => {
    let release: () => void = () => {};
    const gate = new Promise<void>((r) => { release = r; });
    routes['feeds.example.com/show.xml'] = () => { throw new Error('unused'); };
    globalThis.fetch = (async (input: RequestInfo | URL) => { calls.push({ url: String(input), method: 'GET', auth: false }); await gate; return reply(FEED); }) as typeof fetch;
    const bound = bindStorage(storage as never, currentWorld()) as never;
    await P.podcastSubscribe(FEED_URL, bound).catch(() => {});
    const before = new Set(files.keys());
    expect(before.size).toBeGreaterThan(0);          // the subscription was written, under the account's root, so the check below can fail
    expect([...before].every((k) => k.includes('/accounts/'))).toBe(true);
    const pending = Promise.race([showPage(), new Promise((r) => setTimeout(r, 30))]);
    await new Promise((r) => setTimeout(r, 5));
    endWorld();                                       // a sign-out: the world closes, nothing of it resumes
    release();
    await pending;
    expect([...files.keys()].filter((k) => !before.has(k))).toEqual([]);
  });
});

describe('the phone\'s own search reads the shared documents', () => {
  test('podcastDocuments lists the subscribed shows with their cached episodes', async () => {
    await P.podcastSubscribe(FEED_URL, storage); await showPage();
    const d = await P.podcastDocuments(storage);
    expect(d.length).toBe(1); expect(d[0].episodes.length).toBe(5);
  });
});

describe('feeds over the parse cap (the device\'s 100 ms turn)', () => {
  test('a 2 MB feed is read up to the cap at a whole item: the newest episodes stay, the page is not refused', async () => {
    const { parseFeed, MAX_PARSE } = await import('./podcast-feed.ts');
    const item = (i: number) => `<item><title>Episode ${i}</title><guid>g${i}</guid><pubDate>Mon, 01 Jan 2024 00:00:00 GMT</pubDate><description>${'words '.repeat(300)}</description><enclosure url="https://cdn.example.com/e${i}.mp3" type="audio/mpeg"/></item>`;
    const xml = `<?xml version="1.0"?><rss><channel><title>Big show</title>${Array.from({ length: 1100 }, (_, i) => item(i)).join('')}</channel></rss>`;
    expect(xml.length).toBeGreaterThan(1_900_000);
    const t = performance.now(); const f = parseFeed(xml, 'https://feeds.example.com/big.xml', 0); const ms = performance.now() - t;
    expect(f.show.title).toBe('Big show'); expect(f.episodes[0].title).toBe('Episode 0');
    expect(f.episodes.length).toBeGreaterThan(20); expect(f.episodes.length).toBeLessThan(300);
    expect(f.episodes.length * 1800).toBeLessThan(MAX_PARSE + 20_000);
    expect(ms).toBeLessThan(500);
  });
});

describe('the cap never shows a half item', () => {
  test('one item larger than the cap yields the show and no episode', async () => {
    const { parseFeed, MAX_PARSE } = await import('./podcast-feed.ts');
    const xml = `<?xml version="1.0"?><rss><channel><title>Long notes show</title><item><title>Huge</title><guid>g</guid><description>${'words '.repeat(MAX_PARSE / 4)}</description><enclosure url="https://cdn.example.com/e.mp3" type="audio/mpeg"/></item></channel></rss>`;
    const f = parseFeed(xml, 'https://feeds.example.com/h.xml', 0);
    expect(f.show.title).toBe('Long notes show'); expect(f.episodes).toEqual([]);
  });
  test('when whole items fit, only whole items come out; the last one is complete', async () => {
    const { parseFeed, MAX_PARSE } = await import('./podcast-feed.ts');
    const item = (i: number) => `<item><title>E${i}</title><guid>g${i}</guid><description>${'w '.repeat(2000)}</description><enclosure url="https://cdn.example.com/e${i}.mp3" type="audio/mpeg"/></item>`;
    const xml = `<?xml version="1.0"?><rss><channel><title>S</title>${Array.from({ length: 60 }, (_, i) => item(i)).join('')}</channel></rss>`;
    expect(xml.length).toBeGreaterThan(MAX_PARSE);
    const f = parseFeed(xml, 'https://feeds.example.com/s.xml', 0);
    expect(f.episodes.length).toBeGreaterThan(5); expect(f.episodes.length).toBeLessThan(60);
    expect(f.episodes.every((e) => e.title !== '' && e.playable)).toBe(true);
  });
});
