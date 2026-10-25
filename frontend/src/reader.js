// The Reader preview page: /read.html.
//
// A small, sign-in-free surface that proves the Reader where nothing else can
// reach it: in a visual capture, offline, or before an account exists. The
// capture harness answers /auth/session with a definitive signed-out body and
// /api/ with 503, so this page never asks for either: it opens the sample
// document that ships beside it (a real PDF, written by sample-pdf.js at build
// time) and falls back to the in-app text sample when that file cannot be
// fetched. No DOM in reader-text.js, reader-prefs.js or pdf-text.js is
// browser-specific beyond what a browser has.
// The page's outer chrome (page bar, background, the --choir-* variables
// reader.css is built on). Imported here rather than linked from read.html so
// the build emits it at its hashed address and the page never links a path
// that does not exist.
import '../reader-chrome.css';
import './packages/reader/reader.css';
import {
  cleanText,
  detectFormat,
  modelChapterCount,
  modelWordCount,
  readingMinutes,
  searchDocuments,
  sectionsFromText,
} from './packages/reader/reader-text.js';
import {
  FONT_STEPS,
  MEASURE_NAMES,
  MEASURE_STEPS,
  READER_THEMES,
  loadPrefs,
  normalizePrefs,
  sampleModel,
  savePrefs,
} from './packages/reader/reader-prefs.js';
import { extractPdfText, modelFromPdfPages } from './packages/reader/pdf-text.js';

const SAMPLE_PDF_URL = '/read/sample-document.pdf';
const SAMPLE_HTML_URL = '/read/sample-document.html';

const state = {
  prefs: loadPrefs(window.localStorage),
  model: null,
  section: 0,
  pdfPages: [],
  pdfPage: 1,
  searchOpen: false,
  searchTerm: '',
  hits: [],
  hit: -1,
  source: 'loading',
  message: '',
};

const $ = (selector) => document.querySelector(selector);

function el(tag, attrs, children) {
  const node = document.createElement(tag);
  Object.entries(attrs || {}).forEach(([key, value]) => {
    if (key === 'class') node.className = value;
    else if (key === 'text') node.textContent = value;
    else if (key.startsWith('on')) node.addEventListener(key.slice(2), value);
    else if (value !== false && value != null) node.setAttribute(key, value === true ? '' : String(value));
  });
  for (const child of [].concat(children || [])) {
    if (child) node.appendChild(typeof child === 'string' ? document.createTextNode(child) : child);
  }
  return node;
}

function toneClass() {
  return 'reader-tone-' + state.prefs.theme;
}

function render() {
  const root = $('#reader-root');
  if (!root) return;
  root.className = 'reader-shell ' + toneClass();
  root.replaceChildren(header(), tools(), body(), footer());
}

function header() {
  const title = state.model?.title || 'Reader';
  const meta = [];
  meta.push(el('span', { class: 'reader-chip', 'data-reader-chip': state.model?.sample ? 'sample' : 'format', text: state.model?.sample ? 'Sample' : state.model?.format === 'pdf' ? 'PDF' : 'EPUB' }));
  if (state.model?.author) meta.push(el('span', { 'data-reader-author': '', text: state.model.author }));
  const words = modelWordCount(state.model);
  if (words) meta.push(el('span', { 'data-reader-words': '', text: words.toLocaleString() + ' words - ' + readingMinutes(state.model) + ' min' }));
  meta.push(el('span', { 'data-reader-preview-note': '', text: 'Signed-out preview' }));
  return el('header', { class: 'reader-top', 'data-reader-top': '' }, [
    el('div', { class: 'reader-identity' }, [el('h1', { 'data-reader-title': '', text: title }), el('p', {}, meta)]),
    el('div', { class: 'reader-top-actions' }, [
      el('button', {
        type: 'button',
        class: 'reader-icon-button',
        title: 'Table of contents',
        'aria-label': 'Table of contents',
        'data-reader-toc-toggle': '',
        text: '☰',
        onclick: () => $('#reader-toc')?.scrollIntoView({ block: 'nearest' }),
      }),
      el('button', {
        type: 'button',
        class: 'reader-icon-button',
        title: 'Search this document',
        'aria-label': 'Search this document',
        'data-reader-search-toggle': '',
        text: '⌕',
        onclick: () => {
          state.searchOpen = !state.searchOpen;
          render();
          if (state.searchOpen) $('#reader-search')?.focus();
        },
      }),
      el('a', { class: 'reader-button', href: '/', 'data-reader-signin': '', text: 'Open your own reader' }),
    ]),
  ]);
}

