<script>
  import { onDestroy, onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';
  import { buildListenPath, formatTime, parsePodcastFeed } from './podcast.js';
  import { loadMediaProgress, saveMediaPosition } from './media-utils.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';

  export let appContext = {};
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let library = [];
  let libraryLoading = false;
  let libraryError = '';
  let libraryRequestSeq = 0;
  let searchQuery = '';
  let searchResults = [];
  let searchLoading = false;
  let searchStatus = '';
  let importUrl = '';
  let importing = false;
  let activeEpisodeId = '';
  let activeAudioEl = null;
  let playbackSpeed = 1;
  let playbackPosition = 0;
  let playbackDuration = 0;
  let playbackError = '';
  let isPlaying = false;
  let removeLiveListener = () => {};

  const recommendedPodcasts = [
    {
      title: 'Lenny’s Podcast',
      description: 'Product, growth, and startup conversations.',
      feedUrl: 'https://api.substack.com/feed/podcast/10845.rss',
    },
    {
      title: 'Acquired',
      description: 'Company histories and technology strategy.',
      feedUrl: 'https://feeds.transistor.fm/acquired',
    },
    {
      title: 'The Vergecast',
      description: 'Weekly technology news and product analysis.',
      feedUrl: 'https://feeds.megaphone.fm/vergecast',
    },
  ];

  $: sourceUrl = item?.source_url || appContext?.sourceUrl || '';
  $: contentId = appContext?.contentId || appContext?.content_id || '';
  $: feed = item?.text_content ? parsePodcastFeed(item.text_content, item) : null;
  $: listenPath = feed ? buildListenPath(feed, item) : null;
  $: episodes = listenPath?.episodes || [];
  $: activeEpisode =
    episodes.find((episode) => episode.id === activeEpisodeId) ||
    episodes.find((episode) => episode.audioUrl) ||
    episodes[0] ||
    null;
  $: showTitle = feed?.title || item?.title || appContext?.windowTitle || 'Podcast';
  $: showLibraryLoading = libraryLoading && library.length === 0;
  $: playableLabel = listenPath
    ? `${listenPath.episodeCount} episodes, ${listenPath.playableCount} playable`
    : '';

  async function loadInitialState() {
    if (item) {
      upsertLibraryItem(item);
      return;
    }
    if (!authenticated) {
      libraryLoading = false;
      libraryError = '';
      return;
    }
    if (contentId || sourceUrl) {
      await loadContentItem();
      return;
    }
    await loadLibrary();
  }

  async function loadContentItem() {
    if (!authenticated) {
      error = 'Sign in to load this podcast feed into your computer.';
      return;
    }
    loading = true;
    error = '';
    try {
      const res = contentId
        ? await fetchWithRenewal(`/api/content/items/${encodeURIComponent(contentId)}`)
        : await fetchWithRenewal('/api/content/import-url', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: sourceUrl, query: appContext?.windowTitle || sourceUrl }),
          });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        error = body.error || `Podcast load failed (${res.status})`;
        return;
      }
      item = await res.json();
      upsertLibraryItem(item);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = 'Podcast load failed';
    } finally {
      loading = false;
    }
  }

  async function loadLibrary({ force = false } = {}) {
    if (!authenticated) {
      libraryLoading = false;
      libraryError = '';
      return;
    }
    if (libraryLoading && !force) return;
    const requestSeq = ++libraryRequestSeq;
    libraryLoading = true;
    libraryError = '';
    try {
      const res = await fetchWithRenewal('/api/content/items?limit=100');
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        libraryError = body.error || `Podcast library failed (${res.status})`;
        return;
      }
      const body = await res.json();
      if (requestSeq !== libraryRequestSeq) return;
      library = (body.items || []).filter(isPodcastContent);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      libraryError = 'Podcast library failed';
    } finally {
      if (requestSeq === libraryRequestSeq) {
        libraryLoading = false;
      }
    }
  }

  function isPodcastContent(content) {
    return (
      content?.app_hint === 'podcast' ||
      content?.media_type === 'application/rss+xml' ||
      /podcast|rss/i.test(`${content?.source_url || ''} ${content?.file_path || ''}`)
    );
  }

  function feedForContent(content) {
    return content?.text_content ? parsePodcastFeed(content.text_content, content) : null;
  }

  function libraryTitle(content) {
    return feedForContent(content)?.title || content?.title || content?.source_url || 'Podcast feed';
  }

  function libraryMeta(content) {
    const parsed = feedForContent(content);
    if (parsed?.episodes?.length) {
      const playable = parsed.episodes.filter((episode) => episode.audioUrl).length;
      return `${parsed.episodes.length} episodes, ${playable} playable`;
    }
    return content?.source_url || content?.file_path || 'RSS feed artifact';
  }

  function upsertLibraryItem(content) {
    if (!content) return;
    library = [
      content,
      ...library.filter((existing) => existing.content_id !== content.content_id),
    ];
  }

  function openPodcastItem(content) {
    upsertLibraryItem(content);
    item = content;
    error = '';
    activeEpisodeId = '';
    playbackPosition = 0;
    playbackDuration = 0;
    playbackError = '';
    isPlaying = false;
  }

  function backToLibrary() {
    upsertLibraryItem(item);
    item = null;
    activeEpisodeId = '';
    activeAudioEl = null;
    playbackError = '';
    isPlaying = false;
    loadLibrary({ force: true });
  }

  async function searchPodcasts() {
    const query = searchQuery.trim();
    if (!query || searchLoading) return;
    if (!authenticated) {
      requestAuth('podcast_search');
      return;
    }
    searchLoading = true;
    searchStatus = '';
    libraryError = '';
    try {
      const res = await fetchWithRenewal(`/api/podcast/search?q=${encodeURIComponent(query)}&limit=12`);
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        libraryError = body.error || `Podcast search failed (${res.status})`;
        return;
      }
      const body = await res.json();
      searchResults = body.results || [];
      searchStatus = `${searchResults.length} result${searchResults.length === 1 ? '' : 's'} from ${body.provider_status || body.provider || 'provider'}`;
      if ((body.warnings || []).length > 0) {
        searchStatus += `; ${body.warnings[0]}`;
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      libraryError = 'Podcast search failed';
    } finally {
      searchLoading = false;
    }
  }

  async function importPodcastFeed() {
    const url = importUrl.trim();
    if (!url || importing) return;
    if (!authenticated) {
      requestAuth('podcast_import');
      return;
    }
    importing = true;
    libraryError = '';
    try {
      const res = await fetchWithRenewal('/api/content/import-url', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url, query: url }),
      });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        libraryError = body.error || `Podcast import failed (${res.status})`;
        return;
      }
      item = await res.json();
      importUrl = '';
      upsertLibraryItem(item);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      libraryError = 'Podcast import failed';
    } finally {
      importing = false;
    }
  }

  async function importPodcastResult(result) {
    if (!result?.feed_url || importing) return;
    importUrl = result.feed_url;
    await importPodcastFeed();
  }

  function requestAuth(kind = 'podcast') {
    dispatch('authrequired', { kind, appId: 'podcast', appName: 'Podcast' });
  }

  function episodeProgressSource(episode = activeEpisode) {
    return episode ? {
      filePath: `podcast:${episode.id}`,
      contentId: episode.id,
      sourceUrl: episode.audioUrl || '',
      title: episode.title || '',
      playbackRate,
    } : {};
  }

  async function selectEpisode(episode) {
    activeEpisodeId = episode.id;
    try {
      const progress = await loadMediaProgress('podcast', episodeProgressSource(episode));
      playbackPosition = Number(progress.currentTime) || 0;
    } catch (_err) {
      playbackPosition = 0;
    }
    playbackDuration = 0;
    playbackError = '';
    isPlaying = false;
    syncAudioFromServer();
  }

  function syncAudioFromServer() {
    setTimeout(async () => {
      if (!activeAudioEl || !activeEpisode) return;
      activeAudioEl.playbackRate = playbackSpeed;
      let saved = playbackPosition;
      try {
        const progress = await loadMediaProgress('podcast', episodeProgressSource());
        saved = Number(progress.currentTime) || 0;
      } catch (_err) {
        saved = playbackPosition;
      }
      if (saved && Number.isFinite(saved) && Math.abs(activeAudioEl.currentTime - saved) > 3) {
        activeAudioEl.currentTime = saved;
      }
      savePlaybackProgress();
    }, 0);
  }

  function savePlaybackProgress() {
    if (!activeAudioEl || !activeEpisode) return;
    playbackPosition = activeAudioEl.currentTime || 0;
    playbackDuration = Number.isFinite(activeAudioEl.duration) ? activeAudioEl.duration : 0;
    saveMediaPosition('podcast', episodeProgressSource(), playbackPosition, playbackDuration);
  }

  async function togglePlayback() {
    if (!activeAudioEl) return;
    playbackError = '';
    try {
      if (activeAudioEl.paused) {
        await activeAudioEl.play();
        isPlaying = true;
      } else {
        activeAudioEl.pause();
        isPlaying = false;
      }
    } catch (err) {
      playbackError = 'Playback could not start. Check browser audio permissions or the episode source.';
    }
  }

  function seekBy(seconds) {
    if (!activeAudioEl) return;
    const duration = Number.isFinite(activeAudioEl.duration) ? activeAudioEl.duration : Infinity;
    activeAudioEl.currentTime = Math.max(0, Math.min(duration, activeAudioEl.currentTime + seconds));
    savePlaybackProgress();
  }

  function seekTo(value) {
    if (!activeAudioEl) return;
    const next = Number(value);
    if (!Number.isFinite(next)) return;
    const duration = Number.isFinite(activeAudioEl.duration)
      ? activeAudioEl.duration
      : Math.max(next, playbackDuration || 0);
    activeAudioEl.currentTime = Math.max(0, Math.min(duration || next, next));
    savePlaybackProgress();
  }

  function setSpeed(speed) {
    playbackSpeed = Number(speed) || 1;
    if (activeAudioEl) activeAudioEl.playbackRate = playbackSpeed;
  }

  onMount(() => {
    void loadInitialState();
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
      if (kind === 'content.item.created' && !item) {
        void loadLibrary({ force: true });
      }
      if (kind === 'media.progress.updated' && activeEpisode?.id) {
        const payload = message.payload || {};
        if (payload.kind === 'podcast' && payload.identity === `podcast:${activeEpisode.id}` && !isPlaying) {
          playbackPosition = Number(payload.current_time) || playbackPosition;
          playbackDuration = Number(payload.duration) || playbackDuration;
        }
      }
    });
  });

  onDestroy(() => {
    removeLiveListener();
  });
