<script lang="ts">
  // The Files picker, in the shell modal (lib/modal.ts). It lists folders with
  // the Files app's own listing (lib/files-listing.ts), so it is the same
  // browser in a modal, not a second one. Resolves with a Files path, or
  // undefined when cancelled (Cancel, Esc, backdrop).
  //   data-file-picker          the picker
  //   data-picker-crumb         a breadcrumb segment (valued by its path)
  //   data-picker-entry         a row (valued by its name), data-entry-type
  //   data-picker-thumb         an image thumbnail
  //   data-picker-filter        the type filter select
  //   data-picker-selected      the chosen path line
  //   data-picker-cancel / data-picker-choose   the two buttons
  import { onMount, tick } from 'svelte';
  import { fileIconFor, formatFileSize, listFolder, type FileEntry } from './files-listing';
  import { apiFileURL } from './media-utils.js';

  export let close: (result?: unknown) => void = () => {};
  export let title = 'Choose a file';
  export let confirmLabel = 'Choose';
  /** Lower-case extensions that may be chosen ([] = any file). */
  export let extensions: string[] = [];
  /** What the extension filter is called ("Images"). */
  export let filterLabel = 'Matching files';
  /** Folder names at the Files root that are never listed (Mail-agent custody). */
  export let hideAtRoot: string[] = [];
  /** Folder to open first, as a Files path ('Pictures/Backgrounds'); falls back to the root. */
  export let startPath = '';

  let path: string[] = [];
  let entries: FileEntry[] = [];
  let loading = true;
  let error = '';
  let selected = '';
  let showAll = false;
  let listEl: HTMLElement | null = null;
  let requestId = 0;
  let firstLoad = true;

  const matches = (name: string) => extensions.length === 0 || extensions.includes(String(name).toLowerCase().split('.').pop() || '');
  $: filtering = extensions.length > 0 && !showAll;
  $: visible = entries
    .filter((e) => !(path.length === 0 && hideAtRoot.some((h) => h.toLowerCase() === String(e.name).toLowerCase())))
    .filter((e) => e.type === 'directory' || !filtering || matches(e.name));
  $: selectedPath = selected ? [...path, selected].join('/') : '';
  $: chooseable = Boolean(selected) && matches(selected);

  async function load(segments: string[], fallbackToRoot = false) {
    const mine = ++requestId;
    loading = true;
    error = '';
    const result = await listFolder(segments);
    if (mine !== requestId) return;
    loading = false;
    if (!result.ok) {
      if (fallbackToRoot && segments.length) return load([], false);
      error = result.error;
      entries = [];
      return;
    }
    path = segments;
    entries = result.entries;
    selected = '';
    await tick();
    // Rows were replaced: keep keyboard focus in the list rather than on the body.
    const active = document.activeElement;
    if (firstLoad || !active || active === document.body || (listEl && !listEl.contains(active) && !active.isConnected)) {
      listEl?.querySelector<HTMLElement>('[data-picker-entry]')?.focus({ preventScroll: true });
    }
    firstLoad = false;
  }

  const open = (name: string) => load([...path, name]);
  const goTo = (depth: number) => load(path.slice(0, depth));

  function activate(entry: FileEntry) {
    if (entry.type === 'directory') void open(entry.name);
    else selected = entry.name;
  }

  function choose(name = selected) {
    if (name && matches(name)) close([...path, name].join('/'));
  }

  async function moveFocus(from: number, delta: number) {
    const rows = listEl ? [...listEl.querySelectorAll<HTMLElement>('[data-picker-entry]')] : [];
    if (!rows.length) return;
    const next = Math.max(0, Math.min(rows.length - 1, from + delta));
    await tick();
    rows[next]?.focus();
  }

  function onRowKeydown(event: KeyboardEvent, entry: FileEntry, index: number) {
    if (event.key === 'ArrowDown') { event.preventDefault(); void moveFocus(index, 1); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); void moveFocus(index, -1); }
    else if (event.key === 'Home') { event.preventDefault(); void moveFocus(0, 0); }
    else if (event.key === 'End') { event.preventDefault(); void moveFocus(visible.length - 1, 0); }
    else if (event.key === 'Enter') {
      event.preventDefault();
      if (entry.type === 'directory') void open(entry.name);
      else { selected = entry.name; choose(entry.name); }
    } else if (event.key === ' ') { event.preventDefault(); activate(entry); }
    else if (event.key === 'ArrowLeft' || event.key === 'Backspace') {
      if (path.length) { event.preventDefault(); void goTo(path.length - 1); }
    } else if (event.key === 'ArrowRight' && entry.type === 'directory') { event.preventDefault(); void open(entry.name); }
  }

  onMount(() => {
    const start = String(startPath || '').split('/').filter(Boolean);
    void load(start, true);
  });
</script>

