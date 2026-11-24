process.env.TZ = 'UTC';   // dates are the listener's wall clock; the tests fix it
// Mail under bun test: the model, the sanitiser, and the sources with a canned computer (the signed-in seam `./api.ts` is replaced, its real
// request is api.test.ts's) and a fake storage and secret store. Fixtures are invented people at example.com (mobile/fixtures/mail).
//
// Failure list, written before the code (each line has a test below; the numbers are the Swift list in mail-model.ts and MailTests/RepairTests that this replaced):
//  M1  threading: prefixes, repeats, an empty subject never groups (8).   M2  readable text: html only, script and style dropped, "(no text)", capped and said so (5, 6).
//  M3  blocks: re-flow, quotes, signature (the MailTests testBlocks).      M4  dates, with and without fractions, unparseable raw (12).   M5  attachments (9), addresses, reply quote (10, 11).
//  M6  a sparse JSON message never throws (13).   M7  the 62-row hostile HTML table, what is kept, an unclosed head, escaped attributes, entity decoding (RepairTests MailSanitizerTests).
//  B1  the mailbox: ready, empty, a server error with its words, 401 words, offline keeps the last good rows, a cold offline start shows the phone's copy (1, 2, 3, 4).
//  B2  one request per answer; the request is exact (folder, limit); the real list shape (nanosecond times, next_cursor).
//  G1  a message: the view, the thread, mark-read once and a later "unread" is not undone, 404 words, "idle" for new/reply/draft addresses (4, 9).
//  G2  "Show original": the cleaned document is written only when asked, nothing remote survives, other documents are removed, no HTML is kept in the phone's copy.
//  C1  reply addressing (Reply-To, outbound, "Re: " once), a refused draft makes no request, the request body, no alias, the approval link, 403 words, send is never called (10, 11).
//  P1  the phone's copy: plain JSON files written by shared code (no cipher, no native call), no storage means no file, a file that does not parse is ignored and removed, the oldest go past the limit.
//  P2  the catch-up: fetches missing bodies, skips held ones, resumes from its cursor, stops at the budget, counts only.   P3  search over the copy.
//  W1  the old copy: leftover `app:/cache/mail-*.html` are removed at first use; `wipeMail` removes every file; the old Swift store's folder is removed by the native sign-out wipe (AutoradioModule.swift "app-files").
//  L1  logs carry counts and ids only (14).   A1  the grants match the paths the modules use.
import { beforeEach, describe, expect, test } from 'bun:test';
import * as M from './mail-model.ts';
import { sanitized, clean, decodeAttr } from './mail-sanitize.ts';
import * as C from './mail-store.ts';
import hostile from '../fixtures/mail/hostile-html.json';
import inboxList from '../fixtures/mail/inbox-list.json';
import inboxPage2 from '../fixtures/mail/inbox-page2.json';
import msgPlain from '../fixtures/mail/message-plain.json';
import msgHtml from '../fixtures/mail/message-html-only.json';
import msgOut from '../fixtures/mail/message-outbound.json';

const NOW = Date.parse('2026-11-18T18:00:00Z');

// ---- the canned computer: the real signed-in seam (api.ts) over a stubbed `fetch` and a stubbed native key reader
import { SignedOutError, HTTPError, bindNative, resetSession, resetSeam, beginSignOut } from './api.ts';
type Req = { path: string; opts: { method?: string; query?: Record<string, string | number>; body?: unknown } };
let calls: Req[] = [];
let route: (r: Req) => { status: number; body: unknown } | 'network' = () => ({ status: 404, body: { error: 'not found' } });
const mail = await import('./mail.ts');
const app = await import('./app.ts');
let signedOutNative = false;
const reader = { available: true, later: async (r: { op?: string; account?: string }) => {
  if (r.op === 'session') return { found: true, value: 'test-key-0001-not-real', computerId: 'comp-1' };
  if (r.op === 'signOut') { signedOutNative = true; return { done: true }; }
  return { found: r.account !== 'key-id', value: r.account === 'api-key' ? 'test-key-0001-not-real' : 'comp-1' };
} };
function stubFetch(): void {
  globalThis.fetch = (async (u: string, init: RequestInit) => {
    const url = new URL(String(u));
    const query: Record<string, string | number> = {};
    url.searchParams.forEach((v, k) => { query[k] = /^\d+$/.test(v) ? Number(v) : v; });
    const req: Req = { path: url.pathname, opts: { method: init.method, query: Object.keys(query).length ? query : undefined, body: init.body ? JSON.parse(String(init.body)) : undefined } };
    calls.push(req);
    if (fetchGate && req.opts.method === 'POST') await fetchGate;
    const a = route(req);
    if (a === 'network') throw new Error('The Internet connection appears to be offline');
    return new Response(JSON.stringify(a.body), { status: a.status, headers: { date: new Date(NOW).toUTCString(), 'content-type': 'application/json' } });
  }) as never;
}

// ---- a fake storage and secret store
const files = new Map<string, Uint8Array>();
const err = (code: string): Error => Object.assign(new Error(code), { code });
const storage = {
  fs: {
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw err('ENOENT'); return b; },
    atomicWriteFile: async (p: string, d: Uint8Array) => { writeStarted += 1; if (writeGate) await writeGate; files.set(p, new Uint8Array(d)); },
    mkdir: async (p: string) => { dirs.add(p); },
    rm: async (p: string) => { if (dirs.delete(p)) return; if (!files.delete(p)) { const dir = [...files.keys()].some((k) => k.startsWith(p + '/')); if (dir) throw err('ENOTEMPTY'); throw err('ENOENT'); } },
    readdir: async (p: string) => {
      if (readdirGate && p === 'app:/cache') await readdirGate;
      const names = [...files.keys()].filter((k) => k.startsWith(p + '/')).map((k) => k.slice(p.length + 1)).filter((n) => !n.includes('/'));
      if (names.length === 0) throw err('ENOENT');
      return names;
    },
    stat: async (p: string) => { const b = files.get(p); if (!b) throw err('ENOENT'); return { size: b.length, isFile: true, isDirectory: false, modifiedMs: mtimes.get(p) ?? 0 }; },
  },
} as never;
const mtimes = new Map<string, number>();
let fetchGate: Promise<void> | null = null; const dirs = new Set<string>(); let writeStarted = 0; let writeGate: Promise<void> | null = null; let readdirGate: Promise<void> | null = null;
const env = (): C.Env => ({ storage });
import { createHash } from 'node:crypto';
const hex16 = (id: string): string => createHash('sha256').update(id).digest('hex').slice(0, 32);
const accessor = async (): Promise<C.Files> => (await C.Files.open(env()))!;

let logs: string[] = [];
const realLog = console.log;
beforeEach(() => {
  resetSeam(); bindNative(reader as never); stubFetch();
  mail.forget(); calls = []; files.clear(); mtimes.clear(); signedOutNative = false; fetchGate = null; dirs.clear(); writeStarted = 0; writeGate = null; readdirGate = null; logs = [];
  route = () => ({ status: 404, body: { error: 'not found' } });
  console.log = (...a: unknown[]) => { logs.push(a.join(' ')); };
});
const restore = (): void => { console.log = realLog; };

const SECRETS = ['maya.ortiz@example.com', 'Maya Ortiz', 'Thursday', 'dinner', 'receipt', 'Receipt', 'seven at the usual place', 'maya.replies@example.com'];

