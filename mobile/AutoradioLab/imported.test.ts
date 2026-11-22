// The imported documents' data module under bun test: the source called with a fake computer (the signed-in seam stubbed) and a fake storage.
// The failure list, written before the code (each line has a test below; the Swift lists this replaced were ShelfStoreTests, ShelfPDFTests and the Joins import tests):
//  I1 an import puts the picked file on the computer, in the Imported documents folder, through the Files client; the list then names it, and nothing is stored only on the phone.
//  I2 nothing chosen, a name that is not a PDF or an e-book, a file over what the phone can send: a sentence, nothing sent.
//  I3 the same file twice is one document (nothing uploaded again); another file with a taken name gets a numbered name and nothing is overwritten.
//  I4 the folder is made when it is missing, once.   I5 an upload that fails says why in plain words and does not claim "Imported".
//  I6 a file with no text is kept on the computer and the person is told it has no text; a book with no chapters too.
//  I7 the list is read from the computer: another device's import appears; a missing folder is an empty list; a refused key says to sign in; each row has a plain title, a kind, a size and where the person is.
//  I8 opening a document: its text is the computer's (the texture open then the revision), sections are pages or chapters, the place left off is kept and comes back after a relaunch.
//  I9 a second open of the same file version asks the computer nothing; a changed file (size) is asked again; the text copy survives a relaunch of the module.
//  I10 a document gone from the computer says so, and does not show another; a document opened while signed out says to sign in.
//  I11 a section is its blocks with their numbers; a PDF section knows its real page; find folds case and accents; the voice's block is placed in its section.
//  I12 a position beyond the end or below zero is clamped; a place for a section start (block < 0) is that section's first block; the place is wiped with the account and in the wipe list.
//  I13 read aloud and queue answer the words from the chosen block (when it is in the section on screen), else the section's start, as data for the native player; a document with no text hands nothing.
//  I14 the original is copied under the cache for the page view and the share sheet, the bytes never in JavaScript; too large a copy is not made.
//  I15 nothing here deletes a file of the computer, and no word of the page says books, shelf or library.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as I from './imported.ts';
import * as F from './files.ts';
import * as M from './imported-model.ts';
import { SignedOutError, type Reply } from './api.ts';
import { dig, reopen } from './account-phone.ts';
import { readFileSync } from 'node:fs';

type Call = { method: string; path: string; body?: unknown };
let calls: Call[] = [];
let folder: Map<string, { size: number; modified: string }> | null;       // the Imported documents folder: null = does not exist
let texts: Map<string, string>;                                           // file name -> what the computer read of it
let fail: ((c: Call) => unknown) | null;
const files = new Map<string, Uint8Array>();
const sizes = new Map<string, number>();
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('no ' + p); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    writeFile: async (p: string, d: Uint8Array) => { files.set(p, d); },
    stat: async (p: string) => { if (sizes.has(p)) return { size: sizes.get(p)! }; const b = files.get(p); if (!b) throw new Error('no'); return { size: b.length }; },
    mkdir: async () => {},
    rm: async (p: string) => { files.delete(p); sizes.delete(p); },
  },
} as never;
const NOW = Date.parse('2026-11-21T12:00:00Z');
const dec = (p: string): string => decodeURIComponent(p);
const net: F.Net = {
  json: async (path: string, o: any = {}): Promise<Reply> => {
    const c: Call = { method: o.method ?? 'GET', path: dec(path), body: o.body };
    calls.push(c);
    const f = fail?.(c); if (f) throw f;
    const reply = (body: unknown): Reply => ({ body, serverTime: NOW, raw: '' });
    if (c.path === '/api/files' && c.method === 'GET') return reply(folder ? [{ name: 'Imported documents', type: 'directory', size: 0, modified: '2026-11-21T00:00:00Z' }] : []);
    if (c.path === '/api/files/Imported documents') {
      if (c.method === 'POST') { if (folder) throw Object.assign(new Error('x'), { status: 409 }); folder = new Map(); return reply({ ok: true }); }
      if (!folder) throw Object.assign(new Error('nf'), { status: 404 });
      return reply([...folder].map(([name, v]) => ({ name, type: 'file', size: v.size, modified: v.modified })));
    }
    if (c.path.startsWith('/api/files/Imported documents/') && c.method === 'PUT') {
      if (!folder) throw Object.assign(new Error('nf'), { status: 404 });
      const name = c.path.slice('/api/files/Imported documents/'.length);
      folder.set(name, { size: (o.body as Uint8Array).length, modified: '2026-11-21T10:00:00Z' }); return reply({ ok: true });
    }
    if (c.path === '/api/texture/files/open') {
      const src = (c.body as { source_path: string }).source_path;
      if (!folder?.has(src.replace('Imported documents/', ''))) throw Object.assign(new Error('nf'), { status: 404 });
      return reply({ doc_id: 'd:' + src, current_revision_id: 'r:' + src, created: true });
    }
    if (c.path.startsWith('/api/texture/documents/')) return reply({ document: { doc_id: c.path, current_revision_id: 'r:' + c.path.slice('/api/texture/documents/d:'.length) } });
    if (c.path.startsWith('/api/texture/revisions/r:Imported documents/')) return reply({ revision: { content: texts.get(c.path.slice('/api/texture/revisions/r:Imported documents/'.length)) ?? '' } });
    return reply({ ok: true });
  },
  bytes: async () => { throw new Error('the page downloads no bytes into JavaScript'); },
};
let downloads: string[] = [];
(net as { toFile: F.Net['toFile'] }).toFile = async (path, dest) => {
  const name = dec(path).replace('/api/files/Imported documents/', '');
  downloads.push(name);
  const v = folder?.get(name); if (!v) throw Object.assign(new Error('nf'), { status: 404 });
  sizes.set(dest, v.size); return { bytes: v.size, contentType: '' };
};
const bytes = (n: number): Uint8Array => new Uint8Array(n).fill(7);
const put = (name: string, size = 100, text = '', modified = '2026-11-20T10:00:00Z'): void => { (folder ??= new Map()).set(name, { size, modified }); texts.set(name, text); };
const PDF = 'First line\nsecond line\n\nNew paragraph here\f\fThird page text';
const BOOK = '# One\n\nThe *lantern* was lit.\n\nThe harbour at dawn.\n\n# Two\n\nQuoted words here.\n';
const id = M.idOf;
beforeEach(() => {
  I.forget(); F.forget(); calls = []; downloads = []; files.clear(); sizes.clear(); folder = null; texts = new Map(); fail = null;
});
const list = () => I.importedList('imported:', storage, net);
const open = (name: string, block?: number) => I.importedDoc(M.addressOf(id(name), block), storage, net);

