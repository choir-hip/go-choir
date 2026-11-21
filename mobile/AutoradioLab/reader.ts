// The reader for stories, in shared code: a `pub:` page read on a host with no native reader (Android; the Swift reader, ReaderShelf/ReaderStore, is the iOS one and decodes more: citations, versions,
// related stories, tables, embeds). This twin reads the public route the paper already uses (GET /api/platform/publications/resolve?route=, no key), takes the readable text, and cuts it into
// blocks the reader page draws (h, p, li, quote), one text run each. Citations, versions, related stories, hear-it rows and editing are NOT here (owed: they come with the full port).
// Logs carry counts only.
import { getJSON } from './api.ts';
import type { Result } from './app.contract.d.ts';
import { storyText } from './content.ts';

export type ReaderDoc = Result<'readerDocShared'>;
const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');

const empty = (address: string, status: string, message = ''): ReaderDoc => ({
  address, status, message, title: '', kicker: '', dek: '', byline: '', meta: '', kind: 'story', disclosure: '', blocks: [], cites: [], sources: [], versions: [], related: [],
  revision: 0, latest: 0, newer: false, historical: false, revising: false, revisingNote: '', trajectory: '', permalink: '', plain: '', textBlocks: 0, markdown: '', editable: false, source: '', docId: '',
});

const run = (id: string, text: string) => ({ id, text, kind: 'text', weight: 400, slant: '', anchor: '', label: '', sent: 0, href: '' });
const block = (id: string, kind: string, text: string, level = 0): ReaderDoc['blocks'][number] => ({ id, kind, level, depth: 0, marker: kind === 'li' ? '•' : '', runs: [run(`${id}.0`, text)], rows: [], text, ordinal: 0, cites: [], sentences: 0 });

/** Markdown-ish text to blocks: headings (#), bullets (- or *), quotes (>), paragraphs. Fences and rules are dropped; the rest is plain text. */
export function blocksOf(markdown: string): ReaderDoc['blocks'] {
  const out: ReaderDoc['blocks'] = [];
  let para: string[] = []; let inCode = false;
  const flush = (): void => { if (para.length > 0) { out.push(block(`b${out.length}`, 'p', para.join(' '))); para = []; } };
  for (const raw of markdown.split('\n')) {
    const t = raw.trim();
    if (t.startsWith('```')) { flush(); inCode = !inCode; continue; }
    if (inCode) continue;
    if (t === '' || /^([-*_]\s*){3,}$/.test(t)) { flush(); continue; }
    const h = /^(#{1,6})\s+(.*)$/.exec(t);
    if (h) { flush(); out.push(block(`b${out.length}`, 'h', h[2], h[1].length)); continue; }
    const li = /^[-*•]\s+(.*)$/.exec(t);
    if (li) { flush(); out.push(block(`b${out.length}`, 'li', li[1])); continue; }
    if (t.startsWith('>')) { flush(); out.push(block(`b${out.length}`, 'quote', t.replace(/^>\s?/, ''))); continue; }
    para.push(t);
  }
  flush();
  return out;
}

/** The story at a `pub:` address. Another address is "loading": another reader owns it. */
export async function readerDocShared(address: string): Promise<ReaderDoc> {
  if (!address.startsWith('pub:')) return empty(address, 'loading');
  const route = address.slice(4);
  try {
    const body = (await getJSON('/api/platform/publications/resolve', { route })).body;
    const text = storyText(body);
    if (text === null || text.trim() === '') return empty(address, 'error', 'This story has no text to show.');
    const title = obj(body) && obj(body.publication) ? str(body.publication.title) : '';
    // storyText drops headings' hashes; the blocks are cut from the same lines so a story keeps its paragraphs
    const raw = obj(body) ? (obj(body.payload) && str(body.payload.text) !== '' ? str(body.payload.text) : obj(body.artifact) ? str(body.artifact.content) : '') : '';
    const blocks = blocksOf(raw !== '' ? raw : text);
    const d = empty(address, 'ready');
    d.title = title; d.blocks = blocks; d.plain = blocks.map((b) => b.text).join('\n\n'); d.textBlocks = blocks.filter((b) => b.kind === 'p').length; d.permalink = route.replace(/^\//, '');
    console.log(`reader: story blocks=${blocks.length}`);
    return d;
  } catch { return empty(address, 'error', 'Could not load this story. Check the connection and try again.'); }
}
