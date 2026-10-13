<!--
  AutopaperApp — the Autopaper front list.

  One row per story, newest first: title, dek, an "Updated" marker when the
  story has more than one version, when it was updated, and its version
  count. A story is not read here: it opens in the Texture app's read-only
  published mode, the same Texture UI as a user's own documents. On the
  desktop that is a Texture window; on the public /autopaper page it is the
  standalone story page (the parent page owns the URL).

  Reads only the public front list route, logged out.
-->
<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    fetchFrontPage,
    formatWhen,
    parsePublicationRoute,
    storyPath,
    storyTextureContext,
    type FrontStory,
  } from './autopaper';

  /** True on the public /autopaper page; false inside a desktop window. */
  export let standalone = false;
  export let windowId = '';

  const dispatch = createEventDispatcher();

  let stories: FrontStory[] = [];
  let nextCursor = '';
  let status: 'loading' | 'ready' | 'error' = 'loading';
  let loadingMore = false;

  async function loadFront() {
    status = 'loading';
    try {
      const page = await fetchFrontPage();
      stories = page.stories;
      nextCursor = page.next_cursor || '';
      status = 'ready';
    } catch (_err) {
      status = 'error';
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

  function openStory(event: MouseEvent, row: FrontStory) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    const route = row.route?.bare || '';
    if (standalone) {
      dispatch('openstory', { route, title: row.title });
      return;
    }
    dispatch('launchapp', {
      appId: 'texture',
      appName: 'Texture',
      icon: '📝',
      appContext: storyTextureContext(route, row.title),
    });
  }

  onMount(() => {
    void loadFront();
  });
</script>

<div class="autopaper" class:standalone data-autopaper data-autopaper-window-id={windowId || undefined}>
  <section class="ap-front" data-autopaper-front aria-labelledby="ap-front-title">
    <h1 id="ap-front-title" class="ap-front-title">Latest stories</h1>
    {#if status === 'loading' && stories.length === 0}
      <p class="ap-status" data-autopaper-status>Loading stories…</p>
    {:else if status === 'error'}
      <p class="ap-status ap-error" data-autopaper-error>Could not load stories. Try again shortly.</p>
    {:else if stories.length === 0}
      <p class="ap-status" data-autopaper-empty>No stories are published yet.</p>
    {:else}
      <ol class="ap-rows">
        {#each stories as row (row.publication_id)}
          <li class="ap-row" data-autopaper-story-row data-version-count={row.version_count} data-publication-route={row.route?.bare}>
            <a class="ap-row-link" href={storyPath(parsePublicationRoute(row.route?.bare).slug)} on:click={(e) => openStory(e, row)}>
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
</div>

<style>
  .autopaper {
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    background: var(--choir-bg);
    color: var(--choir-text-primary, var(--choir-fg));
    font-family: var(--choir-font-ui, inherit);
  }

  .ap-front {
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

  .ap-row-meta {
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
</style>
