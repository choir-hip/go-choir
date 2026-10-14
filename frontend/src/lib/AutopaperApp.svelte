<!--
  AutopaperApp — the Autopaper front page, laid out like a broadsheet's.

  A masthead (the nameplate over a double rule, then the edition line: the
  reader's date, the tagline, when the paper last moved), then the stories
  in front-page order (rankFrontStories in autopaper.ts): the lead with the
  largest headline and its dek, the top tier with deks, briefs as headlines
  only, and the rest as a compact list of more stories, with older stories
  paged in below. Columns follow the width of this component, not of the
  screen, so the same layout serves the public /autopaper page and a
  desktop window: one column on a phone, two on a tablet, four when wide.

  A story is not read here: it opens in the Texture app's read-only
  published mode, the same Texture UI as a user's own documents. On the
  desktop that is a Texture window; on the public /autopaper page it is the
  standalone story page (the parent page owns the URL).

  Reads only the public front list route, logged out.
-->
<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import AutopaperStory from './AutopaperStory.svelte';
  import {
    editionTime,
    fetchFrontPage,
    formatWhen,
    layoutFrontPage,
    rankFrontStories,
    storyTextureContext,
    type FrontStory,
  } from './autopaper';

  /** True on the public /autopaper page; false inside a desktop window. */
  export let standalone = false;
  export let windowId = '';

  /** The Autopaper tagline, as the app registry describes it. */
  const TAGLINE = 'Living stories with checked sources';

  const dispatch = createEventDispatcher();

  // The first page is the front page; older pages extend "More stories" in
  // the order the server returns them, so paging never reshuffles the front.
  let frontStories: FrontStory[] = [];
  let olderStories: FrontStory[] = [];
  let nextCursor = '';
  let status: 'loading' | 'ready' | 'error' = 'loading';
  let loadingMore = false;
  let today = new Date();

  $: layout = layoutFrontPage(rankFrontStories(frontStories));
  $: moreStories = [...layout.more, ...olderStories];
  $: storyCount = frontStories.length + olderStories.length;
  $: edition = editionTime(frontStories);
  $: editionISO = Number.isFinite(edition) ? new Date(edition).toISOString() : '';
  $: todayLabel = today.toLocaleDateString(undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' });
  $: todayISO = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`;

  async function loadFront() {
    status = 'loading';
    try {
      const page = await fetchFrontPage();
      frontStories = page.stories;
      olderStories = [];
      nextCursor = page.next_cursor || '';
      today = new Date();
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
      const seen = new Set([...frontStories, ...olderStories].map((s) => s.publication_id));
      olderStories = [...olderStories, ...page.stories.filter((s) => !seen.has(s.publication_id))];
      nextCursor = page.next_cursor || '';
    } catch (_err) {
      // Keep what is shown; the button stays for a retry.
    } finally {
      loadingMore = false;
    }
  }

  function openStory(event: MouseEvent, row: FrontStory, route: string) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    if (!route) return;
    event.preventDefault();
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
  <div class="ap-sheet">
    <header class="ap-masthead" data-autopaper-masthead>
      {#if $$slots.ear}
        <div class="ap-ear"><slot name="ear" /></div>
      {/if}
      <h1 class="ap-nameplate">Autopaper</h1>
      <p class="ap-edition" data-autopaper-edition>
        <time class="ap-edition-date" datetime={todayISO}>{todayLabel}</time>
        <span class="ap-tagline">{TAGLINE}</span>
        {#if editionISO}
          <span class="ap-edition-updated" data-autopaper-updated>
            Updated <time datetime={editionISO}>{formatWhen(editionISO)}</time>
          </span>
        {/if}
      </p>
    </header>

    <section class="ap-front" data-autopaper-front aria-label="Front page" aria-busy={status === 'loading'}>
      {#if status === 'loading' && storyCount === 0}
        <p class="ap-status" role="status" data-autopaper-status>Loading the front page…</p>
      {:else if status === 'error'}
        <p class="ap-status ap-error" role="alert" data-autopaper-error>Could not load stories. Try again shortly.</p>
      {:else if storyCount === 0}
        <p class="ap-status" data-autopaper-empty>No stories yet.</p>
      {:else}
        <div class="ap-grid" data-autopaper-grid>
          {#if layout.lead}
            <div class="ap-cell ap-cell-lead" data-autopaper-lead>
              <AutopaperStory story={layout.lead} tier="lead" rank={0} onOpen={openStory} />
            </div>
          {/if}
          {#each layout.top as story, i (story.publication_id)}
            <div class="ap-cell ap-cell-top">
              <AutopaperStory {story} tier="top" rank={i + 1} onOpen={openStory} />
            </div>
          {/each}
          {#if layout.briefs.length}
            <section class="ap-cell ap-cell-briefs" aria-label="In brief" data-autopaper-briefs>
              <p class="ap-label" aria-hidden="true">In brief</p>
              <ul class="ap-briefs">
                {#each layout.briefs as story, i (story.publication_id)}
                  <li>
                    <AutopaperStory {story} tier="brief" rank={1 + layout.top.length + i} onOpen={openStory} />
                  </li>
                {/each}
              </ul>
            </section>
          {/if}
        </div>

        {#if moreStories.length || nextCursor}
          <section class="ap-more-section" aria-label="More stories" data-autopaper-more-stories>
            <p class="ap-label" aria-hidden="true">More stories</p>
            {#if moreStories.length}
              <ol class="ap-more-list">
                {#each moreStories as story, i (story.publication_id)}
                  <li>
                    <AutopaperStory
                      {story}
                      tier="more"
                      rank={1 + layout.top.length + layout.briefs.length + i}
                      onOpen={openStory}
                    />
                  </li>
                {/each}
              </ol>
            {/if}
            {#if nextCursor}
              <button class="ap-older" type="button" on:click={loadMore} disabled={loadingMore} data-autopaper-more>
                {loadingMore ? 'Loading…' : 'Older stories'}
              </button>
            {/if}
          </section>
        {/if}
      {/if}
    </section>
  </div>
</div>

<style>
  /* Paper and ink map to the theme's document surface and text, so the
     page is ink on paper in a light theme and inverts with a dark one.
     Inside a desktop window the shell recolors every p and a under
     .app-content with !important; the few !important colors here keep the
     window and the public page the same paper. */
  .autopaper {
    --ap-paper: var(--choir-surface-document, var(--choir-bg));
    --ap-ink: var(--choir-text-primary, var(--choir-fg));
    --ap-ink-soft: color-mix(in srgb, var(--ap-ink) 84%, var(--ap-paper));
    --ap-ink-muted: var(--choir-text-muted, var(--ap-ink));
    --ap-rule: color-mix(in srgb, var(--ap-ink) 22%, transparent);
    --ap-serif: 'Iowan Old Style', 'Charter', 'Bitstream Charter', 'Sitka Text', Cambria, Georgia, 'Times New Roman', serif;
    container: autopaper / inline-size;
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    background: var(--ap-paper);
    color: var(--ap-ink);
    font-family: var(--choir-font-ui, inherit);
  }

  .ap-sheet {
    --ap-gap-x: 1.75rem;
    --ap-gap-y: 1.5rem;
    width: 100%;
    max-width: 1240px;
    margin: 0 auto;
    padding: 0 16px 3rem;
  }

  /* ---- masthead ---- */

  .ap-masthead {
    padding-top: 1.1rem;
    text-align: center;
  }

  .ap-ear {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 0.25rem;
  }

  .ap-nameplate {
    margin: 0;
    padding: 0.2rem 0 0.7rem;
    border-bottom: 3px double var(--ap-ink);
    color: var(--ap-ink);
    font-family: var(--ap-serif);
    font-size: clamp(2.6rem, 13vw, 5.4rem);
    font-size: clamp(2.6rem, 13cqi, 5.4rem);
    font-weight: 700;
    line-height: 1;
    letter-spacing: -0.01em;
  }

  .ap-edition {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    align-items: baseline;
    gap: 0.15rem 1rem;
    margin: 0;
    padding: 0.5rem 0;
    border-bottom: 1px solid var(--ap-rule);
    color: var(--ap-ink-muted);
    font-size: 0.74rem;
    letter-spacing: 0.04em;
    line-height: 1.4;
  }

  .ap-edition-date {
    color: var(--ap-ink);
    font-weight: 600;
  }

  .ap-tagline {
    font-family: var(--ap-serif);
    font-size: 0.86rem;
    font-style: italic;
    letter-spacing: 0;
  }

  /* ---- states ---- */

  .ap-status {
    margin: 0;
    padding: 2.5rem 0;
    color: var(--ap-ink-muted);
    font-family: var(--ap-serif);
    font-size: 1.1rem;
    font-style: italic;
    text-align: center;
  }

  .ap-error {
    color: var(--choir-status-danger, var(--ap-ink)) !important;
  }

  /* ---- the front grid ----
     Each cell draws a hairline to its left and above it, half a gap out.
     The grid clips at its own edge, so first-column and first-row rules
     fall outside and only the rules between columns and rows show, like a
     printed page. A few pixels of vertical padding keep tall glyphs in the
     first row unclipped; story links mark focus inside their own box. */

  .ap-front {
    padding-top: 1.25rem;
  }

  .ap-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    grid-auto-flow: row dense;
    gap: var(--ap-gap-y) var(--ap-gap-x);
    margin: -4px 0;
    padding: 4px 0;
    overflow: hidden;
  }

  .ap-cell {
    position: relative;
    min-width: 0;
  }

  .ap-cell::before,
  .ap-cell::after {
    content: '';
    position: absolute;
    pointer-events: none;
  }

  .ap-cell::before {
    top: 0;
    bottom: 0;
    left: calc(var(--ap-gap-x) / -2);
    border-left: 1px solid var(--ap-rule);
  }

  .ap-cell::after {
    top: calc(var(--ap-gap-y) / -2);
    left: calc(var(--ap-gap-x) / -2);
    right: calc(var(--ap-gap-x) / -2);
    border-top: 1px solid var(--ap-rule);
  }

  .ap-cell-lead {
    padding-bottom: 0.25rem;
  }

  .ap-label {
    margin: 0 0 0.6rem;
    padding-top: 0.35rem;
    border-top: 2px solid var(--ap-ink);
    color: var(--ap-ink) !important;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.14em;
    text-transform: uppercase;
  }

  .ap-briefs,
  .ap-more-list {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .ap-briefs li,
  .ap-more-list li {
    break-inside: avoid;
    padding: 0.6rem 0;
    border-bottom: 1px solid var(--ap-rule);
  }

  .ap-briefs li:first-child,
  .ap-more-list li:first-child {
    padding-top: 0.1rem;
  }

  .ap-briefs {
    column-gap: var(--ap-gap-x);
    column-rule: 1px solid var(--ap-rule);
  }

  /* ---- more stories ---- */

  .ap-more-section {
    margin-top: 2.25rem;
  }

  .ap-more-list {
    column-gap: var(--ap-gap-x);
    column-rule: 1px solid var(--ap-rule);
  }

  .ap-older {
    display: block;
    min-height: 2.5rem;
    margin: 1.5rem auto 0;
    padding: 0.5rem 1.4rem;
    border: 1px solid var(--ap-ink);
    border-radius: 0;
    background: transparent;
    color: var(--ap-ink);
    font: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    cursor: pointer;
  }

  .ap-older:hover {
    background: var(--ap-ink);
    color: var(--ap-paper);
  }

  .ap-older:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 3px;
  }

  .ap-older:disabled {
    cursor: progress;
    opacity: 0.7;
  }

  /* ---- tablet: two columns ---- */

  @container autopaper (min-width: 641px) {
    .ap-sheet {
      padding-inline: clamp(1.5rem, 4cqi, 2.5rem);
    }

    .ap-edition {
      justify-content: space-between;
    }

    .ap-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .ap-cell-lead,
    .ap-cell-briefs {
      grid-column: span 2;
    }

    .ap-briefs,
    .ap-more-list {
      columns: 2;
    }
  }

  /* ---- wide: four columns. The first row is the lead over two columns
     with the first two top stories beside it, one column each, so the row
     is balanced; the other two top stories and the briefs fill the next. */

  @container autopaper (min-width: 1100px) {
    .ap-sheet {
      --ap-gap-x: 2rem;
    }

    .ap-grid {
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }

    .ap-cell-lead,
    .ap-cell-briefs {
      grid-column: span 2;
    }

    .ap-more-list {
      columns: 3;
    }
  }
</style>
