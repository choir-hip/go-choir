<script>
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import {
    appTitle,
    clampNumber,
    formatTime,
    loadRecentMedia,
    loadMediaPosition,
    loadContextContentItem,
    mediaSourceIdentity,
    recentMediaAppContext,
    rememberRecentMedia,
    resolveMediaSource,
    saveMediaPosition,
  } from './media-utils.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let appContext = {};
  export let windowId = '';

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
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';
  let removeLiveListener = () => {};

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: mediaSeekPercent = mediaDuration > 0 ? Math.min(100, Math.max(0, (mediaCurrentTime / mediaDuration) * 100)) : 0;
  $: sourceIdentity = mediaSourceIdentity(source);
  $: if (source.displayUrl && sourceIdentity && sourceIdentity !== rememberedIdentity) {
    void rememberCurrentSource();
  }

  async function refreshRecentFiles() {
    recentFiles = await loadRecentMedia(kind);
  }

  async function rememberCurrentSource() {
    rememberedIdentity = sourceIdentity;
    if (await rememberRecentMedia(kind, source)) {
      await refreshRecentFiles();
    }
  }

  function setPlaybackSpeed() {
    if (mediaEl) mediaEl.playbackRate = Number(playbackSpeed) || 1;
  }

  function updateMediaState() {
    if (!mediaEl) return;
    mediaCurrentTime = Number.isFinite(mediaEl.currentTime) ? mediaEl.currentTime : 0;
    mediaDuration = Number.isFinite(mediaEl.duration) ? mediaEl.duration : 0;
    mediaPlaying = !mediaEl.paused;
    saveMediaPosition(kind, { ...source, playbackRate: playbackSpeed }, mediaCurrentTime, mediaDuration);
  }

  async function restoreMediaPosition() {
    if (!mediaEl || restoredPosition) return;
    const stored = await loadMediaPosition(kind, source);
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
    const result = await loadContextContentItem(effectiveContext, item, appTitle(kind));
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

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
    mediaCurrentTime = 0;
    mediaDuration = 0;
    mediaPlaying = false;
    restoredPosition = false;
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    await loadContentItem();
  }

  onMount(() => {
    void refreshRecentFiles();
    void loadContentItem();
    removeLiveListener = addLiveEventListener((message) => {
      if (liveEventKind(message) === 'media.recent.updated') {
        if (liveEventPayload(message).kind === kind) void refreshRecentFiles();
        return;
      }
      if (liveEventKind(message) !== 'media.progress.updated') return;
      const payload = liveEventPayload(message);
      if (payload.kind !== kind || payload.identity !== sourceIdentity || mediaPlaying) return;
      mediaCurrentTime = Number(payload.current_time) || mediaCurrentTime;
      mediaDuration = Number(payload.duration) || mediaDuration;
      if (mediaEl && Math.abs((mediaEl.currentTime || 0) - mediaCurrentTime) > 3) {
        mediaEl.currentTime = mediaCurrentTime;
      }
    });
  });

  onDestroy(() => {
    removeLiveListener();
  });
</script>

