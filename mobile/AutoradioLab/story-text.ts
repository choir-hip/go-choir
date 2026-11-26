// The words of a published story, taken from its resolve bundle without parsing the whole bundle.
//
// The bundle is about 1.4 MB because the story's text (about 23 KB) is carried beside its blocks,
// source entities, transclusions and anchors. `JSON.parse` of it is one synchronous call; on the
// phone it took a radio turn over Exact's 100 ms budget and the answer was refused (2026-11-26).
// A reader page still parses the bundle; the radio only needs the title and the text, so this
// finds those two string fields with native `indexOf` and decodes each string literal on its own.
//
// The keys it looks for are object keys (`"payload":{`), which cannot appear inside a JSON string
// (there the quotes are escaped), so a scan cannot be fooled by text that mentions them.
export type StoryWords = { title: string; paragraphs: string[] };

/** The index just past the JSON string that starts at `quote`, or -1 when it never closes. */
function stringEnd(raw: string, quote: number): number {
  for (let i = quote + 1; i < raw.length; i++) {
    const c = raw.charCodeAt(i);
    if (c === 0x5c) { i += 1; continue; }                 // a backslash escapes the next character
    if (c === 0x22) return i + 1;
  }
  return -1;
}

/** The index just past the object that starts at `from` (braces inside strings do not count), or raw.length. */
function objectEnd(raw: string, from: number): number {
  let depth = 0;
  for (let i = from; i < raw.length; i++) {
    const c = raw.charCodeAt(i);
    if (c === 0x22) { const e = stringEnd(raw, i); if (e < 0) return raw.length; i = e - 1; continue; }
    if (c === 0x7b) depth += 1;
    else if (c === 0x7d) { depth -= 1; if (depth === 0) return i + 1; }
  }
  return raw.length;
}

/** The decoded string at `key` inside the object that starts at `from`, or undefined. */
function field(raw: string, from: number, key: string, stop = raw.length): string | undefined {
  const at = raw.indexOf(`"${key}":"`, from);
  if (at < 0 || at >= stop) return undefined;
  const quote = at + key.length + 3;                     // `"key":"` is key.length + 4 characters
  const end = stringEnd(raw, quote);
  if (end < 0 || end > stop) return undefined;
  try { const value = JSON.parse(raw.slice(quote, end)); return typeof value === 'string' ? value : undefined; } catch { return undefined; }
}

/** Plain reading of a story's text: headings, emphasis and links lose their Markdown, paragraphs stay. */
export function storyParagraphs(text: string): string[] {
  const plain = text
    .replace(/```[\s\S]*?```/g, '')                                  // fenced code is not read aloud
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^[ \t]{0,3}>[ \t]?/gm, '')
    .replace(/^\s*([-*_])\1{2,}\s*$/gm, '')                          // a horizontal rule
    .replace(/(\*\*|__)(.*?)\1/g, '$2')
    .replace(/\*(?!\s)(.*?)(?<!\s)\*/g, '$1')                        // emphasis, never an underscore inside a word
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\r\n?/g, '\n');
  return plain.split(/\n\s*\n+/).map((p) => p.replace(/\s+/g, ' ').trim()).filter((p) => p !== '');
}

/** The story's title and paragraphs from the text of a resolve bundle, or undefined when it holds neither. */
export function storyWords(raw: string): StoryWords | undefined {
  // Anchored to the first field of each Go struct's JSON (`PublicationBundle.publication` starts
  // with `id`; `PublicationVersionPayload` with `revision`; `PublicationArtifact` with
  // `manifest_id`), so a key named in a story's own text cannot capture the scan.
  const publication = raw.indexOf('"publication":{"id":');
  const title = publication < 0 ? undefined : field(raw, publication, 'title', objectEnd(raw, publication));
  const payload = raw.indexOf('"payload":{"revision":');
  const manifest = payload < 0 ? -1 : raw.indexOf('"manifest":{', payload);
  // `text` comes before `manifest`, so the manifest bounds the scan without walking it.
  let text = payload < 0 ? undefined : field(raw, payload, 'text', manifest > payload ? manifest : objectEnd(raw, payload));
  if (text === undefined || text === '') {
    const artifact = raw.indexOf('"artifact":{"manifest_id":');
    text = artifact < 0 ? undefined : field(raw, artifact, 'content', objectEnd(raw, artifact));
  }
  if (text === undefined || text === '') return undefined;
  const paragraphs = storyParagraphs(text);
  if (paragraphs.length === 0) return undefined;
  return { title: title ?? '', paragraphs };
}
