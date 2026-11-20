// The take session in shared code under bun test (take-session.ts). It replaces the Swift TakeSessionTests (R1 to R7) and the page tests of TakesTests, and adds the upgrade ones.
// The microphone, the file, the recogniser and the preview player stay native (TakeSession.swift: the voice); here they are a FakeDevice that answers what the native recorder answers.
//
// Failure list, written before the code (S = session, U = what an upgraded phone keeps):
//   S1  every way a take can end without a recording says so in words: no microphone, a refused gate, a recorder that would not open, under 3 s, nothing recorded, too large; the
//       ceiling says "Stopped at 60 seconds" and keeps the take; a transcript that came back blank says "Record it again" and cannot be published
//   S2  the page follows the recorder: Record when idle, Stop with the time and the 1:00 bound, a wait line while the words are made, the review actions after, nothing to press
//       while publishing; the story on air is named while a take is under way; a take with nothing on air cites nothing
//   S3  Publish needs a take in review with words and a signed-in phone; it is ONE multipart POST whose claim is what the file's own header says, clamped to 3..60; the id and the
//       publisher's handle are remembered so the public list never offers the voice back; the file is dropped and the radio given back once
//   S4  a failure keeps the recording: a 500, a server without takes (404) and the daily limit (429) each say so in their words and the take stays in review; a retry posts again
//       (and only then); a second Publish while one is in flight posts nothing
//   S5  the preview toggles through the recorder and Publish stops it first
//   S6  delete drops the row and the own id; refresh reads the listener's takes and record; a server without takes says "not available yet"
//   S7  a mute is kept, mirrored to the native programme and sent to the learning engine ONCE; the same handle twice is one mute; unmute undoes it
//   S8  Record sends the bounds to the recorder; Discard drops the take and clears the words; a phone with no recorder says so and nothing else changes
//   U1  an upgraded phone's own ids, handle and muted voices (the Swift files) are carried into app:/data/takes.json once, unioned with what is already there, and the old copy is read,
//       never removed; the carry-over is marked only when it was written; no answer from native is tried again
//   U2  a mute, a publish or a delete made before the carry-over has finished cannot lose the old voices (they wait for it)
//   U3  a take in review is the recorder's: a new session (the shared code restarted) still shows it and can publish it; a failed publish never discards it; a double tap never posts twice
import { describe, expect, test } from 'bun:test';
import { act, pageFor, newOverlay, parseNative, IDLE, limits, type Device, type Env, type Native, type Overlay } from './take-session.ts';
import { client, emptyKept, KEPT_FILE, loadKept, wipeKept, BOUNDS, type Net } from './takes.ts';
import { SignedOutError, nextGeneration } from './api.ts';
import publishFx from '../fixtures/takes/publish.json';
import mineFx from '../fixtures/takes/mine.json';
import recordFx from '../fixtures/takes/record.json';
import refused from '../fixtures/takes/refused-daily.json';

// ---------------------------------------------------------------------------------------------------------------- doubles

const mp4 = (seconds: number): Uint8Array => {
  const box = (t: string, b: number[]): number[] => { const n = 8 + b.length; return [n >>> 24, (n >> 16) & 255, (n >> 8) & 255, n & 255, ...[...t].map((c) => c.charCodeAt(0)), ...b]; };
  const d = Math.round(seconds * 1000);
  const mvhd = [...new Array(12).fill(0), 0, 0, 0x03, 0xe8, d >>> 24, (d >> 16) & 255, (d >> 8) & 255, d & 255, ...new Array(80).fill(0)];
  return new Uint8Array([...box('ftyp', [...'M4A '].map((c) => c.charCodeAt(0)).concat([0, 0, 0, 0])), ...box('moov', box('mvhd', mvhd))]);
};

