<script>
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import {
    appTitle,
    clampNumber,
    loadRecentMedia,
    loadMediaProgress,
    loadContextContentItem,
    mediaSourceIdentity,
    recentMediaAppContext,
    rememberRecentMedia,
    resolveMediaSource,
    saveMediaPosition,
  } from './media-utils.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let appContext = {};
  export let windowId = '';

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
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: extractedText = item?.text_content || effectiveContext.textContent || '';
  $: activeChapter = chapters[activeChapterIndex] || null;
  $: readerTitle = epubTitle || source.title;
  $: sourceIdentity = mediaSourceIdentity(source);
  $: if (source.displayUrl && sourceIdentity && sourceIdentity !== rememberedIdentity) {
    void rememberCurrentSource();
  }

  async function refreshRecentFiles() {
    recentFiles = await loadRecentMedia(kind);
  }

  async function rememberCurrentSource() {
    rememberedIdentity = sourceIdentity;
    if (await rememberRecentMedia(kind, source)) {
      await refreshRecentFiles();
    }
  }

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

  async function loadStoredPosition() {
    try {
      const progress = await loadMediaProgress(kind, source);
      if (Number.isFinite(progress.currentTime)) {
        activeChapterIndex = clampNumber(Math.floor(progress.currentTime), 0, Math.max(0, chapters.length - 1));
        readerProgress = clampNumber(Math.round((progress.currentTime - activeChapterIndex) * 100), 0, 100);
      }
    } catch (_err) {
      // Reader progress sync is additive; loading must not block reading.
    }
  }

  function savePosition() {
    saveMediaPosition(kind, source, activeChapterIndex + (readerProgress / 100), Math.max(1, chapters.length));
  }

  function setActiveChapter(index) {
    activeChapterIndex = clampNumber(index, 0, Math.max(0, chapters.length - 1));
    tick().then(() => {
      if (scrollEl) scrollEl.scrollTop = 0;
      updateEpubSearch();
      savePosition();
    });
  }

  function moveChapter(delta) {
    if (!chapters.length) return;
    setActiveChapter(activeChapterIndex + delta);
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
      await loadStoredPosition();
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
    const result = await loadContextContentItem(effectiveContext, item, appTitle(kind));
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
      await loadStoredPosition();
      return;
    }
    await loadEpubArchive();
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
    chapters = [];
    activeChapterIndex = 0;
    epubTitle = '';
    epubSearchMatches = [];
    loadedSourceKey = '';
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    await loadContentItem();
  }

  onMount(() => {
    void refreshRecentFiles();
    void loadContentItem();
    const removeLiveListener = addLiveEventListener((message) => {
      if (liveEventKind(message) === 'media.recent.updated' && liveEventPayload(message).kind === kind) {
        void refreshRecentFiles();
      }
    });
    return () => removeLiveListener();
  });
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

    <div class="epub-page-nav" data-epub-page-nav>
      <button type="button" on:click={() => moveChapter(-1)} disabled={activeChapterIndex <= 0} aria-label="Previous EPUB chapter" title="Previous chapter" data-epub-prev-float>&lt;</button>
      <button type="button" on:click={() => moveChapter(1)} disabled={activeChapterIndex >= chapters.length - 1} aria-label="Next EPUB chapter" title="Next chapter" data-epub-next-float>&gt;</button>
    </div>

    <details class="epub-controls" data-media-toolbar data-epub-toolbar data-media-controls>
      <summary aria-label="EPUB controls" title="EPUB controls"><span aria-hidden="true">...</span></summary>
      <div class="epub-toolbar">
        <button type="button" on:click={() => moveChapter(-1)} disabled={activeChapterIndex <= 0} data-epub-prev>Prev</button>
        <button type="button" on:click={() => moveChapter(1)} disabled={activeChapterIndex >= chapters.length - 1} data-epub-next>Next</button>
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
      {#if recentFiles.length}
        <div class="epub-recent" data-media-recent-list>
          <span>Recently opened</span>
          {#each recentFiles as recent}
            <button type="button" data-media-recent-item on:click={() => openRecentFile(recent)}>
              <strong>{recent.title}</strong>
              <small>{recent.filePath || recent.sourceUrl}</small>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  {#if !loading && !error}
    <details class="epub-meta">
      <summary aria-label="EPUB info" title="EPUB info"><span aria-hidden="true">i</span></summary>
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
    left: 0;
    width: max-content;
    max-width: min(760px, calc(100% - 20px));
    max-height: calc(100% - 20px);
    color: #cbd5e1;
    overflow: auto;
    transform: translateX(-34%);
  }

  .epub-controls summary {
    display: grid;
    width: 32px;
    height: 32px;
    place-items: center;
    border: 1px solid rgba(99, 153, 255, 0.28);
    border-radius: 999px;
    background: rgba(8, 14, 28, 0.54);
    backdrop-filter: blur(12px);
    cursor: pointer;
    font-size: 0;
    font-weight: 820;
    list-style: none;
    padding: 0;
  }

  .epub-controls summary::-webkit-details-marker {
    display: none;
  }

  .epub-controls summary span {
    font-size: 0.92rem;
    line-height: 1;
  }

  .epub-controls[open] {
    left: 10px;
    border: 1px solid rgba(99, 153, 255, 0.28);
    border-radius: 12px;
    background: rgba(8, 14, 28, 0.86);
    backdrop-filter: blur(12px);
    transform: none;
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

  .epub-toolbar button:disabled {
    cursor: not-allowed;
    opacity: 0.52;
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
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    padding: 0;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
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
    display: grid;
    min-height: 0;
    place-content: center;
    gap: 12px;
    padding: 16px;
    overflow: auto;
  }

  .epub-empty > * {
    width: min(100%, 520px);
  }

  .epub-page-nav {
    position: absolute;
    inset: 0;
    z-index: 3;
    pointer-events: none;
  }

  .epub-page-nav button {
    display: grid;
    position: absolute;
    top: 50%;
    width: 30px;
    height: 54px;
    place-items: center;
    border: 1px solid rgba(126, 180, 255, 0.22);
    border-radius: 999px;
    background: rgba(5, 10, 22, 0.34);
    color: #eaf2ff;
    cursor: pointer;
    font: inherit;
    font-size: 1.3rem;
    font-weight: 900;
    line-height: 1;
    opacity: 0.34;
    pointer-events: auto;
    backdrop-filter: blur(10px);
  }

  .epub-page-nav button:first-child {
    left: 0;
    transform: translate(-42%, -50%);
  }

  .epub-page-nav button:last-child {
    right: 0;
    transform: translate(42%, -50%);
  }

  .epub-page-nav button:hover:not(:disabled),
  .epub-page-nav button:focus-visible {
    opacity: 0.92;
  }

  .epub-page-nav button:disabled {
    cursor: default;
    opacity: 0.14;
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

  .epub-recent {
    display: grid;
    gap: 8px;
    border: 1px solid rgba(126, 180, 255, 0.2);
    border-radius: 16px;
    background: rgba(8, 14, 28, 0.74);
    padding: 12px;
  }

  .epub-recent > span {
    color: #93c5fd;
    font-size: 0.74rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .epub-recent button {
    display: grid;
    gap: 2px;
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 11px;
    background: rgba(255, 255, 255, 0.055);
    color: #e5eefc;
    cursor: pointer;
    padding: 9px 10px;
    text-align: left;
  }

  .epub-recent button:hover,
  .epub-recent button:focus-visible {
    border-color: rgba(96, 165, 250, 0.45);
    background: rgba(96, 165, 250, 0.12);
  }

  .epub-recent strong,
  .epub-recent small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .epub-recent small {
    color: #94a3b8;
    font-size: 0.74rem;
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
    width: max-content;
    max-width: min(520px, calc(100% - 20px));
    color: #a8adbd;
  }

  .epub-meta:not([open]) {
    right: 0;
    transform: translateX(34%);
  }

  .epub-meta summary {
    display: grid;
    width: 30px;
    height: 30px;
    place-items: center;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 999px;
    background: rgba(10, 15, 27, 0.52);
    backdrop-filter: blur(12px);
    cursor: pointer;
    color: #dbeafe;
    font-size: 0;
    font-weight: 800;
    list-style: none;
    margin-left: auto;
    padding: 0;
  }

  .epub-meta summary::-webkit-details-marker {
    display: none;
  }

  .epub-meta summary span {
    font-size: 0.86rem;
    line-height: 1;
  }

  .epub-meta[open] {
    left: 10px;
    right: 10px;
    width: auto;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 10px;
    padding: 7px 9px;
    background: rgba(10, 15, 27, 0.76);
    backdrop-filter: blur(12px);
    transform: none;
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
      top: 6px;
      max-width: calc(100% - 16px);
    }

    .epub-controls[open] {
      left: 6px;
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
