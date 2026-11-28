// The Autoradio page's source under bun test (radio-page.ts): the native facts and the bar's facts in, the page out. The page's model (status lines, rows, greeting) is radio-model.ts,
// tested in radio.test.ts; this file tests what the source adds. The failure list, written first:
//  R1 the sheet arrives as the store's wire JSON: it is read whole; empty, broken or mistyped is an empty ready page, never a throw.
//  R2 the clock is the facts' own `at` (a data source may not read time); the greeting and the date follow it, in the listener's zone.
//  R3 listening comes from the BAR's state: a turn open dims the page and shows the words; an error held on the bar beats it (the bar says error, not listening).
//  R4 playing is the run's (the deck plays the entry on air); the bar's facts no longer say what plays.
//  R5 remote audio shows its own clock labels from seconds, "-" before the remaining; no duration is no remaining.
//  R6 "published" is when the story on air was updated, as words, from the facts' time; none known is "".
//  R7 the versions of stories the phone holds come as two lists; a cite of another version is noted on its row; lists of different length are ignored whole.
//  R8 a take on air shows its transcript and record from the facts; another entry shows none.
//  R10 an on-air entry with no title (a play that dropped it) shows the headline of the programme on air: title, small line and built-on label, never the kind word or a route.
//  R9 stale ahead entries are noted; the desk's words (writing, notice, dry, wants) pass to the page.
import { describe, expect, test } from 'bun:test';
import { radioPage, EMPTY_FACTS } from './radio-page.ts';
import type { Entry } from './runsheet-model.ts';

const NOW = 1_700_000_000;   // seconds: Tue 14 Nov 2023 22:13 UTC
const entry = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `Title ${id}`, est_seconds: 60, revision: 0, text: 'Words.', ...o } as Entry);
const sheet = (entries: Entry[]) => JSON.stringify({ version: 3, on_air: entries.find((e) => e.state === 'on_air')?.id, position_seconds: 0, entries, master_version: 0, sheet_epoch: 0 });
const facts = (entries: Entry[], o: Record<string, unknown> = {}) => ({ ...EMPTY_FACTS, at: NOW, sheet: sheet(entries), ...o });
const bar = (o: Record<string, unknown> = {}) => ({ phase: 'playing', transcript: '', blockCount: 0, running: false, playing: false, title: '', message: '', onAir: true, ...o });
const none = { entries: [] };
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
      expect(p.dateLine).toBe(''); expect(p.greeting).toBe(''); expect(p.published).toBe('');   // no clock is no date, never 1970
    }
    expect(radioPage(facts([air]), undefined, undefined, 0).onAir).toBe(true);
  });
});

