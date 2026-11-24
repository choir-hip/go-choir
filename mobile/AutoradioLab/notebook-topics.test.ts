// The listener's topics become the first lines of "What you asked for" in the notebook (it was Swift: NotebookStore.writeTopics; a Texture canonical write). Failure list, written before the code:
//  T1 no notebook yet: one is made (POST /api/texture/documents, title "My notebook") and the seed with the topics is saved into it.   T2 a notebook: the missing topics are appended to its section, nothing else changes.
//  T3 the marker: the same topics again ask the server nothing.                T4 the server cannot say whether there is a notebook: nothing is created on a guess, no marker.
//  T5 no topics: nothing.                                                      T6 the account moved on while it ran: no marker, nothing of it is kept.
//  T7 a refused save: false, no marker (the page asks again).                  T8 two asks at once make one document.
import { beforeEach, describe, expect, test } from 'bun:test';
import { writeTopics, MARKER } from './notebook-topics.ts';
import { Editor, type Net } from './texture-edit.ts';
import { HTTPError, resetSeam, nextGeneration } from './api.ts';

const files = new Map<string, Uint8Array>();
const storage = { fs: {
  readFile: async (p: string) => { const b = files.get(p); if (!b) throw Object.assign(new Error('no'), { code: 'ENOENT' }); return b; },
  atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
  rm: async (p: string) => { files.delete(p); },
} } as never;
type Call = { method: string; path: string; body?: Record<string, unknown> };
let calls: Call[]; let edition: unknown; let docs: Map<string, { rev: string; content: string }>; let revs: Map<string, string>; let created = 0; let refuse = false;
const reply = (body: unknown) => ({ body, serverTime: null, raw: '' });
const net: Net = { json: async (path, opts) => {
  const method = opts?.method ?? 'GET';
  calls.push({ method, path, body: opts?.body as Record<string, unknown> | undefined });
  if (path === '/api/texture/edition') { if (edition === null) throw new Error('down'); return reply(edition); }
  if (method === 'POST' && path === '/api/texture/documents') { created += 1; const id = `nb-${created}`; docs.set(id, { rev: '', content: '' }); return reply({ doc_id: id }); }
  let m = path.match(/^\/api\/texture\/documents\/([^/]+)$/);
  if (m) { const d = docs.get(m[1])!; return reply({ document: { doc_id: m[1], current_revision_id: d.rev } }); }
  m = path.match(/^\/api\/texture\/revisions\/([^/]+)$/);
  if (m) return reply({ revision: { revision_id: m[1], content: revs.get(m[1]) ?? '', version_number: 0 } });
  m = path.match(/^\/api\/texture\/documents\/([^/]+)\/revisions$/);
  if (m && method === 'POST') {
    if (refuse) throw new HTTPError(403, '403');
    const id = `rv-${calls.length}`; revs.set(id, String(opts!.body && (opts!.body as Record<string, unknown>).content)); docs.get(m[1])!.rev = id; return reply({ revision: { revision_id: id, version_number: 0 } });
  }
  throw new Error('unexpected ' + method + ' ' + path);
} };
const live = (topics: string[]) => ({ net, editor: new Editor({ net, storage }), topics: async () => topics });
const posts = (): Call[] => calls.filter((c) => c.method === 'POST' && c.path.endsWith('/revisions'));
beforeEach(() => { resetSeam(); files.clear(); calls = []; edition = { found: false }; docs = new Map(); revs = new Map(); created = 0; refuse = false; });

describe('the topics write', () => {
  test('T1 no notebook: one is made and the seed saved into it', async () => {
    expect(await writeTopics(storage, live(['Space', 'Chess']))).toBe(true);
    expect(calls.find((c) => c.method === 'POST' && c.path === '/api/texture/documents')!.body).toEqual({ title: 'My notebook' });
    expect(posts().length).toBe(1);
    expect(posts()[0].body!.content).toBe('# My notebook\n\n## What you asked for\n- Space\n- Chess\n');
    expect(files.has(MARKER)).toBe(true);
  });
  test('T2 a notebook: the missing topics are appended, the rest is untouched', async () => {
    edition = { found: true, doc_id: 'nb-0' };
    const text = '# My notebook\n\n## What you asked for\n- Space\n\n## What Autoradio has learned\n- A thing. [L2 · heard 1 of 2]\n';
    docs.set('nb-0', { rev: 'r0', content: text }); revs.set('r0', text);
    expect(await writeTopics(storage, live(['Space', 'Chess']))).toBe(true);
    expect(created).toBe(0);
    expect(posts()[0].body!.content).toBe('# My notebook\n\n## What you asked for\n- Space\n- Chess\n\n## What Autoradio has learned\n- A thing. [L2 · heard 1 of 2]\n');
    expect(posts()[0].body!.parent_revision_id).toBe('r0');
  });
  test('T2b every topic already there: nothing is posted, the marker is written', async () => {
    edition = { found: true, doc_id: 'nb-0' };
    const text = '# My notebook\n\n## What you asked for\n- Space\n'; docs.set('nb-0', { rev: 'r0', content: text }); revs.set('r0', text);
    expect(await writeTopics(storage, live(['space']))).toBe(true);
    expect(posts().length).toBe(0); expect(files.has(MARKER)).toBe(true);
  });
  test('T3 the same topics again ask nothing', async () => {
    await writeTopics(storage, live(['Space']));
    calls = [];
    expect(await writeTopics(storage, live(['space']))).toBe(true);
    expect(calls).toEqual([]);
  });
  test('T4 the server cannot say: nothing is created on a guess, no marker', async () => {
    edition = null;
    expect(await writeTopics(storage, live(['Space']))).toBe(false);
    expect(created).toBe(0); expect(files.has(MARKER)).toBe(false);
  });
  test('T5 no topics: nothing', async () => {
    expect(await writeTopics(storage, live([]))).toBe(true);
    expect(calls).toEqual([]);
  });
  test('T6 the account moved on while it ran: no marker', async () => {
    const d = live(['Space']);
    const lookup = net.json;
    (net as { json: Net['json'] }).json = async (p, o) => { const r = await lookup(p, o); if (p === '/api/texture/edition') nextGeneration(); return r; };
    expect(await writeTopics(storage, d)).toBe(false);
    (net as { json: Net['json'] }).json = lookup;
    expect(files.has(MARKER)).toBe(false); expect(created).toBe(0);
  });
  test('T7 a refused save is false and leaves no marker', async () => {
    refuse = true;
    expect(await writeTopics(storage, live(['Space']))).toBe(false);
    expect(files.has(MARKER)).toBe(false);
  });
  test('T8 two asks at once make one document', async () => {
    const d = live(['Space']);
    const [a, b] = await Promise.all([writeTopics(storage, d), writeTopics(storage, d)]);
    expect(a && b).toBe(true); expect(created).toBe(1); expect(posts().length).toBe(1);
  });
});