class FakeDevice implements Device {
  snap: Native = { ...IDLE };
  log: string[] = [];
  words = 'A neighbour speaking.';
  seconds = 10; file: Uint8Array | null = mp4(10);
  refuse: Native['ended'] | '' = ''; detail = '';
  responds = { title: '', route: '' };
  legacyAnswer: { found: boolean; ids: string[]; handle: string; muted: string[] } | null = null; legacyCalls = 0;
  mirrored: { ids: string[]; handle: string; muted: string[] }[] = [];
  steered: string[] = [];
  limits: unknown = null;
  async state() { return this.snap; }
  async start(l: unknown) {
    this.log.push('start'); this.limits = l;
    if (this.refuse) this.snap = { ...IDLE, ended: this.refuse, detail: this.detail };
    else this.snap = { ...IDLE, phase: 'recording', respondsTitle: this.responds.title, respondsRoute: this.responds.route };
    return this.snap;
  }
  async stop(auto = false) {
    this.log.push('stop');
    if (this.seconds < BOUNDS.minSeconds) this.snap = { ...IDLE, ended: 'short' };
    else if (this.file && this.file.length > BOUNDS.maxBytes) this.snap = { ...IDLE, ended: 'large' };
    else this.snap = { ...this.snap, phase: 'review', transcript: this.words.trim(), seconds: this.seconds, elapsed: this.seconds, ended: auto ? 'ceiling' : '' };
    return this.snap;
  }
  async preview(on: boolean | null) { this.log.push(`preview:${on}`); this.snap = { ...this.snap, previewing: on === null ? !this.snap.previewing : on }; return this.snap; }
  async discard() { this.log.push('discard'); this.snap = { ...IDLE }; return this.snap; }
  async audio() { this.log.push('audio'); return this.file; }
  async publishing(on: boolean) { this.log.push(`publishing:${on}`); this.snap = { ...this.snap, publishing: on }; return this.snap; }
  async done() { this.log.push('done'); this.snap = { ...IDLE }; return this.snap; }
  async steerMute(h: string) { this.steered.push(h); }
  async mirror(k: { ids: string[]; handle: string; muted: string[] }) { this.mirrored.push({ ids: [...k.ids], handle: k.handle, muted: [...k.muted] }); }
  async legacy() { this.legacyCalls++; return this.legacyAnswer; }
}

