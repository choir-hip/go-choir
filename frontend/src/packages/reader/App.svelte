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
  import {
    READER_UPLOAD_ACCEPT,
    READER_UPLOAD_ACCEPT_TYPES,
    READER_UPLOAD_FORMATS,
    uploadFormatFor,
    uploadReaderFile,
  } from './open-file.js';
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
  let fileInputEl = null;
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
  data-media-kind={format}
  aria-label={'Reader: ' + (model?.title || source.title || 'Reader')}
  tabindex="-1"
>
  <div
    class="reader-drop-target"
    class:reader-drop-target-active={dropping}
    role="region"
    aria-label="Drop a PDF or EPUB here to open it"
    data-reader-drop
  >
    <input
      class="reader-visually-hidden"
      type="file"
      accept={uploadAccept()}
      data-reader-file-input
      bind:this={fileInputEl}
      on:change={onFilePickerChange}
      tabindex="-1"
      aria-hidden="true"
    />
    <p class="reader-drop-hint" aria-live="polite" data-reader-drop-hint>
      {#if uploading}
        Saving {uploadName || 'your file'} and opening it...
      {:else}
        Drop a file anywhere in this window, or use Open file, to read your own document. Reader opens {uploadFormats().map((type) => READER_UPLOAD_FORMATS.find((spec) => spec.mediaType === type)?.label).filter(Boolean).join(' and ')}.
      {/if}
    </p>
    {#if dropping}
      <div class="reader-drop-veil" data-reader-drop-veil aria-hidden="true">
        <span class="reader-drop-badge">
          {authenticated ? 'Drop to open in Reader' : 'Drop to open the sample (sign in to open your own files)'}
        </span>
      </div>
    {/if}
  </div>

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
        class="reader-button reader-open-file"
        data-reader-open-file
        title={authenticated ? 'Open a PDF or EPUB from your computer' : 'Sign in to open your own PDF or EPUB'}
        on:click={openFilePicker}
      >Open file</button>
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

  {#if uploading || uploadProgress > 0}
    <div class="reader-upload" data-reader-upload role="status" aria-live="polite">
      <div class="reader-upload-bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={Math.round(uploadProgress * 100)} data-reader-upload-progress>
        <i style={'width: ' + Math.round(uploadProgress * 100) + '%'}></i>
      </div>
      <span>{uploadProgress > 0 && uploadProgress < 1 ? Math.round(uploadProgress * 100) + '% of ' + uploadName : uploadStatus || 'Saving ' + uploadName}</span>
      {#if uploading}
        <button type="button" class="reader-button" data-reader-upload-cancel on:click={cancelUpload}>Cancel</button>
      {/if}
    </div>
  {/if}
  {#if uploadError}
    <div class="reader-alert reader-upload-alert" role="alert" data-reader-upload-error>
      <strong>Reader could not open that file</strong>
      <span>{uploadError}</span>
    </div>
  {/if}

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
          <p>Open a PDF or an EPUB from your computer or from Files, or read the sample below. Reader keeps your place once you are signed in.</p>
          <div class="reader-empty-actions">
            <button type="button" class="reader-button" data-reader-open-file-empty on:click={openFilePicker}>Open file</button>
            <button type="button" class="reader-button" data-reader-open-sample on:click={openSample}>Open the sample document</button>
          </div>
          {#if recentFiles.length}
            <div class="reader-recents" data-reader-recents>
              <span>Recently opened</span>
              {#each recentFiles as recent}
                <button type="button" class="reader-recent-item" data-reader-recent on:click={() => openRecent(recent)}>
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

  /* ---- Opening a file from disk ---- */
  .reader-open-file {
    font-weight: 600;
  }
  /* The drop target is one region over the window, in the reading surface's
     own palette: it is laid out where the reader's paper is, so it reads as
     part of the page rather than a floating system layer. */
  .reader-drop-target {
    position: relative;
    display: flex;
    justify-content: flex-end;
    padding: 0 18px 2px;
    background: var(--choir-surface-document, var(--choir-surface-app));
  }
  .reader-drop-hint {
    margin: 0;
    color: var(--reader-muted, var(--choir-text-muted));
    font-family: var(--choir-font-ui, inherit);
    font-size: 11.5px;
    letter-spacing: 0.01em;
  }
  .reader-drop-veil {
    position: absolute;
    inset: -46px 8px 8px;
    z-index: 5;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 2px dashed var(--choir-accent);
    border-radius: var(--choir-radius-panel, 16px);
    background: color-mix(in srgb, var(--reader-paper, var(--choir-surface-app)) 88%, transparent);
    box-shadow: 0 0 0 6px color-mix(in srgb, var(--choir-accent) 14%, transparent);
    pointer-events: none;
  }
  .reader-drop-badge {
    padding: 10px 18px;
    border: 1px solid var(--choir-border-strong);
    border-radius: var(--choir-radius-pill, 999px);
    color: var(--choir-text-on-accent);
    background: var(--choir-accent);
    font-family: var(--choir-font-ui, inherit);
    font-size: 13px;
    font-weight: 600;
  }
  .reader-upload {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 18px;
    border-bottom: 1px solid var(--choir-border);
    color: var(--choir-text-muted);
    background: var(--choir-surface-pane);
    font-family: var(--choir-font-ui, inherit);
    font-size: 12px;
  }
  .reader-upload-bar {
    flex: 1;
    height: 6px;
    overflow: hidden;
    border-radius: var(--choir-radius-pill, 999px);
    background: var(--choir-border);
  }
  .reader-upload-bar i {
    display: block;
    height: 100%;
    background: var(--choir-accent);
    transition: width var(--choir-duration-fast, 120ms) linear;
  }
  .reader-upload-alert {
    margin: 10px 18px 0;
  }
  .reader-empty-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }
</style>
