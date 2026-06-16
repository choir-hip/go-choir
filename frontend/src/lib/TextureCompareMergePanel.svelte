<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let compareResult: any = null;
  export let mergePreview: any = null;
  export let comparePending = false;
  export let mergePending = false;
  export let compareError = '';
  export let versionLabel = '';
  export let nextVersionLabel = '';
  export let compareTargetVersionLabel = 'latest';
  export let selectedMergeSuggestionIds: string[] = [];

  const dispatch = createEventDispatcher();

  $: visible = !!compareResult || !!mergePreview || comparePending || mergePending || !!compareError;

  function suggestionSelected(id: string): boolean {
    return selectedMergeSuggestionIds.includes(id);
  }
</script>

{#if visible}
  <section class="compare-panel" class:compare-panel-error={compareError && !comparePending && !mergePending} data-texture-compare-panel>
    <div class="compare-heading">
      <div>
        <p class="eyebrow">{compareError ? 'Compare failed' : mergePreview ? 'Merge preview' : `What changed since ${versionLabel}`}</p>
        <h3>
          {#if mergePending}
            Model merge in progress
          {:else if comparePending}
            Model compare in progress
          {:else if compareError}
            Could not compare {versionLabel} to {compareTargetVersionLabel}
          {:else}
            {mergePreview ? `Merged into ${nextVersionLabel}` : `Compare ${versionLabel} → ${compareTargetVersionLabel}`}
          {/if}
        </h3>
      </div>
      {#if mergePreview}
        <span class="compare-chip">from {versionLabel}</span>
      {/if}
    </div>
    {#if comparePending || mergePending}
      <div class="compare-working" role="status" aria-live="polite">
        <span class="work-pulse" aria-hidden="true"></span>
        <span>{mergePending ? 'Building a reviewable merge preview with the configured Texture model.' : 'Comparing versions with the configured Texture model.'}</span>
      </div>
    {/if}
    {#if compareError && !comparePending && !mergePending}
      <div class="compare-error" role="alert" data-texture-compare-error>
        <span>{compareError}</span>
        <button type="button" class="secondary-action" on:click={() => dispatch('retry-compare')}>
          Retry compare
        </button>
      </div>
    {/if}
    {#if compareResult?.summary?.length}
      <div class="compare-summary">
        {#each compareResult.summary as finding}
          <span>{finding}</span>
        {/each}
      </div>
    {/if}
    {#if compareResult?.suggestions?.length && !mergePreview}
      <div class="merge-suggestions">
        {#each compareResult.suggestions as suggestion}
          <label class="merge-suggestion" data-texture-merge-suggestion>
            <input
              type="checkbox"
              checked={suggestionSelected(suggestion.id)}
              on:change={() => dispatch('toggle-suggestion', suggestion.id)}
            />
            <span>
              <strong>{suggestion.label}</strong>
              <small>{suggestion.status} · {suggestion.description}</small>
            </span>
          </label>
        {/each}
      </div>
    {:else if mergePreview?.suggestions?.length}
      <div class="provenance-strip">
        {#each mergePreview.suggestions as suggestion}
          <span>{suggestion.label}</span>
        {/each}
      </div>
    {/if}
  </section>
{/if}

<style>
  .compare-panel {
    flex: 0 0 auto;
    display: grid;
    gap: 0.75rem;
    padding: 0.8rem 0.95rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-surface-raised);
    color: var(--choir-text-primary);
  }

  .compare-panel-error {
    border-bottom-color: var(--choir-status-danger);
  }

  .compare-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.8rem;
    min-width: 0;
  }

  .compare-heading h3 {
    margin: 0.12rem 0 0;
    color: var(--choir-text-primary);
    font-size: 0.92rem;
    line-height: 1.2;
  }

  .compare-chip {
    flex: 0 0 auto;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    padding: 0.34rem 0.55rem;
    color: var(--choir-text-accent);
    font-size: 0.68rem;
    font-weight: 720;
  }

  .compare-working {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
    color: var(--choir-text-secondary);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .compare-error {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.65rem;
    color: var(--choir-text-primary);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .compare-error span {
    flex: 1 1 18rem;
    min-width: 0;
  }

  .compare-summary,
  .provenance-strip {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.42rem 0.8rem;
    color: var(--choir-text-secondary);
    font-size: 0.72rem;
    line-height: 1.35;
  }

  .merge-suggestions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.48rem;
  }

  .merge-suggestion {
    display: flex;
    align-items: flex-start;
    gap: 0.48rem;
    min-width: 0;
    padding: 0.56rem 0.62rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.5rem;
    background: var(--choir-state-selected);
    color: var(--choir-text-primary);
    cursor: pointer;
  }

  .merge-suggestion input {
    flex: 0 0 auto;
    margin-top: 0.1rem;
  }

  .merge-suggestion span {
    min-width: 0;
    display: grid;
    gap: 0.18rem;
  }

  .merge-suggestion strong {
    font-size: 0.76rem;
    line-height: 1.2;
  }

  .merge-suggestion small {
    color: var(--choir-text-secondary);
    font-size: 0.66rem;
    line-height: 1.25;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .secondary-action {
    background: var(--choir-state-hover);
    border-color: var(--choir-border);
  }

  .secondary-action {
    border: 1px solid var(--choir-border);
    border-radius: 999px;
    color: inherit;
    cursor: pointer;
    transition: transform 160ms ease, background 160ms ease, border-color 160ms ease;
  }

  .secondary-action {
    padding: 0.55rem 0.78rem;
    font-size: 0.74rem;
    font-weight: 720;
  }

  .secondary-action:hover:enabled {
    transform: translateY(-1px);
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
  }

  .secondary-action:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  .work-pulse {
    width: 0.62rem;
    height: 0.62rem;
    border-radius: 999px;
    background: var(--choir-text-accent);
    box-shadow: 0 0 0 0 var(--choir-border-strong);
    animation: work-pulse 1.1s ease-out infinite;
  }

  @keyframes work-pulse {
    0% {
      box-shadow: 0 0 0 0 var(--choir-border-strong);
    }
    100% {
      box-shadow: 0 0 0 0.8rem transparent;
    }
  }

  @media (max-width: 768px) {
    .compare-panel {
      padding: 0.68rem 0.72rem;
      gap: 0.6rem;
    }

    .compare-summary,
    .provenance-strip,
    .merge-suggestions {
      grid-template-columns: minmax(0, 1fr);
    }

    .secondary-action {
      padding: 0.5rem 0.64rem;
      font-size: 0.72rem;
    }
  }
</style>
