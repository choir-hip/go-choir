// The programme the desk writes, read by the phone: the stretch decoder, the server's block split, titles and cites, a segment becoming an entry,
// and the writer's rules. Ported from ProgrammePlayerTests.swift (F01 to F10), ProgrammePlayerRepairTests.swift (S3 to S5), RunSheetStoreTests.swift
// (12 the old kind names map; 22 the titles the listener sees; 23 write-ahead) and read against the server builder's fixtures
// (fixtures/learning/blocks.json, blocks_unicode.json, stretch_lines.json, segment-settle.sse).
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
  StretchDecoder, splitBlocks, title, label, oneLine, entryFrom, entryID, placeOp, shouldAskMore, askOutcome, plainWords, toldRoutes, type Incoming, type StretchEvent, type Lookups, type ClipInfo,
} from './programme-model.ts';
import { apply, emptySheet, validEntry, type Entry } from './runsheet-model.ts';

const fx = (n: string) => readFileSync(new URL(`../fixtures/learning/${n}`, import.meta.url), 'utf8');
const segs = (e: StretchEvent[]): Incoming[] => e.flatMap((x) => (x.type === 'segment' ? [x.segment] : []));
const line = (o: unknown) => JSON.stringify(o);
const none: Lookups = { clip: () => undefined, take: () => undefined };

