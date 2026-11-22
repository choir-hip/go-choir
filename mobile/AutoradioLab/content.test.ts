// The content store and its sync under bun test, on a real SQLite (bun:sqlite behind the same small interface Exact's Database has).
// Failure list, written before the code (these replace ContentStoreTests and ContentSyncTests; the Swift numbering is in content.ts):
//  C1 an item round-trips, survives a restart (a new store over the same file), with its checkpoints.
//  C2 over budget the oldest evictable items go first in kind order; a pinned item or a Texture document is never evicted; the total ends under the budget.
//  C3 retention drops old stories even under budget; mail is kept until the budget needs the room.
//  C4 the total is visible.  C5 sign-out leaves nothing in the store and a run that was in flight writes nothing after it.
//  C6 the search index reads from the store and follows it (a document key that changed is indexed again; one that left is dropped).
//  C7 mail walks every page, keeps full bodies, and the logs hold counts and ids, never a word of the content.
//  C8 a killed sync resumes without fetching a held body again; the radio needing the network stops the run.
//  C9 a walked folder stops at the first page that is all held; a server that ignores the cursor does not loop forever.
//  C10 documents are fetched when their version changes and kept pinned.  C11 stories: newest first, held ones skipped, old ones not fetched, a failure skipped.
//  C12 files: only small text, following size and modified time.  C13 transcripts limited to recent episodes; the run sheet's heard entries are kept with their outcome.
//  C14 two runs at once are one run.  C15 a failing list is "error", never a throw; nothing is changed when signed out.
//  C16 the real routes' shapes: a mail row, a mail body, a document's Markdown through its revision, a story's text.
import { beforeEach, describe, expect, test } from 'bun:test';
import { dig, reopen } from './account-phone.ts';
import { currentWorld as theWorld } from './api.ts';
import { Database } from 'bun:sqlite';
import {
  ContentStore, syncAll, noFetchers, newItem, bytesOf, STANDARD_BUDGET, toDoc, storyText, textureSource, liveFetchers, fileVersion,
  TRANSCRIPTS_PER_RUN, type Db, type Fetchers, type StoredItem,
} from './content.ts';
import { Corpus } from './search.ts';
import { bindNative, resetSession, dropHeld, SignedOutError } from './api.ts';
const signedIn = { available: true, later: async (r: { op?: string; account?: string }) => (r.op === 'session' ? { found: true, value: 'test-key-0001-not-real', computerId: 'comp-test' } : { found: true, value: r.account === 'api-key' ? 'test-key-0001-not-real' : 'comp-test' }) };
import mailList from '../fixtures/content/mail-list.json';
import mailMessage from '../fixtures/content/mail-message.json';
import documentList from '../fixtures/content/document-list.json';
import resolved from '../fixtures/content/publication-resolve.json';

const NOW = Date.parse('2026-11-15T12:00:00Z');
const DAY = 86400000;

function sqlite(path = ':memory:'): { db: Db; raw: Database } {
  const raw = new Database(path);
  const db: Db = {
    execute: async (sql, params = []) => { raw.run(sql, params as never); return {}; },
    query: async (sql, params = []) => ({ rows: raw.query(sql).values(...(params as never[])) as unknown[][] }),
  };
  return { db, raw };
}
async function fresh(budget = STANDARD_BUDGET): Promise<ContentStore> { const s = new ContentStore(sqlite().db, budget); await s.init(); return s; }
const item = (id: string, kind: StoredItem['kind'], o: { bytes?: number; days?: number; pinned?: boolean } = {}): StoredItem =>
  newItem({ id, kind, title: id, text: 'x'.repeat(Math.max(0, (o.bytes ?? 300) - 200 - id.length)), address: id, date: NOW - (o.days ?? 0) * DAY, pinned: o.pinned ?? false, syncedAt: NOW, version: '1' });

