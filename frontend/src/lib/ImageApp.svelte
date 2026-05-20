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

  const kind = 'image';
  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let zoom = 1;
  let imageFitMode = 'fit';
  let rotation = 0;
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: imageZoomLabel = `${Math.round(zoom * 100)}%`;
  $: imageWidth = imageFitMode === 'original' ? 'auto' : `${Math.round(zoom * 100)}%`;
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

  function setImageFit(mode) {
    imageFitMode = mode;
    if (mode === 'fit') zoom = 1;
  }

  function zoomImage(delta) {
    imageFitMode = 'zoom';
    zoom = clampNumber(Math.round((zoom + delta) * 100) / 100, 0.25, 4);
  }

  function rotateImage(delta) {
    rotation = (rotation + delta + 360) % 360;
  }

  function resetImageView() {
    imageFitMode = 'fit';
    zoom = 1;
    rotation = 0;
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
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
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

<section class="image-app" data-media-app data-media-kind="image" data-image-app>
  {#if loading}
    <p class="image-status">Loading image...</p>
  {:else if error}
    <p class="image-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <div class="image-empty" data-media-empty data-media-recent-empty>
      <p class="image-status">No readable image source is attached to this window.</p>
      {#if recentFiles.length}
        <div class="image-recent" data-media-recent-list>
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
    <div class="image-canvas {imageFitMode}" data-media-stage data-image-stage>
      <img src={source.displayUrl} alt={source.title} data-image-viewer style={`width: ${imageWidth}; transform: rotate(${rotation}deg);`} />
    </div>

    <details class="image-controls" data-media-toolbar data-image-toolbar data-media-controls>
      <summary aria-label="Image controls" title="Image controls"><span aria-hidden="true">...</span></summary>
      <div class="image-control-panel">
        <button type="button" class:selected={imageFitMode === 'fit'} on:click={() => setImageFit('fit')} data-image-fit>Fit</button>
        <button type="button" class:selected={imageFitMode === 'original'} on:click={() => setImageFit('original')} data-image-original>Original</button>
        <button type="button" on:click={() => zoomImage(-0.25)} data-image-zoom-out>-</button>
        <span data-image-zoom-level>{imageZoomLabel}</span>
        <button type="button" on:click={() => zoomImage(0.25)} data-image-zoom-in>+</button>
        <button type="button" on:click={() => rotateImage(-90)} data-image-rotate-left>Rotate left</button>
        <span data-image-rotation>{rotation}deg</span>
        <button type="button" on:click={() => rotateImage(90)} data-image-rotate-right>Rotate right</button>
        <button type="button" on:click={resetImageView} data-image-reset>Reset</button>
      </div>
    </details>

    <details class="image-info">
      <summary aria-label="Image info" title="Image info"><span aria-hidden="true">i</span></summary>
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
  .image-app {
    position: relative;
    display: grid;
    height: 100%;
    min-height: 0;
    background:
      linear-gradient(45deg, rgba(255, 255, 255, 0.035) 25%, transparent 25%),
      linear-gradient(-45deg, rgba(255, 255, 255, 0.035) 25%, transparent 25%),
      #050814;
    background-position: 0 0, 0 10px;
    background-size: 20px 20px;
    color: #f5f7ff;
    overflow: hidden;
  }

  .image-canvas {
    display: grid;
    height: 100%;
    min-height: 0;
    padding: 10px;
    place-items: center;
    overflow: auto;
  }

  .image-canvas img {
    height: auto;
    max-width: none;
    min-height: auto;
    transition: transform 120ms ease;
  }

  .image-canvas.fit img {
    max-width: 100%;
    max-height: 100%;
  }

  .image-canvas.original img {
    width: auto;
    max-width: none;
  }

  .image-controls {
    position: absolute;
    z-index: 2;
    top: 10px;
    left: 10px;
    width: max-content;
    max-width: min(520px, calc(100% - 20px));
    color: #cbd5e1;
  }

  .image-controls summary {
    display: grid;
    width: 36px;
    height: 36px;
    place-items: center;
    border: 1px solid rgba(99, 153, 255, 0.24);
    border-radius: 999px;
    background: rgba(5, 10, 22, 0.64);
    backdrop-filter: blur(12px);
    cursor: pointer;
    font-size: 0;
    font-weight: 820;
    list-style: none;
    padding: 0;
  }

  .image-controls summary::-webkit-details-marker {
    display: none;
  }

  .image-controls summary span {
    font-size: 1rem;
    line-height: 1;
  }

  .image-controls[open] {
    border: 1px solid rgba(99, 153, 255, 0.24);
    border-radius: 12px;
    background: rgba(5, 10, 22, 0.76);
    backdrop-filter: blur(12px);
  }

  .image-control-panel {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 7px;
    padding: 0 7px 7px;
  }

  .image-control-panel button {
    min-height: 32px;
    border: 1px solid rgba(126, 180, 255, 0.32);
    border-radius: 9px;
    background: rgba(37, 64, 108, 0.72);
    color: #eef5ff;
    cursor: pointer;
    font-weight: 760;
    padding: 6px 9px;
  }

  .image-control-panel button:hover,
  .image-control-panel button.selected {
    background: rgba(56, 96, 160, 0.86);
  }

  .image-control-panel span {
    color: #cbd5e1;
    font-size: 0.82rem;
    font-weight: 760;
  }

  .image-info {
    position: absolute;
    z-index: 2;
    right: 10px;
    bottom: 10px;
    width: max-content;
    max-width: min(520px, calc(100% - 20px));
    color: #a8adbd;
  }

  .image-info summary {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 999px;
    background: rgba(10, 15, 27, 0.64);
    backdrop-filter: blur(12px);
    cursor: pointer;
    color: #dbeafe;
    font-size: 0;
    font-weight: 800;
    list-style: none;
    margin-left: auto;
    padding: 0;
  }

  .image-info summary::-webkit-details-marker {
    display: none;
  }

  .image-info summary span {
    font-size: 0.95rem;
    line-height: 1;
  }

  .image-info[open] {
    left: 10px;
    width: auto;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 10px;
    padding: 7px 9px;
    background: rgba(10, 15, 27, 0.72);
    backdrop-filter: blur(12px);
  }

  .image-info h2 {
    margin: 10px 0;
    color: #f8fbff;
    font-size: 1rem;
    overflow-wrap: anywhere;
  }

  .image-info dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px 10px;
    margin: 0 0 4px;
  }

  .image-info dt {
    color: #dbeafe;
    font-weight: 760;
  }

  .image-info dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .image-info a {
    color: #bfdbfe;
  }

  .image-status,
  .image-error {
    align-self: center;
    justify-self: center;
    margin: 0;
    border-radius: 14px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
    color: #a8adbd;
  }

  .image-error {
    color: #ffd6d6;
  }

  .image-empty {
    display: grid;
    width: min(100% - 24px, 520px);
    place-self: center;
    gap: 12px;
  }

  .image-recent {
    display: grid;
    gap: 8px;
    border: 1px solid rgba(126, 180, 255, 0.18);
    border-radius: 14px;
    background: rgba(5, 10, 22, 0.68);
    padding: 12px;
  }

  .image-recent > span {
    color: #93c5fd;
    font-size: 0.74rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .image-recent button {
    display: grid;
    gap: 2px;
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 10px;
    background: rgba(255, 255, 255, 0.055);
    color: #e5eefc;
    cursor: pointer;
    padding: 9px 10px;
    text-align: left;
  }

  .image-recent button:hover,
  .image-recent button:focus-visible {
    border-color: rgba(96, 165, 250, 0.45);
    background: rgba(96, 165, 250, 0.12);
  }

  .image-recent strong,
  .image-recent small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .image-recent small {
    color: #94a3b8;
    font-size: 0.74rem;
  }
</style>
