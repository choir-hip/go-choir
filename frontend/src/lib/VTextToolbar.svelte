<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let toolbarHidden = false;
  export let versionLabel = '';
  export let previousVersionLabel = '';
  export let nextRevisionLabel = '';
  export let previousDisabled = true;
  export let nextDisabled = true;
  export let revisionLineLabel = '';
  export let stateLabel = '';
  export let isPublishedReader = false;
  export let isPublishedMode = false;
  export let isViewingHistorical = false;
  export let hasMergePreview = false;
  export let hasCompareResult = false;
  export let currentUser = null;
  export let loading = false;
  export let submitting = false;
  export let agentPending = false;
  export let cancelPending = false;
  export let comparePending = false;
  export let mergePending = false;
  export let restorePending = false;
  export let publishedActionPending = false;
  export let publishMenuOpen = false;
  export let promptLabel = 'Revise';
  export let sourceCandidateCount = 0;
  export let selectedMergeSuggestionCount = 0;
  export let hasCurrentDoc = false;
  export let hasCurrentRevision = false;

  const dispatch = createEventDispatcher();

  $: navDisabled = loading || submitting;
  $: previousAriaLabel = previousVersionLabel ? `Older version (${previousVersionLabel})` : 'At oldest version';
  $: nextAriaLabel = nextRevisionLabel ? `Newer version (${nextRevisionLabel})` : 'At latest version';
  $: previousTitle = previousVersionLabel ? `Go to ${previousVersionLabel}` : 'At oldest version';
  $: nextTitle = nextRevisionLabel ? `Go to ${nextRevisionLabel}` : 'At latest version';
  $: publishDisabled = loading || submitting || agentPending || hasMergePreview || publishedActionPending || !hasCurrentDoc;
</script>

