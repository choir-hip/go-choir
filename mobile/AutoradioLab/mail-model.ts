// Mail's pure data: what the listener's computer sends (the maild JSON), the readable text made from a message, threading, dates and the
// plain words for a failure. No request, no storage, no clock (the server's `Date` header is "now", handed in). It replaces the pure half of the
// Swift ChoirMail.swift (MailText, MailDates, MailParse, MailThreads, MailService.words and .problem); mail.ts is the source that uses it.
//
// Ways this can fail (the Swift list, kept; each has a test in mail.test.ts):
//  1. A folder the server does not know (400 "unsupported folder"): the page shows the server's plain words, not a spinner.
//  2. A folder with nothing in it: status "empty" with a sentence, never a blank list.
//  3. No network: status "offline"; the last good rows stay on screen under a banner (kept in memory, then the phone's own copy).
//  4. 401 and 403: "sign in again" words; 404 on a message: "this message is gone".
//  5. A message with only an HTML body: the readable text is made from it (script and style dropped, block tags become paragraph breaks,
//     entities decoded); with neither body, a single "(no text)" block, never a crash.
//  6. A huge body: the readable text is capped (200,000 characters) and the page says it was cut.
//  8. Threading: "Re:", "Fwd:", "Fw:", "[list]" prefixes, repeated, and an empty subject (which never groups).
//  9. An attachment name with path separators or no name; inbound attachments have no download route, so the row says so.
// 10. Reply addressing: Reply-To over From; for our own sent message the reply goes to its first recipient; "Re: " is never doubled; the quote is 30 lines.
// 11. A draft with no recipient, a malformed one, or no words is refused with a plain sentence, no request made.
// 12. Dates: ISO 8601 with or without fractional seconds; an unparseable one is shown as the raw text (cut to 16).
// 13. A field missing from the JSON (no display name, no snippet, no recipients) is tolerated, never an error.
// 14. Nothing here logs: a body, a subject or an address never leaves these functions except as the page's own value.

export type Summary = {
  id: string; direction: string; fromAddress: string; fromDisplay: string; subject: string; snippet: string; trust: string; readAt: string; at: string;
  hasAttachments: boolean;
};
export type AttachmentMeta = { id: string; filename: string; contentType: string; status: string; size: number };
export type Detail = { summary: Summary; text: string; html: string; replyTo: string; to: string[]; cc: string[]; attachments: AttachmentMeta[] };
export type DraftInfo = { id: string; status: string; from: string; subject: string; text: string; replyToMessageID: string; to: string[] };

export type MailRow = {
  id: string; address: string; sender: string; subject: string; snippet: string; stamp: string; trust: string; direction: string;
  unread: boolean; hasAttachments: boolean; count: number;
};
export type MailBlock = { kind: string; text: string };

export const FOLDERS: { name: string; label: string }[] = [
  { name: 'inbox', label: 'Inbox' }, { name: 'drafts', label: 'Drafts' }, { name: 'sent', label: 'Sent' }, { name: 'quarantine', label: 'Quarantine' },
];

export const from = (s: Summary): string => (s.fromDisplay !== '' ? s.fromDisplay : s.fromAddress);
export const isUnread = (s: Summary): boolean => s.readAt === '' && s.direction !== 'outbound';

/** The part of an address after the first ":" (PageAddress.rest). */
export function rest(address: string): string {
  const i = address.indexOf(':');
  return i < 0 ? '' : address.slice(i + 1);
}

