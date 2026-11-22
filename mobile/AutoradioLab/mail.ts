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
import { authedJSON, accountGeneration, SignedOutError, scoped, currentWorld, appPath } from './api.ts';
import * as M from './mail-model.ts';
import { sanitized } from './mail-sanitize.ts';
import * as C from './mail-store.ts';
import { sha256, hex } from './mail-seal.ts';

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
/** What a draft may hold when saved from the phone (the server states no limit; a message is sent by the owner from the approval page). */
export const MAX_DRAFT_SUBJECT = 998;
export const MAX_DRAFT_BODY = 200_000;
/** Bodies fetched by one catch-up answer. A bound on one turn's work, not a limit on the listener: the next open of the mailbox goes on from where this stopped. */
export const SYNC_BODIES = 25;
const OFFLINE_NOTE = 'Offline. Showing mail saved on this phone.';

// ---- memory (one session)

type Held = { rows: M.Summary[]; total: number; unread: number; next: string };
const S = scoped(() => ({ kept: new Map<string, Held>(), autoRead: new Set<string>(), drafts: [] as M.DraftInfo[], savedDrafts: new Map<string, string>(), alias: null as string | null, clock: 0, started: false }));      // module state belongs to the world (api.ts `scoped`)

/** Forget what this session holds. */
export function forget(): void { S.reset(); }

/** A source starts under one account generation (api.ts); a sign-out moves it. Everything a source does with a reply after the await (memory, the sealed
 * files) checks it first, so a reply that arrived just before the sign-out cannot write the old account's mail into the wiped stores (review 2026-11-19). */
const stamped = (env: C.Env): C.Env => ({ ...env, gen: accountGeneration() });
const live = (env: C.Env): void => { if (env.gen !== undefined && env.gen !== accountGeneration()) throw new SignedOutError('signed out while the mail was being read'); };
/** Between two bodies of a long answer: lets the next turn start (Exact gives a turn about 100 ms of CPU; Bun has no scheduler). */
const yieldTurn = async (): Promise<void> => { const sc = (globalThis as { scheduler?: { yield?: () => Promise<void> } }).scheduler; if (sc?.yield) await sc.yield(); };

// ---- the computer

class StatusError extends Error { status: number; constructor(status: number, message: string) { super(`HTTP ${status}: ${message}`); this.name = 'StatusError'; this.status = status; } }
type Call = { method?: 'GET' | 'POST'; query?: Record<string, string | number>; body?: unknown };

async function call(path: string, opts: Call = {}): Promise<unknown> {
  const r = await authedJSON(path, { ...opts, world: currentWorld() });
  if (r.serverTime !== null && r.serverTime > 0) S().clock = r.serverTime;
  return r.body;
}

/** "now" for dates: the server's clock, else the newest date the lists hold. */
function nowOf(rows: M.Summary[] = []): number {
  if (S().clock > 0) return S().clock;
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
    const c = S().kept.get('inbox');
    return { name: f.name, label: f.label, address: 'mailbox:' + f.name, current: f.name === v.folder, badge: f.name === 'inbox' && c && c.unread > 0 ? String(c.unread) : '' };
  });
  return v;
}

async function ensureKept(env: C.Env, folder: string): Promise<Held | null> {
  const k = S().kept.get(folder);
  if (k) return k;
  const c = await C.loadFolder(env, folder);
  if (!c) return null;
  const h: Held = { rows: c.rows, total: c.total, unread: c.unread, next: '' };
  S().kept.set(folder, h);
  return h;
}

/** The last list this session fetched, or the phone's saved one, answered at once: a return visit shows it and refreshes underneath. */
export async function mailFrame(address: string, env: C.Env): Promise<BoxView> {
  const v = frame(address);
  if (!address.startsWith('mailbox:') || v.folder === 'drafts') { v.status = 'frame'; return v; }
  const h = await ensureKept(env, v.folder);
  if (!h) { v.status = 'frame'; return v; }
  if (v.folder === 'inbox') for (const f of v.folders) if (f.name === 'inbox') f.badge = h.unread > 0 ? String(h.unread) : '';
  v.rows = M.threadRows(h.rows, nowOf(h.rows)); v.status = 'cached';
  return v;
}

