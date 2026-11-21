// The story cards, the launcher and what the paper has learned (cards.ts): every assertion of the Swift CardsTests and LearnedFromPreferencesTests, plus the store and the steering tap.
// Failure list: see the head of cards.ts (1 to 9, the launcher lines, sign-out).
import { beforeEach, describe, expect, test } from 'bun:test';
import * as C from './cards.ts';
import * as R from './recent-pages.ts';
import * as S from './steer.ts';
import * as P from './pages.ts';
import { HTTPError, nextGeneration, resetSeam } from './api.ts';
import type { Story } from './paper-model.ts';

const files = new Map<string, Uint8Array>();
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    rm: async (p: string) => { files.delete(p); },
  },
} as never;
let clock = 1_000_000;
beforeEach(() => { files.clear(); resetSeam(); C.forget(); R.forget(); S.forget(); P.forget(); clock = 1_000_000; C.setClock(() => clock); });

const story = (route: string, title: string, section = 'World', entities: string[] = []): C.StoryFacts => ({ route, title, dek: '', section, entities });
const toldOf = (): C.Told => { const t = new C.Told(() => clock); return t; };

describe('preferences', () => {
  test('1 a story with nothing to learn from is refused; an unknown kind too', () => {
    const p = toldOf();
    expect(p.record('more', story('r', 'Untitled', '', []))).toBeNull();
    expect(p.acts.length).toBe(0);
    expect(p.record('sideways', story('r', 'x', 'World', ['A']))).toBeNull();
  });
  test('2 entities are normalised before matching', () => {
    const p = toldOf();
    p.record('more', story('a', 'A', 'World', ['Harbor Authority']));
    expect(p.lift(story('b', 'B', 'Local', [' harbor  authority ']))).toBeGreaterThan(0.1);
    expect(Math.abs(p.lift(story('c', 'C', 'Tech', ['Grid'])))).toBeLessThan(1e-9);
  });
  test('3 the same gesture within two seconds is one act', () => {
    const p = toldOf();
    expect(p.record('more', story('a', 'A', 'World', ['X']))).not.toBeNull();
    clock += 1;
    expect(p.record('more', story('a', 'A', 'World', ['X']))).toBeNull();
    clock += 5;
    expect(p.record('more', story('a', 'A', 'World', ['X']))).not.toBeNull();
    expect(p.acts.length).toBe(2);
  });
  test('5 the lift is bounded and more and less cancel', () => {
    const s = story('a', 'A', 'World', ['X']);
    const p = toldOf();
    for (let i = 0; i < 20; i++) { clock += 3; p.record('more', s); }
    expect(p.lift(s)).toBeLessThanOrEqual(C.MAX_LIFT + 1e-9);
    expect(p.lift(s)).toBeGreaterThan(0.2);
    const q = toldOf();
    q.record('more', s); clock += 3; q.record('less', s);
    expect(Math.abs(q.lift(s))).toBeLessThan(1e-6);
    const r = toldOf();
    for (let i = 0; i < 20; i++) { clock += 3; r.record('less', s); }
    expect(r.lift(s)).toBeGreaterThanOrEqual(-C.MAX_LIFT - 1e-9);
  });
  test('6 future acts weigh at most one and old acts fade', () => {
    const s = story('a', 'A', 'World', ['X']);
    const p = toldOf();
    p.record('more', s);
    const fresh = p.lift(s);
    clock -= 86_400 * 3;
    expect(Math.abs(p.lift(s) - fresh)).toBeLessThan(1e-9);
    clock = 1_000_000 + 86_400 * 14;
    expect(p.lift(s)).toBeLessThan(fresh * 0.75);
    clock = 1_000_000 + 86_400 * 200;
    expect(p.lift(s)).toBeLessThan(0.005);
  });
  test('7 the order is a stable permutation and a more lifts a related story', () => {
    const list = [story('a', 'A', 'World', ['Harbor']), story('b', 'B', 'Tech', ['Grid']), story('c', 'C', 'Culture', ['Libraries']), story('d', 'D', 'Local', ['Harbor', 'Festival'])];
    const p = toldOf();
    expect(p.order(list).map((x) => x.story.route)).toEqual(['a', 'b', 'c', 'd']);
    p.record('more', list[3]);
    const o = p.order(list).map((x) => x.story.route);
    expect(new Set(o)).toEqual(new Set(['a', 'b', 'c', 'd']));
    expect(o.length).toBe(4);
    expect(o.indexOf('d')).toBeLessThan(3);
    expect(o.indexOf('a')).toBeLessThan(o.indexOf('b'));
    expect(p.order([]).length).toBe(0);
  });
  test('9 forget and reset', () => {
    const p = toldOf();
    expect(p.forget('nope')).toBe(0);
    expect(p.reset()).toBe(0);
    const s = story('a', 'A', 'World', ['X']);
    p.record('more', s); clock += 5; p.record('less', s);
    expect(p.forget('a')).toBe(2);
    expect(p.lift(s)).toBe(0);
    p.record('more', s);
    expect(p.reset()).toBe(1);
  });
  test('the document lines, the notes and why', () => {
    clock = 1_700_000_000;
    const p = toldOf();
    const s = story('a', 'Talks resume', 'World', ['Ukraine', 'Russia']);
    p.record('more', s);
    expect(C.learnedRows(p.acts, true).map((r) => r.line + ' · ' + r.ago)).toEqual(['More like this: Talks resume (Ukraine, Russia) · 14 Nov 22:13']);
    expect(C.note('more', s)).toBe('More on Ukraine');
    expect(C.note('less', s)).toBe('Less of Ukraine');
    expect(C.note('more', story('z', 'z', '', []))).toBe('More like this');
    expect(p.why(s)).toContain('you asked for more on Ukraine (once)');
    expect(p.why(story('q', 'q', 'Tech', ['Grid']))).toContain('Nothing you told it');
  });
  test('entity derivation is weak and a given list wins', () => {
    expect(C.derive('The council of Port Hale approves a barrier')).toEqual(['Port Hale']);
    expect(C.derive('libraries extend evening hours')).toEqual(['Libraries', 'Extend']);
    expect(C.derive('')).toEqual([]);
    expect(C.entitiesOf('r', 'x', ['Given'], { r: ['Mapped'] })).toEqual(['Given']);
    expect(C.entitiesOf('r', 'x', [], { r: ['Mapped'] })).toEqual(['Mapped']);
  });
});

