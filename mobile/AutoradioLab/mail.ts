// Mail's sources: the mailbox, a message, a compose page, saving a draft, asking for the approval link, marking unread, and the catch-up that keeps the
// phone's copy. They replace the Swift MailService (ChoirMail.swift), its Exact handlers (AutoradioModule+Mail.swift), MailPageSource, the mail half of
// ContentSync and the mail rows of ContentStore. The listener's computer (the Mail agent) is the only custodian of email: the phone never talks to a mail
// provider, only to that computer, through the signed-in routes the web uses (`authedJSON`; the server's handlers are internal/maild/api.go and drafts.go):
//   GET /api/email/messages?folder=&limit=&cursor=   -> {messages, next_cursor, total, unread}
//   GET /api/email/messages/{id}                      -> {message, text_body, html_body, raw_headers, recipients, attachments}   (html_body is NOT cleaned)
//   POST /api/email/messages/{id}/read and /unread    GET /api/email/aliases    GET and POST /api/email/drafts    GET /api/email/drafts/{id}
//   POST /api/email/drafts/{id}/approval-email        -> {review_url}   (sending is an owner act: POST .../send answers 403 to the phone and is never called)
//
// A source never throws for a failure the page has words for: it answers the page's own state (`status` "error" or "offline" and `message`), so the page shows
// its own error and retry, never a spinner word. Time is never read: "now" is the server's `Date` header from the last reply (or the newest row's date).
// Logs carry ids and counts only: never a sender, a subject, an address or a word of a message.

import type { Storage } from './app.contract.d.ts';
import { authedJSON } from './api.ts';
import * as M from './mail-model.ts';
import { sanitized } from './mail-sanitize.ts';
import * as C from './mail-store.ts';
import { sha256, hex } from './mail-seal.ts';
import demoInbox from './fixtures/demo/mail-inbox.json';
import demoSent from './fixtures/demo/mail-sent.json';
import demoQuarantine from './fixtures/demo/mail-quarantine.json';
import demoAliases from './fixtures/demo/mail-aliases.json';
import demoDrafts from './fixtures/demo/mail-drafts.json';
import demoDraft1 from './fixtures/demo/mail-draft-demo-draft-1.json';
import demoM1 from './fixtures/demo/mail-msg-m1.json';
import demoM2 from './fixtures/demo/mail-msg-m2.json';
import demoM3 from './fixtures/demo/mail-msg-m3.json';
import demoM4 from './fixtures/demo/mail-msg-m4.json';
import demoM5 from './fixtures/demo/mail-msg-m5.json';
import demoM6 from './fixtures/demo/mail-msg-m6.json';
import demoQ1 from './fixtures/demo/mail-msg-q1.json';
import demoS1 from './fixtures/demo/mail-msg-s1.json';

export type BoxView = {
  status: string; message: string; folder: string; title: string; unread: number; total: number; cached: boolean; rows: M.MailRow[];
  folders: { name: string; label: string; address: string; badge: string; current: boolean }[];
};
export type MessageView = {
  status: string; message: string; id: string; address: string; subject: string; sender: string; fromAddress: string; toLine: string; ccLine: string; date: string;
  trust: string; trustNote: string; unread: boolean; htmlOnly: boolean; hasOriginal: boolean; hasQuoted: boolean; canReply: boolean; cut: boolean; originalPath: string;
  blocks: M.MailBlock[];
  attachments: { id: string; filename: string; size: string; kind: string; address: string; note: string; available: boolean }[];
  thread: { id: string; address: string; sender: string; stamp: string; snippet: string; current: boolean }[];
  replyLabel: string; spoken: string;
};
export type ComposeView = {
  status: string; forAddress: string; sendAs: string; toLine: string; subject: string; body: string; draftId: string; note: string; reviewURL: string; replyTo: string;
};
export type Ack = { status: string; message: string };
export type SyncView = { status: string; held: number; fetched: number; note: string };

