<script>
  // Reader: one window for PDFs and EPUBs.
  //
  // A first-party package (src/packages/reader), published as the default
  // opener for .pdf and .epub under the ids 'pdf' and 'epub' that the Files
  // app and saved windows already use. It reads sources logged-out through the
  // shell's public preview, and opens its own sample when a signed-out window
  // was not given a file, so the surface is usable before sign-in and in a
  // capture with no backend.
  //
  // Parsing: pdfjs renders PDF pages; JSZip plus DOMParser unpack EPUB
  // chapters. Both are dynamic imports, so a window that only reads EPUB text
  // never loads pdfjs. Position goes through the shell's media progress route
  // when the user is signed in; text settings are local to the browser.
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import {
    clampNumber,
    loadRecentMedia,
    loadContextContentItem,
    mediaSourceIdentity,
    recentMediaAppContext,
    rememberRecentMedia,
    resolveMediaSource,
    addLiveEventListener,
    liveEventKind,
    liveEventPayload,
  } from '../../lib/app-sdk';
  import {
    blocksFromHtml,
    chapterKey,
    cleanText,
    detectFormat,
    modelChapterCount,
    modelWordCount,
    readingMinutes,
    resolvePath,
    searchDocuments,
    sectionsFromText,
  } from './reader-text.js';
  import {
    sampleModel,
    FONT_STEPS,
    MEASURE_NAMES,
    MEASURE_STEPS,
    READER_THEMES,
    loadPrefs,
    normalizePrefs,
    savePrefs,
  } from './reader-prefs.js';
  import { modelFromPdfPages } from './pdf-text.js';
  import './reader.css';

  export let appContext = {};
  export let windowId = '';
  export let authenticated = false;

  const dispatch = createEventDispatcher();
  const ZOOM_MODES = [
    { value: 'page-width', label: 'Fit width' },
    { value: 'page-fit', label: 'Fit page' },
    { value: '100', label: '100%' },
    { value: '150', label: '150%' },
    { value: '200', label: '200%' },
  ];

  let item = appContext?.contentItem || null;
  let selectedContext = null;
  let recentFiles = [];
  let loading = false;
  let error = '';
  let authNeeded = false;
  let status = 'idle';

  let model = null;
  let loadedSourceKey = '';
  let format = 'pdf';

  let pdfjsLibPromise = null;
  let jszipPromise = null;
  let pdfDoc = null;
  let pdfPage = 1;
  let pageCount = 0;
  let zoomMode = 'page-width';
  let lastScale = 1;
  let rendering = false;
  let rendered = false;
  let canvasEl = null;
  let stageEl = null;
  let renderSeq = 0;
  let activeRenderTask = null;
  let activeRenderPromise = null;

  let sectionIndex = 0;
  let prefs = normalizePrefs(null);
  let tocOpen = false;
  let searchOpen = false;
  let searchTerm = '';
  let searchHits = [];
  let hitIndex = -1;
  let progress = 0;
  let toast = '';
  let toastTimer = 0;

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, appContext?.readerKind || appContext?.kind || 'pdf');
  $: sourceKey = source.displayUrl || '';
  $: sourceIdentity = mediaSourceIdentity(source);
  $: extractedText = item?.text_content || effectiveContext.textContent || '';
  $: if (sourceKey && sourceKey !== loadedSourceKey && !error && !loading) {
    void loadSource();
  }
  $: section = model?.sections?.[sectionIndex] || null;
  $: chapterCount = modelChapterCount(model);
  $: words = modelWordCount(model);
  $: minutes = readingMinutes(model);
  $: tone = 'reader-tone-' + prefs.theme;
  $: sheetStyle = '--reader-font-size: ' + prefs.fontSize + 'px; --reader-measure: ' + prefs.measure + ';';
  $: canPrev = format === 'pdf' ? pdfPage > 1 : sectionIndex > 0;
  $: canNext = format === 'pdf' ? pageCount > 0 && pdfPage < pageCount : sectionIndex < chapterCount - 1;
  $: pageLabel = pageCount ? pdfPage + ' / ' + pageCount : String(pdfPage);

  function flash(message) {
    toast = message;
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toast = '';
      toastTimer = 0;
    }, 2600);
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

  async function loadJSZip() {
    if (!jszipPromise) {
      jszipPromise = import('jszip').then((module) => module.default);
    }
    return jszipPromise;
  }

  function parseXML(text, label) {
    const doc = new DOMParser().parseFromString(text, 'application/xml');
    if (doc.querySelector('parsererror')) throw new Error(label + ' XML could not be parsed');
    return doc;
  }

  function byLocalName(root, name) {
    return Array.from(root.getElementsByTagName('*')).filter((element) => element.localName === name);
  }

  function firstByLocalName(root, name) {
    return byLocalName(root, name)[0] || null;
  }

  function titleFromDoc(doc, fallback) {
    const heading = doc.querySelector('h1,h2,h3,title');
    return cleanText(heading?.textContent) || fallback;
  }

  async function textFile(zip, path) {
    const file = zip.file(path);
    return file ? file.async('text') : '';
  }

  async function parseEpub(arrayBuffer) {
    const JSZip = await loadJSZip();
    const zip = await JSZip.loadAsync(arrayBuffer);
    const containerText = await textFile(zip, 'META-INF/container.xml');
    if (!containerText) throw new Error('EPUB container.xml is missing');
    const opfPath = firstByLocalName(parseXML(containerText, 'container'), 'rootfile')?.getAttribute('full-path');
    if (!opfPath) throw new Error('EPUB package path is missing');
    const opfText = await textFile(zip, opfPath);
    if (!opfText) throw new Error('EPUB package document is missing');
    const opf = parseXML(opfText, 'package');
    const title = cleanText(firstByLocalName(opf, 'title')?.textContent) || source.title;
    const author = cleanText(firstByLocalName(opf, 'creator')?.textContent);
    const manifest = new Map();
    byLocalName(opf, 'item').forEach((entry) => {
      const id = entry.getAttribute('id');
      const href = entry.getAttribute('href');
      if (id && href) {
        manifest.set(id, {
          href: resolvePath(opfPath, href),
          mediaType: entry.getAttribute('media-type') || '',
        });
      }
    });
    const spine = byLocalName(opf, 'itemref')
      .map((entry) => manifest.get(entry.getAttribute('idref')))
      .filter(Boolean)
      .filter((entry) => /xhtml|html/i.test(entry.mediaType) || /\\.x?html?$/i.test(entry.href));
    if (!spine.length) throw new Error('EPUB spine has no readable chapters');
    const sections = [];
    for (let index = 0; index < spine.length; index++) {
      const chapterText = await textFile(zip, spine[index].href);
      if (!chapterText) continue;
      const doc = new DOMParser().parseFromString(chapterText, 'text/html');
      const blocks = blocksFromHtml(doc);
      if (!blocks.length) continue;
      sections.push({
        id: chapterKey(spine[index].href, index),
        href: spine[index].href,
        title: titleFromDoc(doc, 'Chapter ' + (index + 1)),
        blocks,
      });
    }
    if (!sections.length) throw new Error('EPUB chapters did not contain readable text');
    return { format: 'epub', title, author, sample: false, status: 'ready', sections };
  }

  function openSample() {
    model = sampleModel;
    format = 'epub';
    sectionIndex = 0;
    pdfDoc = null;
    pageCount = 0;
    progress = 0;
    error = '';
    authNeeded = false;
    status = 'sample';
  }

  async function loadSource() {
    const key = sourceKey;
    loadedSourceKey = key;
    loading = true;
    error = '';
    authNeeded = false;
    pdfDoc = null;
    pageCount = 0;
    pdfPage = 1;
    rendered = false;
    model = null;
    format = detectFormat({
      mediaType: source.mediaType,
      fileName: effectiveContext.fileName,
      sourceUrl: source.sourceUrl,
      kind: appContext?.readerKind || appContext?.kind,
      extractedText,
    });
    try {
      const result = await loadContextContentItem(effectiveContext, item, format === 'pdf' ? 'PDF' : 'EPUB').catch(() => ({ error: 'Content lookup failed' }));
      if (result.item) item = result.item;
      if (result.error && !key) throw new Error(result.error);
      if (result.authRequired && !key) {
        authNeeded = true;
        status = 'auth';
        dispatch('authexpired', { kind: 'app_launch', appId: 'reader', appName: 'Reader', appContext: { windowTitle: source.title } });
        return;
      }
      if (format === 'epub') {
        let text = item?.text_content || extractedText;
        if (!text && key) {
          const res = await fetch(key, sourceFetchOptions(key));
          if (!res.ok) throw new Error('EPUB source failed (' + res.status + ')');
          model = await parseEpub(await res.arrayBuffer());
        } else {
          const sections = sectionsFromText(text, { title: source.title });
          if (!sections.length) throw new Error('The attached text is empty');
          model = { format: 'epub', title: source.title, author: '', sample: false, status: 'ready', sections };
        }
      } else {
        await loadPdfDocument(key);
      }
      status = 'ready';
      await tick();
      await restorePosition();
    } catch (err) {
      const detail = err?.message || 'Reader could not open this source';
      if (!authenticated) {
        // A signed-out window has no durable source: read the sample and say
        // why, so a visitor still has something to read instead of an error.
        openSample();
        authNeeded = true;
        flash(detail);
      } else {
        error = detail;
        status = 'error';
      }
    } finally {
      loading = false;
    }
  }

  async function loadPdfDocument(url) {
    const res = await fetch(url, sourceFetchOptions(url));
    if (!res.ok) throw new Error('PDF source failed (' + res.status + ')');
    const buffer = await res.arrayBuffer();
    const pdfjsLib = await loadPdfjs();
    const doc = await pdfjsLib.getDocument({ data: buffer }).promise;
    pdfDoc = doc;
    pageCount = doc.numPages || 0;
    pdfPage = clampNumber(pdfPage, 1, pageCount || 1);
    model = { format: 'pdf', title: source.title, sample: false, status: 'ready', sections: [] };
    await tick();
    await renderPage();
    void hydratePdfText(doc).catch(() => {});
  }

  // PDF text search reads the document once, in the background: search works
  // as soon as the pages it has read answer, and the rendered page never waits.
  let pdfSearchModel = null;
  async function hydratePdfText(doc) {
    if (pdfSearchModel && pdfSearchModel.doc === doc) return pdfSearchModel.model;
    const pages = [];
    for (let index = 1; index <= doc.numPages; index++) {
      const page = await doc.getPage(index);
      const content = await page.getTextContent();
      pages.push(content.items.map((entry) => entry.str || '').join(' ').replace(/\\s+/g, ' ').trim());
    }
    const modelForSearch = modelFromPdfPages(pages, { title: source.title });
    pdfSearchModel = { doc, model: modelForSearch };
    if (searchOpen && searchTerm.trim()) runSearch();
    return modelForSearch;
  }

  function zoomScale(page) {
    const base = page.getViewport({ scale: 1 });
    const rect = stageEl?.getBoundingClientRect();
    const width = Math.max(220, (rect?.width || 760) - 36);
    const height = Math.max(220, (rect?.height || 560) - 36);
    if (zoomMode === 'page-width') return clampNumber(width / base.width, 0.2, 5);
    if (zoomMode === 'page-fit') return clampNumber(Math.min(width / base.width, height / base.height), 0.2, 5);
    return clampNumber((Number(zoomMode) || 100) / 100, 0.2, 5);
  }

  async function renderPage() {
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
    let task = null;
    try {
      await tick();
      const page = await pdfDoc.getPage(pdfPage);
      if (seq !== renderSeq) return;
      const scale = zoomScale(page);
      lastScale = scale;
      const viewport = page.getViewport({ scale });
      const ratio = Math.max(1, window.devicePixelRatio || 1);
      const canvas = canvasEl;
      const context = canvas.getContext('2d');
      canvas.width = Math.floor(viewport.width * ratio);
      canvas.height = Math.floor(viewport.height * ratio);
      canvas.style.width = Math.floor(viewport.width) + 'px';
      canvas.style.height = Math.floor(viewport.height) + 'px';
      context.setTransform(ratio, 0, 0, ratio, 0, 0);
      task = page.render({ canvasContext: context, viewport });
      activeRenderTask = task;
      activeRenderPromise = task.promise;
      await task.promise;
      if (seq !== renderSeq) return;
      rendered = true;
      progress = pageCount > 1 ? Math.round(((pdfPage - 1) / (pageCount - 1)) * 100) : 100;
      rememberPosition();
    } catch (err) {
      if (err?.name === 'RenderingCancelledException') return;
      error = err?.message || 'PDF page render failed';
      status = 'error';
    } finally {
      if (activeRenderTask === task) {
        activeRenderTask = null;
        activeRenderPromise = null;
      }
      if (seq === renderSeq) rendering = false;
    }
  }

  function goToPage(next) {
    if (!pdfDoc) return;
    const target = clampNumber(Math.floor(Number(next) || 1), 1, pageCount || 1);
    if (target === pdfPage) return;
    pdfPage = target;
    void renderPage();
  }

  function goToSection(next) {
    if (!chapterCount) return;
    sectionIndex = clampNumber(Math.floor(Number(next) || 0), 0, chapterCount - 1);
    tick().then(() => {
      if (stageEl) stageEl.scrollTop = 0;
      progress = chapterCount > 1 ? Math.round((sectionIndex / (chapterCount - 1)) * 100) : 100;
      rememberPosition();
    });
  }

  function move(delta) {
    if (format === 'pdf') goToPage(pdfPage + delta);
    else goToSection(sectionIndex + delta);
  }

  function setZoom(value) {
    zoomMode = value;
    void renderPage();
  }

  // Position is remembered through the media progress route only when the
  // window is signed in: a signed-out window has no durable place to keep it,
  // and the shell answers the route with auth-required.
  let lastSavedAt = 0;
  let remembered = '';
  function rememberPosition() {
    if (!authNeeded && sourceKey) {
      const identity = mediaSourceIdentity(source);
      if (identity) remembered = identity;
    }
  }

  async function rememberCurrentSource() {
    if (!sourceKey || !sourceIdentity || !effectiveContext?.windowTitle) return;
    if (await rememberRecentMedia(format, source)) recentFiles = await loadRecentMedia(format);
  }

  async function refreshRecentFiles() {
    if (recentFiles.length) return;
    try {
      recentFiles = await loadRecentMedia(format);
    } catch (_err) {
      recentFiles = [];
    }
  }

  async function restorePosition() {
    if (!sourceKey) return;
    try {
      const res = await fetch('/api/media/progress?kind=' + encodeURIComponent(format) + '&identity=' + encodeURIComponent(sourceIdentity || sourceKey));
      if (!res.ok) return;
      const body = await res.json();
      const place = Number(body.current_time) || 0;
      if (!place) return;
      if (format === 'pdf' && pdfDoc) {
        goToPage(Math.floor(place) + 1);
      } else if (format === 'epub' && chapterCount) {
        goToSection(Math.floor(place));
      }
      flash('Resumed where you left off');
    } catch (_err) {
      // Resume is additive; reading must never wait on it.
    }
  }

  function savePositionNow() {
    if (!sourceKey || authNeeded) return;
    const now = Date.now();
    if (now - lastSavedAt < 1500) return;
    lastSavedAt = now;
    const position = format === 'pdf' ? pdfPage - 1 : sectionIndex;
    const total = format === 'pdf' ? pageCount : chapterCount;
    try {
      fetch('/api/media/progress', {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        keepalive: true,
        body: JSON.stringify({
          kind: format,
          identity: sourceIdentity || sourceKey,
          current_time: position,
          duration: total,
          playback_rate: 1,
        }),
      }).catch(() => {});
    } catch (_err) {
      // Progress is best effort; it never interrupts reading.
    }
  }

  function updateProgress() {
    if (format === 'pdf') return;
    if (!stageEl) return;
    const scrollable = Math.max(0, stageEl.scrollHeight - stageEl.clientHeight);
    const within = scrollable > 0 ? stageEl.scrollTop / scrollable : 0;
    progress = chapterCount > 1 ? Math.round(((sectionIndex + within) / chapterCount) * 100) : Math.round(within * 100);
    savePositionNow();
  }

  function changeFont(delta) {
    changePrefs({ fontSize: clampNumber(prefs.fontSize + delta, FONT_STEPS.min, FONT_STEPS.max) });
  }

  function changePrefs(next) {
    prefs = normalizePrefs({ ...prefs, ...next });
    savePrefs(window.localStorage, prefs);
  }

  function runSearch() {
    const needle = cleanText(searchTerm);
    if (!needle) {
      searchHits = [];
      hitIndex = -1;
      return;
    }
    const target = format === 'pdf' ? pdfSearchModel?.model : model;
    searchHits = target ? searchDocuments(target, needle) : [];
    hitIndex = searchHits.length ? 0 : -1;
  }

  function searchModelForJump() {
    return format === 'pdf' ? pdfSearchModel?.model : model;
  }

  function jumpToHit(index) {
    if (!searchHits.length) return;
    const wrapped = ((index % searchHits.length) + searchHits.length) % searchHits.length;
    hitIndex = wrapped;
    const hit = searchHits[wrapped];
    if (format === 'pdf') {
      goToPage(hit.page || 1);
    } else {
      goToSection(hit.sectionIndex);
      tick().then(() => {
        const target = stageEl?.querySelector('[data-reader-block="' + hit.blockIndex + '"]');
        target?.scrollIntoView({ block: 'center' });
      });
    }
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
    loadedSourceKey = '';
    model = null;
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    if (sourceKey) await loadSource();
  }

  function openAuth() {
    dispatch('authexpired', { kind: 'app_launch', appId: 'reader', appName: 'Reader', appContext: { windowTitle: source.title || 'Reader' } });
  }

  function handleKeydown(event) {
    if (event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement) return;
    if (event.key === 'ArrowRight' || event.key === 'PageDown' || event.key === 'j') {
      if (canNext) {
        event.preventDefault();
        move(1);
      }
    } else if (event.key === 'ArrowLeft' || event.key === 'PageUp' || event.key === 'k') {
      if (canPrev) {
        event.preventDefault();
        move(-1);
      }
    } else if (event.key === 'f' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      searchOpen = true;
      tick().then(() => document.querySelector('[data-reader-search]')?.focus());
    }
  }

  onMount(() => {
    prefs = loadPrefs(window.localStorage);
    void refreshRecentFiles();
    if (sourceKey) void loadSource();
    else if (authenticated) {
      loading = false;
      status = 'empty';
      void refreshRecentFiles();
    } else {
      openSample();
    }
    const stopLive = addLiveEventListener((message) => {
      if (liveEventKind(message) === 'media.recent.updated' && liveEventPayload(message).kind === format) {
        void loadRecentMedia(format).then((list) => {
          recentFiles = list;
        }).catch(() => {});
      }
    });
    const onHide = () => savePositionNow();
    window.addEventListener('pagehide', onHide);
    document.addEventListener('visibilitychange', onHide);
    return () => {
      stopLive();
      window.removeEventListener('pagehide', onHide);
      document.removeEventListener('visibilitychange', onHide);
      if (toastTimer) clearTimeout(toastTimer);
      if (activeRenderTask) activeRenderTask.cancel();
    };
  });
