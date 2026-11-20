// The paper page's data module under bun test: `paperFront` with a canned server and a fake storage.
// Failure list, written before the code (each line has a test below; V1 to V10 and layout 1 to 10 are the Swift lists this replaced):
//  F1 the real front answer (fixtures/paper/front-live-2026-11-18.json, captured from choir.news) becomes a ready page: the newest 60, once each, the
//     live sections in house order with counts from the whole list, the request public and exact (limit=100, no credentials).
//  F2 an empty front is "empty" with its own line, the sections still there.     F3 a server error is "error" with a plain line, never an empty page of nothing.
//  F4 a server error with a saved list is "offline" and shows the saved list.    F5 a malformed body (not JSON, not the object) is an error, never a throw.
//  F6 a story without title or route is dropped; nanosecond timestamps parse.    F7 one ask per list: a view switch asks nothing, a pull to refresh asks again.
//  F8 a category asks `?section=` and uses that list; when it fails the front list is filtered.
//  F9 ordering: higher weight first, ties by newer then route; input order does not matter; opinion never leads.
//  F10 My edition: no picks, picks matching nothing, category pick, subtopic whole words, plural/synonym, more picks first (V1 to V3, V5, V8, V9).
//  F11 a pair that does not fit degrades to a stack narrow and sits side by side wide.  F12 the saved list is written only for the front list.
//  F13 demo mode reads the fixtures and asks nothing.
import { beforeEach, describe, expect, test } from 'bun:test';
import { paperFront, forget, composeFront, CACHE_FILE, type PaperArgs, type PaperFront } from './paper.ts';
import { parseDate, parseList, parseStory, buildMine, sections, parseLiveSections, HOUSE } from './paper-model.ts';
import { ORIGIN } from './api.ts';
import { compose, approxMeasurer } from './paper-layout.ts';
import real from '../fixtures/paper/front-live-2026-11-18.json';
import demoBig from '../fixtures/demo/front-big.json';
import demoEven from '../fixtures/demo/front-even.json';
import demoThin from '../fixtures/demo/front-thin.json';

const args = (o: Partial<PaperArgs> = {}): PaperArgs => ({ view: 'front', edition: '', width: 393, tick: 0, focus: 'paper', picks: '', demo: false, ...o });
const NOW = Date.parse('2026-11-18T13:00:00Z');

type Call = { url: string; init?: RequestInit };
let calls: Call[] = [];
let handler: (url: URL) => Response;
const files = new Map<string, Uint8Array>();
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  },
} as never;

const json = (body: unknown, status = 200, headers: Record<string, string> = { date: new Date(NOW).toUTCString() }): Response =>
  new Response(typeof body === 'string' ? body : JSON.stringify(body), { status, headers });

beforeEach(() => {
  forget(); calls = []; files.clear();
  handler = () => json(real);
  globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return handler(new URL(String(input)));
  }) as typeof fetch;
});

const st = (route: string, title: string, o: Record<string, unknown> = {}) => ({ title, route: { bare: route }, section: 'world', sections: ['world'], importance: 5, ...o });
const body = (stories: unknown[], live: unknown = HOUSE) => ({ stories, sections: live });
const routesOf = (f: PaperFront) => f.bands.flatMap((b) => b.items.map((i) => i.route));

describe('F1 the real front answer', () => {
  test('becomes a ready page, public and exact', async () => {
    const f = await paperFront(args(), storage);
    expect(calls.length).toBe(1);
    expect(calls[0].url).toBe('https://choir.news/api/platform/publications/front?limit=100');
    expect(JSON.stringify(calls[0].init)).not.toMatch(/authorization|cookie/i);
    expect(f.status).toBe('ready');
    expect(f.viewId).toBe('front'); expect(f.viewLabel).toBe('Front page');
    expect(f.count).toBe(60);
    expect(f.routes.length).toBe(new Set(f.routes).size);
    expect(f.routes).toEqual(routesOf(f));
    expect(f.routes.length).toBe(f.titles.length);
    expect(f.sections.map((s) => s.id)).toEqual(['world', 'us', 'politics', 'business', 'tech_ai', 'science', 'health', 'climate', 'culture', 'sports']);
    const all = parseList(real);
    expect(f.sections.find((s) => s.id === 'world')!.count).toBe(all.filter((s) => s.section === 'world' || s.sections.includes('world')).length);
    expect(f.dateLine).toMatch(/^(Tue 17|Wed 18|Thu 19) Nov$/);       // the reader's local date: a day either side of the fixture's UTC day
    expect(f.updated).toMatch(/^Updated /);
    expect(f.bands[0].items[0].size).toBeGreaterThan(0);
  });
});

