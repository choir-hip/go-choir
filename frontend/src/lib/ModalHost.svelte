<script lang="ts">
  // Draws the one open modal (lib/modal.ts): backdrop, dialog, focus trap, Esc
  // and backdrop-click cancel, focus back to the opener. Full-screen sheet at
  // phone width. The desktop mounts this once.
  //   data-modal-backdrop  the backdrop
  //   data-modal           the dialog (role=dialog, aria-modal)
  import { onDestroy, onMount, tick } from 'svelte';
  import { closeModal, modalState, registerModalHost } from './modal';

  let dialogEl: HTMLElement | null = null;
  let shownSeq = 0;
  let opener: HTMLElement | null = null;

  const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

  const focusables = () =>
    dialogEl ? [...dialogEl.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => el.offsetParent !== null || el === document.activeElement) : [];

  async function settleFocus() {
    await tick();
    if (!dialogEl) return;
    const preferred = dialogEl.querySelector<HTMLElement>('[data-modal-autofocus]');
    (preferred || focusables()[0] || dialogEl).focus({ preventScroll: true });
  }

  $: if ($modalState && $modalState.seq !== shownSeq) {
    shownSeq = $modalState.seq;
    opener = $modalState.opener;
    void settleFocus();
  }

  let wasOpen = false;
  $: {
    const open = Boolean($modalState);
    if (wasOpen && !open) {
      const back = opener;
      opener = null;
      shownSeq = 0;
      // M4: an opener that left the page is skipped.
      queueMicrotask(() => { if (back && back.isConnected) back.focus({ preventScroll: true }); });
    }
    wasOpen = open;
  }

  function onKeydown(event: KeyboardEvent) {
    if (!$modalState) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      closeModal(undefined);
      return;
    }
    if (event.key === 'Tab' && dialogEl) {
      const items = focusables();
      if (!items.length) { event.preventDefault(); dialogEl.focus(); return; }
      const first = items[0];
      const last = items[items.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (!dialogEl.contains(active)) { event.preventDefault(); first.focus(); }
      else if (event.shiftKey && (active === first || active === dialogEl)) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && active === last) { event.preventDefault(); first.focus(); }
    }
  }

  function onFocusIn(event: FocusEvent) {
    if (!$modalState || !dialogEl) return;
    if (!dialogEl.contains(event.target as Node)) (focusables()[0] || dialogEl).focus({ preventScroll: true });
  }

  let removeHost = () => {};
  onMount(() => {
    removeHost = registerModalHost();
    window.addEventListener('keydown', onKeydown, true);
    document.addEventListener('focusin', onFocusIn);
  });
  onDestroy(() => {
    removeHost();
    if (typeof window !== 'undefined') window.removeEventListener('keydown', onKeydown, true);
    if (typeof document !== 'undefined') document.removeEventListener('focusin', onFocusIn);
  });

  function onBackdropPointerDown(event: PointerEvent) {
    if (event.target === event.currentTarget) closeModal(undefined);
  }
</script>

{#if $modalState}
  {#key $modalState.seq}
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="modal-backdrop" data-modal-backdrop on:pointerdown={onBackdropPointerDown}>
      <div
        class="modal-dialog"
        data-modal
        role="dialog"
        aria-modal="true"
        aria-label={$modalState.label}
        tabindex="-1"
        bind:this={dialogEl}
      >
        <svelte:component this={$modalState.component} {...$modalState.props} close={closeModal} />
      </div>
    </div>
  {/key}
{/if}

<style>
  /* Above every window and the prompt bar; the context menu (2147483000) stays above. */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    z-index: 2147482000;
    display: grid;
    place-items: center;
    padding: 24px;
    background: color-mix(in srgb, var(--choir-shadow-color) 52%, transparent);
    backdrop-filter: blur(3px);
  }

  .modal-dialog {
    display: flex;
    flex-direction: column;
    width: min(720px, 100%);
    max-height: min(640px, 100%);
    overflow: hidden;
    border: 1px solid var(--choir-border-strong);
    border-radius: var(--choir-radius-panel, 18px);
    background: var(--choir-panel, var(--choir-bg));
    color: var(--choir-text-accent, inherit);
    box-shadow: 0 24px 80px color-mix(in srgb, var(--choir-shadow-color) 50%, transparent);
    outline: none;
  }

  /* Phone: a full-screen sheet. */
  @media (max-width: 640px) {
    .modal-backdrop {
      padding: 0;
      place-items: stretch;
    }
    .modal-dialog {
      width: 100%;
      max-height: none;
      height: 100%;
      border: 0;
      border-radius: 0;
    }
  }
</style>