describe('no clock', () => {
  test('R2b a non-positive `at` is no clock: no date, no greeting, no relative time, even with rows ahead and a published time', () => {
    for (const at of [0, -5, NaN, undefined]) {
      const p = radioPage({ ...facts([entry('a'), entry('b')], { publishedAt: 5000 }), at }, bar(), none, 0);
      expect(p.dateLine).toBe(''); expect(p.greeting).toBe(''); expect(p.published).toBe('');
      expect(p.ahead.length).toBe(2);
    }
    const onAir = radioPage({ ...facts([air], { publishedAt: 5000 }), at: 0 }, bar(), none, 0);
    expect(onAir.published).toBe(''); expect(onAir.dateLine).toBe('');
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
  test('R4 playing is the run\'s, not the bar\'s', () => {
    expect(radioPage(facts([air], { playing: true }), bar(), none, 0).playing).toBe(true);
    expect(radioPage(facts([air], { playing: false }), bar(), none, 0).playing).toBe(false);
    expect(radioPage(facts([air], { playing: false }), bar({ playing: true, title: 'T', blockCount: 2, running: true }), none, 0).playing).toBe(false);          // facts the bar no longer carries change nothing
    expect(radioPage(facts([entry('b')], { playing: true }), bar(), none, 0).playing).toBe(false);   // nothing on air is never playing
  });
});

describe('a story on air whose sheet entry carries no title', () => {
  // The simulator proof of 19c4c7b6b: the entry was placed by a play that dropped the title (native choirPlay passed only the route), so the sheet held title "" and label "".
  // The old page showed the headline only when the radio's own list happened to hold the story. The programme on air has the headline, so the page uses it.
  const ROUTE = '/pub/texture/anthropic-prices-its-ipo-pub1';
  const bare = (o: Partial<Entry> = {}) => entry('p', { kind: 'page', state: 'on_air', placed_by: 'listener', title: '', text: undefined, source: { ref: 'pub:' + ROUTE, label: '' }, ...o } as Partial<Entry>);
  const HEAD = 'Anthropic closes up 17% in volatile Nasdaq debut';
  test('R10 the headline of the programme on air is the title, the small line and the built-on label, never "Story" or a route', () => {
    const p = radioPage(facts([bare()], { airTitle: HEAD, airRoute: ROUTE, playing: true }), bar(), none, 0);
    expect(p.title).toBe(HEAD); expect(p.source).toBe(HEAD);
    expect(p.builtOn.map((b) => b.title)).toEqual([HEAD]);
    expect(p.title).not.toBe('Story'); expect(p.builtOn[0].title.startsWith('/pub/')).toBe(false);
  });
  test('R10 an entry that carries its own title keeps it; the programme title does not replace it', () => {
    const p = radioPage(facts([bare({ title: 'The entry title', source: { ref: 'pub:/pub/x-pub1', label: 'The entry label' } } as Partial<Entry>)], { airTitle: HEAD, airRoute: ROUTE }), bar(), none, 0);
    expect(p.title).toBe('The entry title'); expect(p.source).toBe('The entry label'); expect(p.builtOn[0].title).toBe('The entry label');
  });
  test('R10 with no programme title the kind word stays (nothing is invented), and entries ahead are never retitled', () => {
    expect(radioPage(facts([bare()], { airTitle: '', airRoute: ROUTE }), bar(), none, 0).title).toBe('Story');
    const ahead = entry('q', { kind: 'page', title: '', text: undefined, source: { ref: 'pub:/pub/y-pub2', label: '' } } as Partial<Entry>);
    expect(radioPage(facts([air, ahead], { airTitle: HEAD, airRoute: ROUTE }), bar(), none, 0).ahead[0].title).toBe('Story');
  });
  test('R10 a bare entry played while ANOTHER document is loaded is not named by that document\'s headline', () => {
    const p = radioPage(facts([bare()], { airTitle: 'The previous story headline', airRoute: '/pub/texture/the-previous-story-pub9' }), bar(), none, 0);
    expect(p.title).toBe('Story'); expect(p.builtOn[0].title).not.toBe('The previous story headline');
    expect(radioPage(facts([bare()], { airTitle: 'A headline', airRoute: '' }), bar(), none, 0).title).toBe('Story');   // no route on air: not known to be this entry's
  });
  test('R10 a take on air is not retitled by the programme', () => {
    const take = entry('t', { kind: 'take', state: 'on_air', title: '', text: undefined, source: { ref: 'take:abc', label: 'Ana' } } as Partial<Entry>);
    const p = radioPage(facts([take], { airTitle: HEAD, airRoute: ROUTE }), bar(), none, 0);
    expect(p.title).toBe('Ana'); expect(p.takePublisher).toBe('Ana');
  });
  test('R10 remote audio is not retitled by the programme', () => {
    const rem = entry('r', { kind: 'episode', state: 'on_air', title: '', text: undefined, source: { ref: 'https://x/a.mp3', label: 'Show' } } as Partial<Entry>);
    expect(radioPage(facts([rem], { airTitle: HEAD, airRoute: ROUTE, remoteAudio: true }), bar(), none, 0).title).toBe('Show');
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


describe('spoken time with no estimate (the proof showed no elapsed label on a paper story played from its page)', () => {
  const noEstimate = entry('a', { state: 'on_air', est_seconds: 0 });
  test('R11 an entry with no estimate shows the elapsed time the voice has counted and omits only the remaining time', () => {
    const p = radioPage(facts([noEstimate], { playing: true, progress: 0.2, spokenElapsed: 42 }), bar(), none, 0);
    expect(p.elapsed).toBe('0:42'); expect(p.remaining).toBe('');
  });
  test('R11 with an estimate both stay, as before; with nothing counted yet and no estimate both are empty', () => {
    const p = radioPage(facts([entry('a', { state: 'on_air', est_seconds: 60 })], { playing: true, progress: 0.5, spokenElapsed: 7 }), bar(), none, 0);
    expect(p.elapsed).toBe('0:30'); expect(p.remaining).toBe('-0:30');
    const q = radioPage(facts([noEstimate], { playing: true, spokenElapsed: 0 }), bar(), none, 0);
    expect(q.elapsed).toBe(''); expect(q.remaining).toBe('');
  });
});

describe('B2-7 the page and the bar agree on playing (review)', () => {
  test('a run that says the entry is held for the audio session or not on air is not "On air"; one that says the deck plays it is (the bar\'s facts no longer say what plays)', () => {
    const f = (o: Record<string, unknown>) => facts([air], { playing: false, ready: true, onAir: true, ...o });
    expect(radioPage(f({ waiting: true }), bar(), none, 0).state).toBe('paused');
    expect(radioPage(f({ playing: true }), bar(), none, 0).state).toBe('onair');
    expect(radioPage(f({ onAir: false }), bar(), none, 0).state).not.toBe('onair');
    expect(radioPage(f({}), bar({ playing: true, title: 'x', blockCount: 2, running: true }), none, 0).state).toBe('paused');       // a bar that still carried the old native fields cannot make the page play
  });
  test('B2-4 only a speech failure is labelled Can\'t speak; another message is shown as it is', () => {
    const p = radioPage(facts([air], { message: 'That address is too long to add.' }), bar({ playing: false }), none, 0);
    expect(p.statusLine).toBe('That address is too long to add.'); expect(p.cannotSpeak).toBe('');
    const q = radioPage(facts([air], { message: 'Can’t speak: the voice made no sound' }), bar({ playing: false }), none, 0);
    expect(q.statusLine).toBe('Can’t speak: the voice made no sound'); expect(q.cannotSpeak).toBe('the voice made no sound');
  });
});