describe('F2 to F5 the page when the list is empty or the server fails', () => {
  test('F2 an empty front', async () => {
    handler = () => json(body([]));
    const f = await paperFront(args(), storage);
    expect(f.status).toBe('empty'); expect(f.message).toBe('Nothing on the front page yet.');
    expect(f.sections.length).toBe(10); expect(f.bands).toEqual([]);
    expect(files.size).toBe(0);
  });
  test('F3 a server error is an error with a plain line', async () => {
    handler = () => json({ error: 'failed to list stories' }, 500);
    const f = await paperFront(args(), storage);
    expect(f.status).toBe('error');   // the page shows its Try again button for "error", not for "empty"
    expect(f.message).toBe('Could not reach choir.news. Pull to try again.');
    expect(f.message).not.toMatch(/loading/i);
    expect(f.count).toBe(0);
  });
  test('F3 a transport failure is the same', async () => {
    globalThis.fetch = (async () => { throw new TypeError('offline'); }) as typeof fetch;
    const f = await paperFront(args(), storage);
    expect(f.message).toBe('Could not reach choir.news. Pull to try again.');
    expect(f.bands).toEqual([]);
  });
  test('F4 a server error with a saved list is offline and shows it', async () => {
    await paperFront(args(), storage);
    expect(files.has(CACHE_FILE)).toBe(true);
    forget();
    handler = () => json('', 503);
    const f = await paperFront(args(), storage);
    expect(f.status).toBe('offline'); expect(f.message).toBe('Offline. Showing the last front page.');
    expect(f.count).toBe(60);
  });
  test('F5 malformed bodies', async () => {
    for (const b of ['not json', '[1,2]', '{"stories":"x"}', '']) {
      forget(); handler = () => json(b);
      const f = await paperFront(args(), storage);
      expect(f.message).toBe('Could not reach choir.news. Pull to try again.');
      expect(f.bands).toEqual([]);
    }
  });
  test('F5 a corrupt saved list is ignored', async () => {
    files.set(CACHE_FILE, new TextEncoder().encode('{nope'));
    handler = () => json('', 500);
    const f = await paperFront(args(), storage);
    expect(f.message).toBe('Could not reach choir.news. Pull to try again.');
  });
});

describe('F6 parsing', () => {
  test('drops a story without title or route; reads nanoseconds and the route as string', () => {
    expect(parseStory({ title: '', route: { bare: 'r' } })).toBeNull();
    expect(parseStory({ title: 'T' })).toBeNull();
    expect(parseStory({ title: 'T', route: 'plain' })!.route).toBe('plain');
    expect(parseDate('2026-11-18T12:19:13.769525189Z')).toBe(Date.parse('2026-11-18T12:19:13.769Z'));
    expect(parseDate('2026-11-18T12:19:13+01:00')).toBe(Date.parse('2026-11-18T11:19:13Z'));
    expect(parseDate('yesterday')).toBeNull();
    const s = parseStory({ title: 'T', route: { bare: 'r' }, entities: [{ id: 'a', label: 'A' }, { id: '', label: '' }], sections: ['world', '', 3], source_count: 27, version_count: 22 })!;
    expect(s.entities).toEqual([{ id: 'a', label: 'A' }]); expect(s.sections).toEqual(['world']); expect(s.sources).toBe(27);
    expect(parseStory({ title: 'T', route: { bare: 'r' } })!.versions).toBe(1);
    expect(parseStory({ title: 'a', dek: 'Abu Dhabi . The Kremlin earlier: .', route: 'r' })!.dek).toBe('Abu Dhabi. The Kremlin earlier:');
  });
  test('live sections: unique, ordered, labelled', () => {
    expect(parseLiveSections({ sections: [{ id: 'us', label: 'U.S.' }, { id: 'us', label: 'dup' }, { id: '' }, { label: 'x' }, { id: 'tech_ai' }] }))
      .toEqual([{ id: 'us', label: 'U.S.' }, { id: 'tech_ai', label: 'Technology' }]);
  });
});

