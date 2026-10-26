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

<div class="slides-shell" bind:this={containerEl} data-slides-app tabindex="0" on:keydown={handleKeydown}>
  <header class="topbar"><div class="brand"><span class="brand-mark" aria-hidden="true">S</span><div><p class="eyebrow">PRESENTATION</p><h1>{item?.title || source.title || 'Slides'}</h1></div></div>{#if slides.length}<div class="top-actions"><span class="deck-count">{slides.length} slides</span><button class="control-button primary" on:click={toggleFullscreen} aria-label={isFullscreen ? 'Exit presentation mode' : 'Present fullscreen'}>Present <span aria-hidden="true">⛶</span></button></div>{/if}</header>
  {#if loading}<div class="state-panel" role="status"><span class="spinner" aria-hidden="true"></span><h2>Preparing your presentation</h2><p>Loading slides…</p></div>
  {:else if error}<div class="state-panel error-panel" role="alert"><span class="state-icon" aria-hidden="true">!</span><h2>Couldn't open this presentation</h2><p>{error}</p><button class="control-button" on:click={loadContentItem}>Try again</button></div>
  {:else if slides.length === 0}<main class="welcome"><div class="welcome-copy"><span class="eyebrow">YOUR PRESENTATION SPACE</span><h2>Make room for the big picture.</h2><p>Open a PPTX, PDF, or HTML file from Files to present it here.</p></div><div class="recent-panel"><span class="eyebrow">CONTINUE WHERE YOU LEFT OFF</span><h3>Recent presentations</h3>{#if recentFiles.length}<ul class="recent-list">{#each recentFiles as entry}<li><button class="recent-item" on:click={() => openRecentFile(entry)}><span class="file-icon" aria-hidden="true">▤</span><span class="recent-title">{entry.title || entry.fileName || 'Untitled'}</span><span aria-hidden="true">→</span></button></li>{/each}</ul>{:else}<p class="muted">Your recently opened decks will appear here.</p>{/if}</div></main>
  {:else}<main class="workspace"><aside class="navigator" aria-label="Slide navigation"><div class="nav-heading"><div><span class="eyebrow">DECK</span><h2>Slides</h2></div><span class="deck-count">{slides.length}</span></div><ol class="slide-list">{#each slides as slide, i}<li><button class:active={i === currentSlide} class="slide-row" aria-current={i === currentSlide ? 'page' : undefined} aria-label="Go to slide {i + 1}: {slide.title || slide.texts?.[0] || 'Slide'}" on:click={() => goToSlide(i)}><span class="row-number">{String(i + 1).padStart(2, '0')}</span><span class="row-preview">{#if slide.type === 'image'}<img src={slide.src} alt="" />{:else if slide.type === 'pptx'}{#if slide.imageUrls?.[0]}<img src={slide.imageUrls[0]} alt="" />{:else}<span class="preview-text">{slide.texts?.[0] || 'Slide'}</span>{/if}{:else}<span class="preview-text">{slide.title || 'Slide'}</span>{/if}</span><span class="row-title">{slide.title || slide.texts?.[0] || 'Slide'}</span></button></li>{/each}</ol><p class="nav-hint">Use ← → to navigate</p></aside>
  <section class="presentation" aria-label="Presentation view"><div class="stage-wrap"><div class="stage" bind:this={stageEl} role="group" aria-label="Slide {currentSlide + 1} of {slides.length}">{#if slides[currentSlide]?.type === 'image'}<img class="slide-image" src={slides[currentSlide].src} alt="Slide {currentSlide + 1}" />{:else if slides[currentSlide]?.type === 'pptx'}<article class="slide-document" data-choir-content>{#each slides[currentSlide].imageUrls || [] as imgUrl}<img class="document-image" src={imgUrl} alt="Image on slide {currentSlide + 1}" />{/each}{#each slides[currentSlide].texts as text}<p>{text}</p>{/each}</article>{:else if slides[currentSlide]?.type === 'html'}<article class="slide-document slide-html" data-choir-content>{@html slides[currentSlide].html}</article>{/if}</div></div>
  <footer class="player-bar"><div class="player-status"><span class="live-dot" aria-hidden="true"></span><span>Slide <strong>{currentSlide + 1}</strong> of {slides.length}</span></div><div class="player-controls"><button class="icon-button" on:click={prevSlide} disabled={currentSlide === 0} aria-label="Previous slide" title="Previous slide (Left arrow)">←</button><span class="slide-position">{slideLabel}</span><button class="icon-button" on:click={nextSlide} disabled={currentSlide === slides.length - 1} aria-label="Next slide" title="Next slide (Right arrow)">→</button><span class="control-divider"></span><button class="icon-button" on:click={toggleFullscreen} aria-label={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'} title="Fullscreen (F)">⛶</button></div><span class="keyboard-help">← → navigate · Home / End jump · Esc exit</span></footer></section></main>{/if}
</div>
<style>
  .slides-shell { display:flex; flex-direction:column; height:100%; min-height:0; color:var(--choir-fg); background:var(--choir-bg); outline:none; }
  .slides-shell:focus-visible { outline:2px solid var(--choir-accent); outline-offset:-2px; }
  .topbar { min-height:72px; display:flex; align-items:center; justify-content:space-between; gap:1rem; padding:0 1.5rem; border-bottom:1px solid var(--choir-border); background:var(--choir-panel); }
  .brand,.top-actions,.player-status,.player-controls,.nav-heading { display:flex; align-items:center; }.brand { gap:.8rem; min-width:0; }.brand-mark { display:grid; place-items:center; width:38px; height:38px; border-radius:11px; color:var(--choir-accent); background:var(--choir-selected); font-weight:800; font-size:1.15rem; }.brand h1,.nav-heading h2 { margin:0; font-size:1rem; font-weight:650; }.brand h1 { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; max-width:42vw; }.eyebrow { margin:0 0 .2rem; color:var(--choir-muted); font-size:.64rem; font-weight:750; letter-spacing:.11em; }.top-actions { gap:1rem; }.deck-count { color:var(--choir-muted); font-size:.8rem; font-variant-numeric:tabular-nums; }
  .control-button,.icon-button { color:var(--choir-fg); border:1px solid var(--choir-border); background:var(--choir-panel); border-radius:8px; cursor:pointer; transition:background .15s,border-color .15s; }.control-button { padding:.55rem .85rem; font:inherit; font-size:.82rem; }.control-button.primary { background:var(--choir-accent); border-color:var(--choir-accent); color:var(--choir-bg); font-weight:700; }.control-button:hover,.icon-button:hover:not(:disabled) { background:var(--choir-selected); }.control-button:focus-visible,.icon-button:focus-visible,.slide-row:focus-visible,.recent-item:focus-visible { outline:2px solid var(--choir-accent); outline-offset:2px; }
  .workspace { display:grid; grid-template-columns:minmax(210px,260px) 1fr; flex:1; min-height:0; }.navigator { display:flex; flex-direction:column; min-height:0; padding:1.1rem .75rem .75rem; border-right:1px solid var(--choir-border); background:var(--choir-panel); }.nav-heading { justify-content:space-between; padding:0 .55rem .9rem; }.nav-heading .deck-count { padding:.2rem .48rem; border:1px solid var(--choir-border); border-radius:20px; }.slide-list { display:flex; flex-direction:column; gap:.3rem; margin:0; padding:0; list-style:none; overflow-y:auto; }.slide-row { display:grid; grid-template-columns:24px 72px minmax(0,1fr); align-items:center; gap:.55rem; width:100%; padding:.45rem; text-align:left; color:var(--choir-fg); background:transparent; border:1px solid transparent; border-radius:8px; cursor:pointer; }.slide-row:hover,.slide-row.active { background:var(--choir-selected); }.slide-row.active { border-color:var(--choir-accent); }.row-number { color:var(--choir-muted); font-size:.7rem; text-align:center; }.row-preview { display:grid; place-items:center; height:48px; overflow:hidden; border:1px solid var(--choir-border); border-radius:5px; background:var(--choir-bg); }.row-preview img { width:100%; height:100%; object-fit:contain; }.preview-text { overflow:hidden; padding:.25rem; color:var(--choir-muted); font-size:.58rem; text-align:center; }.row-title { overflow:hidden; color:var(--choir-fg); font-size:.74rem; text-overflow:ellipsis; white-space:nowrap; }.nav-hint { margin:auto 0 0; padding:.8rem .55rem 0; color:var(--choir-muted); font-size:.68rem; }
  .presentation { display:flex; flex-direction:column; min-width:0; min-height:0; }.stage-wrap { display:grid; flex:1; min-height:0; place-items:center; padding:clamp(1rem,3vw,3rem); }.stage { display:grid; place-items:center; width:100%; height:100%; min-height:180px; overflow:auto; }.slide-image { max-width:100%; max-height:100%; object-fit:contain; border:1px solid var(--choir-border); border-radius:8px; box-shadow:0 14px 48px rgb(0 0 0 / 18%); }.slide-document { box-sizing:border-box; width:min(100%,1000px); max-height:100%; overflow:auto; padding:clamp(1.5rem,5vw,4.5rem); color:var(--choir-fg); background:var(--choir-panel); border:1px solid var(--choir-border); border-radius:10px; box-shadow:0 14px 48px rgb(0 0 0 / 18%); font-size:clamp(1rem,2vw,1.5rem); line-height:1.55; }.slide-document p { margin:.35em 0; }.document-image { display:block; max-width:100%; max-height:40vh; margin:0 auto 1rem; object-fit:contain; }.slide-html :global(h1) { font-size:2em; line-height:1.15; }.slide-html :global(h2) { font-size:1.5em; }.slide-html :global(img) { max-width:100%; height:auto; }.player-bar { display:flex; align-items:center; justify-content:space-between; gap:1rem; min-height:56px; padding:.5rem 1.25rem; border-top:1px solid var(--choir-border); background:var(--choir-panel); }.player-status { gap:.55rem; color:var(--choir-muted); font-size:.78rem; }.player-status strong { color:var(--choir-fg); }.live-dot { width:7px; height:7px; border-radius:50%; background:var(--choir-accent); }.player-controls { gap:.5rem; }.icon-button { display:grid; place-items:center; width:34px; height:34px; font-size:1rem; }.icon-button:disabled { opacity:.42; cursor:not-allowed; }.slide-position { min-width:54px; color:var(--choir-muted); font-size:.75rem; text-align:center; }.control-divider { height:22px; border-left:1px solid var(--choir-border); }.keyboard-help { color:var(--choir-muted); font-size:.68rem; }.slides-shell:fullscreen { background:var(--choir-bg); }
  .state-panel,.welcome { display:grid; flex:1; place-content:center; justify-items:center; gap:1rem; padding:2rem; text-align:center; }.state-panel h2,.welcome h2 { margin:0; font-size:1.4rem; }.state-panel p,.welcome p,.muted { max-width:480px; margin:0; color:var(--choir-muted); line-height:1.6; }.spinner { width:26px; height:26px; border:3px solid var(--choir-border); border-top-color:var(--choir-accent); border-radius:50%; animation:spin .8s linear infinite; }.state-icon { display:grid; place-items:center; width:40px; height:40px; border:1px solid var(--choir-border); border-radius:50%; color:var(--choir-accent); }.welcome { grid-template-columns:minmax(240px,.9fr) minmax(300px,1.1fr); justify-items:stretch; align-items:center; max-width:980px; width:100%; margin:auto; text-align:left; }.welcome-copy h2 { max-width:380px; font-size:clamp(1.8rem,4vw,3rem); letter-spacing:-.035em; line-height:1.08; }.welcome-copy p { margin-top:.8rem; }.recent-panel { padding:1.2rem; border:1px solid var(--choir-border); border-radius:12px; background:var(--choir-panel); }.recent-panel h3 { margin:.2rem 0 .8rem; font-size:1rem; }.recent-list { display:grid; gap:.35rem; margin:0; padding:0; list-style:none; }.recent-item { display:flex; align-items:center; gap:.75rem; width:100%; padding:.65rem; color:var(--choir-fg); text-align:left; border:1px solid transparent; border-radius:8px; background:transparent; cursor:pointer; }.recent-item:hover { background:var(--choir-selected); }.file-icon { display:grid; place-items:center; width:34px; height:34px; border-radius:8px; color:var(--choir-accent); background:var(--choir-selected); }.recent-title { flex:1; overflow:hidden; font-size:.85rem; text-overflow:ellipsis; white-space:nowrap; }
  @keyframes spin { to { transform:rotate(360deg); } }
  @media (max-width:760px) { .topbar { min-height:62px; padding:0 .85rem; }.brand-mark { width:32px; height:32px; }.brand h1 { max-width:38vw; }.top-actions .deck-count { display:none; }.workspace { grid-template-columns:1fr; grid-template-rows:minmax(110px,30%) 1fr; }.navigator { grid-row:2; padding:.5rem; border-top:1px solid var(--choir-border); border-right:0; }.nav-heading { display:none; }.slide-list { flex-direction:row; }.slide-list li { flex:0 0 138px; }.slide-row { grid-template-columns:18px 48px minmax(0,1fr); gap:.35rem; padding:.3rem; }.row-preview { height:38px; }.nav-hint { display:none; }.presentation { grid-row:1; }.stage-wrap { padding:.6rem; }.player-bar { min-height:48px; padding:.4rem .6rem; }.keyboard-help { display:none; }.welcome { grid-template-columns:1fr; }.slides-shell:fullscreen .workspace { grid-template-columns:1fr; } }
  @media (prefers-reduced-motion:reduce) { *,*::before,*::after { animation-duration:.01ms !important; transition-duration:.01ms !important; } }
</style>
