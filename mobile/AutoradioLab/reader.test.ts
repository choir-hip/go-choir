// The shared reader (reader.ts): the story or document at an address, held for the session, and the page's actions. Failure list, written first:
//  R1 a `pub:` story: the public route (no key) and its related list are asked; blocks, title, related and the permalink come back; status ready.
//  R2 failures say so in words: a failed story is an error doc (never a spinner); a document with no key, a locked phone, a refused key and an unknown state each have their words.
//  R3 an address that is not a story or a document is "loading" (another page owns it) and nothing is asked.
//  R4 a `texture:` document: the document, its revision and its revision list are asked with the key; the head is editable and carries its Markdown and id; versions, revising and the trajectory come through.
//  R5 the document route's answer wrapped in {document} or {revision} reads the same as a bare one; a failed revision list still opens the document; a failed related list still opens the story.
//  R6 held: a story is fetched once for two asks; a document's head is asked again each time (it changes); a document at a revision is held; a failed ask shows the held copy.
//  R7 accounts: a held story is dropped at a new account generation; an answer that began under an older generation is never held and is not shown.
//  R8 prefetch: the neighbours that are stories and not held are fetched once each, an ask for one in flight shares the request, the empty address and other pages are skipped, a failure is kept nowhere.
//  R9 at most 40 held, the least recently used goes first.
//  R10 reload forgets; restore posts the revision as latest and forgets, the head is already the latest; stop cancels the trajectory (a 409 or 404 is fine) and forgets; each says whether it worked.
//  R13 the page in front is handed to the native voice (it holds the page text the turn reads): its blocks with sentences, once per ask, only a ready document, nothing when there is no native module, never from another account.
//  R14 the log says "handed" only when native answered that it held the page; a refusal is logged.
//  R11 "Hear it": the story's clips come with it; a failing clip route gives none and does not fail the story.   R12 a document the desk is revising says so.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as R from './reader.ts';
import { HTTPError, SignedOutError, nextGeneration, resetSeam, type Reply } from './api.ts';

const reply = (body: unknown): Reply => ({ body, serverTime: 0, raw: '' });
const bundle = (title: string, text = 'The council voted.') => ({ publication: { title, kind: 'story' }, route: { bare: 'pub/x', permalink: 'pub/x/v1' }, artifact: { content: `# ${title}\n\n${text}` }, nav: { revision: 1, latest: 1 } });
type Call = { path: string; method: string; query?: Record<string, unknown>; body?: unknown; authed: boolean };
function fake(over: { pub?: (path: string, q: Record<string, unknown>) => unknown; authed?: (path: string, o: { method?: string; body?: unknown }) => unknown; clips?: (a: string) => unknown[] } = {}) {
  const calls: Call[] = [];
  const net: R.Net = {
    pub: async (path, q = {}) => { calls.push({ path, method: 'GET', query: q, authed: false }); const v = over.pub ? over.pub(path, q) : path.endsWith('/related') ? { links: [{ title: 'Next', route: '/pub/n', dek: 'd' }] } : bundle('Harbor barrier'); if (v instanceof Error) throw v; return reply(v); },
    authed: async (path, o = {}) => { calls.push({ path, method: o.method ?? 'GET', body: o.body, authed: true }); const v = over.authed ? over.authed(path, o) : defaultAuthed(path); if (v instanceof Error) throw v; return reply(v); },
    clips: async (a) => (over.clips ? over.clips(a) : []) as never,
  };
  return { net, calls, count: (p: string) => calls.filter((c) => c.path.includes(p)).length };
}
function defaultAuthed(path: string): unknown {
  if (path.endsWith('/revisions')) return { revisions: [{ revision_id: 'r2', version_number: 1, author_label: 'appagent', created_at: '2026-11-09T10:00:00Z' }, { revision_id: 'r1', version_number: 0, author_label: 'you', created_at: '2026-11-08T10:00:00Z' }] };
  if (path.startsWith('/api/texture/revisions/')) return { revision_id: path.endsWith('r1') ? 'r1' : 'r2', version_number: path.endsWith('r1') ? 0 : 1, content: '## Notes\n\nSome text.', created_at: '2026-11-09T10:00:00Z' };
  return { doc_id: 'd1', title: 'Plan', current_revision_id: 'r2', trajectory_id: 'traj-1' };
}
beforeEach(() => { resetSeam(); R.resetReaderForTest(); });