export const LIST_LIMIT = 100;
/** Bodies fetched by one catch-up answer. A bound on one turn's work, not a limit on the listener: the next open of the mailbox goes on from where this stopped. */
export const SYNC_BODIES = 25;
const DEMO_STATES = ['demo-empty', 'demo-loading', 'demo-offline', 'demo-error'];
const OFFLINE_NOTE = 'Offline. Showing mail saved on this phone.';

// ---- memory (one session)

type Held = { rows: M.Summary[]; total: number; unread: number; next: string };
const kept = new Map<string, Held>();
const autoRead = new Set<string>();
let drafts: M.DraftInfo[] = [];
let alias: string | null = null;
let clock = 0;
let started = false;

/** Forget what this session holds. */
export function forget(): void { kept.clear(); autoRead.clear(); drafts = []; alias = null; clock = 0; started = false; }

/** SIGN-OUT. Removes the phone's copy of the account's mail (every sealed file, every "Show original" document, the key) and what this session holds.
 * The sign-in code calls this once, with the app's `storage` (and `store` when it has it, so the key is forgotten too); it never throws and logs counts only. */
export async function wipeMail(storage?: Storage): Promise<{ files: number; originals: number }> {
  forget();
  const r = await C.wipeFiles({ storage });
  console.log(`mail: wiped files=${r.files} originals=${r.originals}`);
  return r;
}

// ---- the computer (or the demo fixtures)

class StatusError extends Error { status: number; constructor(status: number, message: string) { super(`HTTP ${status}: ${message}`); this.name = 'StatusError'; this.status = status; } }
type Call = { method?: 'GET' | 'POST'; query?: Record<string, string | number>; body?: unknown };

const DEMO_MSG: Record<string, unknown> = { m1: demoM1, m2: demoM2, m3: demoM3, m4: demoM4, m5: demoM5, m6: demoM6, q1: demoQ1, s1: demoS1 };

/** Demo mode answers the mail routes from fixtures/demo (the Swift MailDemo); nothing is sent anywhere. */
function demoRoute(path: string, opts: Call): unknown {
  const method = opts.method ?? 'GET';
  const parts = path.slice('/api/email/'.length).split('/');
  if (method === 'GET' && parts[0] === 'messages' && parts.length === 1) {
    const f = String(opts.query?.folder ?? '') || 'inbox';
    if (f === 'inbox') return demoInbox;
    if (f === 'sent') return demoSent;
    if (f === 'quarantine') return demoQuarantine;
    throw new StatusError(400, 'unsupported folder');
  }
  if (method === 'GET' && parts[0] === 'messages' && parts.length === 2) { const m = DEMO_MSG[decodeURIComponent(parts[1])]; if (m) return m; throw new StatusError(404, 'not found'); }
  if (method === 'POST' && parts[0] === 'messages' && parts.length === 3) return {};
  if (method === 'GET' && parts[0] === 'aliases') return demoAliases;
  if (method === 'GET' && parts[0] === 'drafts' && parts.length === 1) return demoDrafts;
  if (method === 'GET' && parts[0] === 'drafts' && parts.length === 2) return demoDraft1;
  if (method === 'POST' && parts[0] === 'drafts' && parts.length === 1) {
    const b = (opts.body ?? {}) as Record<string, unknown>;
    return { id: 'demo-draft-new', status: 'draft', version: 1, from_address: b.from_address, to_addresses: b.to_addresses, subject: b.subject, text_body: b.text_body, reply_to_message_id: b.reply_to_message_id };
  }
  if (method === 'POST' && parts[0] === 'drafts' && parts[2] === 'approval-email') return { status: 'sent', review_url: 'https://choir.news/mail/review/demo', token_id: 'demo' };
  throw new StatusError(404, 'demo mode serves fixtures only');
}

async function call(demo: boolean, path: string, opts: Call = {}): Promise<unknown> {
  if (demo) return demoRoute(path, opts);
  const r = await authedJSON(path, opts);
  if (r.serverTime !== null && r.serverTime > 0) clock = r.serverTime;
  return r.body;
}

