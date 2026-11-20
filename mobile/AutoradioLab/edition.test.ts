// Edition setup under bun test. Ported from EditionSetupTests.swift (E1 to E5, E8, T1 to T3 and the spoken forms; E6, E7, E9 are edition-picks.test.ts). The edition writer's tests (C1 to C4) stay with the Swift writer.
import { describe, expect, test } from 'bun:test';
import {
  suggestions, TREE, key, parentsOf, Selection, parseSpoken, isEmptyEdit, labelForSectionID, treeRows, EXPLAINER, VOICE_HINT, EMPTY_LINE, SPOKEN_OPEN, selectionLine,
  MAX_TOPIC_LENGTH,
} from './edition-model.ts';

const sug = suggestions();
const rows = (exp: string[], sel: string[], live: string[] = []) => treeRows({ liveSections: live, expanded: new Set(exp.map(key)), selection: new Selection(sel), own: [] });

describe('what is offered', () => {
  test('suggestions are generous and start with the house sections', () => {
    expect(sug.length).toBeGreaterThanOrEqual(18); expect(TREE.length).toBe(10);
    expect(sug.slice(0, 10)).toEqual(['World', 'U.S.', 'Politics', 'Business', 'Technology', 'Science', 'Health', 'Climate', 'Culture', 'Sports']);
    expect(new Set(sug.map(key)).size).toBe(sug.length);
  });
  test('live sections are merged without duplicates', () => {
    const s = suggestions(['tech_ai', 'world', 'housing_market']);
    expect(s.filter((x) => x === 'Technology').length).toBe(1); expect(s).toContain('Housing market'); expect(labelForSectionID('us')).toBe('U.S.');
  });
  test('no suggestion or line asks what the reader does not want', () => {
    const words = [...sug, EXPLAINER, VOICE_HINT, EMPTY_LINE, SPOKEN_OPEN].join(' ').toLowerCase();
    for (const bad of ['tired of', 'do not want', "don't want", 'avoid', 'less of', 'anything you']) expect(words).not.toContain(bad);
    expect(EXPLAINER).toContain('any time');
  });
  test('T2: the graph is sound', () => {
    const tops = TREE.map((c) => c.label);
    expect(new Set(tops.map(key)).size).toBe(tops.length);
    for (const c of TREE) {
      expect(new Set(c.children.map(key)).size).toBe(c.children.length);
      for (const k of c.children) { expect(tops).not.toContain(k); expect(parentsOf(k).length).toBeGreaterThan(0); }
    }
    expect(new Set(parentsOf('AI'))).toEqual(new Set(['Technology', 'Business']));
    expect(new Set(parentsOf('Public health'))).toEqual(new Set(['Health', 'Politics']));
    expect(new Set(parentsOf('Energy'))).toEqual(new Set(['Climate', 'Business']));
    expect(new Set(parentsOf('Security'))).toEqual(new Set(['Technology', 'World']));
    expect(parentsOf('ai')).toEqual(parentsOf('AI'));
    expect(sug.filter((x) => key(x) === 'ai').length).toBe(1);
    expect(new Selection().add('ai', sug)).toBe('AI'); expect(new Selection().apply('add AI', sug).added).toEqual(['AI']);
  });
});

describe('the tree', () => {
  test('T1: a shared subtopic is one topic lit everywhere', () => {
    const r = rows(['Technology', 'Business'], ['AI']);
    const ai = r.filter((x) => x.label === 'AI');
    expect(ai.length).toBe(2); expect(ai.every((x) => x.selected)).toBe(true); expect(ai.map((x) => x.note)).toEqual(['also in Technology', 'also in Business']);
    expect(new Selection(['AI', 'ai', 'AI']).topics).toEqual(['AI']);
    expect(r.find((x) => x.label === 'Technology')?.note).toBe('1 picked'); expect(r.find((x) => x.label === 'Business')?.note).toBe('1 picked');
  });
  test('T3: partial selection, and nothing moves on picking', () => {
    expect(rows([], []).length).toBe(10);
    expect(rows(['Culture'], []).map((x) => x.key)).toEqual(rows(['Culture'], ['Film', 'Music', 'Culture', 'Sports']).map((x) => x.key));
    const r = rows([], ['Film']);
    expect(r.find((x) => x.label === 'Culture')?.partial).toBe(true); expect(r.find((x) => x.label === 'Culture')?.selected).toBe(false);
    const p = rows([], ['Culture']);
    expect(p.find((x) => x.label === 'Culture')?.selected).toBe(true); expect(p.find((x) => x.label === 'Culture')?.partial).toBe(false);
    expect(rows(['Culture'], ['Culture']).filter((x) => x.kind === 'sub').every((x) => !x.selected)).toBe(true);
    expect(rows(['Sports'], []).length).toBe(10); expect(rows(['Culture'], []).length).toBe(17); expect(rows([], []).find((x) => x.label === 'Sports')?.expandable).toBe(false);
  });
});

