<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    appTitle,
    clampNumber,
    formatTime,
    loadMediaPosition,
    loadContextContentItem,
    resolveMediaSource,
    saveMediaPosition,
    youtubeEmbedURL,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'video';
  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let playbackSpeed = 1;
  let mediaEl = null;
  let mediaCurrentTime = 0;
  let mediaDuration = 0;
  let mediaPlaying = false;
  let restoredPosition = false;

  $: source = resolveMediaSource(appContext, item, kind);
  $: embedUrl = source.mediaType === 'video/youtube' || /youtube\.com|youtu\.be/.test(source.sourceUrl)
    ? youtubeEmbedURL(source.sourceUrl)
    : '';
  $: mediaSeekPercent = mediaDuration > 0 ? Math.min(100, Math.max(0, (mediaCurrentTime / mediaDuration) * 100)) : 0;

  function setPlaybackSpeed() {
    if (mediaEl) mediaEl.playbackRate = Number(playbackSpeed) || 1;
  }

  function updateMediaState() {
    if (!mediaEl) return;
    mediaCurrentTime = Number.isFinite(mediaEl.currentTime) ? mediaEl.currentTime : 0;
    mediaDuration = Number.isFinite(mediaEl.duration) ? mediaEl.duration : 0;
    mediaPlaying = !mediaEl.paused;
    saveMediaPosition(kind, source, mediaCurrentTime, mediaDuration);
  }

  function restoreMediaPosition() {
    if (!mediaEl || restoredPosition) return;
    const stored = loadMediaPosition(kind, source);
    const duration = Number.isFinite(mediaEl.duration) ? mediaEl.duration : 0;
    if (stored > 0 && (!duration || stored < duration - 2)) {
      mediaEl.currentTime = stored;
    }
    restoredPosition = true;
    setPlaybackSpeed();
    updateMediaState();
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
    mediaEl.currentTime = clampNumber((mediaEl.currentTime || 0) + seconds, 0, duration || Number.MAX_SAFE_INTEGER);
    updateMediaState();
  }

  function seekMedia(event) {
    if (!mediaEl || !mediaDuration) return;
    const percent = clampNumber(Number(event.currentTarget.value) || 0, 0, 100);
    mediaEl.currentTime = (mediaDuration * percent) / 100;
    updateMediaState();
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

<section class="video-app" data-media-app data-media-kind="video" data-video-app>
  {#if loading}
    <p class="video-status">Loading video...</p>
  {:else if error}
    <p class="video-error" role="alert">{error}</p>
  {:else if !source.displayUrl && !embedUrl}
    <p class="video-status">No playable video source is attached to this window.</p>
  {:else if embedUrl}
    <div class="video-theater video-embed-stage" data-media-stage data-video-embed-stage>
      <details class="video-embed-controls" data-media-toolbar data-video-toolbar data-media-controls>
        <summary>Controls</summary>
        <span data-video-embedded-controls>Embedded player controls active</span>
      </details>
      <iframe
        title={source.title}
        src={embedUrl}
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
        allowfullscreen
        data-video-frame
      />
    </div>
  {:else}
    <div class="video-theater video-stage" data-media-stage data-video-stage>
      <video
        src={source.displayUrl}
        playsinline
        bind:this={mediaEl}
        on:loadedmetadata={restoreMediaPosition}
        on:timeupdate={updateMediaState}
        on:play={updateMediaState}
        on:pause={updateMediaState}
        on:ended={updateMediaState}
        data-video-player
      >
        <track kind="captions" />
      </video>
      <details class="video-controls" data-media-player data-video-controls data-media-controls>
        <summary>Controls</summary>
        <div class="video-control-panel">
          <div class="video-transport" data-media-transport>
            <button type="button" on:click={() => seekBy(-15)} data-media-skip-back data-video-skip-back>15s back</button>
            <button type="button" class="video-play" on:click={togglePlayback} data-media-play data-video-play>
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
          <div class="video-seek-row">
            <span data-media-current-time>{formatTime(mediaCurrentTime)}</span>
            <input type="range" min="0" max="100" step="0.1" value={mediaSeekPercent} on:input={seekMedia} data-media-seek data-video-seek />
            <span data-media-duration>{formatTime(mediaDuration)}</span>
          </div>
          <p class="video-position-note" data-media-position-status>Playback position is saved on this device.</p>
        </div>
      </details>
    </div>
  {/if}

  {#if !loading && !error}
    <details class="video-info">
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
  .video-app {
    position: relative;
    display: block;
    height: 100%;
    min-height: 0;
    color: #f8fbff;
    background: #02040a;
    overflow: hidden;
  }

  .video-theater {
    position: absolute;
    inset: 0;
    display: grid;
    width: 100%;
    height: 100%;
    min-height: 0;
    place-items: center;
    background:
      radial-gradient(circle at 50% 45%, rgba(55, 65, 81, 0.24), transparent 42%),
      #000;
    overflow: hidden;
  }

  .video-stage video,
  .video-embed-stage iframe {
    width: 100%;
    height: 100%;
    min-height: 240px;
    border: 0;
    object-fit: contain;
    background: #000;
  }

  .video-embed-controls {
    position: absolute;
    top: 12px;
    right: 12px;
    z-index: 2;
    border: 1px solid rgba(126, 180, 255, 0.26);
    border-radius: 12px;
    background: rgba(4, 9, 21, 0.74);
    color: #cbd5e1;
    font-size: 0.82rem;
    backdrop-filter: blur(12px);
  }

  .video-embed-controls summary,
  .video-controls summary {
    cursor: pointer;
    font-weight: 820;
    padding: 8px 10px;
  }

  .video-controls {
    position: absolute;
    right: 12px;
    bottom: 12px;
    left: 12px;
    border: 1px solid rgba(126, 180, 255, 0.28);
    border-radius: 14px;
    background: rgba(4, 9, 21, 0.82);
    box-shadow: 0 18px 60px rgba(0, 0, 0, 0.4);
    backdrop-filter: blur(12px);
  }

  .video-control-panel {
    display: grid;
    gap: 10px;
    padding: 0 12px 12px;
  }

  .video-embed-controls span {
    display: block;
    padding: 0 10px 8px;
  }

  .video-transport {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
  }

  .video-transport button,
  .video-transport select {
    min-height: 36px;
    border: 1px solid rgba(126, 180, 255, 0.34);
    border-radius: 999px;
    background: rgba(20, 38, 72, 0.86);
    color: #eef5ff;
    cursor: pointer;
    font: inherit;
    font-weight: 760;
    padding: 7px 12px;
  }

  .video-transport button:hover,
  .video-transport select:hover {
    background: rgba(39, 73, 128, 0.9);
  }

  .video-transport .video-play {
    min-width: 92px;
    background: rgba(45, 118, 255, 0.82);
  }

  .video-transport label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: #cbd5e1;
    font-size: 0.84rem;
  }

  .video-seek-row {
    display: grid;
    grid-template-columns: auto minmax(100px, 1fr) auto;
    align-items: center;
    gap: 10px;
    color: #cbd5e1;
    font-variant-numeric: tabular-nums;
  }

  .video-seek-row input[type='range'] {
    width: 100%;
  }

  .video-position-note {
    margin: 0;
    color: #a8adbd;
    font-size: 0.82rem;
    text-align: center;
  }

  .video-status,
  .video-error {
    place-self: center;
    margin: 0;
    border-radius: 14px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
    color: #a8adbd;
  }

  .video-error {
    color: #fecaca;
  }

  .video-info {
    position: absolute;
    top: 12px;
    left: 12px;
    z-index: 3;
    max-width: min(520px, calc(100% - 24px));
    border: 1px solid rgba(126, 180, 255, 0.22);
    border-radius: 12px;
    padding: 8px 10px;
    background: rgba(4, 9, 21, 0.82);
    color: #a8adbd;
  }

  .video-info summary {
    cursor: pointer;
    font-weight: 820;
  }

  .video-info h2 {
    margin: 10px 0;
    color: #f8fbff;
    font-size: 1rem;
    overflow-wrap: anywhere;
  }

  .video-info dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px 10px;
    margin: 0 0 4px;
  }

  .video-info dt {
    color: #dbeafe;
    font-weight: 760;
  }

  .video-info dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .video-info a {
    color: #bfdbfe;
  }

  @media (max-width: 720px) {
    .video-controls {
      right: 8px;
      bottom: 8px;
      left: 8px;
    }

    .video-transport {
      justify-content: stretch;
    }

    .video-transport button,
    .video-transport label {
      flex: 1 1 104px;
      justify-content: center;
    }

    .video-seek-row {
      grid-template-columns: auto minmax(80px, 1fr) auto;
    }

    .video-info,
    .video-embed-controls {
      top: 8px;
    }

    .video-info {
      left: 8px;
      max-width: calc(100% - 16px);
    }

    .video-embed-controls {
      right: 8px;
    }
  }
</style>
