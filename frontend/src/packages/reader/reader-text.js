// Reader: text handling. Pure functions only: no DOM, no network.
//
// A "document model" is the one shape both formats reduce to, so the surface
// that renders it has a single code path:
//
//   { format, title, author, sections: [{ id, title, blocks: [{ tag, text }] }],
//     sample, status }

const HEADING_TAGS = new Set(['h1', 'h2', 'h3', 'h4']);

export function isHeadingTag(tag) {
  return HEADING_TAGS.has(String(tag || '').toLowerCase());
}

export function cleanText(value) {
  return String(value == null ? '' : value).replace(/\s+/g, ' ').trim();
}

export function dirname(path) {
  const index = String(path || '').lastIndexOf('/');
  return index >= 0 ? path.slice(0, index + 1) : '';
}

// Resolve an href against the document that contains it, the way EPUB
// archives lay out: relative to the OPF for the manifest, relative to each
// chapter for links. Absolute URLs pass through untouched.
export function resolvePath(base, href) {
  if (!href) return '';
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return href;
  const stack = dirname(base).split('/').filter(Boolean);
  for (const part of String(href).split(/[#?]/)[0].split('/')) {
    if (!part || part === '.') continue;
    if (part === '..') stack.pop();
    else stack.push(part);
  }
  return stack.join('/');
}

export function extensionOf(path) {
  const name = String(path || '').split(/[?#]/)[0];
  const dot = name.lastIndexOf('.');
  if (dot < 0 || dot === name.length - 1) return '';
  return name.slice(dot + 1).toLowerCase();
}

// The format a source is read as. An explicit media type wins, then the file
// name, then the app id the window was opened under (a Reader window opened
// from the launcher remembers "pdf"). With nothing to go on, attached text
// means EPUB text, and otherwise PDF.
export function detectFormat(options) {
  const chosen = options || {};
  const type = String(chosen.mediaType || '').toLowerCase();
  if (type.indexOf('pdf') >= 0) return 'pdf';
  if (type.indexOf('epub') >= 0) return 'epub';
  const fromName = extensionOf(chosen.fileName || chosen.sourceUrl || '');
  if (fromName === 'pdf') return 'pdf';
  if (fromName === 'epub') return 'epub';
  if (chosen.kind === 'pdf' || chosen.kind === 'epub') return chosen.kind;
  return chosen.extractedText ? 'epub' : 'pdf';
}

// Blocks from a parsed HTML document (an EPUB chapter). Headings, paragraphs,
// list items, quotes and preformatted runs become the reader block list.
// Elements nested inside another emitted element, such as a paragraph inside
// a quote, are not emitted twice. A chapter with none of those elements falls
// back to its plain text.
export function blocksFromHtml(root) {
  if (!root) return [];
  const body = root.body || root;
  const all = Array.from(body.querySelectorAll('h1,h2,h3,h4,p,li,blockquote,pre'));
  const nested = new Set();
  all.forEach((node) => {
    if (all.some((outer) => outer !== node && outer.contains(node))) nested.add(node);
  });
  const blocks = all
    .filter((node) => !nested.has(node))
    .map((node) => ({ tag: String(node.tagName || 'p').toLowerCase(), text: cleanText(node.textContent) }))
    .filter((block) => block.text);
  if (blocks.length) return blocks;
  const text = cleanText(body.textContent);
  return text ? [{ tag: 'p', text }] : [];
}

// Sections from plain text: an EPUB whose archive was not shipped, or text
// extracted onto a content item. Blank lines separate paragraphs, and a
// single line that starts with a hash marks a heading, because extracted
// text keeps no tags.
export function sectionsFromText(text, options) {
  const chosen = options || {};
  const parts = String(text || '').split(/\n{2,}/);
  const blocks = [];
  for (const part of parts) {
    const trimmed = part.trim();
    if (!trimmed) continue;
    const heading = /^#{1,6}\s/.test(trimmed) && trimmed.split('\n').length === 1;
    const line = cleanText(trimmed.replace(/^#+\s*/, ''));
    if (line) blocks.push({ tag: heading ? 'h2' : 'p', text: line });
  }
  if (!blocks.length) return [];
  const title = chosen.title || 'Document';
  return [{ id: 'text-1', title, blocks }];
}

export function chapterKey(href, index) {
  const slug = String(href || '').replace(/[^a-z0-9]+/gi, '-').replace(/^-+|-+$/g, '').toLowerCase() || 'chapter';
  return 'ch-' + String(index + 1) + '-' + slug;
}

export function escapeRegExp(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// Locate the needle in a block, returning a short snippet with an ellipsis
// where text was cut. Case-insensitive, and runs of whitespace in the needle
// count as one space.
export function snippetAround(text, needle, before, after) {
  const flat = String(text || '').replace(/\s+/g, ' ');
  const pattern = cleanText(needle).toLowerCase();
  if (!pattern) return '';
  const match = new RegExp(escapeRegExp(pattern), 'i').exec(flat);
  if (!match) return '';
  const lead = Number.isFinite(before) ? before : 42;
  const trail = Number.isFinite(after) ? after : 72;
  const start = Math.max(0, match.index - lead);
  const end = Math.min(flat.length, match.index + match[0].length + trail);
  return (start > 0 ? '...' : '') + flat.slice(start, end) + (end < flat.length ? '...' : '');
}

// Search every block of every section. Matches carry where they are so the
// surface can jump to them. An empty needle finds nothing, not everything.
export function searchDocuments(model, needle) {
  const matches = [];
  if (!cleanText(needle) || !model || !Array.isArray(model.sections)) return matches;
  model.sections.forEach((section, sectionIndex) => {
    (section.blocks || []).forEach((block, blockIndex) => {
      const snippet = snippetAround(block.text, needle);
      if (snippet) {
        matches.push({
          sectionIndex,
          blockIndex,
          sectionId: section.id,
          title: section.title || 'Section ' + String(sectionIndex + 1),
          page: block.page || 0,
          snippet,
        });
      }
    });
  });
  return matches;
}

export function modelWordCount(model) {
  if (!model || !Array.isArray(model.sections)) return 0;
  return model.sections.reduce(
    (total, section) => total + (section.blocks || []).reduce((sum, block) => sum + cleanText(block.text).split(/\s+/).filter(Boolean).length, 0),
    0,
  );
}

export function modelChapterCount(model) {
  return model && Array.isArray(model.sections) ? model.sections.length : 0;
}

// Reading minutes at a deliberately slow two hundred words a minute, rounded
// up, so the estimate does not promise more than it can keep.
export function readingMinutes(model) {
  const words = modelWordCount(model);
  return Math.max(1, Math.ceil(words / 200));
}
