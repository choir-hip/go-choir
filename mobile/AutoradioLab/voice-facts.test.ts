// What shared code hands the voice about the radio (voice-facts.ts), ported from the Swift handlers it replaces: RetrievalFacts' read of the run sheet
// (AutoradioModule+Retrieval.swift) and TakesTests.testRespondsToFromTheEntryOnAir. Failure list, written before the code:
//  V1  the voice sees only the entry on air and the entries ahead, each as {id, title, text, state on_air|ahead, ref}; heard, skipped and left entries are not shown.
//  V2  an entry with no text and no source gives "" for each (never undefined); the text is not cut here.
//  V3  the take's "responds to" is the page the entry on air tells: the paper route of a story, else its first cited story.
//  V4  an opening, a transition, a take, an answer, a result and no entry on air respond to nothing, whatever they cite.
//  V6  the page on air (the reader's follow-along) is the story's or the Texture's address, else nothing.
import { describe, expect, test } from 'bun:test';
import { voiceFacts, respondsOf, airAddressOf } from './voice-facts.ts';
import type { Entry } from './runsheet-model.ts';

const entry = (id: string, o: Partial<Entry> = {}): Entry => ({ id, kind: 'story', placed_by: 'desk', state: 'ahead', title: `T ${id}`, est_seconds: 5, revision: 0, text: 'Words.', ...o });

describe('what the voice is told of the run sheet', () => {
  test('V1 V2 only the entry on air and those ahead, in the shape the retrieval reads', () => {
    const f = voiceFacts([entry('h', { state: 'heard' }), entry('a', { state: 'on_air', source: { ref: 'pub:harbor@v2', label: 'x' } }), entry('b'), entry('c', { text: undefined }), entry('s', { state: 'skipped' })]);
    expect(f).toEqual([
      { id: 'a', title: 'T a', text: 'Words.', state: 'on_air', ref: 'pub:harbor@v2' },
      { id: 'b', title: 'T b', text: 'Words.', state: 'ahead', ref: '' },
      { id: 'c', title: 'T c', text: '', state: 'ahead', ref: '' },
    ]);
  });
});

describe('what a take responds to', () => {
  test('V3 the story on air, by its cite', () => {
    const story = entry('a', { state: 'on_air', cites: [{ ref: 'pub:harbor-fees@v2', role: 'subject', label: 'Harbor fees rise' }] });
    expect(respondsOf(story)).toEqual({ route: 'harbor-fees', title: 'Harbor fees rise' });
  });
  test('V3 a story on air with a paper route answers its own title', () => {
    expect(respondsOf(entry('a', { state: 'on_air', title: 'Harbor fees rise', source: { ref: 'pub:harbor-fees', label: 'Harbor fees rise' } }))).toEqual({ route: 'harbor-fees', title: 'Harbor fees rise' });
  });
  test('V4 an opening, a transition, a take and nothing respond to nothing', () => {
    expect(respondsOf(entry('o', { kind: 'opening' }))).toBeUndefined();
    expect(respondsOf(entry('t', { kind: 'transition' }))).toBeUndefined();
    expect(respondsOf(entry('k', { kind: 'take' }))).toBeUndefined();
    expect(respondsOf(undefined)).toBeUndefined();
    const cites = [{ ref: 'pub:harbor-fees@v2', role: 'subject' as const, label: 'Harbor fees rise' }];
    for (const kind of ['opening', 'answer', 'result', 'transition', 'take'] as const) expect(respondsOf(entry('x', { kind, cites }))).toBeUndefined();
  });
  test('V6 the address of the page on air', () => {
    expect(airAddressOf(entry('a', { state: 'on_air', source: { ref: 'pub:harbor-fees@v2', label: 'x' } }))).toBe('pub:harbor-fees');
    expect(airAddressOf(entry('a', { state: 'on_air', kind: 'page', source: { ref: 'texture:d1', label: 'x' } }))).toBe('texture:d1');
    expect(airAddressOf(entry('a', { state: 'on_air', kind: 'page', source: { ref: 'texture:d1@r2', label: 'x' } }))).toBe('texture:d1');
    expect(airAddressOf(entry('a', { kind: 'opening' }))).toBe('');
    expect(airAddressOf(undefined)).toBe('');
  });
});