/** "now" for dates: the server's clock, else the newest date the lists hold. */
function nowOf(rows: M.Summary[] = []): number {
  if (clock > 0) return clock;
  let n = 0;
  for (const r of rows) { const t = M.parseDate(r.at); if (t !== null && t > n) n = t; }
  return n;
}

// ---- the mailbox

const emptyBox = (): BoxView => ({ status: '', message: '', folder: '', title: '', unread: 0, total: 0, cached: false, rows: [], folders: [] });

/** The page's constant frame, answered with no fetch: the title and the four tabs (the inbox's unread count when it is known). */
export function frame(address: string): BoxView {
  const v = emptyBox();
  const folder = M.rest(address);
  v.folder = folder === '' ? 'inbox' : folder;
  v.title = M.FOLDERS.find((f) => f.name === v.folder)?.label ?? 'Mail';
  v.folders = M.FOLDERS.map((f) => {
    const c = kept.get('inbox');
    return { name: f.name, label: f.label, address: 'mailbox:' + f.name, current: f.name === v.folder, badge: f.name === 'inbox' && c && c.unread > 0 ? String(c.unread) : '' };
  });
  return v;
}

async function ensureKept(env: C.Env, folder: string, demo: boolean): Promise<Held | null> {
  const k = kept.get(folder);
  if (k) return k;
  if (demo) return null;
  const c = await C.loadFolder(env, folder);
  if (!c) return null;
  const h: Held = { rows: c.rows, total: c.total, unread: c.unread, next: '' };
  kept.set(folder, h);
  return h;
}

/** The last list this session fetched, or the phone's saved one, answered at once: a return visit shows it and refreshes underneath. */
export async function mailFrame(address: string, env: C.Env): Promise<BoxView> {
  const v = frame(address);
  if (!address.startsWith('mailbox:') || DEMO_STATES.includes(v.folder) || v.folder === 'drafts') { v.status = 'frame'; return v; }
  const h = await ensureKept(env, v.folder, false);
  if (!h) { v.status = 'frame'; return v; }
  if (v.folder === 'inbox') for (const f of v.folders) if (f.name === 'inbox') f.badge = h.unread > 0 ? String(h.unread) : '';
  v.rows = M.threadRows(h.rows, nowOf(h.rows)); v.status = 'cached';
  return v;
}

function savedWords(copyAt: number): string {
  const n = clock;
  if (n <= 0 || copyAt <= 0) return OFFLINE_NOTE;
  const s = (n - copyAt) / 1000;
  const when = s < 90 ? 'just now' : s < 5400 ? `${Math.floor(s / 60)} minutes ago` : s < 129600 ? `${Math.round(s / 3600)} hours ago` : `${Math.round(s / 86400)} days ago`;
  return `Offline. Showing mail saved on this phone, last synced ${when}.`;
}

export async function mailBox(address: string, demo: boolean, env: C.Env): Promise<BoxView> {
  if (!address.startsWith('mailbox:')) { const i = emptyBox(); i.status = 'idle'; return i; }
  const v = frame(address);
  if (DEMO_STATES.includes(v.folder)) return demoState(v);
  try {
    if (v.folder === 'drafts') {
      const o = (await call(demo, '/api/email/drafts', {})) as { drafts?: unknown[] } | null;
      drafts = (o?.drafts ?? []).map(M.parseDraft);
      v.rows = drafts.map((d) => ({
        id: d.id, address: 'mail:draft/' + d.id, sender: 'To ' + (d.to[0] ?? '(nobody yet)'), subject: d.subject === '' ? '(no subject)' : d.subject, snippet: d.text.slice(0, 120),
        stamp: '', trust: '', direction: 'draft', unread: false, hasAttachments: false, count: 1,
      }));
      v.total = drafts.length;
    } else {
      const o = await call(demo, '/api/email/messages', { query: { folder: v.folder, limit: LIST_LIMIT } });
      const l = M.parseList(o);
      kept.set(v.folder, { rows: l.rows, total: l.total, unread: l.unread, next: l.next });
      if (!demo) await C.saveFolder(env, { folder: v.folder, rows: l.rows, total: l.total, unread: l.unread, savedAt: clock });
      v.rows = M.threadRows(l.rows, nowOf(l.rows)); v.total = l.total; v.unread = l.unread;
      if (v.folder === 'inbox') for (const f of v.folders) if (f.name === 'inbox') f.badge = l.unread > 0 ? String(l.unread) : '';
    }
    if (v.rows.length === 0) { v.status = 'empty'; v.message = M.emptyWords(v.folder); } else v.status = 'ready';
    console.log(`mail: box ${v.folder} rows=${v.rows.length}`);
  } catch (e) {
    const w = M.words(e);
    v.status = w.status; v.message = w.message;
    console.log(`mail: box ${v.folder} failed ${w.status}`);
    const inMemory = kept.has(v.folder);
    const h = await ensureKept(env, v.folder, demo);          // failure 3: keep the last good rows under the banner
    if (h && v.folder !== 'drafts') {
      v.rows = M.threadRows(h.rows, nowOf(h.rows)); v.total = h.total; v.unread = h.unread; v.cached = true;
      if (v.status === 'offline' && !inMemory && v.rows.length > 0) v.message = savedWords((await C.loadFolder(env, v.folder))?.savedAt ?? 0);   // never opened this session: the phone still holds the mail
    }
  }
  return v;
}

