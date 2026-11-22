// The clips pages under bun test: a show's clip rows and what a row does. These carry the rules of the Swift JoinsTests (J1, J3, J6) and the
// shared half of DJClipsTests that belong to the pages; the native half ("clips play from fingerprinted audio") stays in ClipLocatorTests, where the
// locator and the fingerprint are. Failure list:
//  C1 the fixture's clips parse; a row with no id, no http(s) audio or an empty or negative range is dropped; a bad body is nothing.
//  C2 "clips play from fingerprinted audio": a row offers the clip only when both fingerprints decode (12 s each, version 1, bands 8), the file is
//     seekable by bitrate and the bitrate is known; any other row says "episode only" and Play never asks the locator for it.
//  C3 a show lists its own clips (the feed matched without case or a trailing slash), ordered by id, at most ten, with the quote, the story and the length.
//  C4 a show's feed is asked once a session (and again after a retry), not on every draw; a 404 or a failure is no rows, silently.
//  C5 Play and Queue answer the clip's id for the native locator; a clip the phone does not have says so; Open episode answers the episode's page with
//     the clip's audio and start in the address, and does nothing for a clip with no feed.
import { moveAccount } from './account-phone.ts';
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import * as C from './clips.ts';
import { parseEpisode } from './podcast-model.ts';
import { resetWorld } from './api.ts';

const body = JSON.parse(readFileSync(new URL('../fixtures/podcasts/clips.json', import.meta.url), 'utf8')) as { clips: Record<string, unknown>[] };
const FEED = 'https://feeds.example.com/show.xml';
let calls: string[] = [];
let handler: (u: URL) => Response;

beforeEach(() => {
  resetWorld(); calls = [];
  handler = () => new Response(JSON.stringify(body), { status: 200 });
  globalThis.fetch = (async (input: RequestInfo | URL) => { calls.push(String(input)); return handler(new URL(String(input))); }) as typeof fetch;
});

describe('C1 parsing', () => {
  test('the fixture parses; unusable rows are dropped', () => {
    expect(C.parseClips(body).map((c) => c.clipID)).toEqual(['clip-000000000001', 'clip-000000000002', 'clip-000000000003', 'clip-000000000004']);
    const bad = [{ ...body.clips[0], clip_id: '' }, { ...body.clips[0], audio_url: 'file:///x.mp3' }, { ...body.clips[0], start: 70, end: 10 }, { ...body.clips[0], start: -1 }, 'x', null];
    expect(C.parseClips({ clips: bad })).toEqual([]);
    for (const b of [null, 'nope', {}, { clips: 'x' }]) expect(C.parseClips(b)).toEqual([]);
  });
});

describe('C2 clips play from fingerprinted audio', () => {
  const fp = (o: Record<string, unknown> = {}): ReturnType<typeof C.parseClip> => C.parseClip({ ...body.clips[0], fingerprint: { ...(body.clips[0].fingerprint as object), ...o } });
  test('a usable fingerprint offers the clip', () => { expect(C.row(fp()!).episodeOnly).toBe(false); });
  test('a clip with no fingerprint (an old record) is the episode', () => { expect(C.row(C.parseClip(body.clips[1])!).episodeOnly).toBe(true); });
  test('every way a fingerprint is unusable is the episode', () => {
    const short = btoa(String.fromCharCode(1, 8, 0, 60) + '\u0007'.repeat(60 * 8));
    for (const o of [{ seekable_by_bitrate: false }, { bitrate_kbps: 0 }, { version: 2 }, { end: 10, start: 10 }, { head: 'not base64!!' }, { tail: short }, { head: btoa('xx') }]) {
      expect(C.row(fp(o)!).episodeOnly).toBe(true);
    }
  });
});

