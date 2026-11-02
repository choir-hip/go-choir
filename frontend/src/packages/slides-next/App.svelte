<script>
  // Slides (slides-next): a first-party package that replaces the old
  // SlidesApp to meet the Choir App Design Bar. One grid holds a filmstrip, a
  // 16:9 stage and a 44px player bar. There is no brand header, no slide
  // count outside "N / M", no keyboard-hint text and no Unicode icons.
  // Every state (loading, error, empty) answers in one calm panel, and
  // Present is the only primary action. Opening files stays with the static
  // registry and the reader package, so this package declares no "opens".
  // Parsing lives in deck.js; colors come only from --choir-* variables.
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
    addLiveEventListener,
    liveEventKind,
    liveEventPayload,
  } from '../../lib/app-sdk';
  import { fileExtension, parseHtml, parsePdf, parsePptx } from './deck.js';
  import Glyph from './Glyph.svelte';
  import './slides.css';

  export let appContext = {};
  export let windowId = '';
  export let authenticated = false;

  const kind = 'slides';
  const dispatch = createEventDispatcher();
  const FORMAT_LINES = {
    pdf: 'Reading the PDF.',
    pptx: 'Reading the PPTX presentation.',
    html: 'Reading the HTML deck.',
  };
  const REASONS = {
    notFound: 'The file was not found.',
    unreadable: 'The file could not be read.',
    unsupported: 'This file type is not supported yet.',
    other: 'The file could not be opened.',
  };

  let item = appContext?.contentItem || null;
  let loading = Boolean(
    appContext?.contentItem || appContext?.contentId || appContext?.filePath || appContext?.sourceUrl
  );
  let failure = null;
  let loadingFormat = '';
  let slides = [];
  let currentSlide = 0;
  let isFullscreen = false;
  let stageEl = null;
  let shellEl = null;
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';
  let loadedSourceKey = '';
  let objectUrls = [];

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: sourceIdentity = mediaSourceIdentity(source);
  $: current = slides[currentSlide] || null;
  $: if (authenticated && source.displayUrl && sourceIdentity && sourceIdentity !== rememberedIdentity) {
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

  function fail(reason, detail) {
    const err = new Error(detail);
    err.reason = reason;
    return err;
  }

  function releaseObjectUrls() {
    objectUrls.forEach((url) => URL.revokeObjectURL(url));
    objectUrls = [];
  }

  async function resolveImages(parsed) {
    for (const slide of parsed) {
      if (slide.images && slide.images.length) {
        slide.imageUrls = await Promise.all(
          slide.images.map((blobPromise) => blobPromise.then((blob) => URL.createObjectURL(blob)))
        );
        objectUrls = objectUrls.concat(slide.imageUrls);
      }
    }
  }

  async function parseResponse(res, ext) {
    if (ext === 'pdf') return parsePdf(await res.arrayBuffer());
    if (ext === 'pptx') return parsePptx(await res.arrayBuffer());
    return parseHtml(await res.text());
  }

  async function loadSlides() {
    if (!sourceKey || loadedSourceKey === sourceKey) return;
    loadedSourceKey = sourceKey;
    loading = true;
    failure = null;
    slides = [];
    currentSlide = 0;
    releaseObjectUrls();

    try {
      const ext = fileExtension(source.filePath || source.title || sourceKey);
      const supported = ['pdf', 'pptx', 'html', 'htm'].includes(ext);
      loadingFormat = ext === 'htm' ? 'html' : ext;
      if (!supported) throw fail('unsupported', `Unsupported file type: .${ext}`);
      const res = await fetch(sourceKey, sourceFetchOptions(sourceKey));
      if (!res.ok) {
        throw fail(res.status === 404 ? 'notFound' : 'unreadable', `Source fetch failed (${res.status})`);
      }
      const parsed = await parseResponse(res, ext);
      await resolveImages(parsed);
      slides = parsed;
      currentSlide = 0;
    } catch (err) {
      failure = {
        reason: REASONS[err?.reason] || REASONS.other,
        detail: err?.message || 'Failed to load slides',
      };
    } finally {
      loading = false;
    }
  }

  async function loadContentItem() {
    loading = true;
    failure = null;
    const result = await loadContextContentItem(effectiveContext, item, appTitle(kind));
    if (result.authRequired) {
      loading = false;
      dispatch('authexpired');
      return;
    }
    if (result.error) {
      loading = false;
      failure = { reason: REASONS.other, detail: result.error };
      return;
    }
    if (result.item) item = result.item;
    await tick();
    if (!sourceKey) {
      loading = false;
      return;
    }
    await loadSlides();
  }

  function retry() {
    loadedSourceKey = '';
    void loadContentItem();
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    failure = null;
    slides = [];
    currentSlide = 0;
    loadedSourceKey = '';
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    await loadContentItem();
  }

  function openFiles() {
    dispatch('launchapp', { appId: 'files', appName: 'Files' });
  }

  function goToSlide(index) {
    if (!slides.length) return;
    currentSlide = clampNumber(index, 0, slides.length - 1);
  }

  function nextSlide() {
    goToSlide(currentSlide + 1);
  }

  function prevSlide() {
    goToSlide(currentSlide - 1);
  }

  function toggleFullscreen() {
    if (!shellEl) return;
    if (!document.fullscreenElement) {
      shellEl.requestFullscreen?.().catch(() => {});
    } else {
      document.exitFullscreen?.();
    }
  }

  function handleFullscreenChange() {
    isFullscreen = document.fullscreenElement === shellEl;
  }

  function handleKeydown(event) {
    if (event.target instanceof HTMLButtonElement && event.key === ' ') return;
    if (event.key === 'f' || event.key === 'F') {
      event.preventDefault();
      toggleFullscreen();
    }
    if (!slides.length) return;
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
    }
  }

  function handleStageClick(event) {
    const rect = stageEl?.getBoundingClientRect();
    if (!rect) return;
    if (event.clientX - rect.left > rect.width / 2) nextSlide();
    else prevSlide();
  }

  function recentName(entry) {
    return entry.title || entry.fileName || 'Untitled';
  }

  function recentMeta(entry) {
    const path = String(entry.filePath || '');
    const cut = path.lastIndexOf('/');
    if (cut > 0) return path.slice(0, cut);
    const opened = entry.openedAt || entry.opened_at;
    if (!opened) return '';
    const date = new Date(opened);
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleDateString();
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
      releaseObjectUrls();
    };
  });