describe('a story', () => {
  test('R1 the public route and the related list; no key', async () => {
    const f = fake();
    const d = await R.loadReader('pub:pub/x', f.net);
    expect(d.status).toBe('ready'); expect(d.title).toBe('Harbor barrier'); expect(d.address).toBe('pub:pub/x');
    expect(d.blocks.map((b) => b.text)).toEqual(['The council voted.']); expect(d.permalink).toBe('pub/x/v1'); expect(d.related).toEqual([{ address: 'pub:pub/n', title: 'Next', dek: 'd' }]);
    expect(f.calls.every((c) => !c.authed)).toBe(true); expect(f.count('/publications/resolve')).toBe(1); expect(f.count('/publications/related')).toBe(1);
    expect(f.calls.find((c) => c.path.endsWith('/resolve'))?.query).toEqual({ route: 'pub/x' });
  });
  test('R2 failures have words', async () => {
    const bad = fake({ pub: (p) => (p.endsWith('/resolve') ? new Error('down') : {}) });
    const d = await R.loadReader('pub:pub/x', bad.net);
    expect(d.status).toBe('error'); expect(d.message).toMatch(/^Could not open this page\./);
    for (const [reason, want] of [['none', 'Sign in to open your documents.'], ['locked', 'Unlock your phone to continue.'], ['refused', 'Sign in again to open your documents.']] as const) {
      const f = fake({ authed: () => new SignedOutError('x', reason) });
      const t = await R.loadReader('texture:d1', f.net);
      expect([t.status, t.message]).toEqual(['error', want]);
    }
    const u = await R.loadReader('texture:d1', fake({ authed: () => new SignedOutError('x', 'unknown') }).net);
    expect(u.message).toBe('Could not check your sign-in just now. Try again in a moment.');
    const h = await R.loadReader('texture:d1', fake({ authed: () => new HTTPError(500, 'HTTP 500') }).net);
    expect(h.message).toBe('Could not open this page. HTTP 500');
  });
  test('R3 another address is loading and nothing is asked', async () => {
    const f = fake();
    for (const a of ['paper:front', '', 'pub:', 'texture:', 'texture:@r']) { const d = await R.loadReader(a, f.net); expect(d.status).toBe('loading'); expect(d.address).toBe(a); expect(d.blocks).toEqual([]); }
    expect(f.calls.length).toBe(0);
  });
  test('R5 a failed related list still opens the story', async () => {
    const f = fake({ pub: (p) => (p.endsWith('/related') ? new Error('404') : bundle('T')) });
    const d = await R.loadReader('pub:pub/x', f.net);
    expect(d.status).toBe('ready'); expect(d.related).toEqual([]);
  });
});