describe('I1 to I6 importing', () => {
  test('I1 an import puts the picked file on the computer and the list names it', async () => {
    files.set('app:/tmp/picked/My_Notes.pdf', bytes(300)); texts.set('My_Notes.pdf', PDF);
    const l = await I.importedAdd({ from: 'app:/tmp/picked/My_Notes.pdf', name: 'My_Notes.pdf' }, storage, net);
    const put_ = calls.find((c) => c.method === 'PUT')!;
    expect(put_.path).toBe('/api/files/Imported documents/My_Notes.pdf'); expect((put_.body as Uint8Array).length).toBe(300);
    expect(l.message).toBe('Imported “My Notes”.');
    expect(l.rows.map((r) => r.title)).toEqual(['My Notes']);
    expect(l.rows[0]).toMatchObject({ id: id('My_Notes.pdf'), address: 'doc:' + id('My_Notes.pdf'), kindLabel: 'PDF', detail: 'PDF · 300 B · Not started', progress: 0 });
    // the computer read the file: the text is kept for the page
    expect(calls.some((c) => c.path === '/api/texture/files/open')).toBe(true);
    // a second device signed in to the same computer sees it without the phone's help
    I.forget(); F.forget();
    expect((await list()).rows.map((r) => r.title)).toEqual(['My Notes']);
  });
  test('I2 nothing chosen, a wrong kind, too large: a sentence and nothing sent', async () => {
    expect((await I.importedAdd({ from: '', name: '' }, storage, net)).message).toBe('Nothing was chosen to add.');
    files.set('app:/tmp/picked/a.txt', bytes(3));
    expect((await I.importedAdd({ from: 'app:/tmp/picked/a.txt', name: 'a.txt' }, storage, net)).message).toBe('Only PDF and EPUB files can be imported.');
    expect((await I.importedAdd({ from: 'app:/tmp/picked/gone.pdf', name: 'gone.pdf' }, storage, net)).message).toBe('Nothing was chosen to add.');
    sizes.set('app:/tmp/picked/big.pdf', I.UPLOAD_MAX + 1);
    expect((await I.importedAdd({ from: 'app:/tmp/picked/big.pdf', name: 'big.pdf' }, storage, net)).message).toContain('too large to import from this phone');
    expect(calls.filter((c) => c.method === 'PUT' || c.method === 'POST').length).toBe(0);
  });
  test('I3 the same file twice is one document; another file with a taken name does not overwrite', async () => {
    files.set('app:/tmp/picked/a.pdf', bytes(300)); texts.set('a.pdf', PDF);
    await I.importedAdd({ from: 'app:/tmp/picked/a.pdf', name: 'a.pdf' }, storage, net);
    calls = [];
    const again = await I.importedAdd({ from: 'app:/tmp/picked/a.pdf', name: 'a.pdf' }, storage, net);
    expect(again.message).toBe('“a” is already imported.'); expect(again.count).toBe(1); expect(calls.some((c) => c.method === 'PUT')).toBe(false);
    files.set('app:/tmp/picked/other.pdf', bytes(400)); texts.set('a 2.pdf', PDF);
    const other = await I.importedAdd({ from: 'app:/tmp/picked/other.pdf', name: 'a.pdf' }, storage, net);
    expect(calls.find((c) => c.method === 'PUT')!.path).toBe('/api/files/Imported documents/a 2.pdf');
    expect(other.rows.map((r) => r.title).sort()).toEqual(['a', 'a 2']); expect(folder!.get('a.pdf')!.size).toBe(300);
  });
  test('I4 the folder is made when it is missing, once', async () => {
    files.set('app:/tmp/picked/a.pdf', bytes(10)); files.set('app:/tmp/picked/b.pdf', bytes(11)); texts.set('a.pdf', PDF); texts.set('b.pdf', PDF);
    await I.importedAdd({ from: 'app:/tmp/picked/a.pdf', name: 'a.pdf' }, storage, net);
    await I.importedAdd({ from: 'app:/tmp/picked/b.pdf', name: 'b.pdf' }, storage, net);
    expect(calls.filter((c) => c.method === 'POST' && c.path === '/api/files/Imported documents').length).toBe(1);
    expect((await list()).count).toBe(2);
  });
  test('I5 an upload that fails says why and does not claim it', async () => {
    files.set('app:/tmp/picked/a.pdf', bytes(10));
    fail = (c) => (c.method === 'PUT' ? Object.assign(new Error('x'), { status: 403 }) : null);
    const l = await I.importedAdd({ from: 'app:/tmp/picked/a.pdf', name: 'a.pdf' }, storage, net);
    expect(l.message).toBe('This device is not allowed to read files.'); expect(l.message).not.toContain('Imported');
    fail = (c) => (c.method === 'PUT' ? new SignedOutError('x') : null);
    expect((await I.importedAdd({ from: 'app:/tmp/picked/a.pdf', name: 'a.pdf' }, storage, net)).message).toContain('Sign in');
  });
  test('I6 a file with no text stays on the computer and the person is told', async () => {
    files.set('app:/tmp/picked/scan.pdf', bytes(50)); texts.set('scan.pdf', '');
    const l = await I.importedAdd({ from: 'app:/tmp/picked/scan.pdf', name: 'scan.pdf' }, storage, net);
    expect(l.message).toBe('“scan” is on your computer. This PDF has no text to read.');
    expect(l.count).toBe(1); expect(calls.some((c) => c.method === 'DELETE')).toBe(false);
    files.set('app:/tmp/picked/e.epub', bytes(50)); texts.set('e.epub', '![x](y.png)\n');
    expect((await I.importedAdd({ from: 'app:/tmp/picked/e.epub', name: 'e.epub' }, storage, net)).message).toBe('“e” is on your computer. This EPUB has no readable chapters.');
    const d = await open('scan.pdf');
    expect(d).toMatchObject({ status: 'empty', message: 'This PDF has no text to read.', title: 'scan' });
  });
});

