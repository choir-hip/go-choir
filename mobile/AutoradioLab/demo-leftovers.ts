// What demo mode left in the SHARED files of a phone that was in it. Demo mode was deleted on 2026-11-20; this runs once per phone, on the first answer after a launch
// (app.ts), and removes only what demo mode demonstrably wrote: positive trace only, never a real item. The native half (open-pages.json, signals, the told-the-paper acts,
// the demo theme, launch flags) is AutoradioKit's Demo.clearMoreLeftovers. Two files hold demo's trace here:
//   recent-files.json   an entry is demo's only when its path, its byte size AND its modified time are the demo tree's: the demo backend stamped every node with a time fixed
//                       from the clock 1_700_000_000_000 (Nov 2023) minus a set number of days, which a real file does not share together with the path and size
//   open-pages-shared.json   an address is demo's only when it is one of the ids demo mode served (the five stories, five Texture documents, four mailbox states, the demo
//                            messages, the sample book, a demo message's attachment path)
// "Once": ONE attempt per process (a shared promise), run by app.ts before any source answers, so pages.ts and files.ts have not loaded their state yet and read what is on disk
// after it. The marker is written AFTER the removals; a failed write leaves it unset and the next launch retries (that process loads the uncleaned file again, so nothing in memory
// can undo a cleaned file within a process). An unreadable file is never rewritten. Sign-out removes the marker
// with the rest (account.ts), which only means the pass may run once more, and finds nothing.
import type { Storage } from './app.contract.d.ts';
import { RECENT_FILE } from './files.ts';
import { PAGES_FILE } from './pages.ts';
import { currentWorld } from './api.ts';

export const DEMO_MARKER = 'app:/data/demo-leftovers-cleared';

const STORIES = new Set(['demo-ev-charging', 'demo-grid-storage', 'demo-harbor-barrier', 'demo-library-hours', 'demo-river-festival']);
const TEXTURES = new Set(['demo-notes', 'demo-draft', 'demo-edition', 'demo-letter', 'demo-notebook']);
const MAILBOXES = new Set(['demo-empty', 'demo-loading', 'demo-offline', 'demo-error']);
const MESSAGES = new Set(['m1', 'm2', 'm3', 'm4', 'm5', 'm6', 'q1', 's1', 'demo-draft-1', 'demo-draft-new']);
const SAMPLE_BOOK = '6916a7c31cd3';
/** The demo file tree: path -> [byte size, age in days]. The demo backend gave every node `modified` = 1_700_000_000_000 - age * 86_400_000 (a file added by the demo upload is
 * "Added note.txt", 37 bytes, age 0, in any folder). */
const DEMO_NOW = 1_700_000_000_000, DAY = 86_400_000;
const FILES: Record<string, [number, number]> = {
  '/Books/The Lantern Keeper.epub': [2728, 20], '/Documents/Budget 2026.csv': [289, 3], '/Documents/Council minutes.pdf': [2553, 1], '/Documents/Harbor notes.md': [448, 0],
  '/Documents/Key list.txt': [274, 40], '/Documents/Lamps.swift': [245, 9], '/Documents/Survey readings.dat': [512, 12], '/Media/Pattern.mp4': [3611, 8],
  '/Media/Tone 440.wav': [12044, 7], '/Pictures/Harbor test card.png': [2844, 5], '/Readme.txt': [55, 30],
};

const idOf = (rest: string): string => rest.replace(/^\/+/, '').split(/[#@]/)[0];

/** An address only demo mode could have opened (by what follows the scheme). */
export function isDemoAddress(address: string): boolean {
  const i = address.indexOf(':');
  if (i < 0) return false;
  const scheme = address.slice(0, i), rest = address.slice(i + 1);
  switch (scheme) {
    case 'pub': return STORIES.has(idOf(rest));
    case 'texture': return TEXTURES.has(idOf(rest));
    case 'mailbox': return MAILBOXES.has(idOf(rest));
    case 'mail': return MESSAGES.has(idOf(rest));
    case 'doc': return idOf(rest) === SAMPLE_BOOK;
    case 'file': { const m = /^\/Mail\/([^/]+)\//.exec(rest); return m !== null && MESSAGES.has(m[1]); }
    default: return false;
  }
}

/** A recent row is demo's only on the exact path, size AND modified time of a node of the demo tree (or of the demo's "Added note.txt": that name, 37 bytes, the demo clock). */
export function isDemoFile(path: string, size: number, modified: unknown): boolean {
  const f = FILES[path];
  if (f) return f[0] === size && modified === DEMO_NOW - f[1] * DAY;
  return /\/Added note\.txt$/.test(path) && size === 37 && modified === DEMO_NOW;
}

type Outcome = 'clean' | 'rewritten' | 'failed';
async function rewrite(storage: Storage, file: string, change: (v: unknown) => unknown | undefined): Promise<Outcome> {
  let raw: unknown;
  try { raw = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(file))); } catch { return 'clean'; }          // absent or unreadable: nothing was read, nothing is written
  const next = change(raw);
  if (next === undefined) return 'clean';
  try { await storage.fs.atomicWriteFile(file, new TextEncoder().encode(JSON.stringify(next))); return 'rewritten'; } catch { return 'failed'; }
}

let attempt: { w: unknown; p: Promise<void>; done: boolean } | null = null;
/** One attempt per process: every caller gets the same promise. (For tests: `resetDemoLeftovers`.) */
export function clearDemoLeftovers(storage: Storage | undefined, w: unknown = currentWorld()): Promise<void> {
  // Once per process; but an attempt whose world closed before it finished never ends (api.ts, the world), so the next world's runs it again.
  if (!attempt || (attempt.w !== w && !attempt.done)) { const a = { w, p: run(storage).then(() => { a.done = true; }), done: false }; attempt = a; }
  return attempt.p;
}
export function resetDemoLeftovers(): void { attempt = null; }

async function run(storage: Storage | undefined): Promise<void> {
  if (!storage) return;
  try { await storage.fs.readFile(DEMO_MARKER); return; } catch { /* not yet */ }
  const recent = await rewrite(storage, RECENT_FILE, (v) => {
    if (!Array.isArray(v)) return undefined;
    const keep = v.filter((r) => !(r && typeof r.path === 'string' && typeof r.size === 'number' && isDemoFile(r.path, r.size, r.modified)));
    return keep.length === v.length ? undefined : keep;
  });
  const pages = await rewrite(storage, PAGES_FILE, (v) => {
    const o = (typeof v === 'object' && v !== null ? v : null) as { pages?: unknown; recency?: unknown } | null;
    if (!o || !Array.isArray(o.pages) || !Array.isArray(o.recency)) return undefined;
    const p = o.pages.filter((x: { address?: unknown }) => !(typeof x?.address === 'string' && isDemoAddress(x.address)));
    const r = o.recency.filter((a) => !(typeof a === 'string' && isDemoAddress(a)));
    return p.length === o.pages.length && r.length === o.recency.length ? undefined : { ...o, pages: p, recency: r };
  });
  if (recent === 'failed' || pages === 'failed') return;
  try { await storage.fs.atomicWriteFile(DEMO_MARKER, new Uint8Array([49])); } catch { /* retried at the next launch */ }
}