/** Demo mode only: the four states a real mailbox can be in, as real screens. */
function demoState(base: BoxView): BoxView {
  const v = base;
  switch (v.folder) {
    case 'demo-empty': v.status = 'empty'; v.message = M.emptyWords('inbox'); break;
    case 'demo-offline': {
      v.status = 'offline'; v.message = 'You are offline, or your computer is not answering.';
      const c = kept.get('inbox');
      if (c) { v.rows = M.threadRows(c.rows, nowOf(c.rows)); v.cached = true; }
      break;
    }
    case 'demo-error': v.status = 'error'; v.message = 'Your computer could not list this folder (500).'; break;
    default: v.status = 'empty'; v.message = M.emptyWords('inbox');       // "slow": the Swift demo waited 12 s; a state the page can show is enough here
  }
  v.title = 'Mail';
  return v;
}

// ---- a message

const originalName = async (id: string): Promise<string> => `mail-${hex((await sha256(new TextEncoder().encode(id))).subarray(0, 16))}.html`;

function allKept(): M.Summary[] { return [...kept.values()].flatMap((h) => h.rows); }

async function setRead(id: string, read: boolean, demo: boolean): Promise<boolean> {
  try {
    await call(demo, `/api/email/messages/${M.segment(id)}/${read ? 'read' : 'unread'}`, { method: 'POST' });
    for (const h of kept.values()) {
      for (const r of h.rows) {
        if (r.id !== id) continue;
        if (read && r.readAt === '') { r.readAt = 'read'; h.unread = Math.max(0, h.unread - 1); }
        if (!read && r.readAt !== '' && r.direction !== 'outbound') { r.readAt = ''; h.unread += 1; }
      }
    }
    return true;
  } catch { return false; }
}

function emptyMessage(): MessageView {
  return {
    status: '', message: '', id: '', address: '', subject: '', sender: '', fromAddress: '', toLine: '', ccLine: '', date: '', trust: '', trustNote: '', unread: false, htmlOnly: false,
    hasOriginal: false, hasQuoted: false, canReply: false, cut: false, originalPath: '', blocks: [], attachments: [], thread: [], replyLabel: '', spoken: '',
  };
}

type Loaded = { view: MessageView; detail: M.Detail | null };

