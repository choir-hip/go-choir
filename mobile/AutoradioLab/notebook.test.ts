// The notebook page and the scorecard under bun test (the Swift LearningLoopTests F3 to F8, same fixtures in mobile/fixtures/learning).
// Failure list: see notebook.ts (N1 to N8).
import { describe, expect, test } from 'bun:test';
import {
  parse, merging, mineRef, sha256, editionText, parseLearning, readLearning, page, HELD, notebookPage, loadDoc, lookupEdition, CACHE_FILE, saveCache, loadCache,
  EDITION_MAX_BYTES, type Get, type DocLoad,
} from './notebook.ts';
import { HTTPError, SignedOutError } from './api.ts';
import notebookMd from '../fixtures/learning/notebook.md' with { type: 'text' };
import learning from '../fixtures/learning/learning.json';
import learningEmpty from '../fixtures/learning/learning-empty.json';

const files = new Map<string, Uint8Array>();
const storage = { fs: { readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; }, atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); }, rm: async (p: string) => { files.delete(p); } } } as never;
const nb = parse(notebookMd as string);

describe('N1 to N3 the text', () => {
  test('N1 read as lines', () => {
    expect(nb.sections.map((s) => s.key)).toEqual(['asked', 'learned', 'trying', 'steering']);
    const asked = nb.sections[0].lines;
    expect(asked.length).toBe(3); expect(asked[0].tag).toBeNull(); expect(asked[0].ref).toBe(mineRef('Mornings: world news first, then science.'));
    expect(mineRef('x').length).toBe('mine:'.length + 8);
    const l = nb.sections[1].lines;
    expect(l[0]).toMatchObject({ ref: 'lesson:L1', heard: 9, of: 11, text: 'Long policy pieces before 8 am get skipped; short ones are heard.' });
    expect(l[1].ref).toBe('lesson:L9'); expect(nb.sections[2].lines[0].ref).toBe('try:T2');
    expect(nb.sections[3].lines.length).toBe(2); expect(nb.sections[3].lines[0].ref).toBeNull();
    const odd = parse('## What you asked for\n- A line about [L and a bracket [Lx] stays whole.\nNot a bullet\n');
    expect(odd.sections[0].lines.map((x) => x.text)).toEqual(['A line about [L and a bracket [Lx] stays whole.']); expect(odd.sections[0].lines[0].tag).toBeNull();
  });
  test('the synchronous SHA-256 agrees with the platform\'s', async () => {
    for (const s of ['', 'abc', 'x'.repeat(55), 'x'.repeat(56), 'x'.repeat(64), 'é'.repeat(100)]) {
      const want = new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s)));
      expect([...sha256(new TextEncoder().encode(s))]).toEqual([...want]);
    }
  });
  test('N2 the topics move in once', () => {
    const seed = merging(['Space', 'Chess'], null);
    expect(seed.startsWith('# My notebook\n\n## What you asked for\n- Space\n- Chess')).toBe(true);
    const once = merging(['Space', 'Chess'], notebookMd as string);
    const parsed = parse(once);
    expect(parsed.sections[0].lines.map((x) => x.text).slice(-2)).toEqual(['Space', 'Chess']);
    expect(parsed.sections[0].lines.length).toBe(5); expect(parsed.sections[1].lines.length).toBe(2);
    expect(merging(['Space', 'Chess', 'space'], once)).toBe(once);
    const noSection = merging(['Art'], '# My notebook\n\n## What Autoradio has learned\n- A thing. [L2 · heard 1 of 1]\n');
    expect(parse(noSection).sections.find((s) => s.key === 'asked')?.lines.map((x) => x.text)).toEqual(['Art']);
    expect(noSection).toContain('[L2 · heard 1 of 1]');
  });
  test('N3 the DJ is sent the notebook, else the topics, else nothing, inside 32 KB', () => {
    const t = (notebookMd as string).trim();
    expect(editionText(notebookMd as string, ['Space'])).toBe(t);
    expect(editionText(null, ['Space', 'Chess'])).toBe('What the listener follows:\n- Space\n- Chess');
    expect(editionText(null, [])).toBeNull(); expect(editionText('  \n', [])).toBeNull();
    expect(new TextEncoder().encode(editionText('a line of words\n'.repeat(4000), [])!).length).toBeLessThanOrEqual(EDITION_MAX_BYTES);
  });
});

