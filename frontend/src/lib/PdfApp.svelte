<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    appTitle,
    loadContextContentItem,
    resolveMediaSource,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'pdf';
  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let pdfPage = 1;
  let pdfZoom = 'page-width';

  $: source = resolveMediaSource(appContext, item, kind);
  $: pdfReaderUrl = source.displayUrl
    ? `${source.displayUrl}${source.displayUrl.includes('#') ? '&' : '#'}page=${encodeURIComponent(String(pdfPage))}&zoom=${encodeURIComponent(pdfZoom)}`
    : '';

  function setPdfPage(nextPage) {
    pdfPage = Math.max(1, Math.floor(Number(nextPage) || 1));
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

<section class="media-app pdf-app" data-media-app data-media-kind="pdf" data-pdf-app>
  <header class="media-header">
    <div>
      <p>PDF</p>
      <h2 data-media-title>{source.title}</h2>
    </div>
  </header>

  {#if loading}
    <p class="media-status">Loading PDF...</p>
  {:else if error}
    <p class="media-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <p class="media-status">No readable PDF source is attached to this window.</p>
  {:else}
    <div class="media-toolbar" data-media-toolbar data-pdf-toolbar>
      <button type="button" on:click={() => setPdfPage(pdfPage - 1)} disabled={pdfPage <= 1} data-pdf-prev>Prev</button>
      <label>
        Page
        <input type="number" min="1" bind:value={pdfPage} on:change={(event) => setPdfPage(event.currentTarget.value)} data-pdf-page />
      </label>
      <button type="button" on:click={() => setPdfPage(pdfPage + 1)} data-pdf-next>Next</button>
      <label>
        Zoom
        <select bind:value={pdfZoom} data-pdf-zoom>
          <option value="page-width">Fit width</option>
          <option value="page-fit">Fit page</option>
          <option value="100">100%</option>
          <option value="150">150%</option>
          <option value="200">200%</option>
        </select>
      </label>
    </div>

    <div class="media-stage pdf-stage" data-media-stage data-pdf-stage>
      <object title={source.title} data={pdfReaderUrl} type="application/pdf" data-pdf-reader>
        <div class="reader-blocker">
          <strong>PDF preview is unavailable in this browser.</strong>
          <span>The PDF is still opened in the PDF app; use Source and details to inspect the file reference.</span>
        </div>
      </object>
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
  .pdf-stage {
    min-height: 0;
  }
</style>
