// Imported documents as passages a question can be answered from and a search can find (the Swift LibraryCorpus.swift, deleted). Pure over the phone's own copies: a document whose
// text the phone already holds (it was opened or imported here) is a Doc with one part per section, each keeping the block it starts at, so a hit can say where it is and open there.
// A document the phone has not read is not indexed (the computer's extraction is not asked for every file to build an index). The REGISTRATION is the retrieval lane's
// (retrieval-live.ts `live.setDocSource('imported', ...)` and `registerSearchProvider`, origin/lane/exact-retrieval): until that lands nothing calls `importedDocs`.
import type { Storage } from './app.contract.d.ts';
import type { Doc } from './search.ts';
import * as F from './files.ts';
import * as FM from './files-model.ts';
import * as M from './imported-model.ts';
import { entriesOf } from './imported.ts';

export const MAX_PARTS = 6000;

/** The text kept for a document version, or null. */
async function kept(e: FM.Entry, storage: Storage): Promise<M.DocText | null> {
  try {
    const v = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(`${F.CACHE_DIR}/${FM.hash(`${e.path}|${e.size}|${e.modified ?? ''}`)}/text.json`)));
    return Array.isArray(v?.sections) && Array.isArray(v?.blocks) ? (v as M.DocText) : null;
  } catch { return null; }
}

export function docOf(e: FM.Entry, t: M.DocText): Doc | null {
  const kind = M.kindOfName(e.name);
  if (!kind) return null;
  const parts: NonNullable<Doc['parts']> = [];
  t.sections.forEach((s, i) => {
    const text = t.blocks.slice(s.start, s.start + s.count).filter((b) => b.kind !== 'code' && b.text !== '').map((b) => b.text).join('\n');
    if (text !== '') parts.push({ label: `${kind === 'pdf' ? 'page' : 'chapter'} ${i + 1}${s.title ? ': ' + s.title : ''}`, text, block: s.start });
  });
  if (parts.length === 0) return null;
  return { id: 'imported:' + M.idOf(e.name), kind: 'document', title: M.titleOf(e.name), text: '', date: e.modified, source: 'Imported documents', address: M.addressOf(M.idOf(e.name)), parts, key: `${e.size}|${e.modified ?? ''}` };
}

/** The documents the phone holds the text of, newest first, at most MAX_PARTS parts in all. */
export async function importedDocs(storage: Storage | undefined, net: F.Net = F.realNet): Promise<Doc[]> {
  if (!storage) return [];
  const out: Doc[] = [];
  let budget = MAX_PARTS;
  for (const e of await entriesOf(net).catch(() => [])) {
    if (budget <= 0) break;
    const t = await kept(e, storage);
    const d = t ? docOf(e, t) : null;
    if (d) { out.push(d); budget -= d.parts?.length ?? 0; }
  }
  return out;
}
