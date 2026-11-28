// What the bar shows of what plays, from the run sheet and the deck's state (radio-follow.ts `runOf`), the same on both phones: the failure list of the bar's facts in shared code
// (docs/design/sentence-follow-in-shared-code-2026-11-28.md, S4). Every function is pure, so the tests only pass values in. Written before the code.
//  G1  a title from another entry: the title is the sheet's on-air entry's, never the deck's or a held one (a skip, an entry that just ended).
//  G2  a stale playing state: the deck playing another item, or nothing on air, is no verdict about the entry on air.
//  G3  another entry's progress: a place for another entry, or for other text (a revision), gives progress 0, never that one's share.
//  G4  paused by the listener, paused by the system and held for the audio session each have their word; stopped, empty and held for the engine say nothing is playing.
//  G5  audio is the seconds over its length, clamped; no length is 0; a bad number is 0.
//  G6  the same arguments give the same answer (nothing is kept), and the contract lists every value it reads (the deck's state and the on-air entry).
//  G7  after a sign-out or another account's key the bar says nothing plays (S19-bar in account.test.ts holds the account side).
//  G8  a phone with no voice answers none of the voice's facts and still shows what plays.
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { EMPTY_STATE } from './deck-native.ts';
import { barInfo, forgetBar } from './bar.ts';
import { digestOf, runOf as runOfAir, type Air, type Place } from './radio-follow.ts';

const TEXT = 'First sentence here. Second one follows it. Third closes.\n\nA new paragraph begins now. It has two sentences.';
const FIRST = 'First sentence here. Second one follows it. Third closes.';
const place = (over: Partial<Place> = {}): Place => ({ entry: 'e1', digest: digestOf(TEXT), start: 0, length: FIRST.length, seconds: 0, total: 0, ...over });
const ds = (over: Record<string, unknown> = {}, p: Partial<Place> = {}) => ({ ...EMPTY_STATE, why: 'playing', item: 'e1', place: place(p), ...over });
const air = (over: Partial<Air> = {}): Air => ({ id: 'e1', address: 'pub:/p/x', handedId: 'e1', handedText: TEXT, ...over });
const spoken = { onAir: true, title: 'Harbor strike ends', audio: false };
const clip = { onAir: true, title: 'Episode 12', audio: true };
/** The run as the contract asks it: one deck state and one on-air entry (its title and audio flag in the same answer as its id). `view.onAir` false is an empty on-air entry. */
const runOf = (view: { onAir: boolean; title: string; audio: boolean }, d: unknown, a: unknown) => runOfAir(d, view.onAir ? { ...(a as object), title: view.title, audio: view.audio } : { id: '', address: '', handedId: '', handedText: '', title: '', audio: false });

describe('G1 and G2 the entry on air decides which deck state is about it', () => {
  test('the deck playing the entry on air: playing, with the sheet\'s title', () => {
    expect(runOf(spoken, ds(), air())).toMatchObject({ ready: true, onAir: true, playing: true, paused: false, waiting: false, title: 'Harbor strike ends' });
  });
  test('G2 skip: the deck is still on the old item while the sheet has the new one on air: nothing plays, and the title is the new one\'s', () => {
    expect(runOf({ ...spoken, title: 'New story' }, ds({ item: 'old' }), air({ id: 'new', handedId: 'new' }))).toMatchObject({ playing: false, paused: false, waiting: false, title: '', progress: 0 });   // never the new title with the old item's place
  });
  test('G2 the deck has moved on to the next item and the sheet has not: nothing plays', () => {
    expect(runOf(spoken, ds({ item: 'next', place: place({ entry: 'next' }) }), air())).toMatchObject({ playing: false, paused: false, progress: 0 });
  });
  test('G2 nothing on air: nothing plays, no title, whatever the deck says', () => {
    expect(runOf({ onAir: false, title: 'Left over', audio: false }, ds(), air({ id: '', handedId: '' }))).toMatchObject({ onAir: false, playing: false, paused: false, waiting: false, title: '', progress: 0 });
  });
  test('G2 an empty on-air id never matches an empty deck item', () => {
    expect(runOf(spoken, ds({ item: '' }), air({ id: '', handedId: '' }))).toMatchObject({ playing: false, paused: false });
  });
  test('the contract\'s empty forms (no answer yet) say nothing plays', () => {
    expect(runOfAir({}, {})).toMatchObject({ playing: false, paused: false, waiting: false, progress: 0 });
    expect(runOfAir(undefined, undefined)).toMatchObject({ playing: false });
  });
});

