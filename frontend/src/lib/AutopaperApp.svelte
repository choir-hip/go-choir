<!--
  AutopaperApp — the Autopaper reader (Cut 2 unit 2b, Reader W3a).

  Front page: one row per story, newest first, marked "updated" when the
  story has more than one version. Story page: the latest version in place at
  the bare link, or version n at /v<n>; numbered sources, each citation's
  quote marked inside its pinned source block with its live check badge
  (✓ exact, ≈ tolerant, pending, ✗ failed); transcluded blocks drawn as
  blocks; prev/next chevrons from nav and a "new version available" control.

  Standalone (the public /autopaper routes) it owns the URL through the
  History API; inside a desktop window it navigates in place. Both read only
  the public platform routes, logged out.
-->
<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import {
    AUTOPAPER_BASE,
    AutopaperNotFound,
    buildStoryView,
    checkLabel,
    checkTitle,
    fetchFrontPage,
    fetchStoryBundle,
    formatWhen,
    parseAutopaperPath,
    slugFromPublicRoute,
    splitAtQuote,
    storyPath,
    type AutopaperLocation,
    type FrontStory,
    type StoryView,
  } from './autopaper';

  /** True on the public /autopaper routes; false inside a desktop window. */
  export let standalone = false;
  /** The reader path to open first (defaults to the front page). */
  export let initialPath = '';
  export let appContext: any = {};

  const POLL_MS = 60_000;

  let root: HTMLElement;
  let location: AutopaperLocation = parseAutopaperPath(initialPath || appContext?.autopaperPath || AUTOPAPER_BASE);

  let stories: FrontStory[] = [];
  let nextCursor = '';
  let frontStatus: 'idle' | 'loading' | 'ready' | 'error' = 'idle';
  let loadingMore = false;

  let story: StoryView | null = null;
  let storyStatus: 'idle' | 'loading' | 'ready' | 'missing' | 'error' = 'idle';
  let liveLatest = 0;
  let loadSeq = 0;
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let flashAnchor = '';

  $: currentSlug = location.kind === 'story' ? location.slug : '';
  $: newestKnown = story ? Math.max(story.nav.latest, liveLatest) : 0;
  $: newVersionAvailable = !!story && newestKnown > story.revision;
  $: viewingBare = location.kind === 'story' && location.revision === 0;

  $: void load(location);

  async function load(loc: AutopaperLocation) {
    const seq = ++loadSeq;
    stopPolling();
    if (loc.kind === 'front') {
      story = null;
      await loadFront(seq);
      return;
    }
    if (loc.kind === 'story') {
      await loadStory(seq, loc.slug, loc.revision);
      return;
    }
    story = null;
    storyStatus = 'missing';
  }

  async function loadFront(seq: number) {
    frontStatus = 'loading';
    try {
      const page = await fetchFrontPage();
      if (seq !== loadSeq) return;
      stories = page.stories;
      nextCursor = page.next_cursor || '';
      frontStatus = 'ready';
    } catch (_err) {
      if (seq !== loadSeq) return;
      frontStatus = 'error';
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return;
    loadingMore = true;
    try {
      const page = await fetchFrontPage(nextCursor);
      const seen = new Set(stories.map((s) => s.publication_id));
      stories = [...stories, ...page.stories.filter((s) => !seen.has(s.publication_id))];
      nextCursor = page.next_cursor || '';
    } catch (_err) {
      // Keep what is shown; the button stays for a retry.
    } finally {
      loadingMore = false;
    }
  }

  async function loadStory(seq: number, slug: string, revision: number) {
    storyStatus = 'loading';
    liveLatest = 0;
    try {
      const bundle = await fetchStoryBundle(slug, revision);
      if (seq !== loadSeq) return;
      story = buildStoryView(bundle);
      storyStatus = 'ready';
      startPolling(seq, slug);
    } catch (err) {
      if (seq !== loadSeq) return;
      story = null;
      storyStatus = err instanceof AutopaperNotFound ? 'missing' : 'error';
    }
  }

  // The latest may change while a story is open: poll the bare link and
  // offer the newer version instead of swapping the text under the reader.
  function startPolling(seq: number, slug: string) {
    stopPolling();
    pollTimer = setInterval(async () => {
      if (seq !== loadSeq || (typeof document !== 'undefined' && document.visibilityState === 'hidden')) return;
      try {
        const bundle = await fetchStoryBundle(slug, 0);
        const latest = Number(bundle?.nav?.latest || bundle?.payload?.revision || 0);
        if (seq === loadSeq && latest > liveLatest) liveLatest = latest;
      } catch (_err) {
        // A failed poll changes nothing on the page.
      }
    }, POLL_MS);
  }

  function stopPolling() {
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = null;
  }

  async function go(path: string) {
    if (standalone && typeof window !== 'undefined') {
      if (window.location.pathname !== path) window.history.pushState({ autopaper: true }, '', path);
    }
    location = parseAutopaperPath(path);
    await tick();
    (document.activeElement as HTMLElement | null)?.blur?.();
    root?.scrollTo?.({ top: 0 });
  }

  function navigate(event: MouseEvent, path: string) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    void go(path);
  }

  function reloadLatest() {
    if (!currentSlug) return;
    if (viewingBare) {
      location = { kind: 'story', slug: currentSlug, revision: 0 };
    } else {
      void go(storyPath(currentSlug));
    }
  }

  // Citation markers jump to their quote inside the source block.
  function handleArticleClick(event: MouseEvent) {
    const target = (event.target as HTMLElement | null)?.closest?.('a.ap-cite') as HTMLAnchorElement | null;
    if (!target) return;
    event.preventDefault();
    const anchor = target.dataset.citeAnchor || '';
    const el = root?.querySelector(`[data-citation-anchor="${CSS.escape(anchor)}"]`);
    if (el) {
      el.scrollIntoView({ block: 'center', behavior: 'smooth' });
      flashAnchor = anchor;
      setTimeout(() => {
        if (flashAnchor === anchor) flashAnchor = '';
      }, 1600);
    }
  }

  // The markers are links inside {@html} prose, so the click is delegated
  // from the article.
  function citationLinks(node: HTMLElement) {
    node.addEventListener('click', handleArticleClick);
    return { destroy: () => node.removeEventListener('click', handleArticleClick) };
  }

  function handlePopState() {
    location = parseAutopaperPath(window.location.pathname);
  }

  onMount(() => {
    if (standalone) {
      window.addEventListener('popstate', handlePopState);
      document.title = 'Autopaper';
    }
  });

  onDestroy(() => {
    stopPolling();
    if (standalone && typeof window !== 'undefined') window.removeEventListener('popstate', handlePopState);
  });

  $: if (standalone && typeof document !== 'undefined') {
    document.title = story && location.kind === 'story' ? `${story.title} · Autopaper` : 'Autopaper';
  }
