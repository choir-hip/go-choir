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

// Phrase: { size, font ('F1' roman or 'F2' bold), gap, text, pageBreak }. The
// layout is a list of pages, each a list of phrases-as-lines laid out on it. A
// phrase is one block of the document, so the page writes one text object for
// it and the extractor reads one block back; gap is the air above the phrase,
// in points, and pageBreak true starts the phrase on a fresh page.
export function layoutFromPhrases(phrases) {
  const pages = [];
  let phrasesOnPage = [];
  let y = PAGE_HEIGHT - MARGIN;
  const newPage = () => {
    if (!phrasesOnPage.length) return;
    pages.push(phrasesOnPage);
    phrasesOnPage = [];
    y = PAGE_HEIGHT - MARGIN;
  };
  for (const phrase of phrases) {
    const size = phrase.size || 11;
    const gap = phrase.gap == null ? 6 : phrase.gap;
    const maxChars = Math.max(20, Math.floor((PAGE_WIDTH - MARGIN * 2) / (size * 0.5)));
    const lines = wrapLine(phrase.text, maxChars);
    const height = lines.length * LEADING * (size / 11);
    // The air above the phrase belongs to the phrase: a page starts at the
    // margin, so the first block on it is not pushed down by a gap it earned on
    // the page before.
    if (phrase.pageBreak) newPage();
    if (phrasesOnPage.length) y -= gap;
    // A block is never split across pages, and a heading is never the last
    // block of one: the phrase moves whole to the next page when the lines and
    // the room below them for a following block do not fit.
    const reserve = phrase.heading ? LEADING * 3 : 0;
    if (phrasesOnPage.length && y - height - reserve < MARGIN) newPage();
    phrasesOnPage.push({ x: MARGIN, y, lines, font: phrase.font || 'F1', size });
    y -= height;
  }
  newPage();
  return pages;
}

// One page's operator stream: one BT ... ET per phrase, each line placed by its
// own Tm inside that text object. A heading or a paragraph is therefore one
// text object, and the extractor reads it as one block with its wrapped lines
// joined; a new text object is what starts the next block. Valid PDF, and the
// shape the placement already had (each line matrixed above its text).
export function pageStreamFromRuns(phrases) {
  const ops = [];
  for (const phrase of phrases) {
    ops.push('BT /' + phrase.font + ' ' + phrase.size + ' Tf');
    for (const line of phrase.lines) {
      ops.push('1 0 0 1 ' + phrase.x + ' ' + phrase.y + ' Tm');
      ops.push('(' + escapePdfText(line) + ') Tj');
      phrase.y -= LEADING * (phrase.size / 11);
    }
    ops.push('ET');
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
// selection. Page one is the title, an opening paragraph, a section heading and
// three paragraphs of prose; page two opens on its own heading because the
// phrase carries an explicit page break, and its text says it is the second
// page. Nothing is left to where y happens to run out.
export function samplePdfPhrases() {
  return [
    { size: 22, font: 'F2', heading: true, text: 'Reader sample', gap: 0 },
    { size: 11, font: 'F1', text: 'A PDF opened by Choir Reader, before sign-in, with no computer behind you.', gap: 18 },
    { size: 15, font: 'F2', heading: true, text: 'One window for long things', gap: 18 },
    { size: 11, font: 'F1', text: 'Reader is the quiet place where Choir keeps papers, reports, manuals and books. It opens PDFs and EPUBs, and it opens onto the place you left.', gap: 12 },
    { size: 11, font: 'F1', text: 'A page of prose wants a measure, not a viewport. Reader sets one, and keeps the text size you choose. Search finds a page in a PDF and a paragraph in an EPUB, without leaving the document and without the network.', gap: 12 },
    { size: 11, font: 'F1', text: 'Signed out, your text settings are kept in this browser, and nowhere else.', gap: 12 },
    { size: 15, font: 'F2', heading: true, text: 'Reading position', gap: 0, pageBreak: true },
    { size: 11, font: 'F1', text: 'This is the second page of the sample, so the page controls have somewhere to go. Signed in, Reader remembers where you stopped on your computer, through the media progress route, and follows you to any device. Turn the page with Next page, the arrow keys, or the floating arrow.', gap: 12 },
    { size: 15, font: 'F2', heading: true, text: 'The PDF and EPUB windows stay', gap: 18 },
    { size: 11, font: 'F1', text: 'The PDF and EPUB windows remain in the launcher, unchanged, for anything Reader has not learned to do yet.', gap: 12 },
  ];
}