/** An id as one path segment: everything but unreserved characters is escaped; "." and ".." too (Swift PathSegment.encode). */
export function segment(id: string): string {
  if (id === '.') return '%2E';
  if (id === '..') return '%2E%2E';
  return encodeURIComponent(id).replace(/[!'()*]/g, (c) => '%' + c.charCodeAt(0).toString(16).toUpperCase());
}

// ---- text

const trimWS = (s: string): string => s.trim();
const HSPACE = '[ \\t\\u00A0\\u1680\\u2000-\\u200A\\u202F\\u205F\\u3000]';
// A scan, not a regex: `^X+|X+$` rescans a long run of spaces from every start (quadratic) on a hostile line.
const isH = (c: number): boolean => c === 32 || c === 9 || c === 0xA0 || c === 0x1680 || (c >= 0x2000 && c <= 0x200A) || c === 0x202F || c === 0x205F || c === 0x3000;
const trimH = (s: string): string => {
  let a = 0, b = s.length;
  while (a < b && isH(s.charCodeAt(a))) a += 1;
  while (b > a && isH(s.charCodeAt(b - 1))) b -= 1;
  return a === 0 && b === s.length ? s : s.slice(a, b);
};

/** Subject without "Re:", "Fwd:", "[list]" prefixes, lowercased; "" for an empty subject. */
export function normalizedSubject(subject: string): string {
  let t = trimWS(subject);
  let changed = true;
  while (changed) {
    changed = false;
    const low = t.toLowerCase();
    for (const p of ['re:', 'fwd:', 'fw:', 'aw:', 'sv:']) {
      if (low.startsWith(p)) { t = trimH(t.slice(p.length)); changed = true; break; }
    }
    if (!changed && t.startsWith('[')) {
      const close = t.indexOf(']');
      if (close >= 0 && close <= 24) { t = trimH(t.slice(close + 1)); changed = true; }
    }
  }
  return t.toLowerCase();
}

export function replySubject(subject: string): string {
  const t = trimWS(subject);
  if (t === '') return 'Re:';
  return t.toLowerCase().startsWith('re:') ? t : 'Re: ' + t;
}

const ENTITIES: Record<string, string> = {
  amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ', hellip: '…', mdash: '—', ndash: '–', rsquo: '’', lsquo: '‘', rdquo: '”', ldquo: '“', copy: '©',
};

function scalar(v: number): string | null {
  if (!Number.isFinite(v) || v < 0 || v > 0x10ffff || (v >= 0xd800 && v <= 0xdfff)) return null;
  return String.fromCodePoint(v);
}

export function decodeEntities(s: string): string {
  if (!s.includes('&')) return s;
  let out = '';
  let i = 0;
  while (i < s.length) {
    if (s[i] === '&') {
      const semi = s.slice(i, i + 12).indexOf(';');
      if (semi >= 0) {
        const name = s.slice(i + 1, i + semi);
        let rep: string | null | undefined;
        if (/^#[xX]/.test(name)) rep = /^[0-9a-fA-F]+$/.test(name.slice(2)) ? scalar(parseInt(name.slice(2), 16)) : null;
        else if (name.startsWith('#')) rep = /^[0-9]+$/.test(name.slice(1)) ? scalar(parseInt(name.slice(1), 10)) : null;
        else rep = ENTITIES[name.toLowerCase()];
        if (rep !== null && rep !== undefined) { out += rep; i += semi + 1; continue; }
      }
    }
    out += s[i]; i += 1;
  }
  return out;
}

/** The most of a message's HTML that anything here reads. Every scan below is linear, and the cap is applied BEFORE any of them: a page's turn is
 * 100 ms of CPU, and the sender chooses the size of the body (a 5 MB message must cost no more than a 300 KB one). Over it, the text is cut and the page says so. */
export const HTML_MAX = 300_000;

const DROPPED = new Set(['script', 'style', 'head', 'title', 'svg', 'noscript']);
const BLOCK_END = new Set(['p', 'div', 'tr', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'table', 'blockquote']);
const lowerAscii = (s: string): string => s.replace(/[A-Z]/g, (c) => String.fromCharCode(c.charCodeAt(0) + 32));

/** HTML to plain paragraphs: no script or style, block ends are blank lines, list items are bullets. One pass over the string with indexOf: no regex
 * rescans an unclosed `<`, `<!--` or `<script` to the end (that is quadratic). Input is cut at HTML_MAX first. */
export function htmlToText(html: string): string {
  const src = html.length > HTML_MAX ? html.slice(0, HTML_MAX) : html;
  const low = lowerAscii(src);
  const n = src.length;
  const noCloser = new Set<string>();                    // a name whose closing tag is absent from here to the end is absent for every later opener too
  let out = '';
  let i = 0;
  while (i < n) {
    const lt = src.indexOf('<', i);
    if (lt < 0) { out += src.slice(i); break; }
    out += src.slice(i, lt);
    if (low.startsWith('<!--', lt)) {
      const e = src.indexOf('-->', lt + 4);
      if (e < 0) { out += src.slice(lt); break; }        // an unclosed comment is text (as the pattern left it); nothing later can close it either
      i = e + 3; continue;
    }
    const gt = src.indexOf('>', lt + 1);
    if (gt < 0) { out += src.slice(lt); break; }         // no '>' after this '<' means none after any later one
    const head = low.slice(lt + 1, Math.min(gt, lt + 41));
    const m = /^(\/?)([a-z][a-z0-9]*)/.exec(head);
    const closing = !!m && m[1] === '/';
    const name = m ? m[2] : '';
    i = gt + 1;
    if (!closing && DROPPED.has(name)) {
      if (!noCloser.has(name)) {
        let from = gt + 1, found = -1, end = -1;
        for (;;) {
          const c = low.indexOf('</' + name, from);
          if (c < 0) break;
          let k = c + 2 + name.length;
          while (k < n && /\s/.test(low[k])) k += 1;
          if (low[k] === '>') { found = c; end = k + 1; break; }
          from = c + 2;
        }
        if (found >= 0) { i = end; continue; }
        noCloser.add(name);
      }
      continue;                                          // no closer: only the tag itself goes
    }
    const inner = src.slice(lt + 1, Math.min(gt, lt + 41));
    if (!closing && name === 'br' && gt - lt <= 40 && /^br\s*\/?$/i.test(inner)) out += '\n';
    else if (!closing && name === 'li') out += '\n• ';
    else if (closing && BLOCK_END.has(name) && /^\/[a-z0-9]+\s*$/i.test(src.slice(lt + 1, Math.min(gt, lt + 41)))) out += '\n\n';
  }
  let t = decodeEntities(out);
  t = t.replace(/[ \t\u00A0]+/g, ' ');
  t = t.replace(/ ?\n ?/g, '\n');
  t = t.replace(/\n{3,}/g, '\n\n');
  return trimWS(t);
}

export const MAX_CHARS = 200_000;

/** Join hard-wrapped lines with spaces, except where the lines look like a list or a short-lined block. */
function reflow(lines: string[]): string {
  const t = lines.map(trimH);
  const listy = t.some((l) => l.startsWith('- ') || l.startsWith('* ') || l.startsWith('• ') || /^[0-9]+[.)] /.test(l));
  const avg = t.length === 0 ? 0 : Math.floor(t.reduce((n, l) => n + l.length, 0) / t.length);
  if (listy || t.length === 1 || avg < 40) return t.join('\n');
  return t.join(' ');
}

/** The readable body as blocks: paragraphs re-flowed (plain mail is hard-wrapped at 72), quoted paragraphs marked, a signature (from a "-- " line)
 * marked. `htmlOnly`: the text came from HTML. `cut`: it was longer than MAX_CHARS. */
export function blocks(text: string, html: string): { blocks: MailBlock[]; htmlOnly: boolean; cut: boolean } {
  const plain = trimWS(text);
  let source = plain;
  let htmlOnly = false;
  let cut = false;
  if (plain === '' && trimWS(html.length > HTML_MAX ? html.slice(0, HTML_MAX) : html) !== '') { source = htmlToText(html); htmlOnly = true; cut = html.length > HTML_MAX; }
  if (source === '') return { blocks: [{ kind: 'p', text: '(no text)' }], htmlOnly, cut };
  if (source.length > MAX_CHARS) { source = source.slice(0, MAX_CHARS); cut = true; }
  source = source.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
  const out: MailBlock[] = [];
  let inSig = false;
  for (const chunk of source.split('\n\n')) {
    let lines = chunk.split('\n');
    // "On <date>, <who> wrote:" stands apart from the quote under it.
    if (!inSig && lines.length > 1) {
      const first = trimH(lines[0]);
      if (first.endsWith('wrote:')) { out.push({ kind: 'p', text: first }); lines = lines.slice(1); }
    }
    if (trimH(lines[0] ?? '') === '--' || inSig) {
      inSig = true;
      const body = lines.filter((l) => trimH(l) !== '--').join('\n');
      if (body !== '') out.push({ kind: 'sig', text: body });
      continue;
    }
    const nonEmpty = lines.filter((l) => trimH(l) !== '');
    if (nonEmpty.length === 0) continue;
    const quoted = nonEmpty.filter((l) => trimH(l).startsWith('>')).length * 2 > nonEmpty.length;
    if (quoted) {
      const stripped = nonEmpty.map((l) => {
        let s = trimH(l);
        while (s.startsWith('>')) { s = s.slice(1); if (s.startsWith(' ')) s = s.slice(1); }
        return s;
      });
      out.push({ kind: 'quote', text: reflow(stripped) });
    } else {
      out.push({ kind: 'p', text: reflow(nonEmpty) });
    }
  }
  return { blocks: out.length === 0 ? [{ kind: 'p', text: '(no text)' }] : out, htmlOnly, cut };
}

export const hasQuoted = (b: MailBlock[]): boolean => b.some((x) => x.kind === 'quote');

/** What the radio says for a message: who, the subject, and the unquoted body, capped. */
export function spoken(fromWho: string, subject: string, bs: MailBlock[]): string {
  const body = bs.filter((b) => b.kind === 'p').map((b) => b.text).join(' ');
  const capped = body.length > 4000 ? body.slice(0, 4000) + ' … and more.' : body;
  return `Message from ${fromWho}. Subject: ${subject === '' ? 'none' : subject}. ${capped}`;
}

/** The quoted body of a reply: "On <date>, <who> wrote:" and up to 30 lines each starting "> ". */
export function replyBody(date: string, who: string, bs: MailBlock[]): string {
  const src = bs.filter((b) => b.kind !== 'sig' && b.text !== '(no text)').map((b) => b.text).join('\n\n');
  const lines = src.split('\n').slice(0, 30).map((l) => '> ' + l).join('\n');
  return `\n\nOn ${date}, ${who} wrote:\n${lines}`;
}

export function fileSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / 1048576).toFixed(1)} MB`;
}

export function safeFilename(s: string): string {
  const parts = s.replace(/\\/g, '/').split('/').filter((p) => p !== '');
  const name = trimWS(parts.length === 0 ? '' : parts[parts.length - 1]);
  return name === '' || name === '.' || name === '..' ? 'attachment' : name;
}

function extensionOf(filename: string): string {
  const parts = filename.split('/').filter((p) => p !== '');
  const last = parts.length === 0 ? '' : parts[parts.length - 1];
  const dot = last.lastIndexOf('.');
  return dot <= 0 ? '' : last.slice(dot + 1).toLowerCase();
}

export function attachmentKind(filename: string, contentType: string): string {
  const ext = extensionOf(filename);
  if (contentType.startsWith('image/') || ['png', 'jpg', 'jpeg', 'gif', 'heic', 'webp'].includes(ext)) return 'image';
  if (contentType === 'application/pdf' || ext === 'pdf') return 'pdf';
  if (contentType.startsWith('audio/')) return 'audio';
  if (contentType.startsWith('video/')) return 'video';
  if (contentType.startsWith('text/') || ['txt', 'md', 'csv'].includes(ext)) return 'text';
  if (ext === 'epub') return 'book';
  return 'file';
}

/** A light check, not a validator: one "@", something either side, a dot in the domain, no spaces. */
export function looksLikeAddress(s: string): boolean {
  const t = trimH(s);
  const at = t.indexOf('@');
  if (t.includes(' ') || at <= 0) return false;
  const domain = t.slice(at + 1);
  return domain.includes('.') && !domain.startsWith('.') && !domain.endsWith('.') && !domain.includes('@');
}

/** "Ana R. <ana@example.com>" gives "ana@example.com"; a bare address stays. */
export function addressOnly(s: string): string {
  const lt = s.indexOf('<'), gt = s.indexOf('>');
  if (lt >= 0 && gt >= 0 && lt < gt) return trimH(s.slice(lt + 1, gt));
  return trimH(s);
}

/** Words if the draft cannot be saved as it is (failure 11), else null. */
export function problem(to: string, body: string): string | null {
  const addrs = to.split(/[,;]/).map(trimH).filter((a) => a !== '');
  if (addrs.length === 0) return 'Add who it is for.';
  const bad = addrs.find((a) => !looksLikeAddress(addressOnly(a)));
  if (bad !== undefined) return `Check the address: ${bad}`;
  const words = trimWS(body.split('\n').filter((l) => !l.startsWith('>') && !l.startsWith('On ')).join(''));
  if (words === '') return 'Write something first.';
  return null;
}

export function recipients(to: string): string[] {
  return to.split(/[,;]/).map((a) => addressOnly(a)).filter((a) => a !== '');
}

// ---- dates

const ISO = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
/** Epoch milliseconds of an ISO 8601 date-time with a zone, or null. Fractions are cut to milliseconds (the server sends nanoseconds). */
export function parseDate(s: string): number | null {
  const m = ISO.exec(s);
  if (!m) return null;
  const ms = m[2] ? m[2].slice(0, 4).padEnd(4, '0') : '';
  const t = Date.parse(m[1] + (ms === '.000' || ms === '' ? '' : ms) + m[3]);
  return Number.isFinite(t) ? t : null;
}

const DAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const two = (n: number): string => String(n).padStart(2, '0');

/** A moment shifted to the listener's wall clock: read its UTC fields. `offsetMinutes` is Date.getTimezoneOffset (minutes behind UTC). */
function wall(ts: number, offsetMinutes: number): Date { return new Date(ts - offsetMinutes * 60000); }
const zoneOf = (ts: number): number => new Date(ts).getTimezoneOffset();
const dayNumber = (w: Date): number => Math.floor(w.getTime() / 86400000);

/** "09:12" today, "Mon" within a week, "12 Mar" otherwise; the raw text (cut) when unparseable. `now` is the server's clock. */
export function shortDate(s: string, now: number, offset?: number): string {
  const t = parseDate(s);
  if (t === null) return s.slice(0, 16);
  const o = offset ?? zoneOf(t);
  const d = wall(t, o), n = wall(now, offset ?? zoneOf(now));
  if (dayNumber(d) === dayNumber(n)) return `${two(d.getUTCHours())}:${two(d.getUTCMinutes())}`;
  if (now - t < 6 * 86400000 && t < now) return DAYS[d.getUTCDay()];
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}`;
}

