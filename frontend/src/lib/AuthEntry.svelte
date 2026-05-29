<!--
  AuthEntry — guest auth entry experience for signed-out users.

  Exposes distinct register and login views, each with a clear primary
  action to begin the passkey flow. Does not call any protected
  (/api/shell/bootstrap, /api/ws) routes while signed out.

  Displays passkey ceremony errors (cancel/failure) from the parent
  component via the `passkeyError` prop, keeping the user in a
  retryable guest auth state.

  Data attributes for test targeting:
    data-auth-entry       — root container
    data-register-toggle  — control to switch to register view
    data-login-toggle     — control to switch to login view
    data-register-view    — register view container
    data-login-view       — login view container
    data-passkey-error    — passkey ceremony error message area
-->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let passkeyError = '';

  /** Whether a passkey ceremony is in progress (disables the form). */
  export let ceremonyInProgress = false;

  const dispatch = createEventDispatcher();

  /** @type {'register' | 'login'} */
  let view = 'register';

  /** Email input for the current view. */
  let email = '';

  /** Validation error message (empty email etc). */
  let error = '';

  /** Combined error to display: validation error takes precedence, then passkeyError. */
  $: displayError = error || passkeyError;

  /** Simple email format validation. */
  function isValidEmail(value) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  }

  function switchView(newView) {
    view = newView;
    email = '';
    error = '';
    // Clear passkeyError when switching views so the user gets a clean retry state.
    dispatch('clearpasskeyerror');
  }

  function handleRegister() {
    error = '';
    if (!email.trim()) {
      error = 'Please enter an email address.';
      return;
    }
    if (!isValidEmail(email.trim())) {
      error = 'Please enter a valid email address.';
      return;
    }
    dispatch('authbegin', { email: email.trim(), type: 'register' });
  }

  function handleLogin() {
    error = '';
    if (!email.trim()) {
      error = 'Please enter an email address.';
      return;
    }
    if (!isValidEmail(email.trim())) {
      error = 'Please enter a valid email address.';
      return;
    }
    dispatch('authbegin', { email: email.trim(), type: 'login' });
  }
</script>

