// Reader: a text extractor for PDFs, without pdfjs.
//
// Good enough to read a text PDF: it takes the streams, inflates FlateDecode
// ones when DecompressionStream exists, and keeps the strings inside the text
// objects. It is not a renderer. The desktop app uses pdfjs for rendering and
// for its own text layer; this module is the fallback the preview page uses
// when it reads the sample without a worker, and it keeps the pure parts
// testable on their own.
import { cleanText, isHeadingTag } from './reader-text.js';

const BACKSLASH = String.fromCharCode(92);
const LF = String.fromCharCode(10);
// Whitespace and the PDF delimiters: the characters that end a name token.
const DELIMITERS = ' \t\r\n' + String.fromCharCode(9, 12) + '()<>[]{}/%';

function bytesToLatin1(bytes) {
  // One char per byte. Buffer carries this in Node; the browser path builds
  // the string in fixed chunks, never with one recursive call per element.
  if (typeof Buffer !== 'undefined' && typeof Buffer.from === 'function') {
    return Buffer.from(bytes.buffer ? new Uint8Array(bytes) : bytes).toString('latin1');
  }
  let out = '';
  const chunk = 8192;
  for (let index = 0; index < bytes.length; index += chunk) {
    out += String.fromCharCode.apply(null, Array.prototype.slice.call(bytes.subarray(index, index + chunk)));
  }
  return out;
}

async function inflateBytes(bytes) {
  if (typeof DecompressionStream === 'undefined') return null;
  try {
    const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('deflate'));
    const buffer = await new Response(stream).arrayBuffer();
    return new Uint8Array(buffer);
  } catch (_err) {
    return null;
  }
}

function pdfUnescape(value) {
  // PDF string escapes: a backslash before a delimiter or a backslash, an
  // n/r/t/b/f shorthand, or one to three octal digits. Characters are compared
  // directly, so no escape sequence appears in a pattern here.
  const chars = String(value).split('');
  let out = '';
  for (let index = 0; index < chars.length; index++) {
    if (chars[index] !== BACKSLASH) {
      out += chars[index];
      continue;
    }
    const escape = chars[index + 1];
    if (escape === undefined) break;
    if (escape === 'n') { out += LF; index += 1; continue; }
    if (escape === 'r') { out += String.fromCharCode(13); index += 1; continue; }
    if (escape === 't') { out += String.fromCharCode(9); index += 1; continue; }
    if (escape === 'b' || escape === 'f') { index += 1; continue; }
    if (escape >= '0' && escape <= '7') {
      let digits = '';
      while (digits.length < 3 && chars[index + 1] >= '0' && chars[index + 1] <= '7') {
        digits += chars[index + 1];
        index += 1;
      }
      out += String.fromCharCode(parseInt(digits, 8));
      continue;
    }
    out += escape;
    index += 1;
  }
  return out;
}