<div class="doc-toolbar" class:toolbar-hidden={toolbarHidden} data-vtext-toolbar>
  <div class="version-controls">
    <span class="nav-version" data-vtext-version>{versionLabel}</span>
    <button
      class="nav-btn"
      data-vtext-prev
      aria-label={previousAriaLabel}
      title={previousTitle}
      on:click={() => dispatch('prev')}
      disabled={navDisabled || previousDisabled}
    >
      &lt;
    </button>
    <button
      class="nav-btn"
      data-vtext-next
      aria-label={nextAriaLabel}
      title={nextTitle}
      on:click={() => dispatch('next')}
      disabled={navDisabled || nextDisabled}
    >
      &gt;
    </button>
    <span class="draft-line" data-vtext-draft-line>{revisionLineLabel}</span>
  </div>

  <div class="doc-state" data-vtext-state>{stateLabel}</div>

  <div class="doc-actions">
    {#if isPublishedReader}
      <button
        class="secondary-action"
        data-vtext-copy-full-text
        on:click={() => dispatch('copy-full-text')}
        disabled={loading || publishedActionPending}
      >
        Copy text
      </button>
      <details class="download-menu" data-vtext-download-menu>
        <summary>Download</summary>
        <button type="button" data-vtext-download-md on:click={() => dispatch('download', 'md')} disabled={loading || publishedActionPending}>Markdown</button>
        <button type="button" data-vtext-download-txt on:click={() => dispatch('download', 'txt')} disabled={loading || publishedActionPending}>Text</button>
        <button type="button" data-vtext-download-html on:click={() => dispatch('download', 'html')} disabled={loading || publishedActionPending}>HTML</button>
        <button type="button" data-vtext-download-docx on:click={() => dispatch('download', 'docx')} disabled={loading || publishedActionPending}>DOCX</button>
        <button type="button" data-vtext-download-pdf on:click={() => dispatch('download', 'pdf')} disabled={loading || publishedActionPending}>PDF</button>
      </details>
      <button
        class="prompt-btn"
        data-vtext-edit-published
        on:click={() => dispatch('edit-published')}
        disabled={loading || publishedActionPending}
      >
        {publishedActionPending ? 'Opening…' : currentUser ? 'Edit my version' : 'Edit'}
      </button>
    {:else}
      <button
        class="prompt-btn"
        data-vtext-prompt
        data-vtext-save
        on:click={() => dispatch('prompt')}
        disabled={loading || submitting || agentPending || isViewingHistorical || publishedActionPending}
      >
        {promptLabel}
      </button>
      {#if agentPending}
        <button
          class="secondary-action danger"
          data-vtext-cancel-revision
          on:click={() => dispatch('cancel-revision')}
          disabled={cancelPending}
        >
          {cancelPending ? 'Cancelling…' : 'Cancel'}
        </button>
      {/if}
      {#if isPublishedMode}
        <button
          class="secondary-action"
          data-vtext-submit-proposal
          on:click={() => dispatch('submit-proposal')}
          disabled={loading || submitting || agentPending || publishedActionPending || !hasCurrentDoc}
        >
          {publishedActionPending ? 'Submitting…' : 'Propose'}
        </button>
      {:else}
        {#if hasMergePreview}
          <button
            class="prompt-btn"
            data-vtext-accept-merge
            on:click={() => dispatch('accept-merge')}
            disabled={mergePending || publishedActionPending || !hasCurrentDoc}
          >
            {mergePending ? 'Accepting…' : 'Accept'}
          </button>
          <button
            class="secondary-action danger"
            data-vtext-discard-merge
            on:click={() => dispatch('discard-merge')}
            disabled={mergePending || publishedActionPending}
          >
            Discard
          </button>
        {:else}
          <button
            class="secondary-action"
            data-vtext-compare
            on:click={() => dispatch('compare')}
            disabled={loading || submitting || agentPending || comparePending || nextDisabled}
          >
            {comparePending ? 'Comparing…' : 'Compare'}
          </button>
          <button
            class="secondary-action"
            data-vtext-source-panel
            on:click={() => dispatch('sources')}
            disabled={loading || submitting || agentPending || !hasCurrentDoc}
          >
            Sources{sourceCandidateCount ? ` ${sourceCandidateCount}` : ''}
          </button>
          {#if isViewingHistorical}
            <button
              class="secondary-action"
              data-vtext-restore-version
              on:click={() => dispatch('restore')}
              disabled={loading || submitting || agentPending || restorePending || !hasCurrentRevision}
            >
              {restorePending ? 'Restoring…' : 'Restore'}
            </button>
          {/if}
          {#if hasCompareResult}
            <button
              class="secondary-action"
              data-vtext-merge-preview
              on:click={() => dispatch('merge-preview')}
              disabled={mergePending || selectedMergeSuggestionCount === 0}
            >
              {mergePending ? 'Merging…' : 'Merge into draft'}
            </button>
          {/if}
        {/if}
        <div class="publish-menu-wrap">
          <button
            class="secondary-action publish-action"
            data-vtext-publish
            aria-haspopup="menu"
            aria-expanded={publishMenuOpen}
            on:click={() => dispatch('toggle-publish')}
            disabled={publishDisabled}
          >
            {publishedActionPending ? 'Publishing…' : `Publish ${versionLabel}`}
          </button>
          {#if publishMenuOpen}
            <div class="publish-menu" data-vtext-publish-menu role="menu" aria-label="Publish this version">
              <div class="publish-menu-heading">
                <p class="eyebrow">Publish</p>
                <h3>Publish {versionLabel}</h3>
                <p>This creates a public link with the current text and source snapshots.</p>
              </div>
              <div class="publish-menu-actions">
                <button
                  type="button"
                  class="primary-action"
                  data-vtext-publish-confirm
                  on:click={() => dispatch('publish-confirm')}
                  disabled={publishedActionPending}
                >
                  {publishedActionPending ? 'Publishing…' : 'Publish'}
                </button>
                <button
                  type="button"
                  class="secondary-action"
                  data-vtext-publish-cancel
                  on:click={() => dispatch('publish-cancel')}
                  disabled={publishedActionPending}
                >
                  Cancel
                </button>
              </div>
            </div>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</div>

<style>
  .doc-toolbar {
    position: relative;
    z-index: 10;
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.55rem;
    min-height: 3.7rem;
    padding: 0.5rem 0.72rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    max-height: 3.7rem;
    overflow: visible;
    transition:
      opacity 180ms ease,
      transform 180ms ease,
      max-height 180ms ease,
      padding 180ms ease,
      border-color 180ms ease;
    will-change: opacity, transform, max-height;
  }

  .doc-toolbar.toolbar-hidden {
    height: 0;
    max-height: 0;
    min-height: 0;
    padding-top: 0;
    padding-bottom: 0;
    border-bottom-color: transparent;
    opacity: 0;
    overflow: hidden;
    pointer-events: none;
    transform: translateY(-100%);
  }

  .doc-toolbar.toolbar-hidden > * {
    visibility: hidden;
  }

  .doc-toolbar.toolbar-hidden:focus-within {
    height: auto;
    max-height: 4.2rem;
    padding-top: 0.58rem;
    padding-bottom: 0.58rem;
    border-bottom-color: var(--choir-border-strong);
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
  }

  .doc-toolbar.toolbar-hidden:focus-within > * {
    visibility: visible;
  }

  .version-controls,
  .doc-actions {
    display: flex;
    align-items: center;
    gap: 0.42rem;
    min-width: 0;
    flex-wrap: nowrap;
  }

  .doc-actions {
    justify-content: flex-end;
  }

  .doc-state {
    min-width: 0;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--choir-text-accent);
    font-size: 0.74rem;
  }

  .nav-version {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 3.1rem;
    height: 1.95rem;
    padding: 0 0.6rem;
    border-radius: 999px;
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    font-size: 0.76rem;
    font-weight: 650;
    backdrop-filter: blur(8px);
  }

  .draft-line {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 7.6rem;
    height: 1.95rem;
    padding: 0 0.72rem;
    border-radius: 999px;
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-surface-raised);
    color: var(--choir-text-primary);
    font-size: 0.74rem;
    font-weight: 680;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .nav-btn,
  .prompt-btn,
  .secondary-action,
  .download-menu > summary,
  .download-menu button,
  .primary-action {
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    backdrop-filter: blur(10px);
    transition: transform 120ms ease, background 120ms ease, border-color 120ms ease;
  }

  .nav-btn {
    width: 1.95rem;
    height: 1.95rem;
    border-radius: 999px;
    font-size: 0.92rem;
    font-weight: 700;
  }

  .prompt-btn {
    border-radius: 999px;
    padding: 0.62rem 0.95rem;
    font-size: 0.82rem;
    font-weight: 700;
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

  .primary-action {
    border-radius: 999px;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .publish-action {
    min-width: 7.4rem;
  }

  .secondary-action.danger {
    border-color: var(--choir-status-danger);
    color: var(--choir-status-danger);
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

  .publish-menu-wrap {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .publish-menu {
    position: absolute;
    top: calc(100% + 0.48rem);
    right: 0;
    z-index: 8;
    width: min(18rem, calc(100vw - 2rem));
    display: grid;
    gap: 0.68rem;
    padding: 0.78rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.5rem;
    background: #081225;
    box-shadow: var(--choir-shadow-lg);
    color: var(--choir-text-primary);
  }

  .publish-menu-heading h3 {
    margin: 0.12rem 0 0;
    color: var(--choir-text-primary);
    font-size: 0.92rem;
    line-height: 1.2;
  }

  .publish-menu-heading p:not(.eyebrow) {
    margin: 0.34rem 0 0;
    color: var(--choir-text-secondary);
    font-size: 0.76rem;
    line-height: 1.35;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .publish-menu-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.48rem;
    min-width: 0;
  }

  .nav-btn:hover:enabled,
  .prompt-btn:hover:enabled,
  .secondary-action:hover:enabled,
  .primary-action:hover:enabled {
    transform: translateY(-1px);
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
  }

  .nav-btn:disabled,
  .prompt-btn:disabled,
  .secondary-action:disabled,
  .primary-action:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  @media (max-width: 768px) {
    .doc-toolbar {
      grid-template-columns: minmax(0, 1fr);
      align-items: stretch;
      gap: 0.38rem;
      max-height: none;
      padding: 0.46rem 0.55rem 0.56rem;
    }

    .version-controls,
    .doc-actions {
      gap: 0.32rem;
      flex-wrap: wrap;
      justify-content: flex-start;
    }

    .doc-state {
      order: 3;
      text-align: left;
      font-size: 0.68rem;
      white-space: normal;
      line-height: 1.2;
    }

    .nav-version {
      min-width: 2.05rem;
      height: 1.78rem;
      padding: 0 0.48rem;
      font-size: 0.7rem;
    }

    .draft-line {
      width: auto;
      max-width: min(8.8rem, 46vw);
      height: 1.78rem;
      padding: 0 0.52rem;
      font-size: 0.68rem;
    }

    .nav-btn {
      width: 1.78rem;
      height: 1.78rem;
      font-size: 0.82rem;
    }

    .prompt-btn {
      padding: 0.5rem 0.7rem;
      font-size: 0.75rem;
    }

    .secondary-action {
      min-width: auto;
      padding: 0.5rem 0.64rem;
      font-size: 0.72rem;
    }

    .publish-action {
      min-width: auto;
    }

    .publish-menu {
      right: -0.2rem;
      width: min(18rem, calc(100vw - 1.4rem));
    }
  }
</style>
