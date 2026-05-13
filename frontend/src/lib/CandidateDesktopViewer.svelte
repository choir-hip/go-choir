<!--
  CandidateDesktopViewer — embeds the normal Choir Svelte desktop for a
  candidate desktop selector. The candidate VM only serves sandbox APIs; the
  Svelte shell is still loaded from the current frontend origin.
-->
<script>
  export let appContext = {};

  let desktopId = appContext?.desktopId || appContext?.candidateDesktopId || '';
  let draftDesktopId = desktopId;

  $: normalizedDesktopId = String(desktopId || '').trim();
  $: candidateSrc = normalizedDesktopId
    ? `/?desktop_id=${encodeURIComponent(normalizedDesktopId)}&embedded=1`
    : '';

  function openCandidate() {
    desktopId = String(draftDesktopId || '').trim();
  }

  function handleKeydown(event) {
    if (event.key === 'Enter') {
      openCandidate();
    }
  }
</script>

<section
  class="candidate-viewer"
  data-candidate-desktop-viewer
  data-candidate-desktop-id={normalizedDesktopId}
>
  <header class="viewer-toolbar">
    <div class="viewer-title">
      <strong>Candidate Desktop</strong>
      <span>Same Choir UI, candidate-scoped APIs</span>
    </div>
    <label class="desktop-id-field">
      <span>Desktop ID</span>
      <input
        data-candidate-desktop-input
        bind:value={draftDesktopId}
        on:keydown={handleKeydown}
        placeholder="candidate desktop id"
        spellcheck="false"
      />
    </label>
    <button
      class="open-btn"
      data-candidate-desktop-open
      on:click={openCandidate}
      disabled={!String(draftDesktopId || '').trim()}
    >
      Open
    </button>
  </header>

  {#if candidateSrc}
    <iframe
      class="candidate-frame"
      data-candidate-desktop-frame
      src={candidateSrc}
      title="Candidate desktop {normalizedDesktopId}"
      sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
    ></iframe>
  {:else}
    <div class="empty-state" data-candidate-desktop-empty>
      <strong>No candidate selected</strong>
      <span>Enter a candidate desktop ID to preview it through the normal Svelte desktop route.</span>
    </div>
  {/if}
</section>

<style>
  .candidate-viewer {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: #0d1117;
    color: #dbe7ff;
  }

  .viewer-toolbar {
    display: grid;
    grid-template-columns: minmax(180px, 1fr) minmax(220px, 320px) auto;
    align-items: end;
    gap: 10px;
    padding: 10px;
    border-bottom: 1px solid rgba(148, 163, 184, 0.2);
    background: #111827;
  }

  .viewer-title {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }

  .viewer-title strong {
    font-size: 0.92rem;
  }

  .viewer-title span,
  .desktop-id-field span {
    color: #9fb0ca;
    font-size: 0.72rem;
  }

  .desktop-id-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .desktop-id-field input {
    width: 100%;
    min-width: 0;
    border: 1px solid #334155;
    border-radius: 4px;
    background: #0f172a;
    color: #e5edf8;
    font-size: 0.82rem;
    padding: 7px 8px;
  }

  .open-btn {
    min-height: 34px;
    border: 1px solid rgba(96, 165, 250, 0.38);
    border-radius: 4px;
    background: rgba(37, 99, 235, 0.24);
    color: #bfdbfe;
    cursor: pointer;
    font-size: 0.78rem;
    padding: 0 12px;
  }

  .open-btn:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .candidate-frame {
    flex: 1;
    min-height: 0;
    width: 100%;
    border: 0;
    background: #05070c;
  }

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 24px;
    text-align: center;
    color: #9fb0ca;
  }

  .empty-state strong {
    color: #e5edf8;
  }

  @media (max-width: 720px) {
    .viewer-toolbar {
      grid-template-columns: 1fr;
      align-items: stretch;
    }
  }
</style>
