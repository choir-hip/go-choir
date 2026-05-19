<script>
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import {
    appTitle,
    clampNumber,
    loadContextContentItem,
    resolveMediaSource,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'epub';
  const dispatch = createEventDispatcher();

  let jszipPromise = null;
  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let readerFontSize = 18;
  let readerMeasure = 72;
  let readerProgress = 0;
  let chapters = [];
  let activeChapterIndex = 0;
  let epubTitle = '';
  let epubSearch = '';
  let epubSearchMatches = [];
  let scrollEl = null;
  let loadedSourceKey = '';

  $: source = resolveMediaSource(appContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: extractedText = item?.text_content || appContext.textContent || '';
  $: activeChapter = chapters[activeChapterIndex] || null;
  $: readerTitle = epubTitle || source.title;
  $: positionKey = `choir-epub-position:${source.filePath || source.sourceUrl || source.title || 'untitled'}`;

  function sourceFetchOptions(url) {
    return String(url || '').startsWith('/') ? { credentials: 'include' } : { credentials: 'omit' };
  }

  async function loadJSZip() {
    if (!jszipPromise) {
      jszipPromise = import('jszip').then((module) => module.default);
    }
    return jszipPromise;
  }

  function dirname(path) {
    const index = String(path || '').lastIndexOf('/');
    return index >= 0 ? path.slice(0, index + 1) : '';
  }

  function resolvePath(base, href) {
    if (!href) return '';
    if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return href;
    const stack = dirname(base).split('/').filter(Boolean);
    for (const part of href.split('/')) {
      if (!part || part === '.') continue;
      if (part === '..') stack.pop();
      else stack.push(part);
    }
    return stack.join('/');
  }

  function parseXML(text, label) {
    const doc = new DOMParser().parseFromString(text, 'application/xml');
    const parserError = doc.querySelector('parsererror');
    if (parserError) throw new Error(`${label} XML could not be parsed`);
    return doc;
  }

  function elementsByLocalName(root, localName) {
    return Array.from(root.getElementsByTagName('*')).filter((element) => element.localName === localName);
  }

  function firstByLocalName(root, localName) {
    return elementsByLocalName(root, localName)[0] || null;
  }

  function cleanText(value) {
    return String(value || '').replace(/\s+/g, ' ').trim();
  }

  function titleFromDoc(doc, fallback) {
    return cleanText(doc.querySelector('h1,h2,h3,title')?.textContent) || fallback;
  }

  function chapterBlocksFromDoc(doc) {
    const body = doc.body || doc.querySelector('body');
    if (!body) return [];
    const nodes = Array.from(body.querySelectorAll('h1,h2,h3,h4,p,li,blockquote,pre'));
    const blocks = nodes
      .map((node) => ({ tag: node.tagName?.toLowerCase() || 'p', text: cleanText(node.textContent) }))
      .filter((block) => block.text);
    if (blocks.length) return blocks;
    const text = cleanText(body.textContent);
    return text ? [{ tag: 'p', text }] : [];
  }

  async function textFile(zip, path) {
    const file = zip.file(path);
    return file ? file.async('text') : '';
  }

  function loadStoredPosition() {
    try {
      const raw = window.localStorage.getItem(positionKey);
      if (!raw) return;
      const parsed = JSON.parse(raw);
      if (Number.isFinite(parsed.chapter)) {
        activeChapterIndex = clampNumber(parsed.chapter, 0, Math.max(0, chapters.length - 1));
      }
      tick().then(() => {
        if (scrollEl && Number.isFinite(parsed.scrollTop)) {
          scrollEl.scrollTop = parsed.scrollTop;
          updateReaderProgress({ currentTarget: scrollEl });
        }
      });
    } catch (_err) {
      // Ignore corrupt local reader state.
    }
  }

  function savePosition() {
    try {
      window.localStorage.setItem(positionKey, JSON.stringify({
        chapter: activeChapterIndex,
        scrollTop: scrollEl?.scrollTop || 0,
        progress: readerProgress,
      }));
    } catch (_err) {
      // Local persistence is best-effort.
    }
  }

  function setActiveChapter(index) {
    activeChapterIndex = clampNumber(index, 0, Math.max(0, chapters.length - 1));
    tick().then(() => {
      if (scrollEl) scrollEl.scrollTop = 0;
      updateEpubSearch();
      savePosition();
    });
  }

  function updateReaderProgress(event) {
    const el = event.currentTarget;
    const scrollable = Math.max(0, el.scrollHeight - el.clientHeight);
    readerProgress = scrollable > 0 ? Math.round((el.scrollTop / scrollable) * 100) : 0;
    savePosition();
  }

  function changeReaderSize(delta) {
    readerFontSize = clampNumber(readerFontSize + delta, 14, 28);
  }

  function updateEpubSearch() {
    const needle = cleanText(epubSearch).toLowerCase();
    if (!needle) {
      epubSearchMatches = [];
      return;
    }
    const matches = [];
    chapters.forEach((chapter, chapterIndex) => {
      chapter.blocks.forEach((block, blockIndex) => {
        const haystack = block.text.toLowerCase();
        const matchIndex = haystack.indexOf(needle);
        if (matchIndex >= 0) {
          const start = Math.max(0, matchIndex - 42);
          const end = Math.min(block.text.length, matchIndex + needle.length + 72);
          matches.push({
            chapterIndex,
            blockIndex,
            title: chapter.title,
            snippet: `${start > 0 ? '...' : ''}${block.text.slice(start, end)}${end < block.text.length ? '...' : ''}`,
          });
        }
      });
    });
    epubSearchMatches = matches;
  }

  function jumpToMatch(match) {
    setActiveChapter(match.chapterIndex);
    tick().then(() => {
      const target = scrollEl?.querySelector(`[data-epub-block-index="${match.blockIndex}"]`);
      target?.scrollIntoView({ block: 'center' });
    });
  }

  function extractedTextChapters(text) {
    return [{
      href: 'extracted-text',
      title: source.title,
      blocks: String(text || '')
        .split(/\n{2,}/)
        .map((part) => ({ tag: 'p', text: cleanText(part) }))
        .filter((block) => block.text),
    }];
  }

  async function parseEpubArchive(arrayBuffer) {
    const JSZip = await loadJSZip();
    const zip = await JSZip.loadAsync(arrayBuffer);
    const containerText = await textFile(zip, 'META-INF/container.xml');
    if (!containerText) throw new Error('EPUB container.xml is missing');
    const container = parseXML(containerText, 'container');
    const opfPath = firstByLocalName(container, 'rootfile')?.getAttribute('full-path');
    if (!opfPath) throw new Error('EPUB package path is missing');
    const opfText = await textFile(zip, opfPath);
    if (!opfText) throw new Error('EPUB package document is missing');
    const opf = parseXML(opfText, 'package');
    epubTitle = cleanText(firstByLocalName(opf, 'title')?.textContent) || source.title;

    const manifest = new Map();
    elementsByLocalName(opf, 'item').forEach((entry) => {
      const id = entry.getAttribute('id');
      const href = entry.getAttribute('href');
      if (id && href) {
        manifest.set(id, {
          href: resolvePath(opfPath, href),
          mediaType: entry.getAttribute('media-type') || '',
          properties: entry.getAttribute('properties') || '',
        });
      }
    });

    const spineItems = elementsByLocalName(opf, 'itemref')
      .map((entry) => manifest.get(entry.getAttribute('idref')))
      .filter(Boolean)
      .filter((entry) => /xhtml|html/i.test(entry.mediaType) || /\.x?html?$/i.test(entry.href));

    if (!spineItems.length) throw new Error('EPUB spine has no readable chapters');

    const parsedChapters = [];
    for (let index = 0; index < spineItems.length; index++) {
      const entry = spineItems[index];
      const chapterText = await textFile(zip, entry.href);
      if (!chapterText) continue;
      const doc = new DOMParser().parseFromString(chapterText, 'text/html');
      const blocks = chapterBlocksFromDoc(doc);
      if (!blocks.length) continue;
      parsedChapters.push({
        href: entry.href,
        title: titleFromDoc(doc, `Chapter ${index + 1}`),
        blocks,
      });
    }

    if (!parsedChapters.length) throw new Error('EPUB chapters did not contain readable text');
    return parsedChapters;
  }

  async function loadEpubArchive() {
    if (!sourceKey || loadedSourceKey === sourceKey || extractedText) return;
    loadedSourceKey = sourceKey;
    loading = true;
    error = '';
    try {
      const res = await fetch(sourceKey, sourceFetchOptions(sourceKey));
      if (!res.ok) throw new Error(`EPUB source failed (${res.status})`);
      const data = await res.arrayBuffer();
      chapters = await parseEpubArchive(data);
      activeChapterIndex = 0;
      updateEpubSearch();
      await tick();
      loadStoredPosition();
    } catch (err) {
      const message = err?.message || 'EPUB load failed';
      error = `${message}. The EPUB app needs a valid browser-fetchable EPUB archive; CORS-blocked remote files should be imported into Files first.`;
    } finally {
      loading = false;
    }
  }

  async function loadContentItem() {
    loading = true;
    error = '';
    const result = await loadContextContentItem(appContext, item, appTitle(kind));
    loading = false;
    if (result.authRequired) {
      dispatch('authexpired');
      return;
    }
    if (result.error) {
      error = result.error;
      return;
    }
    if (result.item) item = result.item;
    if (extractedText) {
      chapters = extractedTextChapters(extractedText);
      epubTitle = source.title;
      await tick();
      loadStoredPosition();
      return;
    }
    await loadEpubArchive();
  }

  onMount(loadContentItem);
</script>

<section class="epub-app" data-media-app data-media-kind="epub" data-epub-app>
  {#if loading && !chapters.length}
    <p class="epub-status">Loading EPUB...</p>
  {:else if error}
    <div class="epub-blocker" role="alert" data-epub-blocker>
      <strong>EPUB reader could not open this source.</strong>
      <span>{error}</span>
    </div>
  {:else if chapters.length}
    <div class="epub-scroll" data-media-stage data-epub-scroll on:scroll={updateReaderProgress} bind:this={scrollEl}>
      <article
        class="epub-reader"
        data-epub-reader
        style={`--reader-font-size: ${readerFontSize}px; --reader-measure: ${readerMeasure}ch;`}
      >
        <header class="epub-chapter-header">
          <span data-epub-chapter-count>{activeChapterIndex + 1} / {chapters.length}</span>
          <h3 data-epub-chapter-title>{activeChapter.title}</h3>
        </header>
        {#each activeChapter.blocks as block, index}
          {#if ['h1', 'h2', 'h3', 'h4'].includes(block.tag)}
            <h4 data-epub-block-index={index}>{block.text}</h4>
          {:else if block.tag === 'blockquote'}
            <blockquote data-epub-block-index={index}>{block.text}</blockquote>
          {:else if block.tag === 'pre'}
            <pre data-epub-block-index={index}>{block.text}</pre>
          {:else}
            <p data-epub-block-index={index}>{block.text}</p>
          {/if}
        {/each}
      </article>
    </div>

    <details class="epub-controls" data-media-toolbar data-epub-toolbar data-media-controls>
      <summary>Controls</summary>
      <div class="epub-toolbar">
        <button type="button" on:click={() => changeReaderSize(-1)} data-epub-font-smaller>-</button>
        <span data-epub-font-size>{readerFontSize}px</span>
        <button type="button" on:click={() => changeReaderSize(1)} data-epub-font-larger>+</button>
        <label>
          Width
          <select bind:value={readerMeasure} data-epub-width>
            <option value={58}>Narrow</option>
            <option value={72}>Comfort</option>
            <option value={88}>Wide</option>
          </select>
        </label>
        <label>
          Chapter
          <select value={activeChapterIndex} on:change={(event) => setActiveChapter(Number(event.currentTarget.value))} data-epub-toc>
            {#each chapters as chapter, index}
              <option value={index}>{index + 1}. {chapter.title}</option>
            {/each}
          </select>
        </label>
        <span data-epub-progress>{readerProgress}%</span>
        <label class="reader-search">
          Search
          <input type="search" bind:value={epubSearch} on:input={updateEpubSearch} placeholder="Find text" data-epub-search />
        </label>
        {#if epubSearch.trim()}
          <span data-epub-search-count>{epubSearchMatches.length} matches</span>
        {/if}
      </div>
      {#if epubSearchMatches.length}
        <div class="reader-results" data-epub-search-results>
          {#each epubSearchMatches.slice(0, 8) as match}
            <button type="button" on:click={() => jumpToMatch(match)} data-epub-search-result>
              <strong>{match.title}</strong>
              <span>{match.snippet}</span>
            </button>
          {/each}
        </div>
      {/if}
    </details>
  {:else}
    <div class="epub-empty" data-media-stage>
      <div class="epub-blocker" data-epub-blocker>
        <strong>EPUB reader needs a readable source.</strong>
        <span>No EPUB file or extracted text is attached to this window.</span>
        {#if source.filePath}<span>File: {source.filePath}</span>{/if}
      </div>
    </div>
  {/if}

  {#if !loading && !error}
    <details class="epub-meta">
      <summary>Info</summary>
      <h2 data-media-title>{readerTitle}</h2>
      <dl>
        {#if source.sourceUrl}<dt>Source</dt><dd><a href={source.sourceUrl} target="_blank" rel="noreferrer" data-media-open-source>{source.sourceUrl}</a></dd>{/if}
        {#if source.filePath}<dt>File</dt><dd>{source.filePath}</dd>{/if}
        {#if source.mediaType}<dt>Type</dt><dd>{source.mediaType}</dd>{/if}
        {#if item?.content_hash}<dt>Hash</dt><dd>{item.content_hash}</dd>{/if}
      </dl>
    </details>
  {/if}
</section>

<style>
  .epub-app {
    position: relative;
    display: block;
    height: 100%;
    min-height: 0;
    color: #f5f7ff;
    background: #050814;
    overflow: hidden;
  }

  .epub-controls {
    position: absolute;
    z-index: 4;
    top: 10px;
    left: 10px;
    max-width: min(760px, calc(100% - 20px));
    max-height: calc(100% - 20px);
    border: 1px solid rgba(99, 153, 255, 0.28);
    border-radius: 12px;
    background: rgba(8, 14, 28, 0.86);
    color: #cbd5e1;
    overflow: auto;
    backdrop-filter: blur(12px);
  }

  .epub-controls summary {
    cursor: pointer;
    font-weight: 820;
    padding: 8px 10px;
  }

  .epub-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 0 8px 8px;
  }

  .epub-toolbar button {
    min-height: 34px;
    border: 1px solid rgba(126, 180, 255, 0.32);
    border-radius: 9px;
    background: rgba(37, 64, 108, 0.72);
    color: #eef5ff;
    cursor: pointer;
    font-weight: 760;
    padding: 7px 10px;
  }

  .epub-toolbar button:hover {
    background: rgba(56, 96, 160, 0.82);
  }

  .epub-toolbar span,
  .epub-toolbar label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: #a8adbd;
    font-size: 0.84rem;
  }

  .epub-toolbar select,
  .epub-toolbar input[type='search'] {
    border: 1px solid rgba(99, 153, 255, 0.34);
    border-radius: 8px;
    background: rgba(5, 10, 22, 0.72);
    color: #f8fbff;
    padding: 7px 8px;
  }

  .epub-scroll {
    position: absolute;
    inset: 0;
    min-height: 0;
    background: #070b16;
    overflow: auto;
  }

  .epub-reader {
    max-width: var(--reader-measure, 72ch);
    margin: 0 auto;
    padding: 32px;
    color: #e8eefc;
    font-size: var(--reader-font-size, 18px);
    line-height: 1.62;
  }

  .epub-reader p,
  .epub-reader blockquote,
  .epub-reader pre,
  .epub-reader h4 {
    overflow-wrap: anywhere;
  }

  .epub-reader h4 {
    margin: 1.3em 0 0.45em;
    color: #f8fbff;
    font-size: 1.08em;
  }

  .epub-reader blockquote {
    margin-left: 0;
    border-left: 3px solid rgba(147, 197, 253, 0.56);
    padding-left: 16px;
    color: #cbd5e1;
  }

  .epub-reader pre {
    border-radius: 10px;
    padding: 12px;
    background: rgba(2, 6, 23, 0.72);
    white-space: pre-wrap;
  }

  .epub-chapter-header {
    margin-bottom: 18px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.22);
    padding-bottom: 12px;
  }

  .epub-chapter-header span {
    color: var(--choir-muted, #a8adbd);
    font-size: 0.78rem;
    font-weight: 820;
    text-transform: uppercase;
  }

  .epub-chapter-header h3 {
    margin: 4px 0 0;
    font-size: 1.35em;
  }

  .epub-empty {
    flex: 1 1 auto;
    min-height: 0;
    border: 1px solid rgba(120, 135, 170, 0.18);
    border-radius: 12px;
    background: #070b16;
    overflow: auto;
  }

  .epub-status,
  .epub-blocker {
    margin: 0;
    border-radius: 14px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
    color: #a8adbd;
  }

  .epub-blocker {
    display: grid;
    gap: 6px;
    color: #dce6ff;
  }

  .reader-search input {
    width: min(180px, 35vw);
  }

  .reader-results {
    display: grid;
    gap: 6px;
    max-height: 132px;
    padding: 0 8px 8px;
    overflow: auto;
  }

  .reader-results button {
    display: grid;
    gap: 3px;
    border: 1px solid rgba(99, 153, 255, 0.28);
    border-radius: 9px;
    background: rgba(13, 24, 44, 0.82);
    color: #eaf2ff;
    cursor: pointer;
    padding: 8px 10px;
    text-align: left;
  }

  .reader-results span {
    color: var(--choir-muted, #a8adbd);
    font-size: 0.82rem;
  }

  .epub-meta {
    position: absolute;
    z-index: 4;
    right: 10px;
    bottom: 10px;
    left: 10px;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 10px;
    padding: 7px 9px;
    background: rgba(10, 15, 27, 0.76);
    color: #a8adbd;
    backdrop-filter: blur(12px);
  }

  .epub-meta summary {
    cursor: pointer;
    font-weight: 800;
  }

  .epub-meta h2 {
    margin: 10px 0;
    color: #f8fbff;
    font-size: 1rem;
    overflow-wrap: anywhere;
  }

  .epub-meta dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px 10px;
    margin: 0 0 4px;
  }

  .epub-meta dt {
    color: #dbeafe;
    font-weight: 760;
  }

  .epub-meta dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .epub-meta a {
    color: #bfdbfe;
  }

  @media (max-width: 720px) {
    .epub-reader {
      padding: 22px;
    }

    .epub-controls {
      top: 8px;
      left: 8px;
      max-width: calc(100% - 16px);
    }

    .epub-toolbar {
      gap: 6px;
      padding: 0 6px 6px;
    }

    .reader-search {
      flex: 1 1 100%;
    }

    .reader-search input {
      width: 100%;
    }
  }
</style>