export function longDate(s: string, offset?: number): string {
  const t = parseDate(s);
  if (t === null) return s.slice(0, 32);
  const d = wall(t, offset ?? zoneOf(t));
  return `${DAYS[d.getUTCDay()]} ${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}, ${two(d.getUTCHours())}:${two(d.getUTCMinutes())}`;
}

// ---- parsing the maild JSON (every field optional)

type Obj = Record<string, unknown>;
const obj = (v: unknown): Obj => (v !== null && typeof v === 'object' && !Array.isArray(v) ? (v as Obj) : {});
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
const str = (o: Obj, k: string): string => (typeof o[k] === 'string' ? (o[k] as string) : '');

export function parseSummary(v: unknown): Summary {
  const o = obj(v);
  const at = [str(o, 'received_at'), str(o, 'sent_at'), str(o, 'created_at')].find((x) => x !== '') ?? '';
  return {
    id: str(o, 'id'), direction: str(o, 'direction'), fromAddress: str(o, 'from_address'), fromDisplay: str(o, 'from_display'), subject: str(o, 'subject'),
    snippet: str(o, 'snippet'), trust: str(o, 'trust_status'), readAt: str(o, 'read_at'), at, hasAttachments: o.has_attachments === true,
  };
}

export function parseList(v: unknown): { rows: Summary[]; total: number; unread: number; next: string } {
  const o = obj(v);
  const rows = arr(o.messages).map(parseSummary);
  return {
    rows, total: Number.isInteger(o.total) ? (o.total as number) : rows.length, unread: Number.isInteger(o.unread) ? (o.unread as number) : 0, next: str(o, 'next_cursor'),
  };
}