</script>

<section class="podcast-app" data-media-app data-media-kind="podcast" data-podcast-app data-content-app="podcast">
  <header class="podcast-topbar">
    {#if item}
      <button class="back-button" type="button" on:click={backToLibrary} aria-label="Back to podcast library" data-podcast-back>Back</button>
    {/if}
    <div class="title-group">
      <p>{item ? 'Now playing' : 'Podcast'}</p>
      <h2>{showTitle}</h2>
    </div>
    {#if item && playableLabel}
      <span class="show-count">{playableLabel}</span>
    {/if}
  </header>

  {#if loading}
    <p class="status">Loading podcast...</p>
  {:else if error}
    <p class="error" role="alert">{error}</p>
  {:else if !item}
    <section class="podcast-library" data-media-stage data-podcast-library>
      <form class="search-row" on:submit|preventDefault={searchPodcasts} data-podcast-search>
        <input
          bind:value={searchQuery}
          type="search"
          placeholder="Search podcasts"
          aria-label="Search podcasts"
          data-podcast-search-query
        />
        <button type="submit" disabled={searchLoading || !searchQuery.trim()} data-podcast-search-submit>
          {searchLoading ? 'Searching' : 'Search'}
        </button>
      </form>

      <details class="advanced-import">
        <summary>Import RSS feed</summary>
        <form class="import-row" on:submit|preventDefault={importPodcastFeed} data-podcast-import>
          <input
            bind:value={importUrl}
            type="url"
            placeholder="https://example.com/feed.rss"
            aria-label="Podcast RSS feed URL"
            data-podcast-import-url
          />
          <button type="submit" disabled={importing || !importUrl.trim()} data-podcast-import-submit>
            {importing ? 'Importing' : 'Import'}
          </button>
        </form>
      </details>

      {#if searchStatus}<p class="status subtle" data-podcast-search-status>{searchStatus}</p>{/if}
      {#if libraryError}<p class="error" role="alert">{libraryError}</p>{/if}

      <div class="library-scroll">
        {#if !authenticated}
          <section class="library-section" data-podcast-library-recommended>
            <h3>Recommended Podcasts</h3>
            <div class="library-list">
              {#each recommendedPodcasts as podcast}
                <article class="result-row">
                  <div>
                    <strong>{podcast.title}</strong>
                    <span>{podcast.description}</span>
                    <span>{podcast.feedUrl}</span>
                  </div>
                  <button type="button" on:click={() => requestAuth('podcast_add_recommended')}>Add</button>
                </article>
              {/each}
            </div>
            <p class="status subtle">Sign in to search providers, subscribe, and keep playback positions across sessions.</p>
          </section>
        {/if}

        {#if searchResults.length > 0}
          <section class="library-section" data-podcast-search-results>
            <h3>Search Results</h3>
            <div class="library-list">
              {#each searchResults as result}
                <article class="result-row" data-podcast-search-result>
                  <div>
                    <strong>{result.title || result.feed_url}</strong>
                    {#if result.author}<span>{result.author}</span>{/if}
                    <span>{result.feed_url}</span>
                  </div>
                  <button type="button" on:click={() => importPodcastResult(result)} disabled={importing} data-podcast-result-import>
                    Add
                  </button>
                </article>
              {/each}
            </div>
          </section>
        {/if}

        <section class="library-section">
          <h3>Subscribed Podcasts</h3>
          {#if showLibraryLoading}
            <p class="status">Loading podcast library...</p>
          {:else if library.length === 0}
            <p class="status">No subscribed podcasts yet. Search above to add one.</p>
          {:else}
            {#if libraryLoading}<p class="status subtle">Updating library...</p>{/if}
            <div class="library-list">
              {#each library as content}
                <button class="show-row" type="button" on:click={() => openPodcastItem(content)} data-podcast-library-item>
                  <span class="cover-mark">RSS</span>
                  <span>
                    <strong>{libraryTitle(content)}</strong>
                    <small>{libraryMeta(content)}</small>
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        </section>
      </div>
    </section>
  {:else if feed && episodes.length > 0}
    <section class="podcast-detail" data-media-stage data-podcast-feed data-listen-path-id={listenPath?.id || ''}>
      <section class="show-strip" data-radio-listen-path>
        <div>
          <h3>{feed.title}</h3>
          {#if feed.description}<p>{feed.description}</p>{/if}
        </div>
      </section>

      <section class="episode-list" data-podcast-episodes-scroll>
        {#each episodes as episode}
          <article class:selected={activeEpisode?.id === episode.id} class="episode-row" data-podcast-episode data-episode-id={episode.id}>
            <button class="episode-main" type="button" on:click={() => selectEpisode(episode)} data-podcast-select-episode disabled={!episode.audioUrl}>
              <strong>{episode.title}</strong>
              <span>
                {#if episode.publishedAt}{episode.publishedAt}{/if}
                {#if episode.duration}{episode.publishedAt ? ' | ' : ''}{episode.duration}{/if}
              </span>
              {#if episode.description}<small>{episode.description}</small>{/if}
            </button>
          </article>
        {/each}
      </section>

      {#if activeEpisode?.audioUrl}
        <footer class="podcast-player" data-podcast-player>
          <div class="player-title">
            <span>Playing</span>
            <strong>{activeEpisode.title}</strong>
          </div>
          <audio
            src={activeEpisode.audioUrl}
            preload="metadata"
            bind:this={activeAudioEl}
            on:loadedmetadata={syncAudioFromServer}
            on:durationchange={savePlaybackProgress}
            on:timeupdate={savePlaybackProgress}
            on:play={() => isPlaying = true}
            on:pause={() => { isPlaying = false; savePlaybackProgress(); }}
            on:ended={() => { isPlaying = false; savePlaybackProgress(); }}
            on:error={() => playbackError = 'Episode audio could not be loaded by the browser.'}
            data-podcast-audio
          />
          <div class="controls" data-podcast-controls>
            <button type="button" on:click={() => seekBy(-15)} data-podcast-seek-back>-15s</button>
            <button class="play-button" type="button" on:click={togglePlayback} aria-pressed={isPlaying} data-podcast-play-pause>
              {isPlaying ? 'Pause' : 'Play'}
            </button>
            <button type="button" on:click={() => seekBy(30)} data-podcast-seek-forward>+30s</button>
            <select bind:value={playbackSpeed} on:change={() => setSpeed(playbackSpeed)} aria-label="Playback speed" data-podcast-speed>
              <option value={0.75}>0.75x</option>
              <option value={1}>1x</option>
              <option value={1.25}>1.25x</option>
              <option value={1.5}>1.5x</option>
              <option value={2}>2x</option>
            </select>
          </div>
          <div class="timeline">
            <span>{formatTime(playbackPosition)}</span>
            <input
              type="range"
              min="0"
              max={Math.max(playbackDuration || playbackPosition || 1, 1)}
              step="1"
              value={playbackPosition}
              on:input={(event) => seekTo(event.currentTarget.value)}
              aria-label="Seek episode"
              data-podcast-progress
              data-podcast-seek
            />
            <span>{playbackDuration ? formatTime(playbackDuration) : '--:--'}</span>
          </div>
          {#if playbackError}<p class="error compact" role="alert" data-podcast-playback-error>{playbackError}</p>{/if}
        </footer>
      {/if}
    </section>
  {:else}
    <p class="status">This feed has no playable episodes.</p>
  {/if}
</section>

<style>
  .podcast-app {
    display: flex;
    flex-direction: column;
    gap: 14px;
    height: 100%;
    min-height: 0;
    padding: 16px;
    color: var(--choir-fg, #f5f7ff);
    background:
      radial-gradient(circle at 16% 0%, rgba(68, 121, 212, 0.22), transparent 30%),
      linear-gradient(180deg, rgba(11, 18, 32, 0.98), rgba(6, 8, 16, 0.98));
    overflow: hidden;
  }

  .podcast-topbar {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 12px;
    align-items: center;
    flex: 0 0 auto;
  }

  .title-group {
    min-width: 0;
  }

  .title-group p,
  .show-count {
    margin: 0;
    color: var(--choir-muted, #a8adbd);
    font-size: 0.78rem;
    font-weight: 800;
    text-transform: uppercase;
  }

  .title-group h2 {
    margin: 2px 0 0;
    font-size: 1.45rem;
    overflow-wrap: anywhere;
  }

  button,
  select {
    border: 1px solid rgba(99, 153, 255, 0.46);
    border-radius: 12px;
    color: #ecf3ff;
    background: rgba(18, 32, 56, 0.82);
  }

  button {
    padding: 9px 12px;
    cursor: pointer;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.54;
  }

  .back-button {
    padding: 6px 10px;
    border-radius: 10px;
    font-size: 0.82rem;
  }

  .podcast-library,
  .podcast-detail {
    flex: 1 1 auto;
    min-height: 0;
  }

  .podcast-library {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .search-row,
  .import-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
  }

  input {
    min-width: 0;
    border: 1px solid rgba(120, 135, 170, 0.34);
    border-radius: 12px;
    padding: 10px 12px;
    color: var(--choir-fg, #f5f7ff);
    background: rgba(255, 255, 255, 0.08);
  }

  .advanced-import {
    border: 1px solid rgba(120, 135, 170, 0.24);
    border-radius: 12px;
    padding: 8px 10px;
    background: rgba(10, 15, 27, 0.7);
  }

  .advanced-import summary {
    cursor: pointer;
    color: var(--choir-muted, #a8adbd);
    font-size: 0.86rem;
    font-weight: 750;
  }

  .advanced-import .import-row {
    margin-top: 10px;
  }

  .library-scroll,
  .episode-list {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
    touch-action: pan-y;
  }

  .library-scroll {
    display: grid;
    align-content: start;
    gap: 16px;
    padding-right: 4px;
  }

  .library-section {
    display: grid;
    gap: 10px;
  }

  .library-section h3 {
    margin: 0;
    font-size: 1rem;
  }

  .library-list {
    display: grid;
    gap: 10px;
  }

  .show-row,
  .result-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 12px;
    align-items: center;
    width: 100%;
    text-align: left;
    border-color: rgba(120, 135, 170, 0.28);
    background: rgba(12, 18, 31, 0.88);
  }

  .result-row {
    grid-template-columns: minmax(0, 1fr) auto;
    padding: 12px;
    border: 1px solid rgba(120, 135, 170, 0.28);
    border-radius: 14px;
  }

  .show-row span:last-child,
  .result-row div {
    display: grid;
    gap: 4px;
    min-width: 0;
  }

  .show-row small,
  .result-row span,
  .status,
  .error {
    color: var(--choir-muted, #a8adbd);
  }

  .cover-mark {
    display: grid;
    place-items: center;
    width: 42px;
    height: 42px;
    border-radius: 12px;
    color: #dbe8ff;
    background: linear-gradient(135deg, rgba(72, 118, 255, 0.7), rgba(40, 196, 165, 0.55));
    font-size: 0.7rem;
    font-weight: 900;
  }

  .podcast-detail {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    gap: 12px;
    min-height: 0;
    overflow: hidden;
  }

  .show-strip {
    border: 1px solid rgba(120, 135, 170, 0.24);
    border-radius: 16px;
    padding: 12px;
    background: rgba(8, 13, 24, 0.76);
  }

  .show-strip h3 {
    margin: 0 0 4px;
    font-size: 1.02rem;
  }

  .show-strip p {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    margin: 0;
    overflow: hidden;
    color: var(--choir-muted, #a8adbd);
  }

  .episode-list {
    display: grid;
    align-content: start;
    gap: 10px;
    padding-right: 4px;
    min-height: 0;
  }

  .episode-row {
    border: 1px solid rgba(120, 135, 170, 0.25);
    border-radius: 14px;
    background: rgba(10, 16, 29, 0.84);
  }

  .episode-row.selected {
    border-color: rgba(99, 153, 255, 0.72);
    background: rgba(33, 72, 148, 0.26);
  }

  .episode-main {
    display: grid;
    gap: 5px;
    width: 100%;
    border: 0;
    text-align: left;
    background: transparent;
  }

  .episode-main strong {
    font-size: 0.98rem;
  }

  .episode-main span,
  .episode-main small {
    color: var(--choir-muted, #a8adbd);
  }

  .episode-main small {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .podcast-player {
    display: grid;
    flex: 0 0 auto;
    gap: 8px;
    border: 1px solid rgba(99, 153, 255, 0.42);
    border-radius: 16px;
    padding: 10px;
    background: rgba(7, 12, 23, 0.96);
    box-shadow: 0 -12px 30px rgba(0, 0, 0, 0.22);
  }

  .podcast-player audio {
    display: none;
  }

  .player-title {
    display: grid;
    gap: 2px;
    min-width: 0;
  }

  .player-title span {
    color: var(--choir-muted, #a8adbd);
    font-size: 0.72rem;
    font-weight: 850;
    text-transform: uppercase;
  }

  .player-title strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .controls {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .controls button {
    padding: 8px 10px;
  }

  .play-button {
    min-width: 72px;
  }

  select {
    padding: 8px;
  }

  .timeline {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    gap: 8px;
    align-items: center;
  }

  .timeline span {
    color: var(--choir-muted, #a8adbd);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
  }

  .timeline input[type='range'] {
    width: 100%;
    min-width: 0;
    accent-color: #7aa2ff;
  }

  .status,
  .error {
    margin: 0;
    border-radius: 14px;
    padding: 12px 14px;
    background: rgba(255, 255, 255, 0.06);
  }

  .error {
    color: #ffd6d6;
  }

  .error.compact {
    padding: 8px 10px;
  }

  .subtle {
    padding: 8px 10px;
    font-size: 0.85rem;
  }

  @media (max-width: 720px) {
    .podcast-app {
      gap: 10px;
      padding: 8px;
    }

    .podcast-topbar {
      grid-template-columns: auto minmax(0, 1fr);
    }

    .show-count {
      grid-column: 2;
      justify-self: start;
    }

    .search-row,
    .import-row,
    .result-row {
      grid-template-columns: 1fr;
    }

    .title-group h2 {
      font-size: 1.22rem;
    }

    .podcast-detail {
      grid-template-rows: auto minmax(0, 1fr) auto;
    }

    .episode-list {
      min-height: 0;
    }
  }
</style>