async function loadMessage(address: string, demo: boolean, wantOriginal: boolean, env: C.Env): Promise<Loaded> {
  const v = emptyMessage();
  if (!address.startsWith('mail:')) { v.status = 'idle'; return { view: v, detail: null }; }
  const id = M.rest(address);
  if (id === '' || id === 'new' || id.startsWith('reply/') || id.startsWith('draft/')) { v.status = 'idle'; return { view: v, detail: null }; }
  v.id = id; v.address = address;
  if (!started) { started = true; await C.forgetOldCopy(env.storage); }      // documents a past session left in the cache
  try {
    let d: M.Detail;
    try {
      d = M.parseDetail(await call(demo, '/api/email/messages/' + M.segment(id), {}));
      if (!demo) {
        const folder = [...kept.entries()].find(([, h]) => h.rows.some((r) => r.id === id))?.[0] ?? (await C.loadMessage(env, id))?.folder ?? 'inbox';
        await C.saveMessage(env, { id, folder, summary: d.summary, text: d.text, to: d.to, cc: d.cc, replyTo: d.replyTo, attachments: d.attachments, savedAt: clock });
      }
    } catch (e) {
      const held = demo ? null : await C.loadMessage(env, id);       // no network: the message the phone holds
      if (!held || M.words(e).status !== 'offline') throw e;
      d = { summary: held.summary, text: held.text, html: '', replyTo: held.replyTo, to: held.to, cc: held.cc, attachments: held.attachments };
      v.message = savedWords(held.savedAt);
    }
    if (!kept.has('inbox')) { try { const l = M.parseList(await call(demo, '/api/email/messages', { query: { folder: 'inbox', limit: LIST_LIMIT } })); kept.set('inbox', { rows: l.rows, total: l.total, unread: l.unread, next: l.next }); } catch { await ensureKept(env, 'inbox', demo); } }
    const s = d.summary;
    v.subject = s.subject === '' ? '(no subject)' : s.subject;
    v.sender = M.from(s) === '' ? '(unknown sender)' : M.from(s); v.fromAddress = s.fromAddress;
    v.toLine = d.to.join(', '); v.ccLine = d.cc.join(', ');
    v.date = M.longDate(s.at); v.unread = M.isUnread(s);
    v.trust = s.trust;
    v.trustNote = s.trust === 'quarantined' ? 'Held back: this sender could not be trusted. Nothing in it was run or loaded.' : s.trust === 'unverified' ? 'The sender could not be verified.' : '';
    const b = M.blocks(d.text, d.html);
    v.blocks = b.blocks; v.htmlOnly = b.htmlOnly; v.cut = b.cut; v.hasQuoted = M.hasQuoted(b.blocks);
    v.hasOriginal = d.html !== '';
    const name = v.hasOriginal ? await originalName(id) : '';
    v.originalPath = v.hasOriginal ? `${C.CACHE}/${name}` : '';
    // The sender's own layout is written only while it is asked for, and every other document is removed: the message's HTML sits on the phone, readable, for as short a time as it can.
    if (v.hasOriginal && wantOriginal && env.storage) {
      await C.forgetOldCopy(env.storage, name);
      try { await env.storage.fs.atomicWriteFile(v.originalPath, new TextEncoder().encode(sanitized(d.html))); } catch { v.hasOriginal = false; v.originalPath = ''; }
    } else { await C.forgetOldCopy(env.storage); }
    v.attachments = d.attachments.map((a) => {
      const filename = M.safeFilename(a.filename);
      // Failure 9: the web does not offer an inbound download either, so the row is honest about it. In demo mode the attachments are bundled files the viewers open by address.
      return {
        id: a.id, filename, size: M.fileSize(a.size), kind: M.attachmentKind(a.filename, a.contentType), available: demo,
        address: demo ? `file:/Mail/${s.id}/${filename}` : '', note: demo ? '' : 'Saving attachments from mail is not offered yet.',
      };
    });
    const now = nowOf(allKept());
    v.thread = M.threadOf(s, allKept()).map((t) => ({ id: t.id, address: 'mail:' + t.id, sender: M.from(t), stamp: M.shortDate(t.at, now), snippet: t.snippet, current: t.id === s.id }));
    v.canReply = (s.direction !== 'outbound' || d.to.length > 0) && s.trust !== 'quarantined';
    const who = M.from(s).split(' <')[0];
    v.replyLabel = 'Reply to ' + (who === '' ? 'sender' : who);
    v.spoken = M.spoken(v.sender, v.subject, v.blocks);
    v.status = 'ready';
    // Opening a message marks it read, once: a "Mark unread" afterwards stays unread when the page asks again.
    if (M.isUnread(s) && !autoRead.has(id)) { autoRead.add(id); await setRead(id, true, demo); }
    console.log(`mail: message ready blocks=${v.blocks.length} attachments=${v.attachments.length}`);
    return { view: v, detail: d };
  } catch (e) {
    const w = M.words(e);
    v.status = w.status; v.message = w.message;
    console.log(`mail: message failed ${w.status}`);
    return { view: v, detail: null };
  }
}

