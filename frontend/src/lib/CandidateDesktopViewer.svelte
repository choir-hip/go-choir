<!--
  CandidateDesktopViewer — embeds the normal Choir Svelte desktop for a
  candidate desktop selector. The candidate VM only serves sandbox APIs; the
  Svelte shell is still loaded from the current frontend origin.
-->
<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';

  export let appContext = {};

  const dispatch = createEventDispatcher();

  let desktopId = appContext?.desktopId || appContext?.candidateDesktopId || '';
  let draftDesktopId = desktopId;
  let loading = true;
  let error = '';
  let candidates = [];
  let removeLiveListener = () => {};

  $: normalizedDesktopId = String(desktopId || '').trim();
  $: candidateSrc = normalizedDesktopId
    ? `/?desktop_id=${encodeURIComponent(normalizedDesktopId)}&embedded=1`
    : '';
  $: activeCandidate = candidates.find((candidate) => candidateDesktopId(candidate) === normalizedDesktopId);

  function candidateDesktopId(candidate) {
    return String(candidate?.vm_id || candidate?.desktop_id || candidate?.candidate_id || '').trim();
  }

  function candidateTitle(candidate) {
    return candidate?.summary || candidate?.candidate_id || 'Candidate patchset';
  }

  function candidateMeta(candidate) {
    return [
      candidate?.vm_id || 'no VM id',
      candidate?.integration_branch || candidate?.destination_branch || 'not integrated',
    ].join(' · ');
  }

  async function refreshCandidates() {
    loading = true;
    error = '';
    try {
      const res = await fetchWithRenewal('/api/promotions?limit=20', { method: 'GET' });
      if (!res.ok) {
        throw new Error(`Promotion candidates failed (${res.status})`);
      }
      const body = await res.json();
      candidates = Array.isArray(body?.candidates) ? body.candidates : [];
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Promotion candidates unavailable';
      candidates = [];
    } finally {
      loading = false;
    }
  }

  function openCandidate() {
    desktopId = String(draftDesktopId || '').trim();
  }

  function openPromotionCandidate(candidate) {
    const nextDesktopId = candidateDesktopId(candidate);
    if (!nextDesktopId) return;
    draftDesktopId = nextDesktopId;
    desktopId = nextDesktopId;
  }

  function handleKeydown(event) {
    if (event.key === 'Enter') {
      openCandidate();
    }
  }

  onMount(() => {
    void refreshCandidates();
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
      if (
        kind === 'promotion.candidate.queued' ||
        kind === 'promotion.candidate.verified' ||
        kind === 'promotion.candidate.failed' ||
        kind === 'promotion.candidate.promoted' ||
        kind === 'promotion.candidate.reviewed'
      ) {
        void refreshCandidates();
      }
    });
  });

  onDestroy(() => {
    removeLiveListener();
  });
</script>

<section
  class="candidate-viewer"
  data-candidate-desktop-viewer
  data-candidate-desktop-id={normalizedDesktopId}
