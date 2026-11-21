// The Files page's data module under bun test: the source called with a fake computer (the signed-in seam stubbed) and a fake storage.
// The failure list, written before the code (each line has a test below; the Swift lists this replaced were FilesTests 1-12, FileKindsOpenTests, FilesOpenTests, FilesInReaderTests, FilesInTextureTests):
//  F1 a path escapes the root ("..", an encoded "..", a NUL): refused before any request.   F2 a bad name (slash, dot, empty, padded, 256 bytes) is refused, nothing sent.
//  F3 names with spaces, "?", "#", "%" reach the URL percent-encoded; the cache name is plain.   F4 a changed file never shares a cache entry with the old one.
//  F5 sorting is total: folders first, natural numbers, case ignored, ties by path, desc flips the key only.   F6 CSV: quotes, doubled quotes, CRLF, ragged rows.
//  F7 the root listing of the real shape: sorted rows, labels, dates from the server's clock.   F8 a missing folder, a refused key, an unreachable computer: an error status with one plain sentence.
//  F9 search walks folders, bounded.   F10 opening: image, table and audio are brought and described; other kinds are facts only; a file too large is refused before any download; a missing file says so.
//  F11 a PDF or an e-book is read by the listener's computer (D4) and opens as its texture: page; text, Markdown and code are sent as the web sends them; a failure says why and is asked again next time.
//  F12 the changes use the right verbs and answer the refreshed listing; a folder is not renamed to nothing.   F13 an upload sends the picked file's bytes to the computer.
//  F14 recents: newest first, capped at 30, kept on the phone, dropped when the file is gone.
//  F18 a big file (60 MB) is written to disk by the host with no bytes in JavaScript: the signed-in reply is never read, the file system is never asked to read it back.
//  F19 the caps are the Swift app's (800 MB video, 300 MB audio, 60 MB images); one byte over is refused before any download.
//  F20 a failed download leaves no file that a later open takes for whole; a file of the wrong size is fetched again.
//  F21 the signed-in download (api.ts authedToFile): the body goes to the named app file, a refusal or a sign-out leaves none, a host that wrote nothing is an error.
//  F22 audio from Files reaches the run sheet: the Play and Add to queue chips send queueAdd (kind file) with the app file the page brought, which the interface declares.
//  F16 no word "Loading" in what the source answers.   F17 the grants cover the paths the module touches.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as F from './files.ts';
import * as M from './files-model.ts';
import { SignedOutError, type Reply } from './api.ts';
import rootListing from '../fixtures/files/root-listing.json';
import textureOpen from '../fixtures/files/texture-open.json';
import { readFileSync } from 'node:fs';