// A page content stream, read token by token. The text objects are the
// structure: BT opens a block and a new BT after ET is a new block, so a
// heading or a paragraph a writer set as one text object comes back as one
// block. Inside a text object, Tm, Td, TD and T* start a new line of the same
// block, which is how a paragraph's wrapped lines are placed; the strings
// shown on a line join it. Font names like /Helvetica or /F1 are skipped as
// tokens, so their letters are never mistaken for operators.
export function extractTextFromPdfContent(content) {
  const source = String(content || '');
  const lines = [];
  let current = '';
  let sawText = false;
  let index = 0;
  let inText = false;
  // Set while a line of text is open. A string shown after a Tm, Td, T*, ET or
  // BT starts a line; a second string on the same line (a font change mid-line)
  // continues it.
  let lineOpen = false;
  // A line is the text written since the stream last moved. An empty line is
  // the break between two blocks, and it is what paragraphsFromLines splits on;
  // a second empty line in a row says nothing more and is not written.
  const pushLine = () => {
    const line = cleanText(current);
    if (line) lines.push(line);
    else if (current !== '' && lines.length && lines[lines.length - 1] !== '') lines.push('');
    current = '';
    lineOpen = false;
  };
  // A new text object starts a new block: the line above ends, and an empty
  // line separates the two unless the block is the page's first.
  const startBlock = () => {
    pushLine();
    if (lines.length && lines[lines.length - 1] !== '') lines.push('');
  };
  while (index < source.length) {
    const char = source[index];
    if (char === '%') {
      const eol = source.indexOf(LF, index);
      index = eol < 0 ? source.length : eol + 1;
      continue;
    }
    if (char === '/') {
      // Advance past the slash first: '/' is one of the delimiters the loop
      // tests, so starting the loop on it would never move index and a name
      // token such as /F1 would hang the extractor forever.
      index += 1;
      while (index < source.length && DELIMITERS.indexOf(source[index]) < 0) index += 1;
      continue;
    }
    if (char === '(') {
      let depth = 1;
      let cursor = index + 1;
      let literal = '';
      while (cursor < source.length && depth > 0) {
        const inner = source[cursor];
        if (inner === BACKSLASH) {
          literal += inner + (source[cursor + 1] || '');
          cursor += 2;
          continue;
        }
        if (inner === '(') depth += 1;
        if (inner === ')') {
          depth -= 1;
          if (depth === 0) break;
        }
        literal += inner;
        cursor += 1;
      }
      // A show operator writes on the line it is on. A stream that moves the
      // text position without a Tm or Td between two strings (a BT followed by
      // two strings) leaves the line open, and the two join; a Tm or a new text
      // object closes the line first, so each run is its own line.
      if (!lineOpen) {
        current = '';
        lineOpen = true;
      }
      current += pdfUnescape(literal);
      sawText = true;
      index = cursor + 1;
      continue;
    }
    if (char === 'B' && source[index + 1] === 'T') {
      // BT after ET opens a new block; a BT that never followed an ET is the
      // same object repeated, and only closes the line before it.
      if (inText) pushLine();
      else startBlock();
      inText = true;
      index += 2;
      continue;
    }
    if (char === 'T' && inText && (source[index + 1] === 'd' || source[index + 1] === 'D')) {
      pushLine();
      index += 2;
      continue;
    }
    if (char === 'T' && inText && source[index + 1] === '*') {
      pushLine();
      index += 2;
      continue;
    }
    if (char === 'T' && source[index + 1] === 'm') {
      // A matrix inside a text object places the next line of the same block;
      // one outside a text object is a page that moved without a BT, and the
      // block above ends there.
      if (inText) pushLine();
      else startBlock();
      index += 2;
      continue;
    }
    if (char === 'E' && source[index + 1] === 'T') {
      // The text object ends the line it holds; the block it wrote stays until
      // another text object opens.
      inText = false;
      pushLine();
      index += 2;
      continue;
    }
    index += 1;
  }
  pushLine();
  // A page ends without a trailing blank line: a break at the foot of a page
  // has nothing to separate.
  while (lines.length && lines[lines.length - 1] === '') lines.pop();
  return { text: lines.join(LF), sawText };
}

// A page's lines as the blocks the surface renders. The extractor keeps the
// stream's own line breaks (Tm, Td, T* inside a text object) and writes a
// blank line where a new text object began, so the split is the document's,
// not a guess: a blank line ends a block, and the lines between two of them
// are one block of prose, set as one paragraph with its wrapped lines joined.
// A block that reads as a short phrase without a closing mark is a heading.
// The rule this replaces inferred paragraphs from the lengths of neighbouring
// lines; that cannot tell a writer's tight text from its paragraphs, and it
// merged a heading into the paragraph under it.
export function paragraphsFromLines(text) {
  const lines = String(text || '').split(LF).map((row) => cleanText(row));
  const blocks = [];
  let buffer = [];
  const flush = () => {
    const block = cleanText(buffer.join(' '));
    buffer = [];
    if (!block) return;
    blocks.push({ tag: paragraphLooksLikeHeading(block) ? 'h3' : 'p', text: block });
  };
  // Lines in this page's text, and where the empty ones separate them.
  const packed = lines.join(String.fromCharCode(1));
  const runs = packed.split(String.fromCharCode(1));
  let sinceBreak = 0;
  for (const row of runs) {
    if (row) {
      buffer.push(row);
      sinceBreak += 1;
    } else if (sinceBreak) {
      flush();
      sinceBreak = 0;
    }
  }
  flush();
  return blocks;
}

