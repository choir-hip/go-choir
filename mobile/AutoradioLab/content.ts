// The local content store and its sync with the listener's computer. Owner, 2026-11-16: "we should keep mail on the phone, and we should keep
// everything on the phone as much as we can. It's mostly just text." Every text the phone is shown or can fetch is kept here in full and
// indexed from here (search.ts reads `docs()`). Shared code: it replaces the Swift ContentStore.swift, ContentSync.swift and StoryBodies.swift.
//
// STORAGE: one SQLite file (`app:/data/content.db`), the Exact way (storage.sqlite). The Swift store was one file per item because the Kit
// had no SQLite; the platform's own encryption and backup rules for `app:/data` apply here. Only what a list needs is read for a list.
// BUDGET: total 400 MB, retention by kind, eviction oldest-first in `evictionOrder`; documents the listener wrote and pinned items are never evicted.
//
// PER KIND (list route / body route / rule), as the Swift table said:
//  story     GET /api/platform/publications/front?limit=100 (public) / GET .../publications/resolve?route= / newest first; a story held with the same
//            updated_at is not fetched again; retention 30 days.
//  (mail is NOT synced here: the phone's mail copy is mail-store.ts, sealed; a sync of mail into this plain SQLite file was deleted, review 2026-11-19.)
//  [deleted] GET /api/email/messages?folder=&limit=100&cursor= / GET /api/email/messages/{id} (text_body) / pages newest first; once a folder has been
//            walked (`mail.<folder>.done`) the first all-held page ends it; a killed first walk resumes from the top and skips held bodies.
//  document  GET /api/texture/documents?limit=500&cursor= / GET /api/texture/documents/{id} then /api/texture/revisions/{rid} (the Markdown) / all pages;
//            a document is fetched when its updated_at differs from the held version; never evicted.
//  file      GET /api/files/{dir} / GET /api/files/{path} / breadth first from /, at most 200 folders, text and Markdown under 1 MB.
//  podcast   show notes and transcripts of subscribed shows: the podcasts page's own data, passed in (`fetch.podcasts`, `fetch.transcriptTargets`).
//  runsheet  heard and skipped entries with their outcome, copied from the sheet when it changes (`fetch.runSheet`); retention 90 days.
//
// WAYS THIS CAN FAIL (tests in content.test.ts, fake fetchers and a fake clock):
//  1. Killed mid-sync: the next run skips held items and continues.  2. A failing item (404, bad body): skipped, counted, the run goes on; not retried in
//  the same run.  3. Radio busy: the run stops before the next request.  4. Not signed in / offline: nothing is changed.  5. Two runs at once: the second
//  returns at once.  6. A cancelled run stops at the next item.  7. Logs carry counts and ids, never titles or text.  8. A list page that repeats (a
//  server that ignores the cursor): the walk stops when a cursor repeats.  9. A sign-out during a run: writes after the wipe are dropped.
//
// Time arrives as an argument (epoch milliseconds); nothing here reads a clock.

import { authedJSON, authedBytes, getJSON, SignedOutError, currentWorld } from './api.ts';
import { parseDate } from './paper-model.ts';
import type { Doc } from './search.ts';

export type Kind = 'story' | 'mail' | 'document' | 'file' | 'podcast' | 'transcript' | 'runsheet';
export const KINDS: Kind[] = ['story', 'mail', 'document', 'file', 'podcast', 'transcript', 'runsheet'];
export const KIND_LABEL: Record<Kind, string> = { story: 'Stories', mail: 'Mail', document: 'Documents', file: 'Files', podcast: 'Podcast notes', transcript: 'Transcripts', runsheet: 'Heard' };
export const evictable = (k: Kind): boolean => k !== 'document';

export type StoredItem = {
  id: string; kind: Kind; title: string; text: string; date: number | null;
  /** What opens or plays it: `pub:<route>`, `mail:<id>`, `texture:<id>`, `file:<path>`, a podcast address. */
  address: string; source: string;
  /** An etag or revision: an item with the same version is not fetched again. */
  version: string; pinned: boolean; syncedAt: number;
  /** Kind-specific fields (mail: folder, from, fromAddress, snippet, direction, readAt, trust, to). */
  meta: Record<string, string>;
};
export const newItem = (o: Partial<StoredItem> & Pick<StoredItem, 'id' | 'kind' | 'title' | 'text' | 'address'>): StoredItem =>
  ({ date: null, source: '', version: '', pinned: false, syncedAt: 0, meta: {}, ...o });