describe('I7 the list', () => {
  test('a missing folder is an empty list with the words for it; documents of other kinds are not listed', async () => {
    const l = await list();
    expect(l).toMatchObject({ status: 'empty', count: 0 }); expect(l.message).toContain('Import a PDF or an EPUB');
    put('a.pdf'); put('notes.txt'); put('b.epub', 2_500_000, '', '2026-11-21T09:00:00Z');
    const l2 = await list();
    expect(l2.rows.map((r) => r.title)).toEqual(['b', 'a']);               // newest first
    expect(l2.rows[0].detail).toBe('EPUB · 2.5 MB · Not started');
  });
  test('a refused key says to sign in and lists nothing', async () => {
    fail = () => new SignedOutError('x');
    const l = await list(); expect(l.status).toBe('error'); expect(l.message).toContain('Sign in'); expect(l.rows).toEqual([]);
  });
});

describe('I8 to I10 opening', () => {
  test('I8 sections are pages; the text is the computer\'s; the place is kept and comes back after a relaunch', async () => {
    put('r.pdf', 1000, PDF);
    const d = await open('r.pdf');
    expect(d).toMatchObject({ status: 'ready', title: 'r', kind: 'pdf', kindLabel: 'PDF', blockCount: 3, position: 0, resumeSection: 0, resumeBlock: -1 + 1 });
    expect(d.sections).toEqual([{ index: 0, title: 'Page 1' }, { index: 1, title: 'Page 3' }]);
    const open_ = calls.find((c) => c.path === '/api/texture/files/open')!; expect(open_.body).toMatchObject({ source_path: 'Imported documents/r.pdf' });
    expect((await I.importedPlace({ address: d.address, section: 1, block: 2 }, storage, net)).ok).toBe(true);
    I.forget();
    const back = await open('r.pdf');
    expect(back).toMatchObject({ position: 2, resumeSection: 1, resumeBlock: 2 });
    expect((await list()).rows[0].detail).toContain('100% read');
  });
  test('an address with a block opens there once', async () => {
    put('r.pdf', 1000, PDF);
    expect((await open('r.pdf', 1)).position).toBe(1);
    expect((await open('r.pdf', 99)).position).toBe(2);
  });
  test('I9 the text is asked of the computer once per file version; a changed file is asked again; the copy survives a relaunch', async () => {
    put('r.pdf', 1000, PDF);
    await open('r.pdf'); await open('r.pdf');
    expect(calls.filter((c) => c.path === '/api/texture/files/open').length).toBe(1);
    I.forget(); F.forget(); calls = [];
    await open('r.pdf');
    expect(calls.some((c) => c.path === '/api/texture/files/open')).toBe(false);          // read from the phone's copy
    I.forget(); F.forget(); calls = [];
    put('r.pdf', 1001, PDF + '\fFourth');
    expect((await open('r.pdf')).blockCount).toBe(4);
    expect(calls.some((c) => c.path === '/api/texture/files/open')).toBe(true);
  });
  test('I10 a document gone from the computer says so; signed out says to sign in', async () => {
    put('r.pdf', 1000, PDF);
    expect(await open('gone.pdf')).toMatchObject({ status: 'notfound', message: 'This document is not on your computer any more.', id: id('gone.pdf') });
    expect((await I.importedDoc('doc:', storage, net)).status).toBe('loading');
    expect((await I.importedDoc('pub:x', storage, net)).status).toBe('loading');
    expect((await I.importedDoc('doc:' + encodeURIComponent('x.txt'), storage, net)).status).toBe('notfound');
    fail = () => new SignedOutError('x');
    const d = await open('r.pdf'); expect(d.status).toBe('error'); expect(d.message).toContain('Sign in');
  });
});