// A heading: at most a dozen words, no closing mark a sentence ends with, and
// opening with a capital letter, a digit or a quote. Text that reads as one
// line of a heading is still a heading, and that is the cost of telling a
// heading from prose without a font.
export function paragraphLooksLikeHeading(text) {
  const part = cleanText(text);
  if (!part || part.length > 90) return false;
  if (part.split(' ').length > 12) return false;
  if (/[.!?,;:]$/.test(part)) return false;
  return /^[A-Z0-9"'()]/.test(part);
}

// Paragraphs are separated by one or more blank lines: that is how a layout
// writes the space between them, and how the lines of one paragraph stay
// together.
function paragraphsOf(text) {
  return String(text || '')
    .split(new RegExp(LF + '{2,}'))
    .map((line) => cleanText(line))
    .filter(Boolean);
}


export async function extractPdfText(data) {
  const bytes = data instanceof Uint8Array ? data : new Uint8Array(data);
  const latin = bytesToLatin1(bytes);
  const streams = [];
  let cursor = 0;
  while (streams.length < 64) {
    const start = latin.indexOf('stream', cursor);
    if (start < 0) break;
    let dataStart = start + 'stream'.length;
    if (latin[dataStart] === String.fromCharCode(13)) dataStart += 1;
    if (latin[dataStart] === LF) dataStart += 1;
    const end = latin.indexOf('endstream', dataStart);
    if (end < 0) break;
    const dictionary = latin.slice(Math.max(0, start - 400), start);
    streams.push({ raw: bytes.subarray(dataStart, end), flate: /FlateDecode/.test(dictionary) });
    cursor = end + 'endstream'.length;
  }
  // One content stream is one page in the sample this reads, so pages stay
  // apart: each page is a list of its paragraphs, the shape the surface
  // renders. Joining them into one list would make every paragraph look like
  // a page and lose page turning.
  const pages = [];
  const pieces = [];
  for (const stream of streams) {
    let payload = stream.raw;
    if (stream.flate) {
      const inflated = await inflateBytes(payload);
      if (!inflated) return { text: '', pages: [], sawText: false, extractable: false };
      payload = inflated;
    }
    const result = extractTextFromPdfContent(bytesToLatin1(payload));
    if (!result.sawText) continue;
    pieces.push(result.text);
    const blocks = paragraphsFromLines(result.text);
    // An extractor that hands a page back as one string would leave the page
    // with no paragraphs at all; then the blank-line split is the only
    // structure there is.
    pages.push(blocks.length ? blocks : paragraphsOf(result.text));
  }
  const text = pieces.join(LF + LF).trim();
  return { text, pages, sawText: Boolean(text), extractable: true };
}

// A page-per-section model for extracted PDF text. Each block keeps its page
// number so search results can say which page they are on.
export function modelFromPdfPages(pages, options) {
  const chosen = options || {};
  const list = Array.isArray(pages) ? pages : [];
  const blocks = [];
  list.forEach((page, pageIndex) => {
    // The page marker is what a PDF's contents list shows; it is emitted only
    // when the page has text, so an empty trailing page adds no heading.
    const parts = pagesSplitIntoParagraphs(page);
    if (!parts.length) return;
    blocks.push({ tag: 'h3', text: 'Page ' + String(pageIndex + 1), page: pageIndex + 1 });
    parts.forEach((block) => {
      blocks.push({ ...block, page: pageIndex + 1 });
    });
  });
  const title = chosen.title || chosen.fileName || 'PDF';
  return {
    format: 'pdf',
    title,
    fileName: chosen.fileName || '',
    sample: false,
    status: 'ready',
    sections: [{ id: 'pdf', title, blocks }],
  };
}

// A page is a list of blocks { tag, text } once it has been read for headings,
// or a list of plain paragraph strings from an extractor that kept no line
// shape. Both become blocks here, so the model always carries tags.
function pagesSplitIntoParagraphs(page) {
  if (!Array.isArray(page)) return paragraphsOf(page).map((part) => ({ tag: paragraphLooksLikeHeading(part) ? 'h3' : 'p', text: part }));
  return page
    .map((part) => (part && typeof part === 'object' ? { tag: isHeadingTag(part.tag) ? 'h3' : 'p', text: cleanText(part.text) } : { tag: paragraphLooksLikeHeading(part) ? 'h3' : 'p', text: cleanText(part) }))
    .filter((block) => block.text);
}