describe('C3 and C4 a show\'s clips', () => {
  test('its own clips by feed, ordered, with quote, story and length', async () => {
    const r = await C.showClips('podcast:show/' + encodeURIComponent(FEED), 0);
    expect(r.rows.map((x) => x.clipID)).toEqual(['clip-000000000001', 'clip-000000000002', 'clip-000000000004']);
    expect(r.rows[0]).toEqual({ clipID: 'clip-000000000001', show: 'Synthetic Hour', episode: 'Episode One', length: '1 min', quote: 'A quote 1.', story: 'Story 1', episodeOnly: false });
    expect(r.rows[1].episodeOnly).toBe(true);
  });
  test('asked once, and again after a retry; the request is public and names the feed', async () => {
    const a = 'podcast:show/' + encodeURIComponent(FEED);
    await C.showClips(a, 0); await C.showClips(a, 0);
    expect(calls.length).toBe(1); expect(calls[0]).toContain('/api/platform/podcast-clips?'); expect(calls[0]).toContain('feed=' + encodeURIComponent(FEED));
    await C.showClips(a, 1);
    expect(calls.length).toBe(2);
  });
  test('a missing route, a failure, another page: no rows, silently', async () => {
    handler = () => new Response('', { status: 404 });
    expect((await C.showClips('podcast:show/' + encodeURIComponent(FEED), 0)).rows).toEqual([]);
    globalThis.fetch = (async () => { throw new Error('offline'); }) as typeof fetch;
    expect((await C.showClips('podcast:show/' + encodeURIComponent('https://x.example.com/'), 0)).rows).toEqual([]);
    expect((await C.showClips('radio:', 0)).rows).toEqual([]);
  });
  test('at most ten', async () => {
    const many = Array.from({ length: 14 }, (_, i) => ({ ...body.clips[0], clip_id: 'clip-' + (100 + i).toString(16).padStart(12, '0') }));
    handler = () => new Response(JSON.stringify({ clips: many }), { status: 200 });
    expect((await C.showClips('podcast:show/' + encodeURIComponent(FEED), 0)).rows.length).toBe(10);
  });
});

describe('C5 what a row does', () => {
  test('play and queue answer the clip for the native locator', async () => {
    const p = await C.clipAct('play', 'clip-000000000001');
    expect({ ...p.play, record: '' }).toEqual({ op: 'play', id: '', url: '', title: '', show: '', start: 0, clip: 'clip-000000000001', feed: FEED, record: '' }); expect(p.message).toBe('');
    const q = await C.clipAct('queue', 'clip-000000000001');
    expect(q.play.op).toBe('queue'); expect(q.message).toBe('Added to the queue.');
  });
  test('a clip the phone does not have says so and does nothing, for every op', async () => {
    for (const op of ['play', 'queue', 'open']) {
      const a = await C.clipAct(op, 'gone');
      expect(a.message).toBe(C.GONE); expect(a.play.clip).toBe(''); expect(a.open).toBe('');
    }
    expect(calls.length).toBe(1);                                                // the bulk set was asked for once
  });
  test('open episode names the episode page with the clip\'s audio and start', async () => {
    const a = await C.clipAct('open', 'clip-000000000001');
    const p = parseEpisode(a.open)!;
    expect(p.feed).toBe(FEED); expect(p.id).toBe('guid-1'); expect(p.hint).toEqual({ enclosure: 'https://cdn.example.com/one.mp3', start: 10, title: 'Episode One', show: 'Synthetic Hour' });
    expect(a.openTitle).toBe('Episode One');
    const none = { ...body.clips[0], clip_id: 'clip-0000000000f1', feed_url: '', episode_guid: '' };
    handler = () => new Response(JSON.stringify({ clips: [none] }), { status: 200 }); resetWorld();
    expect((await C.clipAct('open', 'clip-0000000000f1')).open).toBe('');
    const noGuid = { ...body.clips[0], clip_id: 'clip-0000000000f2', episode_guid: '' };
    handler = () => new Response(JSON.stringify({ clips: [noGuid] }), { status: 200 }); resetWorld();
    expect(parseEpisode((await C.clipAct('open', 'clip-0000000000f2')).open)!.id).toBe('https://cdn.example.com/one.mp3');
  });
});

