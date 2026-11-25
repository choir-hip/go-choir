// The two live sources of the retrieval runtime (retrieval-run.ts), wired to the signed-in seam. Kept apart from retrieval-run.ts because api.ts
// imports app.ts, which imports this file: the runtime and its tests must not sit in that cycle.
//   retrievalServe  the voice turn's ask (native raises it: retrievalAsk, interface/retrieval.json) answered through the native op `retrievalAnswer`. The ask's facts carry only the run sheet;
//                   the open pages, the topics and the page's text are shared code's own (`sharedFacts`).
//                   With no ask (id "") it only starts the warm-up: the index is built and the vectors are kept up to date in the background.
//   paperSearch     the launcher's paper-wide search (the server's public route), merged after the local rows.
import type { Storage } from './app.contract.d.ts';
import { getJSON, nativeLater, accountGeneration } from './api.ts';
import { modelsAgreement } from './consent.ts';
import { onKickSearch, onTestSearch } from './search-kick.ts';
import { clock } from './clock.ts';
import { Retrieval, nativeEmbedder, parseAsk, launcherSearch, registerSearchProvider, type Found } from './retrieval-run.ts';
import { onSignOut, type Row } from './search.ts';
import { contentDocs } from './content-run.ts';
import { importedDocs } from './imported-search.ts';
import { ImportedSource } from './imported-source.ts';
import { pagesView } from './pages.ts';
import { picksText } from './edition-picks.ts';
import { heldText } from './reader.ts';
import type { Facts } from './retrieval-run.ts';

/** The one runtime of the process. Document sources register here (`live.setDocSource`: the content store and the imported documents, both wired just below). */
export const live = new Retrieval({ mayDownload: () => modelsAgreement(live.storage, 'search') });
onKickSearch((storage) => { live.storage = storage ?? live.storage; live.kick(); });
let warmed = false;
// The Test button of the search model (models.ts): the fixed query over the phone's own index, as an ask with no page and no run sheet. It reads what is on the phone and downloads nothing.
onTestSearch(async (query) => {
  const c = await live.serve({ id: 'trial', words: query, pageTitle: '', pageAddress: '', now: clock.ms, facts: { runSheet: [], pages: [], topics: [], pageText: '' } });
  const n = c.items.length;
  return n === 0 ? 'It found nothing yet: the index of your stories is still being built, or there is nothing to find.' : `It found ${n} ${n === 1 ? 'story or document' : 'stories and documents'} on this phone for: ${query}`;
});

// The document sources: the phone's content store (content-run.ts) and the imported documents the phone holds the text of (imported-search.ts). Both read through the runtime's storage.
live.setDocSource('store', () => contentDocs(live.storage));
const importedSource = new ImportedSource(() => importedDocs(live.storage), () => accountGeneration(), () => live.kick());   // never awaited on an ask: the folder list is a network read (imported-search.ts)
live.setDocSource('imported', () => importedSource.docs());
// Sign-out: the copy of the imported titles goes, and the warm-up runs again for the next account (the runtime's own reset empties its index and vectors).
onSignOut(() => { importedSource.reset(); warmed = false; });
// The launcher's local rows for imported documents: their titles, from the copy the index last read (the index is built before any search is useful).
registerSearchProvider('imported', (q) => {
  const w = q.toLowerCase().split(/\s+/).filter((x) => x !== '');
  return w.length === 0 ? [] : importedSource.titles.filter((d) => w.every((x) => d.title.toLowerCase().includes(x))).map((d) => ({ address: d.address, title: d.title, kind: 'document', detail: 'Imported document' }));
});

export async function retrievalServe(a: { id: string; words: string; pageTitle: string; pageAddress: string; now: number; facts: string }, storage: Storage | undefined): Promise<{ id: string; ok: boolean }> {
  live.storage = storage ?? live.storage;
  const later = nativeLater();
  live.native = later ? nativeEmbedder(later as never) : null;
  if (!warmed) { warmed = true; live.kick(true); }
  const ask = parseAsk(a.id, a.words, a.pageTitle, a.pageAddress, a.now, a.facts);
  if (!ask || !later) return { id: a.id, ok: false };
  ask.facts = { ...ask.facts, ...(await sharedFacts(live.storage, ask.pageAddress)) };      // native's ask carries only the run sheet; the rest is shared code's own
  return live.answer(ask, later as never);
}

/** What the turn's retrieval takes from shared code itself, not from what native hands back in the ask (docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md): the open pages (pages.ts),
 * the edition topics (edition-picks.ts, the one reader of their file) and the text of the page in front (reader.ts holds it). Each part that fails is empty: the turn goes on with what there is. */
export async function sharedFacts(storage: Storage | undefined, pageAddress: string): Promise<Omit<Facts, 'runSheet'>> {
  const pages = await pagesView(storage).then((v) => v.rows.map((p) => ({ address: p.address, title: p.title, kind: p.kind })), () => []);
  const topics = await picksText(storage, { device: null, live: async () => [] }).then((p) => p.picks.split('\n').filter((t) => t !== ''), () => [] as string[]);
  return { pages, topics, pageText: pageAddress === '' ? '' : heldText(pageAddress) };
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