describe('reading the stretch (F01 to F07)', () => {
  test('F01: a chunk ending inside a line waits', () => {
    const d = new StretchDecoder();
    const l = line({ kind: 'story', title: 'T', text: 'Hello there.' });
    expect(d.feed(l.slice(0, 10))).toEqual([]); expect(d.feed(l.slice(10, 25))).toEqual([]);
    expect(segs(d.feed(l.slice(25) + '\n')).map((s) => s.text)).toEqual(['Hello there.']);
    expect(d.finish()).toEqual([]);
  });
  test('F01: line endings LF, CRLF and a lone CR; a CR at a chunk end may be half a CRLF', () => {
    const a = line({ kind: 'story', title: 'A', text: 'One.' }), b = line({ kind: 'story', title: 'B', text: 'Two.' }), c = line({ kind: 'story', title: 'C', text: 'Three.' });
    expect(segs(new StretchDecoder().feed(a + '\r\n' + b + '\r' + c + '\n')).map((s) => s.title)).toEqual(['A', 'B', 'C']);
    const e = new StretchDecoder();
    expect(segs(e.feed(a + '\r')).map((s) => s.title)).toEqual(['A']);
    expect(segs(e.feed('\n' + b + '\n')).map((s) => s.title)).toEqual(['B']);
    expect(e.ignoredCount).toBe(0);
  });
  test('F01: the last line without a line ending is read at finish', () => {
    const d = new StretchDecoder();
    expect(d.feed(line({ kind: 'story', title: 'T', text: 'No newline.' }))).toEqual([]);
    expect(segs(d.finish()).map((s) => s.text)).toEqual(['No newline.']);
  });
  test('F02: noise is ignored and counted, and the stream goes on', () => {
    const d = new StretchDecoder();
    expect(d.feed(': autoradio\n\n\nevent: handoff\ndata: {"x":1}\n\nnot json at all\n[1,2,3]\n"a string"\n{"kind":"story"\n')).toEqual([]);
    expect(d.ignoredCount).toBeGreaterThan(0);
    expect(segs(d.feed(line({ kind: 'story', title: 'T', text: 'Still works.' }) + '\n')).map((s) => s.text)).toEqual(['Still works.']);
  });
  test('F03: the server\'s fixture stretch: the phone drops only what it cannot speak', () => {
    const lines = JSON.parse(fx('stretch_lines.json')).segment_lines as { line: unknown }[];
    expect(lines.length).toBe(32);
    const d = new StretchDecoder();
    let got: Incoming[] = [];
    for (const l of lines) got = got.concat(segs(d.feed(line(l.line) + '\n')));
    got = got.concat(segs(d.finish()));
    expect(d.droppedCount).toBe(2);                         // an unknown kind (weather) and an empty text; the server validates the rest
    expect(got.length).toBe(30); expect(got.some((s) => s.kind === 'weather')).toBe(false);
    const titles = got.map((s) => title(s.title, s.kind));
    expect(titles.some((t) => t !== t.trim() || t === '')).toBe(false);
    expect(titles).toContain('A quiet harbour'); expect(titles).toContain('Story');
    expect(got.every((s) => s.text.trim() !== '')).toBe(true);
  });
  test('F04: titles', () => {
    const t = (v: unknown, kind = 'story') => {
      const o: Record<string, unknown> = { kind, text: 'Words.' }; if (v !== undefined) o.title = v;
      const s = segs(new StretchDecoder().feed(line(o) + '\n'))[0];
      return s ? title(s.title, s.kind) : '<none>';
    };
    expect(t('a\nb')).toBe('a b'); expect(t('a\u0007b')).toBe('a b'); expect(t('   Padded   title  ')).toBe('Padded title');
    expect(new TextEncoder().encode(t('é'.repeat(150))).length).toBeLessThanOrEqual(200); expect(t('é'.repeat(150))).not.toContain('�');
    expect(t(undefined, 'ident')).toBe('Station break'); expect(t('', 'story')).toBe('Story'); expect(t('pub:harbour-1')).toBe('Story');   // a raw ref is never a title
    expect(title('', 'bridge')).toBe('Autoradio');
  });
  test('S4: zero-width and bidi format characters never reach a title', () => {
    expect(title('Ha​rbour ‮news⁦', 'story')).toBe('Harbour news');
    expect(title('​‮', 'story')).toBe('Story'); expect(title('Café ﻿open', 'story')).toBe('Café open');
    expect(oneLine('a‍b')).toBe('a‍b');           // joiners are part of the text
  });
  test('F05: cites, one ref once, and a chip only for a cite with a label', () => {
    const s = segs(new StretchDecoder().feed(line({ kind: 'story', title: 'T', text: 'Words.', cites: ['pub:a', { ref: 'pub:b', role: 'subject', label: 'The harbour story' }, { ref: 'pub:b', role: 'quoted', label: 'The harbour story' }, { ref: 'mail:1', role: 'mentioned' }] }) + '\n'))[0];
    const e = entryFrom(s, none) as Entry;
    expect(e.cites?.map((c) => c.ref)).toEqual(['pub:a', 'pub:b', 'mail:1']);
    expect(e.cites?.map((c) => c.label).filter((l) => l !== '')).toEqual(['The harbour story']);
    expect(label('pub:x')).toBe('');
  });
  test('F06: ids are local and repeated ids are one entry; a header names the stretch', () => {
    const d = new StretchDecoder();
    const a = segs(d.feed(line({ kind: 'story', title: 'A', text: 'One.' }) + '\n' + line({ kind: 'story', title: 'B', text: 'Two.' }) + '\n'));
    expect(a.map((s) => s.segment_id)).toEqual(['local/0', 'local/1']); expect(new Set(a.map((s) => s.stretch_id))).toEqual(new Set(['local']));
    const e = new StretchDecoder();
    e.feed('event: stretch\ndata: {"stretch_id":"st-9"}\n\n');
    const b = segs(e.feed('event: segment\ndata: ' + line({ kind: 'story', title: 'A', text: 'One.' }) + '\n\n'));
    expect([b[0].stretch_id, b[0].segment_id]).toEqual(['st-9', 'st-9/0']);
    const c = segs(e.feed('event: segment\ndata: ' + line({ stretch_id: 'st-9', segment_id: 'st-9/5', kind: 'headlines', text: 'Words.', block_count: 3 }) + '\n\n'));
    expect([c[0].segment_id, c[0].block_count]).toEqual(['st-9/5', 3]);
    const first = entryFrom(c[0], none) as Entry;
    const sheet = apply(apply(emptySheet(), placeOp(first, 'extend')).sheet, placeOp(first, 'extend'));
    expect(sheet.result.dropped).toBe('duplicate');           // a segment seen twice is one op
  });
  test('F07: done, error and a stream with no done', () => {
    const d = new StretchDecoder();
    const ev = d.feed('event: segment\ndata: ' + line({ kind: 'story', title: 'A', text: 'One.' }) + '\n\nevent: error\ndata: {"spoken":"Trouble.","reason":"upstream"}\n\nevent: done\ndata: {"stretch_id":"x"}\n\n');
    expect(ev.length).toBe(3);
    expect(ev[1].type === 'error' && ev[1].message.includes('upstream')).toBe(true); expect(ev[2]).toEqual({ type: 'done' });
    const e = new StretchDecoder();
    const got = [...e.feed(line({ kind: 'story', title: 'A', text: 'One.' }) + '\n'), ...e.finish()];
    expect(segs(got).length).toBe(1); expect(got.some((x) => x.type === 'done')).toBe(false);    // no done: what arrived is kept
  });
  test('S3: a bare event line is dispatched by a blank line, the next event line, or the end', () => {
    const d = new StretchDecoder();
    expect(d.feed('event: done\n')).toEqual([]); expect(d.feed('\n')).toEqual([{ type: 'done' }]);
    const e = new StretchDecoder();
    expect(e.feed('event: error\nevent: done\n')).toEqual([{ type: 'error', message: 'the stretch failed' }]); expect(e.finish()).toEqual([{ type: 'done' }]);
    const f = new StretchDecoder();
    expect(f.feed('event: error\ndata: {"reason":"x"}\n')).toEqual([{ type: 'error', message: 'x' }]); expect(f.finish()).toEqual([]);
  });
  test('block_count is only an integer', () => {
    const count = (raw: string) => segs(new StretchDecoder().feed(`{"kind":"story","title":"T","text":"Words.","block_count":${raw}}\n`))[0]?.block_count;
    expect(count('3')).toBe(3); expect(count('3.0')).toBe(3);
    for (const bad of ['true', 'false', '2.5', '"3"']) expect(count(bad)).toBeUndefined();
  });
  test('C3: a podcast_clip line decodes with its clip; one with no clip is dropped by the decoder', () => {
    const d = new StretchDecoder();
    const ok = line({ kind: 'podcast_clip', title: 'T', text: 'Lead.', clip: { clip_id: 'clip-000000000001', story_ref: 'pub:a@v2', seconds: 56.5 } });
    const bad = line({ kind: 'podcast_clip', title: 'T', text: 'Lead.' });
    const got = segs(d.feed(ok + '\n' + bad + '\n'));
    expect(got.length).toBe(1);
    expect(got[0].clip).toEqual({ clip_id: 'clip-000000000001', story_ref: 'pub:a@v2', seconds: 56.5 });
    expect(d.droppedCount).toBe(1);
  });
  test('F1: a forecast carrying only the claim id keeps no probability and no cause; a full one keeps all three', () => {
    const d = new StretchDecoder();
    const only = segs(d.feed(line({ kind: 'story', title: 'T', text: 'Words.', claim_id: 'c9' }) + '\n'))[0];
    expect(only.claim_id).toBe('c9'); expect(only.forecast).toBeUndefined();
    const full = segs(d.feed(line({ kind: 'story', title: 'T', text: 'Words.', forecast: { p_heard: 0.62, caused_by: 'the listener skipped long pieces', claim_id: 'c3' } }) + '\n'))[0];
    expect(full.forecast).toEqual({ p_heard: 0.62, caused_by: 'the listener skipped long pieces', claim_id: 'c3' }); expect(full.claim_id).toBe('c3');
    const flat = segs(d.feed(line({ kind: 'story', title: 'T', text: 'Words.', p_heard: 0.4, caused_by: 'flat', claim_id: 'c5' }) + '\n'))[0];
    expect(flat.forecast).toEqual({ p_heard: 0.4, caused_by: 'flat', claim_id: 'c5' });   // the flat fields of the line are read as the nested ones are
    for (const bad of [1.5, -0.1, '0.5']) expect(segs(d.feed(line({ kind: 'story', title: 'T', text: 'Words.', p_heard: bad }) + '\n'))[0].forecast).toBeUndefined();   // out of range or not a number: no forecast
    const none = segs(d.feed(line({ kind: 'story', title: 'T', text: 'Words.' }) + '\n'))[0];
    expect(none.claim_id).toBeUndefined(); expect(none.forecast).toBeUndefined();
  });
  test('the settle fixture: entry_id and claim_id ride on the event and the entry is the server\'s', () => {
    const d = new StretchDecoder();
    const ev = [...d.feed(fx('segment-settle.sse')), ...d.finish()];
    const ss = segs(ev);
    expect(ss.length).toBe(2); expect(ev.at(-1)).toEqual({ type: 'done' });
    const story = entryFrom(ss[0], none) as Entry, tr = entryFrom(ss[1], none) as Entry;
    expect(story.id).toBe('d-s7-1'); expect(story.forecast).toEqual({ caused_by: '', claim_id: 'calib:autoradio-dj:s7:1' });   // no number the phone does not hold
    expect(story.forecast?.p_heard).toBeUndefined(); expect(story.source?.ref).toBe('pub:harbor-fees@v2');
    expect(tr.kind).toBe('transition'); expect(tr.title).toBe(''); expect(tr.between).toEqual({ after: 'd-s7-1', before: 'd-s7-3' });
    expect([story, tr].every(validEntry)).toBe(true);
  });
});