describe('F7 one ask per list', () => {
  test('a view switch asks nothing; a pull to refresh asks again', async () => {
    await paperFront(args(), storage);
    await paperFront(args({ view: 'mine', focus: 'article' }), storage);
    await paperFront(args({ width: 430 }), storage);
    expect(calls.length).toBe(1);
    await paperFront(args({ tick: 1 }), storage);
    expect(calls.length).toBe(2);
  });
});

describe('F8 categories', () => {
  test('asks ?section= and uses that list', async () => {
    handler = (u) => json(u.searchParams.get('section') === 'sports'
      ? body([st('sp1', 'Sports story one today', { section: 'sports', sections: ['sports'] }), st('sp2', 'Sports story two today', { section: 'sports', sections: ['sports'] })])
      : body([st('w1', 'A world story today')]));
    const f = await paperFront(args({ view: 'sports' }), storage);
    expect(calls.map((c) => new URL(c.url).search)).toEqual(['?limit=100', '?limit=100&section=sports']);
    expect(f.viewId).toBe('sports'); expect(f.sectionLabel).toBe('Sports'); expect(f.count).toBe(2);
    expect(f.routes.sort()).toEqual(['sp1', 'sp2']);
    expect(files.size).toBe(1);                                     // F12 only the front list is saved
    expect(new TextDecoder().decode(files.get(CACHE_FILE)!)).toContain('A world story today');
  });
  test('a category that cannot be asked filters the front list', async () => {
    handler = (u) => (u.searchParams.get('section') ? json({ error: 'invalid section' }, 400) : json(body([
      st('a', 'World one today'), st('b', 'Also sports story', { section: 'world', sections: ['world', 'sports'] })])));
    const f = await paperFront(args({ view: 'sports' }), storage);
    expect(f.routes).toEqual(['b']);
  });
  test('V5/V6/V7: a page of one, a friendly empty category, an unknown id, a slug', () => {
    const live = HOUSE;
    const one = composeFront(parseList(body([st('a', 'World one'), st('b', 'The only sports story', { section: 'sports', sections: ['sports'] })])),
      { view: 'sports', picks: [], category: null, live, width: 393, now: NOW, edition: '', status: 'ready', message: '' });
    expect(one.routes).toEqual(['b']);
    const none = composeFront(parseList(body([st('a', 'World one')])), { view: 'climate', picks: [], category: null, live, width: 393, now: NOW, edition: '', status: 'ready', message: '' });
    expect(none.status).toBe('empty'); expect(none.message).toBe('Nothing in Climate yet.');
    const odd = composeFront(parseList(body([st('a', 'World one')])), { view: 'no_such_section', picks: [], category: null, live, width: 393, now: NOW, edition: '', status: 'ready', message: '' });
    expect(odd.viewId).toBe('no_such_section'); expect(odd.viewLabel).toBe('No such section'); expect(odd.count).toBe(0);
    const slug = composeFront(parseList(body([st('a', 'Tech one', { section: 'tech_ai', sections: ['tech_ai'] })])), { view: 'tech-ai', picks: [], category: null, live, width: 393, now: NOW, edition: '', status: 'ready', message: '' });
    expect(slug.viewId).toBe('tech_ai'); expect(slug.count).toBe(1);
  });
  test('the switcher does not change with the view', async () => {
    const a = await paperFront(args(), storage), b = await paperFront(args({ view: 'sports' }), storage), c = await paperFront(args({ view: 'mine', picks: 'AI' }), storage);
    expect(b.sections).toEqual(a.sections); expect(c.sections).toEqual(a.sections);
  });
  test('without a live list the switcher comes from the stories in house order', () => {
    const s = parseList(body([st('a', 'A', { section: 'Sports' }), st('b', 'B', { section: 'Politics' }), st('c', 'C', { section: 'Zoology' }), st('d', 'D', { section: 'World' })]));
    expect(sections([], s).map((x) => x.label)).toEqual(['World', 'Politics', 'Sports', 'Zoology']);
    expect(sections([], s).map((x) => x.id)).toEqual(['world', 'politics', 'sports', 'zoology']);
  });
});

