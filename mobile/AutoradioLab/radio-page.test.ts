// The Autoradio page's source under bun test (radio-page.ts): the native facts and the bar's facts in, the page out. The page's model (status lines, rows, greeting) is radio-model.ts,
// tested in radio.test.ts; this file tests what the source adds. The failure list, written first:
//  R1 the sheet arrives as the store's wire JSON: it is read whole; empty, broken or mistyped is an empty ready page, never a throw.
//  R2 the clock is the facts' own `at` (a data source may not read time); the greeting and the date follow it, in the listener's zone.
//  R3 listening comes from the BAR's state: a turn open dims the page and shows the words; an error held on the bar beats it (the bar says error, not listening).
//  R4 playing is the player's, or the bar's: either says the radio plays.
//  R5 remote audio shows its own clock labels from seconds, "-" before the remaining; no duration is no remaining.
//  R6 "published" is when the story on air was updated, as words, from the facts' time; none known is "".
//  R7 the versions of stories the phone holds come as two lists; a cite of another version is noted on its row; lists of different length are ignored whole.
//  R8 a take on air shows its transcript and record from the facts; another entry shows none.
//  R9 stale ahead entries are noted; the desk's words (writing, notice, dry, wants) pass to the page.
import { describe, expect, test } from 'bun:test';
import { radioPage, EMPTY_FACTS } from './radio-page.ts';
import type { Entry } from './runsheet-model.ts';

const NOW = 1_700_000_000;   // seconds: Tue 14 Nov 2023 22:13 UTC
const entry = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `Title ${id}`, est_seconds: 60, revision: 0, text: 'Words.', ...o } as Entry);
const sheet = (entries: Entry[]) => JSON.stringify({ version: 3, on_air: entries.find((e) => e.state === 'on_air')?.id, position_seconds: 0, entries, master_version: 0, sheet_epoch: 0 });
const facts = (entries: Entry[], o: Record<string, unknown> = {}) => ({ ...EMPTY_FACTS, at: NOW, sheet: sheet(entries), ...o });
const bar = (o: Record<string, unknown> = {}) => ({ phase: 'playing', transcript: '', blockCount: 0, running: false, playing: false, title: '', message: '', onAir: true, ...o });
const none = { entries: [], dismissed: '' };
const air = entry('a', { state: 'on_air' });

describe('the sheet', () => {
  test('R1 the wire JSON is read whole', () => {
    const p = radioPage(facts([air, entry('b'), entry('c')]), bar(), none, 0);
    expect(p.onAir).toBe(true); expect(p.title).toBe('Title a'); expect(p.ahead.map((r) => r.id)).toEqual(['b', 'c']);
  });
  test('R1 empty, broken and mistyped facts are an empty ready page', () => {
    for (const f of [undefined, null, 'x', {}, { sheet: '' }, { sheet: 'not json' }, { sheet: '[1]' }, { sheet: '{"entries":5}' }, { sheet: 4 }]) {
      const p = radioPage(f, bar(), none, 0);
      expect(p.state).toBe('ready'); expect(p.onAir).toBe(false); expect(p.ahead).toEqual([]); expect(p.canPlay).toBe(true);
    }
    expect(radioPage(facts([air]), undefined, undefined, 0).onAir).toBe(true);
  });
});

describe('the clock', () => {
  test('R2 the date and greeting follow the facts, in the listener zone', () => {
    const f = facts([entry('a'), entry('b')]);
    const p = radioPage(f, bar(), none, 0);
    expect(p.dateLine).toBe('Tue 14 Nov'); expect(p.greeting.startsWith('Good evening. Two things are ready')).toBe(true);
    expect(radioPage({ ...f, at: NOW + 4 * 3600 }, bar(), none, 0).greeting.startsWith('Hello.')).toBe(true);   // 02:13 UTC
    expect(radioPage(f, bar(), none, 480).dateLine).toBe('Wed 15 Nov');                                          // 06:13 at UTC+8
    expect(radioPage(f, bar(), none, 480).greeting.startsWith('Good morning.')).toBe(true);
  });
});