<section class="audio-app" data-media-app data-media-kind="audio" data-audio-app>
  {#if loading}
    <p class="audio-status">Loading audio...</p>
  {:else if error}
    <p class="audio-error" role="alert">{error}</p>
  {:else if !source.displayUrl}
    <div class="audio-empty" data-media-empty data-media-recent-empty>
      <p class="audio-status">No playable audio source is attached to this window.</p>
      {#if recentFiles.length}
        <div class="audio-recent" data-media-recent-list>
          <span>Recently opened</span>
          {#each recentFiles as recent}
            <button type="button" data-media-recent-item on:click={() => openRecentFile(recent)}>
              <strong>{recent.title}</strong>
              <small>{recent.filePath || recent.sourceUrl}</small>
            </button>
          {/each}
        </div>
      {/if}
    </div>
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
        <p class="audio-position-note" data-media-position-status>Playback position syncs across your devices.</p>
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
      <summary aria-label="Audio info" title="Audio info"><span aria-hidden="true">i</span></summary>
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
    color: var(--choir-text-accent);
    background:
      radial-gradient(circle at 50% 26%, var(--choir-state-selected), transparent 38%),
      linear-gradient(150deg, var(--choir-state-selected) 0%, var(--choir-state-selected) 52%, var(--choir-state-selected) 100%);
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
    border: 1px solid var(--choir-border-strong);
    border-radius: 18px;
    padding: clamp(18px, 4vw, 34px);
    background: var(--choir-state-selected);
    box-shadow: 0 22px 80px color-mix(in srgb, var(--choir-shadow-color) 32%, transparent);
  }

  .audio-art {
    display: grid;
    place-items: center;
    min-height: 150px;
    border: 1px solid var(--choir-border-strong);
    border-radius: 16px;
    background:
      linear-gradient(135deg, var(--choir-state-selected), var(--choir-state-hover)),
      color-mix(in srgb, var(--choir-text-primary) 4%, transparent);
    color: var(--choir-text-accent);
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
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    font: inherit;
    font-weight: 760;
    padding: 8px 14px;
  }

  .audio-transport button:hover,
  .audio-transport select:hover {
    background: var(--choir-state-selected);
  }

  .audio-transport .audio-play {
    min-width: 94px;
    background: var(--choir-state-selected);
  }

  .audio-transport label {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    color: var(--choir-text-accent);
    font-size: 0.86rem;
  }

  .audio-seek-row {
    display: grid;
    grid-template-columns: auto minmax(100px, 1fr) auto;
    align-items: center;
    gap: 10px;
    color: var(--choir-text-accent);
    font-variant-numeric: tabular-nums;
  }

  .audio-seek-row input[type='range'] {
    width: 100%;
  }

  .audio-position-note {
    position: absolute;
    width: 1px;
    height: 1px;
    margin: -1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
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
    background: color-mix(in srgb, var(--choir-text-primary) 6%, transparent);
    color: var(--choir-text-accent);
  }

  .audio-error {
    color: var(--choir-status-danger);
  }

  .audio-empty {
    display: grid;
    width: min(100% - 24px, 520px);
    height: 100%;
    place-content: center;
    justify-self: center;
    gap: 12px;
  }

  .audio-recent {
    display: grid;
    gap: 8px;
    border: 1px solid var(--choir-border-strong);
    border-radius: 16px;
    background: var(--choir-state-selected);
    padding: 12px;
  }

  .audio-recent > span {
    color: var(--choir-text-accent);
    font-size: 0.74rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .audio-recent button {
    display: grid;
    gap: 2px;
    border: 1px solid var(--choir-border-strong);
    border-radius: 11px;
    background: color-mix(in srgb, var(--choir-text-primary) 6%, transparent);
    color: var(--choir-text-accent);
    cursor: pointer;
    padding: 9px 10px;
    text-align: left;
  }

  .audio-recent button:hover,
  .audio-recent button:focus-visible {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-hover);
  }

  .audio-recent strong,
  .audio-recent small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .audio-recent small {
    color: var(--choir-text-accent);
    font-size: 0.74rem;
  }

  .audio-info {
    position: absolute;
    z-index: 2;
    right: 10px;
    bottom: 10px;
    width: max-content;
    max-width: min(520px, calc(100% - 20px));
    color: var(--choir-text-accent);
  }

  .audio-info summary {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    background: var(--choir-state-selected);
    backdrop-filter: blur(12px);
    cursor: pointer;
    color: var(--choir-text-accent);
    font-size: 0;
    font-weight: 820;
    list-style: none;
    margin-left: auto;
    padding: 0;
  }

  .audio-info summary::-webkit-details-marker {
    display: none;
  }

  .audio-info summary span {
    font-size: 0.95rem;
    line-height: 1;
  }

  .audio-info[open] {
    left: 10px;
    width: auto;
    border: 1px solid var(--choir-border-strong);
    border-radius: 12px;
    padding: 8px 10px;
    background: var(--choir-state-selected);
    backdrop-filter: blur(12px);
  }

  .audio-info h2 {
    margin: 10px 0;
    color: var(--choir-text-accent);
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
    color: var(--choir-text-accent);
    font-weight: 760;
  }

  .audio-info dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .audio-info a {
    color: var(--choir-text-accent);
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