describe('F9 ordering', () => {
  const comp = (xs: unknown[], width = 393) => compose(parseList(body(xs)), width, NOW);
  test('the front page shows the newest 60 only', async () => {
    handler = () => json(body(Array.from({ length: 90 }, (_, i) => st(`s${i}`, `Story number ${i} of the day`))));
    expect((await paperFront(args(), storage)).count).toBe(60);
  });
  test('input order does not matter; each story once; none invented', () => {
    for (const d of [demoBig, demoEven, demoThin]) {
      const s = parseList(d);
      const a = compose(s, 393, NOW), b = compose([...s].reverse(), 393, NOW);
      expect(a).toEqual(b);
      const r = a.flatMap((x) => x.items.map((i) => i.route));
      expect(r.length).toBe(new Set(r).size);
      expect(r.every((x) => s.some((y) => y.route === x))).toBe(true);
    }
    const dup = comp([st('a', 'A story'), st('a', 'A story again')]);
    expect(dup.flatMap((b) => b.items.map((i) => i.route))).toEqual(['a']);
  });
  test('higher importance and depth lead; equal weights fall to newer then route', () => {
    const b = comp([st('x', 'Low', { importance: 2 }), st('y', 'High', { importance: 10, source_count: 12 })]);
    expect(b[0].items[0].route).toBe('y');
    const t = comp([st('b', 'Same', { updated_at: '2026-11-18T10:00:00Z' }), st('a', 'Same', { updated_at: '2026-11-18T10:00:00Z' }), st('c', 'Same', { updated_at: '2026-11-18T12:00:00Z' })]);
    expect(t.flatMap((x) => x.items.map((i) => i.route))[0]).toBe('c');
  });
  test('empty, one and two stories', () => {
    expect(comp([])).toEqual([]);
    expect(comp([st('a', 'Only story')]).map((b) => b.module)).toEqual(['lead']);
    expect(comp([st('a', 'First story here'), st('b', 'Second story here')]).flatMap((b) => b.items.map((i) => i.route)).sort()).toEqual(['a', 'b']);
  });
  test('opinion never leads; defaults hold', () => {
    const b = comp([st('o', 'A column', { section: 'Opinion', importance: 10, source_count: 30, kind: 'opinion' }), st('n', 'A news story', { section: '' })]);
    expect(b[0].items[0].route).toBe('n'); expect(b[0].items[0].section).toBe('News'); expect(b[b.length - 1].module).toBe('opinion');
  });
  test('a headline too long for any step is clamped at the smallest', () => {
    const it = comp([st('a', 'word '.repeat(80).trim())])[0].items[0];
    expect(it.size).toBe(27); expect(it.lines).toBeLessThanOrEqual(5);
  });
  test('width zero never divides by zero', () => { expect(comp([st('a', 'Story'), st('b', 'Story two')], 0).length).toBeGreaterThan(0); });
});