export function parseDetail(v: unknown): Detail {
  const o = obj(v);
  let replyTo = '';
  for (const [k, val] of Object.entries(obj(o.raw_headers))) if (k.toLowerCase() === 'reply-to') replyTo = addressOnly(typeof val === 'string' ? val : '');
  const rec = obj(o.recipients);
  const names = (k: string): string[] => arr(rec[k]).map((r) => {
    const x = obj(r); const disp = str(x, 'display'), addr = str(x, 'address');
    return disp === '' ? addr : `${disp} <${addr}>`;
  });
  return {
    summary: parseSummary(o.message), text: str(o, 'text_body'), html: str(o, 'html_body'), replyTo, to: names('to'), cc: names('cc'),
    attachments: arr(o.attachments).map((a) => {
      const x = obj(a);
      return { id: str(x, 'id'), filename: str(x, 'filename'), contentType: str(x, 'content_type'), status: str(x, 'status'), size: typeof x.size_bytes === 'number' ? x.size_bytes : 0 };
    }),
  };
}

export function parseDraft(v: unknown): DraftInfo {
  const o = obj(v);
  return {
    id: str(o, 'id'), status: str(o, 'status'), from: str(o, 'from_address'), subject: str(o, 'subject'), text: str(o, 'text_body'),
    replyToMessageID: str(o, 'reply_to_message_id'), to: arr(o.to_addresses).filter((x): x is string => typeof x === 'string'),
  };
}

