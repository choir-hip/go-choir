<script>
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import {
    appTitle,
    clampNumber,
    loadRecentMedia,
    loadContextContentItem,
    mediaSourceIdentity,
    recentMediaAppContext,
    rememberRecentMedia,
    resolveMediaSource,
  } from './media-utils.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let appContext = {};
  export let windowId = '';

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
  let activeRenderTask = null;
  let activeRenderPromise = null;
  let lastPdfScale = 1;
  let pinchStartDistance = 0;
  let pinchStartZoomPercent = 100;
  let pinchZoomFrame = 0;
  let loadedSourceKey = '';
  let resizeObserver = null;
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: pageLabel = pageCount ? `${pdfPage} / ${pageCount}` : String(pdfPage);
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

  function touchDistance(touches) {
    const first = touches[0];
    const second = touches[1];
    if (!first || !second) return 0;
    return Math.hypot(first.clientX - second.clientX, first.clientY - second.clientY);
  }

  function currentZoomPercent() {
    return clampNumber(Math.round(lastPdfScale * 100), 40, 400);
  }

  function renderZoomPercent(percent) {
    pdfZoom = String(clampNumber(Math.round(percent), 40, 400));
    if (pinchZoomFrame) cancelAnimationFrame(pinchZoomFrame);
    pinchZoomFrame = requestAnimationFrame(() => {
      pinchZoomFrame = 0;
      void renderCurrentPage();
    });
  }

  function handlePdfTouchStart(event) {
    if (event.touches.length !== 2) return;
    pinchStartDistance = touchDistance(event.touches);
    pinchStartZoomPercent = currentZoomPercent();
  }

  function handlePdfTouchMove(event) {
    if (event.touches.length !== 2 || !pinchStartDistance) return;
    event.preventDefault();
    const nextDistance = touchDistance(event.touches);
    if (!nextDistance) return;
    renderZoomPercent(pinchStartZoomPercent * (nextDistance / pinchStartDistance));
  }

  function handlePdfTouchEnd(event) {
    if (event.touches.length < 2) pinchStartDistance = 0;
  }

  async function renderCurrentPage() {
    const seq = ++renderSeq;
    if (!pdfDoc || !canvasEl) return;
    if (activeRenderTask && activeRenderPromise) {
      activeRenderTask.cancel();
      try {
        await activeRenderPromise;
      } catch (err) {
        if (err?.name !== 'RenderingCancelledException') throw err;
      }
    }
    if (seq !== renderSeq) return;
    rendering = true;
    rendered = false;
    let renderTask = null;
    try {
      await tick();
      const page = await pdfDoc.getPage(pdfPage);
      if (seq !== renderSeq) return;
      const scale = zoomScale(page, pdfZoom);
      lastPdfScale = scale;
      const viewport = page.getViewport({ scale });
      const outputScale = Math.max(1, window.devicePixelRatio || 1);
      const canvas = canvasEl;
      const context = canvas.getContext('2d');
      canvas.width = Math.floor(viewport.width * outputScale);
      canvas.height = Math.floor(viewport.height * outputScale);
      canvas.style.width = `${Math.floor(viewport.width)}px`;
      canvas.style.height = `${Math.floor(viewport.height)}px`;
      context.setTransform(outputScale, 0, 0, outputScale, 0, 0);
      renderTask = page.render({ canvasContext: context, viewport });
      activeRenderTask = renderTask;
      activeRenderPromise = renderTask.promise;
      await renderTask.promise;
      if (seq !== renderSeq) return;
      rendered = true;
    } catch (err) {
      if (err?.name === 'RenderingCancelledException') return;
      error = err?.message || 'PDF page render failed';
    } finally {
      if (activeRenderTask === renderTask) {
        activeRenderTask = null;
        activeRenderPromise = null;
      }
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
    await tick();
    await loadPdf();
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
    pdfDoc = null;
    pdfPage = 1;
    pageCount = 0;
    pdfTextByPage = [];
    pdfSearchMatches = [];
    loadedSourceKey = '';
    rendered = false;
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    await loadContentItem();
  }

  onMount(() => {
    void refreshRecentFiles();
    void loadContentItem();
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        if (pdfDoc && ['page-width', 'page-fit'].includes(pdfZoom)) void renderCurrentPage();
      });
      if (stageEl) resizeObserver.observe(stageEl);
    }
    const removeLiveListener = addLiveEventListener((message) => {
      if (liveEventKind(message) === 'media.recent.updated' && liveEventPayload(message).kind === kind) {
        void refreshRecentFiles();
      }
    });
    return () => {
      removeLiveListener();
      resizeObserver?.disconnect();
      if (pinchZoomFrame) cancelAnimationFrame(pinchZoomFrame);
    };
  });
