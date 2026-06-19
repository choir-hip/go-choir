<script context="module">
  // TextureVersionHistory renders the published version_history chain as a
  // collapsible "Version history" disclosure. A Texture IS its full versioned
  // history (D1-D5); this makes that legible to a /pub/texture/... reader
  // without displacing the head content. Option A (lineage disclosure) of
  // docs/texture-versioned-reader-ux-options-2026-06-19.md — a strict
  // prerequisite for the revision-browser (B) and diff (C) options.
  //
  // Read-only. Mutates nothing. Renders only when version_history is present.
</script>

<script>
  export let versionHistory = null;

  $: revisions = versionHistory?.revisions ?? [];
  $: revisionCount = versionHistory?.revision_count ?? revisions.length;
  $: chainHeadHash = versionHistory?.chain_head_hash ?? '';
  $: manifestHash = versionHistory?.manifest_hash ?? '';

  // The tamper-evident spine: the manifest's chain head must equal the head
  // revision's own hash. If it does, the chain is internally consistent.
  $: headEntry = revisions.length ? revisions[revisions.length - 1] : null;
  $: chainVerified =
    !!chainHeadHash && !!headEntry?.revision_hash && chainHeadHash === headEntry.revision_hash;

  function shortHash(hash) {
    if (!hash) return '—';
    return hash.length > 14 ? `${hash.slice(0, 12)}…` : hash;
  }

  function formatDate(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString(undefined, {
      year: 'numeric', month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit',
    });
  }

  // One-line grounding summary from typed provenance: authoring model + how
  // many research queries and collated sources back the revision.
  function provenanceSummary(prov) {
    if (!prov) return '';
    const model = prov.authoring_model?.model || prov.authoring_model?.provider;
    const queries = Array.isArray(prov.queries_executed) ? prov.queries_executed.length : 0;
    const sources = Array.isArray(prov.sources) ? prov.sources.length : 0;
    const parts = [];
    if (model) parts.push(model);
    if (queries) parts.push(`${queries} ${queries === 1 ? 'query' : 'queries'}`);
    if (sources) parts.push(`${sources} ${sources === 1 ? 'source' : 'sources'}`);
    return parts.join(' · ');
  }

  function revisionLabel(rev, index) {
    if (rev.version_number === 0 || index === 0) return 'Genesis prompt';
    return `Revision ${rev.version_number ?? index}`;
  }
</script>

{#if versionHistory && revisions.length}
  <details class="texture-version-history" data-texture-version-history>
    <summary>
      <span class="vh-title">Version history</span>
      <span class="vh-count">{revisionCount} {revisionCount === 1 ? 'revision' : 'revisions'}</span>
      {#if chainVerified}
        <span class="vh-verified" title="Chain head hash matches the head revision hash" data-chain-verified>chain verified</span>
      {/if}
    </summary>

    <dl class="vh-manifest">
      <div>
        <dt>Manifest hash</dt>
        <dd><code>{shortHash(manifestHash)}</code></dd>
      </div>
      <div>
        <dt>Chain head</dt>
        <dd><code>{shortHash(chainHeadHash)}</code></dd>
      </div>
    </dl>

    <ol class="vh-lineage" data-version-lineage>
      {#each revisions as rev, index (rev.revision_id)}
        <li class="vh-rev" data-version-number={rev.version_number ?? index}>
          <div class="vh-rev-head">
            <span class="vh-rev-label">{revisionLabel(rev, index)}</span>
            <span class="vh-rev-author">{rev.author_label || rev.author_kind || ''}</span>
            <span class="vh-rev-when">{formatDate(rev.created_at || rev.provenance?.authored_at)}</span>
          </div>
          {#if rev.provenance}
            <p class="vh-rev-prov">{provenanceSummary(rev.provenance)}</p>
          {/if}
          <p class="vh-rev-hash"><code>{shortHash(rev.revision_hash)}</code></p>
        </li>
      {/each}
    </ol>
  </details>
{/if}

<style>
  .texture-version-history {
    margin-top: 1.5rem;
    padding: 0.85rem 1rem;
    border: 1px solid var(--choir-border, #d8d4cc);
    border-radius: 8px;
    background: var(--choir-surface-raised, rgba(0, 0, 0, 0.02));
  }

  .texture-version-history > summary {
    cursor: pointer;
    list-style: none;
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.4rem 0.75rem;
    font-weight: 600;
    color: var(--choir-text-accent, #1f6feb);
  }

  .texture-version-history > summary::-webkit-details-marker { display: none; }

  .vh-title { font-size: 0.95rem; }

  .vh-count {
    font-size: 0.8rem;
    font-weight: 500;
    color: var(--choir-text-secondary, #5b5b5b);
  }

  .vh-verified {
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--choir-ok, #1a7f37);
    border: 1px solid var(--choir-ok, #1a7f37);
    border-radius: 999px;
    padding: 0.05rem 0.45rem;
  }

  .vh-manifest {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.3rem 1rem;
    margin: 0.75rem 0 0.5rem;
  }

  .vh-manifest dt {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--choir-text-secondary, #5b5b5b);
  }

  .vh-manifest dd {
    margin: 0;
    font-size: 0.8rem;
  }

  .vh-manifest code,
  .vh-rev-hash code {
    font-family: var(--choir-mono, ui-monospace, monospace);
    font-size: 0.78rem;
    color: var(--choir-text-secondary, #5b5b5b);
    word-break: break-all;
  }

  .vh-lineage {
    list-style: none;
    margin: 0.5rem 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }

  .vh-rev {
    padding: 0.4rem 0;
    border-top: 1px solid var(--choir-border, #ececec);
  }

  .vh-rev:first-child { border-top: none; }

  .vh-rev-head {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.25rem 0.75rem;
  }

  .vh-rev-label {
    font-weight: 600;
    font-size: 0.85rem;
  }

  .vh-rev-author {
    font-size: 0.78rem;
    color: var(--choir-text-secondary, #5b5b5b);
  }

  .vh-rev-when {
    font-size: 0.74rem;
    color: var(--choir-text-secondary, #5b5b5b);
    margin-left: auto;
  }

  .vh-rev-prov {
    margin: 0.15rem 0 0;
    font-size: 0.76rem;
    color: var(--choir-text-secondary, #5b5b5b);
  }

  .vh-rev-hash {
    margin: 0.1rem 0 0;
  }

  @media (max-width: 540px) {
    .vh-manifest { grid-template-columns: minmax(0, 1fr); }
    .vh-rev-when { margin-left: 0; }
  }
</style>
