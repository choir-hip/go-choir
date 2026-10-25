// Reader: reading preferences and the signed-out sample. Pure functions and
// add only: importing this file must not touch a browser API.
import { cleanText } from './reader-text.js';

export const READER_PREF_KEY = 'choir.reader.prefs.v1';
export const FONT_STEPS = { min: 14, max: 28, start: 18 };
export const MEASURE_STEPS = [58, 72, 88];
export const MEASURE_NAMES = { 58: 'Narrow', 72: 'Comfortable', 88: 'Wide' };
export const READER_THEMES = ['paper', 'sepia', 'noir'];

export function normalizePrefs(raw) {
  const source = raw && typeof raw === 'object' ? raw : {};
  let size = Number(source.fontSize);
  if (!Number.isFinite(size)) size = FONT_STEPS.start;
  size = Math.min(FONT_STEPS.max, Math.max(FONT_STEPS.min, Math.round(size)));
  let measure = Number(source.measure);
  if (MEASURE_STEPS.indexOf(measure) < 0) measure = 72;
  const theme = READER_THEMES.indexOf(source.theme) >= 0 ? source.theme : 'paper';
  return { fontSize: size, measure, theme };
}

export function loadPrefs(storage, key) {
  try {
    const raw = storage && storage.getItem(key || READER_PREF_KEY);
    return normalizePrefs(raw ? JSON.parse(raw) : null);
  } catch (_err) {
    return normalizePrefs(null);
  }
}

export function savePrefs(storage, prefs, key) {
  try {
    if (storage) storage.setItem(key || READER_PREF_KEY, JSON.stringify(normalizePrefs(prefs)));
    return true;
  } catch (_err) {
    return false;
  }
}

export function stepFontSize(prefs, delta) {
  const current = normalizePrefs(prefs);
  return normalizePrefs({ ...current, fontSize: current.fontSize + (Number(delta) || 0) });
}

// ---- the signed-out sample -------------------------------------------------
// A real little document, written for the preview, so Reader can be read and
// operated without an account and without any network. The in-desktop app
// opens it for a signed-out window that was not given a file, and the preview
// page opens it first.

const SAMPLE_SECTIONS = [
  {
    title: 'Welcome to Reader',
    blocks: [
      { tag: 'p', text: 'This is Reader, the quiet place where Choir keeps the long things: papers, reports, manuals, books. It is one window for PDFs and EPUBs, and it opens straight onto the place you left.' },
      { tag: 'p', text: 'What you are reading is a sample. It ships inside the app, so it works before you sign in, on a plane, with no computer behind you at all. Everything else here behaves the same way it does with your own files.' },
      { tag: 'h2', text: 'Reading, not scrolling' },
      { tag: 'p', text: 'A page of prose wants a measure, not a viewport. Reader sets one: narrow, comfortable, or wide, and it keeps your choice. Text size is yours too, and both choices stay in this browser, not in the window that happened to be open.' },
      { tag: 'p', text: 'Search in a PDF finds the page. Search in an EPUB finds the paragraph and scrolls to it. Neither leaves the document to do it, and neither needs the network.' },
    ],
  },
  {
    title: 'What Reader opens',
    blocks: [
      { tag: 'p', text: 'PDFs render page by page with pdfjs, fitted to the width, fitted to the page, or at a zoom you set. EPUBs are unpacked in the browser: chapters become a table of contents, and the text becomes real, selectable, searchable paragraphs rather than a picture of text.' },
      { tag: 'li', text: 'PDF: fit width, fit page, 100, 150 or 200 percent, previous and next page, a page box, and text search across pages.' },
      { tag: 'li', text: 'EPUB: a chapter list, previous and next chapter, font size, measure, reading position saved to your computer, and search across chapters.' },
      { tag: 'p', text: 'The PDF and EPUB windows remain in the launcher, unchanged, for anything Reader has not learned to do yet.' },
    ],
  },
  {
    title: 'Where your place goes',
    blocks: [
      { tag: 'p', text: 'Reading position is the one thing Reader remembers, and it remembers it on your computer, not in this browser. Signed in, position is stored through the media progress route and follows you to any device. Signed out, only your text settings are kept, and only in this browser.' },
      { tag: 'p', text: 'That is the whole promise of this window: open a thing, read it, come back later, and be where you were. Nothing else about what you read leaves the computer.' },
    ],
  },
];

export const sampleModel = {
  format: 'epub',
  title: 'Welcome to Reader',
  author: 'Choir',
  status: 'sample',
  sample: true,
  sections: SAMPLE_SECTIONS.map((section, index) => ({
    id: 'sample-' + String(index + 1),
    title: section.title,
    blocks: section.blocks.map((block) => ({ tag: block.tag, text: block.text })),
  })),
};

export const sampleWordCount = sampleModel.sections.reduce(
  (total, section) => total + section.blocks.reduce((sum, block) => sum + cleanText(block.text).split(/\s+/).filter(Boolean).length, 0),
  0,
);
