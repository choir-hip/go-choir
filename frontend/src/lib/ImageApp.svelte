<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    appTitle,
    clampNumber,
    loadContextContentItem,
    resolveMediaSource,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'image';
  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let zoom = 1;
  let imageFitMode = 'fit';
  let rotation = 0;

  $: source = resolveMediaSource(appContext, item, kind);
  $: imageZoomLabel = `${Math.round(zoom * 100)}%`;
  $: imageWidth = imageFitMode === 'original' ? 'auto' : `${Math.round(zoom * 100)}%`;

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
  }

  onMount(loadContentItem);
</script>

<section class="image-app" data-media-app data-media-kind="image" data-image-app>
  {#if loading}
    <p class="image-status">Loading image...</p>
  {:else if error}
    <p class="image-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <p class="image-status">No readable image source is attached to this window.</p>
  {:else}
    <div class="image-controls" data-media-toolbar data-image-toolbar>
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

    <div class="image-canvas {imageFitMode}" data-media-stage data-image-stage>
      <img src={source.displayUrl} alt={source.title} data-image-viewer style={`width: ${imageWidth}; transform: rotate(${rotation}deg);`} />
    </div>

    <details class="image-info">
      <summary>Info</summary>
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
    right: 10px;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 7px;
    border: 1px solid rgba(99, 153, 255, 0.24);
    border-radius: 12px;
    padding: 7px;
    background: rgba(5, 10, 22, 0.76);
    backdrop-filter: blur(12px);
  }

  .image-controls button {
    min-height: 32px;
    border: 1px solid rgba(126, 180, 255, 0.32);
    border-radius: 9px;
    background: rgba(37, 64, 108, 0.72);
    color: #eef5ff;
    cursor: pointer;
    font-weight: 760;
    padding: 6px 9px;
  }

  .image-controls button:hover,
  .image-controls button.selected {
    background: rgba(56, 96, 160, 0.86);
  }

  .image-controls span {
    color: #cbd5e1;
    font-size: 0.82rem;
    font-weight: 760;
  }

  .image-info {
    position: absolute;
    z-index: 2;
    right: 10px;
    bottom: 10px;
    left: 10px;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 10px;
    padding: 7px 9px;
    background: rgba(10, 15, 27, 0.72);
    color: #a8adbd;
    backdrop-filter: blur(12px);
  }

  .image-info summary {
    cursor: pointer;
    font-weight: 800;
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
</style>