/** The canned computer for the real-shaped fixtures. */
function computer(extra: (r: Req) => ReturnType<typeof route> | undefined = () => undefined): void {
  route = (r) => {
    const x = extra(r);
    if (x !== undefined) return x;
    const p = r.path;
    if (p === '/api/email/messages') return { status: 200, body: r.opts.query?.cursor ? inboxPage2 : r.opts.query?.folder === 'sent' ? { messages: [], total: 0, unread: 0 } : inboxList };
    if (p === '/api/email/messages/em_01hz0a') return { status: 200, body: msgPlain };
    if (p === '/api/email/messages/em_01hz08') return { status: 200, body: msgHtml };
    if (p === '/api/email/messages/em_01hz09') return { status: 200, body: msgOut };
    if (p === '/api/email/messages/em_01hz07' || p === '/api/email/messages/em_01hz06') return { status: 200, body: { message: (inboxList.messages.concat(inboxPage2.messages)).find((m) => p.endsWith(m.id)), text_body: 'x', recipients: {} } };
    if (p.endsWith('/read') || p.endsWith('/unread')) return { status: 200, body: { status: 'read' } };
    if (p === '/api/email/aliases') return { status: 200, body: { aliases: [{ id: 'al1', address: 'listener@example.com' }] } };
    if (p === '/api/email/drafts' && r.opts.method === 'POST') return { status: 201, body: { id: 'd1', status: 'draft_pending_owner_approval', to_addresses: (r.opts.body as { to_addresses: string[] }).to_addresses } };
    if (p === '/api/email/drafts/d1/approval-email') return { status: 200, body: { status: 'sent', review_url: 'https://choir.news/mail/review/x' } };
    return { status: 404, body: { error: 'not found' } };
  };
}

