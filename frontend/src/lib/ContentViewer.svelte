<script>
  import { onMount } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';
  import { createEventDispatcher } from 'svelte';

  export let appContext = {};

  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';

  $: sourceUrl = item?.source_url || appContext?.sourceUrl || '';
  $: filePath = item?.file_path || appContext?.filePath || '';
  $: mediaType = item?.media_type || appContext?.mediaType || '';
  $: appHint = item?.app_hint || appContext?.appHint || appContext?.appId || 'files';
  $: title = item?.title || appContext?.windowTitle || appContext?.title || appHint;

  async function loadContentItem() {
    const contentId = appContext?.contentId || appContext?.content_id || '';
    if (item) return;
    if (!contentId && !(appHint === 'podcast' && sourceUrl)) return;
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
  $: podcastEpisodes = appHint === 'podcast' && item?.text_content
    ? parsePodcastEpisodes(item.text_content)
    : [];

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

  function firstTagText(source, tagName) {
    const match = new RegExp(`<${tagName}(?:\\s[^>]*)?>([\\s\\S]*?)<\\/${tagName}>`, 'i').exec(source);
    return stripMarkup(match?.[1] || '');
  }

  function firstAttribute(source, tagName, attrName) {
    const tag = new RegExp(`<${tagName}\\b[^>]*>`, 'i').exec(source)?.[0] || '';
    const attr = new RegExp(`${attrName}=["']([^"']+)["']`, 'i').exec(tag);
    return stripMarkup(attr?.[1] || '');
  }

  function parsePodcastEpisodesLoosely(xmlText) {
    return Array.from(String(xmlText || '').matchAll(/<item\b[\s\S]*?<\/item>/gi))
      .slice(0, 24)
      .map((match) => {
        const source = match[0];
        return {
          title: firstTagText(source, 'title') || 'Untitled episode',
          description: firstTagText(source, 'description'),
          publishedAt: firstTagText(source, 'pubDate'),
          audioUrl: firstAttribute(source, 'enclosure', 'url') || firstAttribute(source, 'media:content', 'url'),
        };
      })
      .filter((episode) => episode.title || episode.audioUrl);
  }

  function parsePodcastEpisodes(xmlText) {
    try {
      const parsed = new DOMParser().parseFromString(xmlText, 'application/xml');
      if (parsed.querySelector('parsererror')) return parsePodcastEpisodesLoosely(xmlText);
      return Array.from(parsed.getElementsByTagName('item')).slice(0, 24).map((episode) => {
        const enclosure = episode.getElementsByTagName('enclosure')[0];
        const mediaContent = episode.getElementsByTagName('media:content')[0];
        return {
          title: textFromFirst(episode, 'title') || 'Untitled episode',
          description: textFromFirst(episode, 'description'),
          publishedAt: textFromFirst(episode, 'pubDate'),
          audioUrl: enclosure?.getAttribute('url') || mediaContent?.getAttribute('url') || '',
        };
      }).filter((episode) => episode.title || episode.audioUrl);
    } catch (err) {
      return parsePodcastEpisodesLoosely(xmlText);
    }
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
      {#if appHint === 'podcast' && podcastEpisodes.length > 0}
        <div class="podcast-list" data-podcast-feed>
          {#each podcastEpisodes as episode}
            <article class="podcast-episode" data-podcast-episode>
              <div>
                <h3>{episode.title}</h3>
                {#if episode.publishedAt}<p class="episode-date">{episode.publishedAt}</p>{/if}
                {#if episode.description}<p>{episode.description.replace(/<[^>]+>/g, '').slice(0, 420)}</p>{/if}
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