describe('N4 the learning route', () => {
  test('its fixture parses; an empty card and an older shape parse', () => {
    const r = parseLearning(learning)!;
    expect(r.card).toMatchObject({ forecasts: 14, settled: 11, heardThrough: 8, noVerdict: 1 });
    expect(r.card.heardThroughRate).toBeCloseTo(0.7272727, 4);
    expect(r.card.byKind.map((g) => g.label)).toEqual(['story', 'podcast_clip']); expect(r.card.byCause.map((g) => g.label)).toEqual(['lesson', 'importance']);
    expect(r.rows.length).toBe(4);
    expect(r.rows[1]).toMatchObject({ status: 'settled', heardThrough: true }); expect(r.rows[1].op?.outcome).toBe('finished');
    expect(r.rows[3]).toMatchObject({ status: 'no_verdict', noVerdictReason: 'left', heardThrough: null });
    const e = parseLearning(learningEmpty)!;
    expect(e.card.settled).toBe(0); expect(e.card.heardThroughRate).toBeNull(); expect(e.rows).toEqual([]);
    expect(parseLearning({ scorecard: { settled: 3 }, rows: [{ claim_id: 'c' }] })).not.toBeNull();
    expect(parseLearning([1, 2])).toBeNull();
  });
  test('a missing route is not yet, a dead network is offline, no key is signed out; the request is the route with its limit', async () => {
    const seen: string[] = [];
    const ok = await readLearning(async (p, q) => { seen.push(p + JSON.stringify(q)); return learning; });
    expect(ok.state === 'ready' && ok.read.card.settled).toBe(11); expect(seen).toEqual(['/api/autoradio/learning{"limit":200}']);
    for (const status of [404, 501]) expect((await readLearning(async () => { throw new HTTPError(status, `HTTP ${status}`); })).state).toBe('notyet');
    expect((await readLearning(async () => { throw new HTTPError(503, 'HTTP 503'); })).state).toBe('offline');
    expect((await readLearning(async () => { throw new Error('network'); })).state).toBe('offline');
    expect((await readLearning(async () => { throw new SignedOutError(); })).state).toBe('signedout');
    expect((await readLearning(async () => 'not json')).state).toBe('notyet');
  });
});

const found: DocLoad = { state: 'found', address: 'texture:nb1', notebook: nb };
describe('N5 the page', () => {
  test('without the route the page says so and shows no numbers', () => {
    const p = page({ doc: found, learning: { state: 'notyet' }, selected: '', steerQueued: 0 });
    expect(p.scoreState).toBe('notyet'); expect(p.score).toEqual([]); expect(p.byKind).toEqual([]); expect(p.scoreNotice).toContain('not available yet');
    expect(p).toMatchObject({ docState: 'ready', docAddress: 'texture:nb1' }); expect(p.sections.length).toBe(4);
    const none = page({ doc: { state: 'none' }, learning: { state: 'offline' }, selected: '', steerQueued: 0 });
    expect(none).toMatchObject({ docState: 'none', scoreState: 'offline' }); expect(none.sections).toEqual([]);
    const down = page({ doc: { state: 'unavailable' }, learning: { state: 'signedout' }, selected: '', steerQueued: 0 });
    expect(down.docState).toBe('offline'); expect(down.docNotice).not.toBe('');
    expect(page({ doc: null, learning: null, selected: '', steerQueued: 0 })).toMatchObject({ docState: 'loading', scoreState: 'loading' });
  });
  test('with the route: counts, rate, kinds, and the rows behind a line', () => {
    const read = parseLearning(learning)!;
    const input = { doc: found, learning: { state: 'ready', read } as const, selected: '', steerQueued: 0 };
    let p = page(input);
    expect(p.score.map((x) => x.label)).toEqual(['Forecasts settled', 'Heard through', 'Still open']); expect(p.score.map((x) => x.value)).toEqual(['11', '8 of 11 (73%)', '2']);
    expect(p.byKind.map((x) => x.label)).toEqual(['Story', 'Podcast clip']); expect(p.byKind.map((x) => x.value)).toEqual(['6 of 8 heard through', '2 of 3 heard through']);
    expect(p.noVerdict).toBe('1 forecast with no verdict: you left, or the app could not play it.');
    const lesson = p.sections[1].lines[0];
    expect(lesson.tag).toBe('heard 9 of 11'); expect(lesson.canWhy).toBe(true);
    p = page({ ...input, selected: lesson.id });
    expect(p.selectedText).toBe(lesson.text);
    expect(p.evidence.map((x) => x.title)).toEqual(['Council keeps libraries open late', 'A regional grid adds its largest battery', 'Harbor fees rise']);
    expect(p.evidence.map((x) => x.outcome)).toEqual(['Not yet heard', 'Heard through', 'Skipped']);
    expect(p.evidence[2].detail).toBe('The desk forecast 80 in 100.');
    p = page({ ...input, selected: p.sections[0].lines[0].id });
    expect(p.evidence).toEqual([]); expect(p.evidenceNote).not.toBe('');
    expect(page({ ...input, steerQueued: 2 }).steerLine).toBe('2 more or less taps waiting to reach your computer.');
    expect(page({ doc: found, learning: { state: 'ready', read: parseLearning(learningEmpty)! }, selected: '', steerQueued: 0 }).score.map((x) => x.value)).toEqual(['0', '0']);
  });
  test('N6 everything held about the listener is listed with where it is read', () => {
    expect(HELD.length).toBeGreaterThanOrEqual(12);
    for (const h of HELD) { expect(h.what).not.toBe(''); expect(h.whereRead).not.toBe(''); }
    const all = HELD.map((h) => h.what).join('\n');
    for (const k of ['autoradio.turn', 'autoradio.forecast', 'autoradio.settlement', 'autoradio.sheet_op', 'autoradio.lesson', 'autoradio.strike', 'autoradio.note', 'autoradio.take', 'autoradio.no_verdict']) expect(all).toContain(k);
    for (const k of ['More and less taps', 'Topics', 'Mail']) expect(all).toContain(k);
    expect(HELD.some((h) => h.whereRead.includes('only on this phone'))).toBe(true);
  });
});