const NOW = Date.parse('2026-11-18T12:00:00Z');
type Call = { method: string; path: string; query?: Record<string, unknown>; body?: unknown; headers?: Record<string, string> };
let calls: Call[] = [];
let tree: Record<string, unknown[]>;           // folder path -> the server's array
let blobs: Record<string, Uint8Array>;
let fail: ((c: Call) => unknown) | null;
const files = new Map<string, Uint8Array>();
const sizes = new Map<string, number>();        // a file the host wrote that never entered JavaScript: only its size is known
let reads: string[] = [];
const storage = {
  fs: {
    readFile: async (p: string) => { if (p.startsWith(F.CACHE_DIR)) reads.push(p); const b = files.get(p); if (!b) throw new Error('no ' + p); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    writeFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    stat: async (p: string) => { if (sizes.has(p)) return { size: sizes.get(p)! }; const b = files.get(p); if (!b) throw new Error('no'); return { size: b.length }; },
    mkdir: async () => {},
    rm: async (p: string) => { files.delete(p); sizes.delete(p); },
  },
} as never;
const apiPath = (p: string) => decodeURIComponent(p.replace(/^\/api\/files/, '')) || '/';
const net: F.Net = {
  json: async (path: string, o: any = {}): Promise<Reply> => {
    const c: Call = { method: o.method ?? 'GET', path, query: o.query, body: o.body, headers: o.headers };
    calls.push(c);
    const f = fail?.(c); if (f) throw f;
    if (path === '/api/texture/files/open') return { body: textureOpen, serverTime: NOW, raw: '' };
    const dir = apiPath(path);
    if (c.method === 'GET') { if (!tree[dir]) throw Object.assign(new Error('nf'), { status: 404 }); return { body: tree[dir], serverTime: NOW, raw: '' }; }
    return { body: { ok: true }, serverTime: NOW, raw: '' };
  },
  bytes: async (path: string, o: any = {}) => {
    calls.push({ method: 'GET', path, query: o.query });
    const b = blobs[apiPath(path)]; if (!b) throw Object.assign(new Error('nf'), { status: 404 });
    return { bytes: b, contentType: '' };
  },
};
let toFileFail: unknown = null;
(net as { toFile: F.Net['toFile'] }).toFile = async (path, dest, o = {}) => {
  calls.push({ method: 'GET', path, query: o.query, headers: { dest } });
  if (toFileFail) throw toFileFail;
  const dir = apiPath(path);
  if (bigSizes[dir] !== undefined) { sizes.set(dest, bigSizes[dir]); return { bytes: bigSizes[dir], contentType: '' }; }
  const b = blobs[dir]; if (!b) throw Object.assign(new Error('nf'), { status: 404 });
  files.set(dest, b); return { bytes: b.length, contentType: '' };
};
let bigSizes: Record<string, number> = {};
const enc = (s: string) => new TextEncoder().encode(s);
beforeEach(() => {
  F.forget(); calls = []; files.clear(); sizes.clear(); reads = []; bigSizes = {}; toFileFail = null; fail = null;
  tree = { '/': rootListing, '/Documents': [{ name: 'Council notes.txt', type: 'file', size: 30, modified: '2026-11-18T10:00:00Z' }, { name: 'Sub', type: 'directory', size: 0, modified: '2026-11-01T00:00:00Z' }], '/Documents/Sub': [{ name: 'deep lamp.png', type: 'file', size: 9, modified: '2026-11-01T00:00:00Z' }] };
  blobs = { '/readings.csv': enc('day,mm\nMon,"1,5"\r\nTue,"say ""hi"""\nWed'), '/Notes 2.txt': enc('Two lamps.\n'), '/Harbor.md': enc('# Harbor\n\nA note.\n') };
});
const list = (o: Partial<F.ListArgs> = {}) => F.fileList({ path: '/', mode: 'browse', sort: 'name', desc: false, query: '', ...o }, storage, net);
const open = (address: string) => F.fileOpen({ address }, storage, net);

describe('F1 to F6 the pure rules', () => {
  test('F1 a path never escapes the root', () => {
    expect(M.normalize('/a/../..')).toBeNull(); expect(M.normalize('../x')).toBeNull(); expect(M.normalize('/a\0b')).toBeNull();
    expect(M.normalize('/a/./b//c/..')).toBe('/a/b'); expect(M.normalize('')).toBe('/');
    expect(M.fromAddress('file:/a/../../b')).toBeNull();
  });
  test('F2 names', () => {
    for (const bad of ['', '.', '..', 'a/b', ' pad', 'pad ', 'x'.repeat(256), 'a\0']) expect(M.validName(bad)).toBe(false);
    expect(M.validName('Council minutes.pdf')).toBe(true); expect(M.validName('x'.repeat(255))).toBe(true);
  });
  test('F3 encoding and cache names', () => {
    expect(M.encodePath('/a b/c?d#e%f.txt')).toBe('/a%20b/c%3Fd%23e%25f.txt');
    expect(M.cacheName('Harbor test card.PNG')).toBe('Harbor-test-card.png'); expect(M.cacheName('???.pdf')).toBe('file.pdf'); expect(M.cacheName('x'.repeat(100) + '.md').length).toBe(63);
  });
  test('addresses round trip', () => {
    expect(M.addressOf('/', true)).toBe('file:/'); expect(M.addressOf('/a', true)).toBe('file:/a/'); expect(M.addressOf('/a/b.pdf', false)).toBe('file:/a/b.pdf');
    expect(M.fromAddress('file:/a/')).toEqual({ path: '/a', isDirectory: true }); expect(M.fromAddress('file:/a/b.pdf')).toEqual({ path: '/a/b.pdf', isDirectory: false }); expect(M.fromAddress('file:')).toEqual({ path: '/', isDirectory: true });
    expect(M.fromAddress('pub:x')).toBeNull();
    expect(M.crumbsOf('/a/b')).toEqual([{ name: 'Files', path: '/' }, { name: 'a', path: '/a' }, { name: 'b', path: '/a/b' }]);
  });
  test('kinds, labels, sizes and dates', () => {
    expect(M.kindOf('x.PDF', false)).toBe('pdf'); expect(M.kindOf('x.epub', false)).toBe('epub'); expect(M.kindOf('x.svg', false)).toBe('code'); expect(M.kindOf('x', false)).toBe('other'); expect(M.kindOf('x.png', true)).toBe('folder');
    expect(M.sizeLabel(0)).toBe('0 B'); expect(M.sizeLabel(812)).toBe('812 B'); expect(M.sizeLabel(1200)).toBe('1.2 kB'); expect(M.sizeLabel(34000)).toBe('34 kB'); expect(M.sizeLabel(5_600_000)).toBe('5.6 MB');
    const d = (s: string) => M.dateLabel(M.parseDate(s), NOW);
    expect(d('2026-11-18T01:00:00Z')).toBe('Today'); expect(d('2026-11-17T23:00:00Z')).toBe('Yesterday'); expect(d('2026-11-03T00:00:00Z')).toBe('3 Nov'); expect(d('2025-03-02T00:00:00Z')).toBe('2 Mar 2025');
    expect(M.parseDate('2026-11-10T09:00:00.123456789Z')).toBe(Date.parse('2026-11-10T09:00:00.123Z')); expect(M.parseDate('nope')).toBeNull();
  });
  test('F4 the cache key changes with the file', () => {
    expect(M.hash('/a|1|2')).not.toBe(M.hash('/a|1|3')); expect(M.hash('/a|1|2')).toBe(M.hash('/a|1|2'));
  });
  test('F5 sorting is total, folders first, numbers natural', () => {
    const e = (name: string, isDirectory = false, size = 0, modified: number | null = null): M.Entry => ({ name, path: '/' + name, isDirectory, size, modified });
    const names = (l: M.Entry[]) => l.map((x) => x.name);
    const mixed = [e('b.txt'), e('A'), e('notes 10'), e('Notes 2'), e('zdir', true), e('Adir', true)];
    expect(names(M.sortEntries(mixed, 'name', false))).toEqual(['Adir', 'zdir', 'A', 'b.txt', 'Notes 2', 'notes 10']);
    expect(names(M.sortEntries(mixed, 'name', true))).toEqual(['zdir', 'Adir', 'notes 10', 'Notes 2', 'b.txt', 'A']);
    const twins = [e('same'), { ...e('Same'), path: '/a/Same' }];
    expect(names(M.sortEntries(twins, 'name', false))).toEqual(M.sortEntries(twins.slice().reverse(), 'name', false).map((x) => x.name));
    expect(names(M.sortEntries([e('a', false, 5, 2), e('b', false, 1, null), e('c', false, 9, 1)], 'modified', false))).toEqual(['b', 'c', 'a']);
  });
  test('F6 CSV', () => {
    expect(M.parseCsv('a,b\r\n"x,1","say ""hi"""\n\nlast')).toEqual([['a', 'b'], ['x,1', 'say "hi"'], ['last']]);
    expect(M.parseCsv('a\tb\n1\t2', '\t')).toEqual([['a', 'b'], ['1', '2']]); expect(M.parseCsv('a\nb\nc', ',', 2).length).toBe(2); expect(M.parseCsv('a"b,c')).toEqual([['a"b', 'c']]);
    const e = Array.from(enc('é')), smile = Array.from(enc('😀')), ab = Array.from(enc('ab')), trim = (b: number[]) => Array.from(M.withoutPartialUTF8(new Uint8Array(b)));
    expect(trim([...ab, e[0]])).toEqual(ab); expect(trim([...ab, ...e])).toEqual([...ab, ...e]);
    for (let k = 1; k < 4; k++) expect(trim([...ab, ...smile.slice(0, k)])).toEqual(ab);
    expect(trim([...ab, ...smile])).toEqual([...ab, ...smile]); expect(trim([])).toEqual([]); expect(trim([0x80, 0x80])).toEqual([0x80, 0x80]);
    const cut = new TextEncoder().encode('é').slice(0, 1); expect(M.withoutPartialUTF8(cut).length).toBe(0); expect(M.withoutPartialUTF8(enc('aé')).length).toBe(3);
  });
});

describe('F7 to F9 the listing', () => {
  test('F7 the root listing of the real shape', async () => {
    const l = await list();
    expect(calls.map((c) => c.path)).toEqual(['/api/files']);
    expect(l.status).toBe('ready'); expect(l.title).toBe('Files'); expect(l.count).toBe(9);
    expect(l.rows.map((r) => r.name)).toEqual(['Documents', 'Pictures', 'Harbor.md', 'Lantern report.pdf', 'mystery.dat', 'notes 1.txt', 'Notes 2.txt', 'Notes 10.txt', 'readings.csv']);
    const pdf = l.rows[3];
    expect(pdf).toMatchObject({ address: 'file:/Lantern report.pdf', kind: 'pdf', kindLabel: 'PDF', sizeLabel: '1.2 MB', modifiedLabel: 'Yesterday', detail: '1.2 MB · Yesterday', isDir: false });
    expect(l.rows[0]).toMatchObject({ address: 'file:/Documents/', isDir: true, detail: '10 Nov', sizeLabel: '' });
    expect(l.rows[5].modifiedLabel).toBe('2 Mar 2025');
    expect((await list({ sort: 'size', desc: true })).rows[2].name).toBe('Lantern report.pdf');
  });
  test('F8 a missing folder, a refused key and an unreachable computer each end in a stated error', async () => {
    const miss = await list({ path: '/Nope' }); expect(miss.status).toBe('error'); expect(miss.message).toBe('That file is not there any more.'); expect(miss.rows).toEqual([]);
    fail = () => Object.assign(new Error('x'), { status: 401 }); expect((await list()).message).toBe('Sign in again to see your files.');
    fail = () => new SignedOutError('no session'); expect((await list()).message).toBe('Sign in to see your files.');
    fail = () => Object.assign(new Error('x'), { status: 403 }); expect((await list()).message).toBe('This device is not allowed to read files.');
    fail = () => new Error('offline'); expect((await list()).message).toBe('Could not reach your computer. Check the connection and try again.');
    tree['/Documents'] = []; fail = null;
    const empty = await list({ path: '/Documents' }); expect(empty.status).toBe('empty'); expect(empty.message).toBe('This folder is empty.');
    tree['/Documents'] = [{ name: '../evil', type: 'file', size: 1, modified: '2026-11-18T10:00:00Z' }]; expect((await list({ path: '/Documents' })).count).toBe(0);
  });
  test('F9 search walks folders, ignores case and accents, and is bounded', async () => {
    tree['/Documents'].push({ name: 'Déjà lamp.txt', type: 'file', size: 1, modified: '2026-11-18T10:00:00Z' });
    const l = await list({ query: 'LAMP' });
    expect(l.mode).toBe('search'); expect(l.rows.map((r) => r.name).sort()).toEqual(['Déjà lamp.txt', 'deep lamp.png']); expect(l.rows.find((r) => r.name === 'deep lamp.png')!.place).toBe('Sub');
    expect((await list({ query: 'zzz' })).message).toBe('Nothing matches.');
    calls = []; tree = { '/': Array.from({ length: 5 }, (_, i) => ({ name: 'd' + i, type: 'directory', size: 0, modified: '2026-11-18T10:00:00Z' })) };
    for (let i = 0; i < 5; i++) tree['/d' + i] = Array.from({ length: 30 }, (_, j) => ({ name: 'e' + j, type: 'directory', size: 0, modified: '2026-11-18T10:00:00Z' }));
    for (let i = 0; i < 5; i++) for (let j = 0; j < 30; j++) tree[`/d${i}/e${j}`] = [];
    await list({ query: 'nothing-matches' }); expect(calls.length).toBeLessThanOrEqual(80);
  });
});

describe('F10 opening what the phone shows', () => {
  test('a table is read, truncated rows kept, 12 columns at most', async () => {
    const i = await open('file:/readings.csv');
    expect(i).toMatchObject({ status: 'ready', kind: 'csv', kindLabel: 'Table', mime: 'text/csv', name: 'readings.csv' });
    expect(i.rows.map((r) => r.cells)).toEqual([['day', 'mm'], ['Mon', '1,5'], ['Tue', 'say "hi"'], ['Wed']]);
    expect(i.src).toBe(`${F.CACHE_DIR}/nokey/${M.hash(`/readings.csv|80|${Date.parse('2026-11-14T22:00:00Z')}`)}/readings.csv`);
    expect(files.get(i.src)!.length).toBe(blobs['/readings.csv'].length);
  });
  test('an image is brought and measured; a second open fetches nothing', async () => {
    const png = fromHex('89504e470d0a1a0a0000000d49484452000002800000019008020000');
    tree['/'] = [{ name: 'card.png', type: 'file', size: png.length, modified: '2026-11-18T08:00:00Z' }]; blobs['/card.png'] = png;
    const i = await open('file:/card.png'); expect([i.width, i.height]).toEqual([640, 400]); expect(i.status).toBe('ready'); expect(i.local).toBe(i.src);
    const fetched = calls.filter((c) => c.query).length; await open('file:/card.png'); expect(calls.filter((c) => c.query).length).toBe(fetched);
    expect(fetched).toBe(1);
  });
  test('other kinds are facts only, a folder is a folder, a missing file says so', async () => {
    const o = await open('file:/mystery.dat'); expect(o).toMatchObject({ status: 'ready', kind: 'other', src: '' }); expect(calls.every((c) => !c.query)).toBe(true);
    expect((await open('file:/Documents/')).address).toBe('file:/Documents/'); expect((await open('file:/Documents/')).status).toBe('empty');
    const gone = await open('file:/ghost.png'); expect(gone).toMatchObject({ status: 'notfound', message: 'That file is not there any more.' });
    expect((await open('pub:x')).status).toBe('empty');
  });
  test('a file over its cap is refused before any download', async () => {
    tree['/'] = [{ name: 'big.mp4', type: 'file', size: 900_000_000, modified: '2026-11-18T08:00:00Z' }, { name: 'huge.csv', type: 'file', size: 6_000_000, modified: '2026-11-18T08:00:00Z' }];
    const v = await open('file:/big.mp4'); expect(v.status).toBe('toolarge'); expect(v.message).toBe('It is 900 MB, too large to open on this phone.');
    expect((await open('file:/huge.csv')).status).toBe('toolarge'); expect(calls.filter((c) => c.query).length).toBe(0);
  });
});

describe('F18 to F20 large files go to disk and stay out of JavaScript', () => {
  const big = (name: string, size: number) => { tree['/'] = [{ name, type: 'file', size, modified: '2026-11-18T08:00:00Z' }]; bigSizes['/' + name] = size; };
  test('F18 a 60 MB video is written by the host: no reply read, no file read back, the cache path named', async () => {
    big('talk.mp4', 60_000_000);
    const i = await open('file:/talk.mp4');
    expect(i).toMatchObject({ status: 'ready', kind: 'video' }); expect(i.local).toBe(i.src); expect(i.local.startsWith(F.CACHE_DIR + '/')).toBe(true);
    expect(sizes.get(i.local)).toBe(60_000_000);
    expect(calls.filter((c) => c.query).length).toBe(1); expect(calls.find((c) => c.query)!.headers!.dest).toBe(i.local);
    expect(reads).toEqual([]);                                       // 60 MB never enters the engine's heap (or its 100 ms turn)
    const again = await open('file:/talk.mp4'); expect(again.local).toBe(i.local); expect(calls.filter((c) => c.query).length).toBe(1);     // cached: not fetched twice
  });
  test('F18 audio and a large picture the same; a small picture is still measured', async () => {
    big('long.mp3', 250_000_000); expect(await open('file:/long.mp3')).toMatchObject({ status: 'ready', kind: 'audio' }); expect(reads).toEqual([]);
    big('scan.png', 50_000_000); const p = await open('file:/scan.png'); expect(p).toMatchObject({ status: 'ready', kind: 'image', width: 0, height: 0 }); expect(reads).toEqual([]);
  });
  test('F19 the Swift caps: 800 MB video, 300 MB audio, 60 MB images; a byte over is refused before any download', async () => {
    expect([M.maxBytes('video'), M.maxBytes('audio'), M.maxBytes('image')]).toEqual([800_000_000, 300_000_000, 60_000_000]);
    for (const [name, size] of [['a.mp4', 800_000_001], ['a.mp3', 300_000_001], ['a.png', 60_000_001]] as const) {
      big(name, size); const v = await open('file:/' + name); expect(v.status).toBe('toolarge');
    }
    expect(calls.filter((c) => c.query).length).toBe(0);
    big('ok.mp4', 800_000_000); expect((await open('file:/ok.mp4')).status).toBe('ready');
  });
  test('F20 a failed download leaves nothing that a later open takes for whole', async () => {
    big('talk.mp4', 60_000_000);
    toFileFail = new Error('network'); const bad = await open('file:/talk.mp4'); expect(bad.status).toBe('error'); expect(sizes.size).toBe(0); expect(files.size).toBe(0);
    toFileFail = null;
    const dest = `${F.CACHE_DIR}/nokey/${M.hash(`/talk.mp4|60000000|${Date.parse('2026-11-18T08:00:00Z')}`)}/talk.mp4`;
    sizes.set(dest, 1_000_000);                                      // a stray short file at the place (a host that died mid-way)
    const ok = await open('file:/talk.mp4'); expect(ok.status).toBe('ready'); expect(sizes.get(dest)).toBe(60_000_000); expect(calls.filter((c) => c.query).length).toBe(2);
  });
  test('F20 a download that turns out larger than the cap is removed, not kept', async () => {
    big('talk.mp3', 100); bigSizes['/talk.mp3'] = 300_000_001;
    const v = await open('file:/talk.mp3'); expect(v.status).toBe('toolarge'); expect(sizes.size).toBe(0);
  });
});

describe('F22 audio from Files reaches the run sheet', () => {
  test('the chips send queueAdd (kind file) with the page\'s app file, and the interface declares it', () => {
    const v = readFileSync(new URL('./viewers.contract', import.meta.url), 'utf8');
    expect(v).toContain('queueFilePlay("now", fvInfo.address, fvInfo.local, fvInfo.name)');
    expect(v).toContain('queueFilePlay("add", fvInfo.address, fvInfo.local, fvInfo.name)');
    const q = JSON.parse(readFileSync(new URL('../exact-package/interface/queue.json', import.meta.url), 'utf8'));
    expect(q.names.find((n: { send?: string }) => n.send === 'queueAdd')?.options).toEqual({ mode: 'string', kind: 'string', ref: 'string', title: 'string', local: 'string' });
    const m = readFileSync(new URL('./members-queue.contract', import.meta.url), 'utf8');
    expect(m).toContain('queueAdd(mode=mode, kind="file", ref=address, title=title, local=local)');
  });
});

describe('F11 a file that is another page\'s document', () => {
  // Owner, 2026-11-18: "all files (PDF) must load". Every kind the Swift app opened still opens; a PDF is read by the listener's computer.
  test('"all files (PDF) must load": a PDF is opened by the computer and becomes its texture: page', async () => {
    const i = await open('file:/Lantern report.pdf');
    const post = calls.find((c) => c.path === '/api/texture/files/open')!;
    expect(post.method).toBe('POST'); expect(post.body).toEqual({ source_path: 'Lantern report.pdf', title: 'Lantern report.pdf', initial_content: '' });
    expect(calls.some((c) => c.query)).toBe(false);                 // the phone downloads nothing: the computer reads the PDF
    expect(i).toMatchObject({ status: 'ready', kind: 'pdf', target: 'texture:doc-8f3a1c' });
    const r = await F.fileResolve({ address: 'file:/Lantern report.pdf', title: 'x' }, storage, net);
    expect(r).toEqual({ address: 'file:/Lantern report.pdf', target: 'texture:doc-8f3a1c', title: 'Lantern report.pdf', status: 'ready', message: '' });
    expect(calls.filter((c) => c.path === '/api/texture/files/open').length).toBe(1);   // one ask per file version
  });
  test('an e-book in a folder is asked with its path, no leading slash', async () => {
    tree['/Documents'].push({ name: 'The Lantern Keeper.epub', type: 'file', size: 5000, modified: '2026-11-01T00:00:00Z' });
    const r = await F.fileResolve({ address: 'file:/Documents/The Lantern Keeper.epub', title: '' }, storage, net);
    expect(r.target).toBe('texture:doc-8f3a1c'); expect(calls.find((c) => c.path === '/api/texture/files/open')!.body).toMatchObject({ source_path: 'Documents/The Lantern Keeper.epub', initial_content: '' });
  });
  test('Markdown is sent as the web sends it: its text, the path without a slash', async () => {
    const r = await F.fileResolve({ address: 'file:/Harbor.md', title: 'Harbor.md' }, storage, net);
    expect(r.target).toBe('texture:doc-8f3a1c');
    expect(calls.find((c) => c.path === '/api/texture/files/open')!.body).toEqual({ source_path: 'Harbor.md', title: 'Harbor.md', initial_content: '# Harbor\n\nA note.\n' });
  });
  test('a failed open says why, opens nothing, and is asked again next time', async () => {
    fail = (c) => (c.path === '/api/texture/files/open' ? Object.assign(new Error('x'), { status: 502 }) : null);
    const r = await F.fileResolve({ address: 'file:/Lantern report.pdf', title: '' }, storage, net);
    expect(r).toMatchObject({ target: '', status: 'error' }); expect(r.message).toBe('Lantern report.pdf could not be opened. Could not read your files. Tap the file to try again.');
    fail = null; const again = await F.fileResolve({ address: 'file:/Lantern report.pdf', title: '' }, storage, net); expect(again.status).toBe('ready');
  });
  test('a file that is not there, a text over 1 MB and a binary named .txt each end in a stated error', async () => {
    expect((await F.fileResolve({ address: 'file:/ghost.pdf', title: '' }, storage, net)).message).toBe('ghost.pdf could not be opened. That file is not there any more. Tap the file to try again.');
    tree['/'] = [{ name: 'big.txt', type: 'file', size: 2_000_000, modified: '2026-11-18T08:00:00Z' }, { name: 'bin.txt', type: 'file', size: 3, modified: '2026-11-18T08:00:00Z' }];
    blobs['/bin.txt'] = new Uint8Array([0xff, 0xfe, 0xfd]);
    expect((await F.fileResolve({ address: 'file:/big.txt', title: '' }, storage, net)).message).toContain('too large');
    expect((await F.fileResolve({ address: 'file:/bin.txt', title: '' }, storage, net)).message).toContain('not text');
  });
  test('any other file resolves to its own address, so the deck opens it as a file page', async () => {
    expect(await F.fileResolve({ address: 'file:/readings.csv', title: 'T' }, storage, net)).toEqual({ address: 'file:/readings.csv', target: 'file:/readings.csv', title: 'T', status: 'ready', message: '' });
  });
  test('fileOpen of a PDF page answers the same target, and the page can show an error', async () => {
    fail = (c) => (c.path === '/api/texture/files/open' ? new SignedOutError('x') : null);
    const i = await open('file:/Lantern report.pdf'); expect(i.status).toBe('error'); expect(i.message).toContain('Sign in');
  });
});

describe('F12 to F14 changes, uploads and recents', () => {
  test('F12 each change uses its verb, percent-encodes the path and answers the refreshed folder', async () => {
    let l = await F.fileMkdir({ dir: '/Documents', name: 'New one' }, storage, net);
    expect(calls.find((c) => c.method === 'POST')).toMatchObject({ path: '/api/files/Documents/New%20one' }); expect(l.message).toBe('Made the folder “New one”.'); expect(l.path).toBe('/Documents');
    l = await F.fileDelete({ path: '/Documents/Council notes.txt' }, storage, net);
    expect(calls.find((c) => c.method === 'DELETE')!.path).toBe('/api/files/Documents/Council%20notes.txt'); expect(l.message).toBe('Deleted “Council notes.txt”.');
    l = await F.fileRename({ path: '/Documents/Council notes.txt', name: 'Minutes #2.txt' }, storage, net);
    const mv = calls.find((c) => c.method === 'PATCH')!; expect(mv.body).toEqual({ move_to: '/Documents/Minutes #2.txt' }); expect(l.message).toBe('Renamed to “Minutes #2.txt”.');
  });
  test('bad names and the root are refused with nothing sent', async () => {
    calls = [];
    expect((await F.fileMkdir({ dir: '/', name: 'a/b' }, storage, net)).message).toBe('That name cannot be used. Avoid slashes and empty names.');
    expect((await F.fileRename({ path: '/x.txt', name: '..' }, storage, net)).message).toContain('cannot be used');
    expect((await F.fileDelete({ path: '/' }, storage, net)).message).toBe('That cannot be deleted.');
    expect((await F.fileRename({ path: '/a/../..', name: 'x' }, storage, net)).message).toBe('That cannot be renamed.');
    expect(calls.filter((c) => c.method !== 'GET').length).toBe(0);
  });
  test('the computer\'s refusals are said in plain words', async () => {
    fail = (c) => (c.method === 'PATCH' ? Object.assign(new Error('x'), { status: 409 }) : null);
    expect((await F.fileRename({ path: '/Notes 2.txt', name: 'Harbor.md' }, storage, net)).message).toBe('Something with that name is already there.');
    fail = (c) => (c.method === 'PATCH' ? Object.assign(new Error('x'), { status: 405 }) : null);
    expect((await F.fileRename({ path: '/Notes 2.txt', name: 'x.md' }, storage, net)).message).toBe('This computer does not allow that here yet.');
  });
  // Owner, 2026-11-18: "there is no rule that local files can't go to server... autoradio is an interface into user's own choir vm".
  test('"autoradio is an interface into user\'s own choir vm": a picked local file is uploaded to the listener\'s computer', async () => {
    files.set('app:/tmp/picked/photo.png', new Uint8Array([1, 2, 3]));
    const l = await F.fileUpload({ from: 'app:/tmp/picked/photo.png', name: 'photo.png', dir: '/Documents' }, storage, net);
    const put = calls.find((c) => c.method === 'PUT')!;
    expect(put.path).toBe('/api/files/Documents/photo.png'); expect(Array.from(put.body as Uint8Array)).toEqual([1, 2, 3]); expect(put.headers).toEqual({ 'content-type': 'image/png' });
    expect(l.message).toBe('Added “photo.png”.');
    calls = [];
    expect((await F.fileUpload({ from: '', name: '', dir: '/' }, storage, net)).message).toBe('Nothing was chosen to add.'); expect(calls.some((c) => c.method === 'PUT')).toBe(false);
    expect((await F.fileUpload({ from: 'app:/tmp/picked/gone', name: 'g.txt', dir: '/' }, storage, net)).message).toBe('Nothing was chosen to add.');
  });
  test('F14 recents: newest first, capped, kept, dropped when the file is gone or deleted', async () => {
    tree['/'] = Array.from({ length: 35 }, (_, i) => ({ name: `f${i}.dat`, type: 'file', size: i, modified: '2026-11-18T08:00:00Z' }));
    for (let i = 0; i < 35; i++) await open(`file:/f${i}.dat`);
    let l = await list({ mode: 'recent' });
    expect(l.mode).toBe('recent'); expect(l.title).toBe('Recent'); expect(l.count).toBe(F.RECENT_LIMIT); expect(l.rows[0].name).toBe('f34.dat'); expect(l.rows[0].place).toBe('Files');
    F.forget(); l = await list({ mode: 'recent' }); expect(l.count).toBe(30);                    // read back from app:/data/recent-files.json
    await open('file:/ghost.dat'); await F.fileDelete({ path: '/f34.dat' }, storage, net);
    expect((await list({ mode: 'recent' })).rows[0].name).toBe('f33.dat');
    await F.fileRename({ path: '/f33.dat', name: 'renamed.dat' }, storage, net);
    expect((await list({ mode: 'recent' })).rows[0].name).toBe('renamed.dat');
    F.forget(); files.clear(); expect((await list({ mode: 'recent' })).message).toBe('Files you open appear here.');
  });
});

describe('F16 and F17 words and grants', () => {
  test('F16 nothing the source answers says "Loading"', async () => {
    const all = JSON.stringify([await list(), await list({ path: '/x' }), await list({ mode: 'recent' }), await open('file:/mystery.dat'), await open('file:/ghost'), await open('pub:x')]);
    expect(all).not.toMatch(/loading/i);
  });
  test('F17 the grants cover what the module touches', () => {
    const app = readFileSync(new URL('./app.ts', import.meta.url), 'utf8');
    for (const g of [`fs.read ${F.RECENT_FILE}`, `fs.write ${F.RECENT_FILE}`, `fs.read ${F.CACHE_DIR}`, `fs.write ${F.CACHE_DIR}`, 'fs.read app:/tmp/picked', 'net.fetch ${ORIGIN}']) expect(app).toContain(g);
  });
});

function fromHex(h: string): Uint8Array { return new Uint8Array(h.match(/../g)!.map((x) => parseInt(x, 16))); }
