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
  let zoom = 1;
  let imageFitMode = 'fit';
  let mediaCurrentTime = 0;
  let mediaDuration = 0;
  let mediaPlaying = false;
  let pdfPage = 1;
  let pdfZoom = 'page-width';
  let readerFontSize = 18;
  let readerMeasure = 72;
  let readerProgress = 0;

  $: sourceUrl = item?.source_url || appContext?.sourceUrl || '';
  $: filePath = item?.file_path || appContext?.filePath || '';
  $: mediaType = item?.media_type || appContext?.mediaType || '';
  $: title = item?.title || appContext?.windowTitle || appContext?.title || appTitle(kind);
  $: displayUrl = filePath ? apiFileURL(filePath) : sourceUrl;
  $: embedUrl = kind === 'video' && (mediaType === 'video/youtube' || /youtube\.com|youtu\.be/.test(sourceUrl))
    ? youtubeEmbedURL(sourceUrl)
    : '';
  $: mediaSeekPercent = mediaDuration > 0 ? Math.min(100, Math.max(0, (mediaCurrentTime / mediaDuration) * 100)) : 0;
  $: pdfReaderUrl = displayUrl ? `${displayUrl}#page=${encodeURIComponent(String(pdfPage))}&zoom=${encodeURIComponent(pdfZoom)}` : '';
  $: imageZoomLabel = `${Math.round(zoom * 100)}%`;
  $: imageWidth = imageFitMode === 'original' ? 'auto' : `${Math.round(zoom * 100)}%`;

  async function loadContentItem() {
    const contentId = appContext?.contentId || appContext?.content_id || '';
    const shouldImportSource = appContext?.importUrl === true || appContext?.forceImport === true;
    if (item || (!contentId && (!sourceUrl || !shouldImportSource))) return;
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

  function clampNumber(value, min, max) {
    return Math.min(max, Math.max(min, value));
  }

  function setImageFit(mode) {
    imageFitMode = mode;
    if (mode === 'fit') {
      zoom = 1;
    }
  }

  function zoomImage(delta) {
    imageFitMode = 'zoom';
    zoom = clampNumber(Math.round((zoom + delta) * 100) / 100, 0.25, 4);
  }

  function updateMediaState() {
    if (!mediaEl) return;
    mediaCurrentTime = Number.isFinite(mediaEl.currentTime) ? mediaEl.currentTime : 0;
    mediaDuration = Number.isFinite(mediaEl.duration) ? mediaEl.duration : 0;
    mediaPlaying = !mediaEl.paused;
  }

  async function togglePlayback() {
    if (!mediaEl) return;
    try {
      if (mediaEl.paused) {
        await mediaEl.play();
      } else {
        mediaEl.pause();
      }
      updateMediaState();
    } catch (err) {
      error = 'Playback needs a user gesture or a browser-supported source.';
    }
  }

  function seekBy(seconds) {
    if (!mediaEl) return;
    const duration = Number.isFinite(mediaEl.duration) ? mediaEl.duration : 0;
    const next = clampNumber((mediaEl.currentTime || 0) + seconds, 0, duration || Number.MAX_SAFE_INTEGER);
    mediaEl.currentTime = next;
    updateMediaState();
  }

  function seekMedia(event) {
    if (!mediaEl || !mediaDuration) return;
    const percent = clampNumber(Number(event.currentTarget.value) || 0, 0, 100);
    mediaEl.currentTime = (mediaDuration * percent) / 100;
    updateMediaState();
  }

  function formatTime(seconds) {
    if (!Number.isFinite(seconds) || seconds <= 0) return '0:00';
    const total = Math.floor(seconds);
    const minutes = Math.floor(total / 60);
    const remainder = String(total % 60).padStart(2, '0');
    return `${minutes}:${remainder}`;
  }

  function setPdfPage(nextPage) {
    pdfPage = Math.max(1, Math.floor(Number(nextPage) || 1));
  }

  function updateReaderProgress(event) {
    const el = event.currentTarget;
    const scrollable = Math.max(0, el.scrollHeight - el.clientHeight);
    readerProgress = scrollable > 0 ? Math.round((el.scrollTop / scrollable) * 100) : 0;
  }

  function changeReaderSize(delta) {
    readerFontSize = clampNumber(readerFontSize + delta, 14, 28);
  }

  onMount(loadContentItem);
</script>

<section class="media-app" data-media-app data-media-kind={kind}>
  <header class="media-header">
    <div>
      <p>{appTitle(kind)}</p>
      <h2 data-media-title>{title}</h2>
    </div>
  </header>

  {#if loading}
    <p class="status">Loading {appTitle(kind).toLowerCase()}...</p>
  {:else if error}
    <p class="error" role="alert">{error}</p>
  {:else if !displayUrl && !item?.text_content}
    <p class="status">No readable {appTitle(kind).toLowerCase()} source is attached to this window.</p>
  {:else}
    {#if kind === 'image' && displayUrl}
      <div class="media-toolbar" data-media-toolbar data-image-toolbar>
        <button type="button" class:selected={imageFitMode === 'fit'} on:click={() => setImageFit('fit')} data-image-fit>Fit</button>
        <button type="button" class:selected={imageFitMode === 'original'} on:click={() => setImageFit('original')} data-image-original>Original</button>
        <button type="button" on:click={() => zoomImage(-0.25)} data-image-zoom-out>-</button>
        <span data-image-zoom-level>{imageZoomLabel}</span>
        <button type="button" on:click={() => zoomImage(0.25)} data-image-zoom-in>+</button>
      </div>
    {:else if kind === 'pdf' && displayUrl}
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
    {:else if kind === 'epub' && item?.text_content}
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
    {/if}

    <div class="media-stage" data-media-stage>
      {#if kind === 'image' && displayUrl}
        <div class={`image-stage ${imageFitMode}`} data-image-stage>
          <img
            src={displayUrl}
            alt={title}
            data-image-viewer
            style={`width: ${imageWidth};`}
          />
        </div>
      {:else if kind === 'audio' && displayUrl}
        <div class="audio-stage" data-audio-player>
          <div class="media-player-card" data-media-player>
            <div class="media-player-title">{title}</div>
            <div class="media-transport" data-media-transport>
              <button type="button" on:click={() => seekBy(-15)} data-media-skip-back data-audio-skip-back>15s back</button>
              <button type="button" class="primary" on:click={togglePlayback} data-media-play data-audio-play>
                {mediaPlaying ? 'Pause' : 'Play'}
              </button>
              <button type="button" on:click={() => seekBy(30)} data-media-skip-forward data-audio-skip-forward>30s forward</button>
              <label>
                Speed
                <select bind:value={playbackSpeed} on:change={setPlaybackSpeed} data-media-speed data-audio-speed>
                  <option value={0.75}>0.75x</option>
                  <option value={1}>1x</option>
                  <option value={1.25}>1.25x</option>
                  <option value={1.5}>1.5x</option>
                  <option value={2}>2x</option>
                </select>
              </label>
            </div>
            <div class="seek-row">
              <span data-media-current-time>{formatTime(mediaCurrentTime)}</span>
              <input type="range" min="0" max="100" step="0.1" value={mediaSeekPercent} on:input={seekMedia} data-media-seek data-audio-seek />
              <span data-media-duration>{formatTime(mediaDuration)}</span>
            </div>
          </div>
          <audio
            src={displayUrl}
            preload="metadata"
            bind:this={mediaEl}
            on:loadedmetadata={() => { setPlaybackSpeed(); updateMediaState(); }}
            on:timeupdate={updateMediaState}
            on:play={updateMediaState}
            on:pause={updateMediaState}
            on:ended={updateMediaState}
            data-audio-element
          />
        </div>
      {:else if kind === 'video' && embedUrl}
        <div class="video-embed-stage" data-video-embed-stage>
          <div class="media-toolbar embedded" data-media-toolbar data-video-toolbar>
            <span data-video-embedded-controls>Embedded player controls active</span>
          </div>
          <iframe
            title={title}
            src={embedUrl}
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
            allowfullscreen
            data-video-frame
          />
        </div>
      {:else if kind === 'video' && displayUrl}
        <div class="video-stage" data-video-stage>
          <video
            src={displayUrl}
            playsinline
            bind:this={mediaEl}
            on:loadedmetadata={() => { setPlaybackSpeed(); updateMediaState(); }}
            on:timeupdate={updateMediaState}
            on:play={updateMediaState}
            on:pause={updateMediaState}
            on:ended={updateMediaState}
            data-video-player
          >
            <track kind="captions" />
          </video>
          <div class="media-player-card overlay" data-media-player data-video-controls>
            <div class="media-transport" data-media-transport>
              <button type="button" on:click={() => seekBy(-15)} data-media-skip-back data-video-skip-back>15s back</button>
              <button type="button" class="primary" on:click={togglePlayback} data-media-play data-video-play>
                {mediaPlaying ? 'Pause' : 'Play'}
              </button>
              <button type="button" on:click={() => seekBy(30)} data-media-skip-forward data-video-skip-forward>30s forward</button>
              <label>
                Speed
                <select bind:value={playbackSpeed} on:change={setPlaybackSpeed} data-media-speed data-video-speed>
                  <option value={0.75}>0.75x</option>
                  <option value={1}>1x</option>
                  <option value={1.25}>1.25x</option>
                  <option value={1.5}>1.5x</option>
                  <option value={2}>2x</option>
                </select>
              </label>
            </div>
            <div class="seek-row">
              <span data-media-current-time>{formatTime(mediaCurrentTime)}</span>
              <input type="range" min="0" max="100" step="0.1" value={mediaSeekPercent} on:input={seekMedia} data-media-seek data-video-seek />
              <span data-media-duration>{formatTime(mediaDuration)}</span>
            </div>
          </div>
        </div>
      {:else if kind === 'pdf' && displayUrl}
        <iframe title={title} src={pdfReaderUrl} data-pdf-reader />
      {:else if kind === 'epub' && item?.text_content}
        <div class="epub-scroll" data-epub-scroll on:scroll={updateReaderProgress}>
          <article
            class="epub-reader"
            data-epub-reader
            style={`--reader-font-size: ${readerFontSize}px; --reader-measure: ${readerMeasure}ch;`}
          >
            {item.text_content}
          </article>
        </div>
      {:else if kind === 'epub'}
        <div class="reader-blocker" data-epub-blocker>
          <strong>EPUB reader unavailable for this artifact.</strong>
          <span>The app can read extracted EPUB text with reader controls. This source has not been extracted yet, so no fake reader is shown.</span>
        </div>
      {:else}
        <div class="reader-blocker">
          <strong>Unsupported media source.</strong>
          <span>{mediaType || 'No media type'} {displayUrl || ''}</span>
        </div>
      {/if}
    </div>

    <details class="media-details">
      <summary>Source and details</summary>
      <dl>
        {#if sourceUrl}<dt>Source</dt><dd><a href={sourceUrl} target="_blank" rel="noreferrer" data-media-open-source>{sourceUrl}</a></dd>{/if}
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

  .media-toolbar,
  .media-player-card,
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

  .media-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    flex: 0 0 auto;
    padding: 8px;
  }

  .media-toolbar.embedded {
    border-radius: 0;
    border-width: 0 0 1px;
    background: rgba(9, 14, 26, 0.92);
  }

  .media-toolbar button,
  .media-transport button {
    min-height: 34px;
    border: 1px solid rgba(126, 180, 255, 0.32);
    border-radius: 9px;
    background: rgba(37, 64, 108, 0.72);
    color: #eef5ff;
    cursor: pointer;
    font-weight: 760;
    padding: 7px 10px;
  }

  .media-toolbar button:hover:not(:disabled),
  .media-transport button:hover:not(:disabled) {
    background: rgba(56, 96, 160, 0.82);
  }

  .media-toolbar button:disabled,
  .media-transport button:disabled {
    cursor: not-allowed;
    opacity: 0.52;
  }

  .media-toolbar button.selected,
  .media-transport button.primary {
    border-color: rgba(147, 197, 253, 0.72);
    background: rgba(30, 86, 170, 0.9);
  }

  .media-toolbar span,
  .media-toolbar label {
    color: var(--choir-muted, #a8adbd);
    font-size: 0.84rem;
  }

  .media-toolbar input[type='number'] {
    width: 4.8rem;
    border: 1px solid rgba(99, 153, 255, 0.34);
    border-radius: 8px;
    background: rgba(5, 10, 22, 0.72);
    color: #f8fbff;
    padding: 7px 8px;
  }

  .media-stage {
    flex: 1 1 auto;
    min-height: 0;
    border: 1px solid rgba(120, 135, 170, 0.24);
    border-radius: 16px;
    background: rgba(3, 7, 18, 0.76);
    overflow: auto;
  }

  .image-stage {
    min-height: 100%;
    display: grid;
    place-items: center;
    padding: 18px;
  }

  .image-stage img {
    max-width: none;
    height: auto;
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
    display: none;
    width: min(100%, 680px);
  }

  .video-stage,
  .video-embed-stage {
    min-height: 100%;
    display: flex;
    flex-direction: column;
  }

  .video-stage video,
  .video-embed-stage iframe {
    flex: 1 1 auto;
    min-height: 260px;
  }

  .media-player-card {
    width: min(100%, 760px);
    display: grid;
    gap: 12px;
    padding: 14px;
    box-shadow: 0 18px 45px rgba(0, 0, 0, 0.28);
  }

  .media-player-card.overlay {
    width: auto;
    margin: 0;
    border-width: 1px 0 0;
    border-radius: 0;
    background: rgba(8, 13, 25, 0.94);
    box-shadow: none;
  }

  .media-player-title {
    min-width: 0;
    color: #f8fbff;
    font-weight: 840;
    overflow-wrap: anywhere;
  }

  .media-transport {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .seek-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 10px;
    color: var(--choir-muted, #a8adbd);
    font-size: 0.82rem;
  }

  .seek-row input[type='range'] {
    width: 100%;
    min-width: 0;
    accent-color: #93c5fd;
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

  .media-details a {
    color: #bfdbfe;
    overflow-wrap: anywhere;
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

    .audio-stage {
      place-content: stretch;
      align-content: center;
      padding: 14px;
    }

    .media-player-card {
      width: 100%;
    }

    .seek-row {
      grid-template-columns: 1fr;
      gap: 6px;
    }

    .epub-reader {
      padding: 22px;
    }
  }
</style>
