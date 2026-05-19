<script>
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import {
    appTitle,
    clampNumber,
    loadContextContentItem,
    resolveMediaSource,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'pdf';
  const dispatch = createEventDispatcher();

  let pdfjsLibPromise = null;
  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let pdfDoc = null;
  let pdfPage = 1;
  let pageCount = 0;
  let pdfZoom = 'page-width';
  let pdfSearch = '';
  let pdfSearchMatches = [];
  let pdfTextByPage = [];
  let canvasEl = null;
  let stageEl = null;
  let rendered = false;
  let rendering = false;
  let renderSeq = 0;
  let loadedSourceKey = '';
  let resizeObserver = null;

  $: source = resolveMediaSource(appContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: pageLabel = pageCount ? `${pdfPage} / ${pageCount}` : String(pdfPage);

  function sourceFetchOptions(url) {
    return String(url || '').startsWith('/') ? { credentials: 'include' } : { credentials: 'omit' };
  }

  async function loadPdfjs() {
    if (!pdfjsLibPromise) {
      pdfjsLibPromise = Promise.all([
        import('pdfjs-dist'),
        import('pdfjs-dist/build/pdf.worker.mjs?url'),
      ]).then(([pdfjs, worker]) => {
        pdfjs.GlobalWorkerOptions.workerSrc = worker.default;
        return pdfjs;
      });
    }
    return pdfjsLibPromise;
  }

  function setPdfPage(nextPage) {
    const maxPage = pageCount || Number.MAX_SAFE_INTEGER;
    pdfPage = clampNumber(Math.floor(Number(nextPage) || 1), 1, maxPage);
    void renderCurrentPage();
  }

  function normalizeText(value) {
    return String(value || '').toLowerCase();
  }

  function buildSnippet(text, needle) {
    const haystack = normalizeText(text);
    const index = haystack.indexOf(needle);
    if (index < 0) return '';
    const start = Math.max(0, index - 42);
    const end = Math.min(text.length, index + needle.length + 72);
    return `${start > 0 ? '...' : ''}${text.slice(start, end)}${end < text.length ? '...' : ''}`;
  }

  function updatePdfSearch() {
    const needle = normalizeText(pdfSearch).trim();
    if (!needle) {
      pdfSearchMatches = [];
      return;
    }
    pdfSearchMatches = pdfTextByPage
      .map((text, index) => ({ page: index + 1, snippet: buildSnippet(text, needle) }))
      .filter((match) => match.snippet);
  }

  function nextSearchMatch(direction) {
    if (!pdfSearchMatches.length) return;
    const exactIndex = pdfSearchMatches.findIndex((match) => match.page === pdfPage);
    const afterIndex = pdfSearchMatches.findIndex((match) => match.page > pdfPage);
    const currentIndex = exactIndex >= 0 ? exactIndex : (afterIndex >= 0 ? afterIndex : 0);
    let nextIndex = currentIndex + direction;
    if (exactIndex < 0 && direction > 0) nextIndex = currentIndex;
    if (nextIndex < 0) nextIndex = pdfSearchMatches.length - 1;
    if (nextIndex >= pdfSearchMatches.length) nextIndex = 0;
    setPdfPage(pdfSearchMatches[nextIndex].page);
  }

  async function extractPdfText(doc) {
    const pages = [];
    for (let index = 1; index <= doc.numPages; index++) {
      const page = await doc.getPage(index);
      const textContent = await page.getTextContent();
      pages.push(textContent.items.map((entry) => entry.str || '').join(' ').replace(/\s+/g, ' ').trim());
    }
    pdfTextByPage = pages;
    updatePdfSearch();
  }

  function zoomScale(page, mode) {
    const defaultViewport = page.getViewport({ scale: 1 });
    const stageRect = stageEl?.getBoundingClientRect();
    const availableWidth = Math.max(220, (stageRect?.width || 760) - 34);
    const availableHeight = Math.max(220, (stageRect?.height || 560) - 34);
    if (mode === 'page-width') return clampNumber(availableWidth / defaultViewport.width, 0.2, 5);
    if (mode === 'page-fit') {
      return clampNumber(Math.min(availableWidth / defaultViewport.width, availableHeight / defaultViewport.height), 0.2, 5);
    }
    return clampNumber((Number(mode) || 100) / 100, 0.2, 5);
  }

  async function renderCurrentPage() {
    const seq = ++renderSeq;
    if (!pdfDoc || !canvasEl) return;
    rendering = true;
    rendered = false;
    try {
      await tick();
      const page = await pdfDoc.getPage(pdfPage);
      if (seq !== renderSeq) return;
      const scale = zoomScale(page, pdfZoom);
      const viewport = page.getViewport({ scale });
      const outputScale = Math.max(1, window.devicePixelRatio || 1);
      const canvas = canvasEl;
      const context = canvas.getContext('2d');
      canvas.width = Math.floor(viewport.width * outputScale);
      canvas.height = Math.floor(viewport.height * outputScale);
      canvas.style.width = `${Math.floor(viewport.width)}px`;
      canvas.style.height = `${Math.floor(viewport.height)}px`;
      context.setTransform(outputScale, 0, 0, outputScale, 0, 0);
      await page.render({ canvasContext: context, viewport }).promise;
      if (seq !== renderSeq) return;
      rendered = true;
    } catch (err) {
      error = err?.message || 'PDF page render failed';
    } finally {
      if (seq === renderSeq) rendering = false;
    }
  }

  async function loadPdf() {
    if (!sourceKey || loadedSourceKey === sourceKey) return;
    loadedSourceKey = sourceKey;
    loading = true;
    error = '';
    rendered = false;
    pdfDoc = null;
    pageCount = 0;
    pdfTextByPage = [];
    pdfSearchMatches = [];
    try {
      const res = await fetch(sourceKey, sourceFetchOptions(sourceKey));
      if (!res.ok) throw new Error(`PDF source failed (${res.status})`);
      const data = await res.arrayBuffer();
      const pdfjsLib = await loadPdfjs();
      const loadingTask = pdfjsLib.getDocument({ data });
      const doc = await loadingTask.promise;
      pdfDoc = doc;
      pageCount = doc.numPages || 0;
      pdfPage = clampNumber(pdfPage, 1, pageCount || 1);
      await tick();
      if (resizeObserver && stageEl) resizeObserver.observe(stageEl);
      await renderCurrentPage();
      void extractPdfText(doc).catch(() => {
        // Search is additive; rendering remains useful if text extraction fails.
      });
    } catch (err) {
      const message = err?.message || 'PDF load failed';
      error = `${message}. The PDF app needs a browser-fetchable PDF source; CORS-blocked remote files should be imported into Files first.`;
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
    await tick();
    await loadPdf();
  }

  onMount(() => {
    void loadContentItem();
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        if (pdfDoc && ['page-width', 'page-fit'].includes(pdfZoom)) void renderCurrentPage();
      });
      if (stageEl) resizeObserver.observe(stageEl);
    }
    return () => resizeObserver?.disconnect();
  });