describe('picking and saying', () => {
  test('E1: the same topic twice is one', () => {
    const s = new Selection();
    s.toggle('Space', sug); expect(s.topics).toEqual(['Space']);
    expect(s.add('space', sug)).toBeUndefined(); expect(s.add(' SPACE ', sug)).toBeUndefined();
    expect(s.add('tech', sug)).toBe('Technology');
    expect(s.add('technology', sug)).toBeUndefined(); expect(s.add('Spaces', sug)).toBeUndefined();
    expect(s.topics).toEqual(['Space', 'Technology']);
    s.toggle('space', sug); expect(s.contains('Space')).toBe(false);
    const u = new Selection(); u.add('us', sug); expect(u.topics).toEqual(['U.S.']); expect(u.add('U.S.', sug)).toBeUndefined();
    const c = new Selection(); c.add('Chess', sug); expect(c.add('chess', sug)).toBeUndefined();
  });
  test('E2: speech names topics not in the suggestions', () => {
    const s = new Selection();
    const o = s.apply('add space and chess', sug);
    expect(o.added).toEqual(['Space', 'Chess']); expect(o.reply).toBe('Added Space and Chess.'); expect(s.topics).toEqual(['Space', 'Chess']);
    expect(s.apply('also sourdough baking, and the tech', sug).added).toEqual(['Sourdough baking', 'Technology']);
  });
  test('spoken forms', () => {
    expect(parseSpoken("I'd like to hear about space").adds).toEqual(['space']);
    expect(parseSpoken('Space.').adds).toEqual(['space']);
    expect(parseSpoken('please add the U.S. and politics').adds).toEqual(['us', 'politics']);
    expect(parseSpoken('drop chess').removes).toEqual(['chess']);
    expect(parseSpoken('take off sports and music').removes).toEqual(['sports', 'music']);
    expect(parseSpoken('continue').finish).toBe(true); expect(parseSpoken("that's all").finish).toBe(true);
    expect(parseSpoken('tell me about science, health and film').adds).toEqual(['science', 'health', 'film']);
  });
  test('E5: questions and long sentences are not topics', () => {
    for (const q of ['what is this for?', 'how does this work', 'I was thinking about whether the weather today will affect anything at all', '', 'um']) expect(isEmptyEdit(parseSpoken(q))).toBe(true);
    const s = new Selection();
    expect(s.apply('what is this', sug).reply).toContain('Tell me a topic'); expect(s.topics).toEqual([]);
  });
  test('E8: negative words only take off what is there', () => {
    const s = new Selection(['Sports', 'Music']);
    s.apply('not sports', sug); expect(s.topics).toEqual(['Music']);
    expect(s.apply('no gossip', sug).added).toEqual([]); expect(s.topics).toEqual(['Music']);
    expect(s.apply('nothing', sug).added).toEqual([]); expect(s.apply('none', sug).removed).toEqual([]);
  });
  test('E3: long or empty typed topics', () => {
    const s = new Selection();
    expect(s.addTyped('   ', sug)).toEqual([]); expect(s.addTyped('...,;!', sug)).toEqual([]);
    const added = s.addTyped(Array(40).fill('harbour').join(' '), sug);
    expect(added.length).toBe(1); expect([...added[0]].length).toBeLessThanOrEqual(MAX_TOPIC_LENGTH); expect(added[0].endsWith(' ')).toBe(false);
    expect(s.addTyped('space, chess; Arts and crafts', sug)).toEqual(['Space', 'Chess', 'Arts and crafts']);
    expect(s.addTyped('a\u0007b', sug)).toEqual(['Ab']);
  });
  test('E4: no minimum and no maximum', () => {
    expect(selectionLine([])).toContain('general edition');
    const s = new Selection();
    for (const t of sug) s.toggle(t, sug);
    expect(s.topics.length).toBe(sug.length);
    expect(selectionLine(['Space'])).toContain('Space'); expect(selectionLine(['Space', 'Chess', 'Film'])).toContain('3 topics');
  });
});
