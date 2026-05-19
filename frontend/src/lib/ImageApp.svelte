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

<section class="media-app image-app" data-media-app data-media-kind="image" data-image-app>
  <header class="media-header">
    <div>
      <p>Image</p>
      <h2 data-media-title>{source.title}</h2>
    </div>
  </header>

  {#if loading}
    <p class="media-status">Loading image...</p>
  {:else if error}
    <p class="media-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <p class="media-status">No readable image source is attached to this window.</p>
  {:else}
    <div class="media-toolbar" data-media-toolbar data-image-toolbar>
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

    <div class="media-stage image-stage {imageFitMode}" data-media-stage data-image-stage>
      <img src={source.displayUrl} alt={source.title} data-image-viewer style={`width: ${imageWidth}; transform: rotate(${rotation}deg);`} />
    </div>

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
  .image-stage {
    display: grid;
    min-height: 0;
    padding: 18px;
    place-items: center;
  }

  .image-stage img {
    height: auto;
    max-width: none;
    min-height: auto;
  }

  .image-stage.fit img {
    max-width: 100%;
    max-height: calc(100vh - 220px);
  }

  .image-stage.original img {
    width: auto;
    max-width: none;
  }
</style>
