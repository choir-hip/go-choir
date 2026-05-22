<script>
  import { createEventDispatcher, onDestroy } from 'svelte';

  export let candidateDesktopId = '';
  export let title = 'Candidate preview';

  const dispatch = createEventDispatcher();
  let iframeEl;
  let pollTimer;
  let previewState = 'empty';
  let previewMessage = 'Try a change to open a candidate computer without mutating your active computer.';

  $: normalizedDesktopId = String(candidateDesktopId || '').trim();
  $: previewSrc = normalizedDesktopId
    ? `/?desktop_id=${encodeURIComponent(normalizedDesktopId)}&embedded=1`
    : '';
  $: if (!previewSrc) {
    clearPreviewPoll();
    publishPreviewState('empty', 'No candidate preview yet');
  }

  function publishPreviewState(state, message = '') {
    previewState = state;
    previewMessage = message || state;
    dispatch('previewstate', {
      state,
      message: previewMessage,
      candidateDesktopId: normalizedDesktopId,
    });
  }

  function clearPreviewPoll() {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = undefined;
    }
  }

  function inspectPreviewDocument() {
    if (!iframeEl?.contentDocument) return;
    const doc = iframeEl.contentDocument;
    const text = (doc.body?.innerText || '').trim();
    if (doc.querySelector('[data-desktop][data-desktop-ready="true"]')) {
      clearPreviewPoll();
      publishPreviewState('ready', 'Candidate desktop is ready');
      return;
    }
    if (/VM route returned 5\d\d|BOOTSTRAP FAILED|authentication required|failed to resolve|Computer boot is still pending/i.test(text)) {
      publishPreviewState('blocked', text.split('\n').find(Boolean) || 'Candidate preview is blocked');
      return;
    }
    if (/still waiting|retrying|Resolving active computer|Powering user computer|Candidate computer route found/i.test(text)) {
      publishPreviewState('booting', text.split('\n').slice(-1)[0] || 'Candidate preview is still booting');
      return;
    }
    publishPreviewState('loading', 'Candidate preview is loading');
  }

  function handleFrameLoad() {
    clearPreviewPoll();
    publishPreviewState('loading', 'Candidate preview is loading');
    inspectPreviewDocument();
    pollTimer = setInterval(inspectPreviewDocument, 1500);
  }

  onDestroy(clearPreviewPoll);
</script>

<section
  class="change-preview-frame"
  data-change-preview-frame
  data-change-preview-desktop-id={normalizedDesktopId}
  data-change-preview-state={previewState}
>
  {#if previewSrc}
    <iframe
      bind:this={iframeEl}
      class="candidate-frame"
      data-change-preview-iframe
      src={previewSrc}
      title={title}
      sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
      on:load={handleFrameLoad}
    ></iframe>
    {#if previewState !== 'ready'}
      <div class:blocked={previewState === 'blocked'} class="preview-state" data-change-preview-status>
        <strong>{previewState === 'blocked' ? 'Preview blocked' : 'Preview starting'}</strong>
        <span>{previewMessage}</span>
      </div>
    {/if}
  {:else}
    <div class="preview-empty" data-change-preview-empty>
      <strong>No candidate preview yet</strong>
      <span>{previewMessage}</span>
    </div>
  {/if}
</section>

<style>
  .change-preview-frame {
    position: relative;
    display: flex;
    min-height: 0;
    height: 100%;
    border: 1px solid rgba(96, 165, 250, 0.24);
    border-radius: 8px;
    overflow: hidden;
    background: #050914;
  }

  .preview-state {
    position: absolute;
    inset: auto 14px 14px 14px;
    display: grid;
    gap: 4px;
    padding: 10px 12px;
    border: 1px solid rgba(96, 165, 250, 0.35);
    border-radius: 8px;
    background: rgba(6, 10, 22, 0.88);
    color: #dbeafe;
    box-shadow: 0 18px 40px rgba(0, 0, 0, 0.3);
  }

  .preview-state.blocked {
    border-color: rgba(248, 113, 113, 0.48);
    color: #fecaca;
  }

  .preview-state span {
    color: #94a3b8;
    font-size: 12px;
    line-height: 1.35;
    max-height: 3.2em;
    overflow: hidden;
  }

  .candidate-frame {
    width: 100%;
    height: 100%;
    min-height: 0;
    border: 0;
    background: #050914;
  }

  .preview-empty {
    display: grid;
    place-content: center;
    gap: 8px;
    width: 100%;
    min-height: 260px;
    padding: 24px;
    color: #dbeafe;
    text-align: center;
  }

  .preview-empty span {
    max-width: 360px;
    color: #94a3b8;
  }
</style>