</script>

<section class="media-app pdf-app" data-media-app data-media-kind="pdf" data-pdf-app>
  <header class="media-header">
    <div>
      <p>PDF</p>
      <h2 data-media-title>{source.title}</h2>
    </div>
  </header>

  {#if loading && !pdfDoc}
    <p class="media-status">Loading PDF...</p>
  {:else if error}
    <div class="reader-blocker" role="alert" data-pdf-blocker>
      <strong>PDF reader could not open this source.</strong>
      <span>{error}</span>
    </div>
  {:else if !source.displayUrl}
    <p class="media-status">No readable PDF source is attached to this window.</p>
  {:else}
    <div class="media-toolbar" data-media-toolbar data-pdf-toolbar>
      <button type="button" on:click={() => setPdfPage(pdfPage - 1)} disabled={pdfPage <= 1} data-pdf-prev>Prev</button>
      <label>
        Page
        <input type="number" min="1" max={pageCount || undefined} value={pdfPage} on:change={(event) => setPdfPage(event.currentTarget.value)} data-pdf-page />
      </label>
      <span data-pdf-page-count>{pageLabel}</span>
      <button type="button" on:click={() => setPdfPage(pdfPage + 1)} disabled={pageCount > 0 && pdfPage >= pageCount} data-pdf-next>Next</button>
      <label>
        Zoom
        <select bind:value={pdfZoom} on:change={renderCurrentPage} data-pdf-zoom>
          <option value="page-width">Fit width</option>
          <option value="page-fit">Fit page</option>
          <option value="100">100%</option>
          <option value="150">150%</option>
          <option value="200">200%</option>
        </select>
      </label>
      <label class="reader-search">
        Search
        <input type="search" bind:value={pdfSearch} on:input={updatePdfSearch} placeholder="Find text" data-pdf-search />
      </label>
      {#if pdfSearch.trim()}
        <span data-pdf-search-count>{pdfSearchMatches.length} matches</span>
        <button type="button" on:click={() => nextSearchMatch(-1)} disabled={!pdfSearchMatches.length} data-pdf-search-prev>Match -</button>
        <button type="button" on:click={() => nextSearchMatch(1)} disabled={!pdfSearchMatches.length} data-pdf-search-next>Match +</button>
      {/if}
    </div>

    <div class="media-stage pdf-stage" data-media-stage data-pdf-stage bind:this={stageEl}>
      <div class="pdf-page-shell" class:rendering data-pdf-reader data-pdf-rendered={rendered ? 'true' : 'false'}>
        <canvas bind:this={canvasEl} data-pdf-canvas aria-label={`Page ${pdfPage} of ${pageCount || '?'}`}></canvas>
      </div>
      {#if rendering}
        <span class="reader-badge" data-pdf-rendering>Rendering...</span>
      {/if}
    </div>

    {#if pdfSearchMatches.length}
      <div class="reader-results" data-pdf-search-results>
        {#each pdfSearchMatches.slice(0, 6) as match}
          <button type="button" on:click={() => setPdfPage(match.page)} data-pdf-search-result>
            <strong>Page {match.page}</strong>
            <span>{match.snippet}</span>
          </button>
        {/each}
      </div>
    {/if}

    <details class="media-details">
      <summary>Source and details</summary>
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
  .pdf-stage {
    position: relative;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    min-height: 0;
    padding: 16px;
    overflow: auto;
  }

  .pdf-page-shell {
    min-width: min-content;
    border-radius: 8px;
    background: #f8fafc;
    box-shadow: 0 18px 48px rgba(0, 0, 0, 0.44);
  }

  .pdf-page-shell.rendering {
    opacity: 0.72;
  }

  .pdf-page-shell canvas {
    display: block;
    width: auto;
    height: auto;
    min-height: auto;
  }

  .reader-search input {
    width: min(180px, 35vw);
    border: 1px solid rgba(99, 153, 255, 0.34);
    border-radius: 8px;
    background: rgba(5, 10, 22, 0.72);
    color: #f8fbff;
    padding: 7px 8px;
  }

  .reader-badge {
    position: sticky;
    top: 10px;
    align-self: flex-start;
    margin-left: 10px;
    border-radius: 999px;
    padding: 6px 10px;
    background: rgba(10, 18, 35, 0.86);
    color: #dbeafe;
    font-size: 0.78rem;
    font-weight: 800;
  }

  .reader-results {
    display: grid;
    flex: 0 0 auto;
    gap: 6px;
    max-height: 132px;
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

  @media (max-width: 720px) {
    .pdf-stage {
      padding: 10px;
    }

    .reader-search {
      flex: 1 1 100%;
    }

    .reader-search input {
      width: 100%;
    }
  }
</style>
