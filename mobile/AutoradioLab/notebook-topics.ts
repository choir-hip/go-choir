// The listener's topics become lines of "What you asked for" in the notebook, once (it was Swift: NotebookStore.writeTopics, a Texture canonical write through the Swift TextureEditor; the
// merge itself is notebook.ts `merging`). Asked for by the notebook page on every read and by first run when the edition is written, so a write that failed is tried again.
//  - The notebook is found by `GET /api/texture/edition` (found / not found / cannot say). Cannot say is never "there is none": nothing is created on a guess.
//  - It is made (`POST /api/texture/documents`, title "My notebook") only when the server says there is none, and written through the Texture editor (texture-edit.ts), which posts the revision
//    against the head it read and never over a head that moved.
//  - The marker (`app:/data/notebook-topics-written`) holds the topics' keys last written; the same topics again ask the server nothing. It is written only after the notebook holds them.
//  - One write at a time: a second ask while one runs is that one (two would both find no notebook and make two). Every step checks the account generation; a write for an account that was
//    wiped meanwhile writes nothing and no marker (account.ts wipes the marker).
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, authedJSON, writeCurrent } from './api.ts';
import { lookupEdition, merging, topicKey, TITLE } from './notebook.ts';
import { textureSource } from './content.ts';
import { picksText } from './edition-picks.ts';
import { textureWriteAutomatic, type Net } from './texture-edit.ts';

export const MARKER = 'app:/data/notebook-topics-written';
export type Deps = { net: Net; writeAutomatic: (address: string, text: string, from: string | null) => Promise<'saved' | 'busy' | 'failed'>; topics: () => Promise<string[]> };
const real = (storage: Storage | undefined, topics?: string[]): Deps => ({
  net: { json: (path, opts) => authedJSON(path, opts) },
  writeAutomatic: (address, text, from) => textureWriteAutomatic(address, text, from, storage),
  topics: topics ? async () => topics : async () => (await picksText(storage, { device: null, live: async () => [] })).picks.split('\n').filter((t) => t !== ''),
});
/** The topics first run has just written to the picks (named, so the write does not depend on the picks file's timing). */
export const writeTopicsOf = (storage: Storage | undefined, topics: string[]): Promise<boolean> => writeTopics(storage, real(storage, topics));

let writing: Promise<boolean> | null = null;

/** True when the notebook holds the topics afterwards (or there is nothing to write). */
export function writeTopics(storage: Storage | undefined, deps: Deps = real(storage)): Promise<boolean> {
  if (writing) return writing;
  const p = writeNow(storage, deps).catch(() => false).finally(() => { writing = null; });
  writing = p;
  return p;
}
/** Sign-out: forget a write in flight (its steps find the generation moved). */
export function forgetTopicsWrite(): void { writing = null; }

async function writeNow(storage: Storage | undefined, d: Deps): Promise<boolean> {
  const gen = accountGeneration();
  const topics = await d.topics();
  const key = topics.map(topicKey).filter((k) => k !== '').sort().join(',');
  if (key === '') return true;
  try { if (storage && new TextDecoder().decode(await storage.fs.readFile(MARKER)) === key) return true; } catch { /* no marker */ }
  const get = async (path: string): Promise<unknown> => (await d.net.json(path)).body;
  const ed = await lookupEdition(get);
  if (ed === null || gen !== accountGeneration()) return false;                    // the server cannot say: try later, never create on a guess
  let address = '';
  let current: string | null = null;
  if (ed.found) {
    address = `texture:${ed.docID}`;
    try { current = await textureSource(ed.docID, get); } catch { return false; }
  } else {
    const r = await d.net.json('/api/texture/documents', { method: 'POST', body: { title: TITLE } });
    const id = typeof (r.body as { doc_id?: unknown } | null)?.doc_id === 'string' ? (r.body as { doc_id: string }).doc_id : '';
    if (id === '' || gen !== accountGeneration()) return false;
    address = `texture:${id}`;
  }
  if (gen !== accountGeneration()) return false;
  const merged = merging(topics, current);
  if (merged !== current) {
    // ONE call inside the editor's queue (a listener's unsent text there makes it 'busy': ours to leave alone; a failed write drops only OUR text, never one typed behind it).
    const r = await d.writeAutomatic(address, merged, current);
    if (r !== 'saved' || gen !== accountGeneration()) return false;
  }
  return writeCurrent(gen, storage, MARKER, new TextEncoder().encode(key));
}