</script>


<svelte:window on:keydown={handleKeydown} />

<section
  class="reader-shell {tone}"
  data-reader-app
  data-reader-format={format}
  data-reader-status={status}
  data-media-kind={format}
  aria-label={'Reader: ' + (model?.title || source.title || 'Reader')}
  tabindex="-1"
>
  <header class="reader-top" data-reader-top>
    <div class="reader-identity">
      <h1 data-reader-title>{model?.title || source.title || 'Reader'}</h1>
      <p>
        {#if model?.sample}
          <span class="reader-chip" data-reader-chip="sample">Sample</span>
        {:else}
          <span class="reader-chip" data-reader-chip="format">{format === 'pdf' ? 'PDF' : 'EPUB'}</span>
        {/if}
        {#if model?.author}<span data-reader-author>{model.author}</span>{/if}
        {#if words}<span data-reader-words>{words.toLocaleString()} words – {minutes} min</span>{/if}
        {#if authNeeded}<span data-reader-auth-note>Preview</span>{/if}
      </p>
    </div>
    <div class="reader-top-actions">
      <button
        type="button"
        class="reader-icon-button"
        aria-expanded={tocOpen}
        aria-controls="reader-toc"
        title="Table of contents"
        data-reader-toc-toggle
        on:click={() => (tocOpen = !tocOpen)}
      >☰<span class="reader-visually-hidden">Table of contents</span></button>
      <button
        type="button"
        class="reader-icon-button"
        aria-expanded={searchOpen}
        title="Search this document"
        data-reader-search-toggle
        on:click={() => {
          searchOpen = !searchOpen;
          if (searchOpen) tick().then(() => document.querySelector('[data-reader-search]')?.focus());
        }}
      >⌕<span class="reader-visually-hidden">Search</span></button>
      {#if authNeeded}
        <button type="button" class="reader-button" data-reader-signin on:click={openAuth}>Sign in to read more</button>
      {/if}
    </div>
  </header>

  <div class="reader-tools" data-reader-tools role="toolbar" aria-label="Reading controls">
    <div class="reader-group">
      <button type="button" class="reader-button" data-reader-prev on:click={() => move(-1)} disabled={!canPrev}>
        {format === 'pdf' ? 'Previous page' : 'Previous chapter'}
      </button>
      <button type="button" class="reader-button" data-reader-next on:click={() => move(1)} disabled={!canNext}>
        {format === 'pdf' ? 'Next page' : 'Next chapter'}
      </button>
    </div>

    {#if format === 'pdf'}
      <label class="reader-group">
        <span>page</span>
        <input
          class="reader-input reader-page-box"
          type="number"
          min="1"
          max={pageCount || undefined}
          value={pdfPage}
          data-reader-page
          on:change={(event) => goToPage(event.currentTarget.value)}
        />
        <span data-reader-page-count>{pageLabel}</span>
      </label>
      <label class="reader-group">
        <span>zoom</span>
        <select class="reader-select" data-reader-zoom bind:value={zoomMode} on:change={() => setZoom(zoomMode)}>
          {#each ZOOM_MODES as mode}
            <option value={mode.value}>{mode.label}</option>
          {/each}
        </select>
      </label>
    {:else}
      <label class="reader-group" data-grow="true">
        <span>chapter</span>
        <select
          class="reader-select"
          data-reader-toc
          value={sectionIndex}
          on:change={(event) => goToSection(Number(event.currentTarget.value))}
        >
          {#each model?.sections || [] as entry, index}
            <option value={index}>{index + 1}. {entry.title}</option>
          {/each}
        </select>
      </label>
      <div class="reader-group">
        <button type="button" class="reader-icon-button" data-reader-font-smaller title="Smaller text" aria-label="Smaller text" on:click={() => changeFont(-1)} disabled={prefs.fontSize <= FONT_STEPS.min}>A-</button>
        <span data-reader-font-size>{prefs.fontSize}px</span>
        <button type="button" class="reader-icon-button" data-reader-font-larger title="Larger text" aria-label="Larger text" on:click={() => changeFont(1)} disabled={prefs.fontSize >= FONT_STEPS.max}>A+</button>
      </div>
      <label class="reader-group">
        <span>width</span>
        <select class="reader-select" data-reader-measure value={prefs.measure} on:change={(event) => changePrefs({ measure: Number(event.currentTarget.value) })}>
          {#each MEASURE_STEPS as step}
            <option value={step}>{MEASURE_NAMES[step]}</option>
          {/each}
        </select>
      </label>
      <div class="reader-group" role="group" aria-label="Page tone">
        {#each READER_THEMES as theme}
          <button
            type="button"
            class="reader-button"
            data-reader-tone={theme}
            aria-pressed={prefs.theme === theme}
            on:click={() => changePrefs({ theme })}
          >{theme}</button>
        {/each}
      </div>
    {/if}
  </div>

  <div class="reader-body" data-toc-open={tocOpen ? 'true' : 'false'}>
    {#if tocOpen}
      <nav class="reader-toc" id="reader-toc" aria-label="Chapters" data-reader-toc-panel>
        <h2>Contents</h2>
        {#if chapterCount}
          {#each model?.sections || [] as entry, index}
            <button
              type="button"
              class="reader-toc-item"
              aria-current={index === sectionIndex ? 'true' : 'false'}
              data-reader-toc-item
              on:click={() => {
                goToSection(index);
                if (stageEl) stageEl.scrollTop = 0;
              }}
            >
              <span>{index + 1}. {entry.title}</span>
              <small>{entry.blocks.length} blocks</small>
            </button>
          {/each}
        {:else}
          <p class="reader-status">Contents are read page by page in a PDF.</p>
          <button type="button" class="reader-toc-item" aria-current="true" on:click={() => goToSection(0)}>
            <span>All pages</span>
            <small>{pageCount || 0} pages</small>
          </button>
        {/if}
      </nav>
    {/if}

    <div class="reader-stage" bind:this={stageEl} data-reader-stage on:scroll={updateProgress}>
      {#if loading}
        <p class="reader-status" aria-live="polite">Opening {source.title || 'document'}...</p>
      {:else if error}
        <div class="reader-alert" role="alert" data-reader-error>
          <strong>Reader could not open this document</strong>
          <span>{error}</span>
          {#if model?.sample === false || !model}
            <span>The PDF and EPUB windows are still in the launcher.</span>
          {/if}
        </div>
      {:else if format === 'pdf' && pdfDoc}
        <div class="reader-pdf-stage">
          <div class="reader-pdf-page" data-reader-page-canvas data-reader-rendered={rendered ? 'true' : 'false'}>
            <canvas bind:this={canvasEl} aria-label={'Page ' + pdfPage + ' of ' + (pageCount || '?')}></canvas>
          </div>
        </div>
        {#if rendering}<span class="reader-status" aria-live="polite" data-reader-rendering>Rendering page {pdfPage}...</span>{/if}
      {:else if section}
        <article class="reader-sheet" data-reader-sheet style={sheetStyle} aria-labelledby="reader-section-title">
          <h2 class="reader-visually-hidden" id="reader-section-title">{section.title}</h2>
          {#if chapterCount > 1}
            <p class="reader-status" data-reader-section-count>Chapter {sectionIndex + 1} of {chapterCount} – {section.title}</p>
          {/if}
          {#each section.blocks as block, index}
            {#if block.tag === 'h1' || block.tag === 'h2' || block.tag === 'h3' || block.tag === 'h4'}
              {#if block.tag === 'h3' && block.page}
                <h3 class="reader-page-mark" data-reader-block={index} data-reader-block-page={block.page}>Page {block.page}</h3>
              {:else}
                <h3 data-reader-block={index}>{block.text}</h3>
              {/if}
            {:else if block.tag === 'blockquote'}
              <blockquote data-reader-block={index}>{block.text}</blockquote>
            {:else if block.tag === 'pre'}
              <pre data-reader-block={index}>{block.text}</pre>
            {:else if block.tag === 'li'}
              <ul><li data-reader-block={index}>{block.text}</li></ul>
            {:else}
              <p data-reader-block={index}>{block.text}</p>
            {/if}
          {/each}
        </article>
      {:else}
        <div class="reader-empty" data-reader-empty>
          <h2>Nothing open yet</h2>
          <p>Open a PDF or an EPUB from Files, or read the sample below. Reader keeps your place once you are signed in.</p>
          <button type="button" class="reader-button" data-reader-open-sample on:click={openSample}>Open the sample document</button>
          {#if recentFiles.length}
            <div class="reader-recents" data-reader-recents>
              <span>Recently opened</span>
              {#each recentFiles as recent}
                <button type="button" class="reader-recent-item" data-reader-recent on:click={() => openRecentFile(recent)}>
                  <strong>{recent.title}</strong>
                  <small>{recent.filePath || recent.sourceUrl}</small>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if searchOpen}
        <div class="reader-results" data-reader-results role="dialog" aria-label="Search results">
          <header>
            <label class="reader-group">
              <span>find</span>
              <input
                class="reader-input"
                type="search"
                data-reader-search
                placeholder="Search this document"
                bind:value={searchTerm}
                on:input={runSearch}
                on:keydown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault();
                    jumpToHit(hitIndex + (event.shiftKey ? -1 : 1));
                  }
                }}
              />
            </label>
            <span data-reader-search-count>{searchHits.length} matches</span>
            <button type="button" class="reader-icon-button" aria-label="Close search" on:click={() => (searchOpen = false)}>×</button>
          </header>
          {#if searchHits.length}
            {#each searchHits.slice(0, 40) as hit, index}
              <button
                type="button"
                class="reader-hit"
                aria-current={index === hitIndex ? 'true' : 'false'}
                data-reader-hit
                on:click={() => jumpToHit(index)}
              >
                <strong>{format === 'pdf' && hit.page ? 'Page ' + hit.page : hit.title}</strong>
                <span>{hit.snippet}</span>
              </button>
            {/each}
          {:else if searchTerm.trim()}
            <p class="reader-status">No matches for this document.</p>
          {:else}
            <p class="reader-status">Type to find text. Enter moves to the next match, Shift+Enter the previous.</p>
          {/if}
        </div>
      {/if}
    </div>
  </div>

  <footer class="reader-footer" data-reader-footer>
    <div class="reader-progress" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={progress} data-reader-progress>
      <i style={'width: ' + progress + '%'}></i>
    </div>
    <span data-reader-progress-label>{progress}%</span>
    <span data-reader-position>{format === 'pdf' ? 'Page ' + pageLabel : 'Chapter ' + (chapterCount ? sectionIndex + 1 : 0) + ' / ' + chapterCount}</span>
    {#if model?.sample}<span data-reader-sample-note>Signed-out sample</span>{/if}
  </footer>

  <p class="reader-visually-hidden" aria-live="polite" data-reader-live>{toast}</p>
</section>

<!-- One stylesheet for both surfaces: this window and /read.html import the
     same reader.css, so the signed-out page and the desktop app look alike. -->
<style>
  @import './reader.css';
</style>