describe('F11 compositions', () => {
  test('the three demo days compose differently from one document', () => {
    const c = (d: unknown) => composeFront(parseList(d), { view: 'front', picks: [], category: null, live: parseLiveSections(d), width: 393, now: null, edition: '', status: 'ready', message: '' });
    const big = c(demoBig);
    expect(big.composition.startsWith('lead+cluster+pair(stack)+briefs+river')).toBe(true);
    expect(big.composition.endsWith('opinion')).toBe(true);
    expect(big.bands[0].items[0].route).toBe('demo-harbor-barrier'); expect(big.bands[0].items[0].size).toBe(34);
    const cluster = big.bands.find((b) => b.module === 'cluster')!;
    expect(cluster.label).toBe('Developing'); expect(cluster.items.length).toBe(4); expect(cluster.items.every((i) => i.indent)).toBe(true);
    expect(c(demoEven).composition).toBe('colead+pair+briefs+river+opinion');
    expect(c(demoThin).composition).toBe('pair+river');
  });
  test('a pair degrades narrow and sits side by side wide', () => {
    const s = parseList(demoBig);
    const pair = (w: number) => compose(s, w, Date.parse('2026-11-10T07:00:00Z')).find((b) => b.module === 'pair')!;
    expect(pair(393).side).toBe(false); expect(pair(900).side).toBe(true);
  });
  test('the measurer counts lines', () => {
    expect(approxMeasurer('', 17, 600, 300)).toBe(0);
    expect(approxMeasurer('one two', 17, 600, 300)).toBe(1);
    expect(approxMeasurer('word '.repeat(40), 34, 700, 300)).toBeGreaterThan(5);
  });
});

describe('F10 My edition', () => {
  const secs = sections(HOUSE, []);
  const S = (route: string, title: string, o: Record<string, unknown> = {}) => parseStory(st(route, title, o))!;
  const mine = (xs: ReturnType<typeof S>[], picks: string[]) => buildMine(xs, picks, secs);
  const page = (xs: unknown[], picks: string) => paperFront(args({ view: 'mine', picks }), storage);
  test('V1 no picks is the front page with a friendly line', async () => {
    handler = () => json(body(Array.from({ length: 8 }, (_, i) => st(`s${i}`, `Story number ${i} of the day`))));
    const f = await paperFront(args({ view: 'mine', picks: '' }), storage);
    expect(f.mineState).toBe('nopicks'); expect(f.mineFrom).toBe(''); expect(f.note).not.toBe(''); expect(f.status).toBe('ready'); expect(f.count).toBe(8);
    expect((await paperFront(args({ view: 'mine', picks: '  \n...' }), storage)).mineState).toBe('nopicks');
  });
  test('V2 picks that match nothing say so and show the front page', async () => {
    handler = () => json(body([st('a', 'Floods in Spain'), st('b', 'Elections in Peru')]));
    const f = await paperFront(args({ view: 'mine', picks: 'Chess openings' }), storage);
    expect(f.mineState).toBe('nomatch'); expect(f.mineFrom).toBe('Chess openings'); expect(f.note).toContain("Nothing in today's paper");
    expect(f.routes.sort()).toEqual(['a', 'b']);
  });
  test('a category pick takes secondary sections too', () => {
    const r = mine([S('a', 'A', { section: 'world', sections: ['world', 'sports'] }), S('b', 'B'), S('c', 'C', { section: 'business', sections: ['business'] })], ['Sports']);
    expect(r.stories.map((x) => x.route)).toEqual(['a']); expect(r.state).toBe('ready'); expect(r.from).toBe('Sports');
  });
  test('a subtopic matches whole words in title, dek and entities', () => {
    const r = mine([S('a', 'Chipmaker plans new fab', { section: 'business', dek: 'Demand for artificial intelligence hardware' }),
      S('b', 'Said nothing of note'), S('c', 'Regulators meet', { entities: [{ id: 'e', label: 'AI' }] }), S('d', 'Retail sales rise', { section: 'business' })], ['AI']);
    expect(r.stories.map((x) => x.route).sort()).toEqual(['a', 'c']);
  });
  test('V9 plurals, synonyms and one pick said twice', () => {
    const xs = [S('a', 'Election results due'), S('b', 'Voters head to polls')];
    expect(mine(xs, ['Elections']).stories.length).toBe(2); expect(mine(xs, ['election']).stories.length).toBe(2);
    const t = [S('x', 'Phone launch', { section: 'tech_ai', sections: ['tech_ai'] })];
    expect(mine(t, ['Tech', 'Technology']).from).toBe('Technology'); expect(mine(t, ['tech']).stories.length).toBe(1);
  });
  test('V3 odd phrases never throw and never match everything', () => {
    const xs = [S('a', 'Rangers win the derby'), S('b', 'New chess engine beats champions at endgame openings')];
    expect(mine(xs, ['chess openings']).stories.map((x) => x.route)).toEqual(['b']);
    expect(mine(xs, ['x']).state).toBe('nomatch');
    expect(mine(xs, ['\u{1F642} !!']).state).toBe('nopicks');
    expect(mine(xs, ['word '.repeat(80)]).state).toBe('nomatch');
    expect(mine(xs, ['the']).state).toBe('nomatch');
  });
  test('V8 more picks first; equal counts keep the front order', async () => {
    handler = () => json(body([st('a', 'Space agency picks new rocket', { section: 'science', sections: ['science'], importance: 9 }),
      st('b', 'Chip firm backs AI in space', { section: 'tech_ai', sections: ['tech_ai'], importance: 3 }),
      st('c', 'Space telescope sees galaxy', { section: 'science', sections: ['science'], importance: 6 }),
      st('d', 'Mayor opens a bridge', { section: 'us', sections: ['us'], importance: 10 })]));
    const f = await paperFront(args({ view: 'mine', picks: 'Technology\nAI\nSpace' }), storage);
    expect(f.routes).toEqual(['b', 'a', 'c']);
    expect(f.mineFrom).toBe('Technology, AI, Space');
  });
  test('V5 a route twice is listed once; the from line caps at five', () => {
    expect(mine([S('a', 'AI one'), S('a', 'AI one'), S('b', 'AI two')], ['AI']).stories.map((x) => x.route)).toEqual(['a', 'b']);
    expect(mine([S('a', 'AI one')], ['a1', 'b1', 'c1', 'd1', 'e1', 'f1', 'g1']).from).toBe('A1, B1, C1, D1, E1 and 2 more');
  });
  test('the real front with real picks is a selection', async () => {
    const f = await paperFront(args({ view: 'mine', picks: 'Technology\nAI\nSpace' }), storage);
    expect(f.mineState).toBe('ready'); expect(f.count).toBeGreaterThan(3); expect(f.count).toBeLessThan(100);
  });
});

