<!--
  The launcher (the Desk sheet): the apps, the overview, show desktop and the
  account row. A shell surface of the launcher slot (lib/shell-slots.ts).

  Slot ABI, props: placement ('top' | 'bottom'), apps (the launcher apps:
  { id, name, icon, description }), openCount, authenticated, currentUser,
  showDesktopActive. Event: close ({ restoreFocus }). Everything else is an SDK
  call.
-->
<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import { AccountError, listAccounts, logout, logoutAllAccounts, onWindowEvent, openApp, requireAuth, showDesktop, showOverview, signoutAccount, switchAccount } from '../../lib/app-sdk';

  export let placement: 'top' | 'bottom' = 'bottom';
  export let apps: Array<{ id: string; name: string; icon: string; description: string }> = [];
  export let openCount = 0;
  export let authenticated = false;
  export let currentUser: any = null;
  export let showDesktopActive = false;

  const dispatch = createEventDispatcher();

  // The Accounts section: every account signed in on this browser.
  type AccountRow = { user_id: string; email: string; active: boolean; signed_in: boolean };
  let accounts: AccountRow[] = [];
  let accountsError = '';
  let accountsBusy = false;

  async function loadAccounts() {
    if (!authenticated) return;
    try {
      accounts = await listAccounts();
    } catch (_err) {
      accounts = [];
    }
  }

  async function switchTo(account: AccountRow) {
    if (accountsBusy) return;
    accountsError = '';
    if (account.active) return;
    if (!account.signed_in) {
      close(false);
      requireAuth({ kind: 'add_account', email: account.email });
      return;
    }
    accountsBusy = true;
    try {
      await switchAccount(account.user_id);
    } catch (err) {
      if (err instanceof AccountError && err.reason === 'member_signed_out') {
        accountsError = `${account.email} is signed out. Sign in again to use it.`;
        await loadAccounts();
      } else {
        accountsError = err instanceof Error ? err.message : 'Could not switch accounts.';
      }
      accountsBusy = false;
    }
  }

  async function signOutOf(account: AccountRow) {
    if (accountsBusy) return;
    accountsError = '';
    accountsBusy = true;
    try {
      await signoutAccount(account.user_id);
      await loadAccounts();
    } catch (err) {
      accountsError = err instanceof Error ? err.message : 'Could not sign out.';
    }
    accountsBusy = false;
  }

  async function signOutOfAll() {
    if (accountsBusy) return;
    accountsError = '';
    accountsBusy = true;
    try {
      await logoutAllAccounts();
    } catch (err) {
      accountsError = err instanceof Error ? err.message : 'Could not sign out.';
      accountsBusy = false;
    }
  }

  function addAccount() {
    close(false);
    requireAuth({ kind: 'add_account' });
  }
  let closeButton: HTMLButtonElement | null = null;

  $: normalizedPlacement = placement === 'top' ? 'top' : 'bottom';

  function close(restoreFocus = true) {
    dispatch('close', { restoreFocus });
  }

  function launch(app: { id: string; name: string; icon: string }) {
    openApp(app.id, {
      appName: app.name,
      icon: app.icon,
      appContext: app.id === 'podcast' ? { appHint: 'podcast', windowTitle: 'Podcast' } : {},
    });
    close(false);
  }

  function handleKeydown(event: Event) {
    const key = event as KeyboardEvent;
    if (key.key !== 'Escape') return;
    key.preventDefault();
    key.stopPropagation();
    close();
  }

  const offKeydown = onWindowEvent('keydown', handleKeydown);
  onDestroy(offKeydown);

  onMount(() => {
    tick().then(() => closeButton?.focus());
    void loadAccounts();
  });
</script>

