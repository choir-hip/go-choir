<!--
  The changelog app: the owner's events newest first (autopaper contracts 5.2,
  lane E). Major events by default; Show all adds the minor ones. Events newer
  than the seen cursor are marked. The cursor lives on the owner's computer.
  Verdict reasons are untrusted text: rendered as text, never as markup.
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { openApp as shellOpenApp } from '../../lib/app-sdk';
  import { eventTarget, fetchChangelog, formatWhen, kindLabel, markChangelogSeen } from './changelog.js';

  const AUTO_SEEN_MS = 4000;

  let events: any[] = [];
  let nextBefore = 0;
  let head = 0;
  let seenSeq = 0;
  let highlightAfter = 0;
  let unseenMajor = 0;
  let unseenMore = false;
  let showAll = false;
  let loading = false;
  let error = '';
  let seenTimer: ReturnType<typeof setTimeout> | undefined;

  async function load(reset: boolean) {
    loading = true;
    error = '';
    try {
      const page = await fetchChangelog({ minor: showAll, before: reset ? 0 : nextBefore });
      events = reset ? page.events : [...events, ...page.events];
      nextBefore = page.next_before || 0;
      head = page.head || 0;
      seenSeq = page.seen_seq || 0;
      unseenMajor = page.unseen_major || 0;
      unseenMore = !!page.unseen_more;
      if (reset) {
        highlightAfter = seenSeq;
        scheduleSeen();
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Could not load the changelog';
    } finally {
      loading = false;
    }
  }

  function scheduleSeen() {
    clearTimeout(seenTimer);
    if (head > seenSeq) seenTimer = setTimeout(markSeen, AUTO_SEEN_MS);
  }

  async function markSeen() {
    clearTimeout(seenTimer);
    if (head <= seenSeq) return;
    try {
      const out = await markChangelogSeen(head);
      seenSeq = out.seen_seq ?? head;
      unseenMajor = 0;
      unseenMore = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Could not mark the changelog seen';
    }
  }

  function toggleZoom() {
    showAll = !showAll;
    void load(true);
  }

  function openDocument(target: { docId: string; revisionId: string }) {
    shellOpenApp('texture', {
      appName: 'Texture',
      icon: '📝',
      appContext: { docId: target.docId, initialRevisionId: target.revisionId || undefined },
    });
  }

  onMount(() => { void load(true); });
  onDestroy(() => clearTimeout(seenTimer));
</script>

<div class="changelog" data-changelog-app>
  <header class="bar">
    <div>
      <h1>Changelog</h1>
      {#if unseenMajor > 0}
        <p class="unseen" data-changelog-unseen>{unseenMajor}{unseenMore ? '+' : ''} new</p>
      {:else}
        <p class="quiet">{showAll ? 'All events' : 'Major events'}</p>
      {/if}
    </div>
    <div class="actions">
      <button type="button" on:click={toggleZoom} aria-pressed={showAll} data-changelog-zoom>
        {showAll ? 'Major only' : 'Show all'}
      </button>
      <button type="button" on:click={markSeen} disabled={head <= seenSeq} data-changelog-mark-seen>Mark seen</button>
    </div>
  </header>

  {#if error}
    <div class="notice" role="alert">{error}</div>
  {/if}

  <ol class="list" data-changelog-list>
    {#each events as ev (ev.inbox_seq)}
      {@const c = ev.changelog}
      {@const target = eventTarget(c)}
      <li class="event" class:new={ev.inbox_seq > highlightAfter} class:minor={ev.level === 'minor'} data-changelog-event={c.kind}>
        <div class="meta">
          <span class="kind">{kindLabel(c.kind)}</span>
          {#if ev.level === 'minor'}<span class="tag">minor</span>{/if}
          {#if ev.inbox_seq > highlightAfter}<span class="tag new-tag">new</span>{/if}
          <time class="when" datetime={c.occurred_at}>{formatWhen(c.occurred_at)}</time>
        </div>
        <p class="summary">{c.summary}</p>
        {#if c.actor_label}<p class="actor">From {c.actor_label}</p>{/if}
        {#if c.reason}
          <p class="reason"><span>Reviewer's reason (untrusted text):</span> {c.reason}</p>
        {/if}
        {#if target?.kind === 'document'}
          <button type="button" class="link" on:click={() => openDocument(target)} data-changelog-open>Open the document</button>
        {:else if target?.kind === 'route'}
          <a class="link" href={target.route} target="_blank" rel="noopener noreferrer" data-changelog-open>Open</a>
        {/if}
      </li>
    {:else}
      {#if !loading && !error}
        <li class="empty">Nothing here yet. Events about your work appear when they happen.</li>
      {/if}
    {/each}
  </ol>

  {#if nextBefore > 0}
    <button type="button" class="more" on:click={() => load(false)} disabled={loading} data-changelog-more>
      {loading ? 'Loading...' : 'Older events'}
    </button>
  {/if}
</div>

<style>
  .changelog { display: flex; flex-direction: column; gap: 0.75rem; padding: 1rem; box-sizing: border-box; min-height: 100%; color: var(--choir-text-primary); background: var(--choir-bg); }
  .bar { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 0.75rem; }
  h1 { margin: 0; font-size: 1.15rem; }
  .quiet, .unseen { margin: 0.15rem 0 0; font-size: 0.85rem; color: var(--choir-text-muted); }
  .unseen { color: var(--choir-status-warning); font-weight: 600; }
  .actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
  button { min-height: 44px; padding: 0 0.9rem; border: 1px solid var(--choir-border-strong); border-radius: 8px; background: var(--choir-surface-control); color: var(--choir-text-primary); font: inherit; cursor: pointer; }
  button:disabled { opacity: 0.5; cursor: default; }
  .notice { padding: 0.6rem 0.8rem; border: 1px solid var(--choir-status-danger); border-radius: 8px; color: var(--choir-status-danger); }
  .list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.6rem; }
  .event { padding: 0.75rem 0.9rem; border: 1px solid var(--choir-border); border-radius: 10px; background: var(--choir-surface-card); overflow-wrap: anywhere; }
  .event.new { border-color: var(--choir-status-warning); }
  .event.minor { opacity: 0.85; }
  .meta { display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.5rem; font-size: 0.8rem; color: var(--choir-text-muted); }
  .kind { font-weight: 600; color: var(--choir-text-primary); }
  .tag { padding: 0 0.4rem; border: 1px solid var(--choir-border); border-radius: 999px; font-size: 0.7rem; }
  .new-tag { color: var(--choir-status-warning); border-color: var(--choir-status-warning); }
  .when { margin-left: auto; }
  .summary { margin: 0.35rem 0 0; line-height: 1.4; }
  .actor, .reason { margin: 0.3rem 0 0; font-size: 0.85rem; color: var(--choir-text-muted); }
  .reason span { font-style: italic; }
  .link { display: inline-flex; align-items: center; margin-top: 0.5rem; color: var(--choir-text-primary); text-decoration: underline; }
  a.link { min-height: 44px; }
  button.link { border: 0; background: none; padding: 0; text-align: left; }
  .empty { padding: 1.5rem 0; text-align: center; color: var(--choir-text-muted); }
  .more { align-self: center; }
</style>