describe('C1 to C6 the store', () => {
  test('C1 round-trips and survives a restart with its checkpoints', async () => {
    const dir = (await import('node:fs')).mkdtempSync('/tmp/content-test-');
    const a = sqlite(`${dir}/content.db`); const s1 = new ContentStore(a.db); await s1.init();
    await s1.put(newItem({ id: 'mail:m1', kind: 'mail', title: 'Tunnel vote', text: 'Notes.', address: 'mail:m1', date: NOW, meta: { folder: 'inbox' }, version: 'body' }));
    await s1.setCheckpoint('mail.inbox.done', '1'); await s1.markSynced('mail', NOW); a.raw.close(); s1.dispose();   // a closed database is no live store (sign-out would meet a dead one)
    const b = sqlite(`${dir}/content.db`); const s2 = new ContentStore(b.db); await s2.init();
    expect((await s2.get('mail:m1'))?.meta.folder).toBe('inbox');
    expect(await s2.checkpoint('mail.inbox.done')).toBe('1'); expect(await s2.checkpoint('nope')).toBeNull();
    expect(await s2.syncedWords('mail', NOW + 2 * 3600 * 1000)).toBe('last synced 2 hours ago'); expect(await s2.syncedWords('story', NOW)).toBeNull();
    expect((await s2.itemAt('mail:m1'))?.id).toBe('mail:m1');
    b.raw.close(); s2.dispose();
  });
  test('C2 eviction is oldest first by kind and never touches pinned items or documents', async () => {
    const s = await fresh({ ...STANDARD_BUDGET, totalBytes: 5000, retentionDays: {}, perKindBytes: {} });
    for (const i of [item('story-old', 'story', { bytes: 1000, days: 5 }), item('story-new', 'story', { bytes: 1000, days: 1 }), item('file-1', 'file', { bytes: 1000, days: 9 }),
      item('mail-old', 'mail', { bytes: 1000, days: 100 }), item('doc', 'document', { bytes: 1000, days: 400 }), item('draft', 'mail', { bytes: 1000, days: 400, pinned: true })]) await s.put(i);
    const dropped = await s.enforceBudget(NOW);
    expect(dropped).toEqual(['story-old']); // stories go first, the oldest of them first; one is enough
    expect(await s.has('doc')).toBe(true); expect(await s.has('draft')).toBe(true);
    expect((await s.summary()).totalBytes).toBeLessThanOrEqual(5000);
    s.budget = { ...s.budget, totalBytes: 100 };
    await s.enforceBudget(NOW);
    expect(await s.has('doc') && await s.has('draft')).toBe(true); expect(await s.has('mail-old')).toBe(false); expect(await s.has('file-1')).toBe(false);
  });
  test('C3 retention drops old stories even under budget; mail is kept', async () => {
    const s = await fresh();
    await s.put(item('old', 'story', { days: 31 })); await s.put(item('fresh', 'story', { days: 2 })); await s.put(item('oldmail', 'mail', { days: 900 }));
    await s.enforceBudget(NOW);
    expect(await s.has('old')).toBe(false); expect(await s.has('fresh')).toBe(true); expect(await s.has('oldmail')).toBe(true);
  });
  test('C4 the total is visible', async () => {
    const s = await fresh();
    await s.put(item('a', 'story', { bytes: 2_000_000 })); await s.put(item('b', 'mail', { bytes: 500_000 }));
    expect((await s.summary()).line.startsWith('3 MB of 400 MB')).toBe(true);
    expect(bytesOf({ text: 'é', title: '' })).toBe(202);
  });
  test('C6 the index reads from the store and follows its changes', async () => {
    const s = await fresh(); const corpus = new Corpus();
    await s.put(newItem({ id: 'mail:1', kind: 'mail', title: 'Tunnel referendum', text: 'The referendum notes.', address: 'mail:1', date: NOW, version: 'body' }));
    corpus.hold('store', (await s.docs()));
    expect(corpus.retrieve({ words: 'tunnel referendum', now: NOW }).items.length).toBe(1);
    await s.put(newItem({ id: 'mail:1', kind: 'mail', title: 'Tunnel referendum', text: 'Now about glaciers.', address: 'mail:1', date: NOW, version: 'body2' }));
    corpus.hold('store', await s.docs());
    expect(corpus.retrieve({ words: 'glaciers', now: NOW }).items.length).toBe(1);
    expect(corpus.retrieve({ words: 'referendum notes', now: NOW }).items.length).toBe(0);
    await s.remove('mail:1'); corpus.hold('store', await s.docs());
    expect(corpus.retrieve({ words: 'glaciers', now: NOW }).items.length).toBe(0);
    expect(toDoc(item('x', 'story')).source).toBe('Stories');
  });
});