function tools() {
  const isPdf = state.model?.format === 'pdf';
  const list = [];
  list.push(
    el('div', { class: 'reader-group' }, [
      el('button', { type: 'button', class: 'reader-button', 'data-reader-prev': '', text: isPdf ? 'Previous page' : 'Previous chapter', onclick: () => move(-1) }),
      el('button', { type: 'button', class: 'reader-button', 'data-reader-next': '', text: isPdf ? 'Next page' : 'Next chapter', onclick: () => move(1) }),
    ]),
  );
  if (isPdf) {
    list.push(
      el('label', { class: 'reader-group' }, [
        el('span', { text: 'page' }),
        el('input', {
          class: 'reader-input reader-page-box',
          type: 'number',
          min: '1',
          value: String(state.pdfPage),
          'data-reader-page': '',
          onchange: (event) => goToPage(Number(event.target.value)),
        }),
        el('span', { 'data-reader-page-count': '', text: state.pdfPage + ' / ' + Math.max(1, state.pdfPages.length) }),
      ]),
    );
  } else {
    list.push(
      el('label', { class: 'reader-group', 'data-grow': 'true' }, [
        el('span', { text: 'chapter' }),
        el('select', {
          class: 'reader-select',
          'data-reader-toc': '',
          onchange: (event) => goToSection(Number(event.target.value)),
        }, (state.model?.sections || []).map((section, index) => el('option', { value: String(index), selected: index === state.section, text: index + 1 + '. ' + section.title }))),
      ]),
    );
  }
  list.push(
    el('div', { class: 'reader-group' }, [
      el('button', { type: 'button', class: 'reader-icon-button', 'aria-label': 'Smaller text', 'data-reader-font-smaller': '', text: 'A-', onclick: () => changePrefs({ fontSize: state.prefs.fontSize - 1 }) }),
      el('span', { 'data-reader-font-size': '', text: state.prefs.fontSize + 'px' }),
      el('button', { type: 'button', class: 'reader-icon-button', 'aria-label': 'Larger text', 'data-reader-font-larger': '', text: 'A+', onclick: () => changePrefs({ fontSize: state.prefs.fontSize + 1 }) }),
    ]),
  );
  if (!isPdf) {
    list.push(
      el('label', { class: 'reader-group' }, [
        el('span', { text: 'width' }),
        el('select', { class: 'reader-select', 'data-reader-measure': '', onchange: (event) => changePrefs({ measure: Number(event.target.value) }) },
          MEASURE_STEPS.map((step) => el('option', { value: String(step), selected: step === state.prefs.measure, text: MEASURE_NAMES[step] }))),
      ]),
    );
  }
  list.push(
    el('div', { class: 'reader-group', role: 'group', 'aria-label': 'Page tone' },
      READER_THEMES.map((theme) => el('button', {
        type: 'button',
        class: 'reader-button',
        'aria-pressed': theme === state.prefs.theme ? 'true' : 'false',
        'data-reader-tone': theme,
        text: theme,
        onclick: () => changePrefs({ theme }),
      }))),
  );
  return el('div', { class: 'reader-tools', role: 'toolbar', 'aria-label': 'Reading controls', 'data-reader-tools': '' }, list);
}

