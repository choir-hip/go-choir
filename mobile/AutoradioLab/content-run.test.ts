// The content store opened and synced in shared code (content-run.ts), under bun test; the store and the sync themselves are content.ts (content.test.ts C1 to C16).
// Replaces the Swift ContentBoot, ContentSync.kick and LocalStories (OpeningCacheTests' offline fill). Failure list, written before the code:
//  S1 the store is opened once per process and made; a second ask gets the same store; no storage or an open that fails is "unavailable", never a throw
//  S2 after a sign-out wipe (the store's generation moved) the next ask closes the old handle and opens a fresh store (the db file was removed under it)
//  S3 one run per key: the same key answers the last report without fetching; a new key runs again; two at once are one run
//  S4 signed out changes nothing and says so; offline (no server time) changes nothing and says so
//  S5 the radio on the network (native fact) stops the run before the next request
//  S6 the narrow reads other lanes use: every held item as search documents, the item at an address
//  S7 the stories held on the phone, for when the desk cannot be reached: newest first, not already told, ids made safe, empty ones skipped, at most `limit`
//  S8 the grants name the database
import { beforeEach, describe, expect, test } from 'bun:test';
import { Database } from 'bun:sqlite';
import { contentStore, forgetContent, syncContent, contentDocs, contentAt, localStories, type Env } from './content-run.ts';
import { newItem, noFetchers, type Fetchers, type Db } from './content.ts';
import { SignedOutError } from './api.ts';
import { grants } from './app.ts';

const NOW = Date.parse('2026-11-21T12:00:00Z');
function adapter(): Db & { close(): Promise<void> } {
  const raw = new Database(':memory:');
  return { execute: async (sql, p = []) => { raw.run(sql, p as never); return {}; }, query: async (sql, p = []) => ({ rows: raw.query(sql).values(...(p as never[])) as unknown[][] }), close: async () => raw.close() };
}
let opened: number, closed: number;
const storage = { sqlite: { open: async () => { opened++; const d = adapter(); const c = d.close; d.close = async () => { closed++; await c(); }; return d; } } } as never;
const fetchers = (o: Partial<Fetchers> = {}): Fetchers => ({ ...noFetchers(), ...o });
const env = (f: Fetchers, o: Partial<Env> = {}): Env => ({ fetchers: f, now: async () => NOW, busy: async () => false, ...o });
beforeEach(async () => { await forgetContent(); opened = 0; closed = 0; });   // forgetContent now closes the last test's store: let that finish before counting

describe('S1 and S2 opening', () => {
  test('S1 opened once, made, the same store again; nothing to open with is unavailable', async () => {
    const a = await contentStore(storage); const b = await contentStore(storage);
    expect(a).not.toBeNull(); expect(b).toBe(a); expect(opened).toBe(1);
    await a!.put(newItem({ id: 'x', kind: 'story', title: 't', text: 'b', address: 'pub:/x' }));
    expect((await b!.get('x'))?.title).toBe('t');
    forgetContent();
    expect(await contentStore(undefined)).toBeNull();
    expect(await contentStore({ sqlite: { open: async () => { throw new Error('disk'); } } } as never)).toBeNull();
  });
  test('S2 after a wipe the old handle is closed and a fresh store opens', async () => {
    const a = (await contentStore(storage))!;
    await a.put(newItem({ id: 'x', kind: 'story', title: 't', text: 'b', address: 'pub:/x' }));
    await a.wipe();
    const b = (await contentStore(storage))!;
    expect(b).not.toBe(a); expect(opened).toBe(2); expect(closed).toBe(1); expect(await b.get('x')).toBeNull();
  });
});

describe('A-5 forgetContent closes what it drops', () => {
  test('the handle is closed and the store disposed', async () => {
    const a = (await contentStore(storage))!;
    let disposed = 0; const d = a.dispose.bind(a); a.dispose = () => { disposed++; d(); };
    await forgetContent();
    expect(disposed).toBeGreaterThanOrEqual(1); expect(closed).toBe(1);
  });
});