</script>

<div class="autopaper" class:standalone bind:this={root} data-autopaper data-autopaper-view={location.kind}>
  <header class="ap-header">
    <a class="ap-brand" href={AUTOPAPER_BASE} on:click={(e) => navigate(e, AUTOPAPER_BASE)} data-autopaper-home>Autopaper</a>
    {#if standalone}
      <nav class="ap-header-nav" aria-label="Choir">
        <a class="ap-header-link" href="/">Choir</a>
      </nav>
    {/if}
  </header>

  <main class="ap-main">
    {#if location.kind === 'front'}
      <section class="ap-front" data-autopaper-front aria-labelledby="ap-front-title">
        <h1 id="ap-front-title" class="ap-front-title">Latest stories</h1>
        {#if frontStatus === 'loading' && stories.length === 0}
          <p class="ap-status" data-autopaper-status>Loading stories…</p>
        {:else if frontStatus === 'error'}
          <p class="ap-status ap-error" data-autopaper-error>Could not load stories. Try again shortly.</p>
        {:else if stories.length === 0}
          <p class="ap-status" data-autopaper-empty>No stories are published yet.</p>
        {:else}
          <ol class="ap-rows">
            {#each stories as row (row.publication_id)}
              {@const slug = slugFromPublicRoute(row.route?.bare)}
              <li class="ap-row" data-autopaper-story-row data-version-count={row.version_count}>
                <a class="ap-row-link" href={storyPath(slug)} on:click={(e) => navigate(e, storyPath(slug))}>
                  <h2 class="ap-row-title">{row.title}</h2>
                  {#if row.dek}
                    <p class="ap-row-dek">{row.dek}</p>
                  {/if}
                  <p class="ap-row-meta">
                    {#if row.version_count > 1}
                      <span class="ap-updated" data-updated-marker>Updated</span>
                    {/if}
                    <time datetime={row.updated_at}>{formatWhen(row.updated_at)}</time>
                    <span aria-hidden="true">·</span>
                    <span>{row.version_count} {row.version_count === 1 ? 'version' : 'versions'}</span>
                  </p>
                </a>
              </li>
            {/each}
          </ol>
          {#if nextCursor}
            <button class="ap-more" type="button" on:click={loadMore} disabled={loadingMore} data-autopaper-more>
              {loadingMore ? 'Loading…' : 'More stories'}
            </button>
          {/if}
        {/if}
      </section>
    {:else if location.kind === 'story'}
      {#if storyStatus === 'loading' && !story}
        <p class="ap-status" data-autopaper-status>Loading story…</p>
      {:else if storyStatus === 'missing'}
        <p class="ap-status" data-autopaper-missing>This story or version is not published.</p>
        <p><a href={AUTOPAPER_BASE} on:click={(e) => navigate(e, AUTOPAPER_BASE)}>All stories</a></p>
      {:else if storyStatus === 'error'}
        <p class="ap-status ap-error" data-autopaper-error>Could not load this story. Try again shortly.</p>
      {:else if story}
        <article
          class="ap-story"
          data-autopaper-story
          data-revision={story.revision}
          data-latest-revision={story.nav.latest}
          data-manifest-hash={story.manifestHash}
          use:citationLinks
        >
          <header class="ap-story-head">
            <h1 class="ap-story-title">{story.title}</h1>
            <p class="ap-story-meta">
              <span data-story-version>Version {story.revision}</span>
              {#if story.publishedAt}
                <span aria-hidden="true">·</span>
                <time datetime={story.publishedAt}>{formatWhen(story.publishedAt)}</time>
              {/if}
            </p>
            {#if story.nav.prev || story.nav.next || story.nav.latest > 1}
              <nav class="ap-version-nav" aria-label="Story versions" data-version-nav>
                {#if story.nav.prev}
                  <a class="ap-chevron" href={storyPath(currentSlug, story.nav.prev)} on:click={(e) => navigate(e, storyPath(currentSlug, story.nav.prev))} rel="prev" aria-label="Previous version, v{story.nav.prev}" data-version-prev>‹ v{story.nav.prev}</a>
                {:else}
                  <span class="ap-chevron ap-chevron-off" aria-hidden="true">‹</span>
                {/if}
                <span class="ap-version-label">v{story.revision} of v{story.nav.latest}</span>
                {#if story.nav.next}
                  <a class="ap-chevron" href={storyPath(currentSlug, story.nav.next)} on:click={(e) => navigate(e, storyPath(currentSlug, story.nav.next))} rel="next" aria-label="Next version, v{story.nav.next}" data-version-next>v{story.nav.next} ›</a>
                {:else}
                  <span class="ap-chevron ap-chevron-off" aria-hidden="true">›</span>
                {/if}
              </nav>
            {/if}
            {#if newVersionAvailable}
              <div class="ap-new-version" role="status" data-new-version>
                <span>New version available: v{newestKnown}</span>
                <button type="button" on:click={reloadLatest} data-new-version-open>Read the latest</button>
              </div>
            {/if}
          </header>

          <div class="ap-body">
            {#each story.blocks as block}
              {#if block.type === 'heading'}
                {#if block.level === 2}<h2>{@html block.html}</h2>{:else if block.level === 3}<h3>{@html block.html}</h3>{:else}<h4>{@html block.html}</h4>{/if}
              {:else if block.type === 'p'}
                <p>{@html block.html}</p>
              {:else if block.type === 'list'}
                {#if block.ordered}
                  <ol start={block.start}>{#each block.items as item}<li>{@html item}</li>{/each}</ol>
                {:else}
                  <ul>{#each block.items as item}<li>{@html item}</li>{/each}</ul>
                {/if}
              {:else if block.type === 'quote'}
                <blockquote>{@html block.html}</blockquote>
              {:else if block.type === 'code'}
                <pre><code>{block.text}</code></pre>
              {:else if block.type === 'rule'}
                <hr />
              {:else if block.type === 'embed'}
                <figure class="ap-embed" data-embed-block data-citation-anchor={block.citation.anchorId}>
                  {#each block.citation.blocks as pinned (pinned.cid)}
                    <blockquote class="ap-embed-block" data-pinned-cid={pinned.cid}>{pinned.text}</blockquote>
                  {/each}
                  {#if block.more > 0}
                    <p class="ap-embed-more">{block.more} more {block.more === 1 ? 'block' : 'blocks'} in the source</p>
                  {/if}
                  <figcaption>
                    <span class="ap-source-num">[{block.citation.number}]</span>
                    {#if block.source?.url}
                      <a href={block.source.url} rel="noopener noreferrer" target="_blank">{block.source.title}</a>
                    {:else}
                      {block.source?.title || 'Source'}
                    {/if}
                    {#if block.citation.quote}
                      <span class="ap-badge" data-check-badge={block.citation.check} title={checkTitle(block.citation)}>{checkLabel(block.citation.check)}</span>
                    {/if}
                  </figcaption>
                </figure>
              {/if}
            {/each}
          </div>

          {#if story.sources.length}
            <section class="ap-sources" aria-labelledby="ap-sources-title" data-autopaper-sources>
              <h2 id="ap-sources-title">Sources</h2>
              <ol class="ap-source-list">
                {#each story.sources as source (source.key)}
                  <li class="ap-source" id="source-{source.number}" data-source-block data-source-number={source.number}>
                    <div class="ap-source-head">
                      <span class="ap-source-num">[{source.number}]</span>
                      <span class="ap-source-title">
                        {#if source.url}
                          <a href={source.url} rel="noopener noreferrer" target="_blank">{source.title}</a>
                        {:else}
                          {source.title}
                        {/if}
                      </span>
                      {#if source.host && source.host !== source.title}
                        <span class="ap-source-host">{source.host}</span>
                      {/if}
                    </div>
                    {#each source.citations as citation (citation.anchorId)}
                      <div
                        class="ap-citation"
                        class:ap-flash={flashAnchor === citation.anchorId}
                        data-citation-anchor={citation.anchorId}
                        data-citation-check={citation.check}
                      >
                        {#if citation.blocks.length}
                          {#each citation.blocks as pinned (pinned.cid)}
                            {@const split = splitAtQuote(pinned.text, citation.quote)}
                            <blockquote class="ap-pinned" data-pinned-cid={pinned.cid}>
                              {#if split}
                                {split.before}<mark class="ap-quote" data-citation-quote>{split.quote}</mark>{split.after}
                              {:else}
                                {#if citation.quote}<mark class="ap-quote ap-quote-standalone" data-citation-quote>“{citation.quote}”</mark>{/if}
                                <span class="ap-pinned-text">{pinned.text}</span>
                              {/if}
                            </blockquote>
                          {/each}
                        {:else if citation.quote}
                          <blockquote class="ap-pinned"><mark class="ap-quote" data-citation-quote>“{citation.quote}”</mark></blockquote>
                        {/if}
                        <p class="ap-citation-check">
                          <span class="ap-badge" data-check-badge={citation.check} title={checkTitle(citation)}>{checkLabel(citation.check)}</span>
                          {#if citation.judgement === 'supported'}
                            <span class="ap-judgement">supports the sentence</span>
                          {:else if citation.judgement === 'unsupported'}
                            <span class="ap-judgement ap-judgement-failed">does not support the sentence</span>
                          {/if}
                        </p>
                      </div>
                    {/each}
                  </li>
                {/each}
              </ol>
            </section>
          {/if}

          <footer class="ap-story-foot">
            <a href={AUTOPAPER_BASE} on:click={(e) => navigate(e, AUTOPAPER_BASE)}>All stories</a>
            {#if story.permalink}
              <a href={storyPath(currentSlug, story.revision)} on:click={(e) => navigate(e, storyPath(currentSlug, story.revision))} data-story-permalink>Permanent link to v{story.revision}</a>
            {/if}
          </footer>
        </article>
      {/if}
    {:else}
      <p class="ap-status" data-autopaper-missing>Nothing is published here.</p>
      <p><a href={AUTOPAPER_BASE} on:click={(e) => navigate(e, AUTOPAPER_BASE)}>All stories</a></p>
    {/if}
  </main>
</div>

<style>
  .autopaper {
    width: 100%;
    height: 100%;
    min-height: 100%;
    overflow-x: hidden;
    overflow-y: auto;
    background: var(--choir-bg);
    color: var(--choir-text-primary, var(--choir-fg));
    font-family: var(--choir-font-ui, inherit);
  }

  .ap-header {
    position: sticky;
    top: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem clamp(1rem, 4vw, 2.5rem);
    border-bottom: 1px solid var(--choir-border);
    background: color-mix(in srgb, var(--choir-bg) 92%, transparent);
    backdrop-filter: blur(12px);
  }

  .ap-brand {
    color: var(--choir-fg);
    font-family: var(--choir-font-display, inherit);
    font-size: 1rem;
    font-weight: 780;
    text-decoration: none;
    letter-spacing: 0.01em;
  }

  .ap-header-link {
    min-height: 2rem;
    display: inline-flex;
    align-items: center;
    padding: 0.35rem 0.65rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font-size: 0.78rem;
    font-weight: 700;
    text-decoration: none;
  }

  .ap-brand:focus-visible,
  .ap-header-link:hover,
  .ap-header-link:focus-visible {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-hover);
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .ap-main {
    width: min(760px, calc(100% - 2rem));
    margin: 1rem auto 2.5rem;
  }

  .ap-status {
    padding: 1rem 0;
    color: var(--choir-text-muted);
  }

  .ap-error {
    color: var(--choir-status-danger);
  }

  .ap-front-title {
    margin: 0.5rem 0 1rem;
    font-family: var(--choir-font-display, inherit);
    font-size: clamp(1.4rem, 3vw, 2rem);
    line-height: 1.1;
  }

  .ap-rows {
    list-style: none;
    display: grid;
    gap: 0.75rem;
  }

  .ap-row-link {
    display: block;
    padding: clamp(0.85rem, 2.5vw, 1.25rem);
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
    color: inherit;
    text-decoration: none;
  }

  .ap-row-link:hover,
  .ap-row-link:focus-visible {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-hover);
    outline: none;
  }

  .ap-row-link:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .ap-row-title {
    font-family: var(--choir-font-display, inherit);
    font-size: clamp(1.1rem, 2.4vw, 1.35rem);
    line-height: 1.2;
    overflow-wrap: anywhere;
  }

  .ap-row-dek {
    margin-top: 0.4rem;
    color: var(--choir-text-primary);
    font-size: 1rem;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }

  .ap-row-meta,
  .ap-story-meta {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.5rem;
    color: var(--choir-text-muted);
    font-size: 0.78rem;
  }

  .ap-updated {
    padding: 0.1rem 0.45rem;
    border-radius: 999px;
    background: var(--choir-status-warning-soft, var(--choir-surface-control));
    color: var(--choir-text-primary);
    font-weight: 700;
  }

  .ap-more {
    margin-top: 1rem;
    min-height: 2.5rem;
    padding: 0.5rem 1rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    cursor: pointer;
  }

  .ap-story {
    padding: clamp(1rem, 3vw, 2rem);
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
    box-shadow: var(--choir-window-shadow);
  }

  .ap-story-title {
    font-family: var(--choir-font-display, inherit);
    font-size: clamp(1.55rem, 4vw, 2.4rem);
    line-height: 1.1;
    overflow-wrap: anywhere;
  }

  .ap-version-nav {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-top: 0.85rem;
    padding: 0.35rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card, var(--choir-surface-control));
    font-size: 0.85rem;
  }

  .ap-chevron {
    min-height: 2.25rem;
    min-width: 2.75rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem 0.6rem;
    border-radius: 6px;
    color: var(--choir-text-accent, var(--choir-accent));
    font-weight: 700;
    text-decoration: none;
  }

  a.ap-chevron:hover,
  a.ap-chevron:focus-visible {
    background: var(--choir-state-hover);
    outline: 2px solid var(--choir-accent);
    outline-offset: 1px;
  }

  .ap-chevron-off {
    color: var(--choir-text-subtle, var(--choir-text-muted));
  }

  .ap-version-label {
    color: var(--choir-text-muted);
  }

  .ap-new-version {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    margin-top: 0.75rem;
    padding: 0.6rem 0.75rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-status-warning-soft, var(--choir-surface-control));
    font-size: 0.9rem;
  }

  .ap-new-version button {
    min-height: 2.25rem;
    padding: 0.35rem 0.8rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    font-weight: 700;
    cursor: pointer;
  }

  .ap-body {
    margin-top: 1.25rem;
  }

  .ap-body :global(p),
  .ap-body :global(li) {
    font-size: 1.05rem;
    line-height: 1.62;
    overflow-wrap: anywhere;
  }

  .ap-body :global(p) {
    margin: 0.75rem 0;
  }

  .ap-body :global(h2),
  .ap-body :global(h3),
  .ap-body :global(h4) {
    margin: 1.4rem 0 0.5rem;
    line-height: 1.2;
    overflow-wrap: anywhere;
  }

  .ap-body :global(ul),
  .ap-body :global(ol) {
    margin: 0.75rem 0;
    padding-left: 1.4rem;
  }

  .ap-body :global(blockquote) {
    margin: 0.9rem 0;
    padding-left: 0.9rem;
    border-left: 3px solid var(--choir-border-strong);
    color: var(--choir-text-primary);
  }

  .ap-body :global(pre) {
    max-width: 100%;
    overflow-x: auto;
    padding: 0.75rem;
    border-radius: 6px;
    background: var(--choir-surface-card, var(--choir-surface-control));
    font-family: var(--choir-font-mono, monospace);
    font-size: 0.85rem;
  }

  .ap-body :global(a) {
    color: var(--choir-text-accent, var(--choir-accent));
    overflow-wrap: anywhere;
  }

  .ap-body :global(a.ap-cite) {
    margin-left: 0.1rem;
    font-size: 0.78em;
    font-weight: 700;
    text-decoration: none;
    vertical-align: super;
    line-height: 0;
  }

  .ap-embed {
    margin: 1rem 0;
    padding: 0.75rem 0.9rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card, var(--choir-surface-control));
  }

  .ap-embed .ap-embed-block {
    margin: 0 0 0.6rem;
    font-size: 1rem;
    line-height: 1.55;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .ap-embed-more {
    color: var(--choir-text-muted);
    font-size: 0.85rem;
  }

  .ap-embed figcaption {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    color: var(--choir-text-muted);
    font-size: 0.82rem;
    overflow-wrap: anywhere;
  }

  .ap-sources {
    margin-top: 2rem;
    padding-top: 1rem;
    border-top: 1px solid var(--choir-border);
  }

  .ap-sources h2 {
    font-size: 1.05rem;
    margin-bottom: 0.75rem;
  }

  .ap-source-list {
    list-style: none;
    display: grid;
    gap: 0.85rem;
  }

  .ap-source {
    padding: 0.8rem 0.9rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card, var(--choir-surface-control));
    min-width: 0;
  }

  .ap-source-head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.4rem;
    font-size: 0.9rem;
  }

  .ap-source-num {
    color: var(--choir-text-accent, var(--choir-accent));
    font-weight: 780;
  }

  .ap-source-title {
    font-weight: 700;
    overflow-wrap: anywhere;
    min-width: 0;
  }

  .ap-source-title a,
  .ap-embed figcaption a {
    color: var(--choir-text-primary);
  }

  .ap-source-host {
    color: var(--choir-text-muted);
    font-size: 0.78rem;
  }

  .ap-citation {
    margin-top: 0.65rem;
    border-radius: 6px;
    transition: box-shadow var(--choir-motion-sheet, 0.2s ease);
  }

  .ap-citation.ap-flash {
    box-shadow: 0 0 0 2px var(--choir-accent);
  }

  .ap-pinned {
    margin: 0;
    padding: 0.5rem 0.7rem;
    border-left: 3px solid var(--choir-border-strong);
    color: var(--choir-text-muted);
    font-size: 0.92rem;
    line-height: 1.5;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .ap-quote {
    padding: 0 0.1rem;
    border-radius: 3px;
    background: color-mix(in srgb, var(--choir-accent) 22%, transparent);
    color: var(--choir-text-primary);
  }

  .ap-quote-standalone {
    display: block;
    margin-bottom: 0.35rem;
  }

  .ap-citation-check {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.45rem;
    margin-top: 0.4rem;
    font-size: 0.78rem;
  }

  .ap-badge {
    display: inline-flex;
    align-items: center;
    padding: 0.1rem 0.5rem;
    border: 1px solid var(--choir-border);
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-text-muted);
    font-size: 0.75rem;
    font-weight: 700;
    white-space: nowrap;
  }

  .ap-badge[data-check-badge='exact'] {
    border-color: var(--choir-status-success);
    background: var(--choir-status-success-soft, var(--choir-surface-control));
    color: var(--choir-text-primary);
  }

  .ap-badge[data-check-badge='tolerant'] {
    border-color: var(--choir-status-warning);
    background: var(--choir-status-warning-soft, var(--choir-surface-control));
    color: var(--choir-text-primary);
  }

  .ap-badge[data-check-badge='failed'] {
    border-color: var(--choir-status-danger);
    background: var(--choir-status-danger-soft, var(--choir-surface-control));
    color: var(--choir-text-primary);
  }

  .ap-judgement {
    color: var(--choir-text-muted);
  }

  .ap-judgement-failed {
    color: var(--choir-status-danger);
  }

  .ap-story-foot {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 0.75rem;
    margin-top: 1.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--choir-border);
    font-size: 0.85rem;
  }

  .ap-story-foot a,
  .ap-main > p a {
    color: var(--choir-text-accent, var(--choir-accent));
  }

  @media (prefers-reduced-motion: reduce) {
    .ap-citation {
      transition-duration: 1ms;
    }
  }
</style>