</script>

<section class="pdf-app" data-media-app data-media-kind="pdf" data-pdf-app data-pdf-zoom-mode={pdfZoom}>
  {#if loading && !pdfDoc}
    <p class="pdf-status">Loading PDF...</p>
  {:else if error}
    <div class="pdf-blocker" role="alert" data-pdf-blocker>
      <strong>PDF reader could not open this source.</strong>
      <span>{error}</span>
    </div>
  {:else if !source.displayUrl}
    <div class="pdf-empty" data-media-empty data-media-recent-empty>
      <p class="pdf-status">No readable PDF source is attached to this window.</p>
      {#if recentFiles.length}
        <div class="pdf-recent" data-media-recent-list>
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
  {:else}
    <div
      class="pdf-stage"
      data-media-stage
      data-pdf-stage
      bind:this={stageEl}
      on:touchstart={handlePdfTouchStart}
      on:touchmove|nonpassive={handlePdfTouchMove}
      on:touchend={handlePdfTouchEnd}
      on:touchcancel={handlePdfTouchEnd}
    >
      <div class="pdf-page-shell" class:rendering data-pdf-reader data-pdf-rendered={rendered ? 'true' : 'false'}>
        <canvas bind:this={canvasEl} data-pdf-canvas aria-label={`Page ${pdfPage} of ${pageCount || '?'}`}></canvas>
      </div>
      {#if rendering}
        <span class="reader-badge" data-pdf-rendering>Rendering...</span>
      {/if}
    </div>

    <div class="pdf-page-nav" data-pdf-page-nav>
      <button type="button" on:click={() => setPdfPage(pdfPage - 1)} disabled={pdfPage <= 1} aria-label="Previous PDF page" title="Previous page" data-pdf-prev-float>&lt;</button>
      <button type="button" on:click={() => setPdfPage(pdfPage + 1)} disabled={pageCount > 0 && pdfPage >= pageCount} aria-label="Next PDF page" title="Next page" data-pdf-next-float>&gt;</button>
    </div>

    <details class="pdf-controls" data-media-toolbar data-pdf-toolbar data-media-controls>
      <summary aria-label="PDF controls" title="PDF controls"><span aria-hidden="true">...</span></summary>
      <div class="pdf-toolbar">
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
    </details>

    <details class="pdf-meta">
      <summary aria-label="PDF info" title="PDF info"><span aria-hidden="true">i</span></summary>
      <h2 data-media-title>{source.title}</h2>
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
  .pdf-app {
    position: relative;
    display: block;
    height: 100%;
    min-height: 0;
    color: #f5f7ff;
    background: #050814;
    overflow: hidden;
  }

  .pdf-controls {
    position: absolute;
    z-index: 4;
    top: 10px;
    left: 0;
    width: max-content;
    max-width: min(720px, calc(100% - 20px));
    max-height: calc(100% - 20px);
    color: #cbd5e1;
    overflow: auto;
    transform: translateX(-34%);
  }

  .pdf-controls summary {
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

  .pdf-controls summary::-webkit-details-marker {
    display: none;
  }

  .pdf-controls summary span {
    font-size: 0.92rem;
    line-height: 1;
  }

  .pdf-controls[open] {
    left: 10px;
    border: 1px solid rgba(99, 153, 255, 0.28);
    border-radius: 12px;
    background: rgba(8, 14, 28, 0.86);
    backdrop-filter: blur(12px);
    transform: none;
  }

  .pdf-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 0 8px 8px;
  }

  .pdf-toolbar button {
    min-height: 34px;
    border: 1px solid rgba(126, 180, 255, 0.32);
    border-radius: 9px;
    background: rgba(37, 64, 108, 0.72);
    color: #eef5ff;
    cursor: pointer;
    font-weight: 760;
    padding: 7px 10px;
  }

  .pdf-toolbar button:hover:not(:disabled) {
    background: rgba(56, 96, 160, 0.82);
  }

  .pdf-toolbar button:disabled {
    cursor: not-allowed;
    opacity: 0.52;
  }

  .pdf-toolbar span,
  .pdf-toolbar label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: #a8adbd;
    font-size: 0.84rem;
  }

  .pdf-toolbar input[type='number'],
  .pdf-toolbar select,
  .pdf-toolbar input[type='search'] {
    border: 1px solid rgba(99, 153, 255, 0.34);
    border-radius: 8px;
    background: rgba(5, 10, 22, 0.72);
    color: #f8fbff;
    padding: 7px 8px;
  }

  .pdf-toolbar input[type='number'] {
    width: 4.8rem;
  }

  .reader-search input {
    width: min(180px, 35vw);
  }

  .pdf-stage {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    min-height: 0;
    padding: 10px;
    background: #030712;
    overflow: auto;
    touch-action: pan-x pan-y;
    overscroll-behavior: contain;
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

  .pdf-page-nav {
    position: absolute;
    inset: 0;
    z-index: 3;
    pointer-events: none;
  }

  .pdf-page-nav button {
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

  .pdf-page-nav button:first-child {
    left: 0;
    transform: translate(-42%, -50%);
  }

  .pdf-page-nav button:last-child {
    right: 0;
    transform: translate(42%, -50%);
  }

  .pdf-page-nav button:hover:not(:disabled),
  .pdf-page-nav button:focus-visible {
    opacity: 0.92;
  }

  .pdf-page-nav button:disabled {
    cursor: default;
    opacity: 0.14;
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

  .pdf-meta {
    position: absolute;
    z-index: 4;
    right: 10px;
    bottom: 10px;
    width: max-content;
    max-width: min(520px, calc(100% - 20px));
    color: #a8adbd;
  }

  .pdf-meta:not([open]) {
    right: 0;
    transform: translateX(34%);
  }

  .pdf-meta summary {
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

  .pdf-meta summary::-webkit-details-marker {
    display: none;
  }

  .pdf-meta summary span {
    font-size: 0.86rem;
    line-height: 1;
  }

  .pdf-meta[open] {
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

  .pdf-meta h2 {
    margin: 10px 0;
    color: #f8fbff;
    font-size: 1rem;
    overflow-wrap: anywhere;
  }

  .pdf-meta dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px 10px;
    margin: 0 0 4px;
  }

  .pdf-meta dt {
    color: #dbeafe;
    font-weight: 760;
  }

  .pdf-meta dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .pdf-meta a {
    color: #bfdbfe;
  }

  .pdf-status,
  .pdf-blocker {
    margin: 0;
    border-radius: 14px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
    color: #a8adbd;
  }

  .pdf-blocker {
    display: grid;
    gap: 6px;
    color: #dce6ff;
  }

  .pdf-empty {
    display: grid;
    width: min(100% - 24px, 520px);
    height: 100%;
    place-content: center;
    justify-self: center;
    gap: 12px;
  }

  .pdf-recent {
    display: grid;
    gap: 8px;
    border: 1px solid rgba(126, 180, 255, 0.2);
    border-radius: 16px;
    background: rgba(8, 14, 28, 0.74);
    padding: 12px;
  }

  .pdf-recent > span {
    color: #93c5fd;
    font-size: 0.74rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .pdf-recent button {
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

  .pdf-recent button:hover,
  .pdf-recent button:focus-visible {
    border-color: rgba(96, 165, 250, 0.45);
    background: rgba(96, 165, 250, 0.12);
  }

  .pdf-recent strong,
  .pdf-recent small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .pdf-recent small {
    color: #94a3b8;
    font-size: 0.74rem;
  }

  @media (max-width: 720px) {
    .pdf-stage {
      padding: 6px;
    }

    .pdf-controls {
      top: 6px;
      max-width: calc(100% - 16px);
    }

    .pdf-controls[open] {
      left: 6px;
    }

    .pdf-toolbar {
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