describe('the bar', () => {
  test('R3 a turn open dims the page and shows the words; an idle bar does not', () => {
    const f = facts([air], { playing: true });
    let p = radioPage(f, bar({ phase: 'capturing', transcript: 'tell me about the harbour' }), none, 0);
    expect(p.state).toBe('listening'); expect(p.dim).toBe(true); expect(p.heardWords).toBe('tell me about the harbour');
    p = radioPage(f, bar({ phase: 'candidate' }), none, 0);
    expect(p.state).toBe('listening'); expect(p.heardWords).toBe('');
    expect(radioPage(f, bar(), none, 0).dim).toBe(false);
  });
  test('R3 an error held on the bar beats listening', () => {
    const f = facts([air], { playing: true, at: NOW, errorAt: NOW - 1, errorStage: 'voice', errorMessage: 'x' });
    const b = bar({ phase: 'capturing', transcript: 'hi', at: NOW, errorAt: NOW - 1, errorStage: 'voice', errorMessage: 'x' });
    const p = radioPage({ ...f }, b, none, 0);
    expect(p.dim).toBe(false); expect(p.state).not.toBe('listening');
  });
  test('R4 playing is the player\'s or the bar\'s', () => {
    expect(radioPage(facts([air], { playing: true }), bar(), none, 0).playing).toBe(true);
    expect(radioPage(facts([air], { playing: false }), bar(), none, 0).playing).toBe(false);
    expect(radioPage(facts([air], { playing: false }), bar({ playing: true, title: 'T', blockCount: 2, running: true }), none, 0).playing).toBe(true);
    expect(radioPage(facts([entry('b')], { playing: true }), bar(), none, 0).playing).toBe(false);   // nothing on air is never playing
  });
});

describe('remote audio and the story on air', () => {
  const rem = entry('r', { kind: 'episode', state: 'on_air', text: undefined, source: { ref: 'https://x/a.mp3', label: 'Show' } } as Partial<Entry>);
  test('R5 remote labels come from seconds', () => {
    const p = radioPage(facts([rem], { remoteAudio: true, remoteElapsed: 72, remoteDuration: 292, playing: true }), bar(), none, 0);
    expect(p.elapsed).toBe('1:12'); expect(p.remaining).toBe('-3:40'); expect(p.sentence).toBe('');
    const q = radioPage(facts([rem], { remoteAudio: true, remoteElapsed: 72, remoteDuration: 0, playing: true }), bar(), none, 0);
    expect(q.elapsed).toBe('1:12'); expect(q.remaining).toBe('');
  });
  test('R6 published is words from the facts\' time', () => {
    const f = (at: number) => facts([air], { publishedAt: at });
    expect(radioPage(f(NOW - 7300), bar(), none, 0).published).toBe('2 h ago');
    expect(radioPage(f(NOW - 30), bar(), none, 0).published).toBe('just now');
    expect(radioPage(f(0), bar(), none, 0).published).toBe('');
  });
  test('R7 versions note a cite of another version', () => {
    const e = entry('b', { source: { ref: 'pub:harbour@v2', label: 'Harbour' } });
    const note = (f: Record<string, unknown>) => radioPage(facts([air, e], f), bar(), none, 0).ahead[0].builtOn[0]?.note;
    expect(note({ versionRoutes: ['harbour'], versionNumbers: [3] })).toBe('cited v2, now v3');
    expect(note({ versionRoutes: ['harbour'], versionNumbers: [2] })).toBe('');
    expect(note({ versionRoutes: ['harbour', 'x'], versionNumbers: [3] })).toBe('');
  });
  test('R8 a take on air shows what the facts hold of it', () => {
    const take = entry('t', { kind: 'take', state: 'on_air', text: undefined, source: { ref: 'take:abc', label: 'Ana' } } as Partial<Entry>);
    const p = radioPage(facts([take], { takeTranscript: 'what she said', takeRecord: 'her record' }), bar(), none, 0);
    expect(p).toMatchObject({ takePublisher: 'Ana', takeTranscript: 'what she said', takeRecord: 'her record' });
    expect(radioPage(facts([air], { takeTranscript: 'what she said' }), bar(), none, 0).takeTranscript).toBe('');
  });
  test('R9 stale rows and the desk\'s words pass to the page', () => {
    const p = radioPage(facts([air, entry('b'), entry('c')], { stale: ['c'] }), bar(), none, 0);
    expect(p.ahead.map((r) => r.note)).toEqual(['', 'Written for other neighbours']);
    expect(radioPage(facts([], { writing: true, wants: true }), bar(), none, 0)).toMatchObject({ state: 'writing', canPlay: false });
    expect(radioPage(facts([entry('h', { state: 'heard', ended: { outcome: 'finished', at: '', last_block: 0 } } as Partial<Entry>)], { notice: 'Cannot reach the desk.' }), bar(), none, 0).state).toBe('offline');
    expect(radioPage(facts([], { dry: true }), bar(), none, 0).emptyNote).toContain('Nothing is coming up.');
  });
});