export const bytesOf = (i: { text: string; title: string }): number => new TextEncoder().encode(i.text).length + new TextEncoder().encode(i.title).length + 200;

export type Budget = { totalBytes: number; perKindBytes: Partial<Record<Kind, number>>; retentionDays: Partial<Record<Kind, number>>; evictionOrder: Kind[] };
export const STANDARD_BUDGET: Budget = {
  totalBytes: 400_000_000,
  perKindBytes: { story: 40_000_000, mail: 200_000_000, file: 60_000_000, transcript: 40_000_000, podcast: 20_000_000, runsheet: 10_000_000 },
  retentionDays: { story: 30, podcast: 90, transcript: 90, runsheet: 90 },
  evictionOrder: ['story', 'podcast', 'transcript', 'file', 'runsheet', 'mail'],
};

/** What the store needs of Exact's Database (and of a test's). */
export type Db = {
  execute(sql: string, params?: (null | number | string | bigint)[]): Promise<unknown>;
  query(sql: string, params?: (null | number | string | bigint)[]): Promise<{ rows: unknown[][] }>;
};
export const CONTENT_DB = 'app:/data/content.db';

const num = (v: unknown): number => (typeof v === 'bigint' ? Number(v) : typeof v === 'number' ? v : 0);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const COLS = 'id, kind, title, text, date, address, source, version, pinned, synced_at, meta';

function rowToItem(r: unknown[]): StoredItem {
  let meta: Record<string, string> = {};
  try { const m = JSON.parse(str(r[10])); if (m && typeof m === 'object') meta = m as Record<string, string>; } catch { /* a corrupt meta is empty */ }
  return { id: str(r[0]), kind: str(r[1]) as Kind, title: str(r[2]), text: str(r[3]), date: r[4] === null ? null : num(r[4]), address: str(r[5]), source: str(r[6]),
    version: str(r[7]), pinned: num(r[8]) === 1, syncedAt: num(r[9]), meta };
}

/** The stores this session holds, so sign-out can reach them (a sync that is running notices by `generation`; the open db is emptied). */
const liveStores = new Set<ContentStore>();
export async function wipeLiveContent(): Promise<void> { for (const s of [...liveStores]) await s.wipe(); }

export class ContentStore {
  /** Bumped by `wipe`: a sync that started under another generation drops its writes. */
  generation = 0;
  constructor(readonly db: Db, public budget: Budget = STANDARD_BUDGET) { liveStores.add(this); }

  /** The database is closed: sign-out no longer reaches this store. */
  dispose(): void { liveStores.delete(this); }

  async init(): Promise<void> {
    await this.db.execute('CREATE TABLE IF NOT EXISTS items (id TEXT PRIMARY KEY, kind TEXT NOT NULL, title TEXT NOT NULL, text TEXT NOT NULL, date INTEGER, address TEXT NOT NULL, source TEXT NOT NULL, version TEXT NOT NULL, pinned INTEGER NOT NULL, synced_at INTEGER NOT NULL, meta TEXT NOT NULL, bytes INTEGER NOT NULL)');
    await this.db.execute('CREATE INDEX IF NOT EXISTS items_kind ON items (kind)');
    await this.db.execute('CREATE INDEX IF NOT EXISTS items_address ON items (address)');
    await this.db.execute('CREATE TABLE IF NOT EXISTS checkpoints (key TEXT PRIMARY KEY, value TEXT NOT NULL)');
    await this.db.execute('CREATE TABLE IF NOT EXISTS synced (kind TEXT PRIMARY KEY, at INTEGER NOT NULL)');
  }

