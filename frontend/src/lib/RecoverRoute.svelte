<!--
  RecoverRoute — landing page for an account-recovery link
  (/recover#token=choir_rt_...).

  The token arrives in the URL fragment, which browsers never send to a
  server. On load the page reads it, removes it from the address bar and
  history, and waits: the single-use token is redeemed only when the
  person presses "Add a new passkey", so a mail scanner that opens the link
  cannot burn it. If the passkey prompt is cancelled, the registration
  options from the redeemed token stay usable for a few minutes, so "Try
  again" does not need a new email.

  Data attributes for test targeting:
    data-recover-route   — root, with data-recover-state={state}
    data-recover-start   — "Add a new passkey" button
    data-recover-retry   — retry after a cancelled or failed prompt
    data-recover-error   — error message
    data-recover-done    — success panel
    data-recover-open    — "Open Choir" button
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import RecoveryRequestForm from './RecoveryRequestForm.svelte';
  import {
    beginRecoveryPasskey,
    createPasskeyFromOptions,
    passkeyErrorMessage,
    recoveryTokenFromHash,
  } from './auth.js';

  type RecoverState = 'ready' | 'working' | 'retry' | 'done' | 'invalid' | 'request';

  let state: RecoverState = 'request';
  let token = '';
  let creationOptions: object | null = null;
  let error = '';

  onMount(() => {
    token = recoveryTokenFromHash(window.location.hash);
    if (window.location.hash) {
      // Drop the token from the address bar and session history at once.
      window.history.replaceState(window.history.state, '', window.location.pathname);
    }
    state = token ? 'ready' : 'request';
  });

  async function runPasskeyCeremony() {
    if (!creationOptions) return;
    state = 'working';
    error = '';
    try {
      await createPasskeyFromOptions(creationOptions);
      creationOptions = null;
      token = '';
      try {
        window.localStorage?.setItem('choir.auth.returning', 'true');
      } catch (_err) {
        // Storage can be unavailable (private mode); sign-in still worked.
      }
      state = 'done';
    } catch (err) {
      // A DOMException (cancelled, timed out, refused by the device) happens
      // before /auth/register/finish, so the challenge is still unused and
      // the same options can be retried. Any other error came from finish,
      // which consumes the challenge: the person needs a new link.
      const clientSide = typeof DOMException !== 'undefined' && err instanceof DOMException;
      if (err?.name === 'InvalidStateError') {
        error = 'This device already has a passkey for your Choir account. Open Choir and sign in with it.';
      } else {
        error = passkeyErrorMessage(err, 'register');
      }
      if (!clientSide) {
        creationOptions = null;
        state = 'invalid';
        return;
      }
      state = 'retry';
    }
  }

  async function startRecovery() {
    if (!token) {
      state = 'request';
      return;
    }
    state = 'working';
    error = '';
    try {
      creationOptions = await beginRecoveryPasskey(token);
      token = '';
    } catch (err) {
      if (err?.status === 400) {
        // Used, expired, superseded or unknown: the server refused it.
        token = '';
        state = 'invalid';
      } else {
        // Network or server trouble: the token may still be good.
        error = 'Choir could not reach sign-in. Check your connection and try again.';
        state = 'ready';
      }
      return;
    }
    await runPasskeyCeremony();
  }

  function openChoir() {
    window.location.replace('/');
  }
</script>

<main class="recover-page" data-recover-route data-recover-state={state}>
  <div class="recover-card">
    <p class="kicker">Choir account recovery</p>

    {#if state === 'ready' || state === 'working'}
      <h1>Add a new passkey</h1>
      <p class="body">
        Your recovery link is ready. Choir will ask this device to create a new passkey for your
        account. Your other passkeys keep working.
      </p>
      {#if error}
        <p class="error" role="alert" data-recover-error>{error}</p>
      {/if}
      <button
        type="button"
        class="primary-action"
        data-recover-start
        disabled={state === 'working'}
        on:click={startRecovery}
      >
        {#if state === 'working'}
          <span class="spinner" aria-hidden="true"></span>
          Waiting for your device…
        {:else}
          Add a new passkey
        {/if}
      </button>
      <p class="muted">The link works once. If you did not ask for it, close this page.</p>
    {:else if state === 'retry'}
      <h1>Add a new passkey</h1>
      {#if error}
        <p class="error" role="alert" data-recover-error>{error}</p>
      {/if}
      <button type="button" class="primary-action" data-recover-retry on:click={runPasskeyCeremony}>
        Try again
      </button>
      <p class="muted">You have a few minutes to try again. After that, ask for a new link.</p>
      <button type="button" class="link-action" on:click={() => (state = 'request')}>
        Email me a new link
      </button>
    {:else if state === 'done'}
      <div data-recover-done role="status" aria-live="polite">
        <h1>You are back in</h1>
        <p class="body">Your new passkey is saved and you are signed in on this device.</p>
        <button type="button" class="primary-action" data-recover-open on:click={openChoir}>
          Open Choir
        </button>
      </div>
    {:else if state === 'invalid'}
      <h1>This link no longer works</h1>
      <p class="error" role="alert" data-recover-error>
        {error || 'Recovery links work once and expire after 15 minutes.'} Ask for a new link below.
      </p>
      <RecoveryRequestForm lead="Enter your account email and Choir will send a fresh link." on:back={openChoir} />
    {:else}
      <h1>Recover your account</h1>
      <RecoveryRequestForm on:back={openChoir} />
    {/if}
  </div>
</main>

<style>
  .recover-page {
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

  .recover-card {
    width: 100%;
    max-width: 440px;
    padding: 1.4rem;
    border: 1px solid color-mix(in srgb, var(--choir-border-strong) 55%, transparent);
    border-radius: var(--choir-radius-panel, 22px);
    background: var(--choir-sheet-bg, var(--choir-state-selected));
    box-shadow: var(--choir-shadow-floating);
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
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
    margin: 0 0 0.4rem;
    font-family: var(--choir-font-display, inherit);
    font-size: clamp(1.5rem, 6vw, 2rem);
    font-weight: 780;
    line-height: 1.05;
  }

  .body,
  .muted {
    margin: 0 0 0.6rem;
    font-size: 0.95rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .muted {
    color: var(--choir-text-muted);
    font-size: 0.85rem;
  }

  .primary-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.55rem;
    width: 100%;
    min-height: 3.1rem;
    margin: 0.2rem 0 0.6rem;
    padding: 0.85rem 1rem;
    font-size: 1rem;
    font-weight: 820;
    background: var(--choir-accent);
    color: var(--choir-text-on-accent, var(--choir-text-primary));
    border: none;
    border-radius: var(--choir-radius-control, 16px);
    cursor: pointer;
    box-shadow: var(--choir-control-shadow);
  }

  .primary-action:disabled {
    background: var(--choir-surface-control);
    color: var(--choir-text-subtle);
    cursor: not-allowed;
  }

  .link-action {
    align-self: flex-start;
    min-height: 2.5rem;
    padding: 0.4rem 0;
    background: none;
    border: none;
    color: var(--choir-text-accent, var(--choir-accent));
    font-size: 0.9rem;
    font-weight: 700;
    cursor: pointer;
  }

  .error {
    margin: 0 0 0.6rem;
    padding: 0.7rem 0.8rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
    font-size: 0.9rem;
    line-height: 1.35;
  }

  .spinner {
    width: 1.05rem;
    height: 1.05rem;
    border-radius: 50%;
    border: 2px solid color-mix(in srgb, currentColor 35%, transparent);
    border-top-color: currentColor;
    animation: recover-spin 0.7s linear infinite;
  }

  @keyframes recover-spin {
    to { transform: rotate(360deg); }
  }
</style>
