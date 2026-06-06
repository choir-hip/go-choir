<script>
  import { onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';
  import { renderMarkdownBlocks } from './vtext-markdown-renderer';
  import { sourceEntityExcerptText, sourceEntityReaderSnapshotText } from './vtext-source-renderer';

  export let appContext = {};

  const dispatch = createEventDispatcher();

  let item = appContext?.contentItem || null;
  let loading = false;
  let error = '';

  $: sourceEntity = appContext?.sourceEntity || null;
  $: sourceEntityTarget = sourceEntity?.target || {};
  $: sourceEntityReaderSnapshot = sourceEntityReaderSnapshotText(sourceEntity);
  $: sourceEntityFallbackSnapshot = sourceEntityExcerptText(sourceEntity);
  $: sourceUrl = item?.source_url || appContext?.sourceUrl || '';
  $: filePath = item?.file_path || appContext?.filePath || '';
  $: mediaType = item?.media_type || appContext?.mediaType || '';
  $: appHint = item?.app_hint || appContext?.appHint || appContext?.appId || 'files';
  $: title = item?.title || sourceEntity?.label || appContext?.windowTitle || appContext?.title || appHint;
  $: isPublishedSourceReader = !!(appContext?.publishedRoutePath || appContext?.publishedGuest);
  $: displayUrl = filePath ? apiFileURL(filePath) : sourceUrl;
  $: embedUrl = mediaType === 'video/youtube' || /youtube\.com|youtu\.be/.test(sourceUrl)
    ? youtubeEmbedURL(sourceUrl)
    : '';
  $: readerText = String(
    isPublishedSourceReader
      ? sourceEntityReaderSnapshot || item?.text_content || sourceEntityFallbackSnapshot
      : item?.text_content || sourceEntityReaderSnapshot || sourceEntityFallbackSnapshot
  ).trim();
  $: readerHTML = renderMarkdownBlocks(readerText, [], { headingLevelOffset: 1, wrapTables: true });
  $: hasReaderText = readerText.length > 0;
  $: isSourceReader = hasReaderText && (appHint === 'content' || !!sourceEntity);
  $: sourceOpenPlan = appContext?.sourceOpenPlan || {};
  $: allowLiveImport = !!appContext?.allowLiveImport || !!sourceOpenPlan.liveOriginal;
  $: sourceState = sourceEntity?.evidence?.state || sourceEntity?.reader_snapshot_status?.state || item?.provenance?.state || '';

  async function loadContentItem() {
    const contentId = appContext?.contentId || appContext?.content_id || '';
    if (item || (!contentId && !sourceUrl)) return;
    if (sourceEntityReaderSnapshot) return;
    if (sourceEntity && !contentId && !allowLiveImport) return;
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

  onMount(() => {
    loadContentItem();
  });
</script>

<section class="content-viewer" class:source-reader-mode={isSourceReader} data-content-viewer data-content-app={appHint} data-source-reader-mode={isSourceReader ? 'true' : 'false'}>
  <header class="content-header">
    <div>
      {#if !isSourceReader}
        <p class="eyebrow">{appHint} content</p>
      {/if}
      <h2>{title}</h2>
      {#if isSourceReader && (sourceState || mediaType)}
        <p class="source-kicker">
          {#if sourceState}<span>{sourceState}</span>{/if}
          {#if mediaType}<span>{mediaType}</span>{/if}
        </p>
      {/if}
    </div>
    {#if sourceUrl}
      <a class="source-link" href={sourceUrl} target="_blank" rel="noreferrer">{isSourceReader ? 'Open original' : 'Open source'}</a>
    {/if}
  </header>

  {#if loading}
    <p class="status">Loading content metadata...</p>
  {:else if error}
    <p class="error" role="alert">{error}</p>
  {:else}
    <div class:preview-shell={!hasReaderText || appHint === 'image' || appHint === 'audio' || appHint === 'video' || appHint === 'pdf'} class:reader-shell={hasReaderText}>
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
      {:else if hasReaderText}
        <article class="source-reader" data-content-reader-markdown>
          {@html readerHTML}
        </article>
      {:else}
        <p class="empty-source">This content is registered in the shared substrate. A dedicated reader/player can render it when a cleaned source artifact is available.</p>
      {/if}
    </div>

    <aside class="source-apparatus" aria-label="Source details">
      <details class="provenance source-evidence" data-content-evidence>
        <summary>Source evidence</summary>
        <dl>
          <div>
            <dt>Media type</dt>
            <dd>{mediaType || 'unknown'}</dd>
          </div>
          {#if displayUrl}
            <div>
              <dt>Reference</dt>
              <dd>{displayUrl}</dd>
            </div>
          {/if}
          {#if item?.content_hash}
            <div>
              <dt>SHA-256</dt>
              <dd>{item.content_hash}</dd>
            </div>
          {/if}
        </dl>
      </details>

      {#if sourceEntity}
        <details class="provenance" data-source-entity>
          <summary>Source entity</summary>
          <p><strong>Entity:</strong> {appContext?.sourceEntityId || sourceEntity?.entity_id || sourceEntity?.source_entity_id || 'source'}</p>
          {#if appContext?.sourceServiceItemId || sourceEntityTarget?.item_id}
            <p><strong>Source item:</strong> {appContext?.sourceServiceItemId || sourceEntityTarget.item_id}</p>
          {/if}
          {#if sourceEntityTarget?.content_id}
            <p><strong>Content item:</strong> {sourceEntityTarget.content_id}</p>
          {/if}
          {#if sourceEntity?.evidence}
            <p><strong>Evidence:</strong> {sourceEntity.evidence.state || 'available'} / {sourceEntity.evidence.research_state || 'unclassified'}</p>
          {/if}
        </details>
      {/if}

      {#if item?.provenance}
        <details class="provenance" data-content-provenance>
          <summary>Provenance</summary>
          <pre>{JSON.stringify(item.provenance, null, 2)}</pre>
        </details>
      {/if}
    </aside>
  {/if}
</section>

<style>
  .content-viewer {
    display: flex;
    flex-direction: column;
    gap: 16px;
    min-height: 100%;
    padding: 22px;
    color: var(--choir-text-primary);
    background: var(--choir-surface-app);
    overflow: auto;
  }

  .content-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }

  .source-reader-mode .content-header {
    max-width: 76ch;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--choir-border);
  }

  .eyebrow {
    margin: 0 0 6px;
    color: var(--choir-text-muted);
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

  .source-reader-mode h2 {
    font-size: 1.35rem;
    line-height: 1.2;
  }

  .source-kicker {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin: 8px 0 0;
    color: var(--choir-text-muted);
    font-size: 0.82rem;
  }

  .source-link {
    color: var(--choir-text-accent);
    text-decoration: underline;
    text-underline-offset: 0.18em;
    font-weight: 700;
    white-space: nowrap;
  }

  .preview-shell {
    min-height: 320px;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-state-selected);
    overflow: hidden;
  }

  .reader-shell {
    display: flow-root;
    flex: none;
    min-height: 0;
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

  .source-reader {
    display: flow-root;
    max-width: 76ch;
    padding: 2px 0 4px;
    color: var(--choir-text-primary);
    font-size: 1rem;
    line-height: 1.7;
  }

  .source-reader-mode .source-reader {
    font-size: 1.02rem;
  }

  .source-reader :global(h2),
  .source-reader :global(h3),
  .source-reader :global(h4),
  .source-reader :global(h5) {
    margin: 1.25em 0 0.35em;
    color: var(--choir-text-accent);
    line-height: 1.18;
  }

  .source-reader :global(h2:first-child),
  .source-reader :global(h3:first-child) {
    margin-top: 0;
  }

  .source-reader :global(p) {
    margin: 0 0 1em;
  }

  .source-reader :global(blockquote) {
    margin: 1.1em 0;
    padding-left: 1em;
    border-left: 3px solid var(--choir-border-strong);
    color: var(--choir-text-secondary);
  }

  .source-reader :global(ul),
  .source-reader :global(ol) {
    margin: 0 0 1em 1.3em;
    padding: 0;
  }

  .source-reader :global(table) {
    width: max-content;
    min-width: 100%;
    border-collapse: collapse;
    margin: 1.1em 0;
    font-size: 0.95em;
  }

  .source-reader :global(.table-scroll) {
    display: block;
    max-width: 100%;
    overflow-x: auto;
  }

  .source-reader :global(th),
  .source-reader :global(td) {
    border-bottom: 1px solid var(--choir-border);
    padding: 0.45em 0.5em;
    text-align: left;
    vertical-align: top;
  }

  .source-reader :global(pre) {
    white-space: pre-wrap;
    word-break: break-word;
    padding: 12px;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-state-selected);
  }

  .empty-source {
    margin: 0;
    padding: 18px;
  }

  .source-apparatus {
    display: grid;
    flex: none;
    gap: 6px;
    max-width: 76ch;
    padding-top: 8px;
    border-top: 1px solid color-mix(in srgb, var(--choir-border) 70%, transparent);
  }

  .provenance {
    padding: 4px 0;
    overflow-wrap: anywhere;
    color: var(--choir-text-secondary);
    font-size: 0.88rem;
  }

  .provenance summary {
    cursor: pointer;
    color: var(--choir-text-accent);
    font-weight: 700;
  }

  .provenance p {
    margin: 8px 0 0;
  }

  .source-evidence dl {
    display: grid;
    gap: 10px;
    margin: 10px 0 0;
  }

  .source-evidence div {
    display: grid;
    gap: 2px;
  }

  .source-evidence dt {
    color: var(--choir-text-muted);
    font-size: 0.75rem;
    font-weight: 800;
    text-transform: uppercase;
  }

  .source-evidence dd {
    margin: 0;
  }

  .provenance pre {
    max-height: 220px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-word;
    margin: 10px 0 0;
    padding: 10px;
    border-left: 2px solid var(--choir-border);
    background: color-mix(in srgb, var(--choir-text-primary) 4%, transparent);
  }

  .status,
  .error {
    border-radius: 16px;
    padding: 14px 16px;
    background: color-mix(in srgb, var(--choir-text-primary) 6%, transparent);
  }

  .error {
    color: var(--choir-status-danger);
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
