// The Textures place's data module under bun test (textures.ts). Ported from the Swift tests this replaces (TextureDocTests.testTextureList, RealShapeTests.testTextureListRealShape and
// testTheServersEditionIsPinnedFirst, and the sign-in wording of TextureList.load), then the states and the words:
//  L1 the edition is pinned first, documents with no id are hidden, an empty title is "Untitled", the meta line is "3 versions · date". Only the fields the route sends are read.
//  L2 an empty list is ready with no edition; a body that is not a list is an error, not an empty list.   L3 the real shape of the server's list (fractional seconds, extra fields).
//  L4 the server's own answer to "which document is my edition" is pinned first; an edition older than the page is still shown; not found or none changes nothing.
//  L5 the dates are UTC "MMM d, yyyy" and "" when unreadable.
//  L6 load: two requests (the list, the edition); a missing edition answer does not fail the list.
//  L7 the words: no key, locked, refused, unknown, any other failure; never "Loading", and "Sign in" only when signing in is the answer.
//  N1 New document posts one create to the computer and answers the new document's address; N2 every failure answers words and no address, never a throw; N3 a reply with no id is a failure in words.
//  L8 the source: any page but the Textures place answers the last list; another account (a new account generation) never sees it.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as T from './textures.ts';
import { SignedOutError, HTTPError, resetSeam, nextGeneration, type Reply } from './api.ts';
import { WORDS } from './words.ts';

const reply = (body: unknown): Reply => ({ body, serverTime: 0, raw: '' });
beforeEach(() => { resetSeam(); T.resetTexturesForTest(); });

describe('parse (the Swift list tests)', () => {
  test('L1 the edition is pinned first; id-less documents are hidden; Untitled; the meta line', () => {
    const l = T.parseList({ documents: [
      { doc_id: 'a', title: 'Meeting notes', updated_at: '2026-11-09T10:00:00Z', revision_count: 3 },
      { doc_id: 'b', title: 'My edition', updated_at: '2026-11-08T10:00:00Z', revision_count: 1 },
      { title: 'no id' },
      { doc_id: 'd', title: '' },
    ] });
    expect(l.rows.map((r) => r.address)).toEqual(['texture:b', 'texture:a', 'texture:d']);
    expect(l.hasEdition).toBe(true);
    expect(l.rows[1].meta).toBe('3 versions · Nov 9, 2026');
    expect(l.rows[2].title).toBe('Untitled'); expect(l.rows[2].meta).toBe('');
    expect(T.parseList({ documents: [{ doc_id: 'x', title: 'One', revision_count: 1, updated_at: '2026-11-09T10:00:00Z' }] }).rows[0].meta).toBe('1 version · Nov 9, 2026');
  });
  test('L2 an empty list is ready with no edition', () => {
    const l = T.parseList({ documents: [] });
    expect(l.hasEdition).toBe(false); expect(l.status).toBe('ready'); expect(l.rows).toEqual([]);
  });
  test('L2b a 200 that is not a list (null, [], an object without documents, a string) is an error, not an empty ready list', async () => {
    for (const body of [null, [], {}, { documents: null }, 'ok']) {
      expect(() => T.parseList(body)).toThrow();
      const l = await T.loadTextures({ json: async (p) => reply(p.endsWith('/edition') ? { found: false } : body) });
      expect(l.status).toBe('error'); expect(l.message).toMatch(/^Could not load your documents\./); expect(l.rows).toEqual([]);
    }
  });
  test('L3 the real shape: fractional seconds, extra fields, one row, five versions', () => {
    const l = T.parseList({ documents: [{ doc_id: 'd1', owner_id: 'o', title: 'Harbor notes', computer_id: 'computer-x', trajectory_id: 't', current_revision_id: 'r4', current_version_number: 4,
      created_at: '2026-11-10T23:26:43.085Z', updated_at: '2026-11-10T23:28:13.500Z', revision_count: 5, last_editor: 'appagent', last_author_kind: 'appagent' }], next_cursor: '' });
    expect(l.status).toBe('ready'); expect(l.rows.length).toBe(1); expect(l.rows[0].address).toBe('texture:d1'); expect(l.hasEdition).toBe(false);
    expect(l.rows[0].meta).toContain('5 versions'); expect(l.rows[0].meta).toContain('Nov 10, 2026');
  });
});