<div class="picker" data-file-picker>
  <header class="picker-head">
    <h2 class="picker-title">{title}</h2>
    <button type="button" class="picker-x" data-picker-cancel-x aria-label="Cancel" on:click={() => close(undefined)}>×</button>
  </header>

  <nav class="picker-crumbs" aria-label="Folder">
    <button type="button" class="crumb" data-picker-crumb="" on:click={() => goTo(0)}>Files</button>
    {#each path as segment, i}
      <span class="crumb-sep" aria-hidden="true">/</span>
      <button type="button" class="crumb" data-picker-crumb={path.slice(0, i + 1).join('/')} on:click={() => goTo(i + 1)}>{segment}</button>
    {/each}
    {#if extensions.length}
      <label class="picker-filter">
        <span class="sr-only">Show</span>
        <select data-picker-filter bind:value={showAll}>
          <option value={false}>{filterLabel}</option>
          <option value={true}>All files</option>
        </select>
      </label>
    {/if}
  </nav>

  <div class="picker-body" bind:this={listEl} role="listbox" aria-label="Files and folders">
    {#if loading}
      <p class="picker-note">Loading...</p>
    {:else if error}
      <p class="picker-note picker-error" role="alert" data-picker-error>{error}</p>
    {:else if visible.length === 0}
      <p class="picker-note" data-picker-empty>{filtering ? `No ${filterLabel.toLowerCase()} or folders here.` : 'This folder is empty.'}</p>
    {:else}
      {#each visible as entry, i (entry.name)}
        <button
          type="button"
          class="row"
          class:selected={selected === entry.name}
          class:disabled={entry.type !== 'directory' && !matches(entry.name)}
          role="option"
          aria-selected={selected === entry.name}
          data-picker-entry={entry.name}
          data-entry-type={entry.type}
          tabindex={i === 0 ? 0 : -1}
          on:click={() => activate(entry)}
          on:dblclick={() => { if (entry.type !== 'directory') { selected = entry.name; choose(entry.name); } }}
          on:keydown={(event) => onRowKeydown(event, entry, i)}
        >
          {#if entry.type !== 'directory' && extensions.length && matches(entry.name)}
            <img class="thumb" data-picker-thumb loading="lazy" alt="" src={apiFileURL([...path, entry.name].join('/'))} />
          {:else}
            <span class="icon" aria-hidden="true">{fileIconFor(entry)}</span>
          {/if}
          <span class="name">{entry.name}{entry.type === 'directory' ? '/' : ''}</span>
          {#if entry.type !== 'directory'}<span class="size">{formatFileSize(entry.size || 0)}</span>{/if}
        </button>
      {/each}
    {/if}
  </div>

  <footer class="picker-foot">
    <p class="picker-selected" data-picker-selected aria-live="polite">
      {#if selected && !chooseable}
        <span data-picker-refused>{selected} is not {filterLabel.toLowerCase()}; pick another file.</span>
      {:else}
        {selectedPath || 'Nothing chosen'}
      {/if}
    </p>
    <div class="picker-actions">
      <button type="button" class="btn" data-picker-cancel on:click={() => close(undefined)}>Cancel</button>
      <button type="button" class="btn primary" data-picker-choose disabled={!chooseable} on:click={() => choose()}>{confirmLabel}</button>
    </div>
  </footer>
</div>

<style>
  .picker { display: flex; flex-direction: column; min-height: 0; height: 100%; font-size: 0.9rem; }
  .picker-head { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px 6px; }
  .picker-title { margin: 0; font-size: 1.05rem; }
  .picker-x { border: 0; background: none; color: inherit; font-size: 1.4rem; line-height: 1; cursor: pointer; padding: 4px 8px; border-radius: 8px; }
  .picker-crumbs { display: flex; align-items: center; gap: 4px; padding: 6px 16px 10px; overflow-x: auto; white-space: nowrap; border-bottom: 1px solid var(--choir-border-strong); }
  .crumb { border: 0; background: none; color: inherit; cursor: pointer; padding: 4px 8px; border-radius: 8px; font: inherit; }
  .crumb:hover { background: var(--choir-state-hover); }
  .crumb-sep { opacity: 0.5; }
  .picker-filter { margin-left: auto; }
  .picker-filter select { font: inherit; padding: 4px 8px; border-radius: 8px; border: 1px solid var(--choir-border-strong); background: var(--choir-surface-control, transparent); color: inherit; }
  .sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); }
  .picker-body { flex: 1; min-height: 160px; overflow-y: auto; padding: 8px; display: flex; flex-direction: column; gap: 2px; }
  .picker-note { margin: 24px 12px; opacity: 0.75; }
  .picker-error { color: var(--choir-status-danger); opacity: 1; }
  .row { display: flex; align-items: center; gap: 12px; width: 100%; padding: 6px 10px; border: 0; border-radius: 10px; background: none; color: inherit; font: inherit; text-align: left; cursor: pointer; min-height: 48px; }
  .row:hover { background: var(--choir-state-hover); }
  .row:focus-visible { outline: 2px solid var(--choir-accent); outline-offset: -2px; }
  .row.selected { background: var(--choir-state-selected); box-shadow: inset 0 0 0 1px var(--choir-accent); }
  .row.disabled { opacity: 0.55; }
  .icon, .thumb { flex: 0 0 40px; width: 40px; height: 40px; display: grid; place-items: center; font-size: 1.5rem; }
  .thumb { object-fit: cover; border-radius: 6px; background: var(--choir-surface-control); }
  .name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .size { opacity: 0.6; font-size: 0.8rem; }
  .picker-foot { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 16px 14px; border-top: 1px solid var(--choir-border-strong); flex-wrap: wrap; }
  .picker-selected { margin: 0; flex: 1 1 200px; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; opacity: 0.85; }
  .picker-actions { display: flex; gap: 8px; margin-left: auto; }
  .btn { font: inherit; padding: 8px 16px; min-height: 40px; border-radius: 10px; cursor: pointer; border: 1px solid var(--choir-border-strong); background: var(--choir-surface-control, transparent); color: inherit; }
  .btn.primary { background: var(--choir-accent); color: var(--choir-on-accent); border-color: transparent; }
  .btn:disabled { opacity: 0.45; cursor: not-allowed; }
  @media (max-width: 640px) {
    .picker-foot { padding-bottom: calc(14px + env(safe-area-inset-bottom, 0px)); }
    .picker-actions { width: 100%; }
    .picker-actions .btn { flex: 1; }
  }
</style>
