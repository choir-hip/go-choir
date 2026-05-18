<script>
  import { onMount } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';
  import { createEventDispatcher } from 'svelte';

  export let appContext = {};

  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';
  let podcastLibrary = [];
  let podcastLibraryLoading = false;
  let podcastLibraryError = '';
  let podcastImportUrl = '';
  let podcastImporting = false;
  let podcastSearchQuery = '';
  let podcastSearchResults = [];
  let podcastSearchLoading = false;
  let podcastSearchStatus = '';
  let radioStatus = '';

  $: sourceUrl = item?.source_url || appContext?.sourceUrl || '';
  $: filePath = item?.file_path || appContext?.filePath || '';
  $: mediaType = item?.media_type || appContext?.mediaType || '';
  $: appHint = item?.app_hint || appContext?.appHint || appContext?.appId || 'files';
  $: title = item?.title || appContext?.windowTitle || appContext?.title || appHint;

  async function loadContentItem() {
    const contentId = appContext?.contentId || appContext?.content_id || '';
    if (item) return;
    if (!contentId && !(appHint === 'podcast' && sourceUrl)) {
      if (appHint === 'podcast') await loadPodcastLibrary();
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
            body: JSON.stringify({ url: sourceUrl, query: title || sourceUrl }),
          });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        error = body.error || `Content load failed (${res.status})`;
        return;
      }
      item = await res.json();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = 'Content load failed';
    } finally {
      loading = false;
    }
  }

  async function loadPodcastLibrary() {
    if (podcastLibraryLoading) return;
    podcastLibraryLoading = true;
    podcastLibraryError = '';
    try {
      const res = await fetchWithRenewal('/api/content/items?limit=100');
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        podcastLibraryError = body.error || `Podcast library failed (${res.status})`;
        return;
      }
      const body = await res.json();
      podcastLibrary = (body.items || []).filter((content) =>
        content.app_hint === 'podcast' ||
        content.media_type === 'application/rss+xml' ||
        /podcast|rss/i.test(`${content.source_url || ''} ${content.file_path || ''}`)
      );
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      podcastLibraryError = 'Podcast library failed';
    } finally {
      podcastLibraryLoading = false;
    }
  }

  async function importPodcastFeed() {
    const url = podcastImportUrl.trim();
    if (!url || podcastImporting) return;
    podcastImporting = true;
    podcastLibraryError = '';
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
        podcastLibraryError = body.error || `Podcast import failed (${res.status})`;
        return;
      }
      item = await res.json();
      podcastImportUrl = '';
      podcastLibrary = [item, ...podcastLibrary.filter((content) => content.content_id !== item.content_id)];
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      podcastLibraryError = 'Podcast import failed';
    } finally {
      podcastImporting = false;
    }
  }

  async function searchPodcasts() {
    const query = podcastSearchQuery.trim();
    if (!query || podcastSearchLoading) return;
    podcastSearchLoading = true;
    podcastLibraryError = '';
    podcastSearchStatus = '';
    try {
      const res = await fetchWithRenewal(`/api/podcast/search?q=${encodeURIComponent(query)}&limit=12`);
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        podcastLibraryError = body.error || `Podcast search failed (${res.status})`;
        return;
      }
      const body = await res.json();
      podcastSearchResults = body.results || [];
      podcastSearchStatus = `${podcastSearchResults.length} result${podcastSearchResults.length === 1 ? '' : 's'} from ${body.provider_status || body.provider || 'provider'}`;
      if ((body.warnings || []).length > 0) {
        podcastSearchStatus += `; ${body.warnings[0]}`;
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      podcastLibraryError = 'Podcast search failed';
    } finally {
      podcastSearchLoading = false;
    }
  }

  async function importPodcastResult(result) {
    if (!result?.feed_url || podcastImporting) return;
    podcastImportUrl = result.feed_url;
    await importPodcastFeed();
  }

  function openPodcastItem(content) {
    item = content;
    error = '';
    radioStatus = '';
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

  $: displayUrl = filePath ? apiFileURL(filePath) : sourceUrl;
  $: embedUrl = mediaType === 'video/youtube' || /youtube\\.com|youtu\\.be/.test(sourceUrl)
    ? youtubeEmbedURL(sourceUrl)
    : '';
  $: podcastFeed = appHint === 'podcast' && item?.text_content
    ? parsePodcastFeed(item.text_content, item)
    : null;
  $: podcastEpisodes = podcastFeed?.episodes || [];
  $: listenPath = podcastFeed ? buildListenPath(podcastFeed, item) : null;

  function textFromFirst(parent, tagName) {
    return parent.getElementsByTagName(tagName)[0]?.textContent?.trim() || '';
  }

  function stripMarkup(value) {
    return String(value || '')
      .replace(/<!\[CDATA\[([\s\S]*?)\]\]>/g, '$1')
      .replace(/<[^>]+>/g, '')
      .replace(/&amp;/g, '&')
      .replace(/&quot;/g, '"')
      .replace(/&#39;|&apos;/g, "'")
      .replace(/&lt;/g, '<')
      .replace(/&gt;/g, '>')
      .trim();
  }

  function excerpt(value, limit = 260) {
    const clean = stripMarkup(value).replace(/\s+/g, ' ').trim();
    if (clean.length <= limit) return clean;
    return `${clean.slice(0, limit - 1).trim()}...`;
  }

  function firstTagText(source, tagName) {
    const match = new RegExp(`<${tagName}(?:\\s[^>]*)?>([\\s\\S]*?)<\\/${tagName}>`, 'i').exec(source);
    return stripMarkup(match?.[1] || '');
  }

  function firstAttribute(source, tagName, attrName) {
    const tag = new RegExp(`<${tagName}\\b[^>]*>`, 'i').exec(source)?.[0] || '';
    const attr = new RegExp(`${attrName}=["']([^"']+)["']`, 'i').exec(tag);
    return stripMarkup(attr?.[1] || '');
  }

  function stableId(value) {
    let hash = 5381;
    for (const char of String(value || '')) {
      hash = ((hash << 5) + hash) ^ char.charCodeAt(0);
    }
    return Math.abs(hash >>> 0).toString(36);
  }

  function normalizeEpisode(episode, index) {
    const key = episode.guid || episode.audioUrl || episode.link || `${episode.title}:${index}`;
    return {
      id: `episode-${stableId(key)}`,
      title: stripMarkup(episode.title) || 'Untitled episode',
      description: excerpt(episode.description, 420),
      publishedAt: stripMarkup(episode.publishedAt),
      duration: stripMarkup(episode.duration),
      guid: stripMarkup(episode.guid),
      link: stripMarkup(episode.link),
      audioUrl: stripMarkup(episode.audioUrl),
    };
  }

  function parsePodcastEpisodesLoosely(xmlText) {
    return Array.from(String(xmlText || '').matchAll(/<item\b[\s\S]*?<\/item>/gi))
      .slice(0, 24)
      .map((match, index) => {
        const source = match[0];
        return normalizeEpisode({
          title: firstTagText(source, 'title') || 'Untitled episode',
          description: firstTagText(source, 'description'),
          publishedAt: firstTagText(source, 'pubDate'),
          duration: firstTagText(source, 'itunes:duration') || firstTagText(source, 'duration'),
          guid: firstTagText(source, 'guid'),
          link: firstTagText(source, 'link'),
          audioUrl: firstAttribute(source, 'enclosure', 'url') || firstAttribute(source, 'media:content', 'url'),
        }, index);
      })
      .filter((episode) => episode.title || episode.audioUrl);
  }

  function parsePodcastEpisodes(xmlText) {
    try {
      const parsed = new DOMParser().parseFromString(xmlText, 'application/xml');
      if (parsed.querySelector('parsererror')) return parsePodcastEpisodesLoosely(xmlText);
      return Array.from(parsed.getElementsByTagName('item')).slice(0, 24).map((episode, index) => {
        const enclosure = episode.getElementsByTagName('enclosure')[0];
        const mediaContent = episode.getElementsByTagName('media:content')[0];
        return normalizeEpisode({
          title: textFromFirst(episode, 'title') || 'Untitled episode',
          description: textFromFirst(episode, 'itunes:summary') || textFromFirst(episode, 'description'),
          publishedAt: textFromFirst(episode, 'pubDate'),
          duration: textFromFirst(episode, 'itunes:duration') || textFromFirst(episode, 'duration'),
          guid: textFromFirst(episode, 'guid'),
          link: textFromFirst(episode, 'link'),
          audioUrl: enclosure?.getAttribute('url') || mediaContent?.getAttribute('url') || '',
        }, index);
      }).filter((episode) => episode.title || episode.audioUrl);
    } catch (err) {
      return parsePodcastEpisodesLoosely(xmlText);
    }
  }

  function parsePodcastFeedLoosely(xmlText, contentItem) {
    const channel = /<channel\b[\s\S]*?<\/channel>/i.exec(String(xmlText || ''))?.[0] || String(xmlText || '');
    return {
      title: firstTagText(channel, 'title') || contentItem?.title || 'Podcast feed',
      description: excerpt(firstTagText(channel, 'description'), 520),
      link: firstTagText(channel, 'link') || contentItem?.canonical_url || contentItem?.source_url || '',
      episodes: parsePodcastEpisodesLoosely(xmlText),
    };
  }

  function parsePodcastFeed(xmlText, contentItem) {
    try {
      const parsed = new DOMParser().parseFromString(xmlText, 'application/xml');
      if (parsed.querySelector('parsererror')) return parsePodcastFeedLoosely(xmlText, contentItem);
      const channel = parsed.getElementsByTagName('channel')[0] || parsed;
      return {
        title: textFromFirst(channel, 'title') || contentItem?.title || 'Podcast feed',
        description: excerpt(textFromFirst(channel, 'itunes:summary') || textFromFirst(channel, 'description'), 520),
        link: textFromFirst(channel, 'link') || contentItem?.canonical_url || contentItem?.source_url || '',
        episodes: parsePodcastEpisodes(xmlText),
      };
    } catch (err) {
      return parsePodcastFeedLoosely(xmlText, contentItem);
    }
  }

  function buildListenPath(feed, contentItem) {
    const pathSource = contentItem?.source_url || feed.link || '';
    const playable = feed.episodes.filter((episode) => episode.audioUrl);
    return {
      id: `listen-${stableId(`${contentItem?.content_id || ''}:${pathSource}:${feed.title}`)}`,
      title: feed.title || contentItem?.title || 'Podcast feed',
      sourceUrl: pathSource,
      contentId: contentItem?.content_id || '',
      episodeCount: feed.episodes.length,
      playableCount: playable.length,
      episodes: feed.episodes.map((episode, index) => ({
        ...episode,
        position: index + 1,
      })),
    };
  }

  function markdownLink(label, url) {
    const cleanLabel = String(label || '').replace(/\]/g, '\\]');
    const cleanUrl = String(url || '').trim();
    return cleanUrl ? `[${cleanLabel}](${cleanUrl})` : cleanLabel;
  }

  function buildRadioBrief() {
    if (!podcastFeed || !listenPath) return '';
    const briefTitle = /radio$/i.test(listenPath.title.trim())
      ? `${listenPath.title} Brief`
      : `${listenPath.title} Radio Brief`;
    const lines = [
      `# ${briefTitle}`,
      '',
      '## Source',
      `- Feed: ${markdownLink(listenPath.sourceUrl || podcastFeed.link || 'source feed', listenPath.sourceUrl || podcastFeed.link)}`,
      `- Content artifact: ${listenPath.contentId || 'not recorded'}`,
      `- Listen path: ${listenPath.id}`,
      `- Episodes parsed: ${listenPath.episodeCount}`,
      `- Playable episodes: ${listenPath.playableCount}`,
      '',
      '## Feed Note',
      podcastFeed.description || 'No feed description was provided.',
      '',
      '## Listen Path',
    ];

    for (const episode of listenPath.episodes.slice(0, 12)) {
      lines.push(`${episode.position}. ${markdownLink(episode.title, episode.audioUrl || episode.link)}`);
      if (episode.publishedAt) lines.push(`   - Published: ${episode.publishedAt}`);
      if (episode.duration) lines.push(`   - Duration: ${episode.duration}`);
      if (episode.guid) lines.push(`   - Episode id: ${episode.guid}`);
      if (episode.description) lines.push(`   - Note: ${episode.description}`);
    }

    lines.push(
      '',
      '## Radio Work Queue',
      '- Select claims, clips, and narration beats worth promoting.',
      '- Attach source anchors before turning this into public memory.',
      '- Keep unresolved tensions visible instead of smoothing them into narration.'
    );

    return lines.join('\n');
  }

  function openRadioBrief() {
    const content = buildRadioBrief();
    if (!content) return;
    dispatch('openvtext', {
      title: /radio$/i.test(listenPath.title.trim()) ? `${listenPath.title} Brief` : `Radio Brief - ${listenPath.title}`,
      initialContent: content,
      sourceUrl: listenPath.sourceUrl,
      sourceContentId: listenPath.contentId,
      appHint: 'podcast',
      createdFrom: 'podcast_radio_brief',
    });
    radioStatus = 'Opening radio brief in VText...';
  }

  onMount(loadContentItem);
</script>

<section class="content-viewer" data-content-viewer data-content-app={appHint}>
  <header class="content-header">
    <div>
      <p class="eyebrow">{appHint} content</p>
      <h2>{title}</h2>
    </div>
    {#if sourceUrl}
      <a class="source-link" href={sourceUrl} target="_blank" rel="noreferrer">Open source</a>
    {/if}
  </header>

  {#if loading}
    <p class="status">Loading content metadata...</p>
  {:else if error}
    <p class="error" role="alert">{error}</p>
  {:else}
    <div class="preview-shell">
      {#if appHint === 'podcast' && !item}
        <div class="podcast-library" data-podcast-library>
          <div class="library-header">
            <div>
              <h3>Podcast Library</h3>
              <p>Durable RSS feed artifacts that can become VText radio briefs.</p>
            </div>
            <button type="button" on:click={loadPodcastLibrary} disabled={podcastLibraryLoading} data-podcast-refresh>
              {podcastLibraryLoading ? 'Refreshing...' : 'Refresh'}
            </button>
          </div>
          <form class="podcast-import" on:submit|preventDefault={importPodcastFeed} data-podcast-import>
            <input
              bind:value={podcastImportUrl}
              type="url"
              placeholder="https://example.com/feed.rss"
              aria-label="Podcast RSS feed URL"
              data-podcast-import-url
            />
            <button type="submit" disabled={podcastImporting || !podcastImportUrl.trim()} data-podcast-import-submit>
              {podcastImporting ? 'Importing...' : 'Import'}
            </button>
          </form>
          <form class="podcast-search" on:submit|preventDefault={searchPodcasts} data-podcast-search>
            <input
              bind:value={podcastSearchQuery}
              type="search"
              placeholder="Search podcasts"
              aria-label="Search podcasts"
              data-podcast-search-query
            />
            <button type="submit" disabled={podcastSearchLoading || !podcastSearchQuery.trim()} data-podcast-search-submit>
              {podcastSearchLoading ? 'Searching...' : 'Search'}
            </button>
          </form>
          {#if podcastSearchStatus}<p class="status" data-podcast-search-status>{podcastSearchStatus}</p>{/if}
          {#if podcastSearchResults.length > 0}
            <div class="podcast-search-results" data-podcast-search-results>
              {#each podcastSearchResults as result}
                <article class="podcast-search-result" data-podcast-search-result>
                  <div>
                    <strong>{result.title || result.feed_url}</strong>
                    {#if result.author}<span>{result.author}</span>{/if}
                    <span>{result.feed_url}</span>
                  </div>
                  <button type="button" on:click={() => importPodcastResult(result)} disabled={podcastImporting} data-podcast-result-import>
                    Import
                  </button>
                </article>
              {/each}
            </div>
          {/if}
          {#if podcastLibraryError}
            <p class="error" role="alert">{podcastLibraryError}</p>
          {:else if podcastLibraryLoading}
            <p class="status">Loading podcast artifacts...</p>
          {:else if podcastLibrary.length === 0}
            <p class="status">No podcast feed artifacts yet.</p>
          {:else}
            <div class="library-list">
              {#each podcastLibrary as content}
                <button
                  class="library-item"
                  type="button"
                  on:click={() => openPodcastItem(content)}
                  data-podcast-library-item
                >
                  <strong>{content.title || content.source_url || 'Podcast feed'}</strong>
                  <span>{content.source_url || content.file_path || content.content_id}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {:else if appHint === 'podcast' && podcastEpisodes.length > 0}
        <div class="podcast-list" data-podcast-feed data-listen-path-id={listenPath?.id || ''}>
          <div class="radio-panel" data-radio-listen-path>
            <div>
              <p class="eyebrow">Radio listen path</p>
              <h3>{podcastFeed.title}</h3>
              {#if podcastFeed.description}<p>{podcastFeed.description}</p>{/if}
              <p class="path-meta">
                {listenPath.episodeCount} episodes - {listenPath.playableCount} playable - {listenPath.id}
              </p>
            </div>
            <button type="button" on:click={openRadioBrief} data-podcast-open-vtext>
              Open in VText
            </button>
          </div>
          {#if radioStatus}<p class="status" data-radio-status>{radioStatus}</p>{/if}
          {#each podcastEpisodes as episode}
            <article class="podcast-episode" data-podcast-episode data-episode-id={episode.id}>
              <div>
                <h3>{episode.title}</h3>
                {#if episode.publishedAt}<p class="episode-date">{episode.publishedAt}</p>{/if}
                {#if episode.duration}<p class="episode-date">{episode.duration}</p>{/if}
                {#if episode.description}<p>{episode.description}</p>{/if}
              </div>
              {#if episode.audioUrl}
                <audio src={episode.audioUrl} controls data-podcast-audio />
              {/if}
            </article>
          {/each}
        </div>
      {:else if appHint === 'image' && displayUrl}
        <img src={displayUrl} alt={title} />
      {:else if appHint === 'audio' && displayUrl}
        <audio src={displayUrl} controls />
      {:else if appHint === 'video' && embedUrl}
        <iframe title={title} src={embedUrl} allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen />
      {:else if appHint === 'video' && displayUrl}
        <video src={displayUrl} controls>
          <track kind="captions" />
        </video>
      {:else if appHint === 'pdf' && displayUrl}
        <iframe title={title} src={displayUrl} />
      {:else}
        <div class="metadata-card">
          <p><strong>Media type:</strong> {mediaType || 'unknown'}</p>
          {#if displayUrl}<p><strong>Reference:</strong> {displayUrl}</p>{/if}
          {#if item?.content_hash}<p><strong>SHA-256:</strong> {item.content_hash}</p>{/if}
          {#if item?.text_content}
            <pre>{item.text_content.slice(0, 4000)}</pre>
          {:else}
            <p>This content is registered in the shared substrate. A dedicated reader/player can render it in Section 7.</p>
          {/if}
        </div>
      {/if}
    </div>

    {#if item?.provenance}
      <details class="provenance" data-content-provenance>
        <summary>Provenance</summary>
        <pre>{JSON.stringify(item.provenance, null, 2)}</pre>
      </details>
    {/if}
  {/if}
</section>

<style>
  .content-viewer {
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-height: 100%;
    padding: 22px;
    color: var(--choir-fg, #f5f7ff);
    background:
      radial-gradient(circle at 10% 0%, rgba(80, 145, 255, 0.16), transparent 32%),
      var(--choir-panel, #090b12);
    overflow: auto;
  }

  .content-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .eyebrow {
    margin: 0 0 6px;
    color: var(--choir-muted, #a8adbd);
    font-size: 0.78rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h2 {
    margin: 0;
    font-size: clamp(1.4rem, 3vw, 2.2rem);
  }

  .source-link {
    border: 1px solid rgba(99, 153, 255, 0.45);
    border-radius: 999px;
    padding: 9px 14px;
    color: #e7efff;
    text-decoration: none;
    background: rgba(19, 33, 58, 0.78);
  }

  .preview-shell {
    min-height: 320px;
    border: 1px solid var(--choir-border, rgba(120, 135, 170, 0.28));
    border-radius: 22px;
    background: rgba(6, 8, 16, 0.72);
    overflow: hidden;
  }

  img,
  video,
  audio,
  iframe {
    display: block;
    width: 100%;
  }

  img,
  video,
  iframe {
    min-height: 320px;
    height: min(68vh, 680px);
    object-fit: contain;
    border: 0;
  }

  audio {
    margin: 24px;
    width: calc(100% - 48px);
  }

  .podcast-list {
    display: grid;
    gap: 14px;
    padding: 18px;
  }

  .podcast-library {
    display: grid;
    gap: 16px;
    padding: 20px;
  }

  .library-header,
  .radio-panel {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    border: 1px solid rgba(120, 135, 170, 0.26);
    border-radius: 18px;
    padding: 16px;
    background: rgba(12, 17, 30, 0.86);
  }

  .library-header h3,
  .radio-panel h3 {
    margin: 0 0 6px;
  }

  .library-header p,
  .radio-panel p {
    margin: 0;
    color: var(--choir-muted, #a8adbd);
  }

  .podcast-import,
  .podcast-search {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 10px;
  }

  .podcast-import input,
  .podcast-search input {
    min-width: 0;
    border: 1px solid var(--choir-border, rgba(120, 135, 170, 0.28));
    border-radius: 12px;
    padding: 10px 12px;
    color: var(--choir-fg, #f5f7ff);
    background: rgba(255, 255, 255, 0.07);
  }

  button {
    border: 1px solid rgba(99, 153, 255, 0.45);
    border-radius: 12px;
    padding: 9px 13px;
    color: #e7efff;
    background: rgba(19, 33, 58, 0.78);
    cursor: pointer;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.56;
  }

  .library-list {
    display: grid;
    gap: 10px;
  }

  .library-item {
    display: grid;
    gap: 5px;
    width: 100%;
    text-align: left;
  }

  .library-item span,
  .podcast-search-result span,
  .path-meta {
    color: var(--choir-muted, #a8adbd);
    font-size: 0.88rem;
    overflow-wrap: anywhere;
  }

  .podcast-search-results {
    display: grid;
    gap: 10px;
  }

  .podcast-search-result {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 12px;
    align-items: center;
    border: 1px solid rgba(120, 135, 170, 0.26);
    border-radius: 14px;
    padding: 12px;
    background: rgba(12, 17, 30, 0.72);
  }

  .podcast-search-result div {
    display: grid;
    gap: 4px;
    min-width: 0;
  }

  .podcast-episode {
    display: grid;
    gap: 12px;
    border: 1px solid rgba(120, 135, 170, 0.26);
    border-radius: 18px;
    padding: 16px;
    background: rgba(12, 17, 30, 0.86);
  }

  .podcast-episode h3 {
    margin: 0 0 6px;
    font-size: 1.05rem;
  }

  .podcast-episode p {
    margin: 0;
    color: var(--choir-muted, #a8adbd);
  }

  .episode-date {
    font-size: 0.85rem;
  }

  .podcast-episode audio {
    margin: 0;
    width: 100%;
  }

  @media (max-width: 720px) {
    .library-header,
    .radio-panel,
    .podcast-import,
    .podcast-search,
    .podcast-search-result {
      grid-template-columns: 1fr;
    }

    .library-header,
    .radio-panel {
      display: grid;
    }
  }

  .metadata-card {
    padding: 22px;
  }

  pre {
    white-space: pre-wrap;
    word-break: break-word;
    color: #dce6ff;
  }

  .provenance {
    border: 1px solid var(--choir-border, rgba(120, 135, 170, 0.28));
    border-radius: 18px;
    padding: 12px 14px;
    background: rgba(12, 16, 28, 0.75);
  }

  .status,
  .error {
    border-radius: 16px;
    padding: 14px 16px;
    background: rgba(255, 255, 255, 0.06);
  }

  .error {
    color: #ffb8b8;
  }
</style>
