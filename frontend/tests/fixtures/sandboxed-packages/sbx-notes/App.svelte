<script>
  // A stranger's app as the bridge sees it: its own routes through
  // fetchWithRenewal, its own window's title, a toast and its own setting.
  import { onMount } from 'svelte';
  import { fetchWithRenewal, getSetting, setAppContext, toast } from '../../lib/app-sdk';

  export let windowId = '';
  let notes = [];
  let status = 'loading';
  let outside = 'unknown';
  let setting = '';

  onMount(async () => {
    const res = await fetchWithRenewal('/api/app-packages/sbx-notes/notes');
    notes = (await res.json()).notes || [];
    status = 'ready';
    // Another app's route is refused at the bridge, never fetched.
    try {
      await fetchWithRenewal('/api/files/');
      outside = 'allowed';
    } catch (err) {
      outside = 'refused';
    }
    setting = String(await getSetting('greeting').catch(() => 'none'));
  });

  async function add() {
    await fetchWithRenewal('/api/app-packages/sbx-notes/notes', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ text: 'hello from the frame' }),
    });
    notes = [...notes, { text: 'hello from the frame' }];
    // The window id is the bridge's to bind: this names another window and still titles this one.
    await setAppContext({ windowId: 'not-this-window', title: `Notes (${notes.length})` });
    await toast('Note saved');
    status = 'saved';
  }
</script>

<section class="sbx-notes" data-sbx-notes data-window={windowId}>
  <h2>Sandboxed Notes</h2>
  <p data-sbx-status>{status}</p>
  <p data-sbx-outside>{outside}</p>
  <ul>
    {#each notes as note}
      <li data-sbx-note>{note.text}</li>
    {/each}
  </ul>
  <button type="button" data-sbx-add on:click={add}>Add note</button>
</section>

<style>
  .sbx-notes {
    padding: 16px;
    color: var(--choir-fg);
  }
</style>
