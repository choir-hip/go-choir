// Takes under bun test: the wire, the list and the listener's own, the candidates, the multipart body, the Takes section's words.
// Failure list (the Swift TakesTests and TakesPage rules; the numbers are takes.ts's): T1 bounds; T2 a row that is not a live take; T3 candidates;
// T5 the upload agrees with itself; T6 mutes and own ids survive a relaunch; T7 an unknown handle is "new"; T8 only a path on the Choir host is followed;
// T9 the page's words; T10 drill outcomes are marked and never counted; T11 a refusal is shown in the server's words; no server is "not available yet".
import { describe, expect, test } from 'bun:test';
import {
  BOUNDS, parseTake, parseMine, takesOf, mineOf, title, candidates, candidateJSON, merged, multipart, sha256Hex, client, mp4Seconds, recordWords, parseRecord, pathOf,
  loadKept, saveKept, emptyKept, owns, addOwn, mute, unmute, muteLine, isMuted, page, clock, lengthWords, counted, parseOutcomes, stripUnsafe, cut, audioPath,
  TakesUnavailable, TakesRefused, type Net, type Recorder, type Take, KEPT_FILE,
} from './takes.ts';
import list from '../fixtures/takes/list.json';
import mineFx from '../fixtures/takes/mine.json';
import recordFx from '../fixtures/takes/record.json';
import publishFx from '../fixtures/takes/publish.json';
import refused from '../fixtures/takes/refused-daily.json';

