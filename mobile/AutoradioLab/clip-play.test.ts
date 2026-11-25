// What a clip row's Play and Queue put on the run sheet (it was Swift: AutoradioModule+Joins.swift `clipPlay` and `HearIt`, which also kept the clip in a native ClipStore for a locator that no
// production code calls any more). Failure list, written before the code:
//  P1 a clip whose fingerprints are usable is a `clip` entry: the episode's audio from the clip's start to its end, with the clip's id; the title is the episode's (else the show's).
//  P2 a clip with no usable fingerprint is its episode from the start: no start, no end, not a clip entry.
//  P3 the episode id is the podcast key of the feed and the guid, else of the feed and the audio address; no feed, no id.
//  P5 "Keep listening" to a clip asks the deck to stop ending it (`deckAct clearEnd`), through the app module's one `deck` op.
//  P4 queue adds, play plays now; any other op does nothing; a record that is not that clip says it is not available; a missing record likewise.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { parseClips } from './clips.ts';
import { clipEntryOf, radioPlayClip } from './clip-play.ts';
import { audioSpec } from './runsheet-entries.ts';
import { episodeKey } from './podcast-model.ts';
import type { Entry } from './runsheet-model.ts';
import { getRadio, radioPlayAudio, wipeRadio } from './radio-bridge.ts';
import { forgetOnboarding } from './onboarding.ts';
import { adoptNative, resetSeam } from './api.ts';
import { podcastNow } from './podcast-now.ts';

const body = JSON.parse(readFileSync(new URL('../fixtures/podcasts/clips.json', import.meta.url), 'utf8')) as { clips: Record<string, unknown>[] };
const all = parseClips(body);
const withFp = all.find((c) => c.fingerprint !== null && c.feedURL !== '')!;
const noFp = { ...withFp, fingerprint: null };

describe('the entry', () => {
  test('P1 a usable clip plays as a clip', () => {
    const e = clipEntryOf(withFp);
    expect(e.kind).toBe('clip');
    expect(e.source?.ref.startsWith('clip:' + withFp.clipID)).toBe(true);
    expect(e.source?.start).toBe(withFp.start); expect(e.source?.end).toBe(withFp.end);
    expect(e.title).toBe(withFp.episodeTitle === '' ? withFp.show : withFp.episodeTitle);
  });
  test('P2 a clip with no usable fingerprint is its episode from the start', () => {
    const e = clipEntryOf(noFp);
    expect(e.kind).toBe('episode'); expect(e.source?.start).toBeUndefined(); expect(e.source?.end).toBeUndefined();
  });
  test('P3 the episode id', () => {
    const e = clipEntryOf({ ...noFp, episodeGUID: 'g1' });
    expect(e.source?.ref).toBe('episode:' + episodeKey(noFp.feedURL, 'g1'));
    const f = clipEntryOf({ ...noFp, episodeGUID: '' });
    expect(f.source?.ref).toBe('episode:' + episodeKey(noFp.feedURL, noFp.audioURL));
    expect(clipEntryOf({ ...noFp, feedURL: '' }).source?.ref).toBe('episode:' + noFp.audioURL);
  });
});

describe('the act', () => {
  const placed: { op: string; e: Entry }[] = [];
  const place = async (op: string, e: Entry): Promise<void> => { placed.push({ op, e }); };
  const rec = JSON.stringify(withFp.raw);
  test('P4 play, queue, and what is refused', async () => {
    placed.length = 0;
    expect(await radioPlayClip(undefined, undefined, 'play', withFp.clipID, rec, place)).toEqual({ status: 'ok', message: '' });
    expect(await radioPlayClip(undefined, undefined, 'queue', withFp.clipID, rec, place)).toEqual({ status: 'ok', message: '' });
    expect(placed.map((p) => p.op)).toEqual(['play', 'queue']);
    expect((await radioPlayClip(undefined, undefined, 'dance', withFp.clipID, rec, place)).status).toBe('ok'); expect(placed.length).toBe(2);
    expect((await radioPlayClip(undefined, undefined, 'play', 'other-id', rec, place)).status).toBe('error');
    expect((await radioPlayClip(undefined, undefined, 'play', withFp.clipID, 'not json', place)).message).toBe('That clip is not available any more.');
    expect(placed.length).toBe(2);
  });
});