export async function mailMessage(address: string, demo: boolean, wantOriginal: boolean, env: C.Env): Promise<MessageView> {
  return (await loadMessage(address, demo, wantOriginal, env)).view;
}

// ---- compose (a reply, a new message, an existing draft)

const emptyCompose = (): ComposeView => ({ status: '', forAddress: '', sendAs: '', toLine: '', subject: '', body: '', draftId: '', note: '', reviewURL: '', replyTo: '' });

async function aliasAddress(demo: boolean): Promise<string> {
  if (alias) return alias;
  const o = (await call(demo, '/api/email/aliases', {})) as { aliases?: { address?: unknown }[] } | null;
  const a = o?.aliases?.[0]?.address;
  if (typeof a !== 'string' || a === '') throw new StatusError(409, 'You have no sending address yet.');
  alias = a;
  return a;
}

export async function mailCompose(address: string, demo: boolean, env: C.Env): Promise<ComposeView> {
  const v = emptyCompose();
  if (!address.startsWith('mail:')) { v.status = 'idle'; return v; }
  const rest = M.rest(address);
  v.forAddress = address;
  if (rest === 'new') { v.status = 'seed'; return v; }
  if (rest.startsWith('draft/')) {
    const id = rest.slice(6);
    try {
      const d = M.parseDraft(await call(demo, '/api/email/drafts/' + M.segment(id), {}));
      v.status = 'saved'; v.draftId = d.id; v.sendAs = d.from; v.toLine = d.to.join(', '); v.subject = d.subject; v.body = d.text;
      v.note = 'Saved. Sending needs your approval.';
    } catch (e) { v.status = 'error'; v.note = M.words(e).message; }
    return v;
  }
  if (!rest.startsWith('reply/')) { v.status = 'idle'; return v; }
  const id = rest.slice(6);
  const { view: m, detail: d } = await loadMessage('mail:' + id, demo, false, env);
  if (m.status !== 'ready' || !d) { v.status = 'error'; v.note = m.message === '' ? 'Could not open the message to reply to.' : m.message; return v; }
  v.toLine = d.summary.direction === 'outbound' ? M.addressOnly(d.to[0] ?? '') : d.replyTo === '' ? d.summary.fromAddress : d.replyTo;
  v.subject = M.replySubject(d.summary.subject);
  v.body = M.replyBody(m.date, m.sender, m.blocks);
  v.replyTo = id; v.status = 'seed';
  return v;
}

export async function mailSaveDraft(address: string, to: string, subject: string, body: string, demo: boolean): Promise<ComposeView> {
  const v = emptyCompose();
  v.forAddress = address; v.toLine = to; v.subject = subject; v.body = body;
  const rest = M.rest(address);
  if (rest.startsWith('reply/')) v.replyTo = rest.slice(6);
  const p = M.problem(to, body);
  if (p !== null) { v.status = 'error'; v.note = p; return v; }          // refused here, no request made
  try {
    const from = await aliasAddress(demo);
    v.sendAs = from;
    const req: Record<string, unknown> = { from_address: from, subject, text_body: body, to_addresses: M.recipients(to) };
    if (v.replyTo !== '') { req.reply_to_message_id = v.replyTo; req.source_kind = 'mail'; req.source_ref = v.replyTo; }
    const d = M.parseDraft(await call(demo, '/api/email/drafts', { method: 'POST', body: req }));
    v.status = 'saved'; v.draftId = d.id;
    v.note = 'Saved as a draft. It has not been sent.';
    console.log('mail: draft saved');
  } catch (e) { v.status = 'error'; v.note = M.words(e).message; }
  return v;
}

