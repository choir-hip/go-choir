<!--
  DeleteAccountRoute — delete your account (/account/delete).
  docs/design/account-create-delete-2026-11-23.md, with the owner's rulings of 2026-11-23: takes are
  removed; published stories are kept with the author detached; access is revoked at once; the
  irreversible purge waits 72 hours and signing in with a passkey in that time offers to cancel it;
  a deletion receipt and 30 days of abuse logs are kept. The app's page (Settings, Account, Delete
  account) says the same things in the same words.

  A signed-in session alone starts nothing: the button asks for the passkey, a fresh assertion
  minutes old at most (POST /auth/account/delete/begin and /finish).

  Data attributes for test targeting:
    data-delete-route            — root, with data-delete-state={state}
    data-delete-signin-email     — email input when signed out
    data-delete-signin-btn       — "Sign in with passkey"
    data-delete-ask              — "Delete account"
    data-delete-confirm          — "Delete my account" (asks for the passkey)
    data-delete-keep             — "Keep my account"
    data-delete-error            — error message
    data-delete-done             — the scheduled-for-deletion message
    data-delete-cancel           — "Cancel deletion" (a passkey sign-in of a pending account)
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import {
    getSession, loginPasskey, deleteAccountWithPasskey, cancelAccountDeletion, passkeyErrorMessage,
    DeletionPendingError, DeletionPurgingError, plainDeletionDate, AuthRequiredError,
  } from './auth.js';

  type RouteState = 'checking' | 'signin' | 'info' | 'sure' | 'working' | 'scheduled' | 'offer' | 'cancelled';

  let state: RouteState = 'checking';
  let email = '';
  let error = '';
  let purgeOn = '';
  let offer: DeletionPendingError | null = null;

  onMount(async () => {
    try {
      const session = await getSession();
      state = session?.authenticated ? 'info' : 'signin';
    } catch (_err) {
      state = 'signin';
    }
  });

  async function signIn() {
    error = '';
    if (!email.trim()) {
      error = 'Enter the email of your Choir account.';
      return;
    }
    state = 'working';
    try {
      await loginPasskey(email.trim());
      state = 'info';
    } catch (err) {
      if (err instanceof DeletionPendingError) {
        offer = err;
        state = 'offer';
        return;
      }
      if (err instanceof DeletionPurgingError) {
        error = 'This account is being removed now. The removal has begun and cannot be cancelled.';
        state = 'signin';
        return;
      }
      error = passkeyErrorMessage(err, 'login');
      state = 'signin';
    }
  }

  async function confirmDelete() {
    error = '';
    state = 'working';
    try {
      const done = await deleteAccountWithPasskey();
      purgeOn = plainDeletionDate(done?.purge_after);
      state = 'scheduled';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        state = 'signin';
        return;
      }
      if (err?.name === 'NotAllowedError') {
        error = 'The passkey prompt was cancelled. Nothing was deleted.';
      } else if (err?.status === 403) {
        error = 'This account cannot be deleted from here. Nothing was deleted.';
      } else if (err?.status === 503 || err?.name === 'RateLimitError') {
        error = 'Choir could not do this just now. Nothing was deleted. Try again in a minute.';
      } else if (/failed to fetch|network/i.test(String(err?.message || ''))) {
        error = 'No connection. Nothing was deleted. Check the network and try again.';
      } else {
        error = 'The confirmation did not go through. Nothing was deleted. Try again.';
      }
      state = 'sure';
    }
  }

  async function cancelDeletion() {
    error = '';
    state = 'working';
    try {
      await cancelAccountDeletion(offer?.cancelToken || '');
      state = 'cancelled';
    } catch (_err) {
      error = 'Choir could not cancel the deletion just now. Sign in again and try once more.';
      state = 'offer';
    }
  }
</script>