describe('the act, on the real path', () => {
  test('P4b with no injected placer a play puts the clip entry on the run sheet and tells the deck (the radio\'s own verb)', async () => {
    resetSeam(); forgetOnboarding(); await wipeRadio(undefined);
    const seen: { op: string; name?: string; args?: Record<string, unknown> }[] = [];
    const native = { available: true, later: async (r: { op: string; name?: string; args?: Record<string, unknown> }) => {
      seen.push(r);
      if (r.op === 'radioFacts') return { at: 0, ready: true, inbox: [], position: { block: 0, seconds: 0 }, foreground: true, sentences: [], sentenceIndex: 0, progress: 0 };
      if (r.op === 'deck') return { why: r.name === 'deckPlay' ? 'playing' : 'empty', item: r.name === 'deckPlay' ? String(r.args?.id) : '', block: 0, seconds: 0, duration: 100, held: [], seq: 1, acked: 0, journal: [] };
      return { ok: true };
    } };
    adoptNative(native as never, 'test');
    expect(await radioPlayClip(undefined, native as never, 'play', withFp.clipID, JSON.stringify(withFp.raw))).toEqual({ status: 'ok', message: '' });
    const r = await getRadio(undefined, native as never, false);
    const on = r.rt.store.onAir();
    expect(on?.kind).toBe('clip'); expect(on?.source?.start).toBe(withFp.start); expect(on?.source?.end).toBe(withFp.end);
    expect(seen.some((x) => x.op === 'deck' && x.name === 'deckPlay')).toBe(true);
  });
});

const audioEpisode = (e: Entry): string => String(audioSpec(e)?.episode_id ?? '');
const EP = clipEntryOf(withFp).source?.ref.startsWith('clip:') ? String(audioSpec(clipEntryOf(withFp))?.episode_id ?? '') : '';
describe('keep listening', () => {
  const setup = async () => {
    resetSeam(); forgetOnboarding(); await wipeRadio(undefined);
    const seen: { op: string; name?: string; args?: Record<string, unknown> }[] = [];
    let item = '';
    const native = { available: true, later: async (r: { op: string; name?: string; args?: Record<string, unknown> }) => {
      seen.push(r);
      if (r.op === 'radioFacts') return { at: 0, ready: true, inbox: [], position: { block: 0, seconds: 0 }, foreground: true, sentences: [], sentenceIndex: 0, progress: 0 };
      if (r.op === 'deck') { if (r.name === 'deckPlay') item = String(r.args?.id); return { why: item ? 'playing' : 'empty', item, block: 0, seconds: 3, duration: 100, held: [], seq: 1, acked: 0, journal: [] }; }
      return { ok: true };
    } };
    adoptNative(native as never, 'test');
    await radioPlayClip(undefined, native as never, 'play', withFp.clipID, JSON.stringify(withFp.raw));
    return { native, seen };
  };
  test('P5 the deck is told to clear the clip\'s end', async () => {
    const { native, seen } = await setup();
    expect(await radioPlayAudio(undefined, native as never, 'keep', EP, '', '', '', 0)).toEqual({ status: 'ok', message: '' });
    expect(seen.some((r) => r.op === 'deck' && r.name === 'deckAct' && r.args?.op === 'clearEnd')).toBe(true);
  });
  test('P5b after Keep listening the clip is no longer a clip, on the run sheet\'s view and on the episode page, and the screens are told', async () => {
    const { native } = await setup();
    const r = await getRadio(undefined, native as never, false);
    await r.rt.peek();
    expect(r.rt.view().clip).toBe(true); expect((await podcastNow(undefined, native as never)).clip).toBe(true);
    const tick = r.tick;
    await radioPlayAudio(undefined, native as never, 'keep', EP, '', '', '', 0);
    expect(r.rt.view().clip).toBe(false); expect((await podcastNow(undefined, native as never)).clip).toBe(false); expect(r.tick).toBeGreaterThan(tick);
  });
  test('P5d a clip that has ended (the deck plays the NEXT clip, the ended line not pumped yet): Keep listening sends no clearEnd and marks nothing kept', async () => {
    const { native, seen } = await setup();
    const r = await getRadio(undefined, native as never, false);
    const air = r.rt.store.onAir()!;
    const was = seen.length;
    // the deck has moved on to another item; the run sheet still has the first clip on air
    const moved = { available: true, later: async (q: { op: string; name?: string; args?: Record<string, unknown> }) => { seen.push(q); if (q.op === 'deck') return { why: 'playing', item: 'the-next-clip', block: 0, seconds: 1, duration: 100, held: [], seq: 2, acked: 0, journal: [] }; return (native as { later: (x: unknown) => Promise<unknown> }).later(q); } };
    adoptNative(moved as never, 'test');
    await radioPlayAudio(undefined, moved as never, 'keep', String(audioEpisode(air)), '', '', '', 0);
    expect(seen.slice(was).some((x) => x.op === 'deck' && x.name === 'deckAct' && x.args?.op === 'clearEnd')).toBe(false);
    expect(r.rt.isKept(air.id)).toBe(false); expect(r.rt.view().clip).toBe(true);
  });
  test('P5e Keep listening for another episode than the one on air does nothing', async () => {
    const { native, seen } = await setup();
    const was = seen.length;
    await radioPlayAudio(undefined, native as never, 'keep', 'pod:another/episode', '', '', '', 0);
    expect(seen.slice(was).some((x) => x.args?.op === 'clearEnd')).toBe(false);
  });
  test('P5f sign-out forgets what was kept', async () => {
    const { native } = await setup();
    const r = await getRadio(undefined, native as never, false);
    await radioPlayAudio(undefined, native as never, 'keep', EP, '', '', '', 0);
    const id = r.rt.store.onAir()!.id; expect(r.rt.isKept(id)).toBe(true);
    await r.rt.signOut(); expect(r.rt.isKept(id)).toBe(false);
  }, 15000);
  test('P5c Keep listening with no clip on air changes nothing', async () => {
    resetSeam(); forgetOnboarding(); await wipeRadio(undefined);
    const seen: string[] = []; const native = { available: true, later: async (r: { op: string; name?: string; args?: Record<string, unknown> }) => { seen.push(`${r.op}:${r.args?.op ?? ''}`); return r.op === 'deck' ? { why: 'empty', item: '', block: 0, seconds: 0, duration: 0, held: [], seq: 0, acked: 0, journal: [] } : r.op === 'radioFacts' ? { at: 0, ready: true, inbox: [], position: { block: 0, seconds: 0 }, foreground: true, sentences: [], sentenceIndex: 0, progress: 0 } : {}; } };
    adoptNative(native as never, 'test');
    await radioPlayAudio(undefined, native as never, 'keep', '', '', '', '', 0);
    expect(seen.some((x) => x.endsWith('clearEnd'))).toBe(false);
  });
});

