<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    appTitle,
    clampNumber,
    formatTime,
    loadContextContentItem,
    resolveMediaSource,
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

<section class="media-app video-app" data-media-app data-media-kind="video" data-video-app>
  <header class="media-header">
    <div>
      <p>Video</p>
      <h2 data-media-title>{source.title}</h2>
    </div>
  </header>

  {#if loading}
    <p class="media-status">Loading video...</p>
  {:else if error}
    <p class="media-error" role="alert">{error}</p>
  {:else if !source.displayUrl && !embedUrl}
    <p class="media-status">No playable video source is attached to this window.</p>
  {:else if embedUrl}
    <div class="media-stage video-embed-stage" data-media-stage data-video-embed-stage>
      <div class="media-toolbar embedded" data-media-toolbar data-video-toolbar>
        <span data-video-embedded-controls>Embedded player controls active</span>
      </div>
      <iframe
        title={source.title}
        src={embedUrl}
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
        allowfullscreen
        data-video-frame
      />
    </div>
  {:else}
    <div class="media-stage video-stage" data-media-stage data-video-stage>
      <video
        src={source.displayUrl}
        playsinline
        controls
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
  .video-stage,
  .video-embed-stage {
    display: flex;
    min-height: 0;
    flex-direction: column;
  }

  .video-stage video,
  .video-embed-stage iframe {
    flex: 1 1 auto;
    min-height: 260px;
  }
</style>