describe('what the paper has learned', () => {
  test('lines are newest first and omit empty entities', () => {
    const l = C.learnedRows([{ at: 10, kind: 'more', route: 'a', title: 'Harbor vote', entities: ['Harbor Authority'], topic: 'world' }, { at: 20, kind: 'less', route: 'b', title: 'Chef opens', entities: [], topic: 'culture' }], true);
    expect(l.map((r) => r.line)).toEqual(['Less of this: Chef opens', 'More like this: Harbor vote (Harbor Authority)']);
    expect(l.map((r) => r.kind)).toEqual(['less', 'more']);
    expect(C.learnedRows([]).length).toBe(0);
  });
});

const S1 = (route: string, title: string, section = 'World', entities: string[] = []): Story => ({ route, title, dek: '', section, words: 0, importance: 0, sources: 0, versions: 1, updated: null, kind: '', author: '', entities: entities.map((e) => ({ id: e, label: e })), sections: [] });

describe('the store', () => {
  test('4 a corrupt file starts empty; a recorded act round-trips and reset empties the file', async () => {
    files.set(C.TOLD_FILE, new TextEncoder().encode('{not json'));
    expect((await C.learnedInfo(storage)).count).toBe(0);
    const front = async () => [S1('a', 'A', 'World', ['X'])];
    expect((await C.storyMore({ address: 'pub:a', surface: 'page', sheet: '' }, storage, front)).message).toBe('More on X');
    C.forget();
    const info = await C.learnedInfo(storage);
    expect(info.count).toBe(1); expect(info.rows[0].line).toBe('More like this: A (X)');
    expect((await C.learnedReset(storage)).ok).toBe(true);
    C.forget();
    expect((await C.learnedInfo(storage)).count).toBe(0);
    expect(info.message).toBe('');
  });
  test('a page that is not a story, or a story not in the front list, answers nothing and records nothing', async () => {
    const front = async () => [S1('a', 'A', 'World', ['X'])];
    expect((await C.storyMore({ address: 'settings:root', surface: 'page', sheet: '' }, storage, front)).message).toBe('');
    expect((await C.storyMore({ address: 'pub:zzz', surface: 'page', sheet: '' }, storage, front)).message).toBe('');
    expect((await C.acts(storage)).length).toBe(0);
  });
  test('sign-out during the read: the old account\'s file does not come back', async () => {
    const front = async () => [S1('a', 'A', 'World', ['X'])];
    await C.storyMore({ address: 'pub:a', surface: 'page', sheet: '' }, storage, front);
    expect(files.has(C.TOLD_FILE)).toBe(true);
    await C.wipeTold(storage);
    expect(files.has(C.TOLD_FILE)).toBe(false);
    expect((await C.acts(storage)).length).toBe(0);
  });
  test('a write that began under an older account generation is refused', async () => {
    const front = async () => { nextGeneration(); return [S1('a', 'A', 'World', ['X'])]; };
    await C.storyMore({ address: 'pub:a', surface: 'page', sheet: '' }, storage, front);
    expect(files.has(C.TOLD_FILE)).toBe(false);
  });
  test('"more of this" queues one steering tap naming the entry on air and the stories it cites', async () => {
    const sheet = JSON.stringify({ entries: [{ id: 'e1', kind: 'page', placed_by: 'desk', state: 'on_air', title: 'Harbor', est_seconds: 30, cites: [{ ref: 'pub:harbor@v2', role: 'subject', label: 'Harbor' }] }] });
    const posted: Record<string, unknown>[] = [];
    const front = async () => [S1('a', 'A', 'World', ['X'])];
    await C.storyMore({ address: 'pub:a', surface: 'page', sheet }, storage, front);
    await S.flush(storage);                       // joins the flush storyMore started (signed out here: the tap stays)
    expect(await S.queued(storage)).toBe(1);
    await S.flush(storage, async (b) => { posted.push(b); });
    expect(posted).toEqual([{ direction: 'more', entry_id: 'e1', stories: ['harbor', 'a'], title: 'Harbor' }]);
    expect(await S.queued(storage)).toBe(0);
  });
});