describe('the grants', () => {
  test('app.ts grants the one origin and the one saved file the module uses', async () => {
    const app = await Bun.file(new URL('./app.ts', import.meta.url)).text();
    expect(app).toContain("export const ORIGIN = 'https://choir.news';");
    expect(app).toContain('net.fetch ${ORIGIN}');
    expect(app).toContain(`fs.read ${CACHE_FILE}`);
    expect(app).toContain(`fs.write ${CACHE_FILE}`);
    expect(ORIGIN).toBe('https://choir.news');
  });
});

describe('F13 demo mode', () => {
  test('reads the fixtures, asks nothing, dates the edition by its newest story', async () => {
    const f = await paperFront(args({ demo: true, edition: 'even' }), storage);
    expect(calls.length).toBe(0);
    expect(f.status).toBe('ready'); expect(f.composition).toBe('colead+pair+briefs+river+opinion'); expect(f.edition).toBe('even');
    expect((await paperFront(args({ demo: true }), storage)).status).toBe('ready');
  });
});

describe('the paper masthead date is the reader\'s local date', () => {
  test('11:30 pm local on 19 Nov reads Thu 19 Nov whatever the zone (it was UTC: Fri 20 in the evening in the Americas)', async () => {
    const { dateLine } = await import('./paper-model.ts');
    expect(dateLine(new Date(2026, 10, 19, 23, 30).getTime())).toBe('Thu 19 Nov');
    expect(dateLine(new Date(2026, 10, 20, 0, 10).getTime())).toBe('Fri 20 Nov');
  });
});
