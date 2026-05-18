<script>
  import { onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';

  export let appContext = {};
  export let kind = 'file';

  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let playbackSpeed = 1;
  let mediaEl = null;

  $: sourceUrl = item?.source_url || appContext?.sourceUrl || '';
  $: filePath = item?.file_path || appContext?.filePath || '';
  $: mediaType = item?.media_type || appContext?.mediaType || '';
  $: title = item?.title || appContext?.windowTitle || appContext?.title || appTitle(kind);
  $: displayUrl = filePath ? apiFileURL(filePath) : sourceUrl;
  $: embedUrl = kind === 'video' && (mediaType === 'video/youtube' || /youtube\.com|youtu\.be/.test(sourceUrl))
    ? youtubeEmbedURL(sourceUrl)
    : '';

  async function loadContentItem() {
    const contentId = appContext?.contentId || appContext?.content_id || '';
    if (item || (!contentId && !sourceUrl)) return;
    loading = true;
    error = '';
    try {
      const res = contentId
        ? await fetchWithRenewal(`/api/content/items/${encodeURIComponent(contentId)}`)
        : await fetchWithRenewal('/api/content/import-url', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: sourceUrl, query: title || sourceUrl }),
          });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        error = body.error || `${appTitle(kind)} load failed (${res.status})`;
        return;
      }
      item = await res.json();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = `${appTitle(kind)} load failed`;
    } finally {
      loading = false;
    }
  }

  function appTitle(appKind) {
    if (appKind === 'pdf') return 'PDF';
    if (appKind === 'epub') return 'EPUB';
    return `${appKind.slice(0, 1).toUpperCase()}${appKind.slice(1)}`;
  }

  function apiFileURL(path) {
    return '/api/files/' + String(path || '').split('/').map(encodeURIComponent).join('/');
  }

  function youtubeEmbedURL(raw) {
    try {
      const url = new URL(raw);
      if (url.hostname === 'youtu.be') {
        const videoId = url.pathname.startsWith('/') ? url.pathname.slice(1) : url.pathname;
        return `https://www.youtube.com/embed/${encodeURIComponent(videoId)}`;
      }
      const id = url.searchParams.get('v');
      if (id) return `https://www.youtube.com/embed/${encodeURIComponent(id)}`;
    } catch (err) {
      return '';
    }
    return '';
  }

  function setPlaybackSpeed() {
    if (mediaEl) mediaEl.playbackRate = Number(playbackSpeed) || 1;
  }

  onMount(loadContentItem);
</script>

