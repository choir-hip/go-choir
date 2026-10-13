<script context="module">
  // TextureVersionHistory shows where a published Texture version sits among
  // its publication's versions, from the bundle's live nav (Cut 2 unit 2b):
  // the version number, the published versions before and after it, and the
  // latest, with links to their permanent /v<n> routes. The per-revision
  // version_history chain it used to draw was removed from the bundle; each
  // version's manifest now names the previous manifest hash instead.
  //
  // Read-only. Mutates nothing. Renders only when nav is present.
</script>

<script>
  /** The bundle's nav: { revision, prev, next, latest }. */
  export let nav = null;
  /** The bundle's route: { bare, permalink }. */
  export let route = null;
  /** The served version's manifest hash. */
  export let manifestHash = '';

  $: revision = Number(nav?.revision || 0);
  $: prev = Number(nav?.prev || 0);
  $: next = Number(nav?.next || 0);
  $: latest = Number(nav?.latest || 0);
  $: bare = String(route?.bare || '');

  function versionLink(n) {
    return bare ? `${bare.replace(/\/+$/, '')}/v${n}` : '';
  }

  function shortHash(hash) {
    if (!hash) return '';
    return hash.length > 14 ? `${hash.slice(0, 12)}…` : hash;
  }
</script>

{#if revision > 0}
  <nav class="texture-version-history" aria-label="Published versions" data-texture-version-history data-version-revision={revision} data-version-latest={latest}>
    {#if prev && bare}
      <a class="vh-step" href={versionLink(prev)} rel="prev" data-version-prev>‹ v{prev}</a>
    {:else}
      <span class="vh-step vh-step-off" aria-hidden="true">‹</span>
    {/if}
    <span class="vh-title">Version {revision}{latest > revision ? ` of ${latest}` : ''}</span>
    {#if next && bare}
      <a class="vh-step" href={versionLink(next)} rel="next" data-version-next>v{next} ›</a>
    {:else}
      <span class="vh-step vh-step-off" aria-hidden="true">›</span>
    {/if}
    {#if latest > revision && bare}
      <a class="vh-latest" href={bare} data-version-latest-link>New version available: v{latest}</a>
    {/if}
    {#if manifestHash}
      <code class="vh-hash" title="Manifest hash of this version">{shortHash(manifestHash)}</code>
    {/if}
  </nav>
{/if}

<style>
  .texture-version-history {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem 0.75rem;
    margin-top: 1.5rem;
    padding: 0.6rem 0.85rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card, var(--choir-surface-control));
    font-size: 0.85rem;
  }

  .vh-title {
    font-weight: 600;
    color: var(--choir-text-primary);
  }

  .vh-step {
    min-width: 2.5rem;
    min-height: 2rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0.2rem 0.5rem;
    border-radius: 6px;
    color: var(--choir-text-accent, var(--choir-accent));
    font-weight: 600;
    text-decoration: none;
  }

  a.vh-step:hover,
  a.vh-step:focus-visible,
  .vh-latest:focus-visible {
    background: var(--choir-state-hover);
    outline: 2px solid var(--choir-accent);
    outline-offset: 1px;
  }

  .vh-step-off {
    color: var(--choir-text-subtle, var(--choir-text-muted));
  }

  .vh-latest {
    color: var(--choir-text-accent, var(--choir-accent));
    font-weight: 600;
  }

  .vh-hash {
    margin-left: auto;
    font-family: var(--choir-font-mono, ui-monospace, monospace);
    font-size: 0.75rem;
    color: var(--choir-text-muted);
    word-break: break-all;
  }
</style>