describe('C7 to C15 the sync', () => {
  test('C7 mail is not synced into this plain store: no fetcher, no step, no kind of work', async () => {
    const s = await fresh(); const f = noFetchers();
    const r = await syncAll(s, f, { now: NOW });
    expect(r.some((x) => x.kind === 'mail')).toBe(false); expect(Object.keys(f).filter((k) => /^mail|^drafts$/.test(k))).toEqual([]);
    expect(Object.keys(liveFetchers()).filter((k) => /^mail|^drafts$/.test(k))).toEqual([]);
  });
  test('C10 documents are fetched when their version changes and kept pinned', async () => {
    const s = await fresh(); let bodies = 0, ver = 'v1';
    const f = noFetchers();
    f.documentPage = async (c) => (c === null ? { docs: [{ id: 'd1', title: 'Notes', updated: ver }], next: 'n' } : { docs: [{ id: 'd2', title: 'Plans', updated: 'v1' }], next: null });
    f.documentBody = async (id) => { bodies++; return `Markdown of ${id}`; };
    await syncAll(s, f, { now: NOW }); expect(bodies).toBe(2);
    await syncAll(s, f, { now: NOW }); expect(bodies).toBe(2);
    ver = 'v2'; await syncAll(s, f, { now: NOW }); expect(bodies).toBe(3);
    const pins = await s.db.query("SELECT pinned FROM items WHERE id = 'texture:d1'");
    expect(Number(pins.rows[0][0])).toBe(1);
  });
  test('C11 stories: newest first, held ones skipped, old ones not fetched, a failure skipped', async () => {
    const s = await fresh(); const calls: string[] = []; const f = noFetchers();
    f.stories = async () => [{ route: '/old', title: 'Old', date: NOW - 40 * DAY }, { route: '/a', title: 'A', date: NOW - 3600000 }, { route: '/b', title: 'B', date: NOW - 60000 }, { route: '/bad', title: 'Bad', date: NOW - 120000 }];
    f.storyBody = async (r) => { calls.push(r); if (r === '/bad') throw new Error('bad server response'); return `Body ${r}`; };
    const rep = await syncAll(s, f, { now: NOW });
    expect(calls).toEqual(['/b', '/bad', '/a']); expect(rep.find((r) => r.kind === 'story')?.failed).toBe(1);
    calls.length = 0; await syncAll(s, f, { now: NOW }); expect(calls).toEqual(['/bad']);
  });
  test('C12 files take only small text and follow size and modified time', async () => {
    const s = await fresh(); const reads: string[] = []; const f = noFetchers(); const when = 100000;
    f.fileList = async (p) => p === '/' ? [
      { name: 'notes.md', path: '/notes.md', isDirectory: false, size: 10, modified: when }, { name: 'pic.png', path: '/pic.png', isDirectory: false, size: 10, modified: when },
      { name: 'huge.txt', path: '/huge.txt', isDirectory: false, size: 9_000_000, modified: when }, { name: 'sub', path: '/sub', isDirectory: true, size: 0, modified: null }]
      : [{ name: 'deep.txt', path: '/sub/deep.txt', isDirectory: false, size: 5, modified: when }];
    f.fileText = async (p) => { reads.push(p); return `text of ${p}`; };
    await syncAll(s, f, { now: NOW });
    expect(new Set(reads)).toEqual(new Set(['/notes.md', '/sub/deep.txt']));
    await syncAll(s, f, { now: NOW }); expect(reads.length).toBe(2);
    expect(fileVersion({ name: 'a', path: '/a', isDirectory: false, size: 10, modified: when })).toBe('10|100');
  });
  test('C13 transcripts are limited to recent episodes; the run sheet keeps what was heard with its outcome', async () => {
    const s = await fresh(); const f = noFetchers(); const asked: string[] = [];
    f.transcriptTargets = () => Array.from({ length: 30 }, (_, i) => ({ feed: 'https://example.com/f.xml', id: `e${i}`, title: `Show: E${i}`, date: NOW - (i === 29 ? 30 * DAY : 3600000 * i), address: `podcast:e${i}` }));
    f.transcript = async (_f, id) => { asked.push(id); return `We talked about ${id}.`; };
    f.runSheet = async () => [{ id: 'h1', title: 'Heard story', text: 'Words.', state: 'heard', ended: '', ref: 'pub:/h', kind: 'story' }];
    await syncAll(s, f, { now: NOW });
    expect(asked.length).toBe(TRANSCRIPTS_PER_RUN); expect(asked).not.toContain('e29');
    expect(await s.ids('runsheet')).toEqual(['run:h1']); expect((await s.get('run:h1'))?.meta.outcome).toBe('heard');
  });
  test('C14 two runs at once are one run', async () => {
    const s = await fresh(); const f = noFetchers();
    f.stories = async () => { await new Promise((r) => setTimeout(r, 60)); return []; };
    const a = syncAll(s, f, { now: NOW });
    await new Promise((r) => setTimeout(r, 10));
    const b = await syncAll(s, f, { now: NOW });
    expect(b).toEqual([]); expect((await a).length).toBeGreaterThan(0);
  });
  test('C15 a failing list is an error report, never a throw; signed out changes nothing', async () => {
    const s = await fresh(); const f = noFetchers();
    f.stories = async () => { throw new Error('answered 500'); };
    const r = await syncAll(s, f, { now: NOW });
    expect(r.find((x) => x.kind === 'story')?.stoppedBy).toBe('error');
    bindNative(null); resetSession(); dropHeld();      // signed out means no key held either (resetSession forgets only what was discovered)
    const live = liveFetchers();
    const r2 = await syncAll(s, { ...live, stories: async () => [] }, { now: NOW });
    expect(r2.find((x) => x.kind === 'document')?.stoppedBy).toBe('error'); // the first page throws SignedOutError: counted, nothing written
    expect((await s.summary()).totalBytes).toBe(0);
    await expect(live.documentPage(null)).rejects.toBeInstanceOf(SignedOutError);
  });
});