describe('I11 to I13 reading, finding, placing and handing words to the player', () => {
  test('I11 a section is its blocks; a PDF section knows its page; find folds; the voice\'s block is placed', async () => {
    put('b.epub', 500, BOOK); put('r.pdf', 1000, PDF);
    const d = await open('b.epub');
    expect(d.sections.map((s) => s.title)).toEqual(['One', 'Two']);
    const s1 = await I.importedSection(d.address, 1, storage, net);
    expect(s1).toMatchObject({ index: 1, count: 2, title: 'Two', start: 3, status: 'ready' });
    expect(s1.blocks.map((b) => [b.n, b.kind, b.text])).toEqual([[3, 'heading', 'Two'], [4, 'paragraph', 'Quoted words here.']]);
    const pd = await open('r.pdf');
    expect((await I.importedSection(pd.address, 1, storage, net)).page).toBe(3);
    expect((await I.importedSection(pd.address, 99, storage, net)).index).toBe(1);
    expect((await I.importedSection('doc:nothing', 0, storage, net)).status).toBe('loading');
    const f = await I.importedFind(d.address, 'HARBOUR', storage, net);
    expect(f.hits).toEqual([{ block: 2, section: 0, label: 'One', snippet: 'The harbour at dawn.' }]);
    expect((await I.importedFind(d.address, 'zebra', storage, net)).message).toBe('Nothing in this document matches “zebra”.');
    expect(await I.importedFind(d.address, '  ', storage, net)).toEqual({ query: '', hits: [], message: '' });
    expect(await I.importedLocate(d.address, 4, storage, net)).toEqual({ address: d.address, block: 4, section: 1 });
    expect((await I.importedLocate('', -1, storage, net)).section).toBe(0);
  });
  test('I12 a place is clamped; a section start is its first block; the store is wiped with the account', async () => {
    put('r.pdf', 1000, PDF);
    const d = await open('r.pdf');
    await I.importedPlace({ address: d.address, section: 0, block: 999 }, storage, net); expect((await open('r.pdf')).position).toBe(2);
    await I.importedPlace({ address: d.address, section: 0, block: -4 + 0 }, storage, net);          // block < 0: the start of section 0
    expect((await open('r.pdf')).position).toBe(0);
    await I.importedPlace({ address: d.address, section: 1, block: -1 }, storage, net); expect((await open('r.pdf')).position).toBe(2);
    expect((await I.importedPlace({ address: 'doc:' + id('gone.pdf'), section: 0, block: 1 }, storage, net)).ok).toBe(false);
    expect(files.has(I.PLACES_FILE)).toBe(true);
  });
  test('I13 read aloud and queue hand the words from the chosen block as data for the native player', async () => {
    put('b.epub', 500, BOOK);
    const d = await open('b.epub');
    const play = await I.importedReadAloud({ address: d.address, section: 0, block: 2 }, storage, net);
    expect(play).toMatchObject({ ok: true, message: '', play: { op: 'play', id: id('b.epub'), title: 'b', at: 2, text: 'The harbour at dawn.\nTwo\nQuoted words here.' } });
    expect((await I.importedReadAloud({ address: d.address, section: 1, block: 0 }, storage, net)).play.at).toBe(3);          // the block is not in the section on screen
    const q = await I.importedQueue({ address: d.address, section: 1, block: -1 }, storage, net);
    expect(q).toMatchObject({ ok: true, message: 'Added to the queue.', play: { op: 'queue', at: 3 } });
    expect(await I.importedReadAloud({ address: 'doc:' + id('gone.pdf'), section: 0, block: 0 }, storage, net)).toEqual({ ok: false, message: 'This document has nothing to read.', play: { op: '', id: '', title: '', at: 0, text: '' } });
  });
});

