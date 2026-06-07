<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let publishResult = null;
  export let publishedProposal = null;
  export let publicURL = '';

  const dispatch = createEventDispatcher();

  function handleLinkClick(event) {
    event.preventDefault();
    dispatch('open-public');
  }
</script>

{#if publishResult}
  <section
    class="publication-panel publication-result"
    data-vtext-publish-result
    data-publication-id={publishResult.publication_id || ''}
    data-publication-version-id={publishResult.publication_version_id || ''}
    data-public-route={publishResult.route_path || ''}
    data-public-url={publicURL}
  >
    <div class="publication-heading">
      <p class="eyebrow">Published</p>
      <a class="public-link" data-vtext-public-link href={publicURL} on:click={handleLinkClick}>
        {publicURL || 'Public route ready'}
      </a>
    </div>
    <div class="publication-actions">
      <button type="button" class="primary-action" data-vtext-copy-public on:click={() => dispatch('copy-public')}>
        Copy link
      </button>
      <button type="button" class="secondary-action" data-vtext-open-public on:click={() => dispatch('open-public')}>
        Open link
      </button>
      <button type="button" class="secondary-action" data-vtext-copy-full-text on:click={() => dispatch('copy-full-text')}>
        Copy text
      </button>
      <details class="download-menu" data-vtext-download-menu>
        <summary>Download</summary>
        <button type="button" data-vtext-download-md on:click={() => dispatch('download', 'md')}>Markdown</button>
        <button type="button" data-vtext-download-txt on:click={() => dispatch('download', 'txt')}>Text</button>
        <button type="button" data-vtext-download-html on:click={() => dispatch('download', 'html')}>HTML</button>
        <button type="button" data-vtext-download-docx on:click={() => dispatch('download', 'docx')}>DOCX</button>
        <button type="button" data-vtext-download-pdf on:click={() => dispatch('download', 'pdf')}>PDF</button>
      </details>
    </div>
  </section>
{/if}

{#if publishedProposal}
  <section
    class="publication-panel publication-result"
    data-vtext-proposal-result
    data-proposal-id={publishedProposal.proposal_id || ''}
    data-proposal-state={publishedProposal.state || ''}
    data-delivery-state={publishedProposal.delivery_state || ''}
  >
    <div class="publication-heading">
      <p class="eyebrow">Proposal</p>
      <h2>Proposal sent to author</h2>
    </div>
    <div class="publication-facts">
      <span>Your private version is ready for review.</span>
    </div>
  </section>
{/if}

<style>
  .publication-panel {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.65rem;
    align-items: center;
    padding: 0.58rem 0.78rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-status-success-soft);
    border-bottom-color: var(--choir-status-success);
  }

  .publication-heading {
    min-width: 0;
  }

  .publication-heading h2 {
    margin: 0;
    color: var(--choir-text-accent);
    font-size: 1rem;
    line-height: 1.24;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .public-link {
    display: block;
    max-width: min(28rem, 100%);
    color: var(--choir-text-accent);
    font-size: 0.84rem;
    font-weight: 720;
    line-height: 1.2;
    overflow: hidden;
    text-overflow: ellipsis;
    text-decoration: none;
    white-space: nowrap;
  }

  .public-link:hover,
  .public-link:focus-visible {
    color: var(--choir-text-accent);
    text-decoration: underline;
  }

  .publication-actions,
  .publication-facts {
    display: flex;
    flex-wrap: nowrap;
    gap: 0.42rem;
    align-items: center;
    min-width: 0;
  }

  .publication-actions .primary-action {
    background: var(--choir-accent);
    border-color: var(--choir-accent);
    color: var(--choir-text-on-accent);
  }

  .primary-action,
  .secondary-action,
  .download-menu > summary,
  .download-menu button {
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    backdrop-filter: blur(10px);
    transition: transform 120ms ease, background 120ms ease, border-color 120ms ease;
  }

  .primary-action {
    border-radius: 999px;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .secondary-action {
    border-radius: 999px;
    min-width: 5.8rem;
    height: 2.1rem;
    padding: 0.62rem 0.84rem;
    font-size: 0.78rem;
    font-weight: 720;
    color: var(--choir-text-accent);
    line-height: 1;
    white-space: nowrap;
  }

  .download-menu {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .download-menu > summary {
    list-style: none;
    border-radius: 999px;
    padding: 0.62rem 0.84rem;
    font-size: 0.78rem;
    font-weight: 720;
    color: var(--choir-text-accent);
  }

  .download-menu > summary::-webkit-details-marker {
    display: none;
  }

  .download-menu[open] > summary {
    background: var(--choir-accent-soft);
    border-color: var(--choir-accent);
  }

  .download-menu[open] {
    z-index: 4;
  }

  .download-menu[open]::after {
    content: '';
    position: fixed;
    inset: 0;
    z-index: -1;
  }

  .download-menu button {
    display: block;
    width: 100%;
    border-radius: 0;
    border-width: 0;
    border-bottom: 1px solid var(--choir-border);
    background: transparent;
    color: var(--choir-text-primary);
    padding: 0.58rem 0.7rem;
    text-align: left;
    font-size: 0.76rem;
    font-weight: 680;
  }

  .download-menu button:last-child {
    border-bottom: 0;
  }

  .download-menu[open] button {
    min-width: 8rem;
  }

  .download-menu[open] > button,
  .download-menu[open] > :global(button) {
    display: block;
  }

  .download-menu[open] {
    flex-direction: column;
    align-items: stretch;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.65rem;
    background: var(--choir-surface-elevated);
    box-shadow: var(--choir-shadow-lg);
    overflow: hidden;
  }

  .primary-action:hover:enabled,
  .secondary-action:hover:enabled {
    transform: translateY(-1px);
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
  }

  .primary-action:disabled,
  .secondary-action:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  @media (max-width: 768px) {
    .publication-panel {
      grid-template-columns: minmax(0, 1fr);
      gap: 0.5rem;
      padding: 0.62rem 0.7rem;
    }

    .publication-heading h2 {
      font-size: 0.92rem;
    }

    .publication-facts {
      justify-content: flex-start;
    }

    .secondary-action {
      padding: 0.5rem 0.64rem;
      font-size: 0.72rem;
    }
  }
</style>