describe('S3 to S5 the run', () => {
  const story = { route: '/p/a', title: 'A', date: NOW - 3600_000 };
  test('S3 one run per key', async () => {
    let n = 0;
    const f = fetchers({ stories: async () => { n++; return [story]; }, storyBody: async () => 'The body.' });
    const r1 = await syncContent('k1', storage, env(f));
    expect(r1.state).toBe('done'); expect(r1.line).toContain('1 stories'); expect(n).toBe(1);
    expect((await contentStore(storage))?.get('pub:/p/a')).resolves.toMatchObject({ text: 'The body.' });
    expect((await syncContent('k1', storage, env(f))).state).toBe('done'); expect(n).toBe(1);
    await syncContent('k2', storage, env(f)); expect(n).toBe(2);
    forgetContent();
    const slow = fetchers({ stories: async () => { n++; await new Promise((r) => setTimeout(r, 20)); return []; } });
    n = 0; await Promise.all([syncContent('k3', storage, env(slow)), syncContent('k3', storage, env(slow))]); expect(n).toBe(1);
  });
  test('S3b the launch sync never walks the files (stories, documents and the small kinds only)', async () => {
    let walked = 0;
    const r = await syncContent('kf', storage, env(fetchers({ fileList: async () => { walked++; return []; } })));
    expect(r.state).toBe('done'); expect(walked).toBe(0); expect(r.line).not.toContain('file');
  });
  test('S4 signed out and offline change nothing', async () => {
    const out = await syncContent('k', storage, env(fetchers({ documentPage: async () => { throw new SignedOutError(); } })));
    expect(out.state).toBe('signedout');
    forgetContent();
    let asked = 0;
    const off = await syncContent('k', storage, env(fetchers({ stories: async () => { asked++; return []; } }), { now: async () => null }));
    expect(off.state).toBe('offline'); expect(asked).toBe(0);
    forgetContent();
    expect((await syncContent('k', undefined, env(fetchers()))).state).toBe('unavailable');
  });
  test('S5 the radio on the network stops the run', async () => {
    let bodies = 0;
    const f = fetchers({ stories: async () => [story], storyBody: async () => { bodies++; return 'x'; } });
    const r = await syncContent('k', storage, env(f, { busy: async () => true }));
    expect(bodies).toBe(0); expect(r.line).toContain('radio');
  });
});

describe('S6 to S8 reads and grants', () => {
  test('S6 the held items as documents, and the item at an address', async () => {
    const s = (await contentStore(storage))!;
    await s.put(newItem({ id: 'pub:/a', kind: 'story', title: 'A', text: 'aaa', address: 'pub:/a' }));
    expect((await contentDocs(storage)).map((d) => d.id)).toEqual(['store:pub:/a']);
    expect((await contentAt(storage, 'pub:/a'))?.title).toBe('A'); expect(await contentAt(storage, 'pub:/none')).toBeNull();
    forgetContent(); expect(await contentDocs(undefined)).toEqual([]);
  });
  test('S7 the stories held on the phone for an unreachable desk', () => {
    const it = (id: string, title: string, text: string, date: number | null) => newItem({ id, kind: 'story', title, text, address: id, date });
    const out = localStories([it('story:1', 'Tide tables', 'The tide table changes.', 3), it('story:2', 'Market hours', 'Hours change.', 5), it('story:3', 'Harbour lights', 'Lights.', 4), it('story:4', '  ', 'no title', 9), it('story:5', 'Empty', '  ', 9), it('s/ä 6', 'Told', 'Said already.', 1)],
      { told: ['told'], held: new Set(), limit: 2 });
    expect(out.map((e) => e.title)).toEqual(['Market hours', 'Harbour lights']);
    expect(out[0]).toMatchObject({ id: 's-story-2', address: 'story:2' });
    expect(localStories([it('s/ä 6', 'X', 'y', 1)], { told: [], held: new Set(), limit: 5 })[0].id).toBe('s-s---6');
    expect(localStories([it('a', 'A', 'b', 1)], { told: [], held: new Set(['s-a']), limit: 5 })).toEqual([]);
    // the run sheet's bounds: 8 KiB of text and a 300-byte one-line title (a longer entry would be dropped as invalid by the sheet)
    const big = localStories([it('b', 'T'.repeat(500), 'x'.repeat(20000), 1)], { told: [], held: new Set(), limit: 5 })[0];
    expect(new TextEncoder().encode(big.text).length).toBeLessThanOrEqual(8192); expect(new TextEncoder().encode(big.title).length).toBeLessThanOrEqual(300);
  });
  test('S8 the grants name the database', () => { expect(grants).toContain('sqlite.open app:/data/content.db'); });
});