function savedWords(copyAt: number): string {
  const n = S().clock;
  if (n <= 0 || copyAt <= 0) return OFFLINE_NOTE;
  const s = (n - copyAt) / 1000;
  const when = s < 90 ? 'just now' : s < 5400 ? `${Math.floor(s / 60)} minutes ago` : s < 129600 ? `${Math.round(s / 3600)} hours ago` : `${Math.round(s / 86400)} days ago`;
  return `Offline. Showing mail saved on this phone, last synced ${when}.`;
}

export async function mailBox(address: string, env: C.Env): Promise<BoxView> {
  if (!address.startsWith('mailbox:')) { const i = emptyBox(); i.status = 'idle'; return i; }
  const v = frame(address);
  env = stamped(env);
  try {
    if (v.folder === 'drafts') {
      const o = (await call('/api/email/drafts', {})) as { drafts?: unknown[] } | null;
      live(env);
      S().drafts = (o?.drafts ?? []).map(M.parseDraft);
      v.rows = S().drafts.map((d) => ({
        id: d.id, address: 'mail:draft/' + d.id, sender: 'To ' + (d.to[0] ?? '(nobody yet)'), subject: d.subject === '' ? '(no subject)' : d.subject, snippet: d.text.slice(0, 120),
        stamp: '', trust: '', direction: 'draft', unread: false, hasAttachments: false, count: 1,
      }));
      v.total = S().drafts.length;
    } else {
      const o = await call('/api/email/messages', { query: { folder: v.folder, limit: LIST_LIMIT } });
      const l = M.parseList(o);
      live(env);
      S().kept.set(v.folder, { rows: l.rows, total: l.total, unread: l.unread, next: l.next });
      await C.saveFolder(env, { folder: v.folder, rows: l.rows, total: l.total, unread: l.unread, savedAt: S().clock });
      v.rows = M.threadRows(l.rows, nowOf(l.rows)); v.total = l.total; v.unread = l.unread;
      if (v.folder === 'inbox') for (const f of v.folders) if (f.name === 'inbox') f.badge = l.unread > 0 ? String(l.unread) : '';
    }
    if (v.rows.length === 0) { v.status = 'empty'; v.message = M.emptyWords(v.folder); } else v.status = 'ready';
    console.log(`mail: box ${v.folder} rows=${v.rows.length}`);
  } catch (e) {
    const w = M.words(e);
    v.status = w.status; v.message = w.message;
    console.log(`mail: box ${v.folder} failed ${w.status}`);
    const inMemory = S().kept.has(v.folder);
    const h = await ensureKept(env, v.folder);          // failure 3: keep the last good rows under the banner
    if (h && v.folder !== 'drafts') {
      v.rows = M.threadRows(h.rows, nowOf(h.rows)); v.total = h.total; v.unread = h.unread; v.cached = true;
      if (v.status === 'offline' && !inMemory && v.rows.length > 0) v.message = savedWords((await C.loadFolder(env, v.folder))?.savedAt ?? 0);   // never opened this session: the phone still holds the mail
    }
  }
  return v;
}

// ---- a message

const originalName = async (id: string): Promise<string> => `mail-${hex((await sha256(new TextEncoder().encode(id))).subarray(0, 16))}.html`;

function allKept(): M.Summary[] { return [...S().kept.values()].flatMap((h) => h.rows); }

