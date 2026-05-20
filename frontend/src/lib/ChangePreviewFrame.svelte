<script>
  export let candidateDesktopId = '';
  export let title = 'Candidate preview';

  $: normalizedDesktopId = String(candidateDesktopId || '').trim();
  $: previewSrc = normalizedDesktopId
    ? `/?desktop_id=${encodeURIComponent(normalizedDesktopId)}&embedded=1`
    : '';
</script>

<section
  class="change-preview-frame"
  data-change-preview-frame
  data-change-preview-desktop-id={normalizedDesktopId}
>
  {#if previewSrc}
    <iframe
      class="candidate-frame"
      data-change-preview-iframe
      src={previewSrc}
      title={title}
      sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
    ></iframe>
  {:else}
    <div class="preview-empty" data-change-preview-empty>
      <strong>No candidate preview yet</strong>
      <span>Try a change to open a candidate computer without mutating your active computer.</span>
    </div>
  {/if}
</section>

<style>
  .change-preview-frame {
    display: flex;
    min-height: 0;
    height: 100%;
    border: 1px solid rgba(96, 165, 250, 0.24);
    border-radius: 8px;
    overflow: hidden;
    background: #050914;
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
