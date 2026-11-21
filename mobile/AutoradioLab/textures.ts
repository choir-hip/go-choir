// The Textures place's list, in shared code (it was Swift: TextureReaderSource.swift `TextureList`, answered by the native source `texturesList`). The listener's documents
// from GET /api/texture/documents, with the edition pinned first, and the server's own answer to "which document is my edition" (GET /api/texture/edition) applied to it.
// Nothing here is native: the signed-in client (api.ts) is the whole platform need, and the words for "no key", "locked" and "refused" are the app's one list (words.ts).
// Opening a document (`texture:<id>`) is the shared reader (reader.ts).
import { authedJSON, accountGeneration, SignedOutError, HTTPError, type Reply } from './api.ts';
import { WORDS } from './words.ts';

export type TextureRow = { address: string; title: string; updated: string; meta: string; edition: boolean };
export type TextureList = { status: string; message: string; rows: TextureRow[]; hasEdition: boolean };
export type EditionAnswer = { found: boolean; docID: string; updatedAt: string };
export type Net = { json(path: string, opts?: { query?: Record<string, string | number> }): Promise<Reply> };

const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/** "Nov 9, 2026" in UTC (the Swift shortDate: MMM d, yyyy, en_US_POSIX, UTC); "" when the string is not a date. */
export function shortDate(iso: string): string {
  if (iso === '') return '';
  const t = Date.parse(iso);
  if (!Number.isFinite(t)) return '';
  const d = new Date(t);
  return `${MONTHS[d.getUTCMonth()]} ${d.getUTCDate()}, ${d.getUTCFullYear()}`;
}

const list = (status: string, message = '', rows: TextureRow[] = [], hasEdition = false): TextureList => ({ status, message, rows, hasEdition });
const intOf = (v: unknown): number => (typeof v === 'number' && Number.isFinite(v) ? Math.trunc(v) : 0);

/** The list from GET /api/texture/documents (newest first). The route sends doc_id, title, created_at, updated_at, revision_count (internal/textureowner/texture.go textureDocumentResponse);
 * nothing else is read. A document is the listener's edition when its title starts "My edition" (the server has no marker for it in this list; the edition answer is applied after).
 * Documents with no id are hidden. A body that is not an object with a `documents` array is not a list: it throws, and the load says it could not load. */
export function parseList(o: unknown): TextureList {
  if (!obj(o) || !Array.isArray(o.documents)) throw new Error('the computer sent a list that is not a list');
  const docs = o.documents;
  const rows: TextureRow[] = [];
  for (const d of docs) {
    if (!obj(d)) continue;
    const id = str(d.doc_id);
    if (id === '') continue;
    const title = str(d.title).trim();
    const edition = title.toLowerCase().startsWith('my edition');
    const n = intOf(d.revision_count);
    const when = shortDate(str(d.updated_at));
    const meta = [n > 0 ? `${n} version${n === 1 ? '' : 's'}` : '', when].filter((s) => s !== '').join(' · ');
    rows.push({ address: `texture:${id}`, title: title === '' ? 'Untitled' : title, updated: when, meta, edition });
  }
  const ed = rows.filter((r) => r.edition), rest = rows.filter((r) => !r.edition);
  return list('ready', '', [...ed, ...rest], ed.length > 0);
}

/** `{found:false}` before an edition exists (the normal first-run answer, not an error); found without a document id cannot be opened, so it is not found. Not an object: null. */
export function parseEdition(o: unknown): EditionAnswer | null {
  if (!obj(o)) return null;
  const id = str(o.doc_id);
  if (o.found !== true || id === '') return { found: false, docID: '', updatedAt: '' };
  return { found: true, docID: id, updatedAt: str(o.updated_at) };
}

/** The server's answer to "which document is my edition" applied to the list: that document is pinned first even when it is not titled "My edition" or is older than the page of documents. */
export function applyEdition(l: TextureList, ed: EditionAnswer | null): TextureList {
  if (!ed || !ed.found) return l;
  const address = `texture:${ed.docID}`;
  const rows = l.rows.slice();
  const i = rows.findIndex((r) => r.address === address);
  if (i >= 0) { const [r] = rows.splice(i, 1); rows.unshift({ ...r, edition: true }); }
  else { const when = shortDate(ed.updatedAt); rows.unshift({ address, title: 'My edition', updated: when, meta: when, edition: true }); }
  return { ...l, rows, hasEdition: true };
}

/** The words for a failure. Sign in is said only when signing in is the answer (no key); a locked phone is told to unlock, a refused key to sign in again (the Swift said "Sign in" for all). */
function failure(e: unknown): TextureList {
  if (e instanceof SignedOutError) {
    const m = e.reason === 'locked' ? WORDS.locked : e.reason === 'unknown' ? WORDS.unknown : e.reason === 'refused' ? 'Sign in again to see your documents.' : 'Sign in to see your documents.';
    return list('error', m);
  }
  const why = e instanceof HTTPError ? e.message : e instanceof Error && e.message !== '' ? e.message : 'the computer did not answer';
  return list('error', `Could not load your documents. ${why}`);
}

export async function loadTextures(net: Net): Promise<TextureList> {
  try {
    const [docs, ed] = await Promise.all([
      net.json('/api/texture/documents', { query: { limit: 100 } }),
      net.json('/api/texture/edition').then((r) => parseEdition(r.body), () => null),      // no answer is no edition, not a failed list (the Swift `try?`)
    ]);
    return applyEdition(parseList(docs.body), ed);
  } catch (e) { return failure(e); }
}

const real: Net = { json: (path, opts) => authedJSON(path, opts) };
// The last list, kept in memory for the pages that are not the Textures place (the list stays while the place is behind another page); another account never sees it, and it is dropped
// (not just hidden) the first time anything asks under a new account generation.
let last: { gen: number; list: TextureList } | null = null;
/** Test only: forget the last list. */
export function resetTexturesForTest(): void { last = null; }

/** The source: `at` is the page in front, the contract asks it again after a stop, a restore or a reload (a document's versions change). Only the Textures place asks the computer. */
export async function texturesListShared(at: string, net: Net = real): Promise<TextureList> {
  if (last && last.gen !== accountGeneration()) last = null;
  if (at !== 'textures:') return last ? last.list : list('loading');
  const gen = accountGeneration();
  const l = await loadTextures(net);
  if (gen === accountGeneration()) last = { gen, list: l };
  return l;
}