describe('G4 the deck\'s word for each state', () => {
  const why = (w: string) => runOf(spoken, ds({ why: w }), air());
  test('paused by the listener or the system is paused; held for the audio session is waiting', () => {
    expect(why('paused_listener')).toMatchObject({ playing: false, paused: true, waiting: false });
    expect(why('paused_system')).toMatchObject({ playing: false, paused: true, waiting: false });
    expect(why('held_session')).toMatchObject({ playing: false, paused: false, waiting: true });
  });
  test('stopped, empty and held for the engine say nothing plays and nothing waits', () => {
    for (const w of ['stopped', 'empty', 'held_engine']) expect(why(w)).toMatchObject({ playing: false, paused: false, waiting: false });
  });
});

describe('G3 progress is of the entry on air, from the place', () => {
  test('a spoken entry: the unit\'s start plus its share over the text handed over', () => {
    const second = TEXT.indexOf('A new paragraph begins now. It has two sentences.');
    const p = runOf(spoken, ds({}, { start: second, length: TEXT.length - second, seconds: 5, total: 10 }), air()).progress;
    expect(p).toBeCloseTo((second + 0.5 * (TEXT.length - second)) / TEXT.length, 9);
    expect(runOf(spoken, ds(), air()).progress).toBe(0);
  });
  test('paused keeps the place: the bar still shows how far', () => {
    const second = TEXT.indexOf('A new paragraph');
    expect(runOf(spoken, ds({ why: 'paused_listener' }, { start: second, length: TEXT.length - second }), air()).progress).toBeCloseTo(second / TEXT.length, 9);
  });
  test('a place for another entry, or for other words (a revision of the text), gives 0, never that one\'s share', () => {
    expect(runOf(spoken, ds({}, { entry: 'other', seconds: 5, total: 10 }), air()).progress).toBe(0);
    expect(runOf(spoken, ds({}, { digest: digestOf('Revised words of the same entry.'), seconds: 5, total: 10 }), air()).progress).toBe(0);
    expect(runOf(spoken, ds({}, { seconds: 5, total: 10 }), air({ handedId: 'other' })).progress).toBe(0);
    expect(runOf(spoken, ds({}, { seconds: 5, total: 10 }), air({ handedText: TEXT + ' More.' })).progress).toBe(0);
  });
  test('G5 audio is the seconds over its length, clamped; no length, a negative one and a bad number are 0', () => {
    const c = (seconds: number, duration: number) => runOf(clip, ds({ seconds, duration }), air({ handedId: '', handedText: '' })).progress;
    expect(c(30, 120)).toBeCloseTo(0.25, 9);
    expect(c(500, 120)).toBe(1);
    expect(c(-4, 120)).toBe(0);
    expect(c(30, 0)).toBe(0);
    expect(c(30, -1)).toBe(0);
    expect(c(Number.NaN, 120)).toBe(0);
  });
  test('audio of another item gives 0', () => {
    expect(runOf(clip, ds({ item: 'other', seconds: 30, duration: 120 }), air({ handedId: '', handedText: '' })).progress).toBe(0);
  });
});

describe('G6 nothing is kept, and the contract lists what it reads', () => {
  test('the same arguments give the same value, in any order of asking', () => {
    const a = runOf(spoken, ds({}, { seconds: 3, total: 9 }), air()), b = runOf(clip, ds({ why: 'paused_system', seconds: 9, duration: 90 }), air({ handedId: '' }));
    expect(runOf(spoken, ds({}, { seconds: 3, total: 9 }), air())).toEqual(a);
    expect(runOf(clip, ds({ why: 'paused_system', seconds: 9, duration: 90 }), air({ handedId: '' }))).toEqual(b);
  });
  test('the bar run is asked with the deck\'s state and the on-air entry, the entry from the spine\'s own resource', () => {
    const c = (f: string): string => readFileSync(new URL(f, import.meta.url), 'utf8');
    expect(c('spine.contract')).toContain('resource brun = barRun(rpulse.tick, posKey, ds, rair, signedOutTick, acs.state) as shape BarRun');
    expect(c('spine.contract')).toContain('resource rair = radioAir(rpulse.tick, posKey, ds.item, signedOutTick, acs.state)');
    expect(c('members-radio.contract')).toMatch(/mutation rpm as shape SettingsAck refreshes [^\n]*\bbrun\b/);
    expect(c('members-radio.contract')).not.toContain('resource rair =');
    expect(c('app.ts')).toContain('barRun: ([_t, _pos, deck, air, _out, _acct]');
  });
});