describe('the launcher', () => {
  const open = [{ address: 'pub:grid', kind: 'reader', title: 'A regional grid adds a battery' }];
  const pl = C.places(undefined);
  const stories = [story('grid', 'A regional grid adds a battery'), story('lib', 'City libraries extend hours', 'Culture', ['Libraries']), story('h', 'Harbor barrier', 'World', ['Gridlock'])];
  test('places have one short line', () => {
    const rows = C.places('A very long headline ' + 'x'.repeat(90) + '\nsecond line', { mail: '3 unread' });
    expect(rows.map((r) => r.slug)).toEqual(['paper', 'textures', 'files', 'mail', 'podcasts', 'activity', 'settings']);
    expect([...rows[0].line].length).toBeLessThanOrEqual(60);
    expect(rows[0].line).not.toContain('\n');
    expect(rows[3].line).toBe('3 unread');
    expect(rows[1].line).toBe('');
  });
  test('search ranks, dedupes and ignores punctuation', () => {
    expect(C.localHits('  ', open, pl, [], stories).length).toBe(0);
    const h = C.localHits('grid', open, pl, [], stories);
    expect(h.filter((x) => x.address === 'pub:grid').length).toBe(1);
    expect(h[0].address).toBe('pub:grid');
    expect(C.localHits('pap', [], pl, [], [])[0].address).toBe('paper:front');
    expect(C.localHits('.*(', open, pl, [], stories).length).toBe(0);
    expect(C.localHits('libr', [], pl, [], stories)[0].address).toBe('pub:lib');
  });
  test('recent pages exclude open ones, repeats and unknown schemes', () => {
    const ev: R.RecentEvent[] = [{ at: 10, kind: 'open', address: 'pub:a' }, { at: 20, kind: 'close', address: 'pub:a' }, { at: 15, kind: 'open', address: 'pub:b' }, { at: 30, kind: 'open', address: 'weird:thing' }, { at: 5, kind: 'open', address: 'settings:root' }];
    expect(R.addresses(ev, new Set(['settings:root']))).toEqual(['pub:a', 'pub:b']);
    expect(R.addresses(ev, new Set(), 1)).toEqual(['pub:a']);
    expect(C.recentTitle('pub:a', [story('a', 'Story A')])).toBe('Story A');
    expect(C.recentTitle('settings:root', [])).toBe('Settings');
  });
  test('launcherData reads the open pages, the shaded stories and the recent pages from the stores', async () => {
    await P.pagesOpen(storage, 'pub:old', 'Old'); await P.pagesClose(storage, 'pub:old');
    await P.pagesOpen(storage, 'pub:grid', 'A regional grid adds a battery');
    const d = await C.launcherData({ q: 'grid' }, storage, async () => [S1('grid', 'A regional grid adds a battery'), S1('lib', 'City libraries', 'Culture', ['Libraries'])]);
    expect(d.hasQuery).toBe(true); expect(d.stories).toBe(2);
    expect(d.hits[0].address).toBe('pub:grid');
    expect(d.recent.map((r) => r.address)).toEqual(['pub:old']);
    expect(d.places[0].line).toBe('A regional grid adds a battery');
    expect(d.searchNote).toBe('');
    const none = await C.launcherData({ q: '' }, storage, async () => { throw new Error('offline'); });
    expect(none.hits).toEqual([]); expect(none.stories).toBe(0); expect(none.hasQuery).toBe(false);
  });
});

