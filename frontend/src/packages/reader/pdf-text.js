// Reader: a text extractor for PDFs, without pdfjs.
//
// Good enough to read a text PDF: it takes the streams, inflates FlateDecode
// ones when DecompressionStream exists, and keeps the strings inside the text
// objects. It is not a renderer. The desktop app uses pdfjs for rendering and
// for its own text layer; this module is the fallback the preview page uses
// when it reads the sample without a worker, and it keeps the pure parts
// testable on their own.
import { cleanText } from './reader-text.js';

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

// A page content stream, read token by token. A literal string contributes its
// text; Td, TD and T* start a new line; Tm and ET start a new block; and a
// string shown right after another string or a Tm in the same block continues
// the line. Font names like /Helvetica or /F1 are skipped as tokens, so their
// letters are never mistaken for operators.
export function extractTextFromPdfContent(content) {
  const source = String(content || '');
  const lines = [];
  let current = '';
  let sawText = false;
  let index = 0;
  let inText = false;
  const pushLine = () => {
    const line = cleanText(current);
    if (line) lines.push(line);
    current = '';
  };
  const pushBlock = () => {
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
      current += pdfUnescape(literal);
      sawText = true;
      index = cursor + 1;
      continue;
    }
    if (char === 'B' && source[index + 1] === 'T') {
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
    if (char === 'T' && inText && source[index + 1] === 'm') {
      pushBlock();
      index += 2;
      continue;
    }
    if (char === 'E' && source[index + 1] === 'T') {
      inText = false;
      pushLine();
      index += 2;
      continue;
    }
    index += 1;
  }
  pushLine();
  return { text: lines.filter((line, i) => line !== '' || i === 0 || lines[i - 1] !== '').join(LF), sawText };
}

// Paragraphs are separated by a blank line, the layout modelFromPdfPages uses.
function paragraphsOf(text) {
  return String(text || '')
    .split(LF + LF)
    .map((part) => cleanText(part))
    .filter(Boolean);
}

// One page's text, cut into the paragraphs the reader laid out. The extractor
// keeps the breaks a stream declares (Tm, ET) as lines, so a wrapped line sits
// between two others and a paragraph's last line does not. The shapes are
// measured on the lines themselves: a paragraph is a line that wraps onto
// further lines, or a pair of short lines (a heading and the line under it); a
// lone short line is a stray line and stays on its own. Shapes are read as
// pairs because a single-line rule cannot keep a heading with the paragraph it
// belongs to; a heading is short and so is the line after it, and a wrapped
// line is long and so is the line after it.
export function paragraphsFromLines(text) {
  const lines = String(text || '')
    .split(LF)
    .map((line) => cleanText(line))
    .filter(Boolean);
  if (!lines.length) return [];
  const longest = lines.reduce((most, line) => Math.max(most, line.length), 0);
  // A line longer than two thirds of the measure has wrapped.
  const wrapsAt = Math.max(28, Math.floor(longest * 0.66));
  const wraps = (line) => line.length > wrapsAt;
  const paragraphs = [];
  let current = lines[0];
  for (let index = 1; index < lines.length; index++) {
    const previous = lines[index - 1];
    const line = lines[index];
    const currentWraps = wraps(current);
    const nextWraps = wraps(line);
    const newParagraph = currentWraps ? !wraps(previous) : !(nextWraps && !wraps(previous));
    if (newParagraph) {
      paragraphs.push(current);
      current = line;
    } else {
      current += ' ' + line;
    }
  }
  paragraphs.push(current);
  return paragraphs.map((part) => cleanText(part)).filter(Boolean);
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
    pages.push(paragraphsOf(result.text));
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
    blocks.push({ tag: 'h3', text: 'Page ' + String(pageIndex + 1), page: pageIndex + 1 });
    pagesSplitIntoParagraphs(page).forEach((paragraph) => {
      blocks.push({ tag: 'p', text: paragraph, page: pageIndex + 1 });
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

function pagesSplitIntoParagraphs(page) {
  return Array.isArray(page) ? page.map((part) => cleanText(part)).filter(Boolean) : paragraphsOf(page);
}
