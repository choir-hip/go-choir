<script context="module" lang="ts">
  let menuSeq = 0;
</script>

<script lang="ts">
  import { afterUpdate, createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import AutopaperExpand from './AutopaperExpand.svelte';

  export let toolbarHidden = false;
  export let versionLabel = '';
  export let previousVersionLabel = '';
  export let nextRevisionLabel = '';
  export let previousDisabled = true;
  export let nextDisabled = true;
  export let revisionLineLabel = '';
  export let stateLabel = '';
  export let isPublishedReader = false;
  /** False for a read-only published story: the reader offers no write action. */
  export let allowPublishedEdit = true;
  export let isPublishedMode = false;
  export let isViewingHistorical = false;
  export let currentUser = null;
  export let loading = false;
  export let submitting = false;
  export let agentPending = false;
  export let cancelPending = false;
  export let restorePending = false;
  export let publishedActionPending = false;
  export let publishMenuOpen = false;
  export let promptLabel = 'Revise';
  export let sourceCandidateCount = 0;
  // A published Texture's numbered sources, shown above its text when open.
  export let publishedSourceCount = 0;
  export let publishedSourcesOpen = false;
  export let hasCurrentDoc = false;
  export let hasCurrentRevision = false;
  /**
   * The bare route of the published story this window reads. With one, the
   * toolbar offers "More research" (AutopaperExpand shows nothing unless the
   * proxy says this reader may ask). Empty for a private document.
   */
  export let expandRoute = '';

  const dispatch = createEventDispatcher();

  $: navDisabled = loading || submitting;
  $: previousAriaLabel = previousVersionLabel ? `Older version (${previousVersionLabel})` : 'At oldest version';
  $: nextAriaLabel = nextRevisionLabel ? `Newer version (${nextRevisionLabel})` : 'At latest version';
  $: previousTitle = previousVersionLabel ? `Go to ${previousVersionLabel}` : 'At oldest version';
  $: nextTitle = nextRevisionLabel ? `Go to ${nextRevisionLabel}` : 'At latest version';
  $: publishDisabled = loading || submitting || agentPending || publishedActionPending || !hasCurrentDoc;
  $: normalizedRevisionLineLabel = String(revisionLineLabel || '').trim().toLowerCase();
  $: normalizedStateLabel = String(stateLabel || '').trim().toLowerCase();
  $: visibleStateLabel = normalizedStateLabel && normalizedStateLabel !== normalizedRevisionLineLabel ? stateLabel : '';

  // The toolbar is one row. The version controls stay; every other control is
  // an item (data-tb-item) in .doc-actions with a priority (1 is kept longest;
  // ties keep their row order, data-tb-slot). When the items are wider than the
  // row, the lowest-priority ones are moved, as the same DOM nodes, into the
  // "more" menu, so there is one copy of each control and its state. The fit is
  // measured, never a breakpoint: a ResizeObserver on the row and its items
  // re-runs it for window width, browser zoom, large text and label changes.
  const menuId = `texture-toolbar-menu-${++menuSeq}`;
  let rowEl: HTMLElement;
  let versionEl: HTMLElement;
  let stateEl: HTMLElement;
  let actionsEl: HTMLElement;
  let ghostEl: HTMLElement;
  let moreEl: HTMLElement;
  let moreBtnEl: HTMLButtonElement;
  let menuEl: HTMLElement;
  let overflowing = false;
  let menuOpen = false;
  // The fate line of "More research", shown as a status row under the controls
  // wherever the button itself sits (row or menu).
  let expandLine = '';
  let observer: ResizeObserver | null = null;
  const observed = new Set<Element>();

  $: if (!(isPublishedReader && expandRoute)) expandLine = '';
  $: if (toolbarHidden || !overflowing) menuOpen = false;

  const asItems = (host: HTMLElement | undefined) =>
    host ? (Array.from(host.children) as HTMLElement[]).filter((el) => el.hasAttribute('data-tb-item')) : [];
  const num = (el: HTMLElement, key: 'tbSlot' | 'tbPriority') => Number(el.dataset[key]);
  const bySlot = (a: HTMLElement, b: HTMLElement) => num(a, 'tbSlot') - num(b, 'tbSlot');
  const byPriority = (a: HTMLElement, b: HTMLElement) => num(a, 'tbPriority') - num(b, 'tbPriority') || bySlot(a, b);
  const px = (value: string) => parseFloat(value) || 0;
  const sameList = (a: HTMLElement[], b: HTMLElement[]) => a.length === b.length && a.every((el, i) => el === b[i]);

  function fit() {
    if (!rowEl || !actionsEl || !menuEl || !ghostEl || !moreBtnEl) return;
    const rowWidth = rowEl.clientWidth;
    if (!rowWidth) return;
    // An item with nothing in it (a control that renders nothing for this
    // reader) takes no room and needs no menu.
    const all = [...asItems(actionsEl), ...asItems(menuEl)].filter((el) => el.firstElementChild);

    // Natural widths come from inert copies in an off-screen row, so the real
    // controls are not moved just to be measured.
    ghostEl.replaceChildren(...all.map((el) => el.cloneNode(true)), moreBtnEl.cloneNode(true));
    const widths = all.map((_, i) => ghostEl.children[i].getBoundingClientRect().width);
    const moreWidth = ghostEl.lastElementChild?.getBoundingClientRect().width || 0;
    ghostEl.replaceChildren();

    const rowGap = px(getComputedStyle(rowEl).columnGap);
    const itemGap = px(getComputedStyle(actionsEl).columnGap);
    const stateGap = stateEl && getComputedStyle(stateEl).display !== 'none' ? rowGap : 0;
    const room = rowWidth - versionEl.getBoundingClientRect().width - rowGap - stateGap;
    const need = widths.reduce((sum, w) => sum + w, 0) + itemGap * Math.max(0, all.length - 1);

    let keep = all;
    if (need > room) {
      // Keep controls in priority order until the next one does not fit; a
      // smaller, lower-priority control never jumps the queue.
      const left = room - rowGap - moreWidth;
      const kept: HTMLElement[] = [];
      let used = 0;
      for (const el of [...all].sort(byPriority)) {
        const add = widths[all.indexOf(el)] + (kept.length ? itemGap : 0);
        if (used + add > left) break;
        kept.push(el);
        used += add;
      }
      keep = kept;
    }
    const out = all.filter((el) => !keep.includes(el)).sort(byPriority);
    const inRow = keep.slice().sort(bySlot);

    if (!sameList(asItems(menuEl).filter((el) => el.firstElementChild), out) || !sameList(asItems(actionsEl).filter((el) => el.firstElementChild), inRow)) {
      const active = document.activeElement as HTMLElement | null;
      for (const el of inRow) {
        if (el.parentElement === actionsEl) continue;
        const next = asItems(actionsEl).find((other) => num(other, 'tbSlot') > num(el, 'tbSlot'));
        actionsEl.insertBefore(el, next || null);
      }
      for (const el of out) if (el.parentElement !== menuEl) menuEl.appendChild(el);
      if (!sameList(asItems(menuEl).filter((el) => el.firstElementChild), out)) for (const el of out) menuEl.appendChild(el);
      // Moving a node drops its focus; the user keeps their place.
      if (active && active !== document.activeElement && (actionsEl.contains(active) || menuEl.contains(active))) active.focus({ preventScroll: true });
    }
    overflowing = out.length > 0;
    watch();
  }

  // Observe the row, the version controls and every item (wherever it sits).
  function watch() {
    if (!observer) return;
    const want = new Set<Element>([rowEl, versionEl, ...asItems(actionsEl), ...asItems(menuEl)]);
    for (const el of observed) if (!want.has(el)) { observer.unobserve(el); observed.delete(el); }
    for (const el of want) if (el && !observed.has(el)) { observer.observe(el); observed.add(el); }
  }

  onMount(() => {
    observer = new ResizeObserver(() => fit());
    fit();
    // Web fonts change the widths after the first measure.
    void document.fonts?.ready.then(fit);
  });
  afterUpdate(fit);
  onDestroy(() => {
    observer?.disconnect();
    observer = null;
    observed.clear();
  });

  const menuFocusables = () =>
    menuEl
      ? (Array.from(menuEl.querySelectorAll('button:not(:disabled), summary')) as HTMLElement[]).filter(
          (el) => el.getClientRects().length && (el.tagName === 'SUMMARY' || !el.closest('details:not([open])')),
        )
      : [];

  async function toggleMenu() {
    menuOpen = !menuOpen;
    if (!menuOpen) return;
    await tick();
    menuFocusables()[0]?.focus();
  }

  function closeMenu(refocus = false) {
    // A publish confirmation that opened inside the menu closes with it.
    if (menuOpen && publishMenuOpen && menuEl.querySelector('[data-texture-publish-menu]')) dispatch('publish-cancel');
    menuOpen = false;
    if (refocus) moreBtnEl?.focus();
  }

  function onMoreKeydown(event: KeyboardEvent) {
    if (!menuOpen) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      closeMenu(true);
      return;
    }
    if (!menuEl.contains(event.target as Node)) return;
    const list = menuFocusables();
    const at = list.indexOf(document.activeElement as HTMLElement);
    const target =
      event.key === 'ArrowDown' ? list[(at + 1) % list.length]
      : event.key === 'ArrowUp' ? list[(at - 1 + list.length) % list.length]
      : event.key === 'Home' ? list[0]
      : event.key === 'End' ? list[list.length - 1]
      : null;
    if (!target) return;
    event.preventDefault();
    target.focus();
  }

  // Doing something from the menu closes it, except the controls that open
  // something of their own (Download, Publish).
  function onMenuClick(event: MouseEvent) {
    const button = (event.target as HTMLElement).closest('button');
    if (button && menuEl.contains(button) && !button.hasAttribute('aria-haspopup')) closeMenu(true);
  }

  function onWindowPointerDown(event: PointerEvent) {
    if (menuOpen && moreEl && !moreEl.contains(event.target as Node)) closeMenu();
  }

  function onMoreFocusOut(event: FocusEvent) {
    const next = event.relatedTarget as Node | null;
    if (menuOpen && next && !moreEl.contains(next)) closeMenu();
  }