describe('the server\'s block split (F08)', () => {
  test('the server\'s cases', () => {
    const cases = JSON.parse(fx('blocks.json')).cases as { name: string; text: string; blocks: string[]; block_count: number }[];
    expect(cases.length).toBe(9);
    for (const c of cases) { expect([c.name, splitBlocks(c.text)]).toEqual([c.name, c.blocks]); expect(splitBlocks(c.text).length).toBe(c.block_count); }
  });
  test('S5: it matches Go on unicode whitespace', () => {
    const cases = JSON.parse(fx('blocks_unicode.json')).cases as { name: string; text: string; blocks: string[] }[];
    expect(cases.length).toBe(12);
    for (const c of cases) expect([c.name, splitBlocks(c.text)]).toEqual([c.name, c.blocks]);
  });
  test('a lone CR is a line ending', () => {
    expect(splitBlocks('A first.\r\rA second.')).toEqual(['A first.', 'A second.']);
    expect(splitBlocks('A first.\rstill first.\r\r\rA second.')).toEqual(['A first.\nstill first.', 'A second.']);
  });
});

describe('a segment becomes an entry (12, 22)', () => {
  const seg = (id: string, kind: string, o: Partial<Incoming> = {}): Incoming => ({ stretch_id: 'st', segment_id: id, kind, title: `Title ${id}`, text: `Words for ${id}.`, cites: [{ ref: 'pub:a@v2', role: 'subject', label: 'A' }], ...o });
  const clip: ClipInfo = { clip_id: 'clip-0000000000b1', show: 'Show', audio_url: 'https://example.test/b.mp3', start: 10, end: 70, seconds: 60, route: '/a', version: 1, story_title: 'S', episode_title: '', relocatable: true };
  const look: Lookups = {
    clip: (id) => (id === clip.clip_id ? clip : undefined),
    take: (id) => (id === 'take-1' ? { take_id: 'take-1', publisher: 'voice-3fa2c1', duration: 20, title: 'What they said', responds_to: { route: 'harbour', title: 'Harbour' } } : undefined),
  };
  test('12: the old kind names map', () => {
    const k = (s: Incoming) => entryFrom(s, look)?.kind;
    expect([k(seg('a', 'ident')), k(seg('b', 'opening')), k(seg('c', 'bridge')), k(seg('d', 'transition'))]).toEqual(['opening', 'opening', 'transition', 'transition']);
    expect([k(seg('e', 'story')), k(seg('f', 'update')), k(seg('g', 'headlines'))]).toEqual(['story', 'story', 'story']);
    expect(k(seg('h', 'podcast_clip', { clip: { clip_id: clip.clip_id, story_ref: '', seconds: 0 } }))).toBe('clip');
    expect(k(seg('i', 'podcast_clip', { clip: { clip_id: 'clip-nothere', story_ref: '', seconds: 0 } }))).toBeUndefined();   // a clip the phone does not have is not an entry
    const t = entryFrom(seg('t', 'transition', { between: { after: 'st/1', before: 'st/2' } }), look) as Entry;
    expect(t.between).toEqual({ after: 'd-st-1', before: 'd-st-2' }); expect(t.id).toBe('d-t'); expect(t.placed_by).toBe('desk'); expect(t.source?.ref).toBe('pub:a@v2'); expect(validEntry(t)).toBe(true);
  });
  test('22: an opening and a transition have the titles the listener sees', () => {
    expect(entryFrom(seg('a', 'ident', { title: 'Autoradio', text: 'Hello.' }), look)?.title).toBe('Opening');
    expect(entryFrom(seg('b', 'bridge', { title: '', text: 'Next.' }), look)?.title).toBe('');
  });
  test('a clip carries its audio and the story it was cut for; a clip that cannot be found is no entry', () => {
    const e = entryFrom(seg('h', 'podcast_clip', { clip: { clip_id: clip.clip_id, story_ref: 'pub:other', seconds: 0 } }), look) as Entry;
    expect(e.source).toEqual({ ref: 'clip:' + clip.clip_id, label: 'Show', url: clip.audio_url, start: 10, end: 70, clip_id: clip.clip_id });
    expect(e.cites?.map((c) => c.ref)).toEqual(['pub:a@v2', 'pub:/a@v1', 'pub:other']);
    expect(e.est_seconds).toBeGreaterThan(60);
    const missing = { ...look, clip: () => ({ ...clip, relocatable: false }) };
    expect(entryFrom(seg('h', 'podcast_clip', { clip: { clip_id: clip.clip_id, story_ref: '', seconds: 0 } }), missing)).toBeUndefined();
  });
  test('a take the phone holds becomes an entry with its publisher; one it does not hold is not played', () => {
    const e = entryFrom(seg('k', 'take', { take_id: 'take-1', forecast: { p_heard: 0.5, caused_by: 'x', claim_id: 'c' } }), look) as Entry;
    expect(e.kind).toBe('take'); expect(e.source).toEqual({ ref: 'take:take-1', label: 'voice-3fa2c1' }); expect(e.title).toBe('What they said');
    expect(e.cites?.some((c) => c.ref === 'pub:harbour')).toBe(true);
    expect(entryFrom(seg('k', 'take', { take_id: 'take-9', forecast: { p_heard: 0.5, caused_by: 'x', claim_id: 'c' } }), look)).toBeUndefined();
  });
  test('a take line without a forecast or with a bad id is dropped by the decoder (the server drops it too)', () => {
    const d = new StretchDecoder();
    d.feed(line({ kind: 'take', take: 'take-1', text: 'x' }) + '\n'); d.feed(line({ kind: 'take', take: 'bad id!', text: 'x', p_heard: 0.5 }) + '\n');
    expect(d.droppedCount).toBe(2);
    expect(segs(d.feed(line({ kind: 'take', take: 'take-1', text: 'x', p_heard: 0.5, caused_by: 'c', claim_id: 'k' }) + '\n')).length).toBe(1);
  });
  test('no words is no entry; the entry id keeps to the characters an id may have', () => {
    expect(entryFrom(seg('x', 'story', { text: '  \n ' }), look)).toBeUndefined();
    expect(entryID('st-9/5 é')).toBe('d-st-9-5--');
    expect(entryID('x'.repeat(100)).length).toBe(64);
  });
});