describe('the server\'s edition', () => {
  const base = () => T.parseList({ documents: [{ doc_id: 'a', title: 'Notes' }, { doc_id: 'd9', title: 'Morning paper' }] });
  test('L4 pinned first, shown even when older than the page, nothing when not found', () => {
    const pinned = T.applyEdition(base(), { found: true, docID: 'd9', updatedAt: '' });
    expect(pinned.rows.map((r) => r.address)).toEqual(['texture:d9', 'texture:a']); expect(pinned.rows[0].edition).toBe(true); expect(pinned.hasEdition).toBe(true);
    const absent = T.applyEdition(base(), { found: true, docID: 'old', updatedAt: '2026-11-11T05:00:00Z' });
    expect(absent.rows[0]).toMatchObject({ address: 'texture:old', title: 'My edition', edition: true, meta: 'Nov 11, 2026' });
    expect(T.applyEdition(base(), { found: false, docID: '', updatedAt: '' })).toEqual(base());
    expect(T.applyEdition(base(), null)).toEqual(base());
  });
  test('L4b the answer of GET /api/texture/edition: found without a document id cannot be opened; garbage is nothing', () => {
    expect(T.parseEdition({ found: false })).toEqual({ found: false, docID: '', updatedAt: '' });
    expect(T.parseEdition({ found: true }).found).toBe(false);
    expect(T.parseEdition({ found: true, doc_id: 'd9', revision_id: 'r2', updated_at: '2026-11-11T05:00:00Z' })).toEqual({ found: true, docID: 'd9', updatedAt: '2026-11-11T05:00:00Z' });
    expect(T.parseEdition([])).toBeNull();
  });
});

describe('dates', () => {
  test('L5 UTC, "MMM d, yyyy", empty when unreadable', () => {
    expect(T.shortDate('2026-11-09T23:59:59Z')).toBe('Nov 9, 2026'); expect(T.shortDate('2026-11-10T23:28:13.500Z')).toBe('Nov 10, 2026');
    expect(T.shortDate('')).toBe(''); expect(T.shortDate('yesterday')).toBe('');
  });
});

describe('load', () => {
  test('L6 the list then the edition; a failing edition lookup does not fail the list', async () => {
    const paths: string[] = [];
    const ok = await T.loadTextures({ json: async (p) => { paths.push(p); return reply(p.endsWith('/edition') ? { found: true, doc_id: 'e1', updated_at: '2026-11-11T05:00:00Z' } : { documents: [{ doc_id: 'a', title: 'Notes' }] }); } });
    expect(paths.sort()).toEqual(['/api/texture/documents', '/api/texture/edition']);
    expect(ok.rows.map((r) => r.address)).toEqual(['texture:e1', 'texture:a']);
    const noEd = await T.loadTextures({ json: async (p) => { if (p.endsWith('/edition')) throw new HTTPError(404, 'x'); return reply({ documents: [{ doc_id: 'a', title: 'Notes' }] }); } });
    expect(noEd.status).toBe('ready'); expect(noEd.rows.length).toBe(1);
  });
  test('L7 the words by reason: sign in only when no key; unlock when locked; again when refused; neutral when unknown; the server\'s trouble otherwise', async () => {
    const fail = (e: unknown) => T.loadTextures({ json: async () => { throw e; } });
    expect(await fail(new SignedOutError())).toMatchObject({ status: 'error', message: 'Sign in to see your documents.' });
    expect((await fail(new SignedOutError('x', 'locked'))).message).toBe(WORDS.locked);
    expect((await fail(new SignedOutError('x', 'refused'))).message).toBe('Sign in again to see your documents.');
    expect((await fail(new SignedOutError('x', 'unknown'))).message).toBe(WORDS.unknown);
    const other = await fail(new HTTPError(503, 'HTTP 503'));
    expect(other.status).toBe('error'); expect(other.message).toMatch(/^Could not load your documents\./); expect(other.message).not.toMatch(/sign in/i);
    for (const l of [await fail(new SignedOutError()), other]) { expect(l.status).not.toBe('loading'); expect(JSON.stringify(l)).not.toMatch(/Loading/); }
  });
});