const sm = (id: string, subject: string, at: string, o: Partial<M.Summary> = {}): M.Summary => ({
  id, direction: 'inbound', fromAddress: 'a@example.com', fromDisplay: '', subject, snippet: '', trust: 'trusted', readAt: at, at, hasAttachments: false, ...o,
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- the model

describe('M1 to M6 the model', () => {
  test('M1 subjects, threads, the empty subject', () => {
    expect(M.normalizedSubject('Re: Re: Fwd: Q4 Plan')).toBe('q4 plan');
    expect(M.normalizedSubject('[list] RE: Q4 plan')).toBe('q4 plan');
    expect(M.normalizedSubject('   ')).toBe('');
    expect(M.replySubject('Re: Hello')).toBe('Re: Hello'); expect(M.replySubject('Hello')).toBe('Re: Hello'); expect(M.replySubject('')).toBe('Re:');
    const list = [sm('3', 'Re: Plan', '2026-11-09T10:00:00Z', { readAt: '' }), sm('2', 'Plan', '2026-11-08T10:00:00Z'), sm('1', '', '2026-11-07T10:00:00Z'), sm('0', '', '2026-11-06T10:00:00Z')];
    const rows = M.threadRows(list, Date.parse('2026-11-10T00:00:00Z'));
    expect(rows.length).toBe(3);
    expect(rows[0].id).toBe('3'); expect(rows[0].count).toBe(2); expect(rows[0].unread).toBe(true);
    expect(rows[1].subject).toBe('(no subject)');
    expect(M.threadOf(list[1], list).map((m) => m.id)).toEqual(['2', '3']);
    expect(M.threadOf(list[2], list).length).toBe(1);
  });

  test('M2 readable text', () => {
    const t = M.htmlToText('<html><head><style>p{}</style><script>x()</script></head><body><p>Hello &amp; <b>welcome</b></p><ul><li>One</li><li>Two&nbsp;</li></ul>a<br>b&#33;</body></html>');
    expect(t).not.toContain('x()'); expect(t).not.toContain('p{}');
    expect(t).toContain('Hello & welcome'); expect(t).toContain('• One'); expect(t).toContain('a\nb!');
    expect(M.blocks('', '').blocks).toEqual([{ kind: 'p', text: '(no text)' }]);
    const h = M.blocks('', '<p>Only html</p>');
    expect(h.htmlOnly).toBe(true); expect(h.blocks[0].text).toBe('Only html');
    expect(M.blocks('x '.repeat(150_000), '').cut).toBe(true);
  });

  test('M3 blocks: re-flowed, quoted, signed', () => {
    const r = M.blocks('Hello there,\nthis wraps over two long lines of text, which is how plain mail is written when it is sent.\n\nOn Monday Ana wrote:\n> first\n> second\n\n-- \nAna\nHarbour', '');
    expect(r.htmlOnly).toBe(false);
    expect(r.blocks.map((b) => b.kind)).toEqual(['p', 'p', 'quote', 'sig']);
    expect(r.blocks[0].text).not.toContain('\n');
    expect(r.blocks[2].text).toBe('first\nsecond');
    expect(M.hasQuoted(r.blocks)).toBe(true);
  });

  test('M4 dates', () => {
    const now = Date.parse('2026-11-10T12:00:00Z');
    expect(M.shortDate('2026-11-10T08:12:00Z', now, 0)).toBe('08:12');
    expect(M.shortDate('2026-11-08T08:12:00.250Z', now, 0)).toBe('Sun');
    expect(M.shortDate('2026-10-30T08:12:00Z', now, 0)).toBe('30 Oct');
    expect(M.shortDate('2026-11-18T17:40:12.123456789Z', Date.parse('2026-11-18T18:00:00Z'), 0)).toBe('17:40');
    expect(M.shortDate('not a date at all, really long', now, 0)).toBe('not a date at al');
    expect(M.longDate('2026-11-10T08:00:00Z', 0)).toBe('Tue 10 Nov 2026, 08:00');
    expect(M.longDate('2026-11-10T08:00:00Z', -60)).toBe('Tue 10 Nov 2026, 09:00');
  });

  test('M5 attachments, addresses, the reply quote, the refusal of a draft', () => {
    expect(M.safeFilename('../../etc/passwd')).toBe('passwd'); expect(M.safeFilename('')).toBe('attachment'); expect(M.safeFilename('..')).toBe('attachment');
    expect(M.fileSize(482113)).toBe('471 KB'); expect(M.fileSize(10)).toBe('10 B'); expect(M.fileSize(3 * 1048576)).toBe('3.0 MB');
    expect(M.attachmentKind('a.PDF', '')).toBe('pdf'); expect(M.attachmentKind('x', 'image/png')).toBe('image'); expect(M.attachmentKind('b.epub', '')).toBe('book');
    expect(M.addressOnly('Ana R. <ana@example.com>')).toBe('ana@example.com');
    expect(M.problem('', 'hi')).toBe('Add who it is for.');
    expect(M.problem('nobody', 'hi')).toBe('Check the address: nobody');
    expect(M.problem('ana@example.com', '\n\nOn Mon, X wrote:\n> q')).toBe('Write something first.');
    expect(M.problem('Ana <ana@example.com>, ben@example.com', 'Thanks')).toBeNull();
    const body = M.replyBody('d', 'Ana', [{ kind: 'p', text: Array.from({ length: 40 }, (_, i) => `line ${i + 1}`).join('\n') }]);
    expect(body.split('\n').filter((l) => l.startsWith('> ')).length).toBe(30);
    expect(M.segment('a/b c')).toBe('a%2Fb%20c'); expect(M.segment('..')).toBe('%2E%2E');
  });

  test('M6 a sparse JSON message', () => {
    const d = M.parseDetail({ message: { id: 'x' } });
    expect(d.summary.id).toBe('x'); expect(d.text).toBe(''); expect(d.to).toEqual([]);
    expect(M.row(d.summary, 0).sender).toBe('(unknown sender)');
    expect(M.parseDetail(null).summary.id).toBe(''); expect(M.parseList('nonsense').rows).toEqual([]);
    const l = M.parseList(inboxList);
    expect(l.rows.length).toBe(4); expect(l.total).toBe(4); expect(l.unread).toBe(2); expect(l.next).toBe('c2_em_01hz07');
  });

  test('plain words for a failure', () => {
    expect(M.words(new SignedOutError()).message).toBe('Sign in to read your mail.');
    expect(M.words(new HTTPError(404, 'not found')).message).toBe('This message is gone.');
    expect(M.words(new HTTPError(400, 'HTTP 400: unsupported folder "junk"')).message).toContain('unsupported folder');
    expect(M.words(new HTTPError(403, '')).message).toBe('This phone is not allowed to do that yet.');
    expect(M.words(new HTTPError(500, '')).message).toBe('Your computer could not do that (500).');
    expect(M.words(new Error('network: offline'))).toEqual({ status: 'offline', message: 'You are offline, or your computer is not answering.' });
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- the sanitiser

describe('M7 the sanitiser', () => {
  test('the hostile table: nothing banned survives, and 62 rows were read', () => {
    expect(hostile.length).toBe(62);
    for (const row of hostile) {
      const out = sanitized(row.input);
      const body = out.slice(out.indexOf('<body>') + 6);
      for (const b of row.banned) expect(body.includes(b), `${row.name}: '${b}' survived in: ${body}`).toBe(false);
    }
  });

  test('what is kept', () => {
    const out = sanitized(`<html><head><title>t</title><style>p{}</style></head><body><h1>Hello</h1><p style="color:red;font-weight:bold">Some <b>bold</b> &amp; <i>fine</i> text, 1 < 2 > 0.</p>
<a href="https://example.com/ok?a=1&amp;b=2">link</a> <a href="mailto:a@example.com">mail</a> <a href="#top">top</a>
<img src="data:image/png;base64,AAAA" alt="inline" width="10"><table><tr><td colspan="2">cell</td></tr></table></body></html>`);
    for (const keep of ['<h1>Hello</h1>', '<b>bold</b>', '<i>fine</i>', 'color:red;font-weight:bold', '&amp;', '(https://example.com/ok?a=1&amp;b=2)', '(mailto:a@example.com)', '(#top)',
      'src="data:image/png;base64,AAAA"', 'alt="inline"', 'colspan="2"', 'cell', '1 &lt; 2 &gt; 0', 'Content-Security-Policy', 'rel="noopener noreferrer nofollow"']) expect(out).toContain(keep);
    expect(out).not.toContain('<title'); expect(out).not.toContain('p{}');
    expect(out).toContain("default-src 'none'");
  });

  test('an unclosed head does not swallow the message; output attributes are escaped; entities decode once', () => {
    expect(sanitized('<html><head><title>x</title><body><p>the message</p></body></html>')).toContain('the message');
    expect(sanitized('<p title=\'a"b<c>&d\'>x</p>')).toContain('title="a&quot;b&lt;c&gt;&amp;d"');
    expect(decodeAttr('&#106;&#x61;&colon;&Tab;&amp;&lt;&bogus;&#;&')).toBe('ja:\t&<&bogus;&#;&');
    expect(decodeAttr('&#0;&#x110000;')).toBe('��');
    expect(clean('<a href="https://example.com/">x</a>')).toBe('<a rel="noopener noreferrer nofollow">x</a> <small>(https://example.com/)</small>');   // inert, with its target written after it
    expect(clean('<a href="javascript:steal()">x</a>')).toBe('<a rel="noopener noreferrer nofollow">x</a>');
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- the mailbox

describe('B1 B2 the mailbox', () => {
  test('B2 the real list: one request, exact, rows by thread, the inbox badge', async () => {
    computer();
    const v = await mail.mailBox('mailbox:inbox', env());
    expect(calls.length).toBe(1);
    expect(calls[0].path).toBe('/api/email/messages'); expect(calls[0].opts.query).toEqual({ folder: 'inbox', limit: 100 }); expect(calls[0].opts.method ?? 'GET').toBe('GET');
    expect(v.status).toBe('ready'); expect(v.total).toBe(4); expect(v.unread).toBe(2);
    expect(v.rows.length).toBe(3);                                      // "Re: Thursday dinner" and "Thursday dinner" are one thread
    expect(v.rows[0].id).toBe('em_01hz0a'); expect(v.rows[0].count).toBe(2); expect(v.rows[0].unread).toBe(true); expect(v.rows[0].stamp).toBe('17:40');
    expect(v.rows[0].address).toBe('mail:em_01hz0a'); expect(v.rows[1].hasAttachments).toBe(true);
    expect(v.rows[2].subject).toBe('(no subject)'); expect(v.rows[2].trust).toBe('quarantined');
    expect(v.folders.map((f) => f.name)).toEqual(['inbox', 'drafts', 'sent', 'quarantine']);
    expect(v.folders.find((f) => f.current)?.name).toBe('inbox'); expect(v.folders[0].badge).toBe('2');
    const f = await mail.mailFrame('mailbox:inbox', env());
    expect(f.status).toBe('cached'); expect(f.rows.length).toBe(3); expect(f.folders[0].badge).toBe('2');
    expect(calls.length).toBe(1);
  });

  test('B1 empty, a server error, a refused key, a page that is not a mailbox', async () => {
    route = () => ({ status: 200, body: { messages: [], total: 0, unread: 0 } });
    let v = await mail.mailBox('mailbox:sent', env());
    expect(v.status).toBe('empty'); expect(v.message).toBe('Nothing sent yet.');
    route = () => ({ status: 400, body: { error: 'unsupported folder "junk"' } });
    v = await mail.mailBox('mailbox:junk', env());
    expect(v.status).toBe('error'); expect(v.message).toContain('unsupported folder'); expect(v.title).toBe('Mail');
    route = () => ({ status: 401, body: {} });
    v = await mail.mailBox('mailbox:inbox', env());
    expect(v.message).toBe('Sign in again to read your mail.');
    expect((await mail.mailBox('mail:abc', env())).status).toBe('idle');
    route = () => ({ status: 200, body: 'not an object' });
    expect((await mail.mailBox('mailbox:inbox', env())).status).toBe('empty');
  });

  test('B1 offline keeps the last good rows under the banner, and a cold offline start shows the phone\'s copy', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env());
    route = () => 'network';
    let v = await mail.mailBox('mailbox:inbox', env());
    expect(v.status).toBe('offline'); expect(v.cached).toBe(true); expect(v.rows.length).toBe(3);
    mail.forget();                                                                    // the app was killed and opened again, still offline
    v = await mail.mailBox('mailbox:inbox', env());
    expect(v.status).toBe('offline'); expect(v.cached).toBe(true); expect(v.rows.length).toBe(3);
    expect(v.message).toContain('saved on this phone');
    const f = await mail.mailFrame('mailbox:inbox', env());
    expect(f.status).toBe('cached'); expect(f.rows.length).toBe(3);
    // with no copy at all (a fresh phone, or the copy wiped) it says so and shows no rows
    await mail.wipeMail(storage); mail.forget();
    v = await mail.mailBox('mailbox:inbox', env());
    expect(v.status).toBe('offline'); expect(v.rows).toEqual([]); expect(v.cached).toBe(false);
  });

  test('drafts are listed from the computer and not copied', async () => {
    route = (r) => (r.path === '/api/email/drafts' ? { status: 200, body: { drafts: [{ id: 'd9', status: 'draft_pending_owner_approval', to_addresses: ['ben@example.com'], subject: 'Plans', text_body: 'Hello Ben' }] } } : { status: 404, body: {} });
    const v = await mail.mailBox('mailbox:drafts', env());
    expect(v.rows[0].address).toBe('mail:draft/d9'); expect(v.rows[0].sender).toBe('To ben@example.com');
    expect([...files.keys()].length).toBe(0);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- a message

describe('G1 G2 a message', () => {
  test('G1 the view, the thread, mark-read once, and "unread" is not undone by the next answer', async () => {
    computer();
    const v = await mail.mailMessage('mail:em_01hz0a', false, env());
    expect(v.status).toBe('ready'); expect(v.subject).toBe('Re: Thursday dinner'); expect(v.sender).toBe('Maya Ortiz'); expect(v.fromAddress).toBe('maya.ortiz@example.com');
    expect(v.toLine).toBe('Listener <listener@example.com>'); expect(v.ccLine).toBe('friend@example.com'); expect(v.date).toBe('Wed 18 Nov 2026, 17:40');
    expect(v.replyLabel).toBe('Reply to Maya Ortiz'); expect(v.canReply).toBe(true); expect(v.unread).toBe(true);
    expect(v.blocks.map((b) => b.kind)).toEqual(['p', 'p', 'quote', 'sig']); expect(v.hasQuoted).toBe(true);
    expect(v.attachments[0]).toMatchObject({ filename: 'menu.pdf', size: '471 KB', kind: 'pdf', available: false, address: '' });
    expect(v.attachments[0].note).toBe('Saving attachments from mail is not offered yet.');
    expect(v.thread.map((t) => t.id)).toEqual(['em_01hz09', 'em_01hz0a']); expect(v.thread[1].current).toBe(true);
    expect(v.spoken.startsWith('Message from Maya Ortiz. Subject: Re: Thursday dinner.')).toBe(true);
    expect(calls.filter((c) => c.path.endsWith('/read')).length).toBe(1);
    expect(calls.find((c) => c.path.endsWith('/read'))!.opts.method).toBe('POST'); expect(calls.find((c) => c.path.endsWith('/read'))!.path).toBe('/api/email/messages/em_01hz0a/read');
    const u = await mail.mailMarkUnread('mail:em_01hz0a');
    expect(u).toEqual({ status: 'ok', message: '' });
    await mail.mailMessage('mail:em_01hz0a', false, env());
    expect(calls.filter((c) => c.path.endsWith('/read')).length).toBe(1);          // not marked read a second time
    expect(calls.filter((c) => c.path.endsWith('/unread')).length).toBe(1);
  });

  test('G1 404 is "gone", new, reply and draft addresses are not messages, an outbound message can be replied to only if it has recipients', async () => {
    route = () => ({ status: 404, body: { error: 'not found' } });
    const gone = await mail.mailMessage('mail:zzz', false, env());
    expect(gone.status).toBe('error'); expect(gone.message).toBe('This message is gone.');
    for (const a of ['mail:new', 'mail:reply/x', 'mail:draft/x', 'mailbox:inbox']) expect((await mail.mailMessage(a, false, env())).status).toBe('idle');
    computer();
    const out = await mail.mailMessage('mail:em_01hz09', false, env());
    expect(out.canReply).toBe(true); expect(out.unread).toBe(false);
    const q = await mail.mailMessage('mail:em_01hz07', false, env());
    expect(q.canReply).toBe(false); expect(q.trustNote).toContain('Held back');
  });

  test('G1 offline, a message the phone holds opens from the copy and says so', async () => {
    computer();
    await mail.mailMessage('mail:em_01hz0a', false, env());
    route = () => 'network';
    mail.forget();
    const v = await mail.mailMessage('mail:em_01hz0a', false, env());
    expect(v.status).toBe('ready'); expect(v.message).toContain('saved on this phone'); expect(v.subject).toBe('Re: Thursday dinner'); expect(v.blocks.length).toBe(4);
    const none = await mail.mailMessage('mail:em_01hz0b', false, env());
    expect(none.status).toBe('offline');
  });

  test('G2 "Show original" is written only when asked, cleaned, and the old documents go', async () => {
    computer();
    files.set('app:/cache/mail-0123old.html', new TextEncoder().encode('<p>old</p>'));
    files.set('app:/cache/other.html', new TextEncoder().encode('<p>not ours</p>'));
    let v = await mail.mailMessage('mail:em_01hz08', false, env());
    expect(v.hasOriginal).toBe(true); expect(v.htmlOnly).toBe(true); expect(v.blocks[0].text).toBe('Receipt');
    expect([...files.keys()].filter((k) => k.startsWith('app:/cache/mail-'))).toEqual([]);      // nothing written, and the old one removed
    expect(files.has('app:/cache/other.html')).toBe(true);
    v = await mail.mailMessage('mail:em_01hz08', true, env());
    expect(v.originalPath).toMatch(/^app:\/cache\/mail-[0-9a-f]{32}\.html$/);
    const doc = new TextDecoder().decode(files.get(v.originalPath)!);
    expect(doc).toContain('Content-Security-Policy'); expect(doc).toContain('Your order');
    for (const bad of ['<script', 'steal(', 'tracker.example.com', 'javascript:', 'onload', 'src=']) expect(doc).not.toContain(bad);
    expect(doc).not.toContain('href='); expect(doc).toContain('(https://shop.example.com/orders/1)');
    v = await mail.mailMessage('mail:em_01hz0a', true, env());
    expect(v.hasOriginal).toBe(false); expect(v.originalPath).toBe('');
    expect([...files.keys()].filter((k) => k.startsWith('app:/cache/mail-'))).toEqual([]);
    // the phone's copy keeps no HTML
    for (const [k, b] of files) if (k.startsWith('app:/data/mail/')) expect(new TextDecoder().decode(b)).not.toContain('Receipt');
    expect(calls.filter((c) => c.path.includes('tracker')).length).toBe(0);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- compose

describe('C1 compose', () => {
  test('C1 reply addressing, from Reply-To and from our own sent mail', async () => {
    computer();
    let c = await mail.mailCompose('mail:reply/em_01hz0a', env());
    expect(c.status).toBe('seed'); expect(c.toLine).toBe('maya.replies@example.com'); expect(c.subject).toBe('Re: Thursday dinner'); expect(c.replyTo).toBe('em_01hz0a');
    expect(c.body).toContain('> Thursday works for me.'); expect(c.body).toContain('wrote:');
    expect(calls.filter((x) => x.path === '/api/email/messages/em_01hz0a').length).toBe(1);   // one request for the message, not two
    c = await mail.mailCompose('mail:reply/em_01hz09', env());
    expect(c.toLine).toBe('maya.ortiz@example.com'); expect(c.subject).toBe('Re: Thursday dinner');
    expect((await mail.mailCompose('mail:new', env())).status).toBe('seed');
    expect((await mail.mailCompose('mailbox:inbox', env())).status).toBe('idle');
    route = () => ({ status: 404, body: {} });
    c = await mail.mailCompose('mail:reply/zzz', env());
    expect(c.status).toBe('error'); expect(c.note).toBe('This message is gone.');
    route = (r) => (r.path === '/api/email/drafts/d1' ? { status: 200, body: { id: 'd1', status: 'draft', from_address: 'listener@example.com', to_addresses: ['ben@example.com'], subject: 'S', text_body: 'B' } } : { status: 404, body: {} });
    c = await mail.mailCompose('mail:draft/d1', env());
    expect(c).toMatchObject({ status: 'saved', draftId: 'd1', toLine: 'ben@example.com', note: 'Saved. Sending needs your approval.' });
  });

  test('C1 a refused draft makes no request; the request is exact; the approval link; a 403; no alias; send is never called', async () => {
    computer();
    const refused = await mail.mailSaveDraft('mail:new', '', 's', 'b');
    expect(refused.status).toBe('error'); expect(calls.length).toBe(0);
    const d = await mail.mailSaveDraft('mail:reply/em_01hz0a', 'Ana <ana@example.com>', 'Re: Hello', 'Yes');
    expect(d).toMatchObject({ status: 'saved', draftId: 'd1', sendAs: 'listener@example.com', replyTo: 'em_01hz0a' });
    const post = calls.find((c) => c.opts.method === 'POST' && c.path === '/api/email/drafts')!;
    expect(post.opts.body).toEqual({ from_address: 'listener@example.com', subject: 'Re: Hello', text_body: 'Yes', to_addresses: ['ana@example.com'], reply_to_message_id: 'em_01hz0a', source_kind: 'mail', source_ref: 'em_01hz0a' });
    const a = await mail.mailApprovalEmail('mail:reply/em_01hz0a', 'd1');
    expect(a).toMatchObject({ status: 'approval', reviewURL: 'https://choir.news/mail/review/x' });
    expect(calls.some((c) => c.path.endsWith('/send'))).toBe(false);
    route = () => ({ status: 403, body: { error: 'owner session required' } });
    const denied = await mail.mailApprovalEmail('mail:reply/em_01hz0a', 'd1');
    expect(denied.status).toBe('saved'); expect(denied.note).toContain('cannot ask');
    mail.forget();
    route = (r) => (r.path === '/api/email/aliases' ? { status: 200, body: { aliases: [] } } : { status: 404, body: {} });
    const noAlias = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b');
    expect(noAlias.status).toBe('error'); expect(noAlias.note).toContain('no sending address');
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- the phone's copy

describe('P1 to P3 the phone\'s copy', () => {
  test('P1 plain JSON files under app:/data/mail, written by shared code with no native call, and they read back', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env());
    await mail.mailMessage('mail:em_01hz0a', false, env());
    const names = [...files.keys()];
    expect(names.length).toBeGreaterThanOrEqual(2);
    expect(names.every((n) => n.startsWith('app:/data/mail/') && n.endsWith('.json'))).toBe(true);
    expect(JSON.parse(new TextDecoder().decode(files.get((await accessor()).pathOfRows('inbox'))!)).folder).toBe('inbox');
    const c = await C.loadFolder(env(), 'inbox');
    expect(c!.rows.length).toBe(4);
    expect((await C.loadMessage(env(), 'em_01hz0a'))!.text).toContain('Thursday works for me');
  });

  test('P1 no storage means no file, and the mailbox still works', async () => {
    computer();
    const v = await mail.mailBox('mailbox:inbox', {});
    await mail.mailMessage('mail:em_01hz0a', false, {});
    expect(v.status).toBe('ready'); expect(files.size).toBe(0);
    expect(await C.loadFolder({}, 'inbox')).toBeNull();
  });

  test('P1 a file that does not parse is ignored and removed; past the limit the oldest go', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env());
    const k = (await accessor()).pathOfRows('inbox');
    files.set(k, new TextEncoder().encode('{"folder":"inbox","rows":[')); //a half write
    expect(await C.loadFolder(env(), 'inbox')).toBeNull(); expect(files.has(k)).toBe(false);
    for (let i = 0; i < 5; i++) { const f = await (await accessor()).pathOfMessage('m' + i); await C.saveMessage(env(), { id: 'm' + i, folder: 'inbox', summary: sm('m' + i, 's', '2026-11-1' + i + 'T00:00:00Z'), text: 't', to: [], cc: [], replyTo: '', attachments: [], savedAt: 0 }); mtimes.set(f, i); }
    expect(await C.trim(env(), 3)).toBe(2);
    expect(await C.holds(env(), 'm0')).toBe(false); expect(await C.holds(env(), 'm1')).toBe(false); expect(await C.holds(env(), 'm4')).toBe(true);
  });

  test('P2 the catch-up fetches the bodies not held, once, resumes at its cursor, stops at the budget, and says counts only', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env());
    calls = [];
    const first = await mail.mailSync('mailbox:inbox', env());
    expect(first.status).toBe('done'); expect(first.fetched).toBe(5); expect(first.held).toBe(5);       // 4 on page one, 1 on page two
    expect(first.note).toBe('5 messages saved on this phone');
    const bodies = calls.filter((c) => /\/api\/email\/messages\/[^/]+$/.test(c.path));
    expect(bodies.length).toBe(5); expect(calls.find((c) => c.opts.query?.cursor)!.opts.query).toEqual({ folder: 'inbox', limit: 100, cursor: 'c2_em_01hz07' });
    calls = [];
    const again = await mail.mailSync('mailbox:inbox', env());
    expect(again.fetched).toBe(0); expect(calls.length).toBe(0);                                         // the folder was walked: nothing asked
    // a new message on page one is fetched, nothing else
    const fresh = { ...inboxList, messages: [{ ...inboxList.messages[0], id: 'em_new1', subject: 'Fresh' }, ...inboxList.messages] };
    computer((r) => (r.path === '/api/email/messages' && !r.opts.query?.cursor ? { status: 200, body: fresh } : r.path === '/api/email/messages/em_new1' ? { status: 200, body: { message: fresh.messages[0], text_body: 'new' } } : undefined));
    mail.forget(); await mail.mailBox('mailbox:inbox', env()); calls = [];
    const third = await mail.mailSync('mailbox:inbox', env());
    expect(third.fetched).toBe(1); expect(calls.filter((c) => /messages\/[^/]+$/.test(c.path)).map((c) => c.path)).toEqual(['/api/email/messages/em_new1']);
  });

  test('P2 a long folder is walked a budget at a time: the answer says "more", the next one goes on, and every body is fetched once', async () => {
    files.clear(); mail.forget();
    const many = { messages: Array.from({ length: 30 }, (_, i) => ({ id: `em_${String(i).padStart(3, '0')}`, direction: 'inbound', from_address: 'lee.park@example.com', subject: `Subject ${i}`, trust_status: 'trusted', read_at: 'x', created_at: '2026-11-1' + (i % 9) + 'T00:00:00Z', received_at: '2026-11-1' + (i % 9) + 'T00:00:00Z' })), total: 30, unread: 0 };
    route = (r) => (r.path === '/api/email/messages' ? { status: 200, body: many } : { status: 200, body: { message: many.messages.find((m) => r.path.endsWith(m.id)), text_body: 'body ' + r.path } });
    await mail.mailBox('mailbox:inbox', env());
    calls = [];
    const one = await mail.mailSync('mailbox:inbox', env());
    expect(one.status).toBe('more'); expect(one.fetched).toBe(mail.SYNC_BODIES); expect(one.held).toBe(25);
    const two = await mail.mailSync('mailbox:inbox', env());
    expect(two.status).toBe('done'); expect(two.fetched).toBe(5); expect(two.held).toBe(30);
    const asked = calls.filter((c) => /messages\/[^/]+$/.test(c.path)).map((c) => c.path);
    expect(asked.length).toBe(30); expect(new Set(asked).size).toBe(30);
  });

  test('P3 there is no search over the copy and no mail document source: mail text is in no index', async () => {
    expect((C as Record<string, unknown>).search).toBeUndefined(); expect((C as Record<string, unknown>).heldMessages).toBeUndefined();
    const search = await Bun.file(new URL('./search.ts', import.meta.url)).text(); expect(search).not.toContain('export const mailDocs');
    const content = await Bun.file(new URL('./content.ts', import.meta.url)).text(); expect(content.split('\n').filter((l) => !l.trimStart().startsWith('//')).join('\n')).not.toMatch(/\/api\/email|syncMail|mailItem/);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- the old copy, the wipe, logs, grants

describe('W1 L1 D1 A1', () => {
  test('W1 the old documents in the cache are deleted at first use, and nothing else of the cache is', async () => {
    computer();
    files.set('app:/cache/mail-old1.html', new TextEncoder().encode('<p>an old message</p>'));
    files.set('app:/cache/mail-old2.html', new TextEncoder().encode('<p>another</p>'));
    files.set('app:/cache/files/f1.txt', new TextEncoder().encode('a file'));
    files.set('app:/cache/story.json', new TextEncoder().encode('{}'));
    await mail.mailMessage('mail:em_01hz0a', false, env());
    expect([...files.keys()].filter((k) => k.startsWith('app:/cache/mail-'))).toEqual([]);
    expect(files.has('app:/cache/story.json')).toBe(true); expect(files.has('app:/cache/files/f1.txt')).toBe(true);
    expect(await C.forgetOldCopy(storage)).toBe(0);
  });

  test('W1 wipeMail removes every file, every original and the memory; it never throws, even with nothing', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env()); await mail.mailMessage('mail:em_01hz08', true, env()); await mail.mailSync('mailbox:inbox', env());
    expect([...files.keys()].some((k) => k.startsWith('app:/data/mail/'))).toBe(true);
    files.set('app:/data/paper-front.json', new TextEncoder().encode('{}'));
    const r = await mail.wipeMail(storage);
    expect(r.files).toBeGreaterThan(0); expect(r.originals).toBe(1);
    expect([...files.keys()]).toEqual(['app:/data/paper-front.json']);
    route = () => 'network';
    expect((await mail.mailFrame('mailbox:inbox', env())).status).toBe('frame');                // memory is gone too
    expect(await mail.wipeMail(storage)).toEqual({ files: 0, originals: 0 });
    expect(await mail.wipeMail(undefined)).toEqual({ files: 0, originals: 0 });
  });

  test('L1 logs carry ids and counts, never a sender, a subject, an address or a word of a message', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env()); await mail.mailMessage('mail:em_01hz0a', false, env()); await mail.mailSync('mailbox:inbox', env());
    await mail.mailSaveDraft('mail:new', 'maya.ortiz@example.com', 'Thursday dinner', 'Seven at the usual place');
    route = () => 'network'; await mail.mailBox('mailbox:inbox', env()); await mail.wipeMail(storage);
    const all = logs.join('\n');
    restore();
    expect(all.length).toBeGreaterThan(0);
    for (const w of [...SECRETS, 'example.com', 'Seven', 'listener']) expect(all.includes(w), `a log holds "${w}": ${all}`).toBe(false);
  });

  test('A1 the grants name the paths and the secret the modules use', () => {
    const g = app.grants.split('\n');
    expect(g).toContain(`fs.read ${C.DIR}`); expect(g).toContain(`fs.write ${C.DIR}`);
    expect(g).toContain(`fs.read ${C.CACHE}`); expect(g).toContain(`fs.write ${C.CACHE}`);
    expect(g.some((l) => l.startsWith('secret.'))).toBe(false);          // mail needs no secret: the files are plain
    expect(g.some((l) => l.startsWith('net.fetch'))).toBe(true);
  });
});

// ---------------------------------------------------------------------------------------------------------------------------------------------- review 2026-11-19 (mail, seat 1)
// Failure list, written before the code:
//  R1 a hostile body (many unclosed `<`, `<!--`, `<script `, one long run of spaces) costs the same at 4 MB as at 1 MB: the work is capped before any scan and every scan is linear.
//  R2 "Show original" of a body over the cap says the message is too large to show in full (never a blank page); the readable text is still on the page, cut and said so.
//  R3 a mail write suspended across a sign-out leaves no file; writes stay off until a new session is established.
//  R4 a file that does not parse is removed; a file swapped to another record is not used.
//  R6 a draft too long is refused with no request; the same text saved twice is one draft.
//  R7 no link in "Show original" can be followed (its target is written after it); the unsent compose text is cleared by the sign-out action.
describe('R1 R2 hostile bodies', () => {
  const shapes: Record<string, (n: number) => string> = {
    lt: (n) => '<'.repeat(n), comment: (n) => '<!--'.repeat(n / 4), script: (n) => '<script '.repeat(n / 8), spaces: (n) => ' '.repeat(n) + 'x', tags: (n) => '<b'.repeat(n / 2),
    ltgt: (n) => '< >'.repeat(n / 3), unclosedStyle: (n) => '<style>'.repeat(n / 7) + 'text',
  };
  const time = (f: () => unknown): number => { const t = performance.now(); f(); return performance.now() - t; };
  test('R1 htmlToText and blocks: 5 MB of each shape finishes, and 4 MB costs no more than a small multiple of 1 MB', () => {
    for (const [name, make] of Object.entries(shapes)) {
      const small = Math.max(time(() => M.blocks('', make(1_000_000))), 1);
      const big = time(() => M.blocks('', make(5_000_000)));
      expect(big).toBeLessThan(1500);
      expect(big).toBeLessThan(small * 6 + 50);                                         // capped input: constant, not growing with the body (quadratic is 25x)
      expect(name.length).toBeGreaterThan(0);
    }
    expect(M.blocks('', '<p>'.repeat(200_000)).cut).toBe(true);
  });
  test('R1 at the cap itself: every shape of exactly HTML_MAX characters is read within a fixed bound', () => {
    for (const make of Object.values(shapes)) expect(time(() => M.htmlToText(make(M.HTML_MAX)))).toBeLessThan(400);
  });
  test('R1 the scans themselves are linear: htmlToText of an uncapped-looking shape is not quadratic (4x the size, not 16x the time)', () => {
    for (const make of Object.values(shapes)) {
      const a = Math.max(time(() => M.htmlToText(make(250_000))), 1), b = time(() => M.htmlToText(make(1_000_000)));
      expect(b).toBeLessThan(a * 12 + 80);
    }
    const lines = ' '.repeat(190_000) + 'x';
    expect(time(() => M.blocks(lines + '\n\n' + lines, ''))).toBeLessThan(500);          // trimH on a long run of spaces
  });
  test('R1 the text of ordinary html is as before', () => {
    expect(M.htmlToText('<p>One</p><p>Two &amp; <b>three</b></p><!-- c --><script>x</script><ul><li>a</li></ul>a<br/>b')).toBe('One\n\nTwo & three\n\n• a\n\na\nb');
  });
  test('R2 an original over the cap says so, and the message page says it was cut', async () => {
    const big = '<p>' + 'word '.repeat(200_000) + '</p>';
    const doc = sanitized(big);
    expect(doc).toContain('too large to show in full'); expect(doc).not.toContain('word word');
    expect(time(() => sanitized('<'.repeat(5_000_000)))).toBeLessThan(200);
    computer((r) => (r.path === '/api/email/messages/em_big' ? { status: 200, body: { message: inboxList.messages[0], text_body: '', html_body: big, recipients: {} } } : undefined));
    const v = await mail.mailMessage('mail:em_big', true, env());
    expect(v.cut).toBe(true); expect(v.hasOriginal).toBe(true);
    expect(new TextDecoder().decode(files.get(v.originalPath)!)).toContain('too large to show in full');
  });
});

describe('R3 a sign-out while mail is being written', () => {
  test('a write suspended in the disk across the sign-out leaves no file', async () => {
    computer();
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    const pending = mail.mailMessage('mail:em_01hz0a', false, env());
    for (let i = 0; i < 100 && writeStarted === 0; i++) await new Promise((r) => setTimeout(r, 5));
    expect(writeStarted).toBeGreaterThan(0);                               // the message was fetched and its file is being written
    await beginSignOut();
    release();
    await pending.catch(() => undefined);
    await new Promise((r) => setTimeout(r, 20));
    expect([...files.keys()].filter((k) => k.startsWith('app:/data/mail/'))).toEqual([]);
  });
  test('writing is off from the start of a sign-out until another account signs in, and on again when the sign-out is given up', async () => {
    const api = await import('./api.ts');
    expect(await C.stores(env())).toBe(true);
    await beginSignOut();
    expect(api.signOutBlocked()).toBe(true); expect(await C.stores(env())).toBe(false);
    api.abortSignOut();
    expect(api.signOutBlocked()).toBe(false); expect(await C.stores(env())).toBe(true);
  });
  test('a store write that began under an older generation refuses and leaves no file', async () => {
    const api = await import('./api.ts');
    const old: C.Env = { storage, gen: api.accountGeneration() };
    api.nextGeneration();
    expect(await C.saveState(old, { done: {}, syncedAt: {}, cursor: {} })).toBe(false);
    expect(files.size).toBe(0);
  });
});

describe('R4 what is removed and what is kept', () => {
  test('a file that does not parse goes; another record\'s file is not used', async () => {
    computer();
    await mail.mailMessage('mail:em_01hz0a', false, env());
    const file = await (await accessor()).pathOfMessage('em_01hz0a');
    expect(files.has(file)).toBe(true);
    expect((await C.loadMessage(env(), 'em_01hz0a'))?.id).toBe('em_01hz0a');
    const other = await (await accessor()).pathOfMessage('em_other');
    files.set(other, files.get(file)!);                                                                          // a file swapped to another message's name
    expect(await C.loadMessage(env(), 'em_other')).toBeNull();
    files.set(file, new TextEncoder().encode('MS3' + 'x'.repeat(60)));                                          // the earlier sealed format
    expect(await C.loadMessage(env(), 'em_01hz0a')).toBeNull(); expect(files.has(file)).toBe(false);
    await mail.mailBox('mailbox:inbox', env());
    const rows = (await accessor()).pathOfRows('inbox'); const sent = (await accessor()).pathOfRows('sent');
    files.set(sent, files.get(rows)!);                                                                           // the inbox's rows under the sent folder's name
    expect(await C.loadFolder(env(), 'sent')).toBeNull();
  });
});

describe('R5 R6 R7', () => {
  test('R5 a stored text is cut at the page\'s own cap', async () => {
    computer();
    const env1 = env();
    await C.saveMessage({ ...env1, gen: undefined }, { id: 'em_x', folder: 'inbox', summary: { id: 'em_x' } as never, text: 'a'.repeat(500_000), to: [], cc: [], replyTo: '', attachments: [], savedAt: 0 });
    expect((await C.loadMessage(env1, 'em_x'))!.text.length).toBe(200_000);
  });
  test('R6 a draft too long is refused with no request, and the same text is not saved twice', async () => {
    computer((r) => (r.path === '/api/email/drafts' && r.opts.method === 'POST' ? { status: 200, body: { id: 'd1', to_addresses: ['a@example.com'], subject: 's', text_body: 'b' } } : undefined));
    const long = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'x'.repeat(mail.MAX_DRAFT_BODY + 1));
    expect(long.status).toBe('error'); expect(calls.some((c) => c.path === '/api/email/drafts')).toBe(false);
    const a = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b');
    const b = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b');
    expect(a.draftId).toBe('d1'); expect(b.draftId).toBe('d1'); expect(calls.filter((c) => c.path === '/api/email/drafts' && c.opts.method === 'POST').length).toBe(1);
    const c2 = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b2'); expect(c2.status).toBe('saved');
    expect(calls.filter((c) => c.path === '/api/email/drafts' && c.opts.method === 'POST').length).toBe(2);
  });
  test('R7 no link in the original can be followed: no href, the target is written after it', () => {
    const out = sanitized('<a href="https://evil.example/x?a=1">Your invoice</a>');
    expect(out).not.toContain('href'); expect(out).toContain('Your invoice</a> <small>(https://evil.example/x?a=1)</small>');
  });
  // (The sign-out action's reset of mailTo/mailSubject/mailBody/mailFor in account.contract is UNTESTED: the contract test harness has no mail page. The proof tour must open mail:new after a sign-out.)
});

describe('R8 restore, budget, words', () => {
  test('the copy is held to a byte budget as well as a file count: the oldest go first', async () => {
    const e = env();
    const t = `.${(await accessor()).owner}.json`;
    for (let i = 0; i < 4; i++) { files.set(`${C.DIR}/msg-${i}${t}`, new Uint8Array(1000)); mtimes.set(`${C.DIR}/msg-${i}${t}`, i); }
    expect(await C.trim(e, 100, 2500)).toBe(2); expect([...files.keys()].sort()).toEqual([`${C.DIR}/msg-2${t}`, `${C.DIR}/msg-3${t}`]);
  });
  test('the words for 401, 403, 404, 413 and 429 are plain and differ', () => {
    const st = (status: number) => M.words(Object.assign(new Error('x'), { status, name: 'HTTPError' })).message;
    const set = new Set([401, 403, 404, 413, 429].map(st));
    expect(set.size).toBe(5); expect(st(413)).toContain('too large');
  });
});

describe('R9 the last races and words', () => {
  test('a file write in flight when the sign-out begins is removed (the check after the write)', async () => {
    computer();
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    const pending = mail.mailMessage('mail:em_01hz0a', false, env());
    for (let i = 0; i < 40 && writeStarted === 0; i++) await new Promise((r) => setTimeout(r, 5));
    await new Promise((r) => setTimeout(r, 20));
    await beginSignOut();
    release(); await pending.catch(() => undefined); await new Promise((r) => setTimeout(r, 20));
    expect([...files.keys()].filter((k) => k.startsWith('app:/data/mail/'))).toEqual([]);
  });
  test('"Show original" in flight at a sign-out leaves no plain file and no path', async () => {
    computer();
    let release: () => void = () => undefined; readdirGate = new Promise<void>((r) => { release = r; });
    const pending = mail.mailMessage('mail:em_01hz08', true, env());
    await new Promise((r) => setTimeout(r, 30));
    await beginSignOut();
    release(); const v = await pending;
    expect(v.originalPath).toBe(''); expect([...files.keys()].filter((k) => k.startsWith('app:/cache/mail-'))).toEqual([]);
  });
  test('the next session\'s first sync is not answered idle: a different key clears the block at once', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env());
    await beginSignOut();
    expect(await C.stores(env())).toBe(false);
    const api = await import('./api.ts');
    api.nextGeneration();
    bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'session' ? { found: true, value: 'another-key-0002-not-real', computerId: 'comp-2' } : { ok: true }) } as never);
    expect(await C.stores({ storage })).toBe(true);
  });
  test('a timeout has its own words, not "offline"', () => {
    const w = M.words(new Error('timeout: The request timed out'));
    expect(w.message).toContain('too long'); expect(w.message).not.toMatch(/offline/i);
    expect(M.words(new Error('network: The Internet connection appears to be offline')).status).toBe('offline');
  });
});

describe('R10 one account\'s mail is never the next account\'s', () => {
  const otherAccount = (): void => bindNative({ available: true, later: async (r: { op?: string; account?: string }) => (r.op === 'session' ? { found: true, value: 'another-key-0002-not-real', computerId: 'comp-2' } : r.op === 'keystate' ? { state: 'present' } : { found: true, value: 'another-key-0002-not-real' }) } as never);
  test('a file written for one key is not served for another: it is removed, and the other account sees nothing', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env()); await mail.mailMessage('mail:em_01hz0a', false, env());
    expect((await C.loadMessage(env(), 'em_01hz0a'))?.id).toBe('em_01hz0a');
    const api = await import('./api.ts'); api.nextGeneration(); mail.forget();
    otherAccount();
    expect(await C.loadMessage(env(), 'em_01hz0a')).toBeNull(); expect(await C.loadFolder(env(), 'inbox')).toBeNull(); expect(await C.loadState(env())).toEqual({ done: {}, syncedAt: {}, cursor: {} });
    expect([...files.keys()].filter((k) => k.startsWith('app:/data/mail/'))).toEqual([]);          // removed on the refused read
  });
  test('a write refuses while no account is signed in', async () => {
    bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'keystate' ? { state: 'absent' } : { found: false }) } as never);
    expect(await C.saveState(env(), { done: {}, syncedAt: {}, cursor: {} })).toBe(false); expect(files.size).toBe(0);
  });
  test('a write that began before the sign-out leaves no file and no empty folder', async () => {
    computer();
    let release: () => void = () => undefined; writeGate = new Promise<void>((r) => { release = r; });
    const pending = mail.mailMessage('mail:em_01hz0a', false, env());
    for (let i = 0; i < 100 && writeStarted === 0; i++) await new Promise((r) => setTimeout(r, 5));
    await beginSignOut(); release(); await pending.catch(() => undefined); await new Promise((r) => setTimeout(r, 20));
    expect([...files.keys()].filter((k) => k.startsWith('app:/data/mail/'))).toEqual([]); expect(dirs.has(C.DIR)).toBe(false);
  });
  test('a draft saved across a sign-out is not the next account\'s: its id is not reused and its sending address is not kept', async () => {
    computer((r) => (r.path === '/api/email/drafts' && r.opts.method === 'POST' ? { status: 201, body: { id: `d${calls.filter((c) => c.path === '/api/email/drafts').length}`, to_addresses: ['a@example.com'] } } : undefined));
    let release: () => void = () => undefined; fetchGate = new Promise<void>((r) => { release = r; });
    const a = mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b');
    await new Promise((r) => setTimeout(r, 20));
    const api = await import('./api.ts'); await beginSignOut(); api.nextGeneration(); mail.forget(); otherAccount();
    release(); await a; fetchGate = null;
    const b = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b');
    expect(calls.filter((c) => c.path === '/api/email/drafts' && c.opts.method === 'POST').length).toBe(2);        // not "already saved"
    expect(b.draftId).toBe('d2');
  });
});

describe('R11 the identity is the key, read fresh: one account\'s mail is not the next one\'s, however the key changed', () => {
  const swapKey = (): void => bindNative({ available: true, later: async (r: { op?: string; account?: string }) => (r.op === 'session' ? { found: true, value: 'another-key-0002-not-real', computerId: 'comp-2' } : r.op === 'keystate' ? { state: 'present' } : { found: true, value: 'another-key-0002-not-real' }) } as never);
  test('R11a a key replaced WITHOUT a sign-out (a re-sign-in after a refusal): no file, no row in memory, no draft id, no sending address of the first key', async () => {
    computer((r) => (r.path === '/api/email/drafts' && r.opts.method === 'POST' ? { status: 201, body: { id: `d${calls.filter((c) => c.path === '/api/email/drafts').length}`, to_addresses: ['a@example.com'] } } : undefined));
    await mail.mailBox('mailbox:inbox', env()); await mail.mailMessage('mail:em_01hz0a', false, env());
    const first = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b'); expect(first.draftId).toBe('d1');
    swapKey();                                                                       // no nextGeneration, no forget: the generation did not move
    expect((await C.loadMessage(env(), 'em_01hz0a'))).toBeNull(); expect(await C.loadFolder(env(), 'inbox')).toBeNull();
    route = () => 'network';
    const frame = await mail.mailFrame('mailbox:inbox', env());
    expect(frame.rows.length).toBe(0);                                              // the first account's rows are not in the second one's memory
    route = (r) => (r.path === '/api/email/aliases' ? { status: 200, body: { aliases: [{ id: 'al2', address: 'second@example.com' }] } } : r.path === '/api/email/drafts' ? { status: 201, body: { id: 'd2', to_addresses: ['a@example.com'] } } : { status: 404, body: {} });
    const second = await mail.mailSaveDraft('mail:new', 'a@example.com', 's', 'b');
    expect(second.draftId).toBe('d2'); expect(second.sendAs).toBe('second@example.com');
  });
  test('R11b holds and the held count see only this account\'s files: a file of an earlier format or another key is not "held", is fetched again, and is removed', async () => {
    computer();
    await mail.mailBox('mailbox:inbox', env());
    const F = await accessor();
    const legacy = `${C.DIR}/msg-${hex16('em_01hz0a')}.json`;                    // no owner in the name: an earlier build, or a failed wipe
    files.set(legacy, new TextEncoder().encode(JSON.stringify({ id: 'em_01hz0a', folder: 'inbox', text: 'old', to: [], cc: [], replyTo: '', attachments: [], savedAt: 0, summary: {} })));
    files.set(`${C.DIR}/msg-aaaa.0123456789abcdef01234567.json`, new TextEncoder().encode('{}'));
    expect(await F.holds('em_01hz0a')).toBe(false);
    const s = await mail.mailSync('mailbox:inbox', env());
    expect(s.fetched).toBe(5); expect(s.held).toBe(5);                              // not 7: the foreign files are neither held nor counted
    expect(files.has(legacy)).toBe(false);
    expect([...files.keys()].every((k) => k.endsWith(`.${F.owner}.json`))).toBe(true);
  });
  test('R11c nothing is read or written for a key that cannot be read: no one signed in', async () => {
    computer(); await mail.mailBox('mailbox:inbox', env());
    bindNative({ available: true, later: async (r: { op?: string }) => (r.op === 'keystate' ? { state: 'absent' } : { found: false }) } as never);
    expect(await C.loadFolder(env(), 'inbox')).toBeNull(); expect(await C.holds(env(), 'x')).toBe(false); expect(await C.Files.open(env())).toBeNull();
  });
});