</script>

<svelte:window on:pointerdown={onWindowPointerDown} />

<div class="doc-toolbar" class:toolbar-hidden={toolbarHidden} data-texture-toolbar>
  <div class="tb-row" bind:this={rowEl}>
  <div class="version-controls" bind:this={versionEl}>
    <span class="nav-version" data-texture-version>{versionLabel}</span>
    <button
      class="nav-btn"
      data-texture-prev
      aria-label={previousAriaLabel}
      title={previousTitle}
      on:click={() => dispatch('prev')}
      disabled={navDisabled || previousDisabled}
    >
      &lt;
    </button>
    <button
      class="nav-btn"
      data-texture-next
      aria-label={nextAriaLabel}
      title={nextTitle}
      on:click={() => dispatch('next')}
      disabled={navDisabled || nextDisabled}
    >
      &gt;
    </button>
    <span class="draft-line" data-texture-draft-line>{revisionLineLabel}</span>
  </div>

  <div class="doc-state" class:doc-state-empty={!visibleStateLabel} data-texture-state aria-hidden={!visibleStateLabel} bind:this={stateEl}>
    {visibleStateLabel}
  </div>

  <div class="doc-actions" bind:this={actionsEl}>
    {#if isPublishedReader && expandRoute}
      <div class="tb-item" data-tb-item data-tb-slot="0" data-tb-priority="2">
        <AutopaperExpand route={expandRoute} showLine={false} bind:line={expandLine} />
      </div>
    {/if}
    {#if isPublishedReader}
      {#if publishedSourceCount}
        <div class="tb-item" data-tb-item data-tb-slot="2" data-tb-priority="5">
          <button
            class="secondary-action"
            class:active={publishedSourcesOpen}
            data-texture-published-sources-toggle
            aria-pressed={publishedSourcesOpen}
            on:click={() => dispatch('sources')}
            disabled={loading}
          >
            <span class="label-full">Sources {publishedSourceCount}</span>
            <span class="label-compact">Sources</span>
            <span class="label-tiny">Sources</span>
          </button>
        </div>
      {/if}
      <div class="tb-item" data-tb-item data-tb-slot="3" data-tb-priority="6">
        <button
          class="secondary-action"
          data-texture-copy-full-text
          on:click={() => dispatch('copy-full-text')}
          disabled={loading || publishedActionPending}
        >
          Copy text
        </button>
      </div>
      <div class="tb-item" data-tb-item data-tb-slot="4" data-tb-priority="7">
        <details class="download-menu" data-texture-download-menu>
          <summary>Download</summary>
          <button type="button" data-texture-download-md on:click={() => dispatch('download', 'md')} disabled={loading || publishedActionPending}>Markdown</button>
          <button type="button" data-texture-download-txt on:click={() => dispatch('download', 'txt')} disabled={loading || publishedActionPending}>Text</button>
          <button type="button" data-texture-download-html on:click={() => dispatch('download', 'html')} disabled={loading || publishedActionPending}>HTML</button>
          <button type="button" data-texture-download-docx on:click={() => dispatch('download', 'docx')} disabled={loading || publishedActionPending}>DOCX</button>
          <button type="button" data-texture-download-pdf on:click={() => dispatch('download', 'pdf')} disabled={loading || publishedActionPending}>PDF</button>
        </details>
      </div>
      {#if allowPublishedEdit}
        <div class="tb-item" data-tb-item data-tb-slot="5" data-tb-priority="3">
          <button
            class="prompt-btn"
            data-texture-edit-published
            on:click={() => dispatch('edit-published')}
            disabled={loading || publishedActionPending}
          >
            <span class="label-full">{publishedActionPending ? 'Opening…' : currentUser ? 'Edit my version' : 'Edit'}</span>
            <span class="label-compact">{publishedActionPending ? 'Opening…' : 'Edit'}</span>
            <span class="label-tiny">{publishedActionPending ? '…' : 'E'}</span>
          </button>
        </div>
      {/if}
    {:else}
      {#if agentPending}
        <div class="tb-item" data-tb-item data-tb-slot="1" data-tb-priority="2">
          <button
            class="secondary-action danger"
            data-texture-cancel-revision
            on:click={() => dispatch('cancel-revision')}
            disabled={cancelPending}
          >
            <span class="label-full">{cancelPending ? 'Cancelling…' : 'Cancel'}</span>
            <span class="label-compact">{cancelPending ? '…' : 'Cancel'}</span>
            <span class="label-tiny">{cancelPending ? '…' : 'X'}</span>
          </button>
        </div>
      {/if}
      {#if isPublishedMode}
        {#if publishedSourceCount}
          <div class="tb-item" data-tb-item data-tb-slot="2" data-tb-priority="5">
            <button
              class="secondary-action"
              class:active={publishedSourcesOpen}
              data-texture-published-sources-toggle
              aria-pressed={publishedSourcesOpen}
              on:click={() => dispatch('sources')}
              disabled={loading}
            >
              <span class="label-full">Sources {publishedSourceCount}</span>
              <span class="label-compact">Sources</span>
              <span class="label-tiny">Sources</span>
            </button>
          </div>
        {/if}
        <div class="tb-item" data-tb-item data-tb-slot="6" data-tb-priority="4">
          <button
            class="secondary-action"
            data-texture-submit-proposal
            on:click={() => dispatch('submit-proposal')}
            disabled={loading || submitting || agentPending || publishedActionPending || !hasCurrentDoc}
          >
            <span class="label-full">{publishedActionPending ? 'Submitting…' : 'Propose'}</span>
            <span class="label-compact">{publishedActionPending ? '…' : 'Propose'}</span>
            <span class="label-tiny">{publishedActionPending ? '…' : 'Propose'}</span>
          </button>
        </div>
      {:else}
        <div class="tb-item" data-tb-item data-tb-slot="2" data-tb-priority="5">
          <button
            class="secondary-action"
            data-texture-source-panel
            on:click={() => dispatch('sources')}
            disabled={loading || submitting || agentPending || !hasCurrentDoc}
          >
            <span class="label-full">Sources{sourceCandidateCount ? ` ${sourceCandidateCount}` : ''}</span>
            <span class="label-compact">Sources</span>
            <span class="label-tiny">Sources</span>
          </button>
        </div>
        {#if isViewingHistorical}
          <div class="tb-item" data-tb-item data-tb-slot="7" data-tb-priority="2">
            <button
              class="secondary-action"
              data-texture-restore-version
              on:click={() => dispatch('restore')}
              disabled={loading || submitting || agentPending || restorePending || !hasCurrentRevision}
            >
              <span class="label-full">{restorePending ? 'Restoring…' : 'Restore'}</span>
              <span class="label-compact">{restorePending ? '…' : 'Restore'}</span>
              <span class="label-tiny">{restorePending ? '…' : 'Restore'}</span>
            </button>
          </div>
        {/if}
        <div class="tb-item" data-tb-item data-tb-slot="8" data-tb-priority="4">
          <div class="publish-menu-wrap">
            <button
              class="secondary-action publish-action"
              data-texture-publish
              aria-haspopup="menu"
              aria-expanded={publishMenuOpen}
              on:click={() => dispatch('toggle-publish')}
              disabled={publishDisabled}
            >
              <span class="label-full">{publishedActionPending ? 'Publishing…' : `Publish ${versionLabel}`}</span>
              <span class="label-compact">{publishedActionPending ? '…' : 'Publish'}</span>
              <span class="label-tiny">{publishedActionPending ? '…' : 'Publish'}</span>
            </button>
            {#if publishMenuOpen}
              <div class="publish-menu" data-texture-publish-menu role="menu" aria-label="Publish this version">
                <div class="publish-menu-heading">
                  <p class="eyebrow">Publish</p>
                  <h3>Publish {versionLabel}</h3>
                  <p>This creates a public link with the current text and source snapshots.</p>
                  <p class="publish-menu-opinion-note">Or list it in the Autopaper Opinion section, under a short author handle (never your email).</p>
                </div>
                <div class="publish-menu-actions">
                  <button
                    type="button"
                    class="primary-action"
                    data-texture-publish-confirm
                    on:click={() => dispatch('publish-confirm')}
                    disabled={publishedActionPending}
                  >
                    {publishedActionPending ? 'Publishing…' : 'Publish'}
                  </button>
                  <button
                    type="button"
                    class="secondary-action"
                    data-texture-publish-opinion
                    on:click={() => dispatch('publish-opinion-confirm')}
                    disabled={publishedActionPending}
                  >
                    {publishedActionPending ? 'Publishing…' : 'Publish as opinion'}
                  </button>
                  <button
                    type="button"
                    class="secondary-action"
                    data-texture-publish-cancel
                    on:click={() => dispatch('publish-cancel')}
                    disabled={publishedActionPending}
                  >
                    Cancel
                  </button>
                </div>
              </div>
            {/if}
          </div>
        </div>
      {/if}
      <div class="tb-item tb-item-end" data-tb-item data-tb-slot="9" data-tb-priority="3">
        <button
          class="prompt-btn revise-action"
          data-texture-prompt
          data-texture-save
          on:click={() => dispatch('prompt')}
          disabled={loading || submitting || agentPending || isViewingHistorical || publishedActionPending}
        >
          <span class="label-full">{promptLabel}</span>
          <span class="label-compact">{promptLabel}</span>
          <span class="label-tiny">{submitting || agentPending ? '…' : promptLabel}</span>
        </button>
      </div>
    {/if}
  </div>

  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="tb-more" class:tb-more-off={!overflowing} bind:this={moreEl} on:keydown={onMoreKeydown} on:focusout={onMoreFocusOut}>
    <button
      type="button"
      class="more-btn"
      data-texture-toolbar-more
      aria-label="More toolbar actions"
      title="More"
      aria-haspopup="menu"
      aria-expanded={menuOpen}
      aria-controls={menuId}
      bind:this={moreBtnEl}
      on:click={toggleMenu}
    >
      <span aria-hidden="true">⋯</span>
    </button>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <div
      class="tb-menu"
      class:tb-menu-open={menuOpen}
      id={menuId}
      role="menu"
      tabindex="-1"
      aria-label="More toolbar actions"
      data-texture-toolbar-menu
      bind:this={menuEl}
      on:click={onMenuClick}
    ></div>
  </div>
  </div>

  {#if expandLine}
    <div class="tb-status" data-autopaper-expand-line role="status" aria-live="polite">{expandLine}</div>
  {/if}

  <!-- Off-screen, inert copies of the items, only to read their natural widths. -->
  <div class="doc-actions tb-ghost" aria-hidden="true" bind:this={ghostEl}></div>
</div>

<style>
  .doc-toolbar {
    position: relative;
    z-index: 10;
    flex: 0 0 auto;
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 0.3rem;
    /* The controls are one row (.tb-row); what does not fit goes into the
       "more" menu. A status line may sit under the row. max-height is the
       number the hide transition animates from. */
    height: auto;
    min-height: 3.7rem;
    padding: 0.5rem 0.72rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    max-height: 24rem;
    overflow: visible;
    container-type: inline-size;
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
    min-height: 3.7rem;
    max-height: 24rem;
    padding-top: 0.5rem;
    padding-bottom: 0.5rem;
    border-bottom-color: var(--choir-border-strong);
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
  }

  .doc-toolbar.toolbar-hidden:focus-within > * {
    visibility: visible;
  }

  .tb-row {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    min-width: 0;
  }

  .version-controls,
  .doc-actions {
    display: flex;
    align-items: center;
    gap: 0.42rem;
    min-width: 0;
    flex-wrap: nowrap;
  }

  /* fit() gives each item its room, so the row never wraps or shrinks them. */
  .doc-actions {
    flex: 1 1 0;
    justify-content: flex-start;
  }

  /* Only a very large text size squeezes the version label, so that the
     "more" button never leaves the row. */
  .version-controls {
    flex: 0 1 auto;
  }

  .tb-item {
    display: flex;
    align-items: center;
    flex: 0 0 auto;
    min-width: 0;
  }

  .tb-item:not(:has(*)) {
    display: none;
  }

  .tb-item-end {
    margin-left: auto;
  }

  .tb-ghost {
    position: absolute;
    top: 0;
    left: 0;
    flex: none;
    width: max-content;
    height: 0;
    overflow: hidden;
    visibility: hidden;
    pointer-events: none;
  }

  .tb-status {
    color: var(--choir-text-secondary, var(--choir-text-muted));
    font-size: 0.74rem;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .tb-more {
    position: relative;
    display: inline-flex;
    flex: 0 0 auto;
    align-items: center;
  }

  .tb-more-off {
    display: none;
  }

  .tb-menu {
    display: none;
    position: absolute;
    top: calc(100% + 0.4rem);
    right: 0;
    z-index: 12;
    flex-direction: column;
    gap: 0.25rem;
    width: max-content;
    min-width: min(12rem, calc(100cqw - 1.5rem));
    max-width: min(18rem, calc(100cqw - 1.5rem));
    max-height: min(32rem, 70vh);
    overflow-y: auto;
    padding: 0.4rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.65rem;
    background: var(--choir-surface-pane);
    box-shadow: var(--choir-shadow-floating);
  }

  .tb-menu.tb-menu-open {
    display: flex;
  }

  .doc-state {
    flex: 0 1 auto;
    min-width: 0;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--choir-text-accent);
    font-size: 0.74rem;
  }

  .doc-state-empty {
    visibility: hidden;
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
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
  }

  .draft-line {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 7.6rem;
    max-width: 100%;
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
    flex: 0 0 auto;
  }

  .nav-btn,
  .more-btn,
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

  .label-compact,
  .label-tiny {
    display: none;
  }

  .nav-btn,
  .more-btn {
    width: 1.95rem;
    height: 1.95rem;
    border-radius: 999px;
    font-size: 0.92rem;
    font-weight: 700;
    flex: 0 0 auto;
  }

  .more-btn {
    padding: 0;
    line-height: 1;
  }

  .more-btn[aria-expanded='true'] {
    background: var(--choir-accent-soft);
    border-color: var(--choir-accent);
  }

  .prompt-btn {
    border-radius: 999px;
    height: 2.1rem;
    padding: 0.62rem 0.95rem;
    font-size: 0.82rem;
    font-weight: 640;
    line-height: 1;
    white-space: nowrap;
    flex: 0 0 auto;
  }

  .revise-action {
    margin-left: auto;
  }

  .secondary-action {
    border-radius: 999px;
    min-width: 5.8rem;
    height: 2.1rem;
    padding: 0.62rem 0.84rem;
    font-size: 0.78rem;
    font-weight: 620;
    color: var(--choir-text-accent);
    line-height: 1;
    white-space: nowrap;
    flex: 0 0 auto;
  }

  .primary-action {
    border-radius: 999px;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .publish-action {
    min-width: 7.4rem;
  }

  .secondary-action.active {
    border-color: var(--choir-border-strong);
    background: var(--choir-surface-control);
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
    font-weight: 620;
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
    flex: 0 0 auto;
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
  .more-btn:hover:enabled,
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

  @container (max-width: 680px) {
    .doc-toolbar {
      gap: 0.42rem;
      padding-inline: 0.56rem;
    }

    .version-controls,
    .doc-actions {
      gap: 0.32rem;
    }

    .doc-state {
      display: none;
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
      height: 1.95rem;
      padding: 0.5rem 0.7rem;
      font-size: 0.75rem;
      font-weight: 600;
    }

    .secondary-action {
      height: 1.95rem;
      min-width: auto;
      padding: 0.5rem 0.64rem;
      font-size: 0.72rem;
      font-weight: 600;
    }

    .publish-action {
      min-width: auto;
    }

    .publish-menu {
      right: -0.2rem;
      width: min(18rem, calc(100vw - 1.4rem));
    }
  }

  @container (max-width: 560px) {
    .doc-toolbar {
      gap: 0.34rem;
      padding-inline: 0.48rem;
    }

    .version-controls,
    .doc-actions {
      gap: 0.28rem;
    }

    .draft-line {
      max-width: 5.8rem;
    }

    .prompt-btn,
    .secondary-action {
      padding-inline: 0.58rem;
      font-size: 0.69rem;
      letter-spacing: 0;
    }
  }

  @container (max-width: 460px) {
    .draft-line {
      display: none;
    }

    .publish-action {
      min-width: 3.15rem;
    }
  }

  @container (max-width: 380px) {
    .doc-toolbar {
      gap: 0.25rem;
      padding-inline: 0.38rem;
    }

    .version-controls,
    .doc-actions {
      gap: 0.22rem;
    }

    .nav-version {
      width: auto;
      min-width: 2.25rem;
      padding: 0 0.38rem;
    }

    .prompt-btn,
    .secondary-action {
      min-width: 0;
      padding-inline: 0.44rem;
      font-size: 0.64rem;
      font-weight: 560;
    }
  }

  /* (After every base rule above: same specificity, so source order decides.) */
  /* An item in the menu is a full-width row; the control inside is the same
     element that sits in the toolbar, restyled here (:global because the
     items are moved into the menu at run time). */
  .tb-menu :global(.tb-item) {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    margin-left: 0;
  }

  .tb-menu :global(.tb-item:not(:has(*))) {
    display: none;
  }

  .tb-menu :global(.secondary-action),
  .tb-menu :global(.prompt-btn),
  .tb-menu :global(.download-menu > summary),
  .tb-menu :global(.tb-item .ap-expand-button) {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    height: auto;
    min-height: 2.3rem;
    padding: 0.5rem 0.7rem;
    border-radius: 0.5rem;
    text-align: left;
    white-space: normal;
  }

  .tb-menu :global(.download-menu),
  .tb-menu :global(.publish-menu-wrap) {
    display: flex;
    flex-direction: column;
    align-items: stretch;
  }

  .tb-menu :global(.download-menu[open]::after) {
    display: none;
  }

  /* The publish confirmation opens inside the menu, not over the page. */
  .tb-menu :global(.publish-menu) {
    position: static;
    width: auto;
    margin-top: 0.4rem;
  }

  .tb-menu :global(.publish-menu-actions) {
    flex-wrap: wrap;
  }

  /* A finger needs 44 px: on a touch screen every toolbar control grows to
     it (in px, so large text does not also double the tap size). */
  @media (pointer: coarse) {
    .nav-btn,
    .more-btn {
      width: max(1.95rem, 44px);
      height: max(1.95rem, 44px);
    }

    .prompt-btn,
    .secondary-action,
    .tb-menu :global(.tb-item .ap-expand-button) {
      height: auto;
      min-height: 44px;
    }

    .download-menu > summary {
      display: inline-flex;
      align-items: center;
      min-height: 44px;
    }

    .download-menu button {
      min-height: 44px;
    }
  }
</style>