describe('the source', () => {
  test('L8 only the Textures place loads; other pages answer the last list; the first answer is loading-free (idle shape)', async () => {
    let n = 0;
    const net = { json: async (p: string) => { n++; return reply(p.endsWith('/edition') ? { found: false } : { documents: [{ doc_id: 'a', title: 'Notes' }] }); } };
    const idle = await T.texturesListShared('paper:front', net);
    expect(idle.rows).toEqual([]); expect(n).toBe(0);
    const first = await T.texturesListShared('textures:', net);
    expect(first.rows.length).toBe(1); expect(n).toBe(2);
    expect((await T.texturesListShared('radio:', net)).rows.length).toBe(1); expect(n).toBe(2);       // the last list, no request
    nextGeneration();                                                                              // another account: the last list is not served, and is dropped
    expect((await T.texturesListShared('radio:', net)).rows).toEqual([]);
    expect(n).toBe(2);
  });
});

describe('New document (create, then open)', () => {
  type Call = { path: string; opts?: { method?: string; body?: unknown } };
  test('N1 one POST /api/texture/documents with a title; the answer is the new document\'s address, ready, no message', async () => {
    const calls: Call[] = [];
    const m = await T.createTexture({ json: async (path, opts) => { calls.push({ path, opts }); return reply({ doc_id: 'd-new', owner_id: 'o', title: 'Untitled', created_at: '2026-11-24T01:02:03.000Z' }); } });
    expect(calls.length).toBe(1);
    expect(calls[0].path).toBe('/api/texture/documents'); expect(calls[0].opts?.method).toBe('POST');
    expect((calls[0].opts?.body as { title: string }).title.trim()).not.toBe('');                   // the route refuses an empty title (400 "title is required")
    expect(m).toEqual({ address: 'texture:d-new', title: 'Untitled', status: 'ready', message: '' });
  });
  test('N2 a failure is words and no address, never a throw, and never says Sign in unless that is the answer', async () => {
    const fail = (e: unknown) => T.createTexture({ json: async () => { throw e; } });
    const none = await fail(new SignedOutError());
    expect(none).toMatchObject({ address: '', status: 'error', message: 'Sign in to make a document.' });
    expect((await fail(new SignedOutError('x', 'locked'))).message).toBe(WORDS.locked);
    expect((await fail(new SignedOutError('x', 'refused'))).message).toBe('Sign in again to make a document.');
    expect((await fail(new SignedOutError('x', 'unknown'))).message).toBe(WORDS.unknown);
    const e503 = await fail(new HTTPError(503, 'HTTP 503'));
    expect(e503.address).toBe(''); expect(e503.status).toBe('error'); expect(e503.message).toMatch(/^Could not make a document\. /); expect(e503.message).not.toMatch(/sign in/i);
    const down = await fail(new Error(''));
    expect(down.message).toBe('Could not make a document. the computer did not answer');
  });
  test('N3 a reply with no document id (or not an object) cannot be opened: words, no address', async () => {
    for (const body of [null, {}, { doc_id: '' }, [], 'ok']) {
      const m = await T.createTexture({ json: async () => reply(body) });
      expect(m.address).toBe(''); expect(m.status).toBe('error'); expect(m.message).toMatch(/^Could not make a document\./);
    }
  });
});