<section class="media-app" data-media-app data-media-kind={kind}>
  <header class="media-header">
    <div>
      <p>{appTitle(kind)}</p>
      <h2>{title}</h2>
    </div>
    {#if sourceUrl}
      <a class="source-link" href={sourceUrl} target="_blank" rel="noreferrer" data-media-open-source>Source</a>
    {/if}
  </header>

  {#if loading}
    <p class="status">Loading {appTitle(kind).toLowerCase()}...</p>
  {:else if error}
    <p class="error" role="alert">{error}</p>
  {:else if !displayUrl && !item?.text_content}
    <p class="status">No readable {appTitle(kind).toLowerCase()} source is attached to this window.</p>
  {:else}
    <div class="media-stage" data-media-stage>
      {#if kind === 'image' && displayUrl}
        <img src={displayUrl} alt={title} data-image-viewer />
      {:else if kind === 'audio' && displayUrl}
        <div class="audio-stage" data-audio-player>
          <audio
            src={displayUrl}
            controls
            preload="metadata"
            bind:this={mediaEl}
            on:loadedmetadata={setPlaybackSpeed}
            data-audio-element
          />
          <label>
            Speed
            <select bind:value={playbackSpeed} on:change={setPlaybackSpeed} data-audio-speed>
              <option value={0.75}>0.75x</option>
              <option value={1}>1x</option>
              <option value={1.25}>1.25x</option>
              <option value={1.5}>1.5x</option>
              <option value={2}>2x</option>
            </select>
          </label>
        </div>
      {:else if kind === 'video' && embedUrl}
        <iframe
          title={title}
          src={embedUrl}
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowfullscreen
          data-video-frame
        />
      {:else if kind === 'video' && displayUrl}
        <video src={displayUrl} controls bind:this={mediaEl} data-video-player>
          <track kind="captions" />
        </video>
      {:else if kind === 'pdf' && displayUrl}
        <iframe title={title} src={displayUrl} data-pdf-reader />
      {:else if kind === 'epub' && item?.text_content}
        <article class="epub-reader" data-epub-reader>{item.text_content}</article>
      {:else if kind === 'epub'}
        <div class="reader-blocker" data-epub-blocker>
          <strong>EPUB reader unavailable for this artifact.</strong>
          <span>The source is registered, but this build does not yet extract and paginate EPUB files in-browser.</span>
        </div>
      {:else}
        <div class="reader-blocker">
          <strong>Unsupported media source.</strong>
          <span>{mediaType || 'No media type'} {displayUrl || ''}</span>
        </div>
      {/if}
    </div>

    <details class="media-details">
      <summary>Details</summary>
      <dl>
        {#if mediaType}<dt>Type</dt><dd>{mediaType}</dd>{/if}
        {#if displayUrl}<dt>Reference</dt><dd>{displayUrl}</dd>{/if}
        {#if item?.content_hash}<dt>Hash</dt><dd>{item.content_hash}</dd>{/if}
      </dl>
    </details>
  {/if}
</section>

<style>
  .media-app {
    display: flex;
    flex-direction: column;
    gap: 14px;
    height: 100%;
    min-height: 0;
    padding: 16px;
    color: var(--choir-fg, #f5f7ff);
    background: linear-gradient(180deg, rgba(11, 18, 32, 0.98), rgba(6, 8, 16, 0.98));
    overflow: hidden;
  }

  .media-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    flex: 0 0 auto;
  }

  .media-header p {
    margin: 0 0 4px;
    color: var(--choir-muted, #a8adbd);
    font-size: 0.76rem;
    font-weight: 850;
    text-transform: uppercase;
  }

  .media-header h2 {
    margin: 0;
    font-size: 1.34rem;
    overflow-wrap: anywhere;
  }

  .source-link,
  select {
    border: 1px solid rgba(99, 153, 255, 0.42);
    border-radius: 10px;
    color: #ecf3ff;
    background: rgba(18, 32, 56, 0.82);
  }

  .source-link {
    padding: 7px 10px;
    text-decoration: none;
    flex: 0 0 auto;
  }

  .media-stage {
    flex: 1 1 auto;
    min-height: 0;
    border: 1px solid rgba(120, 135, 170, 0.24);
    border-radius: 16px;
    background: rgba(3, 7, 18, 0.76);
    overflow: auto;
  }

  img,
  video,
  iframe {
    display: block;
    width: 100%;
    height: 100%;
    min-height: 260px;
    border: 0;
    object-fit: contain;
  }

  .audio-stage {
    min-height: 100%;
    display: grid;
    place-content: center;
    gap: 18px;
    padding: 24px;
  }

  audio {
    width: min(100%, 680px);
  }

  label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: var(--choir-muted, #a8adbd);
  }

  select {
    padding: 7px 9px;
  }

  .epub-reader {
    max-width: 72ch;
    margin: 0 auto;
    padding: 32px;
    color: #e8eefc;
    white-space: pre-wrap;
    line-height: 1.58;
  }

  .reader-blocker,
  .status,
  .error {
    margin: 0;
    border-radius: 14px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
  }

  .reader-blocker {
    display: grid;
    gap: 6px;
    margin: 18px;
    color: #dce6ff;
  }

  .reader-blocker span,
  .status,
  .media-details,
  .media-details dd {
    color: var(--choir-muted, #a8adbd);
  }

  .error {
    color: #ffd6d6;
  }

  .media-details {
    flex: 0 0 auto;
    border: 1px solid rgba(120, 135, 170, 0.2);
    border-radius: 12px;
    padding: 8px 10px;
    background: rgba(10, 15, 27, 0.68);
  }

  .media-details summary {
    cursor: pointer;
    font-weight: 800;
  }

  dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px 10px;
    margin: 10px 0 0;
  }

  dt {
    color: #dbeafe;
    font-weight: 760;
  }

  dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  @media (max-width: 720px) {
    .media-app {
      padding: 12px;
    }

    .media-header h2 {
      font-size: 1.15rem;
    }
  }
</style>