>
  <header class="viewer-toolbar">
    <div class="viewer-title">
      <strong>Candidate Desktop</strong>
      <span>Open queued candidate worlds without copying raw IDs.</span>
    </div>
  </header>

  <div class="viewer-body">
    <aside class="candidate-queue" data-candidate-desktop-queue>
      <div class="queue-header">
        <strong>Queued candidates</strong>
        <span>{candidates.length} available</span>
      </div>

      {#if error}
        <div class="state-card error" data-candidate-desktop-error role="alert">{error}</div>
      {:else if loading}
        <div class="state-card" data-candidate-desktop-loading>Loading candidate patchsets…</div>
      {:else if candidates.length === 0}
        <div class="state-card" data-candidate-desktop-empty>
          <strong>No candidate patchsets queued</strong>
          <span>When worker or candidate-world exports exist, they appear here automatically.</span>
        </div>
      {:else}
        <div class="candidate-list" data-candidate-desktop-list>
          {#each candidates as candidate}
            <article
              class:active={candidateDesktopId(candidate) === normalizedDesktopId}
              class="candidate-card"
              data-candidate-desktop-card
              data-candidate-desktop-candidate-id={candidate.candidate_id}
            >
              <div class="candidate-card-top">
                <strong>{candidateTitle(candidate)}</strong>
                <span data-candidate-desktop-status>{candidate.status || 'unknown'}</span>
              </div>
              <p>{candidateMeta(candidate)}</p>
              {#if candidate.error}
                <p class="candidate-error">{candidate.error}</p>
              {/if}
              <button
                class="open-btn"
                data-candidate-desktop-open-candidate
                on:click={() => openPromotionCandidate(candidate)}
                disabled={!candidateDesktopId(candidate)}
              >
                {candidateDesktopId(candidate) === normalizedDesktopId ? 'Viewing' : 'Open candidate'}
              </button>
            </article>
          {/each}
        </div>
      {/if}

      <details class="manual-fallback" data-candidate-desktop-manual>
        <summary>Advanced desktop ID</summary>
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
          class="open-btn manual-open"
          data-candidate-desktop-open
          on:click={openCandidate}
          disabled={!String(draftDesktopId || '').trim()}
        >
          Open ID
        </button>
      </details>
    </aside>

    <div class="preview-pane" data-candidate-desktop-preview>
      {#if candidateSrc}
        <div class="preview-header" data-candidate-desktop-active>
          <div>
            <strong>{activeCandidate ? candidateTitle(activeCandidate) : normalizedDesktopId}</strong>
            <span>{activeCandidate ? candidateMeta(activeCandidate) : 'Manual candidate desktop'}</span>
          </div>
          <code>{normalizedDesktopId}</code>
        </div>
        <iframe
          class="candidate-frame"
          data-candidate-desktop-frame
          src={candidateSrc}
          title="Candidate desktop {normalizedDesktopId}"
          sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
        ></iframe>
      {:else}
        <div class="empty-state" data-candidate-desktop-preview-empty>
          <strong>Select a candidate</strong>
          <span>Queued candidate worlds appear on the left with status, VM identity, and branch context.</span>
        </div>
      {/if}
    </div>
  </div>
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
    display: flex;
    align-items: center;
    justify-content: space-between;
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
  .desktop-id-field span,
  .queue-header span,
  .preview-header span,
  .candidate-card p {
    color: #9fb0ca;
    font-size: 0.72rem;
  }

  .viewer-body {
    display: grid;
    grid-template-columns: minmax(230px, 320px) minmax(0, 1fr);
    flex: 1;
    min-height: 0;
  }

  .candidate-queue {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-height: 0;
    border-right: 1px solid rgba(148, 163, 184, 0.18);
    background: #0b1220;
    padding: 10px;
    overflow: auto;
  }

  .queue-header,
  .candidate-card-top,
  .preview-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 8px;
  }

  .queue-header strong,
  .preview-header strong,
  .candidate-card strong {
    color: #e5edf8;
    font-size: 0.86rem;
    overflow-wrap: anywhere;
  }

  .candidate-list {
    display: grid;
    gap: 8px;
  }

  .candidate-card,
  .state-card,
  .manual-fallback {
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.72);
    padding: 10px;
  }

  .candidate-card {
    display: grid;
    gap: 8px;
  }

  .candidate-card.active {
    border-color: rgba(96, 165, 250, 0.58);
    background: rgba(30, 64, 175, 0.28);
  }

  .candidate-card p {
    margin: 0;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  .candidate-card-top span {
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 999px;
    color: #bfdbfe;
    flex-shrink: 0;
    font-size: 0.68rem;
    font-weight: 800;
    padding: 2px 7px;
  }

  .candidate-error,
  .state-card.error {
    color: #fecaca;
  }

  .state-card {
    display: grid;
    gap: 5px;
    color: #9fb0ca;
    font-size: 0.78rem;
    line-height: 1.35;
  }

  .state-card strong {
    color: #e5edf8;
  }

  .manual-fallback {
    margin-top: auto;
  }

  .manual-fallback summary {
    color: #bfdbfe;
    cursor: pointer;
    font-size: 0.76rem;
    font-weight: 800;
  }

  .desktop-id-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    margin-top: 9px;
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

  .open-btn {
    width: fit-content;
  }

  .manual-open {
    margin-top: 8px;
  }

  .open-btn:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .preview-pane {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  .preview-header {
    border-bottom: 1px solid rgba(148, 163, 184, 0.14);
    background: rgba(15, 23, 42, 0.78);
    padding: 9px 10px;
  }

  .preview-header div {
    display: grid;
    gap: 2px;
    min-width: 0;
  }

  .preview-header code {
    color: #93c5fd;
    flex-shrink: 0;
    font-size: 0.72rem;
    max-width: 38%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    .viewer-toolbar,
    .viewer-body {
      grid-template-columns: 1fr;
      align-items: stretch;
    }

    .viewer-toolbar {
      flex-direction: column;
      align-items: stretch;
    }

    .candidate-queue {
      max-height: 42%;
      border-right: 0;
      border-bottom: 1px solid rgba(148, 163, 184, 0.18);
    }

    .preview-header {
      flex-direction: column;
    }

    .preview-header code {
      max-width: 100%;
    }
  }
</style>