<div class="auth-entry" data-auth-entry>
  <div class="auth-card">
    <h1>Choir</h1>
    <p class="tagline">Keep previewing. Sign in only when an action needs your computer.</p>

    <div class="view-tabs">
      <button
        class="tab"
        class:active={view === 'register'}
        data-register-toggle
        on:click={() => switchView('register')}
        disabled={ceremonyInProgress}
      >
        Register
      </button>
      <button
        class="tab"
        class:active={view === 'login'}
        data-login-toggle
        on:click={() => switchView('login')}
        disabled={ceremonyInProgress}
      >
        Sign In
      </button>
    </div>

    {#if view === 'register'}
      <div class="auth-view" data-register-view>
        <h2>Create a passkey</h2>
        <p class="view-desc">Use this for saving, publishing, sending, importing, and private state.</p>

        <form on:submit|preventDefault={handleRegister}>
          <label for="register-email">Email</label>
          <input
            id="register-email"
            type="email"
            bind:value={email}
            placeholder="you@example.com"
            autocomplete="email"
            disabled={ceremonyInProgress}
            required
          />
          <button type="submit" class="primary-action" disabled={ceremonyInProgress} data-auth-submit>
            {#if ceremonyInProgress}
              Creating passkey…
            {:else}
              Create passkey
            {/if}
          </button>
        </form>
      </div>
    {:else}
      <div class="auth-view" data-login-view>
        <h2>Use your passkey</h2>
        <p class="view-desc">Return to your durable computer, mailbox, traces, and saved documents.</p>

        <form on:submit|preventDefault={handleLogin}>
          <label for="login-email">Email</label>
          <input
            id="login-email"
            type="email"
            bind:value={email}
            placeholder="you@example.com"
            autocomplete="email"
            disabled={ceremonyInProgress}
            required
          />
          <button type="submit" class="primary-action" disabled={ceremonyInProgress} data-auth-submit>
            {#if ceremonyInProgress}
              Signing in…
            {:else}
              Sign in
            {/if}
          </button>
        </form>
      </div>
    {/if}

    {#if displayError}
      <p class="error" role="alert" data-passkey-error>{displayError}</p>
    {/if}
  </div>
</div>

<style>
  .auth-entry {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100dvh;
    width: 100%;
  }

  .auth-card {
    background: var(--choir-sheet-bg, #101827);
    border: 1px solid var(--choir-border-strong, rgba(130, 156, 255, 0.44));
    border-radius: var(--choir-radius-panel, 22px);
    padding: 2rem;
    width: 100%;
    max-width: 430px;
    text-align: center;
    box-shadow: var(--choir-shadow-floating, 0 26px 90px rgba(0,0,0,.46));
    color: var(--choir-fg, #f7faff);
  }

  h1 {
    font-family: var(--choir-font-display, inherit);
    font-size: 2.15rem;
    font-weight: 760;
    letter-spacing: 0;
    color: var(--choir-fg, #ffffff);
    margin-bottom: 0.25rem;
  }

  .tagline {
    font-size: 0.9rem;
    color: var(--choir-muted, #9aa9c0);
    margin-bottom: 1.5rem;
  }

  .view-tabs {
    display: flex;
    gap: 0;
    margin-bottom: 1.5rem;
    border-radius: var(--choir-radius-control-sm, 10px);
    overflow: hidden;
    border: 1px solid var(--choir-border, rgba(148, 163, 184, 0.18));
  }

  .tab {
    flex: 1;
    padding: 0.6rem 1rem;
    font-size: 0.9rem;
    font-weight: 500;
    background: transparent;
    color: var(--choir-muted, #999);
    border: none;
    cursor: pointer;
    transition: background 0.2s, color 0.2s;
  }

  .tab:hover {
    background: var(--choir-panel-soft, #222);
    color: var(--choir-fg, #ccc);
  }

  .tab.active {
    background: var(--choir-selected, #2a2a2a);
    color: var(--choir-fg, #ffffff);
  }

  .auth-view {
    text-align: center;
  }

  .auth-view h2 {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--choir-fg, #e0e0e0);
    margin-bottom: 0.5rem;
  }

  .view-desc {
    font-size: 0.85rem;
    color: var(--choir-muted, #888);
    margin-bottom: 1.25rem;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    text-align: left;
  }

  label {
    font-size: 0.8rem;
    font-weight: 500;
    color: var(--choir-muted, #aaa);
  }

  input[type="email"] {
    padding: 0.7rem 0.85rem;
    font-size: 0.95rem;
    background: var(--choir-input-bg, #111);
    border: 1px solid var(--choir-border, #333);
    border-radius: var(--choir-radius-control-sm, 10px);
    color: var(--choir-fg, #e0e0e0);
    outline: none;
    transition: border-color 0.2s;
  }

  input[type="email"]:focus {
    border-color: var(--choir-accent, #6d8dff);
  }

  input[type="email"]::placeholder {
    color: var(--choir-subtle, #65748d);
  }

  .primary-action {
    margin-top: 0.5rem;
    padding: 0.8rem 1rem;
    font-size: 1rem;
    font-weight: 600;
    background: var(--choir-accent, #3b82f6);
    color: var(--choir-on-accent, #ffffff);
    border: none;
    border-radius: var(--choir-radius-control-sm, 10px);
    cursor: pointer;
    transition: background 0.2s;
  }

  .primary-action:hover {
    filter: brightness(1.08);
  }

  .primary-action:disabled {
    background: var(--choir-control-bg, #1e3a5f);
    color: var(--choir-subtle, #667);
    cursor: not-allowed;
  }

  .error {
    margin-top: 1rem;
    color: var(--choir-danger, #f87171);
    font-size: 0.85rem;
  }
</style>
