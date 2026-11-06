<script lang="ts">
  // "Who provides this?" (slice L2): for each part of the shell a package
  // draws, the package drawing it now, the owner's recorded choice, the
  // providers set aside with the reason, and the choice itself. "Use default"
  // selects the slot's built-in package and uninstalls nothing. A choice is a
  // record on the owner's computer, so it holds across a restart.
  import { onMount } from 'svelte';
  import { providerSlots, selectProvider, useDefaultProvider, type ProviderSlot } from '../../lib/app-sdk';

  const NAMES: Record<string, string> = {
    'contextMenu.renderer': 'Context menu',
    'desktop.icons': 'Desktop icons',
    launcher: 'Launcher',
    'notifications.host': 'Notifications',
    overview: 'Overview',
    prompt: 'Prompt bar',
  };

  let rows: ProviderSlot[] = [];
  let busy = '';
  let errors: Record<string, string> = {};

  async function refresh() {
    rows = await providerSlots();
  }

  async function act(slot: string, run: () => Promise<{ ok: boolean; error?: string }>) {
    busy = slot;
    const result = await run();
    errors = { ...errors, [slot]: result.ok ? '' : result.error || 'could not save the choice' };
    busy = '';
    await refresh();
  }

  onMount(() => { void refresh(); });
</script>

<ul class="who-list" data-who-provides>
  {#each rows as row (row.slot)}
    <li class="who-row" data-who-slot={row.slot}>
      <div class="who-head">
        <strong>{NAMES[row.slot] || row.slot}</strong>
        <span data-who-active={row.active || ''}>
          {#if row.active}
            Drawn by {row.active}{row.active === row.defaultPackage ? ' (built in)' : ''}
          {:else}
            Choir's last-resort {(NAMES[row.slot] || row.slot).toLowerCase()}
          {/if}
        </span>
      </div>
      {#if row.selected}
        <p class="who-note" data-who-selected={row.selected}>Your choice: {row.selected}{row.selected !== row.active ? ', not drawing it now' : ''}.</p>
      {/if}
      {#if row.providers.length > 1}
        <div class="who-choices" role="group" aria-label={'Who draws the ' + (NAMES[row.slot] || row.slot)}>
          {#each row.providers as p (p.id)}
            <button
              type="button"
              class:active={p.id === row.active}
              data-who-choose={p.id}
              aria-pressed={p.id === row.active}
              disabled={busy === row.slot || !p.healthy}
              on:click={() => act(row.slot, () => selectProvider(row.slot, p.id))}
            >{p.id}{p.isDefault ? ' (built in)' : ''}</button>
          {/each}
        </div>
      {/if}
      {#each row.providers.filter((p) => !p.healthy) as p (p.id)}
        <p class="who-note who-aside" data-who-aside={p.id}>{p.id} was set aside: {p.reason}</p>
      {/each}
      {#if row.defaultPackage && (row.active !== row.defaultPackage || (row.selected && row.selected !== row.defaultPackage))}
        <button type="button" class="secondary-action" data-who-use-default={row.slot} disabled={busy === row.slot} on:click={() => act(row.slot, () => useDefaultProvider(row.slot))}>
          Use default
        </button>
      {/if}
      {#if errors[row.slot]}<p class="who-error" role="alert">{errors[row.slot]}</p>{/if}
    </li>
  {/each}
</ul>

<style>
  .who-list { list-style: none; margin: 0.75rem 0 0; padding: 0; display: grid; gap: 0.75rem; }
  .who-row { display: grid; gap: 0.35rem; }
  .who-head { display: flex; justify-content: space-between; gap: 1rem; flex-wrap: wrap; }
  .who-head span, .who-note { color: var(--choir-muted, #888); font-size: 0.85rem; margin: 0; }
  .who-choices { display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .who-choices button.active { font-weight: 600; }
  .who-error { margin: 0; color: var(--choir-danger, #c0392b); font-size: 0.85rem; }
</style>
