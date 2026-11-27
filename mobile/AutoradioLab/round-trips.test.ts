// Shared state is not sent to Swift to be read back (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md, item 4). Failure list, written first:
//  C1 the mail approval page opens in the shared web layer only for https on the paper's own origin with no user name or password; anything else is refused with the words, and nothing opens.
//  C2 an approval link that passes is handed to the web layer's opener; one that is refused is not.
//  C3 the page-follow chain behind the dead native `importedFollow` is gone: no `importedLocate` in shared code, no `ldFollow` in a contract (nothing native ever set the block, so the chain never ran).
//  C4 the names `mailOpenReview` and `importedFollow` are gone from the contracts, the interface and the Swift handlers, and the two module files with them.
import { describe, expect, test } from 'bun:test';
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { reviewLink, reviewLinkOn, mailReviewOpen, REVIEW_REFUSED, REVIEW_FAILED } from './mail.ts';
import { queueAct } from './radio-bridge.ts';
import * as imported from './imported.ts';

const OK = 'https://choir.news/mail/review/x';
describe('the mail approval page', () => {
  test('C1 what opens: the paper\'s own origin over https, then / ? # or nothing', () => {
    for (const good of [OK, 'https://choir.news', 'https://choir.news/', 'https://choir.news?a=1', 'https://choir.news#f', 'https://choir.news/a%20b/c?x=y@z#w']) expect(reviewLink(good)).toBe(good);
  });
  test('C1b whitespace, control characters and backslashes are refused (the Swift URL(string:) refused them; a parser would normalise them away)', () => {
    for (const bad of [' ' + OK, OK + ' ', OK + '\n', 'https://choir\t.news/x', 'https://choir\n.news/x', 'https://choir\r.news/x', 'https://choir.news/a b', 'https:\\\\choir.news\\mail\\review\\x', 'https://choir.news\\@evil.example/x', 'https://choir.news/a\\b', 'https://choir.news/\u0000', 'https://choir.news/\u00e9'])
      expect(reviewLink(bad)).toBe('');
  });
  test('C1c a host written another way is refused: percent-encoded, full-width, IDNA-mapped, trailing dot, suffix, substring, userinfo', () => {
    for (const bad of ['https://choir%2enews/x', 'https://%63hoir.news/x', 'https://\uff43\uff48\uff4f\uff49\uff52.\uff4e\uff45\uff57\uff53/x', 'https://choir\u3002news/x', 'https://choir.news./x', 'https://choir.news.evil.example/x', 'https://evilchoir.news/x', 'https://choir.newsx/x',
      'https://u@choir.news/x', 'https://@choir.news/x', 'https://u:p@choir.news/x', 'https://choir.news@evil.example/x', 'https://evil.example@choir.news/x', 'https://choir.news:8443/x', 'https://choir.news:/x', 'https://evil.example/https://choir.news/x'])
      expect(reviewLink(bad)).toBe('');
  });
  test('C1d other schemes and shapes are refused: http, relative, protocol-relative, javascript, data, empty, not a string; and (stricter than the Swift, which lower-cased) an upper-case scheme or host and an explicit :443', () => {
    for (const bad of ['http://choir.news/x', '/x', '//choir.news/x', 'choir.news/x', 'javascript:alert(1)', 'data:text/html,x', 'mailto:a@choir.news', '', 'HTTPS://choir.news/x', 'https://CHOIR.news/x', 'https://choir.news:443/x'])
      expect(reviewLink(bad)).toBe('');
    for (const bad of [undefined, null, 1, {}, ['https://choir.news/x'], { toString: () => OK }]) expect(reviewLink(bad)).toBe('');
  });
  test('C2 a link that passes is handed to the shared web layer; a refused one is not', async () => {
    const opened: string[] = [];
    expect(await mailReviewOpen(OK, async (u) => { opened.push(u); return { webUrl: u }; })).toEqual({ status: 'opened', message: '' });
    expect(await mailReviewOpen('https://evil.example/x', async (u) => { opened.push(u); return { webUrl: u }; })).toEqual({ status: 'refused', message: REVIEW_REFUSED });
    expect(await mailReviewOpen(' ' + OK, async (u) => { opened.push(u); return { webUrl: u }; })).toEqual({ status: 'refused', message: REVIEW_REFUSED });
    expect(opened).toEqual([OK]);
  });
  test('C2b only an opener that answers the layer showing this very link is "opened"; it throws, answers nothing, {}, another page or a message is a failure', async () => {
    expect(await mailReviewOpen(OK, async () => { throw new Error('x'); })).toEqual({ status: 'failed', message: REVIEW_FAILED });
    for (const answer of [undefined, null, {}, { webUrl: '' }, { webUrl: OK + 'x' }]) expect(await mailReviewOpen(OK, async () => answer as never)).toEqual({ status: 'failed', message: REVIEW_FAILED });
    expect(await mailReviewOpen(OK, async () => ({ message: 'Closed.' }))).toEqual({ status: 'failed', message: 'Closed.' });
    expect(await mailReviewOpen(OK, async () => ({ webUrl: 'https://other.example/', message: 'Closed.' }))).toEqual({ status: 'failed', message: 'Closed.' });
    expect(await mailReviewOpen(OK, async () => ({ webUrl: OK }))).toEqual({ status: 'opened', message: '' });
  });
  test('C2c the real opener (the radio\'s openWeb) leaves the page in the web layer', async () => {
    const r = await mailReviewOpen(OK, (u) => queueAct(undefined, undefined, 'openWeb', u));
    expect(r).toEqual({ status: 'opened', message: '' });
    expect((await queueAct(undefined, undefined, 'openWeb', OK)).webUrl).toBe(OK);
  });
});

describe('the origin', () => {
  test('C1e a trailing slash on the origin is dropped once, and the same links pass and fail', () => {
    expect(reviewLinkOn(OK, 'https://choir.news/')).toBe(OK);
    expect(reviewLinkOn('https://choir.news', 'https://choir.news/')).toBe('https://choir.news');
    expect(reviewLinkOn('https://choir.news.evil.example/x', 'https://choir.news/')).toBe('');
    expect(reviewLinkOn('https://choir.news:8443/x', 'https://choir.news//')).toBe('');
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