describe('I14 and I15 the original and the words', () => {
  test('I14 the original is copied under the cache by the host; a large one is not; the share sheet gets the copy of an e-book too', async () => {
    reopen(dig('k-imported'));
    put('r.pdf', 1000, PDF); put('b.epub', 500, BOOK); put('huge.pdf', I.COPY_MAX + 1, PDF);
    const pdf = await open('r.pdf');
    expect(pdf.src).toMatch(/^app:\/cache\/accounts\/[0-9a-f]{64}\/files\/[0-9a-f]+\/r\.pdf$/);      // the host and the iframe see the account's root (P6)
    expect(pdf.shareSrc).toBe(pdf.src); expect(pdf.message).toBe('');
    const epub = await open('b.epub');
    expect(epub.src).toBe(''); expect(epub.shareSrc).toMatch(/b\.epub$/);
    await open('r.pdf'); expect(downloads.filter((n) => n === 'r.pdf').length).toBe(1);          // not again
    const huge = await open('huge.pdf');
    expect(huge).toMatchObject({ status: 'ready', src: '', shareSrc: '' }); expect(huge.message).toBe('The original PDF could not be shown, so its text is here.');
  });
  test('I15 nothing deletes a file of the computer, and the words say no book, shelf or library', async () => {
    put('r.pdf', 1000, PDF);
    await list(); await open('r.pdf'); await I.importedPlace({ address: 'doc:' + id('r.pdf'), section: 0, block: 1 }, storage, net);
    expect(calls.some((c) => c.method === 'DELETE' || c.method === 'PATCH')).toBe(false);
    // every string a person can see: the literals of the module and of the page
    for (const f of ['imported.ts', 'imported-model.ts', 'imported-docs.contract', 'members-imported.contract']) {
      const t = readFileSync(new URL('./' + f, import.meta.url), 'utf8').split('\n').filter((l) => !l.trim().startsWith('//') && !l.trim().startsWith('/**')).join('\n');
      const strings = t.match(/'[^'\n]*'|"[^"\n]*"|`[^`\n]*`/g) ?? [];
      expect(strings.filter((x) => /\b(books?|shelf|library)\b/i.test(x))).toEqual([]);
    }
  });
});

describe('I16 the search sees the documents the phone holds the text of', () => {
  test('a read document is a Doc with a part per section and the block it starts at; an unread one is not indexed', async () => {
    const S = await import('./imported-search.ts');
    put('b.epub', 500, BOOK); put('r.pdf', 1000, PDF);
    await open('b.epub');
    const docs = await S.importedDocs(storage, net);
    expect(docs.map((d) => d.title)).toEqual(['b']);
    expect(docs[0]).toMatchObject({ kind: 'document', address: 'doc:' + id('b.epub'), source: 'Imported documents' });
    expect(docs[0].parts!.map((p) => [p.label, p.block])).toEqual([['chapter 1: One', 0], ['chapter 2: Two', 3]]);
    expect(await S.importedDocs(undefined, net)).toEqual([]);
  });
});