<main class="delete-page" data-delete-route data-delete-state={state}>
  <div class="delete-card">
    <p class="kicker">Choir account</p>

    {#if state === 'checking'}
      <h1>One moment…</h1>
    {:else if state === 'signin' || (state === 'working' && !offer && !purgeOn && email)}
      <h1>Delete your account</h1>
      <p class="body">Sign in with your passkey first, so Choir knows which account this is.</p>
      <input type="email" class="text-input" data-delete-signin-email placeholder="you@example.com" autocomplete="username webauthn" bind:value={email} />
      {#if error}<p class="error" role="alert" data-delete-error>{error}</p>{/if}
      <button type="button" class="primary-action" data-delete-signin-btn disabled={state === 'working'} on:click={signIn}>Sign in with passkey</button>
    {:else if state === 'info' || state === 'sure' || state === 'working'}
      <h1>Delete your account</h1>
      <h2>What is removed</h2>
      <p class="body" data-delete-removed>
        Your account and your passkey. Your computer, with its disk, its stored history and the key that seals it: your edition,
        notes, documents, files, mail and its address, and your listening history. Your synced documents. Any takes you published,
        their words and their audio.
      </p>
      <h2>What stays</h2>
      <p class="body" data-delete-stays>
        <!-- LEAD: say "30 days" here and in the app's Delete page (settings.contract) only when the log retention is enforced (data-inventory section 7). -->
        Stories you published stay public, with your name taken off. A short receipt that the deletion happened, with no
        personal details, and server logs, which are kept for a limited time. Mail you sent or received stays with the people
        on the other end and with our mail provider. Our encrypted backups still hold a copy of the account for about ten days, longer if a clean-up fails, and it is deleted with them; if a backup is ever restored, the deletion is carried out again. Outside AI providers keep what they already
        hold under their own terms; we cannot promise they delete it.
      </p>
      <h2>When</h2>
      <p class="body" data-delete-when>
        Your access ends the moment you confirm, on every device. The removal waits at least 72 hours. Until it begins you can
        change your mind: sign in with your passkey and choose Cancel deletion; nothing will have been removed. Once the
        removal has begun it cannot be undone.
      </p>
      {#if error}<p class="error" role="alert" data-delete-error>{error}</p>{/if}
      {#if state === 'info'}
        <button type="button" class="danger-action" data-delete-ask on:click={() => (state = 'sure')}>Delete account</button>
      {:else if state === 'sure'}
        <p class="body strong" data-delete-sure>Delete this account? You will be asked for your passkey.</p>
        <button type="button" class="primary-action" data-delete-confirm on:click={confirmDelete}>Delete my account</button>
        <button type="button" class="secondary-action" data-delete-keep on:click={() => { error = ''; state = 'info'; }}>Keep my account</button>
      {:else}
        <p class="body" role="status">Waiting for your passkey and the server…</p>
      {/if}
    {:else if state === 'scheduled'}
      <h1>Scheduled for deletion</h1>
      <p class="body" data-delete-done>
        Your account is scheduled for deletion. Your access ended now. The removal begins after {purgeOn || '72 hours'}; until it
        begins, nothing is removed, and you can change your mind: sign in with your passkey and choose Cancel deletion.
      </p>
    {:else if state === 'offer'}
      <h1>This account is scheduled for deletion</h1>
      <p class="body" data-delete-offer>
        The removal begins after {plainDeletionDate(offer?.purgeAfter) || '72 hours'}. It has not begun, and nothing has been removed
        yet. Cancel the deletion to keep your account.
      </p>
      {#if error}<p class="error" role="alert" data-delete-error>{error}</p>{/if}
      <button type="button" class="primary-action" data-delete-cancel on:click={cancelDeletion}>Cancel deletion</button>
      <a class="secondary-action" href="/">Leave it scheduled</a>
    {:else}
      <h1>Deletion cancelled</h1>
      <p class="body" data-delete-cancelled>Your account is as it was, and you are signed in.</p>
      <a class="primary-action" href="/">Open Choir</a>
    {/if}
  </div>
</main>

<style>
  .delete-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100dvh;
    height: 100%;
    overflow: auto;
    padding: 1rem;
    background: var(--choir-bg);
    color: var(--choir-text-primary);
  }
  .delete-card {
    width: 100%;
    max-width: 520px;
    padding: 1.4rem;
    border: 1px solid color-mix(in srgb, var(--choir-border-strong) 55%, transparent);
    border-radius: var(--choir-radius-panel, 22px);
    background: var(--choir-sheet-bg, var(--choir-state-selected));
    box-shadow: var(--choir-shadow-floating);
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
  }
  .kicker {
    margin: 0;
    color: var(--choir-accent);
    font-size: 0.72rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  h1 {
    margin: 0 0 0.3rem;
    font-family: var(--choir-font-display, inherit);
    font-size: clamp(1.5rem, 6vw, 2rem);
    font-weight: 780;
    line-height: 1.05;
  }
  h2 {
    margin: 0.4rem 0 0;
    font-size: 0.78rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--choir-text-muted);
  }
  .body {
    margin: 0;
    font-size: 0.95rem;
    line-height: 1.5;
    overflow-wrap: anywhere;
  }
  .strong {
    font-weight: 700;
  }
  .text-input {
    width: 100%;
    min-height: 2.8rem;
    padding: 0.6rem 0.8rem;
    font-size: 1rem;
    border-radius: var(--choir-radius-control, 16px);
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    box-sizing: border-box;
  }
  .primary-action,
  .secondary-action,
  .danger-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    min-height: 3rem;
    padding: 0.8rem 1rem;
    font-size: 1rem;
    font-weight: 820;
    border-radius: var(--choir-radius-control, 16px);
    cursor: pointer;
    text-decoration: none;
    box-sizing: border-box;
  }
  .primary-action {
    background: var(--choir-accent);
    color: var(--choir-text-on-accent, var(--choir-text-primary));
    border: none;
    box-shadow: var(--choir-control-shadow);
  }
  .secondary-action {
    background: transparent;
    color: var(--choir-text-primary);
    border: 1px solid var(--choir-border-strong);
  }
  .danger-action {
    background: transparent;
    color: var(--choir-status-danger);
    border: 1px solid var(--choir-status-danger);
  }
  .primary-action:disabled,
  .secondary-action:disabled,
  .danger-action:disabled {
    background: var(--choir-surface-control);
    color: var(--choir-text-subtle);
    cursor: not-allowed;
  }
  .error {
    margin: 0;
    padding: 0.7rem 0.8rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
    font-size: 0.9rem;
    line-height: 1.35;
  }
</style>