async function setRead(id: string, read: boolean): Promise<boolean> {
  try {
    await call(`/api/email/messages/${M.segment(id)}/${read ? 'read' : 'unread'}`, { method: 'POST' });
    for (const h of S().kept.values()) {
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

async function loadMessage(address: string, wantOriginal: boolean, env: C.Env): Promise<Loaded> {
  const v = emptyMessage();
  if (!address.startsWith('mail:')) { v.status = 'idle'; return { view: v, detail: null }; }
  const id = M.rest(address);
  if (id === '' || id === 'new' || id.startsWith('reply/') || id.startsWith('draft/')) { v.status = 'idle'; return { view: v, detail: null }; }
  v.id = id; v.address = address;
  env = stamped(env);
  if (!S().started) { S().started = true; await C.forgetOldCopy(env.storage); }      // documents a past session left in the cache
  try {
    let d: M.Detail;
    try {
      d = M.parseDetail(await call('/api/email/messages/' + M.segment(id), {}));
      live(env);
      const folder = [...S().kept.entries()].find(([, h]) => h.rows.some((r) => r.id === id))?.[0] ?? (await C.loadMessage(env, id))?.folder ?? 'inbox';
      await C.saveMessage(env, { id, folder, summary: d.summary, text: d.text, to: d.to, cc: d.cc, replyTo: d.replyTo, attachments: d.attachments, savedAt: S().clock });
    } catch (e) {
      const held = await C.loadMessage(env, id);       // no network: the message the phone holds
      if (!held || M.words(e).status !== 'offline') throw e;
      d = { summary: held.summary, text: held.text, html: '', replyTo: held.replyTo, to: held.to, cc: held.cc, attachments: held.attachments };
      v.message = savedWords(held.savedAt);
    }
    if (!S().kept.has('inbox')) { try { const l = M.parseList(await call('/api/email/messages', { query: { folder: 'inbox', limit: LIST_LIMIT } })); live(env); S().kept.set('inbox', { rows: l.rows, total: l.total, unread: l.unread, next: l.next }); } catch { await ensureKept(env, 'inbox'); } }
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
    const rel = v.hasOriginal ? `${C.CACHE}/${name}` : '';              // the module's path (storage re-roots it); the page and its iframe get the host's (appPath)
    v.originalPath = v.hasOriginal ? appPath(rel) : '';
    // The sender's own layout is written only while it is asked for, and every other document is removed: the message's HTML sits on the phone, readable, for as short a time as it can.
    if (v.hasOriginal && !(await C.sealed(env))) { v.hasOriginal = false; v.originalPath = ''; }      // no cipher on this platform: no plain copy of a message is made
    if (v.hasOriginal && wantOriginal && env.storage) {
      // The one mail write with a plain file: it carries the account stamp like the sealed ones. A sign-out during the awaits blanks the path and removes the file.
      const stale = (): boolean => env.gen !== accountGeneration();
      await C.forgetOldCopy(env.storage, name);
      if (stale()) { v.hasOriginal = false; v.originalPath = ''; }
      else {
        try {
          await env.storage.fs.atomicWriteFile(rel, new TextEncoder().encode(sanitized(d.html)));
          if (stale()) { try { await env.storage.fs.rm(rel); } catch { /* the wipe will */ } v.hasOriginal = false; v.originalPath = ''; }
        } catch { v.hasOriginal = false; v.originalPath = ''; }
      }
    } else { await C.forgetOldCopy(env.storage); }
    v.attachments = d.attachments.map((a) => {
      const filename = M.safeFilename(a.filename);
      // Failure 9: the web does not offer an inbound download either, so the row is honest about it.
      return {
        id: a.id, filename, size: M.fileSize(a.size), kind: M.attachmentKind(a.filename, a.contentType), available: false,
        address: '', note: 'Saving attachments from mail is not offered yet.',
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
    if (M.isUnread(s) && !S().autoRead.has(id)) { S().autoRead.add(id); await setRead(id, true); }
    console.log(`mail: message ready blocks=${v.blocks.length} attachments=${v.attachments.length}`);
    return { view: v, detail: d };
  } catch (e) {
    const w = M.words(e);
    v.status = w.status; v.message = w.message;
    console.log(`mail: message failed ${w.status}`);
    return { view: v, detail: null };
  }
}

export async function mailMessage(address: string, wantOriginal: boolean, env: C.Env): Promise<MessageView> {
  return (await loadMessage(address, wantOriginal, env)).view;
}

// ---- compose (a reply, a new message, an existing draft)

const emptyCompose = (): ComposeView => ({ status: '', forAddress: '', sendAs: '', toLine: '', subject: '', body: '', draftId: '', note: '', reviewURL: '', replyTo: '' });

async function aliasAddress(): Promise<string> {
  const s = S();
  if (s.alias) return s.alias;
  const o = (await call('/api/email/aliases', {})) as { aliases?: { address?: unknown }[] } | null;
  const a = o?.aliases?.[0]?.address;
  if (typeof a !== 'string' || a === '') throw new StatusError(409, 'You have no sending address yet.');
  s.alias = a;
  return a;
}

export async function mailCompose(address: string, env: C.Env): Promise<ComposeView> {
  const v = emptyCompose();
  if (!address.startsWith('mail:')) { v.status = 'idle'; return v; }
  const rest = M.rest(address);
  v.forAddress = address;
  if (rest === 'new') { v.status = 'seed'; return v; }
  if (rest.startsWith('draft/')) {
    const id = rest.slice(6);
    try {
      const d = M.parseDraft(await call('/api/email/drafts/' + M.segment(id), {}));
      v.status = 'saved'; v.draftId = d.id; v.sendAs = d.from; v.toLine = d.to.join(', '); v.subject = d.subject; v.body = d.text;
      v.note = 'Saved. Sending needs your approval.';
    } catch (e) { v.status = 'error'; v.note = M.words(e).message; }
    return v;
  }
  if (!rest.startsWith('reply/')) { v.status = 'idle'; return v; }
  const id = rest.slice(6);
  const { view: m, detail: d } = await loadMessage('mail:' + id, false, env);
  if (m.status !== 'ready' || !d) { v.status = 'error'; v.note = m.message === '' ? 'Could not open the message to reply to.' : m.message; return v; }
  v.toLine = d.summary.direction === 'outbound' ? M.addressOnly(d.to[0] ?? '') : d.replyTo === '' ? d.summary.fromAddress : d.replyTo;
  v.subject = M.replySubject(d.summary.subject);
  v.body = M.replyBody(m.date, m.sender, m.blocks);
  v.replyTo = id; v.status = 'seed';
  return v;
}

export async function mailSaveDraft(address: string, to: string, subject: string, body: string): Promise<ComposeView> {
  const v = emptyCompose();
  v.forAddress = address; v.toLine = to; v.subject = subject; v.body = body;
  const rest = M.rest(address);
  if (rest.startsWith('reply/')) v.replyTo = rest.slice(6);
  const p = M.problem(to, body);
  if (p !== null) { v.status = 'error'; v.note = p; return v; }          // refused here, no request made
  if (subject.length > MAX_DRAFT_SUBJECT || body.length > MAX_DRAFT_BODY) { v.status = 'error'; v.note = 'This draft is too long to save from the phone. Shorten it and try again.'; return v; }
  const same = `${to}\u0000${subject}\u0000${body}`;
  const already = S().savedDrafts.get(same);
  if (already !== undefined) { v.status = 'saved'; v.draftId = already; v.note = 'Saved as a draft. It has not been sent.'; return v; }   // the server has no update route, so the same text is not saved twice
  try {
    const from = await aliasAddress();
    v.sendAs = from;
    const req: Record<string, unknown> = { from_address: from, subject, text_body: body, to_addresses: M.recipients(to) };
    if (v.replyTo !== '') { req.reply_to_message_id = v.replyTo; req.source_kind = 'mail'; req.source_ref = v.replyTo; }
    const d = M.parseDraft(await call('/api/email/drafts', { method: 'POST', body: req }));
    v.status = 'saved'; v.draftId = d.id; S().savedDrafts.set(same, d.id);
    v.note = 'Saved as a draft. It has not been sent.';
    console.log('mail: draft saved');
  } catch (e) { v.status = 'error'; v.note = M.words(e).message; }
  return v;
}

/** Sending is an owner act the server does not accept from a key (POST /api/email/drafts/{id}/send answers 403, "owner session required"), so it is never called:
 * the phone's way is the approval email, and the listener sends from the page it links to. */
export async function mailApprovalEmail(address: string, draft: string): Promise<ComposeView> {
  const v = emptyCompose();
  v.forAddress = address; v.draftId = draft;
  try {
    const o = (await call(`/api/email/drafts/${M.segment(draft)}/approval-email`, { method: 'POST', body: {} })) as { review_url?: unknown } | null;
    v.status = 'approval'; v.reviewURL = typeof o?.review_url === 'string' ? o.review_url : '';
    v.note = 'An approval link was emailed to you. Open it in your browser to read the draft and send it.';
  } catch (e) {
    const w = M.words(e);
    v.status = 'saved'; v.note = M.statusOf(e) === 403 ? 'This phone cannot ask for an approval link yet.' : w.message;
  }
  return v;
}

/** "Summarise this thread", "Reply to ...": the prompt aimed at a message. The words it asks go to the radio as a turn aimed at the page (the contract's `choirAsk`); this is the page's own
 * acknowledgement. Nothing is posted here and the words are not logged. */
export function mailAsk(address: string, question: string): Ack {
  return address === '' || question.trim() === '' ? { status: 'error', message: 'There is nothing to ask.' } : { status: 'asked', message: `Asked: ${question}` };
}

export async function mailMarkUnread(address: string): Promise<Ack> {
  const ok = await setRead(M.rest(address), false);
  return { status: ok ? 'ok' : 'error', message: ok ? '' : 'Could not mark it unread.' };
}

// ---- the phone's copy: the catch-up

/** Keeps the phone's copy of the folder in front: the text of every message in the newest page that is not held yet, then older pages while the folder has not been
 * walked, SYNC_BODIES bodies an answer. Counts only; a failure ends the answer and the next open goes on. Drafts live on the computer and are not copied (a draft is
 * one request). */
export async function mailSync(address: string, env: C.Env): Promise<SyncView> {
  const out: SyncView = { status: '', held: 0, fetched: 0, note: '' };
  if (!address.startsWith('mailbox:')) { out.status = 'idle'; return out; }
  env = stamped(env);
  const folder = M.rest(address);
  if (!M.FOLDERS.some((f) => f.name === folder) || folder === 'drafts' || !env.storage || !(await C.sealed(env))) { out.status = 'idle'; return out; }      // no cipher: no body is fetched to be thrown away
  const h = S().kept.get(folder) ?? (await ensureKept(env, folder));
  if (!h) { out.status = 'idle'; return out; }
  const verdict = await C.validate(env);
  if (verdict === 'cannot') { out.status = 'idle'; return out; }        // the key cannot be read now (a locked Keychain): nothing is judged, nothing is marked held
  const state = verdict === 'purged' ? { done: {}, syncedAt: {}, cursor: {} } : await C.loadState(env);
  let budget = SYNC_BODIES;
  let page = h.rows, nextCursor = h.next, first = true, stopped = false;
  try {
    for (;;) {
      for (const r of page) {
        if (r.id === '' || (await C.holds(env, r.id))) continue;
        if (budget === 0) { stopped = true; break; }
        const d = M.parseDetail(await call('/api/email/messages/' + M.segment(r.id), {}));
        live(env);
        await C.saveMessage(env, { id: r.id, folder, summary: d.summary, text: d.text, to: d.to, cc: d.cc, replyTo: d.replyTo, attachments: d.attachments, savedAt: S().clock });
        budget -= 1; out.fetched += 1;
        await yieldTurn();
      }
      if (stopped) break;
      if (!first) { state.cursor[folder] = nextCursor; if (nextCursor === '') state.done[folder] = true; }
      if (state.done[folder]) break;
      const from = first ? (state.cursor[folder] ?? '') || nextCursor : nextCursor;
      if (from === '') { state.done[folder] = true; break; }
      first = false;
      const l = M.parseList(await call('/api/email/messages', { query: { folder, limit: LIST_LIMIT, cursor: from } }));
      page = l.rows; nextCursor = l.next;
      if (page.length === 0) { state.done[folder] = true; break; }
    }
    out.status = stopped ? 'more' : 'done';
    state.syncedAt[folder] = S().clock;
    await C.saveState(env, state);
    if (out.fetched > 0) await C.trim(env);                 // the copy only grows when a body was saved: the files are not walked on a sync that saved none
  } catch (e) { out.status = M.words(e).status === 'offline' ? 'offline' : 'error'; }
  try { out.held = (await env.storage.fs.readdir(C.DIR)).filter((n) => n.startsWith('msg-')).length; } catch { out.held = 0; }
  out.note = out.held === 0 ? '' : `${out.held} messages saved on this phone`;
  console.log(`mail: sync ${folder} ${out.status} fetched=${out.fetched} held=${out.held}`);
  return out;
}
