<!--
  AutopaperStory — one story on the Autopaper front page.

  The headline is the link to the story's living page (/autopaper/<slug>,
  always its latest version). A quiet meta line says when it was last
  updated and how far it has developed, and links to the pinned version
  the front page saw (/autopaper/<slug>/v<n>). The tier sets the size: the
  lead (h2, with its dek), the top tier (h3 with dek), briefs and more
  stories (h3, headline only). The parent decides what a click opens.
-->
<script lang="ts">
  import {
    formatWhen,
    parsePublicationRoute,
    publicationRoute,
    storyPath,
    storyPathForRoute,
    type FrontStory,
  } from './autopaper';

  export let story: FrontStory;
  export let tier: 'lead' | 'top' | 'brief' | 'more' = 'top';
  export let rank = 0;
  /** Called on a headline or version click with the publication route to open. */
  export let onOpen: (event: MouseEvent, story: FrontStory, route: string) => void = () => {};

  $: bareRoute = story?.route?.bare || '';
  $: slug = parsePublicationRoute(bareRoute).slug;
  $: latest = Number(story?.latest_revision) || 0;
  $: versions = Number(story?.version_count) || 0;
  $: pinnedRoute = parsePublicationRoute(story?.route?.permalink || '').revision > 0
    ? story.route.permalink
    : slug && latest > 0 ? publicationRoute(slug, latest) : '';
  $: versionLabel = versions > 1 ? `${versions} versions` : `v${latest || 1}`;
  $: showDek = Boolean(story?.dek) && (tier === 'lead' || tier === 'top');
  $: isOpinion = story?.kind === 'opinion';
</script>

<article
  class="ap-story ap-tier-{tier}"
  class:ap-top-major={tier === 'top' && rank <= 2}
  data-autopaper-story-row
  data-autopaper-tier={tier}
  data-autopaper-rank={rank}
  data-version-count={versions}
  data-publication-route={bareRoute}
  data-autopaper-kind={isOpinion ? 'opinion' : undefined}
>
  <svelte:element this={tier === 'lead' ? 'h2' : 'h3'} class="ap-headline" data-autopaper-headline dir="auto">
    <a href={slug ? storyPath(slug) : '#'} on:click={(event) => onOpen(event, story, bareRoute)}>{story.title}</a>
  </svelte:element>
  {#if showDek}
    <p class="ap-dek" dir="auto">{story.dek}</p>
  {/if}
  <p class="ap-meta">
    {#if isOpinion}
      <span class="ap-opinion-label" data-autopaper-opinion-label>Opinion</span>
      {#if story.author}
        <span class="ap-byline" data-autopaper-byline>by {story.author}</span>
      {/if}
      <span class="ap-meta-dot" aria-hidden="true">·</span>
    {/if}
    {#if versions > 1}
      <span class="ap-updated" data-updated-marker>Updated</span>
    {/if}
    <time datetime={story.updated_at}>{formatWhen(story.updated_at)}</time>
    {#if pinnedRoute}
      <span class="ap-meta-dot" aria-hidden="true">·</span>
      <a
        class="ap-version"
        href={storyPathForRoute(pinnedRoute)}
        data-autopaper-version-link
        aria-label={`${versionLabel}: ${story.title}, version ${latest || 1}`}
        on:click={(event) => onOpen(event, story, pinnedRoute)}
      >{versionLabel}</a>
    {/if}
  </p>
</article>

<style>
  /* Type sizes step down by tier; the parent .autopaper is the size
     container, so a desktop window and the public page size alike. */
  .ap-story {
    min-width: 0;
    color: var(--ap-ink);
  }

  .ap-headline {
    margin: 0;
    font-family: var(--ap-serif);
    font-weight: 700;
    line-height: 1.14;
    letter-spacing: 0;
    overflow-wrap: anywhere;
    text-wrap: pretty;
  }

  .ap-headline a {
    color: inherit !important;
    text-decoration: none;
    border-radius: 2px;
  }

  .ap-headline a:hover {
    text-decoration: underline;
    text-decoration-thickness: 1px;
    text-underline-offset: 0.14em;
  }

  /* Focus is drawn inside the link (a tint and a thick underline), so the
     front grid, which clips at its edges, never cuts it off. */
  .ap-story a:focus-visible {
    outline: 2px solid transparent;
    background: var(--choir-state-focus);
    box-shadow: inset 0 -2px 0 var(--choir-accent);
    -webkit-box-decoration-break: clone;
    box-decoration-break: clone;
  }

  .ap-tier-lead .ap-headline {
    font-size: 2rem;
    line-height: 1.06;
    text-wrap: balance;
  }

  .ap-tier-top .ap-headline {
    font-size: 1.22rem;
  }

  .ap-top-major .ap-headline {
    font-size: 1.36rem;
  }

  .ap-tier-brief .ap-headline {
    font-size: 1.02rem;
    line-height: 1.22;
  }

  .ap-tier-more .ap-headline {
    font-size: 0.98rem;
    font-weight: 600;
    line-height: 1.25;
  }

  .ap-dek {
    margin: 0.5rem 0 0;
    color: var(--ap-ink-soft) !important;
    font-family: var(--ap-serif);
    font-size: 0.98rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .ap-tier-lead .ap-dek {
    margin-top: 0.75rem;
    font-size: 1.12rem;
    line-height: 1.5;
    max-width: 40em;
  }

  .ap-meta {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.1rem 0.35rem;
    margin: 0.5rem 0 0;
    color: var(--ap-ink-muted);
    font-family: var(--choir-font-ui, inherit);
    font-size: 0.72rem;
    letter-spacing: 0.02em;
    line-height: 1.4;
  }

  .ap-tier-brief .ap-meta,
  .ap-tier-more .ap-meta {
    margin-top: 0.3rem;
  }

  .ap-opinion-label {
    color: var(--ap-ink);
    font-size: 0.66rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .ap-updated {
    color: var(--ap-ink);
    font-size: 0.66rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .ap-version {
    color: inherit !important;
    text-decoration: underline;
    text-decoration-color: var(--ap-rule);
    text-underline-offset: 0.18em;
  }

  .ap-version:hover {
    color: var(--ap-ink) !important;
    text-decoration-color: currentColor;
  }

  @container autopaper (min-width: 641px) {
    .ap-tier-lead .ap-headline {
      font-size: 2.55rem;
    }

    .ap-tier-lead .ap-dek {
      font-size: 1.2rem;
    }

    .ap-top-major .ap-headline {
      font-size: 1.45rem;
    }
  }

  @container autopaper (min-width: 1100px) {
    .ap-tier-lead .ap-headline {
      font-size: 3.15rem;
      line-height: 1.04;
    }

    .ap-tier-lead .ap-dek {
      font-size: 1.3rem;
    }

    .ap-top-major .ap-headline {
      font-size: 1.5rem;
    }
  }
</style>