const files = new Map<string, Uint8Array>();
const storage = { fs: { readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;

type Seen = { m: string; p: string; body?: Uint8Array; q?: Record<string, string | number> };
function netOf(handler: (s: Seen) => { status: number; json?: unknown; bytes?: Uint8Array } | Promise<{ status: number; json?: unknown; bytes?: Uint8Array }>, seen: Seen[] = []): Net {
  return async (m, p, o) => { const s = { m, p, body: o?.body, q: o?.query }; seen.push(s); const r = await handler(s); return { status: r.status, json: r.json ?? null, bytes: r.bytes ?? new Uint8Array() }; };
}
const happy = (s: Seen) => (s.m === 'POST' ? { status: 201, json: publishFx } : s.p.endsWith('/record') ? { status: 200, json: recordFx } : { status: 200, json: { takes: [] } });

function setup(handler: Parameters<typeof netOf>[0] = happy) {
  files.clear();
  const dev = new FakeDevice(), seen: Seen[] = [], o = newOverlay();
  const env: Env = { device: dev, storage, net: netOf(handler, seen) };
  return { dev, seen, o, env };
}
const posts = (seen: Seen[]): number => seen.filter((s) => s.m === 'POST').length;
const text = (b: Uint8Array | undefined): string => new TextDecoder('latin1').decode(b ?? new Uint8Array());
async function recorded(h: ReturnType<typeof setup>): Promise<void> { await act(h.env, h.o, 'record', ''); await act(h.env, h.o, 'stop', ''); }

// ---------------------------------------------------------------------------------------------------------------- S1

describe('S1 every end without a recording says so', () => {
  const cases: [Native['ended'], string, string][] = [
    ['mic', '', 'The microphone is not allowed. Allow it in Settings to record a take.'],
    ['gate', 'another app is using the audio', 'Could not start: another app is using the audio'],
    ['open', '', 'Could not record: the microphone could not be opened.'],
  ];
  for (const [ended, detail, words] of cases) {
    test(`record: ${ended}`, async () => {
      const h = setup(); h.dev.refuse = ended; h.dev.detail = detail;
      const p = await act(h.env, h.o, 'record', '');
      expect(p.state).toBe('idle'); expect(p.canRecord).toBe(true); expect(p.notice).toBe(words);
    });
  }
  test('under 3 s is not kept', async () => {
    const h = setup(); h.dev.seconds = 1.5;
    const p = await act(h.env, h.o, 'record', ''); expect(p.state).toBe('recording');
    const q = await act(h.env, h.o, 'stop', '');
    expect(q.state).toBe('idle'); expect(q.notice).toBe('A take is at least 3 seconds. Nothing was kept.');
  });
  test('nothing recorded, and too large', async () => {
    const a = setup(); await a.dev.start(limits()); a.dev.snap = { ...IDLE, ended: 'empty' };
    expect((await pageFor(a.env, a.o, a.dev.snap)).notice).toBe('Nothing was recorded.');
    const b = setup(); b.dev.file = new Uint8Array(BOUNDS.maxBytes + 1);
    await act(b.env, b.o, 'record', ''); expect((await act(b.env, b.o, 'stop', '')).notice).toBe('That take is too large to publish. Record a shorter one.');
  });
  test('the ceiling stops it and says so; the take is kept for review', async () => {
    const h = setup(); await act(h.env, h.o, 'record', '');
    await h.dev.stop(true);         // the recorder's own ceiling stop: nobody pressed Stop
    const p = await pageFor(h.env, h.o, h.dev.snap);
    expect(p.state).toBe('review'); expect(p.notice).toBe('Stopped at 60 seconds, the longest a take can be.'); expect(p.canPublish).toBe(true);
  });
  test('a blank transcript cannot be published and says to record it again', async () => {
    const h = setup(); h.dev.words = '  ';
    await recorded(h);
    const p = await pageFor(h.env, h.o, h.dev.snap);
    expect(p.state).toBe('review'); expect(p.canPublish).toBe(false); expect(p.notice).toContain('Record it again');
    await act(h.env, h.o, 'publish', ''); expect(h.seen).toEqual([]); expect(h.dev.log).not.toContain('audio');
  });
});

// ---------------------------------------------------------------------------------------------------------------- S2

describe('S2 the page follows the recorder', () => {
  test('idle, recording, review, and the on-air Take control starts the same flow citing the story on air', async () => {
    const h = setup();
    let p = await pageFor(h.env, h.o, h.dev.snap);
    expect(p).toMatchObject({ state: 'idle', canRecord: true, canStop: false, canPublish: false, canDiscard: false, limit: '1:00', elapsed: '0:00', respondsTo: '' });
    h.dev.responds = { title: 'Harbor fees rise', route: 'harbor-fees' };
    await act(h.env, h.o, 'record', '');
    h.dev.snap = { ...h.dev.snap, elapsed: 12 };
    p = await pageFor(h.env, h.o, h.dev.snap);
    expect(p).toMatchObject({ state: 'recording', canRecord: false, canStop: true, elapsed: '0:12', fraction: 0.2, respondsTo: 'Answering: Harbor fees rise' });
    await act(h.env, h.o, 'stop', '');
    p = await pageFor(h.env, h.o, h.dev.snap);
    expect(p).toMatchObject({ state: 'review', transcript: 'A neighbour speaking.', canPublish: true, canDiscard: true, publishLine: BOUNDS.publishLine, rightsLine: BOUNDS.rightsLine, respondsTo: 'Answering: Harbor fees rise' });
    await act(h.env, h.o, 'discard', '');
    h.dev.responds = { title: '', route: '' };
    await act(h.env, h.o, 'record', '');
    expect((await pageFor(h.env, h.o, h.dev.snap)).respondsTo).toBe('');
  });
  test('transcribing shows a wait state with nothing to press', async () => {
    const h = setup(); h.dev.snap = { ...IDLE, phase: 'transcribing', elapsed: 10 };
    expect(await pageFor(h.env, h.o, h.dev.snap)).toMatchObject({ state: 'transcribing', canRecord: false, canStop: false, canPublish: false, canDiscard: false });
  });
  test('the native answer is read defensively', () => {
    expect(parseNative(null)).toEqual(IDLE);
    expect(parseNative({ phase: 'bogus', elapsed: 'x', ended: 'what' })).toEqual(IDLE);
    expect(parseNative({ phase: 'review', transcript: 'hi', elapsed: 4, seconds: 4, previewing: true, ended: 'ceiling', respondsTitle: 'T', respondsRoute: 'r' }))
      .toMatchObject({ phase: 'review', transcript: 'hi', previewing: true, ended: 'ceiling', respondsRoute: 'r' });
  });
});

// ---------------------------------------------------------------------------------------------------------------- S3 and S4

describe('S3 publish', () => {
  test('one multipart POST, then the list; the id and handle are remembered; the file is dropped and the radio given back once', async () => {
    const h = setup(); h.dev.responds = { title: 'Harbor fees rise', route: 'harbor-fees' };
    await recorded(h);
    const p = await act(h.env, h.o, 'publish', '');
    expect(h.seen.map((s) => `${s.m} ${s.p}`).slice(0, 2)).toEqual(['POST /api/platform/takes', 'GET /api/platform/takes']);
    expect(posts(h.seen)).toBe(1);
    const raw = text(h.seen[0].body);
    expect(raw).toContain('harbor-fees'); expect(raw).toContain('name="duration_seconds"\r\n\r\n10.00'); expect(raw).toContain('name="transcript"\r\n\r\nA neighbour speaking.');
    expect(p).toMatchObject({ state: 'idle', justPublished: 'Published as voice-me0001.', handle: 'voice-me0001' });
    expect(h.dev.log.filter((x) => x === 'done')).toHaveLength(1);
    const kept = await loadKept(storage);
    expect(kept.ids).toContain('take-00000000000000b6'); expect(kept.handle).toBe('voice-me0001');
    expect(h.dev.mirrored.at(-1)).toMatchObject({ handle: 'voice-me0001' });
  });
  test('the claim is what the file says, clamped: a header of 60.3 claims exactly 60', async () => {
    const h = setup(); h.dev.file = mp4(60.3); h.dev.seconds = 59.5;
    await recorded(h); await act(h.env, h.o, 'publish', '');
    expect(text(h.seen[0].body)).toContain('name="duration_seconds"\r\n\r\n60.00\r\n');
  });
  test('a file with no readable header claims the recorder\'s own seconds, at least 3', async () => {
    const h = setup(); h.dev.file = new Uint8Array(500).fill(1); h.dev.seconds = 8;
    await recorded(h); await act(h.env, h.o, 'publish', '');
    expect(text(h.seen[0].body)).toContain('name="duration_seconds"\r\n\r\n8.00\r\n');
  });
  test('a signed-out phone is told to sign in and nothing leaves it', async () => {
    const h = setup(() => { throw new SignedOutError('not signed in'); });
    await recorded(h);
    const p = await act(h.env, h.o, 'publish', '');
    expect(p.notice).toBe('Sign in to publish a take.'); expect(p.state).toBe('review'); expect(h.dev.log).not.toContain('done'); expect(h.dev.log).not.toContain('discard');
  });
  test('nothing publishes without a take in review', async () => {
    const h = setup(); await act(h.env, h.o, 'publish', '');
    expect(h.seen).toEqual([]);
  });
});

describe('S4 a failure keeps the recording', () => {
  test('a 500 keeps it; the retry posts again and then publishes', async () => {
    let fail = true;
    const h = setup((s) => (s.m === 'POST' ? (fail ? { status: 500 } : { status: 201, json: publishFx }) : { status: 200, json: { takes: [] } }));
    await recorded(h);
    const p = await act(h.env, h.o, 'publish', '');
    expect(p.state).toBe('review'); expect(p.notice).toContain('The server refused the take (HTTP 500).'); expect(p.notice).toContain('kept');
    expect(h.dev.log).not.toContain('done'); expect(h.dev.snap.phase).toBe('review'); expect(p.transcript).not.toBe('');
    fail = false; const q = await act(h.env, h.o, 'publish', '');
    expect(posts(h.seen)).toBe(2); expect(q.state).toBe('idle'); expect(q.handle).toBe('voice-me0001');
  });
  test('a server without takes keeps the take and says so; refresh says it too', async () => {
    const h = setup(() => ({ status: 404 }));
    await recorded(h);
    const p = await act(h.env, h.o, 'publish', '');
    expect(p.state).toBe('review'); expect(p.notice).toBe('Takes are not available yet. Your recording is kept; publish it again later.'); expect(p.transcript).not.toBe('');
    expect((await act(h.env, h.o, 'refresh', '')).notice).toBe('Takes are not available yet.');
  });
  test('the daily limit is shown in the server\'s words', async () => {
    const h = setup(() => ({ status: 429, json: refused }));
    await recorded(h);
    const p = await act(h.env, h.o, 'publish', '');
    expect(p.notice.startsWith('invalid take: 10 takes in the last 24 hours, the limit is 10')).toBe(true); expect(p.state).toBe('review');
  });
  test('a take over the byte bound is refused before the network', async () => {
    const h = setup(); h.dev.file = new Uint8Array(BOUNDS.maxBytes + 1); h.dev.snap = { ...IDLE, phase: 'review', transcript: 'x', seconds: 9 };
    const p = await act(h.env, h.o, 'publish', '');
    expect(h.seen).toEqual([]); expect(p.notice).toContain('over 512 KB'); expect(p.state).toBe('review');
  });
  test('a recording that is gone says so and stays', async () => {
    const h = setup(); await recorded(h); h.dev.file = null;
    const p = await act(h.env, h.o, 'publish', '');
    expect(p.notice).toContain('the recording is gone'); expect(h.seen).toEqual([]);
  });
  test('a second Publish while one is in flight posts nothing', async () => {
    let release!: () => void; const gate = new Promise<void>((r) => { release = r; });
    const h = setup(async (s) => { if (s.m === 'POST') { await gate; return { status: 201, json: publishFx }; } return { status: 200, json: { takes: [] } }; });
    await recorded(h);
    const first = act(h.env, h.o, 'publish', '');
    await new Promise((r) => setTimeout(r, 5));
    expect((await pageFor(h.env, h.o, h.dev.snap)).state).toBe('publishing');
    await act(h.env, h.o, 'publish', '');
    release(); await first;
    expect(posts(h.seen)).toBe(1); expect(h.dev.log.filter((x) => x === 'done')).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------------------------------------------- S5 to S8

describe('S5 the preview', () => {
  test('toggles through the recorder; Publish stops it first', async () => {
    const h = setup(); await recorded(h);
    await act(h.env, h.o, 'preview', ''); expect((await pageFor(h.env, h.o, h.dev.snap)).previewing).toBe(true);
    await act(h.env, h.o, 'preview', ''); expect((await pageFor(h.env, h.o, h.dev.snap)).previewing).toBe(false);
    await act(h.env, h.o, 'preview', ''); await act(h.env, h.o, 'publish', '');
    expect(h.dev.log).toContain('preview:false'); expect(h.dev.snap.previewing).toBe(false);
    expect(h.dev.log.indexOf('preview:false')).toBeLessThan(h.dev.log.indexOf('audio'));
  });
});

describe('S6 my takes', () => {
  test('refresh reads the listener\'s takes and record; delete drops the row and the own id', async () => {
    const h = setup((s) => {
      if (s.m === 'DELETE') return { status: 204 };
      if (s.p.endsWith('/record')) return { status: 200, json: recordFx };
      return { status: 200, json: s.q?.mine ? mineFx : { takes: [] } };
    });
    let p = await act(h.env, h.o, 'refresh', '');
    expect(p.mine.map((r) => r.id)).toEqual(['take-00000000000000a5']); expect(p.mine[0]).toMatchObject({ state: 'Published', length: '9 sec' });
    expect(p).toMatchObject({ handle: 'voice-me0001', record: 'heard through by 7 of 11 listeners over 5 takes' });
    expect(h.seen.find((s) => s.p.endsWith('/record'))?.q).toEqual({ publisher: 'voice-me0001' });
    p = await act(h.env, h.o, 'delete', 'take-00000000000000a5');
    expect(h.seen.map((s) => `${s.m} ${s.p}`)).toContain('DELETE /api/platform/takes/take-00000000000000a5');
    expect(p.mine).toEqual([]); expect((await loadKept(storage)).ids).not.toContain('take-00000000000000a5');
  });
  test('a failed delete says so and keeps the row', async () => {
    const h = setup((s) => { if (s.m === 'DELETE') throw new Error('connection reset'); return { status: 200, json: s.q?.mine ? mineFx : { takes: [] } }; });
    await act(h.env, h.o, 'refresh', '');
    const p = await act(h.env, h.o, 'delete', 'take-00000000000000a5');
    expect(p.notice).toBe('Could not delete: the connection failed.'); expect(p.mine).toHaveLength(1);
  });
  test('a signed-out refresh changes nothing and says nothing', async () => {
    const h = setup(() => { throw new SignedOutError('not signed in'); });
    const p = await act(h.env, h.o, 'refresh', '');
    expect(p.mine).toEqual([]); expect(p.notice).toBe('');
  });
});

describe('S7 mutes', () => {
  test('kept, mirrored to the native programme, sent to the learning engine once; twice is one; unmute undoes it', async () => {
    const h = setup();
    let p = await act(h.env, h.o, 'mute', 'voice-3fa2c1');
    expect(p.muted).toEqual([{ handle: 'voice-3fa2c1', line: 'Never play takes by voice-3fa2c1.' }]);
    await act(h.env, h.o, 'mute', 'voice-3fa2c1'); await act(h.env, h.o, 'mute', 'bad handle');
    expect(h.dev.steered).toEqual(['voice-3fa2c1']);
    expect((await loadKept(storage)).muted).toEqual(['voice-3fa2c1']); expect(h.dev.mirrored.at(-1)?.muted).toEqual(['voice-3fa2c1']);
    p = await act(h.env, h.o, 'unmute', 'voice-3fa2c1');
    expect(p.muted).toEqual([]); expect((await loadKept(storage)).muted).toEqual([]); expect(h.dev.mirrored.at(-1)?.muted).toEqual([]);
    expect(h.dev.steered).toEqual(['voice-3fa2c1']);
  });
  test('a relaunch (a new overlay) still lists the mute', async () => {
    const h = setup(); await act(h.env, h.o, 'mute', 'voice-3fa2c1');
    expect((await pageFor(h.env, newOverlay(), IDLE)).muted).toHaveLength(1);
  });
});

describe('S8 record and discard', () => {
  test('Record sends the bounds the spec sets; Discard drops the take and clears the words; rerecord from review starts again', async () => {
    const h = setup(); await act(h.env, h.o, 'record', '');
    expect(h.dev.limits).toEqual({ minSeconds: 3, maxSeconds: 60 - BOUNDS.stopEarly, maxBytes: 512 * 1024 }); expect(limits()).toEqual(h.dev.limits);
    await act(h.env, h.o, 'stop', '');
    await act(h.env, h.o, 'rerecord', ''); expect(h.dev.snap.phase).toBe('recording'); expect(h.dev.log.filter((x) => x === 'start')).toHaveLength(2);
    await act(h.env, h.o, 'stop', '');
    const p = await act(h.env, h.o, 'discard', '');
    expect(p).toMatchObject({ state: 'idle', notice: '', transcript: '' }); expect(h.dev.log).toContain('discard'); expect(h.seen).toEqual([]);
  });
  test('Record clears the last "published as" line and the old words', async () => {
    const h = setup(); await recorded(h); await act(h.env, h.o, 'publish', '');
    expect((await pageFor(h.env, h.o, h.dev.snap)).justPublished).toBe('Published as voice-me0001.');
    expect((await act(h.env, h.o, 'record', '')).justPublished).toBe('');
  });
  test('a phone with no recorder says so; the rest of the page still works', async () => {
    const h = setup(); const env = { ...h.env, device: null };
    const p = await act(env, h.o, 'record', '');
    expect(p.state).toBe('idle'); expect(p.notice).toBe('Recording is not available on this phone yet.');
    expect((await act(env, h.o, 'mute', 'voice-1')).muted).toHaveLength(1);
  });
  test('an unknown op changes nothing', async () => {
    const h = setup(); const p = await act(h.env, h.o, 'nonsense', '');
    expect(p.state).toBe('idle'); expect(h.dev.log).toEqual([]);
  });
});

// ---------------------------------------------------------------------------------------------------------------- U1 to U3

describe('U1 the carry-over from the Swift files', () => {
  const legacy = { found: true, ids: ['take-old-1', 'take-old-2'], handle: 'voice-old0001', muted: ['voice-9b0e44', 'bad handle'] };
  test('own ids, handle and mutes are carried once, unioned, marked, and mirrored back', async () => {
    const h = setup(); h.dev.legacyAnswer = legacy;
    const p = await act(h.env, h.o, 'refresh', '');
    expect(p.handle).toBe('voice-old0001'); expect(p.muted.map((m) => m.handle)).toEqual(['voice-9b0e44']);
    const kept = JSON.parse(new TextDecoder().decode(files.get(KEPT_FILE)!));
    expect(kept).toMatchObject({ ids: ['take-old-1', 'take-old-2'], handle: 'voice-old0001', muted: ['voice-9b0e44'], migrated: true });
    expect(h.dev.mirrored.at(-1)).toMatchObject({ ids: ['take-old-1', 'take-old-2'], muted: ['voice-9b0e44'] });
    await act(h.env, h.o, 'refresh', ''); await act(h.env, newOverlay(), 'refresh', '');
    expect(h.dev.legacyCalls).toBe(1);         // the second overlay found the marker in the file
  });
  test('what is already in app:/data/takes.json is kept and the old voices are added to it', async () => {
    const h = setup(); h.dev.legacyAnswer = legacy;
    files.set(KEPT_FILE, new TextEncoder().encode(JSON.stringify({ ids: ['take-new-1'], handle: 'voice-new0001', muted: ['voice-new-mute'] })));
    await act(h.env, h.o, 'refresh', '');
    const k = await loadKept(storage);
    expect(k.ids.sort()).toEqual(['take-new-1', 'take-old-1', 'take-old-2']); expect(k.handle).toBe('voice-new0001'); expect(k.muted.sort()).toEqual(['voice-9b0e44', 'voice-new-mute']);
  });
  test('nothing was found: only the marker is written; no answer: not marked, asked again next time', async () => {
    const a = setup(); a.dev.legacyAnswer = { found: false, ids: [], handle: '', muted: [] };
    await act(a.env, a.o, 'refresh', ''); expect((await loadKept(storage)).migrated).toBe(true);
    const b = setup(); b.dev.legacyAnswer = null;
    await act(b.env, b.o, 'refresh', ''); expect(files.has(KEPT_FILE)).toBe(false);
    b.dev.legacyAnswer = legacy; await act(b.env, b.o, 'refresh', ''); expect((await loadKept(storage)).ids).toContain('take-old-1');
  });
  test('the marker is not written when the write failed', async () => {
    const h = setup(); h.dev.legacyAnswer = legacy;
    const bad = { fs: { readFile: async () => { throw new Error('ENOENT'); }, atomicWriteFile: async () => { throw new Error('disk full'); }, rm: async () => {} } } as never;
    const env = { ...h.env, storage: bad };
    await act(env, h.o, 'refresh', ''); await act(env, h.o, 'refresh', '');
    expect(h.dev.legacyCalls).toBe(2);        // not carried: asked again
    expect(h.dev.mirrored).toEqual([]);       // and the native copy was not touched
  });
  test('nothing found: the old voices are not mirrored back (nothing is written to the native copy)', async () => {
    const h = setup(); h.dev.legacyAnswer = { found: false, ids: [], handle: '', muted: [] };
    await act(h.env, h.o, 'refresh', '');
    expect(h.dev.mirrored).toEqual([]);
  });
});

describe('U2 an action before the carry-over has finished cannot lose the old voices', () => {
  test('mute, then publish, then delete each carry first', async () => {
    const h = setup((s) => (s.m === 'DELETE' ? { status: 204 } : happy(s))); h.dev.legacyAnswer = { found: true, ids: ['take-old-1'], handle: 'voice-old0001', muted: ['voice-old-mute'] };
    await act(h.env, h.o, 'mute', 'voice-3fa2c1');
    expect((await loadKept(storage)).muted.sort()).toEqual(['voice-3fa2c1', 'voice-old-mute']);
    expect(h.dev.mirrored.every((m) => m.muted.includes('voice-old-mute'))).toBe(true);
    const g = setup(); g.dev.legacyAnswer = { found: true, ids: ['take-old-1'], handle: 'voice-old0001', muted: ['voice-old-mute'] };
    await recorded(g); await act(g.env, g.o, 'publish', '');
    const k = await loadKept(storage);
    expect(k.ids.sort()).toEqual(['take-00000000000000b6', 'take-old-1']); expect(k.muted).toEqual(['voice-old-mute']);
    await act(g.env, g.o, 'delete', 'take-00000000000000b6');
    expect((await loadKept(storage)).ids).toEqual(['take-old-1']);
  });
});

describe('U3 a take in review is the recorder\'s', () => {
  test('a new session (shared code restarted) still shows the take and can publish it once', async () => {
    const h = setup(); await recorded(h);
    const fresh = newOverlay();
    const p = await pageFor(h.env, fresh, h.dev.snap);
    expect(p).toMatchObject({ state: 'review', canPublish: true, transcript: 'A neighbour speaking.' });
    await act(h.env, fresh, 'publish', '');
    expect(posts(h.seen)).toBe(1); expect(h.dev.log.filter((x) => x === 'done')).toHaveLength(1);
    await act(h.env, fresh, 'publish', ''); expect(posts(h.seen)).toBe(1);
  });
  test('a failed publish never discards or drops the take, whatever the failure', async () => {
    for (const handler of [() => ({ status: 500 }), () => ({ status: 404 }), () => ({ status: 429, json: refused }), () => { throw new Error('offline'); }]) {
      const h = setup(handler as never); await recorded(h);
      await act(h.env, h.o, 'publish', '');
      expect(h.dev.log).not.toContain('done'); expect(h.dev.log).not.toContain('discard'); expect(h.dev.snap.phase).toBe('review'); expect((await pageFor(h.env, h.o, h.dev.snap)).state).toBe('review');
    }
  });
  test('the client is the shared one: the same wire as takes.ts', async () => {
    const seen: Seen[] = []; const c = client(netOf(happy, seen));
    expect((await c.list()).length).toBe(0); expect(emptyKept().migrated).toBe(false);
  });
});

// ---------------------------------------------------------------------------------------------------------------- review 1 (B1, B2, S1 to S4)

describe('B1 while a publish is in flight the recorder is not touched', () => {
  async function inflight() {
    let release!: () => void; const gate = new Promise<void>((r) => { release = r; });
    const h = setup(async (s) => { if (s.m === 'POST') { await gate; return { status: 201, json: publishFx }; } return { status: 200, json: { takes: [] } }; });
    await recorded(h);
    const first = act(h.env, h.o, 'publish', '');
    await new Promise((r) => setTimeout(r, 5));
    return { h, release, first };
  }
  test('the recorder is told a publish is under way, so the page is asked again and shows it', async () => {
    const { h, release, first } = await inflight();
    expect(h.dev.snap.publishing).toBe(true); expect((await pageFor(h.env, h.o, h.dev.snap))).toMatchObject({ state: 'publishing', canDiscard: false, canRecord: false, canStop: false, canPublish: false });
    release(); await first; expect(h.dev.snap.publishing).toBe(false);
  });
  for (const op of ['discard', 'record', 'rerecord', 'stop', 'preview']) {
    test(`${op} is refused: the take is published, not discarded, and nothing else is cancelled`, async () => {
      const { h, release, first } = await inflight();
      const before = h.dev.log.length;
      const p = await act(h.env, h.o, op, '');
      expect(h.dev.log.slice(before).filter((x) => ['discard', 'start', 'stop'].includes(x) || x.startsWith('preview'))).toEqual([]);
      expect(p.state).toBe('publishing');
      release(); const done = await first;
      expect(posts(h.seen)).toBe(1); expect(done.justPublished).toBe('Published as voice-me0001.'); expect(h.dev.log.filter((x) => x === 'done')).toHaveLength(1);
      expect(h.dev.log).not.toContain('discard');
    });
  }
  test('a failed publish clears the flag and the take can be discarded afterwards', async () => {
    const h = setup(() => ({ status: 500 })); await recorded(h);
    await act(h.env, h.o, 'publish', ''); expect(h.dev.snap.publishing).toBe(false);
    expect((await act(h.env, h.o, 'discard', '')).state).toBe('idle');
  });
  test('the other acts still run during a send: delete, mute, unmute, refresh', async () => {
    const { h, release, first } = await inflight();
    expect((await act(h.env, h.o, 'mute', 'voice-3fa2c1')).muted).toHaveLength(1);
    expect((await act(h.env, h.o, 'unmute', 'voice-3fa2c1')).muted).toHaveLength(0);
    release(); await first;
  });
});

describe('S4 two publishes started in the same window post once', () => {
  test('both pass no await before the flag', async () => {
    const h = setup(); await recorded(h);
    await Promise.all([act(h.env, h.o, 'publish', ''), act(h.env, h.o, 'publish', '')]);
    expect(posts(h.seen)).toBe(1); expect(h.dev.log.filter((x) => x === 'done')).toHaveLength(1);
  });
  test('a recorder that fails to drop the file after a publish does not make the take publishable twice in this session', async () => {
    const h = setup(); await recorded(h); h.dev.done = async () => { throw new Error('native gone'); };
    const p = await act(h.env, h.o, 'publish', '');
    expect(p.justPublished).toBe('Published as voice-me0001.'); expect(posts(h.seen)).toBe(1);
  });
});

describe('B2 a new account does not see or rewrite the old account\'s takes data', () => {
  test('wipe, render, mute: only the new account\'s voice is kept and mirrored; the old files cannot come back', async () => {
    const h = setup(() => ({ status: 200, json: { takes: [] } }));
    await act(h.env, h.o, 'mute', 'voice-OLD');
    expect((await pageFor(h.env, h.o, IDLE)).muted).toHaveLength(1);
    // sign-out: the generation moves, the shared file goes (account.ts wipeKept), the old account's native copy may still be there
    const asked = h.dev.legacyCalls;
    nextGeneration(); await wipeKept(storage);
    h.dev.legacyAnswer = { found: true, ids: ['take-OLD'], handle: 'voice-OLDH', muted: ['voice-OLD'] };
    const page = await pageFor(h.env, h.o, IDLE);
    expect(page.muted).toEqual([]); expect(page.handle).toBe(''); expect(page.mine).toEqual([]); expect(page.record).toBe('');
    const p = await act(h.env, h.o, 'mute', 'voice-NEW');
    expect(p.muted.map((m) => m.handle)).toEqual(['voice-NEW']);
    const k = await loadKept(storage);
    expect(k.muted).toEqual(['voice-NEW']); expect(k.ids).toEqual([]); expect(k.handle).toBe('');
    expect(h.dev.mirrored.at(-1)?.muted).toEqual(['voice-NEW']);
    expect(h.dev.legacyCalls).toBe(asked);          // the old files were never asked for again
    // the native copy of the old account is emptied as soon as the change is seen, even if the native wipe had failed
    expect(h.dev.mirrored.some((m) => m.muted.length === 0 && m.ids.length === 0 && m.handle === '')).toBe(true);
  });
  test('the listener\'s takes and record of the old account are dropped too', async () => {
    const h = setup((s) => (s.p.endsWith('/record') ? { status: 200, json: recordFx } : { status: 200, json: s.q?.mine ? mineFx : { takes: [] } }));
    const p = await act(h.env, h.o, 'refresh', ''); expect(p.mine).toHaveLength(1); expect(p.record).not.toBe('');
    nextGeneration(); await wipeKept(storage);
    const q = await pageFor(h.env, h.o, IDLE);
    expect(q.mine).toEqual([]); expect(q.record).toBe(''); expect(q.handle).toBe(''); expect(q.notice).toBe('');
  });
  test('a notice and a "published as" line of the old account are dropped', async () => {
    const h = setup(); await recorded(h); await act(h.env, h.o, 'publish', '');
    expect((await pageFor(h.env, h.o, IDLE)).justPublished).not.toBe('');
    nextGeneration(); expect((await pageFor(h.env, h.o, IDLE)).justPublished).toBe('');
  });
});

describe('S1 and S3 delete and refresh', () => {
  test('a delete the server refused says so and keeps the row and the own id', async () => {
    for (const status of [500, 403, 429]) {
      const h = setup((s) => (s.m === 'DELETE' ? { status } : { status: 200, json: s.q?.mine ? mineFx : { takes: [] } }));
      await act(h.env, h.o, 'refresh', '');
      const p = await act(h.env, h.o, 'delete', 'take-00000000000000a5');
      expect(p.notice).toContain(`Could not delete: The server refused the delete (HTTP ${status}).`); expect(p.mine).toHaveLength(1); expect((await loadKept(storage)).ids).toContain('take-00000000000000a5');
    }
  });
  test('404 and 409 are "already gone": the row goes', async () => {
    for (const status of [404, 409]) {
      const h = setup((s) => (s.m === 'DELETE' ? { status } : { status: 200, json: s.q?.mine ? mineFx : { takes: [] } }));
      await act(h.env, h.o, 'refresh', '');
      expect((await act(h.env, h.o, 'delete', 'take-00000000000000a5')).mine).toEqual([]);
    }
  });
  test('a refresh that works clears an old "not available yet"', async () => {
    let up = false;
    const h = setup((s) => (up ? (s.m === 'POST' ? { status: 201, json: publishFx } : { status: 200, json: { takes: [] } }) : { status: 404 }));
    await recorded(h); expect((await act(h.env, h.o, 'publish', '')).notice).toContain('not available yet');
    up = true; expect((await act(h.env, h.o, 'refresh', '')).notice).toBe('');
  });
});
