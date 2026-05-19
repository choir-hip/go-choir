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
  let restoredPosition = false;

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

<section class="audio-app" data-media-app data-media-kind="audio" data-audio-app>
  {#if loading}
    <p class="audio-status">Loading audio...</p>
  {:else if error}
    <p class="audio-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <p class="audio-status">No playable audio source is attached to this window.</p>
  {:else}
    <div class="audio-stage" data-media-stage data-audio-stage>
      <div class="audio-player" data-media-player data-audio-player>
        <div class="audio-art">
          <span>Audio</span>
        </div>
        <div class="audio-transport" data-media-transport>
          <button type="button" on:click={() => seekBy(-15)} data-media-skip-back data-audio-skip-back>15s back</button>
          <button type="button" class="audio-play" on:click={togglePlayback} data-media-play data-audio-play>
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
        <div class="audio-seek-row">
          <span data-media-current-time>{formatTime(mediaCurrentTime)}</span>
          <input type="range" min="0" max="100" step="0.1" value={mediaSeekPercent} on:input={seekMedia} data-media-seek data-audio-seek />
          <span data-media-duration>{formatTime(mediaDuration)}</span>
        </div>
        <p class="audio-position-note" data-media-position-status>Playback position is saved on this device.</p>
      </div>
      <audio
        src={source.displayUrl}
        preload="metadata"
        bind:this={mediaEl}
        on:loadedmetadata={restoreMediaPosition}
        on:timeupdate={updateMediaState}
        on:play={updateMediaState}
        on:pause={updateMediaState}
        on:ended={updateMediaState}
        data-audio-element
      />
    </div>

    <details class="audio-info">
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
  .audio-app {
    position: relative;
    display: block;
    height: 100%;
    min-height: 0;
    padding: 0;
    color: #f8fbff;
    background:
      radial-gradient(circle at 50% 26%, rgba(37, 99, 235, 0.24), transparent 38%),
      linear-gradient(150deg, #050814 0%, #071322 52%, #050814 100%);
    overflow: hidden;
  }

  .audio-stage {
    position: absolute;
    inset: 0;
    display: flex;
    min-height: 0;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 20px;
    padding: clamp(16px, 5vw, 48px);
  }

  .audio-player {
    display: grid;
    width: min(100%, 820px);
    gap: 16px;
    border: 1px solid rgba(126, 180, 255, 0.24);
    border-radius: 18px;
    padding: clamp(18px, 4vw, 34px);
    background: rgba(4, 9, 21, 0.72);
    box-shadow: 0 22px 80px rgba(0, 0, 0, 0.32);
  }

  .audio-art {
    display: grid;
    place-items: center;
    min-height: 150px;
    border: 1px solid rgba(99, 153, 255, 0.2);
    border-radius: 16px;
    background:
      linear-gradient(135deg, rgba(37, 99, 235, 0.28), rgba(20, 184, 166, 0.12)),
      rgba(255, 255, 255, 0.04);
    color: rgba(226, 232, 240, 0.72);
    font-size: clamp(1.4rem, 6vw, 3.2rem);
    font-weight: 860;
    letter-spacing: 0.18em;
    text-transform: uppercase;
  }

  .audio-transport {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 10px;
  }

  .audio-transport button,
  .audio-transport select {
    min-height: 40px;
    border: 1px solid rgba(126, 180, 255, 0.34);
    border-radius: 999px;
    background: rgba(20, 38, 72, 0.82);
    color: #eef5ff;
    cursor: pointer;
    font: inherit;
    font-weight: 760;
    padding: 8px 14px;
  }

  .audio-transport button:hover,
  .audio-transport select:hover {
    background: rgba(39, 73, 128, 0.88);
  }

  .audio-transport .audio-play {
    min-width: 94px;
    background: rgba(45, 118, 255, 0.82);
  }

  .audio-transport label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: #a8adbd;
    font-size: 0.86rem;
  }

  .audio-seek-row {
    display: grid;
    grid-template-columns: auto minmax(100px, 1fr) auto;
    align-items: center;
    gap: 10px;
    color: #cbd5e1;
    font-variant-numeric: tabular-nums;
  }

  .audio-seek-row input[type='range'] {
    width: 100%;
  }

  .audio-position-note {
    margin: 0;
    color: #a8adbd;
    font-size: 0.86rem;
    text-align: center;
  }

  .audio-stage audio {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
    pointer-events: none;
  }

  .audio-status,
  .audio-error {
    margin: auto;
    border-radius: 14px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
    color: #a8adbd;
  }

  .audio-error {
    color: #fecaca;
  }

  .audio-info {
    position: absolute;
    z-index: 2;
    right: 10px;
    bottom: 10px;
    left: 10px;
    border: 1px solid rgba(126, 180, 255, 0.22);
    border-radius: 12px;
    padding: 8px 10px;
    background: rgba(4, 9, 21, 0.82);
    color: #a8adbd;
    backdrop-filter: blur(12px);
  }

  .audio-info summary {
    cursor: pointer;
    font-weight: 820;
  }

  .audio-info h2 {
    margin: 10px 0;
    color: #f8fbff;
    font-size: 1rem;
    overflow-wrap: anywhere;
  }

  .audio-info dl {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 6px 10px;
    margin: 0 0 4px;
  }

  .audio-info dt {
    color: #dbeafe;
    font-weight: 760;
  }

  .audio-info dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .audio-info a {
    color: #bfdbfe;
  }

  @media (max-width: 720px) {
    .audio-stage {
      align-items: stretch;
      padding: 12px;
    }

    .audio-player {
      padding: 14px;
    }

    .audio-art {
      min-height: 96px;
    }

    .audio-transport {
      justify-content: stretch;
    }

    .audio-transport button,
    .audio-transport label {
      flex: 1 1 108px;
      justify-content: center;
    }

    .audio-seek-row {
      grid-template-columns: auto minmax(80px, 1fr) auto;
    }
  }
</style>
