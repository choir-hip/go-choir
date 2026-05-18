<script>
  import { onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';

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
  $: displayUrl = filePath ? apiFileURL(filePath) : sourceUrl;
  $: embedUrl = mediaType === 'video/youtube' || /youtube\.com|youtu\.be/.test(sourceUrl)
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
      {#if appHint === 'image' && displayUrl}
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
    overflow-wrap: anywhere;
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
    color: #ffd6d6;
  }

  @media (max-width: 720px) {
    .content-viewer {
      padding: 12px;
    }

    .content-header {
      display: grid;
    }
  }
</style>