function body() {
  const isPdf = state.model?.format === 'pdf';
  const stage = el('div', { class: 'reader-stage', 'data-reader-stage': '', onscroll: updateProgress });
  if (state.source === 'loading') {
    stage.appendChild(el('p', { class: 'reader-status', 'aria-live': 'polite', text: 'Opening the sample document...' }));
  } else if (state.message) {
    stage.appendChild(el('div', { class: 'reader-alert', role: 'alert', 'data-reader-error': '' }, [
      el('strong', { text: 'Reader could not open the sample' }),
      el('span', { text: state.message }),
    ]));
  } else if (isPdf) {
    // A page is a list of paragraphs; an extractor that hands back a page as
    // one string is wrapped as [page] so the render never calls .map on a
    // string.
    const page = state.pdfPages[state.pdfPage - 1];
    const paragraphs = Array.isArray(page) ? page : page ? [page] : [];
    stage.appendChild(el('div', { class: 'reader-pdf-text', 'data-reader-pdf-text': '' },
      paragraphs.length
        ? paragraphs.map((paragraph, index) => el('p', { 'data-reader-block': String(index), 'data-reader-block-page': String(state.pdfPage), text: paragraph }))
        : [el('p', { class: 'reader-status', text: 'This page has no selectable text.' })]));
  } else {
    const section = state.model?.sections?.[state.section];
    const sheet = el('article', { class: 'reader-sheet', 'data-reader-sheet': '', style: '--reader-font-size: ' + state.prefs.fontSize + 'px; --reader-measure: ' + state.prefs.measure + ';' });
    if (section) {
      sheet.appendChild(el('p', { class: 'reader-status', 'data-reader-section-count': '', text: 'Chapter ' + (state.section + 1) + ' of ' + modelChapterCount(state.model) + ' - ' + section.title }));
      section.blocks.forEach((block, index) => {
        const tag = isHeadingBlock(block.tag) ? 'h3' : block.tag === 'blockquote' ? 'blockquote' : block.tag === 'pre' ? 'pre' : 'p';
        if (block.tag === 'li') sheet.appendChild(el('ul', {}, [el('li', { 'data-reader-block': String(index), text: block.text })]));
        else sheet.appendChild(el(tag, { 'data-reader-block': String(index), text: block.text }));
      });
    }
    stage.appendChild(sheet);
  }
  if (state.searchOpen) stage.appendChild(resultsPanel());
  const nav = [];
  nav.push(el('button', { type: 'button', class: 'reader-float-nav', 'data-side': 'prev', 'aria-label': 'Previous', text: '‹', onclick: () => move(-1) }));
  nav.push(el('button', { type: 'button', class: 'reader-float-nav', 'data-side': 'next', 'aria-label': 'Next', text: '›', onclick: () => move(1) }));
  const container = el('div', { class: 'reader-body', 'data-toc-open': 'false' }, [stage, ...nav]);
  return container;
}


function isHeadingBlock(tag) {
  return tag === 'h1' || tag === 'h2' || tag === 'h3' || tag === 'h4';
}

function resultsPanel() {
  const hits = state.hits.slice(0, 24);
  const panel = el('div', { class: 'reader-results', role: 'dialog', 'aria-label': 'Search results', 'data-reader-results': '' }, [
    el('header', {}, [
      el('label', { class: 'reader-group' }, [
        el('span', { text: 'find' }),
        el('input', {
          id: 'reader-search',
          class: 'reader-input',
          type: 'search',
          'data-reader-search': '',
          placeholder: 'Search this document',
          value: state.searchTerm,
          oninput: (event) => {
            state.searchTerm = event.target.value;
            runSearch();
            updateResults();
          },
          onkeydown: (event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              jump(state.hit + (event.shiftKey ? -1 : 1));
            }
          },
        }),
      ]),
      el('span', { 'data-reader-search-count': '', text: state.hits.length + ' matches' }),
      el('button', { type: 'button', class: 'reader-icon-button', 'aria-label': 'Close search', text: '×', onclick: () => { state.searchOpen = false; render(); } }),
    ]),
  ]);
  if (hits.length) {
    hits.forEach((hit, index) => panel.appendChild(el('button', {
      type: 'button',
      class: 'reader-hit',
      'data-reader-hit': '',
      'aria-current': index === state.hit ? 'true' : 'false',
      onclick: () => jump(index),
    }, [el('strong', { text: state.model.format === 'pdf' && hit.page ? 'Page ' + hit.page : hit.title }), el('span', { text: hit.snippet })])));
  } else if (state.searchTerm.trim()) {
    panel.appendChild(el('p', { class: 'reader-status', text: 'No matches for this document.' }));
  } else {
    panel.appendChild(el('p', { class: 'reader-status', text: 'Type to find text. Enter moves to the next match.' }));
  }
  return panel;
}

function footer() {
  const isPdf = state.model?.format === 'pdf';
  const progress = currentProgress();
  return el('footer', { class: 'reader-footer', 'data-reader-footer': '' }, [
    el('div', { class: 'reader-progress', role: 'progressbar', 'aria-valuemin': '0', 'aria-valuemax': '100', 'aria-valuenow': String(progress), 'data-reader-progress': '' }, [el('i', { style: 'width: ' + progress + '%' })]),
    el('span', { 'data-reader-progress-label': '', text: progress + '%' }),
    el('span', { 'data-reader-position': '', text: isPdf ? 'Page ' + state.pdfPage + ' / ' + Math.max(1, state.pdfPages.length) : 'Chapter ' + (state.section + 1) + ' / ' + modelChapterCount(state.model) }),
    el('span', { 'data-reader-sample-note': '', text: 'Runs without an account' }),
  ]);
}

