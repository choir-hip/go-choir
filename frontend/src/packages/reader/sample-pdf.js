// A small, dependency-free PDF writer for the Reader preview sample.
//
// /read/sample-document.pdf is generated here at build time, so the preview
// reads a real PDF through Reader's own text extractor: a cross-reference
// table, one uncompressed content stream per page, and standard Helvetica
// text. Not a general PDF library; it writes exactly the sample below.

const PAGE_WIDTH = 595;
const PAGE_HEIGHT = 842;
const MARGIN = 64;
const LEADING = 17;
const LF = String.fromCharCode(10);

export function escapePdfText(line) {
  // Backslashes first, then the two parenthesis delimiters, as PDF strings
  // require. Character codes keep this free of escape-sequence puzzles.
  return String(line)
    .split(String.fromCharCode(92))
    .join(String.fromCharCode(92, 92))
    .split('(')
    .join(String.fromCharCode(92) + '(')
    .split(')')
    .join(String.fromCharCode(92) + ')');
}

// Wrap at a word boundary on an estimated Helvetica advance of half the font
// size: close enough for a sample, and it never splits a word.
export function wrapLine(line, maxChars) {
  const words = String(line).split(' ').filter(Boolean);
  const lines = [];
  let current = '';
  for (const word of words) {
    if (!current) current = word;
    else if ((current + ' ' + word).length <= maxChars) current += ' ' + word;
    else {
      lines.push(current);
      current = word;
    }
  }
  if (current) lines.push(current);
  return lines;
}

// Phrase: { size, font ('F1' roman or 'F2' bold), gap, text }. The layout is a
// list of pages, each a list of runs { x, y, text, font, size }. A run with an
// empty text marks a paragraph break: it moves the text position without
// drawing, which is how a reader of the content stream sees blocks.
export function layoutFromPhrases(phrases) {
  const pages = [];
  let runs = [];
  let y = PAGE_HEIGHT - MARGIN;
  const newPage = () => {
    pages.push(runs);
    runs = [];
    y = PAGE_HEIGHT - MARGIN;
  };
  const addRun = (font, size, x, position, text) => runs.push({ x, y: position, text, font, size });
  for (const phrase of phrases) {
    const size = phrase.size || 11;
    const font = phrase.font || 'F1';
    const maxChars = Math.max(20, Math.floor((PAGE_WIDTH - MARGIN * 2) / (size * 0.5)));
    for (const line of wrapLine(phrase.text, maxChars)) {
      if (y < MARGIN + LEADING) newPage();
      addRun(font, size, MARGIN, y, line);
      y -= LEADING * (size / 11);
    }
    y -= phrase.gap == null ? 6 : phrase.gap;
    if (y < MARGIN + LEADING) {
      newPage();
    } else {
      addRun(font, size, MARGIN, y, '');
    }
  }
  pages.push(runs);
  return pages;
}

// One page's operator stream. Each vertical move of more than a line is a new
// text matrix, so the extractor reads the same blocks a person sees.
export function pageStreamFromRuns(runs) {
  let previousY = null;
  const ops = [];
  for (const run of runs) {
    if (previousY === null || Math.abs(previousY - run.y) > 1) {
      ops.push('1 0 0 1 ' + run.x + ' ' + run.y + ' Tm');
    }
    ops.push('BT /' + run.font + ' ' + run.size + ' Tf (' + escapePdfText(run.text) + ') Tj ET');
    previousY = run.y;
  }
  return ops.join(LF);
}

function objectString(id, body) {
  return id + ' 0 obj' + LF + body + LF + 'endobj' + LF;
}

// A complete PDF document as a latin1-safe string of bytes.
export function buildPdf(phrases, options) {
  const title = (options && options.title) || 'Sample';
  const streams = layoutFromPhrases(phrases).map(pageStreamFromRuns);
  const pageCount = streams.length;
  const pageId = (index) => 5 + index * 2;
  const objects = [];
  objects.push(objectString(1, '<< /Type /Catalog /Pages 2 0 R >>'));
  objects.push(objectString(2, '<< /Type /Pages /Count ' + pageCount + ' /Kids [' + streams.map((_s, i) => pageId(i) + ' 0 R').join(' ') + '] >>'));
  objects.push(objectString(3, '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>'));
  objects.push(objectString(4, '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>'));
  streams.forEach((stream, index) => {
    const id = pageId(index);
    objects.push(objectString(id, '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ' + PAGE_WIDTH + ' ' + PAGE_HEIGHT + '] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents ' + (id + 1) + ' 0 R >>'));
    objects.push(objectString(id + 1, '<< /Length ' + stream.length + ' >>' + LF + 'stream' + LF + stream + LF + 'endstream'));
  });
  let pdf = '%PDF-1.4' + LF + '%' + String.fromCharCode(226, 227, 207, 211) + LF;
  const offsets = [];
  for (const body of objects) {
    offsets.push(pdf.length);
    pdf += body;
  }
  const xrefAt = pdf.length;
  pdf += 'xref' + LF + '0 ' + String(objects.length + 1) + LF + '0000000000 65535 f ' + LF;
  for (const offset of offsets) pdf += String(offset).padStart(10, '0') + ' 00000 n ' + LF;
  pdf += 'trailer' + LF + '<< /Size ' + String(objects.length + 1) + ' /Root 1 0 R /Info << /Title (' + escapePdfText(title) + ') /Producer (Choir Reader sample) >> >>' + LF;
  pdf += 'startxref' + LF + String(xrefAt) + LF + '%%EOF' + LF;
  return pdf;
}

// The sample as two pages, so the preview proves page turning as well as text
// selection.
export function samplePdfPhrases() {
  // Page one ends with its own heading and paragraph, and page two starts with
  // a heading and a paragraph of its own. Both break points are explicit
  // because the layout guard only fires after the line that overflows: a break
  // left to a filler gap split the last paragraph mid-sentence and put page
  // two's opening prose at the foot of page one, under indicator 1 of 2.
  return [
    { size: 22, font: 'F2', text: 'Reader sample', gap: 6 },
    { size: 11, font: 'F1', text: 'A PDF opened by Choir Reader, before sign-in, with no computer behind you.', gap: 16 },
    { size: 15, font: 'F2', text: 'One window for long things', gap: 4 },
    { size: 11, font: 'F1', text: 'Reader is the quiet place where Choir keeps papers, reports, manuals and books. It opens PDFs and EPUBs, and it opens onto the place you left.', gap: 8 },
    { size: 11, font: 'F1', text: 'A page of prose wants a measure, not a viewport. Reader sets one, and keeps the text size you choose. Search finds a page in a PDF and a paragraph in an EPUB, without leaving the document and without the network.', gap: 8 },
    { size: 11, font: 'F1', text: 'Signed out, your text settings are kept in this browser, and nowhere else.', gap: 560 },
    { size: 15, font: 'F2', text: 'Reading position', gap: 4 },
    { size: 11, font: 'F1', text: 'This is the second page of the sample, so the page controls have somewhere to go. Signed in, Reader remembers where you stopped on your computer, through the media progress route, and follows you to any device. Turn the page with Next page, the arrow keys, or the floating arrow.', gap: 20 },
    { size: 15, font: 'F2', text: 'The PDF and EPUB windows stay', gap: 8 },
    { size: 11, font: 'F1', text: 'The PDF and EPUB windows remain in the launcher, unchanged, for anything Reader has not learned to do yet.', gap: 8 },
  ];
}