</script>

<div
  class="slides-shell"
  bind:this={shellEl}
  data-slides-next-app
  tabindex="0"
  on:keydown={handleKeydown}
>
  {#if loading}
    <div class="state-panel" role="status" data-loading-state>
      <p class="state-line">Opening the presentation...</p>
      {#if FORMAT_LINES[loadingFormat]}
        <p class="state-detail">{FORMAT_LINES[loadingFormat]}</p>
      {/if}
    </div>
  {:else if failure}
    <div class="state-panel" role="alert" data-error-state>
      <h2 class="state-title">This presentation could not be opened</h2>
      <p class="state-reason" data-error-reason>{failure.reason}</p>
      <p class="state-detail">{failure.detail}</p>
      <button type="button" class="text-button" data-retry on:click={retry}>Try again</button>
    </div>
  {:else if slides.length === 0}
    <div class="state-panel empty-panel" data-empty-state>
      <h2 class="state-title">Open a presentation</h2>
      <p class="state-copy">PDF, PPTX or HTML.</p>
      <button type="button" class="text-button" data-open-files on:click={openFiles}>Open from Files</button>
      {#if recentFiles.length}
        <ul class="recent-list">
          {#each recentFiles as entry}
            <li>
              <button type="button" class="recent-row" data-recent-item on:click={() => openRecentFile(entry)}>
                <Glyph name="file" />
                <span class="recent-name">{recentName(entry)}</span>
                <span class="recent-meta">{recentMeta(entry)}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {:else}
    <div class="workspace" data-slides-workspace>
      <aside class="filmstrip" data-filmstrip>
        <ol class="filmstrip-list">
          {#each slides as slide, i}
            <li>
              <button
                type="button"
                class="filmstrip-item"
                class:is-active={i === currentSlide}
                data-filmstrip-item
                aria-current={i === currentSlide ? 'page' : undefined}
                aria-label="Slide {i + 1} of {slides.length}"
                on:click={() => goToSlide(i)}
              >
                <span class="filmstrip-number">{i + 1}</span>
                <span class="filmstrip-thumb">
                  {#if slide.type === 'image'}
                    <img src={slide.src} alt="" />
                  {:else if slide.type === 'pptx'}
                    <span class="slide-document thumb-document" aria-hidden="true">
                      {#each slide.imageUrls || [] as url}
                        <img class="slide-image" src={url} alt="" />
                      {/each}
                      {#each slide.texts || [] as text, t}
                        <span class={t === 0 ? 'slide-title' : 'slide-body'}>{text}</span>
                      {/each}
                    </span>
                  {:else if slide.type === 'html'}
                    <span class="slide-document slide-html thumb-document" aria-hidden="true">
                      {@html slide.html}
                    </span>
                  {/if}
                </span>
              </button>
            </li>
          {/each}
        </ol>
      </aside>

      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <!-- svelte-ignore a11y-no-static-element-interactions -->
      <div class="stage" bind:this={stageEl} data-slides-stage on:click={handleStageClick}>
        <div class="slide-frame" data-slide-frame>
          {#if current?.type === 'image'}
            <img class="slide-page" data-slide-page src={current.src} alt="Slide {currentSlide + 1}" />
          {:else if current?.type === 'pptx'}
            <article class="slide-document" data-slide-document data-choir-content>
              {#each current.imageUrls || [] as url}
                <img class="slide-image" src={url} alt="" />
              {/each}
              {#each current.texts as text, t}
                <p class={t === 0 ? 'slide-title' : 'slide-body'}>{text}</p>
              {/each}
            </article>
          {:else if current?.type === 'html'}
            <article class="slide-document slide-html" data-slide-document data-choir-content>
              {@html current.html}
            </article>
          {/if}
        </div>
      </div>

      <div class="player-bar" data-player-bar>
        <span class="slide-position" data-slide-position>{currentSlide + 1} / {slides.length}</span>
        <div class="player-controls">
          <button
            type="button"
            class="icon-button"
            data-prev
            aria-label="Previous slide"
            title="Previous slide (Left arrow)"
            disabled={currentSlide === 0}
            on:click={prevSlide}
          >
            <Glyph name="chevron-left" />
          </button>
          <button
            type="button"
            class="icon-button"
            data-next
            aria-label="Next slide"
            title="Next slide (Right arrow)"
            disabled={currentSlide === slides.length - 1}
            on:click={nextSlide}
          >
            <Glyph name="chevron-right" />
          </button>
        </div>
        <button
          type="button"
          class="present-button"
          data-present
          aria-label={isFullscreen ? 'Exit fullscreen' : 'Present fullscreen'}
          title="Present fullscreen (F)"
          on:click={toggleFullscreen}
        >
          <Glyph name={isFullscreen ? 'shrink' : 'expand'} />
          <span>{isFullscreen ? 'Exit' : 'Present'}</span>
        </button>
      </div>
    </div>
  {/if}
</div>