// Failure list for the story page's "Hear it" (carries the Swift JoinHearItTests J3, DJClipsTests C9 and J6's bulk-set rules; the Swift HearIt.rows,
// ClipStore.shouldAsk and the native fetch are deleted):
//  H1 a story page lists the clips cut for its story (a version of the story has the story's clips), at most three, with the story's title; another page lists none.
//  H2 the whole set is asked for once a retry (one request, never one per story), again after a retry (a data source has no clock); a 404 or a failure is no rows, silently.
//  H3 Play and Queue hand the native side the clip's own record (its audio and its fingerprint), so the native side never asks the server for a clip.
describe('H1 hear it on a story page', () => {
  test('the story\'s own clips and nothing else', async () => {
    const r = await C.storyClips('pub:a', 0);
    expect(r.rows.map((x) => x.clipID)).toEqual(['clip-000000000001', 'clip-000000000002', 'clip-000000000004']);
    expect(r.rows[0].story).toBe('Story 1');
    expect((await C.storyClips('pub:a/v3', 0)).rows.length).toBe(3);
    expect((await C.storyClips('pub:b', 0)).rows.map((x) => x.clipID)).toEqual(['clip-000000000003']);
    for (const a of ['pub:zzz', 'texture:doc1', 'doc:abc', 'podcast:show/x', '']) expect((await C.storyClips(a, 0)).rows).toEqual([]);
  });
  test('at most three', async () => {
    const many = Array.from({ length: 6 }, (_, i) => ({ ...body.clips[0], clip_id: 'clip-' + (200 + i).toString(16).padStart(12, '0') }));
    handler = () => new Response(JSON.stringify({ clips: many }), { status: 200 });
    expect((await C.storyClips('pub:a', 0)).rows.length).toBe(3);
  });
});

describe('H2 one bulk request', () => {
  test('many stories, one request; the public route with a limit', async () => {
    await C.storyClips('pub:a', 0); await C.storyClips('pub:b', 0); await C.storyClips('pub:a/v2', 0);
    expect(calls.length).toBe(1); expect(calls[0]).toContain('/api/platform/podcast-clips?'); expect(calls[0]).toContain('limit=200'); expect(calls[0]).not.toContain('feed=');
    await C.storyClips('pub:a', 1);
    expect(calls.length).toBe(2);
  });
  test('a page that is not a story asks for nothing', async () => {
    await C.storyClips('texture:doc1', 0); await C.storyClips('radio:', 0);
    expect(calls.length).toBe(0);
  });
  test('a 404 and an offline phone are no rows, and a later success shows them', async () => {
    handler = () => new Response('', { status: 404 });
    expect((await C.storyClips('pub:a', 0)).rows).toEqual([]);
    globalThis.fetch = (async () => { throw new Error('offline'); }) as typeof fetch;
    expect((await C.storyClips('pub:a', 1)).rows).toEqual([]);
    globalThis.fetch = (async (input: RequestInfo | URL) => { calls.push(String(input)); return new Response(JSON.stringify(body), { status: 200 }); }) as typeof fetch;
    expect((await C.storyClips('pub:a', 2)).rows.length).toBe(3);
  });
  test('the clips of a story fetched here are found by Play without a second request', async () => {
    await C.storyClips('pub:a', 0);
    expect((await C.clipAct('play', 'clip-000000000001')).play.clip).toBe('clip-000000000001');
    expect(calls.length).toBe(1);
  });
  test('forgetting the account\'s session drops the set', async () => {
    await C.storyClips('pub:a', 0);
    expect(C.remembered()).toBeGreaterThan(0);
    resetWorld();
    expect(C.remembered()).toBe(0);
    await C.storyClips('pub:a', 0);
    expect(calls.length).toBe(2);
  });
});

