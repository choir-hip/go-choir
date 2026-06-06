<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import {
    sourceEntityID,
    sourceEntityKindLabel,
    sourceEntityTargetURL,
    sourceEntityTitle,
  } from './vtext-source-renderer';

  export let currentDoc = null;
  export let currentRevision = null;
  export let isPublishedReadOnly = false;
  export let sourceCandidates = [];
  export let sourceEntities = [];
  export let sourceSummary = null;
  export let editEvidence = null;
  export let sourceDiagnosisPending = false;
  export let sourceRepairPending = false;
  export let sourceRepairError = '';
  export let sourceReviewMarker = '';
  export let sourceReviewTitle = '';
  export let sourceReviewURL = '';
  export let sourceReviewExcerpt = '';
  export let sourceReviewStatus = '';
  export let selectedSourceEntityID = '';
  export let sourceArtifactTitle = '';
  export let sourceArtifactURL = '';
  export let sourceArtifactText = '';
  export let sourceArtifactPending = false;
  export let sourceArtifactStatus = '';
  export let sourceArtifactError = '';

  const dispatch = createEventDispatcher();

  $: selectedSourceEntity = sourceEntities.find((entity) => sourceEntityID(entity) === selectedSourceEntityID)
    || sourceEntities[0]
    || null;
  $: heading = sourceCandidates.length
    ? `${sourceCandidates.length} source review marker${sourceCandidates.length === 1 ? '' : 's'}`
    : `${sourceEntities.length} represented source${sourceEntities.length === 1 ? '' : 's'}`;
  $: canApplySourceReview = Boolean(currentDoc && currentRevision && sourceReviewMarker && sourceReviewTitle.trim() && sourceReviewExcerpt.trim());
  $: canImportSourceArtifact = Boolean(currentDoc && currentRevision && sourceArtifactURL.trim());
  $: canAttachSourceArtifact = Boolean(currentDoc && currentRevision && sourceArtifactText.trim());
</script>

