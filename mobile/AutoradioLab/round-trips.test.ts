// Shared state is not sent to Swift to be read back (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 4). Failure list, written first:
//  C1 the mail approval page opens in the shared web layer only for https on the paper's own origin with no user name or password; anything else is refused with the words, and nothing opens.
//  C2 an approval link that passes is handed to the radio's web layer (queueAct "openWeb", the shared state the bar reads); one that is refused is not.
//  C3 the picks are not pushed to native: no `editionMirror` or `editionMirrorReset` is sent when the picks change or are wiped.
//  C4 the voice turn's retrieval takes the open pages, the edition topics and the text of the page in front from shared code, not from what native hands back in the ask.
//  C5 the dead and round-trip names are gone from the contracts, the interface and the Swift handlers: mailOpenReview, readerAim, readerAimSet, importedFollow, editionMirror, deckPlay, deckEnqueue, deckAct, deckAck.
import { beforeEach, describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { reviewLink, mailReviewOpen, REVIEW_REFUSED } from './mail.ts';
import * as picks from './edition-picks.ts';
import { resetSeam } from './api.ts';
import { sharedFacts } from './retrieval-live.ts';
import { pagesOpen, forget as forgetPages } from './pages.ts';
import { holdForTest, resetReaderForTest } from './reader.ts';

function memFs() {
  const files = new Map<string, Uint8Array>();
  const fs = {
    directories: { data: 'app:/data', cache: 'app:/cache', temporary: 'app:/tmp' },
    atomicWriteFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
    writeFile: async (p: string, b: Uint8Array) => { files.set(p, b); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    stat: async (p: string) => { if (files.has(p)) return { size: 1, modifiedMs: 0 }; throw new Error('ENOENT'); },
    rm: async (p: string) => { files.delete(p); },
    mkdir: async () => undefined,
    readdir: async () => [],
  };
  return { files, storage: { fs, sqlite: {} } as never };
}

beforeEach(() => { resetSeam(); picks.forgetPicks(); forgetPages(); resetReaderForTest(); });

describe('the mail approval page', () => {
  test('C1 only https on the paper\'s own origin, with no user name, opens', () => {
    expect(reviewLink('https://choir.news/mail/review/x')).toBe('https://choir.news/mail/review/x');
    expect(reviewLink('https://choir.news:443/mail/review/x')).toBe('https://choir.news/mail/review/x');
    for (const bad of ['http://choir.news/mail/review/x', 'https://evil.example/mail/review/x', 'https://choir.news.evil.example/x', 'https://u:p@choir.news/x', 'https://choir.news:8443/x', 'javascript:alert(1)', '', 'not a url'])
      expect(reviewLink(bad)).toBe('');
  });
  test('C2 a link that passes is handed to the shared web layer; a refused one is not', async () => {
    const opened: string[] = [];
    expect(await mailReviewOpen('https://choir.news/mail/review/x', async (u) => { opened.push(u); })).toEqual({ status: 'opened', message: '' });
    expect(await mailReviewOpen('https://evil.example/x', async (u) => { opened.push(u); })).toEqual({ status: 'refused', message: REVIEW_REFUSED });
    expect(opened).toEqual(['https://choir.news/mail/review/x']);
  });
});

describe('the edition picks', () => {
  test('C3 no copy of the picks is pushed to native', async () => {
    const ops: string[] = [];
    const device = picks.nativeDevice;
    void device;
    const m = memFs();
    const env = { live: async () => [] as string[], device: { heard: async () => [], said: async () => undefined, op: (o: string) => ops.push(o) } as never };
    await picks.addTyped(m.storage, 'Chess', env);
    await picks.toggle(m.storage, 'Chess', env);
    await picks.wipePicks(m.storage, env);
    expect(ops).toEqual([]);
    expect(readFileSync('edition-picks.ts', 'utf8')).not.toContain('editionMirror');
  });
});

describe('the voice turn\'s retrieval facts', () => {
  test('C4 the open pages, the topics and the page in front come from shared code', async () => {
    const m = memFs();
    await pagesOpen(m.storage, 'pub:one', 'One story');
    m.files.set(picks.PICKS_FILE, new TextEncoder().encode(JSON.stringify({ topics: ['Chess', 'Space'] })));
    holdForTest({ address: 'pub:one', status: 'ready', title: 'One story', blocks: [{ id: 'b1', kind: 'paragraph', text: 'The harbour at dawn.' }, { id: 'b2', kind: 'paragraph', text: 'Boats leave.' }] } as never);
    const f = await sharedFacts(m.storage, 'pub:one');
    expect(f.pages.map((p) => p.address)).toContain('pub:one');
    expect(f.topics).toEqual(['Chess', 'Space']);
    expect(f.pageText).toBe('The harbour at dawn. Boats leave.');
    expect((await sharedFacts(m.storage, 'pub:other')).pageText).toBe('');
  });
});

describe('the round-trip names are gone', () => {
  test('C5 no contract, interface entry or Swift handler names them', () => {
    const names = ['mailOpenReview', 'readerAim', 'readerAimSet', 'importedFollow', 'editionMirror', 'editionMirrorReset', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck'];
    const contracts = readdirSync('.').filter((f) => f.endsWith('.contract')).map((f) => readFileSync(f, 'utf8')).join('\n');
    const iface = readFileSync('../exact-package/exact.json', 'utf8') + readdirSync('../exact-package/interface').map((f) => readFileSync(`../exact-package/interface/${f}`, 'utf8')).join('\n');
    const swift = readdirSync('../exact-package/apple').filter((f) => f.endsWith('.swift')).map((f) => readFileSync(`../exact-package/apple/${f}`, 'utf8')).join('\n') + readFileSync('modules/apple/ModelsEntry.swift', 'utf8')
      + readFileSync('../AutoradioKit/Sources/AutoradioKit/EditionSetup.swift', 'utf8');
    for (const n of names) {
      expect(contracts).not.toMatch(new RegExp(`\\b${n}\\(`));
      expect(iface).not.toContain(`"${n}"`);
      if (n.startsWith('deck')) expect(swift).not.toContain(`handle("${n}"`);         // the app module's one `deck` op keeps the names as its calls (deck.md); only the package sends go
      else expect(swift).not.toContain(`"${n}"`);
    }
    expect(swift).not.toContain('for name in ["deckPlay"');
    expect(contracts).not.toContain('use readerFollow, readerAim');
  });
});