describe('a document', () => {
  test('R4 the head is editable and carries its text; versions and the trajectory', async () => {
    const f = fake();
    const d = await R.loadReader('texture:d1', f.net);
    expect(d.status).toBe('ready'); expect(d.title).toBe('Plan'); expect(d.editable).toBe(true); expect(d.docId).toBe('d1'); expect(d.source).toBe('## Notes\n\nSome text.');
    expect(d.versions.map((v) => v.address)).toEqual(['texture:d1', 'texture:d1@r1']); expect(d.trajectory).toBe('traj-1');
    expect(f.calls.every((c) => c.authed)).toBe(true);
    expect(f.calls.map((c) => c.path).sort()).toEqual(['/api/texture/documents/d1', '/api/texture/documents/d1/revisions', '/api/texture/revisions/r2']);
    const old = await R.loadReader('texture:d1@r1', f.net);
    expect(old.editable).toBe(false); expect(old.newer).toBe(true);
  });
  test('R5 wrapped answers read the same; a failed revision list opens the document', async () => {
    const f = fake({ authed: (p) => (p.endsWith('/revisions') ? new HTTPError(500, 'x') : p.startsWith('/api/texture/revisions/') ? { revision: { revision_id: 'r2', content: 'Body text.' } } : { document: { doc_id: 'd1', title: 'Wrapped', current_revision_id: 'r2' } }) });
    const d = await R.loadReader('texture:d1', f.net);
    expect([d.status, d.title, d.source, d.versions.length]).toEqual(['ready', 'Wrapped', 'Body text.', 0]);
  });
  test('R12 a document the desk is revising says so', async () => {
    const f = fake({ authed: (p) => (p.endsWith('/revisions') ? { revisions: [] } : p.startsWith('/api/texture/revisions/') ? { revision_id: 'r2', content: 'x' } : { doc_id: 'd1', title: 'P', current_revision_id: 'r2', agent_revision_pending: true, trajectory_id: 'tj' }) });
    const d = await R.loadReader('texture:d1', f.net);
    expect([d.revising, d.revisingNote]).toEqual([true, 'The desk is writing a new version.']);
  });
});

describe('held documents', () => {
  test('R6 a story once; a head again; a revision once; a failure shows the copy held', async () => {
    const f = fake();
    await R.readerDocShared('pub:pub/x', 'in', false, f.net); await R.readerDocShared('pub:pub/x', 'in', false, f.net);
    expect(f.count('/publications/resolve')).toBe(1);
    await R.readerDocShared('texture:d1', 'in', false, f.net); await R.readerDocShared('texture:d1', 'in', false, f.net);
    expect(f.count('/api/texture/documents/d1')).toBe(4);               // the document and its list, twice
    await R.readerDocShared('texture:d1@r1', 'in', false, f.net); await R.readerDocShared('texture:d1@r1', 'in', false, f.net);
    expect(f.count('/api/texture/revisions/r1')).toBe(1);
    const down = fake({ authed: () => new HTTPError(503, 'HTTP 503') });
    const held = await R.readerDocShared('texture:d1', 'in', false, down.net);
    expect([held.status, held.title]).toEqual(['ready', 'Plan']);
    const cold = await R.readerDocShared('texture:other', 'in', false, down.net);
    expect(cold.status).toBe('error');
  });
  test('R7 a new account never sees the old one\'s documents; a stale answer is not held', async () => {
    const f = fake();
    await R.readerDocShared('pub:pub/x', 'in', false, f.net);
    nextGeneration();
    await R.readerDocShared('pub:pub/x', 'in', false, f.net);
    expect(f.count('/publications/resolve')).toBe(2);
    const flip: R.Net = { ...fake().net, pub: async (p) => (p.endsWith('/resolve') ? (nextGeneration(), reply(bundle('Old account story'))) : reply({})) };
    const pending = R.readerDocShared('pub:pub/slow', 'in', false, flip);
    const d = await pending;
    expect(d.status).toBe('loading'); expect(JSON.stringify(d)).not.toContain('Old account story');
    const again = fake(); await R.readerDocShared('pub:pub/slow', 'in', false, again.net);
    expect(again.count('/publications/resolve')).toBe(1);
  });
  test('R7 a held private revision is not shown to the next account either', async () => {
    const f = fake();
    expect((await R.readerDocShared('texture:d1@r1', 'in', false, f.net)).status).toBe('ready');
    await R.readerDocShared('texture:d1@r1', 'in', false, f.net);
    expect(f.count('/api/texture/revisions/r1')).toBe(1);                      // held for the account that read it
    nextGeneration();
    const other = fake({ authed: (p) => (p.startsWith('/api/texture/revisions/') ? { revision_id: 'r1', content: 'The new account\u2019s own text.' } : p.endsWith('/revisions') ? { revisions: [] } : { doc_id: 'd1', title: 'Theirs', current_revision_id: 'r1' }) });
    const d = await R.readerDocShared('texture:d1@r1', 'in', false, other.net);
    expect(other.count('/api/texture/revisions/r1')).toBe(1);                  // asked again with the new account's key
    expect(JSON.stringify(d)).not.toContain('Some text.'); expect(d.title).toBe('Theirs');
  });
  test('R9 at most 40 are held, the least recently used goes first', async () => {
    const f = fake();
    for (let i = 0; i < 41; i++) await R.readerDocShared(`pub:pub/s${i}`, 'in', false, f.net);
    expect(R.heldCount()).toBe(40);
    const before = f.count('/publications/resolve');
    await R.readerDocShared('pub:pub/s1', 'in', false, f.net); expect(f.count('/publications/resolve')).toBe(before);
    await R.readerDocShared('pub:pub/s0', 'in', false, f.net); expect(f.count('/publications/resolve')).toBe(before + 1);
  });
  test('R8 prefetch: stories not held, once each, sharing an ask in flight', async () => {
    const f = fake();
    await R.readerDocShared('pub:pub/held', 'in', false, f.net);
    const asks = f.count('/publications/resolve');
    const ack = await R.readerPrefetch('pub:pub/held', 'pub:pub/n1', f.net);
    expect(ack.ok).toBe(true); expect(f.count('/publications/resolve')).toBe(asks + 1);
    await R.readerPrefetch('', 'texture:d1', f.net); await R.readerPrefetch('paper:front', 'pub:pub/n1', f.net);
    expect(f.count('/publications/resolve')).toBe(asks + 1); expect(f.count('/api/texture/documents/d1')).toBe(0);
    const g = fake({ pub: (p) => (p.endsWith('/resolve') ? new Error('down') : {}) });
    await R.readerPrefetch('pub:pub/z', '', g.net); await R.readerPrefetch('pub:pub/z', '', g.net);
    expect(g.count('/publications/resolve')).toBe(2);                           // a failure is kept nowhere
    const h = fake();
    await Promise.all([R.readerPrefetch('pub:pub/q', '', h.net), R.readerDocShared('pub:pub/q', 'in', false, h.net)]);
    expect(h.count('/publications/resolve')).toBe(1);
  });
});

