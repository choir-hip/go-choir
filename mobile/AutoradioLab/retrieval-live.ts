// The two live sources of the retrieval runtime (retrieval-run.ts), wired to the signed-in seam. Kept apart from retrieval-run.ts because api.ts
// imports app.ts, which imports this file: the runtime and its tests must not sit in that cycle.
//   retrievalServe  the voice turn's ask (native raises it: retrievalAsk, interface/retrieval.json) answered through the native op `retrievalAnswer`.
//                   With no ask (id "") it only starts the warm-up: the index is built and the vectors are kept up to date in the background.
//   paperSearch     the launcher's paper-wide search (the server's public route), merged after the local rows.
import type { Storage } from './app.contract.d.ts';
import { getJSON, nativeInert, accountGeneration, scoped } from './api.ts';
import { Retrieval, nativeEmbedder, parseAsk, launcherSearch, registerSearchProvider, type Found } from './retrieval-run.ts';
import type { Doc, Row } from './search.ts';
import { contentDocs } from './content-run.ts';
import { importedDocs } from './imported-search.ts';

/** The one runtime of the WORLD (api.ts `scoped`): its index, its vectors, its warm-up and the copy of the imported titles are the account's, so another world never meets them. The document sources register inside it. */
const R = scoped(() => {
  const live = new Retrieval();
  // The document sources: the phone's content store (content-run.ts) and the imported documents the phone holds the text of (imported-search.ts). Both read through the runtime's storage.
  live.setDocSource('store', () => contentDocs(live.storage));
  const st = { live, warmed: false, importedHeld: [] as Doc[] };
  live.setDocSource('imported', async () => {
    const gen = accountGeneration();
    const docs = await importedDocs(live.storage);
    if (gen !== accountGeneration()) return [];
    return (st.importedHeld = docs);
  });
  return st;
});
export const live = (): Retrieval => R().live;
// The launcher's local rows for imported documents: their titles, from the copy the index last read (the index is built before any search is useful). The provider is a function; the copy it reads is the world's.
registerSearchProvider('imported', (q) => {
  const w = q.toLowerCase().split(/\s+/).filter((x) => x !== '');
  return w.length === 0 ? [] : R().importedHeld.filter((d) => w.every((x) => d.title.toLowerCase().includes(x))).map((d) => ({ address: d.address, title: d.title, kind: 'document', detail: 'Imported document' }));
});

export async function retrievalServe(a: { id: string; words: string; pageTitle: string; pageAddress: string; now: number; facts: string }, storage: Storage | undefined): Promise<{ id: string; ok: boolean }> {
  const st = R(), live = st.live;
  live.storage = storage ?? live.storage;
  const later = nativeInert();
  live.native = later ? nativeEmbedder(later as never) : null;
  if (!st.warmed) { st.warmed = true; live.kick(true); }
  const ask = parseAsk(a.id, a.words, a.pageTitle, a.pageAddress, a.now, a.facts);
  if (!ask || !later) return { id: a.id, ok: false };
  return live.answer(ask, later as never);
}

const row = (o: unknown): Row | null => {
  if (o === null || typeof o !== 'object') return null;
  const r = o as Record<string, unknown>;
  return typeof r.address === 'string' && typeof r.title === 'string' ? { address: r.address, title: r.title, kind: typeof r.kind === 'string' ? r.kind : '', detail: typeof r.detail === 'string' ? r.detail : '' } : null;
};
export const PAUSE_MS = 350;
export async function paperSearch(q: string, local: unknown): Promise<Found> {
  const rows = Array.isArray(local) ? local.map(row).filter((r): r is Row => r !== null) : [];
  return launcherSearch(typeof q === 'string' ? q : '', rows, async (path, query) => (await getJSON(path, query)).body, { pause: PAUSE_MS });
}