/** Sending is an owner act the server does not accept from a key (POST /api/email/drafts/{id}/send answers 403, "owner session required"), so it is never called:
 * the phone's way is the approval email, and the listener sends from the page it links to. */
export async function mailApprovalEmail(address: string, draft: string, demo: boolean): Promise<ComposeView> {
  const v = emptyCompose();
  v.forAddress = address; v.draftId = draft;
  try {
    const o = (await call(demo, `/api/email/drafts/${M.segment(draft)}/approval-email`, { method: 'POST', body: {} })) as { review_url?: unknown } | null;
    v.status = 'approval'; v.reviewURL = typeof o?.review_url === 'string' ? o.review_url : '';
    v.note = 'An approval link was emailed to you. Open it in your browser to read the draft and send it.';
  } catch (e) {
    const w = M.words(e);
    v.status = 'saved'; v.note = M.statusOf(e) === 403 ? 'This phone cannot ask for an approval link yet.' : w.message;
  }
  return v;
}

export async function mailMarkUnread(address: string, demo: boolean): Promise<Ack> {
  const ok = await setRead(M.rest(address), false, demo);
  return { status: ok ? 'ok' : 'error', message: ok ? '' : 'Could not mark it unread.' };
}

// ---- the phone's copy: the catch-up

/** Keeps the phone's copy of the folder in front: the text of every message in the newest page that is not held yet, then older pages while the folder has not been
 * walked, SYNC_BODIES bodies an answer. Counts only; a failure ends the answer and the next open goes on. Drafts live on the computer and are not copied (a draft is
 * one request); demo mode copies nothing. */
export async function mailSync(address: string, demo: boolean, env: C.Env): Promise<SyncView> {
  const out: SyncView = { status: '', held: 0, fetched: 0, note: '' };
  if (!address.startsWith('mailbox:') || demo) { out.status = 'idle'; return out; }
  const folder = M.rest(address);
  if (!M.FOLDERS.some((f) => f.name === folder) || folder === 'drafts' || !C.sealed(env) || !env.storage) { out.status = 'idle'; return out; }
  const h = kept.get(folder) ?? (await ensureKept(env, folder, false));
  if (!h) { out.status = 'idle'; return out; }
  const state = await C.loadState(env);
  let budget = SYNC_BODIES;
  let page = h.rows, nextCursor = h.next, first = true, stopped = false;
  try {
    for (;;) {
      for (const r of page) {
        if (r.id === '' || (await C.holds(env, r.id))) continue;
        if (budget === 0) { stopped = true; break; }
        const d = M.parseDetail(await call(false, '/api/email/messages/' + M.segment(r.id), {}));
        await C.saveMessage(env, { id: r.id, folder, summary: d.summary, text: d.text, to: d.to, cc: d.cc, replyTo: d.replyTo, attachments: d.attachments, savedAt: clock });
        budget -= 1; out.fetched += 1;
      }
      if (stopped) break;
      if (!first) { state.cursor[folder] = nextCursor; if (nextCursor === '') state.done[folder] = true; }
      if (state.done[folder]) break;
      const from = first ? (state.cursor[folder] ?? '') || nextCursor : nextCursor;
      if (from === '') { state.done[folder] = true; break; }
      first = false;
      const l = M.parseList(await call(false, '/api/email/messages', { query: { folder, limit: LIST_LIMIT, cursor: from } }));
      page = l.rows; nextCursor = l.next;
      if (page.length === 0) { state.done[folder] = true; break; }
    }
    out.status = stopped ? 'more' : 'done';
    state.syncedAt[folder] = clock;
    await C.saveState(env, state);
    await C.trim(env);
  } catch (e) { out.status = M.words(e).status === 'offline' ? 'offline' : 'error'; }
  try { out.held = (await env.storage.fs.readdir(C.DIR)).filter((n) => n.startsWith('msg-')).length; } catch { out.held = 0; }
  out.note = out.held === 0 ? '' : `${out.held} messages saved on this phone`;
  console.log(`mail: sync ${folder} ${out.status} fetched=${out.fetched} held=${out.held}`);
  return out;
}