describe('actions', () => {
  test('R10 reload, restore, stop', async () => {
    const f = fake();
    await R.readerDocShared('pub:pub/x', 'in', false, f.net);
    expect((await R.readerReload('pub:pub/x')).ok).toBe(true);
    await R.readerDocShared('pub:pub/x', 'in', false, f.net); expect(f.count('/publications/resolve')).toBe(2);
    await R.readerDocShared('texture:d1', 'in', false, f.net);
    expect((await R.readerRestore('texture:d1', 'texture:d1@r1', f.net)).ok).toBe(true);
    const post = f.calls.find((c) => c.method === 'POST')!;
    expect(post.path).toBe('/api/texture/documents/d1/restore'); expect(post.body).toEqual({ revision_id: 'r1', mode: 'restore_as_latest' });
    const n = f.calls.length;
    expect((await R.readerRestore('texture:d1', 'texture:d1', f.net)).ok).toBe(true); expect(f.calls.length).toBe(n);
    expect((await R.readerRestore('pub:pub/x', 'pub:pub/x/v1', f.net)).ok).toBe(false);
    const fail = fake({ authed: (p, o) => (o.method === 'POST' ? new HTTPError(500, 'x') : defaultAuthed(p)) });
    expect((await R.readerRestore('texture:d1', 'texture:d1@r1', fail.net)).ok).toBe(false);
    await R.readerDocShared('texture:d1', 'in', false, f.net);
    expect((await R.readerStop('texture:d1', f.net)).ok).toBe(true);
    expect(f.calls.filter((c) => c.method === 'POST').pop()!.path).toBe('/api/texture/lifecycle/traj-1/cancel');
    const done = fake({ authed: (p, o) => (o.method === 'POST' ? new HTTPError(409, 'HTTP 409') : defaultAuthed(p)) });
    await R.readerDocShared('texture:d1', 'in', false, done.net);
    expect((await R.readerStop('texture:d1', done.net)).ok).toBe(true);
  });
});