describe('H4 refresh, replacement, order and the account', () => {
  test('a story reached again from another page asks again, and a failed first ask is retried then (no tick needed)', async () => {
    handler = () => new Response('', { status: 500 });
    expect((await C.storyClips('pub:a', 0)).rows).toEqual([]);
    expect((await C.storyClips('pub:a', 0)).rows).toEqual([]);          // the same page drawn again: no new request
    expect(calls.length).toBe(1);
    handler = () => new Response(JSON.stringify(body), { status: 200 });
    await C.storyClips('radio:', 0);                                      // another page on top
    expect((await C.storyClips('pub:a', 0)).rows.length).toBe(3);
    expect(calls.length).toBe(2);
    await C.storyClips('pub:b', 0);                                       // a swipe to the next story asks nothing more
    expect(calls.length).toBe(2);
  });
  test('a fetched clip replaces the held one with the same id; ids absent from the answer stay', async () => {
    await C.storyClips('pub:a', 0);
    const changed = body.clips.filter((c) => c.clip_id === 'clip-000000000001').map((c) => ({ ...c, audio_url: 'https://cdn.example.com/new.mp3', quote: 'A repaired quote.' }));
    handler = () => new Response(JSON.stringify({ clips: changed }), { status: 200 });
    const r = await C.storyClips('pub:a', 1);
    expect(r.rows.find((x) => x.clipID === 'clip-000000000001')!.quote).toBe('A repaired quote.');
    expect(r.rows.length).toBe(3);
    expect(JSON.parse((await C.clipAct('play', 'clip-000000000001')).play.record).audio_url).toBe('https://cdn.example.com/new.mp3');
  });
  test('a story keeps the server\'s order (best first), not the order of the ids', async () => {
    const mk = (id: string) => ({ ...body.clips[0], clip_id: id, story: { route: '/a', title: 'S' } });
    handler = () => new Response(JSON.stringify({ clips: [mk('clip-0000000000c3'), mk('clip-0000000000a1'), mk('clip-0000000000b2')] }), { status: 200 });
    expect((await C.storyClips('pub:a', 0)).rows.map((x) => x.clipID)).toEqual(['clip-0000000000c3', 'clip-0000000000a1', 'clip-0000000000b2']);
  });
  test('a clip set that lands after the account changed is dropped', async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((r) => { release = r; });
    globalThis.fetch = (async (input: RequestInfo | URL) => { calls.push(String(input)); await gate; return new Response(JSON.stringify(body), { status: 200 }); }) as typeof fetch;
    const pending = C.storyClips('pub:a', 0);
    moveAccount();                                       // the world the ask began in closes: its chain never resumes
    release();
    const settled = await Promise.race([pending.then(() => 'answered'), new Promise<string>((r) => setTimeout(() => r('never'), 50))]);
    expect(settled).toBe('never');
    expect(C.remembered()).toBe(0);
  });
});

describe('H3 the native side is handed the clip, not asked to find it', () => {
  test('play and queue carry the record the server sent, fingerprint included', async () => {
    for (const op of ['play', 'queue']) {
      const a = await C.clipAct(op, 'clip-000000000001');
      const rec = JSON.parse(a.play.record) as Record<string, unknown>;
      expect(rec.clip_id).toBe('clip-000000000001'); expect(rec.audio_url).toBe('https://cdn.example.com/one.mp3');
      expect(rec.fingerprint).toEqual(body.clips[0].fingerprint);
    }
  });
  test('a clip with no fingerprint is handed over without one (the native side plays its episode)', async () => {
    const rec = JSON.parse((await C.clipAct('play', 'clip-000000000002')).play.record) as Record<string, unknown>;
    expect(rec.fingerprint).toBeUndefined();
  });
  test('open episode hands nothing to the native side', async () => {
    expect((await C.clipAct('open', 'clip-000000000001')).play.record).toBe('');
  });
});