describe('write ahead (23)', () => {
  const e = (id: string, secs: number, kind: Entry['kind'] = 'story'): Entry => ({ id, kind, placed_by: 'desk', state: 'ahead', title: id, text: 'w', est_seconds: secs, revision: 0 });
  test('three entries of 20 seconds ahead ask for more (60 s is under 180)', () => expect(shouldAskMore([e('a', 20), e('b', 20), e('c', 20)], 0)).toBe(true));
  test('a full sheet does not ask; fewer than two playable entries asks at once, transitions not counting', () => {
    expect(shouldAskMore([e('a', 100), e('b', 100)], 0)).toBe(false);
    expect(shouldAskMore([e('a', 500)], 0)).toBe(true);
    expect(shouldAskMore([e('a', 500), e('t', 500, 'transition')], 0)).toBe(true);
    expect(shouldAskMore([e('a', 100), e('b', 99)], 100)).toBe(true);        // twice a measured 100 s write is 200 s
  });
  test('an ask that added nothing holds back the next one; a dropped opening does not; a repeat is not a failure', () => {
    expect(askOutcome({ added: 2, received: 2, dropped: 0, failed: false, cause: 'extend', airId: 'a' })).toEqual({ dry: false, clearQuiet: true, notice: 'clear' });
    expect(askOutcome({ added: 0, received: 0, dropped: 0, failed: false, cause: 'extend', airId: 'a' })).toMatchObject({ dry: true, quietOn: 'a', notice: 'keep' });
    expect(askOutcome({ added: 0, received: 1, dropped: 1, failed: false, cause: 'open', airId: 'a' })).toMatchObject({ quietOn: undefined, notice: 'clear' });
    expect(askOutcome({ added: 0, received: 0, dropped: 0, failed: true, cause: 'extend', airId: 'a' })).toMatchObject({ dry: false, notice: 'say' });
    expect(askOutcome({ added: 0, received: 0, dropped: 0, failed: false, cause: 'open', airId: 'a' }).notice).toBe('say');
  });
  test('plain words for what went wrong', () => {
    expect(plainWords({ kind: 'cancelled' })).toBeUndefined(); expect(plainWords({ kind: 'empty' })).toBeUndefined();
    expect(plainWords({ kind: 'signedOut' })).toBe('Sign in to hear the desk.'); expect(plainWords({ kind: 'transport' })).toBe("The desk can't be reached right now.");
    expect(plainWords({ kind: 'other' })).toBe("The desk isn't answering right now.");
  });
  test('the routes told already come from sources and cites, once', () => {
    const x = e('a', 1); x.cites = [{ ref: 'pub:harbour@v2', role: 'subject', label: '' }]; x.source = { ref: 'pub:tram', label: '' };
    expect(toldRoutes([x, e('b', 1)])).toEqual(new Set(['harbour', 'tram']));
  });
});