describe('the voice holds the page in front', () => {
  type Sent = { op: string; address: string; title: string; dek: string; kind: string; blocks: { id: string; kind: string; text: string; sentences: string[] }[] };
  test('R13 a ready document is handed over with its blocks and sentences; others are not', async () => {
    const sent: Sent[] = [];
    R.setHandOverForTest(async (r) => { sent.push(r as Sent); return { ok: true }; });
    const f = fake({ pub: (p) => (p.endsWith('/resolve') ? { publication: { title: 'Harbor', kind: 'story', dek: 'A dek.' }, artifact: { content: 'First sentence here. Second one follows.\n\n```\ncode\n```' }, nav: {} } : {}) });
    await R.readerDocShared('pub:pub/x', 'in', false, f.net);
    expect(sent.length).toBe(1);
    expect(sent[0]).toMatchObject({ op: 'pageText', address: 'pub:pub/x', title: 'Harbor', dek: 'A dek.', kind: 'story' });
    expect(sent[0].blocks[0]).toMatchObject({ kind: 'p', text: 'First sentence here. Second one follows.', sentences: ['First sentence here.', 'Second one follows.'] });
    expect(sent[0].blocks.map((b) => b.kind)).toContain('code');
    const tricky = fake({ pub: (p) => (p.endsWith('/resolve') ? { publication: { title: 'T', kind: 'story' }, artifact: { content: 'Dr. Smith paid $3.5 million. He said \u201CStop.\u201D Then left.' }, nav: {} } : {}) });
    await R.readerDocShared('pub:pub/tricky', 'in', false, tricky.net);
    expect(sent[sent.length - 1].blocks[0].sentences).toEqual(['Dr. Smith paid $3.5 million.', 'He said \u201CStop.\u201D', 'Then left.']);    // the one splitter's sentences are what native matches against
    sent.pop();
    await R.readerDocShared('pub:pub/x', 'in', false, f.net);               // held: handed again (the voice may have dropped it)
    expect(sent.length).toBe(2);
    await R.readerDocShared('paper:front', 'in', false, f.net); await R.readerDocShared('pub:pub/gone', 'in', false, fake({ pub: () => new Error('down') }).net);
    expect(sent.length).toBe(2);
  });
  test('R13 no native module is fine; a hand-over that fails does not fail the page; a stale account is not handed over', async () => {
    R.setHandOverForTest(null);
    expect((await R.readerDocShared('pub:pub/x', 'in', false, fake().net)).status).toBe('ready');
    R.setHandOverForTest(async () => { throw new Error('native down'); });
    expect((await R.readerDocShared('pub:pub/y', 'in', false, fake().net)).status).toBe('ready');
    const sent: unknown[] = []; R.setHandOverForTest(async (r) => { sent.push(r); return {}; });
    const flip: R.Net = { ...fake().net, pub: async (p) => (p.endsWith('/resolve') ? (nextGeneration(), reply(bundle('Old'))) : reply({})) };
    await R.readerDocShared('pub:pub/z', 'in', false, flip);
    expect(sent.length).toBe(0);
  });
  test('R14 "handed" is logged only when native held it, a refusal is logged', async () => {
    const lines: string[] = []; const real = console.log; console.log = (...a: unknown[]) => { lines.push(a.join(' ')); };
    try {
      let answer: unknown = { ok: true };
      R.setHandOverForTest(async () => answer);
      await R.readerDocShared('pub:pub/x', 'in', false, fake().net); await new Promise((r) => setTimeout(r, 5));
      expect(lines.some((l) => l.startsWith('reader: handed to the voice'))).toBe(true);
      lines.length = 0; answer = { ok: false };
      await R.readerDocShared('pub:pub/x', 'in', false, fake().net); await new Promise((r) => setTimeout(r, 5));
      expect(lines.some((l) => l.startsWith('reader: handed to the voice'))).toBe(false);
      expect(lines.some((l) => l.startsWith('reader: the voice did not take the page'))).toBe(true);
    } finally { console.log = real; }
  });
});