describe('steering', () => {
  const tap = (n: number): S.SteerTap => ({ id: `t${n}`, entryID: 'e', direction: 'more', stories: [], title: '', at: '2026-01-01T00:00:00Z' });
  test('S1 and S9 a tap needs an entry; routes lose versions and repeat once', () => {
    expect(S.tapOn(undefined, 'more', 'pub:a')).toBeNull();
    const t = S.tapOn({ id: 'e', kind: 'page', placed_by: 'desk', state: 'on_air', title: 'T', est_seconds: 1, cites: [{ ref: 'pub:x@v3', role: 'subject', label: 'x' }, { ref: 'pub:x', role: 'subject', label: 'x' }] } as never, 'less', 'pub:y@v1')!;
    expect(t.stories).toEqual(['x', 'y']);
  });
  test('S2 the queue keeps at most 200', async () => {
    for (let i = 0; i < 201; i++) await S.enqueue(storage, tap(i));
    expect(await S.queued(storage)).toBe(200);
  });
  test('S3 a missing route keeps the tap and stops; S5 a server error keeps it; S4 400 resends bare and a second 400 drops it', async () => {
    await S.enqueue(storage, tap(1)); await S.enqueue(storage, tap(2));
    let n = 0;
    await S.flush(storage, async () => { n += 1; throw new HTTPError(404, 'x'); });
    expect(n).toBe(1); expect(await S.queued(storage)).toBe(2); expect(S.steerState()).toBe('notyet');
    await S.flush(storage, async () => { throw new HTTPError(500, 'x'); });
    expect(await S.queued(storage)).toBe(2); expect(S.steerState()).toBe('offline');
    const bodies: Record<string, unknown>[] = [];
    await S.flush(storage, async (b) => { bodies.push(b); throw new HTTPError(400, 'x'); });
    expect(bodies.length).toBe(4); expect(Object.keys(bodies[1])).toEqual(['direction', 'entry_id']);      // each tap: full, then bare
    expect(await S.queued(storage)).toBe(0);
  });
  test('S8 two flushes at once send each tap once; S7 a sign-out during a flush leaves no file', async () => {
    await S.enqueue(storage, tap(1));
    let n = 0; let release: () => void = () => undefined;
    const gate = new Promise<void>((r) => { release = r; });
    const post = async () => { n += 1; await gate; };
    const a = S.flush(storage, post), b = S.flush(storage, post);
    for (let i = 0; i < 50 && n === 0; i++) await new Promise((r) => setTimeout(r, 1));
    nextGeneration(); await S.wipeSteer(storage); release(); await Promise.all([a, b]);
    expect(n).toBe(1); expect(files.has(S.STEER_FILE)).toBe(false);
  });
  test('a mute names the handle and no entry', () => { expect(S.body(S.mute('voice-1'), false)).toEqual({ direction: 'mute', handle: 'voice-1' }); });
});