<section class="source-panel" data-vtext-source-diagnostics>
  <div class="source-panel-heading">
    <div>
      <p class="eyebrow">Sources</p>
      <h3>{heading}</h3>
    </div>
    <button
      type="button"
      class="secondary-action"
      data-vtext-load-diagnosis
      on:click={() => dispatch('diagnosis')}
      disabled={!currentDoc || isPublishedReadOnly}
    >
      {sourceDiagnosisPending ? 'Cancel diagnosis' : 'Diagnosis'}
    </button>
  </div>

  {#if sourceCandidates.length}
    <div class="source-marker-list" data-vtext-source-gaps aria-label="Claims needing source review">
      {#each sourceCandidates as marker}
        <span>{marker}</span>
      {/each}
    </div>
  {/if}

  {#if sourceEntities.length}
    <div class="source-entity-list" data-vtext-source-entities>
      {#each sourceEntities as entity}
        <button
          type="button"
          class="source-entity-chip"
          data-vtext-source-entity-chip
          on:click={() => dispatch('source-entity-open', { entity })}
        >
          <strong>{sourceEntityTitle(entity)}</strong>
          <span>{sourceEntityKindLabel(entity.kind)}</span>
        </button>
      {/each}
    </div>
  {/if}

  {#if sourceSummary}
    <div class="source-diagnosis-facts" data-vtext-diagnosis-summary>
      <span>{sourceSummary.revisionCount} revisions</span>
      <span>{sourceSummary.runCount} runs</span>
      {#if sourceSummary.latestVersion}
        <span>{sourceSummary.latestVersion}</span>
      {/if}
      {#if sourceSummary.errorCount}
        <span>{sourceSummary.errorCount} errors</span>
      {/if}
    </div>
  {/if}

  {#if editEvidence}
    <div class="source-edit-evidence" data-vtext-edit-evidence>
      <div>
        <span class="evidence-label">Edit evidence</span>
        <strong>{editEvidence.version || 'revision'}</strong>
        {#if editEvidence.author}
          <span>{editEvidence.author}</span>
        {/if}
      </div>
      <dl>
        {#if editEvidence.contextMode}
          <div data-vtext-edit-context-mode>
            <dt>context</dt>
            <dd>{editEvidence.contextMode}</dd>
          </div>
        {/if}
        {#if editEvidence.operation}
          <div data-vtext-edit-operation>
            <dt>operation</dt>
            <dd>{editEvidence.operation}</dd>
          </div>
        {/if}
        {#if editEvidence.promptChars !== null}
          <div data-vtext-edit-prompt-chars>
            <dt>prompt chars</dt>
            <dd>{editEvidence.promptChars}</dd>
          </div>
        {/if}
        {#if editEvidence.editCount !== null}
          <div data-vtext-edit-count>
            <dt>edits</dt>
            <dd>{editEvidence.editCount}</dd>
          </div>
        {/if}
        {#if editEvidence.deltaChars !== null}
          <div data-vtext-edit-delta-chars>
            <dt>delta chars</dt>
            <dd>{editEvidence.deltaChars}</dd>
          </div>
        {/if}
        {#if editEvidence.latencyMs !== null}
          <div data-vtext-edit-latency-ms>
            <dt>latency ms</dt>
            <dd>{editEvidence.latencyMs}</dd>
          </div>
        {/if}
      </dl>
    </div>
  {/if}

  {#if !isPublishedReadOnly}
    {#if sourceCandidates.length}
      <div class="source-review-panel" data-vtext-source-review-panel>
        <div class="source-artifact-heading">
          <span class="evidence-label">Source review</span>
          <strong>{sourceReviewMarker ? `Repair ${sourceReviewMarker}` : 'Choose marker'}</strong>
        </div>
        <div class="source-review-marker-picker" role="listbox" aria-label="Citation marker to repair">
          {#each sourceCandidates as marker}
            <button
              type="button"
              class:selected={marker === sourceReviewMarker}
              data-vtext-source-review-marker
              data-source-marker={marker}
              on:click={() => dispatch('source-review-marker', { marker })}
            >
              {marker}
            </button>
          {/each}
        </div>
        <label class="source-artifact-field">
          <span>Source title</span>
          <input data-vtext-source-review-title bind:value={sourceReviewTitle} placeholder="Name the confirming or refuting source" />
        </label>
        <label class="source-artifact-field">
          <span>Source URL</span>
          <input data-vtext-source-review-url bind:value={sourceReviewURL} placeholder="Optional public source URL" />
        </label>
        <label class="source-artifact-field">
          <span>Confirming excerpt</span>
          <textarea
            data-vtext-source-review-excerpt
            bind:value={sourceReviewExcerpt}
            spellcheck="true"
            rows="5"
            placeholder="Paste the exact source text or concise reader-mode evidence that supports this marker"
          ></textarea>
        </label>
        <div class="source-panel-actions">
          <button
            type="button"
            class="primary-action"
            data-vtext-apply-source-review
            on:click={() => dispatch('apply-source-review')}
            disabled={sourceRepairPending || !canApplySourceReview}
          >
            {sourceRepairPending ? 'Applying...' : 'Apply source review'}
          </button>
          {#if sourceReviewStatus}
            <span class="source-artifact-status" role="status">{sourceReviewStatus}</span>
          {/if}
          {#if sourceRepairError}
            <span class="source-repair-error" role="alert">{sourceRepairError}</span>
          {/if}
        </div>
      </div>
    {/if}

    <div class="source-artifact-panel" data-vtext-source-artifact-panel>
      <div class="source-artifact-heading">
        <span class="evidence-label">Source artifact</span>
        <strong>{selectedSourceEntity ? sourceEntityTitle(selectedSourceEntity) : 'Choose a source'}</strong>
      </div>
      {#if sourceEntities.length > 0}
        <div class="source-artifact-picker" role="listbox" aria-label="Source artifact target">
          {#each sourceEntities as entity}
            <button
              type="button"
              class:selected={sourceEntityID(entity) === selectedSourceEntityID}
              data-vtext-source-artifact-target
              data-source-entity-id={sourceEntityID(entity)}
              on:click={() => dispatch('source-artifact-target', { entity })}
            >
              {sourceEntityTitle(entity)}
            </button>
          {/each}
        </div>
        <label class="source-artifact-field">
          <span>Title</span>
          <input data-vtext-source-artifact-title bind:value={sourceArtifactTitle} />
        </label>
        <label class="source-artifact-field">
          <span>URL</span>
          <input data-vtext-source-artifact-url bind:value={sourceArtifactURL} />
        </label>
        <div class="source-panel-actions">
          <button
            type="button"
            class="secondary-action"
            data-vtext-import-source-artifact
            on:click={() => dispatch('import-source-artifact')}
            disabled={sourceArtifactPending || !canImportSourceArtifact}
          >
            {sourceArtifactPending ? 'Working...' : 'Import URL'}
          </button>
        </div>
        <label class="source-artifact-field">
          <span>Readable source text</span>
          <textarea
            data-vtext-source-artifact-text
            bind:value={sourceArtifactText}
            spellcheck="true"
            rows="7"
          ></textarea>
        </label>
        <div class="source-panel-actions">
          <button
            type="button"
            class="primary-action"
            data-vtext-attach-source-artifact
            on:click={() => dispatch('attach-source-artifact')}
            disabled={sourceArtifactPending || !canAttachSourceArtifact}
          >
            {sourceArtifactPending ? 'Attaching...' : 'Attach text'}
          </button>
          {#if sourceArtifactStatus}
            <span class="source-artifact-status" role="status">{sourceArtifactStatus}</span>
          {/if}
          {#if sourceArtifactError}
            <span class="source-repair-error" role="alert">{sourceArtifactError}</span>
          {/if}
        </div>
      {:else}
        <p class="source-artifact-empty">No source entities are available in this revision.</p>
      {/if}
    </div>

  {/if}
</section>

<style>
  .source-panel {
    flex: 0 0 auto;
    display: grid;
    gap: 0.62rem;
    padding: 0.74rem 0.86rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-surface-raised);
    color: var(--choir-text-primary);
  }

  .source-panel-heading,
  .source-panel-actions {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.7rem;
    min-width: 0;
  }

  .source-panel-heading h3 {
    margin: 0.12rem 0 0;
    color: var(--choir-text-primary);
    font-size: 0.92rem;
    line-height: 1.2;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .source-marker-list,
  .source-diagnosis-facts {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .source-marker-list span,
  .source-diagnosis-facts span {
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    padding: 0.18rem 0.46rem;
    color: var(--choir-text-accent);
    background: var(--choir-state-selected);
    font-size: 0.72rem;
    font-weight: 720;
  }

  .source-edit-evidence {
    display: grid;
    gap: 0.52rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    padding: 0.58rem;
    background: rgba(255, 255, 255, 0.045);
  }

  .source-edit-evidence > div:first-child {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.38rem;
    color: var(--choir-text-muted);
    font-size: 0.72rem;
  }

  .source-edit-evidence strong {
    color: var(--choir-text-primary);
    font-size: 0.82rem;
  }

  .evidence-label {
    color: var(--choir-text-accent);
    font-weight: 760;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  .source-edit-evidence dl {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.42rem;
    margin: 0;
  }

  .source-edit-evidence dl div {
    min-width: 0;
  }

  .source-edit-evidence dt {
    margin: 0 0 0.12rem;
    color: var(--choir-text-muted);
    font-size: 0.66rem;
  }

  .source-edit-evidence dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--choir-text-primary);
    font-size: 0.76rem;
    font-weight: 700;
  }

  .source-entity-list {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.42rem;
  }

  .source-entity-chip {
    display: grid;
    gap: 0.16rem;
    min-width: 0;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    padding: 0.48rem 0.58rem;
    color: var(--choir-text-primary);
    background: var(--choir-state-selected);
    text-align: left;
    cursor: pointer;
  }

  .source-entity-chip strong,
  .source-entity-chip span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .source-entity-chip strong {
    font-size: 0.76rem;
  }

  .source-entity-chip span {
    color: var(--choir-text-secondary);
    font-size: 0.66rem;
    font-weight: 720;
    text-transform: uppercase;
  }

  .source-review-panel,
  .source-artifact-panel {
    display: grid;
    gap: 0.55rem;
    border-left: 2px solid var(--choir-border-strong);
    padding-left: 0.7rem;
  }

  .source-artifact-heading {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0.42rem;
    min-width: 0;
  }

  .source-artifact-heading strong {
    min-width: 0;
    color: var(--choir-text-primary);
    font-size: 0.82rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .source-review-marker-picker,
  .source-artifact-picker {
    display: flex;
    flex-wrap: wrap;
    gap: 0.32rem;
  }

  .source-review-marker-picker button,
  .source-artifact-picker button {
    border: 1px solid var(--choir-border-subtle);
    border-radius: 999px;
    padding: 0.24rem 0.52rem;
    color: var(--choir-text-secondary);
    background: transparent;
    font-size: 0.68rem;
    font-weight: 720;
    cursor: pointer;
  }

  .source-review-marker-picker button.selected,
  .source-artifact-picker button.selected {
    border-color: var(--choir-border-strong);
    color: var(--choir-text-primary);
    background: var(--choir-state-selected);
  }

  .source-artifact-field {
    display: grid;
    gap: 0.26rem;
    color: var(--choir-text-secondary);
    font-size: 0.7rem;
    font-weight: 720;
  }

  .source-artifact-field input,
  .source-artifact-field textarea {
    width: 100%;
    border: 1px solid var(--choir-border-strong);
    border-radius: 6px;
    padding: 0.48rem 0.54rem;
    color: var(--choir-text-primary);
    background: var(--choir-state-selected);
    font: inherit;
    line-height: 1.35;
  }

  .source-artifact-field textarea {
    min-height: 7rem;
    resize: vertical;
  }

  .source-artifact-status {
    flex: 1 1 auto;
    min-width: 0;
    color: var(--choir-text-secondary);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .source-artifact-empty {
    margin: 0;
    color: var(--choir-text-secondary);
    font-size: 0.76rem;
  }

  .source-repair-error {
    flex: 1 1 auto;
    min-width: 0;
    color: var(--choir-status-danger);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .secondary-action,
  .primary-action {
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    backdrop-filter: blur(10px);
    transition: transform 120ms ease, background 120ms ease, border-color 120ms ease;
  }

  .secondary-action {
    border-radius: 999px;
    padding: 0.62rem 0.84rem;
    font-size: 0.78rem;
    font-weight: 720;
    color: var(--choir-text-accent);
  }

  .primary-action {
    border-radius: 999px;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .secondary-action:hover:enabled,
  .primary-action:hover:enabled {
    transform: translateY(-1px);
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
  }

  .secondary-action:disabled,
  .primary-action:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  @media (max-width: 760px) {
    .secondary-action {
      padding: 0.5rem 0.64rem;
      font-size: 0.72rem;
    }
  }
</style>