const files = new Map<string, Uint8Array>();
const storage = { fs: { readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
const all = takesOf(list);

describe('T1 and T2 the bounds and the list', () => {
  test('T1 the bounds are the spec\'s', () => {
    expect([BOUNDS.minSeconds, BOUNDS.maxSeconds, BOUNDS.maxBytes, BOUNDS.maxTranscriptBytes, BOUNDS.maxCandidates]).toEqual([3, 60, 524288, 2000, 3]);
    expect(cut('é'.repeat(1500), 2000).length).toBe(1000);
  });
  test('T2 only live takes with an id and a handle are takes', () => {
    expect(all.length).toBeGreaterThan(0);
    for (const t of all) { expect(t.state).toBe('live'); expect(t.takeID).toMatch(/^take-/); }
    expect(parseTake({ take_id: 'a b', publisher: 'voice-1', state: 'live' })).toBeNull();
    expect(parseTake({ take_id: 'take-1', publisher: 'bad handle', state: 'live' })).toBeNull();
    expect(parseTake({ take_id: 'take-1', publisher: 'voice-1', state: 'deleted' })).toBeNull();
    expect(parseTake({ take_id: 'take-1', publisher: 'voice-1', state: 'yanked' })).toBeNull();
    expect(parseTake({ take_id: 'take-1', publisher: 'voice-1' })?.state).toBe('live');
    expect(takesOf(list).map((t) => t.takeID)).toEqual([...new Set(takesOf(list).map((t) => t.takeID))]);
  });
  test('the first line of the transcript is the title; hostile characters in a stranger\'s title are made safe', () => {
    expect(all[0].transcript).toContain('Second line.');
    expect(title(all[0].transcript)).toBe('The harbor vote is tonight and nobody has said what the fee is for.');
    expect(title('\n\n  hello ‮ there  ​')).toBe('hello there');
    expect(stripUnsafe('a‮b\u0007c\nd')).toBe('ab c\nd');
    expect(utf(title('x'.repeat(500)))).toBeLessThanOrEqual(120);
  });
  test('T8 an audio address off the host is not followed with the key', () => {
    expect(pathOf('/api/platform/takes/x/audio')).toBe('/api/platform/takes/x/audio');
    expect(pathOf('//evil.example/x')).toBeNull();
    expect(pathOf('https://choir.news/api/platform/takes/x/audio?a=1')).toBe('/api/platform/takes/x/audio?a=1');
    expect(parseTake({ take_id: 'take-1', publisher: 'voice-1', audio_url: '//evil.example/a' })?.audioPath).toBe(audioPath('take-1'));
  });
});
const utf = (s: string): number => new TextEncoder().encode(s).length;

describe('T3 candidates', () => {
  const k = { ...emptyKept(), ids: ['take-00000000000000a5'], handle: 'voice-me0001' };
  test('not mine, not muted, newest first, at most three, kind take', () => {
    const picked = candidates(all, { ...k, muted: ['voice-9b0e44'] });
    expect(picked.length).toBeLessThanOrEqual(3);
    expect(picked.some((t) => t.publisher === 'voice-9b0e44' || t.publisher === 'voice-me0001')).toBe(false);
    const times = picked.map((t) => t.publishedAt ?? 0);
    expect(times).toEqual([...times].sort((a, b) => b - a));
    expect(candidateJSON(picked[0]).kind).toBe('take');
    expect(JSON.stringify(candidateJSON(picked[0]))).not.toContain('audio');
  });
  test('three takes count toward the forty', () => {
    const stories = Array.from({ length: 40 }, (_, i) => ({ kind: 'story', n: i }));
    const m = merged(stories, all.concat(all));
    expect(m.length).toBe(40); expect(m.filter((x) => x.kind === 'take').length).toBe(3);
  });
  test('a take with no words is never offered', () => {
    expect(candidates([{ ...all[0], transcript: '' }], emptyKept())).toEqual([]);
  });
});

describe('T5 the upload', () => {
  test('the multipart body carries the audio, the transcript and the hash, and they agree', async () => {
    const audio = new Uint8Array(2000).map((_, i) => i % 251);
    const m = await multipart({ audio, transcript: 'hello there', duration: 12.5, respondsTo: { route: 'harbor-fees', title: 'Harbor' } }, 'B');
    expect(m.contentType).toBe('multipart/form-data; boundary=B');
    const text = new TextDecoder('latin1').decode(m.body);
    expect(text).toContain('name="transcript"\r\n\r\nhello there');
    expect(text).toContain('name="duration_seconds"\r\n\r\n12.50');
    expect(text).toContain('name="sha256"\r\n\r\n' + (await sha256Hex(audio)));
    expect(text).toContain('name="responds_to"\r\n\r\n{"route":"harbor-fees","title":"Harbor"}');
    expect(text).toContain('filename="take.m4a"\r\nContent-Type: audio/mp4');
    expect(text.includes(new TextDecoder('latin1').decode(audio))).toBe(true);
    expect((await sha256Hex(audio)).length).toBe(64);
    const none = new TextDecoder().decode((await multipart({ audio: new Uint8Array(3), transcript: 'x', duration: 3, respondsTo: null }, 'B')).body);
    expect(none).not.toContain('responds_to');
  });
  test('the file\'s own length is read from its mvhd box', () => {
    const box = (t: string, b: number[]): number[] => { const n = 8 + b.length; return [n >>> 24, (n >> 16) & 255, (n >> 8) & 255, n & 255, ...[...t].map((c) => c.charCodeAt(0)), ...b]; };
    const mvhd = [...new Array(12).fill(0), 0, 0, 0x03, 0xe8, 0, 0, 0x30, 0x39, ...new Array(80).fill(0)];
    const f = new Uint8Array([...box('ftyp', [...'M4A '].map((c) => c.charCodeAt(0)).concat([0, 0, 0, 0])), ...box('moov', box('mvhd', mvhd))]);
    expect(mp4Seconds(f)).toBeCloseTo(12.345, 4);
    expect(mp4Seconds(new TextEncoder().encode('RIFFxxxxWAVEfmt '))).toBeNull();
  });
});

function net(handler: (m: string, p: string, o?: { query?: Record<string, string | number> }) => { status: number; json?: unknown; bytes?: Uint8Array }, seen: string[] = []): Net {
  return async (m, p, o) => { seen.push(`${m} ${p}`); const r = handler(m, p, o); return { status: r.status, json: r.json ?? null, bytes: r.bytes ?? new Uint8Array() }; };
}
describe('T7 and T11 the client', () => {
  test('publish is one multipart POST and returns the take', async () => {
    const seen: string[] = [];
    const t = await client(net(() => ({ status: 201, json: publishFx }), seen)).publish({ audio: new Uint8Array(100).fill(7), transcript: 'hi', duration: 4, respondsTo: null }, 'B');
    expect(t.takeID).toBe('take-00000000000000b6'); expect(seen).toEqual(['POST /api/platform/takes']);
  });
  test('over the bound is refused before the network; the server\'s refusal is shown in its words; no server is "not available yet"', async () => {
    const seen: string[] = [];
    await expect(client(net(() => ({ status: 201 }), seen)).publish({ audio: new Uint8Array(BOUNDS.maxBytes + 1), transcript: 'hi', duration: 4, respondsTo: null }, 'B')).rejects.toBeInstanceOf(TakesRefused);
    expect(seen).toEqual([]);
    await expect(client(net(() => ({ status: 429, json: refused }))).publish({ audio: new Uint8Array(10), transcript: 'hi', duration: 4, respondsTo: null }, 'B'))
      .rejects.toThrow('invalid take: 10 takes in the last 24 hours, the limit is 10');
    const gone = client(net(() => ({ status: 404 })));
    await expect(gone.publish({ audio: new Uint8Array(10), transcript: 'hi', duration: 4, respondsTo: null }, 'B')).rejects.toBeInstanceOf(TakesUnavailable);
    await expect(gone.list()).rejects.toBeInstanceOf(TakesUnavailable); await expect(gone.mine()).rejects.toBeInstanceOf(TakesUnavailable);
  });
  test('list, mine, delete, record, audio', async () => {
    const seen: string[] = [];
    const c = client(net((m, p, o) => {
      if (m === 'GET' && p === '/api/platform/takes') return { status: 200, json: o?.query?.mine ? mineFx : list };
      if (p === '/api/platform/takes/record') return { status: 200, json: recordFx };
      if (m === 'DELETE') return { status: 204 };
      return { status: 200, bytes: new Uint8Array([1, 2, 3]) };
    }, seen));
    expect((await c.list()).length).toBe(all.length);
    const mine = await c.mine();
    expect(mine[0].takeID).toBe('take-00000000000000a5'); expect(mine[0].published).toBe(true);
    await c.remove('take-00000000000000a5');
    expect(seen).toContain('DELETE /api/platform/takes/take-00000000000000a5');
    expect(recordWords(await c.record('voice-me0001'))).toBe('heard through by 7 of 11 listeners over 5 takes');
    expect((await c.audio('/api/platform/takes/x/audio')).length).toBe(3);
    await expect(c.audio('//evil.example/a')).rejects.toThrow();
  });
  test('T7 an unknown publisher reads as new, not as a failure', async () => {
    const r = await client(net(() => ({ status: 404 }))).record('nobody');
    expect(recordWords(r)).toBe('new: no outcomes yet');
    expect(recordWords(parseRecord({ summary: 'heard through 7 of 11 times, by 5 distinct listeners, over 3 takes', new: false }))).toBe('heard through 7 of 11 times, by 5 distinct listeners, over 3 takes');
    expect(recordWords(parseRecord({ new: true }))).toBe('new: no outcomes yet');
    expect(recordWords(null)).toBe('new: no outcomes yet');
  });
});

describe('T6 what the phone keeps', () => {
  test('mutes and own ids are kept, listed, undone, and survive a relaunch; the same handle twice is one mute; wipe removes them', async () => {
    files.clear();
    let k = emptyKept();
    const a = mute(k, 'voice-3fa2c1'); k = a.kept; expect(a.added).toBe(true);
    expect(mute(k, 'voice-3fa2c1').added).toBe(false); expect(mute(k, 'bad handle').added).toBe(false);
    k = addOwn(k, 'take-1', 'voice-me0001');
    await saveKept(storage, k);
    const back = await loadKept(storage);
    expect(back.muted).toEqual(['voice-3fa2c1']); expect(back.handle).toBe('voice-me0001'); expect(owns(back, { takeID: 'take-1', publisher: 'x' })).toBe(true);
    expect(owns(back, { takeID: 'other', publisher: 'voice-me0001' })).toBe(true); expect(isMuted(back, 'voice-3fa2c1')).toBe(true);
    expect(unmute(back, 'voice-3fa2c1').had).toBe(true); expect((await loadKept(storage)).muted.length).toBe(1);
    expect(muteLine('voice-3fa2c1')).toBe('Never play takes by voice-3fa2c1.');
  });
});

describe('T9 and T10 the page and the marks', () => {
  const rec = (o: Partial<Recorder> = {}): Recorder => ({ state: 'idle', elapsed: 0, transcript: '', previewing: false, notice: '', respondsTitle: '', canPublish: false, ...o });
  const base = { mine: [], kept: emptyKept(), myRecord: null, justPublished: '' };
  test('the control offered matches the recorder\'s state', () => {
    expect(page(rec(), base)).toMatchObject({ canRecord: true, canStop: false, canPublish: false, canDiscard: false, limit: '1:00', elapsed: '0:00', publishLine: '' });
    expect(page(rec({ state: 'recording', elapsed: 30 }), base)).toMatchObject({ canRecord: false, canStop: true, fraction: 0.5, elapsed: '0:30' });
    const review = page(rec({ state: 'review', transcript: 'my words', canPublish: true }), base);
    expect(review).toMatchObject({ canDiscard: true, canPublish: true, transcript: 'my words', publishLine: BOUNDS.publishLine, rightsLine: BOUNDS.rightsLine });
    expect(page(rec({ state: 'publishing', transcript: 'x' }), base)).toMatchObject({ canRecord: false, canStop: false, canDiscard: false });
    expect(page(rec({ state: 'review', transcript: '' }), base).canPublish).toBe(false);
  });
  test('a take recorded over a story says which story; just published names the publisher', () => {
    expect(page(rec({ state: 'recording', respondsTitle: 'Harbor fees rise' }), base).respondsTo).toBe('Answering: Harbor fees rise');
    expect(page(rec(), base).respondsTo).toBe('');
    expect(page(rec(), { ...base, justPublished: 'voice-me0001' }).justPublished).toBe('Published as voice-me0001.');
  });
  test('my takes: newest first, a length, published or on this phone; a muted voice is listed with its line; the record is shown once there is a handle', () => {
    const mine = mineOf(mineFx);
    const p = page(rec(), { mine: [...mine, { ...mine[0], takeID: 't2', transcript: '', published: false, at: 1, duration: 0.5 }], kept: { ids: [], handle: 'voice-me0001', muted: ['voice-9b0e44'] }, myRecord: parseRecord(recordFx), justPublished: '' });
    expect(p.mine[0]).toMatchObject({ id: 'take-00000000000000a5', state: 'Published', length: '9 sec' });
    expect(p.mine[1]).toMatchObject({ title: 'A take', state: 'On this phone', length: '' });
    expect(p.muted).toEqual([{ handle: 'voice-9b0e44', line: 'Never play takes by voice-9b0e44.' }]);
    expect(p.record).toBe('heard through by 7 of 11 listeners over 5 takes');
    expect(page(rec(), { ...base, myRecord: parseRecord(recordFx) }).record).toBe('');
    expect(clock(61)).toBe('1:01'); expect(lengthWords(120)).toBe('2 min');
  });
  test('T10 drill outcomes are marked and never counted', () => {
    const o = parseOutcomes({ heard: 3, skipped: 1, drill_heard: 40, drill_skipped: 9 })!;
    expect(o).toEqual({ heard: 3, skipped: 1, drillHeard: 40, drillSkipped: 9 });
    expect(counted(o)).toEqual({ heard: 3, skipped: 1 });
    expect(parseMine({ take_id: 'take-1', outcomes: { heard: 3, skipped: 1, drill_heard: 40, drill_skipped: 9 } })?.outcomes?.drillHeard).toBe(40);
    expect(recordWords(parseRecord({ heard: 3, skipped: 1, takes: 1, new: false, drill: { heard: 40 } }))).toBe('heard through by 3 of 4 listeners over 1 take');
  });
});
