<!--
  AutopaperRoute — the public, logged-out /autopaper pages.

  /autopaper is the front page (AutopaperApp), which carries its own
  masthead, so the slim header bar shows only on story pages. /autopaper/<slug> and
  /autopaper/<slug>/v<n> are a story read in the Texture app's published
  mode, standalone: the same Texture published view a desktop window shows,
  without the desktop around it. This shell owns the URL through the
  History API: opening a story or moving between versions pushes the
  matching /autopaper path, and back/forward return to it.
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import AutopaperApp from './AutopaperApp.svelte';
  import AutopaperExpand from './AutopaperExpand.svelte';
  import {
    AUTOPAPER_BASE,
    parseAutopaperPath,
    publicationRoute,
    storyPathForRoute,
    storyTextureContext,
    type AutopaperLocation,
  } from './autopaper';

  export let initialPath = AUTOPAPER_BASE;

  let location: AutopaperLocation = parseAutopaperPath(initialPath);
  let storyTitle = '';
  let storyContext: Record<string, unknown> | null = null;
  let TextureEditor = null;

  $: storyContext = location.kind === 'story'
    ? storyTextureContext(publicationRoute(location.slug, location.revision), storyTitle)
    : null;
  $: if (location.kind === 'story' && !TextureEditor) {
    void import('./TextureEditor.svelte').then((module) => {
      TextureEditor = module.default;
    });
  }
  $: if (typeof document !== 'undefined') {
    document.title = location.kind === 'story' && storyTitle ? `${storyTitle} · Autopaper` : 'Autopaper';
  }

  function go(path: string) {
    if (window.location.pathname !== path) window.history.pushState({ autopaper: true }, '', path);
    location = parseAutopaperPath(path);
  }

  function home(event: MouseEvent) {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    storyTitle = '';
    go(AUTOPAPER_BASE);
  }

  function handleOpenStory(event: CustomEvent) {
    storyTitle = event.detail?.title || '';
    go(storyPathForRoute(event.detail?.route || ''));
  }

  // The Texture reports its window context: the story title once loaded, and
  // the new route when the reader moves to another version.
  function handleStoryContext(event: CustomEvent) {
    const ctx = event.detail?.appContext || {};
    if (event.detail?.title) storyTitle = event.detail.title;
    const path = storyPathForRoute(String(ctx.publishedRoutePath || ''));
    if (location.kind === 'story' && path !== window.location.pathname) {
      window.history.pushState({ autopaper: true }, '', path);
      // The shell's own location follows the reader, so the context it
      // hands back to the editor names the story and version on screen. A
      // reader that moves to another story (a related link) would
      // otherwise be sent back to the story the page was opened on.
      location = parseAutopaperPath(path);
    }
  }

  function handlePopState() {
    location = parseAutopaperPath(window.location.pathname);
  }

  onMount(() => window.addEventListener('popstate', handlePopState));
  onDestroy(() => {
    if (typeof window !== 'undefined') window.removeEventListener('popstate', handlePopState);
  });
</script>

<div class="autopaper-route" data-autopaper-route data-autopaper-view={location.kind}>
  {#if location.kind !== 'front'}
    <header class="ap-header">
      <a class="ap-brand" href={AUTOPAPER_BASE} on:click={home} data-autopaper-home>Autopaper</a>
      {#if location.kind === 'story' && storyTitle}
        <span class="ap-story-title" data-autopaper-story-title>{storyTitle}</span>
      {/if}
      {#if location.kind === 'story'}
        <AutopaperExpand route={publicationRoute(location.slug)} />
      {/if}
      <a class="ap-header-link" href="/">Choir</a>
    </header>
  {/if}

  <main class="ap-main">
    {#if location.kind === 'front'}
      <AutopaperApp standalone on:openstory={handleOpenStory}>
        <a slot="ear" class="ap-header-link" href="/">Choir</a>
      </AutopaperApp>
    {:else if location.kind === 'story' && storyContext}
      {#if TextureEditor}
        <div class="ap-texture" data-texture-app data-autopaper-story>
          <svelte:component
            this={TextureEditor}
            standalone
            authenticated={false}
            currentUser={null}
            appContext={storyContext}
            on:contextchange={handleStoryContext}
          />
        </div>
      {:else}
        <p class="ap-status" role="status">Loading story…</p>
      {/if}
    {:else}
      <p class="ap-status" data-autopaper-missing>Nothing is published here.</p>
      <p class="ap-status"><a href={AUTOPAPER_BASE} on:click={home}>All stories</a></p>
    {/if}
  </main>
</div>

<style>
  .autopaper-route {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--choir-bg);
    color: var(--choir-text-primary, var(--choir-fg));
    font-family: var(--choir-font-ui, inherit);
  }

  .ap-header {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.75rem;
    min-width: 0;
    padding: 0.7rem clamp(1rem, 4vw, 2.5rem);
    border-bottom: 1px solid var(--choir-border);
    background: color-mix(in srgb, var(--choir-bg) 92%, transparent);
  }

  .ap-brand {
    flex: 0 0 auto;
    color: var(--choir-fg);
    font-family: var(--choir-font-display, inherit);
    font-size: 1rem;
    font-weight: 780;
    text-decoration: none;
    letter-spacing: 0.01em;
  }

  .ap-story-title {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    color: var(--choir-text-muted);
    font-size: 0.85rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ap-header-link {
    flex: 0 0 auto;
    margin-left: auto;
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
    background: var(--choir-state-hover);
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .ap-main {
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;
  }

  .ap-texture {
    width: min(960px, 100%);
    height: 100%;
    margin: 0 auto;
  }

  .ap-status {
    padding: 1rem clamp(1rem, 4vw, 2.5rem);
    color: var(--choir-text-muted);
  }

  .ap-status a {
    color: var(--choir-text-accent, var(--choir-accent));
  }
</style>