// What the player is playing when it is a podcast (it was the native source `podcastNow`, answered from the Swift deck and the Swift run sheet mirror).
//  N1 a podcast episode on air and sounding: its key, whether it is a clip (it has an end), where the clip starts, and the deck's seconds and duration.
//  N2 anything else (nothing on air, speech, a file, a web page's audio with no episode key, the deck stopped or on another item) is the empty answer.
import { podcastNowOf } from './podcast-now.ts';
describe('what the player is playing', () => {
  const clip = clipEntryOf(withFp);
  const deck = (item: string, why = 'playing') => ({ why, item, block: 0, seconds: 12.5, duration: 300, held: [], seq: 1, acked: 0, journal: [] }) as never;
  test('N1 a clip on air', () => {
    const n = podcastNowOf(clip, deck(clip.id));
    expect(n).toEqual({ episodeId: expect.stringMatching(/^pod:/), clip: true, start: withFp.start, elapsed: 12.5, duration: 300 });
    const ep = podcastNowOf(clipEntryOf(noFp), deck(clipEntryOf(noFp).id));
    expect(ep).toMatchObject({ clip: false, start: 0 });
  });
  test('N2 anything else is empty', () => {
    const none = { episodeId: '', clip: false, start: 0, elapsed: 0, duration: 0 };
    expect(podcastNowOf(undefined, deck(''))).toEqual(none);
    expect(podcastNowOf(clip, deck('another'))).toEqual(none);
    expect(podcastNowOf(clip, deck(clip.id, 'stopped'))).toEqual(none);
    expect(podcastNowOf(clip, deck(clip.id, 'empty'))).toEqual(none);
    expect(podcastNowOf(clipEntryOf({ ...noFp, feedURL: '' }), deck(clipEntryOf({ ...noFp, feedURL: '' }).id))).toEqual(none);
  });
});
