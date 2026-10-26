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
    escapeRegExp,
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
    READER_THEME_NAMES,
    loadPrefs,
    normalizePrefs,
    savePrefs,
  } from './reader-prefs.js';
  import { modelFromPdfPages } from './pdf-text.js';
  import {
    READER_UPLOAD_ACCEPT,
    READER_UPLOAD_ACCEPT_TYPES,
    inspectUploadCandidate,
    uploadFormatFor,
    uploadReaderFile,
  } from './open-file.js';
  import './reader.css';

  export let appContext = {};
  export let windowId = '';
  export let authenticated = false;

  const dispatch = createEventDispatcher();
  // Phones and narrow containers put the secondary controls behind one
  // overflow button; the shell sets the class on both surfaces.
  const COMPACT_QUERY = '(max-width: 640px)';
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
  let toast = '';
  let toastTimer = 0;
  let fileInputEl = null;
  let compact = false;
  let prefsOpen = false;
  let overflowOpen = false;
  let toolbarEl = null;
  let stageWidth = 0;
  let renderRaf = 0;
  let closeQuery = null;
  let uploadDragDepth = 0;
  let dropping = false;
  let uploading = false;
  let uploadName = '';
  let uploadProgress = 0;
  let uploadError = '';
  let uploadStatus = '';
  let uploadAbort = null;

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, appContext?.readerKind || appContext?.kind || 'pdf');
  $: sourceKey = source.displayUrl || '';
  $: sourceIdentity = mediaSourceIdentity(source);
  $: extractedText = item?.text_content || effectiveContext.textContent || '';
  // Recents follow the window's source, not just its navigation: a file handed
  // in through appContext on mount (a window opened from Files) resolves its
  // source here and never calls navigateToSource, so this is where it is
  // remembered too. The identity guard is what keeps a resolved source from
  // writing a recents entry again on every store update, and what stops one
  // source's write from racing the next one's (PdfApp.svelte does the same).
  $: if (authenticated && sourceKey && sourceIdentity && sourceIdentity !== rememberedSourceIdentity) {
    void rememberCurrentSource();
  }
  // Changing the page, the chapter, the zoom or the size of the page area
  // re-renders the PDF: a page is drawn from a measured width, so the first
  // draw and every later one go through the same path.
  // A draw is asked for when the page is present and the area is measured, and
  // again once it has drawn so a resize or a new scale redraws it.
  $: if (format === 'pdf' && pdfDoc && canvasEl && stageWidth) {
    scheduleRender();
  }
  $: if (!pdfDoc && rendered) rendered = false;
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
  $: isEmpty = status === 'empty' && !loading && !error && !model && !sourceKey;
  $: showContents = Boolean(chapterCount);

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

  // The filter the picker uses: a window with a file open offers that file's
  // format, and a window with no file yet offers both formats.
  function uploadAccept() {
    const formats = uploadFormats();
    return formats.length > 1 ? READER_UPLOAD_ACCEPT : (formats[0] || READER_UPLOAD_ACCEPT);
  }

  function uploadFormats() {
    return formatsForSource(source.filePath || source.sourceUrl || source.mediaType || appContext?.readerKind || appContext?.kind);
  }

  function formatsForSource(key) {
    const spec = uploadFormatFor(String(key || '').split(/[?#]/)[0]);
    return spec ? [spec.mediaType] : READER_UPLOAD_ACCEPT_TYPES.slice();
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
      .filter((entry) => /xhtml|html/i.test(entry.mediaType) || /\.x?html?$/i.test(entry.href));
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
    error = '';
    authNeeded = false;
    status = 'sample';
  }

  // Open a source this window already knows (a file just written, or one it
  // was handed by Files or the URL): there is no library item to look up, so
  // nothing here can answer "content lookup failed" for a source that is only
  // a path. The parsers are the same ones loadSource uses.
  async function loadSourceDirect() {
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
      fileName: effectiveContext.fileName || effectiveContext.filePath,
      sourceUrl: source.sourceUrl,
      kind: effectiveContext.readerKind || appContext?.readerKind || appContext?.kind,
      extractedText,
    });
    try {
      if (format === 'epub') {
        const res = await fetch(sourceKey, sourceFetchOptions(sourceKey));
        if (!res.ok) throw new Error('EPUB source failed (' + res.status + ')');
        model = await parseEpub(await res.arrayBuffer());
      } else {
        await loadPdfDocument(sourceKey);
      }
      status = 'ready';
      await tick();
      await restorePosition();
    } catch (err) {
      error = err?.message || 'Reader could not open this source';
      status = 'error';
    } finally {
      loading = false;
    }
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
      pages.push(content.items.map((entry) => entry.str || '').join(' ').replace(/\s+/g, ' ').trim());
    }
    const modelForSearch = modelFromPdfPages(pages, { title: source.title });
    pdfSearchModel = { doc, model: modelForSearch };
    if (searchOpen && searchTerm.trim()) runSearch();
    return modelForSearch;
  }

  // Fit modes ask the page area for its measured width. A page is never sized
  // from a fallback: with no measurement there is nothing honest to fit it to.
  function scaleFor(page, stageRect) {
    const base = page.getViewport({ scale: 1 });
    const width = Math.max(160, Math.floor(stageRect.width));
    const height = Math.max(160, Math.floor(stageRect.height));
    if (zoomMode === 'page-width') return clampNumber(width / base.width, 0.15, 5);
    if (zoomMode === 'page-fit') return clampNumber(Math.min(width / base.width, height / base.height), 0.15, 5);
    return clampNumber((Number(zoomMode) || 100) / 100, 0.15, 5);
  }

  // A page is drawn only once its page area has a measured width: pdfjs gets
  // no fallback size, so the canvas is exactly the width the layout asked for
  // times the screen's pixel ratio, and it is shown the moment it is drawn.
  function scheduleRender() {
    if (renderRaf) return;
    renderRaf = requestAnimationFrame(() => {
      renderRaf = 0;
      void renderPage();
    });
  }

  async function renderPage() {
    const seq = ++renderSeq;
    // A page needs both the canvas being on the page and a measured page area.
    // Both are reactive: the canvas is bound when the page markup renders (a
    // draw asked for before that returns here and is asked for again when the
    // area reports its width), and the ResizeObserver is the only thing that
    // says how wide the page may be drawn.
    if (!pdfDoc || !canvasEl || !stageWidth) return;
    const stageRect = stageEl?.getBoundingClientRect();
    if (!stageRect || stageRect.width < 40 || stageRect.height < 40) {
      rendered = false;
      return;
    }
    if (activeRenderTask && activeRenderPromise) {
      activeRenderTask.cancel();
      try {
        await activeRenderPromise;
      } catch (err) {
        if (err?.name !== 'RenderingCancelledException') throw err;
      }
    }
    if (seq !== renderSeq) return;
    rendered = false;
    let task = null;
    try {
      await tick();
      const page = await pdfDoc.getPage(pdfPage);
      if (seq !== renderSeq) return;
      const scale = scaleFor(page, stageRect);
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

  let rememberedSourceIdentity = '';
  async function rememberCurrentSource() {
    if (!sourceKey || !sourceIdentity || !effectiveContext?.windowTitle) return;
    rememberedSourceIdentity = sourceIdentity;
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


  // ---- Toolbar: the reading settings popover and the phone overflow menu ----

  function togglePrefs() {
    prefsOpen = !prefsOpen;
    if (prefsOpen) overflowOpen = false;
  }

  function toggleOverflow() {
    overflowOpen = !overflowOpen;
    if (overflowOpen) prefsOpen = false;
  }

  function toggleSearch() {
    searchOpen = !searchOpen;
    overflowOpen = false;
    if (searchOpen) tick().then(() => document.querySelector('[data-reader-search]')?.focus());
  }

  // A popover closes on a press outside it and on Escape, so a tap on the page
  // never leaves a panel hanging over the text.
  function onDocumentPointerDown(event) {
    if (!prefsOpen && !overflowOpen) return;
    const target = event.target;
    if (toolbarEl && target instanceof Node && toolbarEl.contains(target)) return;
    prefsOpen = false;
    overflowOpen = false;
  }

  function onDocumentKeydown(event) {
    if (event.key !== 'Escape') return;
    prefsOpen = false;
    overflowOpen = false;
  }

  // A hit carries the snippet searchDocuments cut around the match (the model
  // itself is not flat: a block keeps its own line breaks and tag text), so the
  // snippet is what is split around the match, exactly as it was cut.
  // searchDocuments flattens every run of whitespace in a block before it cuts
  // the snippet, so a snippet folded through a needle with spaces still lands
  // on the text the match came from.
  function hitParts(hit) {
    const flat = String(hit?.snippet || '').replace(/\s+/g, ' ');
    const needle = cleanText(searchTerm);
    const match = needle ? new RegExp(escapeRegExp(needle), 'i').exec(flat) : null;
    if (!match) {
      return { before: flat, match: '', after: '' };
    }
    return {
      before: flat.slice(0, match.index),
      match: flat.slice(match.index, match.index + match[0].length),
      after: flat.slice(match.index + match[0].length),
    };
  }

  function hitPlace(hit) {
    if (format === 'pdf' && hit.page) return 'Page ' + hit.page;
    const chapter = (hit.sectionIndex || 0) + 1;
    return 'Chapter ' + chapter + ' · ¶' + ((hit.blockIndex || 0) + 1);
  }

  // ---- Opening a file from disk ----

  function openFilePicker() {
    if (!authenticated) {
      // Signed out there is no library to hold the file, and no session to
      // write it with: ask the shell for sign-in and change nothing here.
      askForUploadSession();
      return;
    }
    uploadError = '';
    uploadStatus = '';
    if (fileInputEl) fileInputEl.value = '';
    fileInputEl?.click();
  }

  function onFilePickerChange(event) {
    const input = event.currentTarget;
    const files = Array.from(input?.files || []);
    if (input) input.value = '';
    if (input) input.blur();
    if (files.length) void acceptDroppedFiles(files);
  }

  // The drop target is announced where it can be understood without seeing it
  // (a labelled region and a live line saying what a drop will do), and it
  // only appears for a drag that carries files, so a text drag leaves the
  // page alone. The depth counter keeps the overlay steady while the pointer
  // crosses child elements.
  function carriesFiles(event) {
    const types = event?.dataTransfer?.types;
    if (!types) return false;
    return Array.from(types).includes('Files');
  }

  function onDragEnter(event) {
    if (!carriesFiles(event)) return;
    uploadDragDepth += 1;
    dropping = true;
  }

  function onDragOver(event) {
    if (!carriesFiles(event)) return;
    event.preventDefault();
    try {
      event.dataTransfer.dropEffect = authenticated ? 'copy' : 'none';
    } catch (_err) {
      // A read-only dataTransfer in some browsers; the drop still works.
    }
  }

  function onDragLeave(event) {
    uploadDragDepth = Math.max(0, uploadDragDepth - 1);
    if (event?.relatedTarget) return;
    uploadDragDepth = 0;
    dropping = false;
  }

  function onDrop(event) {
    event.preventDefault();
    uploadDragDepth = 0;
    dropping = false;
    const files = Array.from(event?.dataTransfer?.files || []);
    if (!files.length) return;
    if (!authenticated) {
      openFilePicker();
      return;
    }
    void acceptDroppedFiles(files);
  }

  // Signed out (or a session the shell's upload could not renew) there is no
  // library to hold the file: Open file and a drop ask the shell for sign-in
  // and do nothing else — no sample swapped in, no toast, no error line.
  function askForUploadSession() {
    dispatch('authrequired', { kind: 'file_upload', appId: 'reader', appName: 'Reader' });
  }

  async function acceptDroppedFiles(files) {
    if (!authenticated) {
      askForUploadSession();
      return;
    }
    if (uploading) return;
    const file = files[0];
    uploadError = '';
    uploadStatus = '';
    uploadProgress = 0;
    uploadName = String(file?.name || '');
    uploading = true;
    uploadAbort = typeof AbortController === 'function' ? new AbortController() : null;
    try {
      // Checked here, before a byte is sent: a name Files would refuse, an
      // empty file, one over the size ceiling, or one whose contents are not
      // the format its name claims is answered on this line, and nothing is
      // uploaded. The same check runs again where the file is written, so a
      // file that changed under us still cannot land unread.
      const candidate = await inspectUploadCandidate(file);
      if (!candidate.ok) {
        uploadError = candidate.error;
        return;
      }
      const result = await uploadReaderFile(file, {
        signal: uploadAbort?.signal,
        onProgress: ({ fraction }) => {
          uploadProgress = Math.max(0, Math.min(1, Number(fraction) || 0));
        },
      });
      if (!result.ok) {
        if (result.kind === 'auth') {
          askForUploadSession();
          return;
        }
        uploadError = result.error;
        return;
      }
      uploadProgress = 1;
      uploadStatus = (result.operation === 'created' || !result.operation)
        ? 'Saved ' + result.name + ' to your files. Opening it...'
        : 'Replaced ' + result.name + ' in your files. Opening it...';
      await openWrittenFile({ name: result.name, format: result.format, mediaType: result.mediaType, size: result.size });
    } finally {
      uploading = false;
      uploadAbort = null;
    }
  }

  function cancelUpload() {
    uploadAbort?.abort();
  }

  // Open a file just written to top-level Files through the window's real
  // source path: the same appContext shape Files hands to a media app, saved
  // into the window, so a reload opens the same document.
  async function openWrittenFile(written) {
    const context = {
      windowTitle: written.name,
      title: written.name,
      fileName: written.name,
      filePath: written.name,
      sourcePath: written.name,
      sourceUrl: '',
      mediaType: written.mediaType,
      contentId: '',
      readerKind: written.format,
    };
    await navigateToSource(context, written.format);
  }

  // Swap this window's source and open it, for a recent file or an upload.
  // Opening writes the recents entry and (once the first page renders) the
  // stored position for the new file, so both continue where reading does.
  async function navigateToSource(context, kind) {
    selectedContext = context;
    item = null;
    error = '';
    authNeeded = false;
    loadedSourceKey = '';
    model = null;
    pdfDoc = null;
    pageCount = 0;
    pdfPage = 1;
    format = kind === 'epub' ? 'epub' : 'pdf';
    dispatch('contextchange', { windowId, appContext: context, title: context.windowTitle });
    await tick();
    if (sourceKey) await loadSource();
    else await loadSourceDirect();
    // Remembering here as well as in the source reactive statement above is
    // harmless: the identity guard makes the second call for one source a
    // no-op.
    void rememberCurrentSource();
  }

  function openRecent(entry) {
    const kind = String(entry?.kind || '').toLowerCase();
    return navigateToSource(recentMediaAppContext(entry), kind === 'epub' ? 'epub' : 'pdf');
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
    // A page is drawn from the page area's measured width, and draw again when
    // that width changes (a window resize, the contents panel opening, a
    // phone turning): the observer is the one place that says \"now measure\".
    const observer = typeof ResizeObserver === 'function' && stageEl
      ? new ResizeObserver((entries) => {
          const rect = entries[0]?.contentRect;
          const width = Math.round(rect?.width || 0);
          if (width === stageWidth) return;
          stageWidth = width;
        })
      : null;
    observer?.observe(stageEl);
    if (typeof window.matchMedia === 'function') {
      const query = window.matchMedia(COMPACT_QUERY);
      compact = query.matches;
      const onChange = (event) => {
        compact = event.matches;
        prefsOpen = false;
        overflowOpen = false;
      };
      query.addEventListener('change', onChange);
      closeQuery = () => query.removeEventListener('change', onChange);
    }
    document.addEventListener('pointerdown', onDocumentPointerDown, true);
    document.addEventListener('keydown', onDocumentKeydown);
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
      if (renderRaf) cancelAnimationFrame(renderRaf);
      observer?.disconnect();
      closeQuery?.();
      document.removeEventListener('pointerdown', onDocumentPointerDown, true);
      document.removeEventListener('keydown', onDocumentKeydown);
    };
  });


</script>

<svelte:window
  on:keydown={handleKeydown}
  on:dragenter={onDragEnter}
  on:dragover={onDragOver}
  on:dragleave={onDragLeave}
  on:drop={onDrop}
/>

<section
  class="reader-shell {tone}"
  data-reader-app
  data-reader-format={format}
  data-reader-status={status}
  data-reader-window
  data-media-kind={format}
  aria-label={'Reader: ' + (model?.title || source.title || 'Reader')}
  tabindex="-1"
>
  <input
    class="reader-file-input"
    type="file"
    accept={uploadAccept()}
    data-reader-file-input
    bind:this={fileInputEl}
    on:change={onFilePickerChange}
    tabindex="-1"
    aria-hidden="true"
  />

  {#if !isEmpty}
    <div class="reader-toolbar" role="toolbar" aria-label="Reading controls" data-reader-toolbar bind:this={toolbarEl}>
      {#if showContents}
        <button
          type="button"
          class="reader-icon"
          aria-expanded={tocOpen}
          aria-controls="reader-toc"
          aria-label="Contents"
          title="Contents"
          data-reader-toc-toggle
          on:click={() => (tocOpen = !tocOpen)}
        >
          <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
            <path d="M3 4h10M3 8h10M3 12h10" />
          </svg>
        </button>
      {/if}
      <button
        type="button"
        class="reader-icon"
        aria-label={format === 'pdf' ? 'Previous page' : 'Previous chapter'}
        title={format === 'pdf' ? 'Previous page' : 'Previous chapter'}
        data-reader-prev
        disabled={!canPrev}
        on:click={() => move(-1)}
      >
        <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
          <path d="M10 3.5 5.5 8 10 12.5" />
        </svg>
      </button>
      <button
        type="button"
        class="reader-icon"
        aria-label={format === 'pdf' ? 'Next page' : 'Next chapter'}
        title={format === 'pdf' ? 'Next page' : 'Next chapter'}
        data-reader-next
        disabled={!canNext}
        on:click={() => move(1)}
      >
        <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
          <path d="M6 3.5 10.5 8 6 12.5" />
        </svg>
      </button>

      {#if format === 'pdf'}
        <div class="reader-position" data-reader-position-field>
          <input
            class="reader-page-input"
            type="number"
            min="1"
            max={pageCount || undefined}
            value={pdfPage}
            aria-label="Page"
            title={'Page of ' + (pageCount || '?')}
            data-reader-page
            on:change={(event) => goToPage(event.currentTarget.value)}
          />
          <span class="reader-of" data-reader-page-count>/ {pageCount || '?'}</span>
        </div>
      {:else}
        <select
          class="reader-chapter"
          aria-label="Chapter"
          title="Chapter"
          data-reader-chapter
          value={sectionIndex}
          on:change={(event) => goToSection(Number(event.currentTarget.value))}
        >
          {#each model?.sections || [] as entry, index}
            <option value={index}>{index + 1} / {chapterCount} · {entry.title}</option>
          {/each}
        </select>
      {/if}

      <span class="reader-toolbar-spacer"></span>

      {#if format === 'pdf' && !compact}
        <select
          class="reader-zoom"
          aria-label="Zoom"
          title="Zoom"
          data-reader-zoom
          bind:value={zoomMode}
          on:change={() => setZoom(zoomMode)}
        >
          {#each ZOOM_MODES as mode}
            <option value={mode.value}>{mode.label}</option>
          {/each}
        </select>
      {/if}

      {#if !compact}
        <div class="reader-popover-anchor">
          <button
            type="button"
            class="reader-aa {prefsOpen ? 'is-open' : ''}"
            aria-expanded={prefsOpen}
            aria-haspopup="dialog"
            aria-label="Reading settings"
            title="Reading settings"
            data-reader-prefs-toggle
            on:click={togglePrefs}
          >Aa</button>
          {#if prefsOpen}
            <div class="reader-popover" role="dialog" aria-label="Reading settings" data-reader-prefs>
              <div class="reader-setting">
                <span>Text size</span>
                <div class="reader-stepper">
                  <button
                    type="button"
                    class="reader-icon reader-icon-sm"
                    aria-label="Smaller text"
                    title="Smaller text"
                    data-reader-font-smaller
                    disabled={prefs.fontSize <= FONT_STEPS.min}
                    on:click={() => changeFont(-1)}
                  >
                    <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
                      <path d="M3.5 8h9" />
                    </svg>
                  </button>
                  <span class="reader-setting-value" data-reader-font-size>{prefs.fontSize}px</span>
                  <button
                    type="button"
                    class="reader-icon reader-icon-sm"
                    aria-label="Larger text"
                    title="Larger text"
                    data-reader-font-larger
                    disabled={prefs.fontSize >= FONT_STEPS.max}
                    on:click={() => changeFont(1)}
                  >
                    <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
                      <path d="M3.5 8h9M8 3.5v9" />
                    </svg>
                  </button>
                </div>
              </div>
              <label class="reader-setting">
                <span>Width</span>
                <select
                  class="reader-select"
                  data-reader-measure
                  value={prefs.measure}
                  on:change={(event) => changePrefs({ measure: Number(event.currentTarget.value) })}
                >
                  {#each MEASURE_STEPS as step}
                    <option value={step}>{MEASURE_NAMES[step]}</option>
                  {/each}
                </select>
              </label>
              <label class="reader-setting">
                <span>Tone</span>
                <select
                  class="reader-select"
                  data-reader-tone-select
                  value={prefs.theme}
                  on:change={(event) => changePrefs({ theme: event.currentTarget.value })}
                >
                  {#each READER_THEMES as theme}
                    <option value={theme} data-reader-tone={theme}>{READER_THEME_NAMES[theme]}</option>
                  {/each}
                </select>
              </label>
            </div>
          {/if}
        </div>
      {/if}

      {#if !compact}
        <button
          type="button"
          class="reader-icon"
          aria-label="Search this document"
          title="Search this document"
          aria-expanded={searchOpen}
          data-reader-search-toggle
          on:click={toggleSearch}
        >
          <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
            <circle cx="7" cy="7" r="4" />
            <path d="M10.2 10.2 14 14" />
          </svg>
        </button>
        <button type="button" class="reader-open" data-reader-open-file on:click={openFilePicker}>
          Open file
        </button>
      {:else}
        <div class="reader-popover-anchor">
          <button
            type="button"
            class="reader-icon"
            aria-label="More controls"
            title="More controls"
            aria-haspopup="menu"
            aria-expanded={overflowOpen}
            data-reader-overflow-toggle
            on:click={toggleOverflow}
          >
            <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
              <circle cx="8" cy="3.5" r="0.9" />
              <circle cx="8" cy="8" r="0.9" />
              <circle cx="8" cy="12.5" r="0.9" />
            </svg>
          </button>
          {#if overflowOpen}
            <div class="reader-popover reader-popover-menu" role="menu" aria-label="More controls" data-reader-overflow>
              <button type="button" role="menuitem" class="reader-menu-item" data-reader-prefs-toggle on:click={togglePrefs}>
                Reading settings
              </button>
              <button type="button" role="menuitem" class="reader-menu-item" data-reader-search-toggle on:click={toggleSearch}>
                Search
              </button>
              {#if format === 'pdf'}
                <label class="reader-menu-item reader-menu-label">
                  <span>Zoom</span>
                  <select
                    class="reader-select"
                    data-reader-zoom
                    bind:value={zoomMode}
                    on:change={() => setZoom(zoomMode)}
                  >
                    {#each ZOOM_MODES as mode}
                      <option value={mode.value}>{mode.label}</option>
                    {/each}
                  </select>
                </label>
              {/if}
              <button type="button" role="menuitem" class="reader-menu-item" data-reader-open-file on:click={openFilePicker}>
                Open file
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  {/if}

  {#if uploading || uploadProgress > 0}
    <div class="reader-upload" data-reader-upload role="status" aria-live="polite">
      <div class="reader-upload-bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(uploadProgress * 100)} data-reader-upload-progress>
        <i style={'width: ' + Math.round(uploadProgress * 100) + '%'}></i>
      </div>
      <span>{uploadProgress > 0 && uploadProgress < 1 ? Math.round(uploadProgress * 100) + '% of ' + uploadName : uploadStatus || 'Saving ' + uploadName}</span>
      {#if uploading}
        <button type="button" class="reader-open" data-reader-upload-cancel on:click={cancelUpload}>Cancel</button>
      {/if}
    </div>
  {/if}
  {#if uploadError}
    <div class="reader-alert" role="alert" data-reader-upload-error>
      <strong>That file could not be opened</strong>
      <span>{uploadError}</span>
    </div>
  {/if}

  {#if dropping}
    <div class="reader-drop-veil" data-reader-drop-veil>
      <span class="reader-drop-badge">
        {authenticated ? 'Drop to open in Reader' : 'Drop to open the sample'}
      </span>
    </div>
  {/if}

  <div class="reader-body" data-toc-open={tocOpen && showContents ? 'true' : 'false'}>
    {#if tocOpen && showContents}
      <nav class="reader-toc" id="reader-toc" aria-label="Contents" data-reader-toc-panel>
        <button
          type="button"
          class="reader-toc-close"
          aria-label="Close contents"
          title="Close contents"
          on:click={() => (tocOpen = false)}
        >
          <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
            <path d="M4 4l8 8M12 4l-8 8" />
          </svg>
        </button>
        <p class="reader-toc-meta" data-reader-words>
          {#if model?.author}<span data-reader-author>{model.author}</span>{/if}
          {#if words}<span>{words.toLocaleString()} words · {minutes} min read</span>{/if}
        </p>
        <div class="reader-toc-list">
          {#each model?.sections || [] as entry, index}
            <button
              type="button"
              class="reader-toc-item"
              aria-current={index === sectionIndex ? 'true' : 'false'}
              data-reader-toc-item
              on:click={() => {
                goToSection(index);
                if (compact) tocOpen = false;
              }}
            >{entry.title}</button>
          {/each}
        </div>
        {#if authNeeded}
          <button type="button" class="reader-open reader-toc-signin" data-reader-signin on:click={openAuth}>Sign in to open your own files</button>
        {/if}
      </nav>
    {/if}

    <div class="reader-stage" bind:this={stageEl} data-reader-stage on:scroll={updateProgress}>
      {#if loading}
        <p class="reader-status" aria-live="polite">Opening {source.title || 'document'}…</p>
      {:else if error}
        <div class="reader-alert" role="alert" data-reader-error>
          <strong>Reader could not open this document</strong>
          <span>{error}</span>
          <button type="button" class="reader-open" data-reader-retry on:click={() => { error = ''; status = 'idle'; loadedSourceKey = ''; if (sourceKey) void loadSource(); else void loadSourceDirect(); }}>
            Try again
          </button>
        </div>
      {:else if format === 'pdf' && pdfDoc}
        <div class="reader-pdf-stage">
          <div class="reader-pdf-page" data-reader-page-canvas data-reader-rendered={rendered ? 'true' : 'false'}>
            <canvas bind:this={canvasEl} aria-label={'Page ' + pdfPage + ' of ' + (pageCount || '?')}></canvas>
          </div>
        </div>
        {#if !rendered}<span class="reader-visually-hidden" aria-live="polite" data-reader-rendering>Drawing page {pdfPage}</span>{/if}
      {:else if section}
        <article class="reader-sheet" data-reader-sheet data-choir-content style={sheetStyle}>
          {#if sectionIndex > 0 && section.title}
            <h2 class="reader-chapter-title">{section.title}</h2>
          {/if}
          {#each section.blocks as block, index}
            {#if block.tag === 'h1' || block.tag === 'h2' || block.tag === 'h3' || block.tag === 'h4'}
              <h3 data-reader-block={index}>{block.text}</h3>
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
      {:else if isEmpty}
        <div class="reader-empty" data-reader-empty>
          <h2>Open a document</h2>
          <p>PDF or EPUB, from your computer or from Files. You can also drop a file here.</p>
          <div class="reader-empty-actions">
            <button type="button" class="reader-open reader-open-primary" data-reader-open-file-empty on:click={openFilePicker}>Open file</button>
            <button type="button" class="reader-text-button" data-reader-open-sample on:click={openSample}>Read the sample</button>
          </div>
          {#if recentFiles.length}
            <div class="reader-recents" data-reader-recents>
              <p class="reader-recents-label">Recent</p>
              {#each recentFiles as recent (recent.identity)}
                <button type="button" class="reader-recent" data-reader-recent on:click={() => openRecent(recent)}>
                  <svg class="reader-glyph reader-recent-icon" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
                    <path d="M4 1.8h5l3 3v9.4H4z" />
                    <path d="M9 1.8v3h3" />
                  </svg>
                  <span class="reader-recent-name">{recent.title}</span>
                  <span class="reader-recent-folder">{recent.filePath || recent.sourceUrl || 'Files'}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if searchOpen}
        <div class="reader-results" data-reader-results role="dialog" aria-label="Search results">
          <header>
            <input
              class="reader-search-input"
              type="search"
              data-reader-search
              placeholder="Search this document"
              aria-label="Search this document"
              bind:value={searchTerm}
              on:input={runSearch}
              on:keydown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  jumpToHit(hitIndex + (event.shiftKey ? -1 : 1));
                }
              }}
            />
            <span class="reader-search-count" data-reader-search-count>{searchHits.length} {searchHits.length === 1 ? 'match' : 'matches'}</span>
            <button type="button" class="reader-icon reader-icon-sm" aria-label="Close search" title="Close search" on:click={() => (searchOpen = false)}>
              <svg class="reader-glyph" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
                <path d="M4 4l8 8M12 4l-8 8" />
              </svg>
            </button>
          </header>
          <div class="reader-results-list">
            {#if searchHits.length}
              {#each searchHits.slice(0, 40) as hit, index}
                {@const parts = hitParts(hit)}
                <button
                  type="button"
                  class="reader-hit"
                  aria-current={index === hitIndex ? 'true' : 'false'}
                  data-reader-hit
                  on:click={() => jumpToHit(index)}
                >
                  <span class="reader-hit-where">{hitPlace(hit)}</span>
                  <span class="reader-hit-text">{parts.before}<mark>{parts.match}</mark>{parts.after}</span>
                </button>
              {/each}
            {:else if searchTerm.trim()}
              <p class="reader-status">No matches in this document.</p>
            {:else}
              <p class="reader-status">Type to find text.</p>
            {/if}
          </div>
        </div>
      {/if}
    </div>
  </div>

  <p class="reader-visually-hidden" aria-live="polite" data-reader-live>{toast}</p>
</section>