describe('N7 the notebook\'s document', () => {
  const route = (o: { edition?: unknown; fail?: boolean; content?: string }): Get => async (p) => {
    if (o.fail) throw new Error('network');
    if (p === '/api/texture/edition') return o.edition;
    if (p === '/api/texture/documents/nb1') return { document: { current_revision_id: 'r1' } };
    if (p === '/api/texture/revisions/r1') return { revision: { content: o.content ?? notebookMd } };
    throw new HTTPError(404, 'HTTP 404');
  };
  test('found: read through the Texture routes and kept on the phone', async () => {
    files.clear();
    const r = await loadDoc(route({ edition: { found: true, doc_id: 'nb1' } }), storage);
    expect(r.load).toMatchObject({ state: 'found', address: 'texture:nb1' }); expect(r.text).toBe(notebookMd);
    expect(await loadCache(storage)).toBe(notebookMd);
  });
  test('found:false is none; an unanswered ask is not none: the last text read, else unavailable', async () => {
    files.clear();
    expect((await loadDoc(route({ edition: { found: false } }), storage)).load.state).toBe('none');
    expect((await loadDoc(route({ edition: { found: true, doc_id: '' } }), storage)).load.state).toBe('none');
    expect((await loadDoc(route({ fail: true }), storage)).load.state).toBe('unavailable');
    await saveCache(storage, notebookMd as string);
    const back = await loadDoc(route({ fail: true }), storage);
    expect(back.load).toMatchObject({ state: 'found', address: '' }); expect(await lookupEdition(route({ fail: true }))).toBeNull();
    const noBody = await loadDoc(route({ edition: { found: true, doc_id: 'nb1' }, content: '' }), storage);
    expect(noBody.load).toMatchObject({ state: 'found', address: 'texture:nb1' }); // the revision said nothing: the held text stands
  });
  test('the page source answers the page from both reads', async () => {
    files.clear();
    const get: Get = async (p, q) => (p === '/api/autoradio/learning' ? learning : route({ edition: { found: true, doc_id: 'nb1' } })(p, q));
    const p = await notebookPage({ selected: '', steerQueued: 0 }, storage, get);
    expect(p).toMatchObject({ docState: 'ready', scoreState: 'ready' });
    const offline = await notebookPage({ selected: '', steerQueued: 0 }, undefined, async () => { throw new Error('network'); });
    expect(offline).toMatchObject({ docState: 'offline', scoreState: 'offline' });
  });
});
