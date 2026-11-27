// Shared state is not sent to Swift to be read back (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 4). Failure list, written first:
//  C1 the mail approval page opens in the shared web layer only for https on the paper's own origin with no user name or password; anything else is refused with the words, and nothing opens.
//  C2 an approval link that passes is handed to the web layer's opener; one that is refused is not.
//  C3 the page-follow chain behind the dead native `importedFollow` is gone: no `importedLocate` in shared code, no `ldFollow` in a contract (nothing native ever set the block, so the chain never ran).
//  C4 the names `mailOpenReview` and `importedFollow` are gone from the contracts, the interface and the Swift handlers, and the two module files with them.
import { describe, expect, test } from 'bun:test';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { reviewLink, mailReviewOpen, REVIEW_REFUSED } from './mail.ts';
import * as imported from './imported.ts';

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

describe('the imported documents place asks Swift for nothing', () => {
  test('C3 no follow chain', () => {
    expect((imported as Record<string, unknown>).importedLocate).toBeUndefined();
    const contracts = readdirSync('.').filter((f) => f.endsWith('.contract')).map((f) => readFileSync(f, 'utf8')).join('\n');
    expect(contracts).not.toMatch(/ldFollow|importedLocate|ImportedLocate|ImportedFollow/);
  });
  test('C4 the names and the two module files are gone', () => {
    const names = ['mailOpenReview', 'importedFollow'];
    const contracts = readdirSync('.').filter((f) => f.endsWith('.contract')).map((f) => readFileSync(f, 'utf8')).join('\n');
    const iface = readFileSync('../exact-package/exact.json', 'utf8') + readdirSync('../exact-package/interface').map((f) => readFileSync(`../exact-package/interface/${f}`, 'utf8')).join('\n');
    const swift = readdirSync('../exact-package/apple').filter((f) => f.endsWith('.swift')).map((f) => readFileSync(`../exact-package/apple/${f}`, 'utf8')).join('\n');
    for (const n of names) {
      expect(contracts).not.toMatch(new RegExp(`\\b${n}\\(`));
      expect(iface).not.toContain(`"${n}"`);
      expect(swift).not.toContain(`"${n}"`);
    }
    expect(existsSync('../exact-package/apple/AutoradioModule+Imported.swift')).toBe(false);
    expect(existsSync('../exact-package/apple/AutoradioModule+Mail.swift')).toBe(false);
    expect(swift).not.toContain('registerMail');
    expect(swift).not.toContain('registerImported');
  });
});