  async put(i: StoredItem): Promise<void> {
    await this.db.execute(`INSERT OR REPLACE INTO items (${COLS}, bytes) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
      [i.id, i.kind, i.title, i.text, i.date, i.address, i.source, i.version, i.pinned || !evictable(i.kind) ? 1 : 0, i.syncedAt, JSON.stringify(i.meta), bytesOf(i)]);
  }
  async get(id: string): Promise<StoredItem | null> {
    const r = await this.db.query(`SELECT ${COLS} FROM items WHERE id = ?`, [id]);
    return r.rows.length === 0 ? null : rowToItem(r.rows[0]);
  }
  /** Held with this version: a sync does not fetch it again. */
  async has(id: string, version?: string): Promise<boolean> {
    const r = await this.db.query('SELECT version FROM items WHERE id = ?', [id]);
    if (r.rows.length === 0) return false;
    return version === undefined ? true : str(r.rows[0][0]) === version;
  }
  async ids(kind: Kind): Promise<string[]> { return (await this.db.query('SELECT id FROM items WHERE kind = ?', [kind])).rows.map((r) => str(r[0])); }
  async items(kind: Kind): Promise<StoredItem[]> { return (await this.db.query(`SELECT ${COLS} FROM items WHERE kind = ?`, [kind])).rows.map(rowToItem); }
  /** The item that opens at an address (a story at `pub:<route>`, a document at `texture:<id>`, a message at `mail:<id>`). */
  async itemAt(address: string): Promise<StoredItem | null> {
    const r = await this.db.query(`SELECT ${COLS} FROM items WHERE address = ? LIMIT 1`, [address]);
    return r.rows.length === 0 ? null : rowToItem(r.rows[0]);
  }
  async remove(id: string): Promise<void> { await this.db.execute('DELETE FROM items WHERE id = ?', [id]); }

  async checkpoint(key: string): Promise<string | null> {
    const r = await this.db.query('SELECT value FROM checkpoints WHERE key = ?', [key]);
    return r.rows.length === 0 ? null : str(r.rows[0][0]);
  }
  async setCheckpoint(key: string, value: string): Promise<void> { await this.db.execute('INSERT OR REPLACE INTO checkpoints (key, value) VALUES (?, ?)', [key, value]); }
  async markSynced(kind: Kind, now: number): Promise<void> { await this.db.execute('INSERT OR REPLACE INTO synced (kind, at) VALUES (?, ?)', [kind, now]); }
  async lastSynced(kind: Kind): Promise<number | null> {
    const r = await this.db.query('SELECT at FROM synced WHERE kind = ?', [kind]);
    return r.rows.length === 0 ? null : num(r.rows[0][0]);
  }
  /** "last synced 2 hours ago" for a banner (null: never synced). */
  async syncedWords(kind: Kind, now: number): Promise<string | null> {
    const at = await this.lastSynced(kind);
    if (at === null) return null;
    const s = (now - at) / 1000;
    const when = s < 90 ? 'just now' : s < 5400 ? `${Math.floor(s / 60)} minutes ago` : s < 129600 ? `${Math.round(s / 3600)} hours ago` : `${Math.round(s / 86400)} days ago`;
    return `last synced ${when}`;
  }

  async summary(): Promise<{ totalBytes: number; budgetBytes: number; perKind: Partial<Record<Kind, { count: number; bytes: number }>>; line: string }> {
    const r = await this.db.query('SELECT kind, COUNT(*), SUM(bytes) FROM items GROUP BY kind');
    const perKind: Partial<Record<Kind, { count: number; bytes: number }>> = {};
    let total = 0;
    for (const x of r.rows) { const k = str(x[0]) as Kind; perKind[k] = { count: num(x[1]), bytes: num(x[2]) }; total += num(x[2]); }
    const size = (b: number): string => (b >= 1_000_000 ? `${Math.round(b / 1e6)} MB` : `${Math.round(b / 1e3)} KB`);
    const parts = KINDS.filter((k) => (perKind[k]?.count ?? 0) > 0).map((k) => `${perKind[k]!.count} ${KIND_LABEL[k].toLowerCase()} ${size(perKind[k]!.bytes)}`);
    return { totalBytes: total, budgetBytes: this.budget.totalBytes, perKind, line: `${size(total)} of ${size(this.budget.totalBytes)}` + (parts.length ? ': ' + parts.join(', ') : '') };
  }

  /** Drop what is past its retention, then the oldest evictable items (kind order) while over the total or a kind's own cap. Returns the ids dropped. */
  async enforceBudget(now: number): Promise<string[]> {
    const r = await this.db.query('SELECT id, kind, bytes, date, synced_at, pinned FROM items');
    type M = { id: string; kind: Kind; bytes: number; at: number; pinned: boolean };
    const metas: M[] = r.rows.map((x) => ({ id: str(x[0]), kind: str(x[1]) as Kind, bytes: num(x[2]), at: x[3] === null ? num(x[4]) : num(x[3]), pinned: num(x[5]) === 1 }));
    const can = (m: M): boolean => !m.pinned && evictable(m.kind);
    const gone = new Set<string>();
    for (const m of metas) { const days = this.budget.retentionDays[m.kind]; if (can(m) && days !== undefined && now - m.at > days * 86400000) gone.add(m.id); }
    const total = (kind?: Kind): number => metas.filter((m) => !gone.has(m.id) && (kind === undefined || m.kind === kind)).reduce((a, m) => a + m.bytes, 0);
    const oldest = (kind: Kind): M[] => metas.filter((m) => m.kind === kind && can(m) && !gone.has(m.id)).sort((a, b) => a.at - b.at);
    for (const kind of this.budget.evictionOrder) {
      const cap = this.budget.perKindBytes[kind] ?? Infinity;
      for (const m of oldest(kind)) { if (total(kind) <= cap) break; gone.add(m.id); }
    }
    for (const kind of this.budget.evictionOrder) {
      for (const m of oldest(kind)) { if (total() <= this.budget.totalBytes) break; gone.add(m.id); }
    }
    for (const id of gone) await this.remove(id);
    return [...gone];
  }

  /** Sign-out: delete everything. A run in progress notices by `generation`. */
  async wipe(): Promise<void> {
    this.generation++;
    await this.db.execute('DELETE FROM items'); await this.db.execute('DELETE FROM checkpoints'); await this.db.execute('DELETE FROM synced');
  }

  /** Every held item as a search document (`store:<id>`); search.ts keeps those whose key is unchanged. */
  async docs(): Promise<Doc[]> {
    const r = await this.db.query(`SELECT ${COLS} FROM items`);
    return r.rows.map(rowToItem).map(toDoc);
  }
}

export function toDoc(it: StoredItem): Doc {
  return { id: `store:${it.id}`, kind: it.kind, title: it.title, text: it.text, date: it.date, source: it.source === '' ? KIND_LABEL[it.kind] : it.source, address: it.address, key: `${it.version}|${bytesOf(it)}|${it.date ?? 0}` };
}

// ---------------------------------------------------------------------------------------------------------------- the sync

export type SyncReport = { kind: Kind; fetched: number; skipped: number; failed: number; stoppedBy: '' | 'radio' | 'cancelled' | 'error' };
const report = (kind: Kind): SyncReport => ({ kind, fetched: 0, skipped: 0, failed: 0, stoppedBy: '' });

export type DocRow = { id: string; title: string; updated: string };
export type Story = { route: string; title: string; date: number | null };
export type FileEntry = { name: string; path: string; isDirectory: boolean; size: number; modified: number | null };
export type Heard = { id: string; title: string; text: string; state: 'heard' | 'skipped'; ended: string; ref: string; kind: string };

export type Fetchers = {
  documentPage(cursor: string | null): Promise<{ docs: DocRow[]; next: string | null }>;
  documentBody(id: string): Promise<string | null>;
  stories(): Promise<Story[]>;
  storyBody(route: string): Promise<string | null>;
  fileList(path: string): Promise<FileEntry[]>;
  fileText(path: string): Promise<string | null>;
  podcasts(): Doc[];
  transcriptTargets(): { feed: string; id: string; title: string; date: number | null; address: string }[];
  transcript(feed: string, id: string): Promise<string | null>;
  runSheet(): Promise<Heard[] | null>;
  /** True while the radio needs the network (a voice line is on air). Default: never. */
  audioBusy(): Promise<boolean>;
};
export const noFetchers = (): Fetchers => ({
  documentPage: async () => ({ docs: [], next: null }), documentBody: async () => null, stories: async () => [], storyBody: async () => null,
  fileList: async () => [], fileText: async () => null, podcasts: () => [], transcriptTargets: () => [], transcript: async () => null, runSheet: async () => null, audioBusy: async () => false,
});

export const MAX_FOLDERS = 200, MAX_FILE_BYTES = 1_000_000, TRANSCRIPTS_PER_RUN = 20, TRANSCRIPT_DAYS = 14;
const TEXT_EXTENSIONS = new Set(['md', 'markdown', 'txt', 'text', 'csv', 'json', 'rtf']);
const extOf = (name: string): string => { const i = name.lastIndexOf('.'); return i < 0 ? '' : name.slice(i + 1).toLowerCase(); };
export const fileVersion = (e: FileEntry): string => `${e.size}|${e.modified === null ? 0 : e.modified / 1000}`;

export type SyncOptions = { now: number; log?: (line: string) => void };

/** One syncer at a time: a second `syncAll` while one runs returns at once with no reports. */
const running = new WeakSet<ContentStore>();

/** Every kind, in the order the listener feels them: stories, mail, documents, then the slow ones. One report per kind. */
export async function syncAll(store: ContentStore, f: Fetchers, o: SyncOptions): Promise<SyncReport[]> {
  if (running.has(store)) return [];
  running.add(store);
  const started = store.generation;
  const live = (): boolean => store.generation === started;
  const log = o.log ?? ((): void => undefined);
  try {
    const out: SyncReport[] = [];
    const sheet = await f.runSheet();
    if (sheet && live()) { await recordRunSheet(store, sheet, o.now); await store.markSynced('runsheet', o.now); }
    const steps: ((c: Ctx) => Promise<SyncReport>)[] = [syncStories, syncDocuments, syncPodcasts, syncFiles];
    const c: Ctx = { store, f, now: o.now, live };
    for (const step of steps) {
      const r = await step(c);
      out.push(r);
      if (live()) await store.markSynced(r.kind, o.now);
      log(`content sync ${r.kind}: fetched ${r.fetched}, held ${r.skipped}, failed ${r.failed}${r.stoppedBy === '' ? '' : ', stopped: ' + r.stoppedBy}`);
      if (r.stoppedBy === 'radio' || r.stoppedBy === 'cancelled') break;
    }
    if (live()) await store.enforceBudget(o.now);
    return out;
  } finally { running.delete(store); }
}

type Ctx = { store: ContentStore; f: Fetchers; now: number; live: () => boolean };
const put = async (c: Ctx, i: StoredItem): Promise<void> => { if (c.live()) await c.store.put(i); };
/** False (and the report says why) when the run must stop before the next request. */
async function gate(c: Ctx, r: SyncReport): Promise<boolean> {
  if (!c.live()) { r.stoppedBy = 'cancelled'; return false; }
  if (await c.f.audioBusy()) { r.stoppedBy = 'radio'; return false; }
  return true;
}

async function syncStories(c: Ctx): Promise<SyncReport> {
  const r = report('story');
  let list: Story[];
  try { list = await c.f.stories(); } catch { r.stoppedBy = 'error'; return r; }
  for (const s of list.filter((x) => x.route !== '').sort((a, b) => (b.date ?? -Infinity) - (a.date ?? -Infinity))) {
    if (s.date !== null && c.now - s.date > 30 * 86400000) continue; // past retention: not fetched only to be dropped
    const version = s.date === null ? '0' : String(s.date / 1000), id = `pub:${s.route}`;
    if (await c.store.has(id, version)) { r.skipped++; continue; }
    if (!(await gate(c, r))) return r;
    try {
      const text = await c.f.storyBody(s.route);
      if (!text) { r.failed++; continue; }
      await put(c, newItem({ id, kind: 'story', title: s.title, text, date: s.date, address: id, source: 'Autopaper', version, syncedAt: c.now }));
      r.fetched++;
    } catch { r.failed++; }
  }
  return r;
}

async function syncDocuments(c: Ctx): Promise<SyncReport> {
  const r = report('document');
  let cursor: string | null = null;
  const seen = new Set<string>();
  for (;;) {
    if (!(await gate(c, r))) return r;
    let page: { docs: DocRow[]; next: string | null };
    try { page = await c.f.documentPage(cursor); } catch { r.failed++; r.stoppedBy = 'error'; return r; }
    for (const d of page.docs.filter((x) => x.id !== '')) {
      const id = `texture:${d.id}`;
      if (await c.store.has(id, d.updated)) { r.skipped++; continue; }
      if (!(await gate(c, r))) return r;
      try {
        const text = await c.f.documentBody(d.id);
        if (text === null) { r.failed++; continue; }
        await put(c, newItem({ id, kind: 'document', title: d.title === '' ? 'Untitled' : d.title, text, date: parseDate(d.updated), address: id, source: 'Texture', version: d.updated, pinned: true, syncedAt: c.now }));
        r.fetched++;
      } catch { r.failed++; }
    }
    if (!page.next || page.next === '' || seen.has(page.next)) break;
    seen.add(page.next); cursor = page.next;
  }
  return r;
}

async function syncPodcasts(c: Ctx): Promise<SyncReport> {
  const r = report('podcast');
  for (const d of c.f.podcasts().filter((x) => !x.id.startsWith('podcast-tr:'))) {
    const id = `pc:${d.id}`, version = String(new TextEncoder().encode(d.text).length);
    if (await c.store.has(id, version)) { r.skipped++; continue; }
    await put(c, newItem({ id, kind: 'podcast', title: d.title, text: d.text, date: d.date, address: d.address, source: 'Podcasts', version, syncedAt: c.now }));
    r.fetched++;
  }
  const tr = report('transcript');
  for (const t of c.f.transcriptTargets().slice(0, TRANSCRIPTS_PER_RUN)) {
    if (t.date !== null && c.now - t.date > TRANSCRIPT_DAYS * 86400000) continue;
    const id = `tr:${t.feed}|${t.id}`;
    if (await c.store.has(id)) { tr.skipped++; continue; }
    if (!(await gate(c, tr))) { r.stoppedBy = tr.stoppedBy; return r; }
    try {
      const text = await c.f.transcript(t.feed, t.id);
      if (!text) { tr.failed++; continue; }
      await put(c, newItem({ id, kind: 'transcript', title: t.title, text, date: t.date, address: t.address, source: 'Podcasts', version: '1', syncedAt: c.now }));
      tr.fetched++;
    } catch { tr.failed++; }
  }
  r.fetched += tr.fetched; r.skipped += tr.skipped; r.failed += tr.failed;
  return r;
}

async function syncFiles(c: Ctx): Promise<SyncReport> {
  const r = report('file');
  const queue = ['/'];
  let visited = 0;
  while (queue.length > 0 && visited < MAX_FOLDERS) {
    const dir = queue.shift()!; visited++;
    if (!(await gate(c, r))) return r;
    let entries: FileEntry[];
    try { entries = await c.f.fileList(dir); } catch { r.failed++; continue; }
    for (const e of entries) {
      if (e.isDirectory) { queue.push(e.path); continue; }
      if (!TEXT_EXTENSIONS.has(extOf(e.name)) || e.size > MAX_FILE_BYTES) continue;
      const id = `file:${e.path}`, version = fileVersion(e);
      if (await c.store.has(id, version)) { r.skipped++; continue; }
      if (!(await gate(c, r))) return r;
      try {
        const text = await c.f.fileText(e.path);
        if (!text) { r.failed++; continue; }
        await put(c, newItem({ id, kind: 'file', title: e.name, text, date: e.modified, address: id, source: 'Files', version, syncedAt: c.now }));
        r.fetched++;
      } catch { r.failed++; }
    }
  }
  return r;
}

/** Copy the heard and skipped entries, with how they ended, from a sheet snapshot. No request. */
export async function recordRunSheet(store: ContentStore, sheet: Heard[], now: number): Promise<void> {
  for (const e of sheet) {
    const id = `run:${e.id}`;
    if (await store.has(id)) continue;
    await store.put(newItem({ id, kind: 'runsheet', title: e.title, text: [e.title, e.text].filter((x) => x !== '').join('\n'), date: now, address: e.ref === '' ? id : e.ref,
      source: e.state === 'skipped' ? 'Skipped' : 'Heard', version: '1', syncedAt: now, meta: { outcome: e.ended === '' ? e.state : e.ended, kind: e.kind } }));
  }
}

// ---------------------------------------------------------------------------------------------------------------- the real routes

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const s = (o: Record<string, unknown>, k: string): string => (typeof o[k] === 'string' ? (o[k] as string) : '');
const nextOf = (o: Record<string, unknown>): string | null => (typeof o.next_cursor === 'string' && o.next_cursor !== '' ? o.next_cursor : null);

/** The text of a resolved publication: its payload text, else its artifact content, as readable lines (no code, rules or embeds). */
export function storyText(body: unknown): string | null {
  if (!isObj(body)) return null;
  const payload = isObj(body.payload) ? body.payload : {}, artifact = isObj(body.artifact) ? body.artifact : {};
  const raw = s(payload, 'text') !== '' ? s(payload, 'text') : s(artifact, 'content');
  let inCode = false;
  const lines: string[] = [];
  for (const l of raw.split('\n')) {
    const t = l.trim();
    if (t.startsWith('```')) { inCode = !inCode; continue; }
    if (inCode || t === '' || /^([-*_]\s*){3,}$/.test(t)) continue;
    lines.push(t.replace(/^#{1,6}\s+/, ''));
  }
  return lines.length === 0 ? null : lines.join('\n');
}

/** A document's Markdown: the document, then its current revision's content. */
export async function textureSource(id: string, get: (path: string) => Promise<unknown>): Promise<string | null> {
  const doc0 = await get(`/api/texture/documents/${encodeURIComponent(id)}`);
  const doc = isObj(doc0) && isObj(doc0.document) ? doc0.document : doc0;
  const rid = isObj(doc) ? s(doc, 'current_revision_id') : '';
  if (rid === '') return null;
  const rev0 = await get(`/api/texture/revisions/${encodeURIComponent(rid)}`);
  const rev = isObj(rev0) && isObj(rev0.revision) ? rev0.revision : rev0;
  const content = isObj(rev) ? s(rev, 'content') : '';
  return content === '' ? null : content;
}

/** The real routes. Mail, documents and files need the listener's session; a signed-out phone throws `SignedOutError` from the first page, and nothing is changed. */
export function liveFetchers(o: { podcasts?: () => Doc[]; transcriptTargets?: Fetchers['transcriptTargets']; transcript?: Fetchers['transcript']; runSheet?: Fetchers['runSheet']; audioBusy?: Fetchers['audioBusy'] } = {}): Fetchers {
  const j = async (path: string, query: Record<string, string | number> = {}): Promise<unknown> => (await authedJSON(path, { query, world: currentWorld() })).body;
  const base = noFetchers();
  return {
    ...base,
    documentPage: async (cursor) => {
      const b = await j('/api/texture/documents', { limit: 500, ...(cursor ? { cursor } : {}) });
      const o2 = isObj(b) ? b : {};
      const docs = (Array.isArray(o2.documents) ? o2.documents : []).filter(isObj).filter((d) => d.archived_at === undefined || d.archived_at === null)
        .map((d) => ({ id: s(d, 'doc_id'), title: s(d, 'title'), updated: s(d, 'updated_at') })).filter((d) => d.id !== '');
      return { docs, next: nextOf(o2) };
    },
    documentBody: (id) => textureSource(id, j),
    stories: async () => {
      const b = (await getJSON('/api/platform/publications/front', { limit: 100 })).body;
      return (isObj(b) && Array.isArray(b.stories) ? b.stories : []).filter(isObj).map((x) => ({
        route: isObj(x.route) ? s(x.route, 'bare') : '', title: s(x, 'title'), date: parseDate(x.updated_at) })).filter((x) => x.route !== '' && x.title !== '');
    },
    storyBody: async (route) => storyText((await getJSON('/api/platform/publications/resolve', { route })).body),
    fileList: async (path) => {
      const b = await j(path === '/' ? '/api/files' : '/api/files' + path.split('/').map((p) => encodeURIComponent(p)).join('/'));
      return (Array.isArray(b) ? b : []).filter(isObj).filter((x) => s(x, 'name') !== '' && !s(x, 'name').includes('/')).map((x) => ({
        name: s(x, 'name'), path: (path === '/' ? '' : path) + '/' + s(x, 'name'), isDirectory: s(x, 'type') === 'directory', size: typeof x.size === 'number' ? x.size : 0, modified: parseDate(x.modified) }));
    },
    fileText: async (path) => {
      const r = await authedBytes('/api/files' + path.split('/').map((p) => encodeURIComponent(p)).join('/'), { query: { disposition: 'inline' }, world: currentWorld() });
      if (r.bytes.length > MAX_FILE_BYTES) return null;
      return new TextDecoder().decode(r.bytes);
    },
    podcasts: o.podcasts ?? base.podcasts, transcriptTargets: o.transcriptTargets ?? base.transcriptTargets, transcript: o.transcript ?? base.transcript,
    runSheet: o.runSheet ?? base.runSheet, audioBusy: o.audioBusy ?? base.audioBusy,
  };
}

export { SignedOutError };