// ---- threads and rows

export function row(m: Summary, now: number): MailRow {
  const sender = from(m);
  return {
    id: m.id, address: 'mail:' + m.id, sender: sender === '' ? '(unknown sender)' : sender, subject: m.subject === '' ? '(no subject)' : m.subject,
    snippet: m.snippet, stamp: shortDate(m.at, now), trust: m.trust, direction: m.direction, unread: isUnread(m), hasAttachments: m.hasAttachments, count: 1,
  };
}

/** One row per thread (newest first as given): the newest message stands for it. A message with no subject is its own thread. */
export function threadRows(list: Summary[], now: number): MailRow[] {
  const order: string[] = [];
  const groups = new Map<string, Summary[]>();
  for (const m of list) {
    const key = normalizedSubject(m.subject);
    const k = key === '' ? 'id:' + m.id : 's:' + key;
    if (!groups.has(k)) { order.push(k); groups.set(k, []); }
    groups.get(k)!.push(m);
  }
  const rows: MailRow[] = [];
  for (const k of order) {
    const g = groups.get(k)!;
    const top = g.reduce((a, b) => (a.at < b.at ? b : a));
    const r = row(top, now);
    r.count = g.length; r.unread = g.some(isUnread);
    rows.push(r);
  }
  return rows;
}

