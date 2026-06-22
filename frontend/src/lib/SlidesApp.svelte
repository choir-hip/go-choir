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

  const kind = 'slides';
  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let slides = [];
  let currentSlide = 0;
  let isFullscreen = false;
  let stageEl = null;
  let containerEl = null;
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';
  let loadedSourceKey = '';
  let pdfjsLibPromise = null;

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: slideLabel = slides.length ? `${currentSlide + 1} / ${slides.length}` : '—';
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

  function fileExtension(path) {
    const ext = String(path || '').toLowerCase().split('.').pop();
    return ext === String(path || '').toLowerCase() ? '' : ext;
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

  // ---- PDF parsing: each page becomes a slide (canvas image) ----
  async function parsePdf(arrayBuffer) {
    const pdfjsLib = await loadPdfjs();
    const loadingTask = pdfjsLib.getDocument({ data: arrayBuffer });
    const doc = await loadingTask.promise;
    const result = [];
    const scale = 2;
    for (let i = 1; i <= doc.numPages; i++) {
      const page = await doc.getPage(i);
      const viewport = page.getViewport({ scale });
      const canvas = document.createElement('canvas');
      canvas.width = Math.floor(viewport.width);
      canvas.height = Math.floor(viewport.height);
      const ctx = canvas.getContext('2d');
      await page.render({ canvasContext: ctx, viewport }).promise;
      result.push({
        type: 'image',
        src: canvas.toDataURL('image/png'),
        width: viewport.width,
        height: viewport.height,
      });
    }
    return result;
  }

  // ---- PPTX parsing: extract slides from OOXML ----
  async function parsePptx(arrayBuffer) {
    const JSZip = (await import('jszip')).default;
    const zip = await JSZip.loadAsync(arrayBuffer);
    const slideFiles = Object.keys(zip.files)
      .filter((name) => /^ppt\/slides\/slide\d+\.xml$/.test(name))
      .sort((a, b) => {
        const na = parseInt(a.match(/slide(\d+)/)[1], 10);
        const nb = parseInt(b.match(/slide(\d+)/)[1], 10);
        return na - nb;
      });

    // Load rels for each slide to resolve image references
    const result = [];
    for (const slidePath of slideFiles) {
      const slideXml = await zip.file(slidePath).async('string');
      const slideNum = slidePath.match(/slide(\d+)/)[1];
      const relsPath = `ppt/slides/_rels/slide${slideNum}.xml.rels`;
      const relsFile = zip.file(relsPath);
      let relsMap = {};
      if (relsFile) {
        const relsXml = await relsFile.async('string');
        relsMap = parseRels(relsXml);
      }
      result.push(parseSlideXml(slideXml, relsMap, zip));
    }
    return result;
  }

  function parseRels(relsXml) {
    const map = {};
    const parser = new DOMParser();
    const doc = parser.parseFromString(relsXml, 'application/xml');
    const relationships = doc.getElementsByTagName('Relationship');
    for (let i = 0; i < relationships.length; i++) {
      const rel = relationships[i];
      const id = rel.getAttribute('Id');
      const target = rel.getAttribute('Target');
      if (id && target) map[id] = target;
    }
    return map;
  }

  function parseSlideXml(slideXml, relsMap, zip) {
    const parser = new DOMParser();
    const doc = parser.parseFromString(slideXml, 'application/xml');
    const texts = [];
    const images = [];

    // Extract text runs
    const textElements = doc.getElementsByTagName('a:t');
    for (let i = 0; i < textElements.length; i++) {
      const text = textElements[i].textContent || '';
      if (text.trim()) texts.push(text.trim());
    }

    // Extract images
    const blipElements = doc.getElementsByTagName('a:blip');
    for (let i = 0; i < blipElements.length; i++) {
      const blip = blipElements[i];
      const embed = blip.getAttribute('r:embed');
      if (embed && relsMap[embed]) {
        const imagePath = relsMap[embed].startsWith('/') ?
          relsMap[embed].slice(1) :
          'ppt/slides/' + relsMap[embed];
        const imgFile = zip.file(imagePath);
        if (imgFile) {
          const blob = imgFile.async('blob');
          images.push(blob);
        }
      }
    }

    return {
      type: 'pptx',
      texts,
      images: images,
      title: texts[0] || `Slide`,
    };
  }

  // ---- HTML parsing: split by <section>, <hr>, or heading boundaries ----
  async function parseHtml(text) {
    const parser = new DOMParser();
    const doc = parser.parseFromString(text, 'text/html');
    const body = doc.body;

    // Try <section> elements first
    let sections = body.querySelectorAll('section');
    if (sections.length > 0) {
      return Array.from(sections).map((section) => ({
        type: 'html',
        html: section.innerHTML,
        title: section.querySelector('h1, h2, h3')?.textContent || `Slide`,
      }));
    }

    // Try splitting by <hr> separators
    const hrSplit = [];
    let current = document.createElement('div');
    for (const node of Array.from(body.childNodes)) {
      if (node.nodeName === 'HR') {
        if (current.innerHTML.trim()) {
          hrSplit.push({
            type: 'html',
            html: current.innerHTML,
            title: current.querySelector('h1, h2, h3')?.textContent || `Slide`,
          });
        }
        current = document.createElement('div');
      } else {
        current.appendChild(node.cloneNode(true));
      }
    }
    if (current.innerHTML.trim()) {
      hrSplit.push({
        type: 'html',
        html: current.innerHTML,
        title: current.querySelector('h1, h2, h3')?.textContent || `Slide`,
      });
    }
    if (hrSplit.length > 0) return hrSplit;

    // Try splitting by h1/h2 headings
    const headings = body.querySelectorAll('h1, h2');
    if (headings.length > 1) {
      const headingSplit = [];
      for (let i = 0; i < headings.length; i++) {
        const start = headings[i];
        const end = i + 1 < headings.length ? headings[i + 1] : null;
        const fragment = document.createElement('div');
        let sibling = start;
        while (sibling && sibling !== end) {
          fragment.appendChild(sibling.cloneNode(true));
          sibling = sibling.nextElementSibling;
        }
        headingSplit.push({
          type: 'html',
          html: fragment.innerHTML,
          title: start.textContent || `Slide`,
        });
      }
      return headingSplit;
    }

    // Fallback: whole document as one slide
    return [{
      type: 'html',
      html: body.innerHTML,
      title: body.querySelector('h1, h2, h3, title')?.textContent || 'Slide',
    }];
  }

  async function loadSlides() {
    if (!sourceKey || loadedSourceKey === sourceKey) return;
    loadedSourceKey = sourceKey;
    loading = true;
    error = '';
    slides = [];
    currentSlide = 0;

    try {
      const res = await fetch(sourceKey, sourceFetchOptions(sourceKey));
      if (!res.ok) throw new Error(`Source fetch failed (${res.status})`);

      const ext = fileExtension(source.filePath || source.title || sourceKey);
      let parsed;

      if (ext === 'pdf') {
        const arrayBuffer = await res.arrayBuffer();
        parsed = await parsePdf(arrayBuffer);
      } else if (ext === 'pptx') {
        const arrayBuffer = await res.arrayBuffer();
        parsed = await parsePptx(arrayBuffer);
      } else if (ext === 'html' || ext === 'htm') {
        const text = await res.text();
        parsed = await parseHtml(text);
      } else {
        throw new Error(`Unsupported file type: .${ext}`);
      }

      // Resolve any blob promises in pptx slides
      for (const slide of parsed) {
        if (slide.images && slide.images.length) {
          slide.imageUrls = await Promise.all(
            slide.images.map((blobPromise) =>
              blobPromise.then((blob) => URL.createObjectURL(blob))
            )
          );
        }
      }

      slides = parsed;
      currentSlide = 0;
    } catch (err) {
      error = err?.message || 'Failed to load slides';
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
    await loadSlides();
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
    slides = [];
    currentSlide = 0;
    loadedSourceKey = '';
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    await loadContentItem();
  }

  function goToSlide(index) {
    currentSlide = clampNumber(index, 0, slides.length - 1);
  }

  function nextSlide() {
    if (currentSlide < slides.length - 1) currentSlide++;
  }

  function prevSlide() {
    if (currentSlide > 0) currentSlide--;
  }

  function toggleFullscreen() {
    if (!containerEl) return;
    if (!document.fullscreenElement) {
      containerEl.requestFullscreen?.().catch(() => {});
    } else {
      document.exitFullscreen?.();
    }
  }

  function handleFullscreenChange() {
    isFullscreen = !!document.fullscreenElement;
  }

  function handleKeydown(event) {
    if (event.key === 'ArrowRight' || event.key === ' ' || event.key === 'PageDown') {
      event.preventDefault();
      nextSlide();
    } else if (event.key === 'ArrowLeft' || event.key === 'PageUp') {
      event.preventDefault();
      prevSlide();
    } else if (event.key === 'Home') {
      event.preventDefault();
      goToSlide(0);
    } else if (event.key === 'End') {
      event.preventDefault();
      goToSlide(slides.length - 1);
    } else if (event.key === 'f' || event.key === 'F') {
      event.preventDefault();
      toggleFullscreen();
    }
  }

  function handleStageClick(event) {
    const rect = stageEl?.getBoundingClientRect();
    if (!rect) return;
    const x = event.clientX - rect.left;
    if (x > rect.width / 2) {
      nextSlide();
    } else {
      prevSlide();
    }
  }

  onMount(() => {
    void refreshRecentFiles();
    void loadContentItem();
    document.addEventListener('fullscreenchange', handleFullscreenChange);
    const removeLiveListener = addLiveEventListener((message) => {
      if (liveEventKind(message) === 'media.recent.updated' && liveEventPayload(message).kind === kind) {
        void refreshRecentFiles();
      }
    });
    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
      removeLiveListener();
    };
  });
</script>

<div
  class="slides-container"
  bind:this={containerEl}
  tabindex="0"
  on:keydown={handleKeydown}
  data-slides-app
>
  {#if loading}
    <div class="slides-state" role="status">
      <div class="slides-spinner" />
      <p>Loading slides…</p>
    </div>
  {:else if error}
    <div class="slides-state" role="alert">
      <p>Could not load slides</p>
      <small>{error}</small>
    </div>
  {:else if slides.length === 0}
    <div class="slides-empty">
      <div class="slides-empty-icon">🖥️</div>
      <p>Open a PPTX, PDF, or HTML file from Files to play it as slides.</p>
      {#if recentFiles.length > 0}
        <div class="slides-recent">
          <h3>Recent</h3>
          {#each recentFiles as entry}
            <button class="slides-recent-item" on:click={() => openRecentFile(entry)}>
              <span class="slides-recent-icon">🖥️</span>
              <span class="slides-recent-name">{entry.title || entry.fileName || 'Untitled'}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  {:else}
    <!-- Slide stage -->
    <div class="slides-stage" bind:this={stageEl} on:click={handleStageClick}>
      {#if slides[currentSlide]?.type === 'image'}
        <img
          class="slide-image"
          src={slides[currentSlide].src}
          alt="Slide {currentSlide + 1}"
          draggable="false"
        />
      {:else if slides[currentSlide]?.type === 'pptx'}
        <div class="slide-pptx">
          {#each slides[currentSlide].imageUrls || [] as imgUrl}
            <img src={imgUrl} alt="Slide image" class="slide-pptx-image" />
          {/each}
          {#each slides[currentSlide].texts as text}
            <p class="slide-pptx-text">{text}</p>
          {/each}
        </div>
      {:else if slides[currentSlide]?.type === 'html'}
        <div class="slide-html">
          {@html slides[currentSlide].html}
        </div>
      {/if}
    </div>

    <!-- Controls bar -->
    <div class="slides-controls">
      <button class="slides-btn" on:click={prevSlide} disabled={currentSlide === 0} title="Previous (←)">
        ◀
      </button>
      <span class="slides-counter">{slideLabel}</span>
      <button class="slides-btn" on:click={nextSlide} disabled={currentSlide === slides.length - 1} title="Next (→ / Space)">
        ▶
      </button>
      <button class="slides-btn slides-btn-fs" on:click={toggleFullscreen} title="Fullscreen (F)">
        {isFullscreen ? '🗗' : '⛶'}
      </button>
    </div>

    <!-- Thumbnail strip -->
    {#if slides.length > 1 && !isFullscreen}
      <div class="slides-thumbnails">
        {#each slides as slide, i}
          <button
            class="slides-thumb {i === currentSlide ? 'active' : ''}"
            on:click={() => goToSlide(i)}
            title="Slide {i + 1}"
          >
            {#if slide.type === 'image'}
              <img src={slide.src} alt="" />
            {:else if slide.type === 'pptx'}
              <div class="slides-thumb-pptx">
                {#if (slide.imageUrls || [])[0]}
                  <img src={slide.imageUrls[0]} alt="" />
                {/if}
                <span>{slide.texts[0] || '…'}</span>
              </div>
            {:else}
              <div class="slides-thumb-html">{slide.title || '…'}</div>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .slides-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #1a1a2e;
    color: #e0e0e0;
    outline: none;
    user-select: none;
  }

  .slides-container:fullscreen {
    background: #000;
  }

  .slides-state {
    display: grid;
    place-content: center;
    gap: 0.5rem;
    height: 100%;
    text-align: center;
  }

  .slides-state p {
    margin: 0;
    font-weight: 600;
    font-size: 1.1rem;
  }

  .slides-state small {
    color: #ff6b6b;
  }

  .slides-spinner {
    width: 32px;
    height: 32px;
    border: 3px solid rgba(255, 255, 255, 0.15);
    border-top-color: #7c9eff;
    border-radius: 50%;
    animation: slides-spin 0.8s linear infinite;
    margin: 0 auto;
  }

  @keyframes slides-spin {
    to { transform: rotate(360deg); }
  }

  .slides-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    height: 100%;
    padding: 2rem;
    text-align: center;
  }

  .slides-empty-icon {
    font-size: 3rem;
    opacity: 0.6;
  }

  .slides-empty > p {
    color: #888;
    max-width: 320px;
    margin: 0;
  }

  .slides-recent {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    max-width: 360px;
    width: 100%;
    text-align: left;
  }

  .slides-recent h3 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #666;
    margin: 0;
  }

  .slides-recent-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 6px;
    color: #ccc;
    cursor: pointer;
    text-align: left;
    transition: background 0.15s;
  }

  .slides-recent-item:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  .slides-recent-icon {
    font-size: 1rem;
  }

  .slides-recent-name {
    font-size: 0.85rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .slides-stage {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    cursor: pointer;
    padding: 1rem;
  }

  .slides-container:fullscreen .slides-stage {
    padding: 0;
  }

  .slide-image {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    border-radius: 4px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
  }

  .slide-pptx {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.8rem;
    max-width: 100%;
    max-height: 100%;
    padding: 2rem;
    background: #fff;
    color: #222;
    border-radius: 8px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
    overflow: auto;
  }

  .slide-pptx-image {
    max-width: 100%;
    max-height: 60vh;
    object-fit: contain;
  }

  .slide-pptx-text {
    margin: 0;
    font-size: 1.1rem;
    line-height: 1.5;
    text-align: center;
  }

  .slide-html {
    max-width: 100%;
    max-height: 100%;
    overflow: auto;
    padding: 2rem;
    background: #fff;
    color: #222;
    border-radius: 8px;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
  }

  .slide-html :global(h1) {
    font-size: 2rem;
    margin: 0 0 1rem;
  }

  .slide-html :global(h2) {
    font-size: 1.5rem;
    margin: 0 0 0.75rem;
  }

  .slide-html :global(p) {
    font-size: 1.1rem;
    line-height: 1.6;
    margin: 0 0 0.5rem;
  }

  .slide-html :global(img) {
    max-width: 100%;
    height: auto;
  }

  .slides-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    background: rgba(0, 0, 0, 0.3);
    border-top: 1px solid rgba(255, 255, 255, 0.06);
  }

  .slides-btn {
    display: grid;
    place-items: center;
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.08);
    color: #e0e0e0;
    font-size: 1rem;
    cursor: pointer;
    transition: background 0.15s;
  }

  .slides-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.15);
  }

  .slides-btn:disabled {
    opacity: 0.3;
    cursor: default;
  }

  .slides-btn-fs {
    margin-left: auto;
  }

  .slides-counter {
    font-size: 0.85rem;
    color: #999;
    min-width: 60px;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }

  .slides-thumbnails {
    display: flex;
    gap: 0.4rem;
    padding: 0.5rem;
    overflow-x: auto;
    background: rgba(0, 0, 0, 0.2);
    border-top: 1px solid rgba(255, 255, 255, 0.04);
  }

  .slides-thumb {
    flex: 0 0 auto;
    width: 80px;
    height: 50px;
    border: 2px solid transparent;
    border-radius: 4px;
    overflow: hidden;
    cursor: pointer;
    background: #fff;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: border-color 0.15s;
  }

  .slides-thumb.active {
    border-color: #7c9eff;
  }

  .slides-thumb img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }

  .slides-thumb-pptx {
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 100%;
    height: 100%;
    overflow: hidden;
  }

  .slides-thumb-pptx img {
    max-height: 32px;
  }

  .slides-thumb-pptx span {
    font-size: 0.55rem;
    color: #333;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    width: 100%;
    text-align: center;
  }

  .slides-thumb-html {
    font-size: 0.55rem;
    color: #333;
    padding: 0.2rem;
    text-align: center;
    overflow: hidden;
  }
</style>