// Search results update in place while typing, so the field keeps focus and
// the caret: only the results section is replaced.
function updateResults() {
  const panel = document.querySelector('[data-reader-results]');
  if (!panel) return;
  panel.replaceWith(resultsPanel());
  const field = document.querySelector('#reader-search');
  if (field && document.activeElement !== field) field.focus();
}

function currentProgress() {
  if (state.model?.format === 'pdf') {
    const total = Math.max(1, state.pdfPages.length);
    return Math.round(((state.pdfPage - 1) / Math.max(1, total - 1)) * 100);
  }
  const total = Math.max(1, modelChapterCount(state.model));
  return Math.round(((state.section + 1) / total) * 100);
}

function updateProgress() {
  const label = $('#reader-root [data-reader-progress-label]');
  if (label) label.textContent = currentProgress() + '%';
}

function changePrefs(next) {
  state.prefs = normalizePrefs({ ...state.prefs, ...next });
  savePrefs(window.localStorage, state.prefs);
  render();
}

function goToPage(page) {
  const total = Math.max(1, state.pdfPages.length);
  state.pdfPage = Math.min(total, Math.max(1, Math.floor(page) || 1));
  render();
}

function goToSection(index) {
  const total = Math.max(1, modelChapterCount(state.model));
  state.section = Math.min(total - 1, Math.max(0, Math.floor(index) || 0));
  render();
}

function move(delta) {
  if (state.model?.format === 'pdf') goToPage(state.pdfPage + delta);
  else goToSection(state.section + delta);
}

function runSearch() {
  const needle = cleanText(state.searchTerm);
  state.hits = needle && state.model ? searchDocuments(state.model, needle) : [];
  state.hit = state.hits.length ? 0 : -1;
}

function jump(index) {
  if (!state.hits.length) return;
  const wrapped = ((index % state.hits.length) + state.hits.length) % state.hits.length;
  state.hit = wrapped;
  const hit = state.hits[wrapped];
  if (state.model.format === 'pdf' && hit.page) goToPage(hit.page);
  else goToSection(hit.sectionIndex);
  const target = document.querySelector('[data-reader-block="' + hit.blockIndex + '"]');
  target?.scrollIntoView({ block: 'center' });
}

async function loadSample() {
  try {
    const res = await fetch(SAMPLE_PDF_URL, { credentials: 'omit' });
    if (!res.ok) throw new Error('The sample PDF answered ' + res.status);
    const bytes = new Uint8Array(await res.arrayBuffer());
    const extracted = await extractPdfText(bytes);
    if (!extracted.sawText) throw new Error('The sample PDF has no extractable text here');
    state.pdfPages = extracted.pages;
    state.model = modelFromPdfPages(extracted.pages, { title: 'Reader sample', fileName: 'sample-document.pdf' });
    state.source = 'ready';
  } catch (pdfError) {
    try {
      const res = await fetch(SAMPLE_HTML_URL, { credentials: 'omit' });
      if (!res.ok) throw pdfError;
      const html = await res.text();
      const doc = new DOMParser().parseFromString(html, 'text/html');
      const sections = sectionsFromText(doc.body?.textContent || '', { title: 'Reader sample' });
      if (!sections.length) throw pdfError;
      state.model = { format: 'epub', title: 'Reader sample', author: 'Choir', sample: true, sections };
      state.source = 'ready';
    } catch (htmlError) {
      state.model = sampleModel;
      state.source = 'ready';
      state.message = '';
    }
  }
  render();
}

window.addEventListener('keydown', (event) => {
  if (event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement) return;
  if (event.key === 'ArrowRight' || event.key === 'PageDown') move(1);
  else if (event.key === 'ArrowLeft' || event.key === 'PageUp') move(-1);
  else if (event.key === 'f' && (event.metaKey || event.ctrlKey)) {
    event.preventDefault();
    state.searchOpen = true;
    render();
  }
});

render();
void loadSample();
