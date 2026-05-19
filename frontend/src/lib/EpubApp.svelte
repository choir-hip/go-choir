<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    appTitle,
    clampNumber,
    loadContextContentItem,
    resolveMediaSource,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'epub';
  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let readerFontSize = 18;
  let readerMeasure = 72;
  let readerProgress = 0;

  $: source = resolveMediaSource(appContext, item, kind);
  $: extractedText = item?.text_content || appContext.textContent || '';

  function updateReaderProgress(event) {
    const el = event.currentTarget;
    const scrollable = Math.max(0, el.scrollHeight - el.clientHeight);
    readerProgress = scrollable > 0 ? Math.round((el.scrollTop / scrollable) * 100) : 0;
  }

  function changeReaderSize(delta) {
    readerFontSize = clampNumber(readerFontSize + delta, 14, 28);
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

<section class="media-app epub-app" data-media-app data-media-kind="epub" data-epub-app>
  <header class="media-header">
    <div>
      <p>EPUB</p>
      <h2 data-media-title>{source.title}</h2>
    </div>
  </header>

  {#if loading}
    <p class="media-status">Loading EPUB...</p>
  {:else if error}
    <p class="media-error" role="alert">{error}</p>
  {:else if extractedText}
    <div class="media-toolbar" data-media-toolbar data-epub-toolbar>
      <button type="button" on:click={() => changeReaderSize(-1)} data-epub-font-smaller>-</button>
      <span data-epub-font-size>{readerFontSize}px</span>
      <button type="button" on:click={() => changeReaderSize(1)} data-epub-font-larger>+</button>
      <label>
        Width
        <select bind:value={readerMeasure} data-epub-width>
          <option value={58}>Narrow</option>
          <option value={72}>Comfort</option>
          <option value={88}>Wide</option>
        </select>
      </label>
      <span data-epub-progress>{readerProgress}%</span>
    </div>

    <div class="media-stage epub-scroll" data-media-stage data-epub-scroll on:scroll={updateReaderProgress}>
      <article
        class="epub-reader"
        data-epub-reader
        style={`--reader-font-size: ${readerFontSize}px; --reader-measure: ${readerMeasure}ch;`}
      >
        {extractedText}
      </article>
    </div>
  {:else}
    <div class="media-stage" data-media-stage>
      <div class="reader-blocker" data-epub-blocker>
        <strong>EPUB reader needs extracted text.</strong>
        <span>This app opens EPUB files as first-class artifacts, but it does not fake a reader until the archive has been extracted into readable text.</span>
        {#if source.filePath}<span>File: {source.filePath}</span>{/if}
      </div>
    </div>
  {/if}

  {#if !loading && !error}
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
  .epub-scroll {
    height: 100%;
    min-height: 0;
    overflow: auto;
  }

  .epub-reader {
    max-width: var(--reader-measure, 72ch);
    margin: 0 auto;
    padding: 32px;
    color: #e8eefc;
    white-space: pre-wrap;
    font-size: var(--reader-font-size, 18px);
    line-height: 1.62;
  }

  @media (max-width: 720px) {
    .epub-reader {
      padding: 22px;
    }
  }
</style>