describe('G8 the bar from what plays alone: a phone with no voice', () => {
  const noVoice = { at: 0, phase: 'playing', transcript: '', message: '', handsFree: false, holding: '', revisedAt: 0, errorAt: 0, errorStage: '', errorMessage: '' };
  test('a spoken entry playing, then paused: its title and its place, and nothing of a voice turn', () => {
    forgetBar();
    const play = barInfo({ ...noVoice, ...runOf(spoken, ds({}, { seconds: 5, total: 10 }), air()) }, { entries: [] });
    expect(play).toMatchObject({ state: 'playing', text: 'Harbor strike ends', playing: true, handsFree: false, talking: false, until: 0 });
    expect(play.progress).toBeGreaterThan(0);
    expect(barInfo({ ...noVoice, ...runOf(spoken, ds({ why: 'paused_listener' }), air()) }, { entries: [] })).toMatchObject({ state: 'paused', text: 'Harbor strike ends', playing: false });
  });
  test('after a skip the new title shows at once and the old progress is gone', () => {
    forgetBar();
    const before = barInfo({ ...noVoice, ...runOf(spoken, ds({}, { seconds: 8, total: 10 }), air()) }, { entries: [] });
    const after = barInfo({ ...noVoice, ...runOf({ ...spoken, title: 'Second story' }, ds({}, { seconds: 8, total: 10 }), air({ id: 'e2', handedId: 'e2' })) }, { entries: [] });
    expect(before.progress).toBeGreaterThan(0.1);
    expect(after).toMatchObject({ state: 'idle', progress: 0 });
  });
});

describe('G9 one snapshot: the title and the place are of the same entry, in every order the two answers can arrive', () => {
  // `ds` and `rair` are two resources; whichever is asked first, the pair is checked by id, so a pair is never half old and half new.
  const A = { id: 'a', handedId: 'a' }, B = { id: 'b', handedId: 'b' };
  const withTitle = (a: object, title: string) => ({ ...air(a as Partial<Air>), title, audio: false });
  const dsOf = (item: string, seconds: number) => ds({ item, place: place({ entry: item, seconds, total: 10 }) });
  test('skip: the new entry\'s air with the old deck state shows neither title nor progress; the old air with the new deck state likewise', () => {
    expect(runOfAir(dsOf('a', 8), withTitle(B, 'Story B'))).toMatchObject({ title: '', progress: 0, playing: false, paused: false });
    expect(runOfAir(dsOf('b', 1), withTitle(A, 'Story A'))).toMatchObject({ title: '', progress: 0, playing: false });
    expect(runOfAir(dsOf('b', 1), withTitle(B, 'Story B'))).toMatchObject({ title: 'Story B', playing: true });
  });
  test('a revision of the entry on air (the text handed over changes, the digest with it): the old place gives progress 0 with the new text', () => {
    expect(runOfAir(ds({}, { seconds: 5, total: 10 }), withTitle({ handedText: TEXT + ' Revised.' }, 'Story'))).toMatchObject({ title: 'Story', progress: 0 });
  });
  test('G10 mid-skip the deck still sounds the old item: nothing is on the bar\'s state but the button says Pause, because a tap pauses (radio-runtime.ts toggle reads the deck)', () => {
    const mid = runOfAir(dsOf('a', 8), withTitle(B, 'Story B'));
    expect(mid.sounding).toBe(true);
    expect(barInfo({ phase: 'playing', ...mid }, { entries: [] })).toMatchObject({ state: 'idle', text: '', playing: true });
    expect(runOfAir(ds({ why: 'paused_listener' }), withTitle({}, 'T')).sounding).toBe(false);
  });
});