<button class="desk-sheet-backdrop" data-desk-sheet-backdrop type="button" aria-label="Close Desk" on:click={() => close()}></button>
<section class="desk-sheet placement-{normalizedPlacement}" data-desk-sheet role="dialog" aria-modal="true" aria-labelledby="desk-sheet-title">
  <div class="sheet-handle" aria-hidden="true"></div>
  <header>
    <div>
      <p id="desk-sheet-title">Desk</p>
      <h2>{openCount} open</h2>
    </div>
    <button bind:this={closeButton} type="button" data-desk-sheet-close on:click={() => close()}>Close</button>
  </header>

  <button class="overview-card" data-desk-overview type="button" on:click={() => { close(false); showOverview(); }}>
    <span>▦</span>
    <strong>Desktop Overview</strong>
    <small>Switch, suspend, focus, and clean up windows</small>
  </button>

  <section class="desk-section">
    <h3>Apps</h3>
    <div class="app-grid" data-desk-sheet-apps>
      {#each apps as app}
        <button data-desk-sheet-app data-desk-app-id={app.id} type="button" on:click={() => launch(app)}>
          <span>{app.icon}</span>
          <strong>{app.name}</strong>
          <small>{app.description}</small>
        </button>
      {/each}
    </div>
  </section>

  <button class="plain-row" data-desk-show-desktop type="button" on:click={() => { close(false); showDesktop(); }}>
    {showDesktopActive ? 'Restore windows' : 'Show desktop'}
  </button>

  {#if authenticated}
    <section class="desk-section" data-desk-accounts aria-label="Accounts">
      <h3>Accounts</h3>
      <ul class="account-list">
        {#each accounts as account (account.user_id)}
          <li class="account-row" data-desk-account data-desk-account-id={account.user_id} data-active={account.active ? 'true' : 'false'}>
            <button
              type="button"
              class="account-switch"
              data-desk-account-switch
              aria-current={account.active ? 'true' : undefined}
              disabled={accountsBusy}
              on:click={() => switchTo(account)}
            >
              <span class="account-email">{account.email}</span>
              {#if account.active}
                <small data-desk-account-active>Active</small>
              {:else if !account.signed_in}
                <small>Signed out</small>
              {/if}
            </button>
            <button type="button" class="account-signout" data-desk-account-signout disabled={accountsBusy} aria-label="Sign out of {account.email}" on:click={() => signOutOf(account)}>Sign out</button>
          </li>
        {/each}
      </ul>
      {#if accountsError}
        <p class="account-error" data-desk-accounts-error role="alert">{accountsError}</p>
      {/if}
      <div class="account-actions">
        <button type="button" class="plain-row" data-desk-account-add on:click={addAccount}>Add another account</button>
        {#if accounts.length > 1}
          <button type="button" class="plain-row" data-desk-account-signout-all disabled={accountsBusy} on:click={signOutOfAll}>Sign out of all accounts</button>
        {/if}
      </div>
    </section>
  {/if}

  <footer>
    {#if authenticated}
      <span data-prompt-surface-user>{currentUser?.email || 'signed in'}</span>
      <button type="button" data-prompt-surface-logout on:click={() => { close(false); logout(); }}>Sign out</button>
    {:else}
      <span data-prompt-surface-user>Public preview</span>
      <button type="button" data-prompt-surface-login on:click={() => { close(false); requireAuth({ kind: 'sign_in' }); }}>Sign in</button>
    {/if}
  </footer>
</section>

<style>
  .desk-sheet-backdrop {
    position: fixed;
    inset: 0;
    z-index: 9998;
    border: 0;
    background: color-mix(in srgb, var(--choir-bg) 72%, transparent);
    cursor: default;
  }

  .desk-sheet {
    position: fixed;
    left: max(12px, env(safe-area-inset-left));
    right: max(12px, env(safe-area-inset-right));
    z-index: 9999;
    display: grid;
    grid-template-rows: none;
    grid-auto-rows: auto;
    gap: 0.48rem;
    max-height: calc(100dvh - var(--choir-prompt-surface-size, 64px) - 28px);
    overflow: auto;
    overscroll-behavior: contain;
    padding: 0.72rem;
    background: var(--choir-sheet-bg);
    color: var(--choir-fg);
    box-shadow: var(--choir-shadow-floating), 0 -18px 70px color-mix(in srgb, var(--choir-accent) 12%, transparent);
    backdrop-filter: blur(var(--choir-blur));
  }

  .desk-sheet.placement-bottom {
    bottom: calc(var(--choir-prompt-surface-size, 64px) + max(18px, env(safe-area-inset-bottom)));
    border-radius: var(--choir-radius-sheet) var(--choir-radius-sheet) var(--choir-radius-control) var(--choir-radius-control);
  }

  .desk-sheet.placement-top {
    top: calc(var(--choir-prompt-surface-size, 64px) + max(18px, env(safe-area-inset-top)));
    border-radius: var(--choir-radius-control) var(--choir-radius-control) var(--choir-radius-sheet) var(--choir-radius-sheet);
  }

  .sheet-handle {
    justify-self: center;
    width: 3rem;
    height: 0.25rem;
    border-radius: 999px;
    background: color-mix(in srgb, var(--choir-accent) 42%, transparent);
    filter: blur(0.1px);
  }

  header,
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  p,
  h2,
  h3 {
    margin: 0;
  }

  header p,
  h3,
  footer span {
    color: var(--choir-muted);
    font-size: 0.78rem;
    font-weight: 800;
    text-transform: uppercase;
  }

  header h2 {
    font-family: var(--choir-font-display);
    font-size: clamp(1.25rem, 2.6vw, 1.55rem);
    letter-spacing: 0;
  }

  button {
    border: 0;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-control-bg);
    color: var(--choir-fg);
    cursor: pointer;
    box-shadow: var(--choir-control-shadow);
  }

  button:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  header button,
  footer button,
  .plain-row {
    padding: 0.48rem 0.64rem;
  }

  .overview-card {
    display: grid;
    grid-template-columns: 1.55rem minmax(0, 1fr);
    gap: 0.12rem 0.55rem;
    align-self: start;
    align-items: center;
    min-height: 2.9rem;
    padding: 0.5rem 0.58rem;
    text-align: left;
    border-radius: var(--choir-radius-control);
  }

  .overview-card span {
    grid-row: span 2;
    display: inline-grid;
    place-items: center;
    width: 1.55rem;
    height: 1.55rem;
    font-size: 1.18rem;
    line-height: 1;
  }

  .overview-card small,
  .app-grid small {
    color: var(--choir-muted);
  }

  .app-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(7.6rem, 1fr));
    gap: 0.38rem;
  }

  .app-grid button {
    display: grid;
    grid-template-columns: 1.55rem minmax(0, 1fr);
    gap: 0.04rem 0.42rem;
    align-items: center;
    min-height: 2.58rem;
    padding: 0.42rem 0.48rem;
    text-align: left;
  }

  .app-grid button span {
    grid-row: span 2;
    display: inline-grid;
    place-items: center;
    width: 1.55rem;
    height: 1.55rem;
    font-size: 1.1rem;
    line-height: 1;
  }

  .app-grid strong {
    font-size: 0.78rem;
    line-height: 1.05;
  }

  .app-grid small {
    font-size: 0.66rem;
    line-height: 1.05;
  }

  .app-grid strong,
  .app-grid small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .overview-card strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.1;
  }

  .plain-row {
    justify-self: start;
  }

  .account-list {
    display: grid;
    gap: 0.3rem;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .account-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.3rem;
  }

  .account-switch {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    min-width: 0;
    padding: 0.48rem 0.64rem;
    text-align: left;
  }

  .account-row[data-active='true'] .account-switch {
    outline: 2px solid color-mix(in srgb, var(--choir-accent) 60%, transparent);
  }

  .account-email {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .account-switch small {
    color: var(--choir-muted);
    font-weight: 800;
    text-transform: uppercase;
  }

  .account-signout {
    padding: 0.48rem 0.64rem;
  }

  .account-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .account-error {
    color: var(--choir-danger, #c0392b);
    font-size: 0.8rem;
  }

  @media (max-width: 768px) {
    .desk-sheet {
      left: 8px;
      right: 8px;
      max-height: calc(100dvh - var(--choir-prompt-surface-size, 64px) - 20px);
      padding: 0.58rem;
      gap: 0.38rem;
    }

    .app-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 0.32rem;
    }

    .app-grid button {
      grid-template-columns: 1fr;
      justify-items: center;
      min-height: 3.35rem;
      gap: 0.14rem;
      padding: 0.38rem 0.28rem;
      text-align: center;
    }

    .app-grid button span {
      grid-row: auto;
      width: 1.65rem;
      height: 1.65rem;
      font-size: 1.24rem;
    }

    .app-grid small,
    .overview-card small {
      display: none;
    }

    .overview-card {
      grid-template-columns: 1.8rem minmax(0, 1fr);
      gap: 0.7rem;
      align-items: center;
      min-height: 4rem;
      padding: 0.64rem 0.78rem;
    }

    .overview-card span {
      grid-row: auto;
      width: 1.8rem;
      height: 1.8rem;
      font-size: 1.28rem;
    }

    .overview-card strong {
      align-self: center;
      font-size: 1rem;
      line-height: 1;
    }

    footer {
      font-size: 0.72rem;
    }
  }
</style>