/** The messages sharing a subject, oldest first; just the one when the subject is empty. */
export function threadOf(m: Summary, all: Summary[]): Summary[] {
  const key = normalizedSubject(m.subject);
  if (key === '') return [m];
  const seen = new Set<string>();
  const same = [m, ...all].filter((x) => normalizedSubject(x.subject) === key && !seen.has(x.id) && !!seen.add(x.id));
  return same.sort((a, b) => (a.at < b.at ? -1 : a.at > b.at ? 1 : 0));
}

// ---- plain words for a failure

export type Words = { status: string; message: string };
const OFFLINE: Words = { status: 'offline', message: 'You are offline, or your computer is not answering.' };

/** The HTTP status an error carries (an `HTTPError`'s `status`, or "HTTP 404" in its message), or 0. */
export function statusOf(e: unknown): number {
  const s = (e as { status?: unknown } | null)?.status;
  if (typeof s === 'number') return s;
  const m = /\bHTTP (\d{3})\b/.exec(String((e as Error | null)?.message ?? ''));
  return m ? Number(m[1]) : 0;
}

/** The server's own short `error` string from an error message ("HTTP 400: unsupported folder") or "". */
function serverWords(e: unknown): string {
  const m = /^HTTP \d{3}: (.*)$/s.exec(String((e as Error | null)?.message ?? ''));
  return m ? m[1] : '';
}

export function words(e: unknown): Words {
  const name = (e as { name?: string } | null)?.name ?? '';
  const message = String((e as Error | null)?.message ?? '');
  if (name === 'SignedOutError') return { status: 'error', message: message.includes('refused') ? 'Sign in again to read your mail.' : 'Sign in to read your mail.' };
  const status = statusOf(e);
  if (status === 401) return { status: 'error', message: 'Sign in again to read your mail.' };
  if (status === 403) return { status: 'error', message: 'This phone is not allowed to do that yet.' };
  if (status === 429) return { status: 'error', message: 'Too many requests. Try again in a moment.' };
  if (status === 404) return { status: 'error', message: 'This message is gone.' };
  if (status === 413) return { status: 'error', message: 'That is too large for your computer to take from the phone.' };
  if (status !== 0) { const w = serverWords(e); return { status: 'error', message: w !== '' ? w : `Your computer could not do that (${status}).` }; }
  if (message.startsWith('timeout:') || name === 'TimeoutError') return { status: 'error', message: 'Your computer took too long to answer. Try again.' };
  if (message.startsWith('network:') || name === 'AbortError' || name === 'TypeError') return OFFLINE;
  if (message.includes('not JSON')) return { status: 'error', message: 'Something unexpected came back from your computer.' };
  return { status: 'error', message: 'Could not reach your mail.' };
}

export function emptyWords(folder: string): string {
  switch (folder) {
    case 'inbox': return 'Nothing in your inbox. New mail to your address will appear here.';
    case 'drafts': return 'No drafts. Reply to a message, or start one, and it will be kept here.';
    case 'sent': return 'Nothing sent yet.';
    case 'quarantine': return 'Nothing held back. Mail that cannot be trusted waits here for you.';
    default: return 'Nothing here.';
  }
}
