<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    appTitle,
    clampNumber,
    formatTime,
    loadContextContentItem,
    resolveMediaSource,
  } from './media-utils.js';

  export let appContext = {};

  const kind = 'audio';
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

<section class="media-app audio-app" data-media-app data-media-kind="audio" data-audio-app>
  <header class="media-header">
    <div>
      <p>Audio</p>
      <h2 data-media-title>{source.title}</h2>
    </div>
  </header>

  {#if loading}
    <p class="media-status">Loading audio...</p>
  {:else if error}
    <p class="media-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <p class="media-status">No playable audio source is attached to this window.</p>
  {:else}
    <div class="media-stage audio-stage" data-media-stage data-audio-stage>
      <div class="media-player-card" data-media-player data-audio-player>
        <div class="media-player-title">{source.title}</div>
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
        src={source.displayUrl}
        preload="metadata"
        controls
        bind:this={mediaEl}
        on:loadedmetadata={() => { setPlaybackSpeed(); updateMediaState(); }}
        on:timeupdate={updateMediaState}
        on:play={updateMediaState}
        on:pause={updateMediaState}
        on:ended={updateMediaState}
        data-audio-element
      />
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
  .audio-stage {
    display: grid;
    min-height: 0;
    gap: 18px;
    align-content: center;
    justify-items: center;
    padding: 24px;
  }

  audio {
    width: min(100%, 760px);
  }

  @media (max-width: 720px) {
    .audio-stage {
      align-content: center;
      justify-items: stretch;
      padding: 14px;
    }
  }
</style>