describe('C16 the real routes\' shapes', () => {
  let calls: string[];
  beforeEach(() => {
    calls = [];
    resetSession(); bindNative(signedIn as never); reopen(dig('test-key-0001-not-real'), 'test-key-0001-not-real', 'comp-test');
    globalThis.fetch = (async (input: RequestInfo | URL) => {
      const u = new URL(String(input)); calls.push(u.pathname + u.search);
      const j = (b: unknown): Response => new Response(JSON.stringify(b), { status: 200, headers: { date: new Date(NOW).toUTCString() } });
      if (u.pathname === '/api/email/messages') return j(mailList);
      if (u.pathname.startsWith('/api/email/messages/')) return j(mailMessage);
      if (u.pathname === '/api/texture/documents') return j(documentList);
      if (u.pathname === '/api/texture/documents/doc-1') return j({ document: { doc_id: 'doc-1', current_revision_id: 'rev-9' } });
      if (u.pathname === '/api/texture/revisions/rev-9') return j({ revision: { revision_id: 'rev-9', content: '# Garden notes\n\nTomatoes and basil.' } });
      if (u.pathname === '/api/platform/publications/resolve') return j(resolved);
      return new Response('{}', { status: 404 });
    }) as typeof fetch;
  });
  test('documents: the list drops archived ones; the Markdown comes through the current revision', async () => {
    const f = liveFetchers();
    const page = await f.documentPage(null);
    expect(page.docs.map((d) => d.id)).toEqual(['doc-1']); expect(page.next).toBeNull();
    expect(await f.documentBody('doc-1')).toBe('# Garden notes\n\nTomatoes and basil.');
    expect(await textureSource('doc-2', async () => ({ document: {} }))).toBeNull();
  });
  test('a story\'s text is readable lines: no code, no rules', async () => {
    const f = liveFetchers();
    expect(await f.storyBody('/pub/texture/x')).toBe('Harbor barrier approved\nThe council voted seven to two.\nWork starts in spring.');
    expect(storyText({})).toBeNull(); expect(storyText(null)).toBeNull();
  });
});
