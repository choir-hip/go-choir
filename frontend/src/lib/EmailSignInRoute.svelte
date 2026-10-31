<!--
  EmailSignInRoute — landing page for an email sign-in link
  (/signin/email#token=choir_st_...).

  The token arrives in the URL fragment, which browsers never send to a
  server. On load the page reads it and clears it from the address bar and
  history with history.replaceState. The single-use token is redeemed only
  when the person presses "Sign in", so a mail scanner that opens the link
  cannot burn it. A link is a bearer credential: it signs in whoever opens it.

  Data attributes for test targeting:
    data-email-signin-route    — root, with data-email-signin-state={state}
    data-email-signin-start    — "Sign in" button
    data-email-signin-error    — error message
    data-email-signin-done     — success panel
    data-email-signin-open     — "Open Choir" button
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { emailTokenFromHash, verifyEmailToken } from './auth.js';

  type RouteState = 'ready' | 'working' | 'done' | 'invalid' | 'unavailable';

  let state: RouteState = 'invalid';
  let token = '';
  let error = '';

  onMount(() => {
    token = emailTokenFromHash(window.location.hash);
    if (window.location.hash) {
      // Drop the token from the address bar and session history at once.
      window.history.replaceState(window.history.state, '', window.location.pathname);
    }
    state = token ? 'ready' : 'invalid';
  });

  async function signIn() {
    if (!token) {
      state = 'invalid';
      return;
    }
    state = 'working';
    error = '';
    const presented = token;
    token = '';
    try {
      await verifyEmailToken(presented);
      try {
        window.localStorage?.setItem('choir.auth.returning', 'true');
      } catch (_err) {
        // Storage can be unavailable (private mode); sign-in still worked.
      }
      state = 'done';
    } catch (err) {
      if (err?.status === 403) {
        state = 'unavailable';
      } else if (err?.status === 400) {
        state = 'invalid';
      } else {
        // Network or server trouble: the token may still be good.
        token = presented;
        error = 'Choir could not reach sign-in. Check your connection and try again.';
        state = 'ready';
      }
    }
  }

  function openChoir() {
    window.location.replace('/');
  }
</script>

<main class="signin-page" data-email-signin-route data-email-signin-state={state}>
  <div class="signin-card">
    <p class="kicker">Choir sign-in</p>

    {#if state === 'ready' || state === 'working'}
      <h1>Sign in to Choir</h1>
      <p class="body">
        Your sign-in link is ready. Only continue if you asked for it from this device or one you
        trust.
      </p>
      {#if error}
        <p class="error" role="alert" data-email-signin-error>{error}</p>
      {/if}
      <button
        type="button"
        class="primary-action"
        data-email-signin-start
        disabled={state === 'working'}
        on:click={signIn}
      >
        {#if state === 'working'}
          <span class="spinner" aria-hidden="true"></span>
          Signing in…
        {:else}
          Sign in
        {/if}
      </button>
      <p class="muted">The link works once. If you did not ask for it, close this page.</p>
    {:else if state === 'done'}
      <div data-email-signin-done role="status" aria-live="polite">
        <h1>You are signed in</h1>
        <p class="body">You are signed in on this device by email. Passkeys are safer when you have one.</p>
        <button type="button" class="primary-action" data-email-signin-open on:click={openChoir}>
          Open Choir
        </button>
      </div>
    {:else if state === 'unavailable'}
      <h1>Email sign-in is not available</h1>
      <p class="error" role="alert" data-email-signin-error>Use your passkey to sign in.</p>
      <button type="button" class="primary-action" data-email-signin-open on:click={openChoir}>
        Sign in with a passkey
      </button>
    {:else}
      <h1>This link no longer works</h1>
      <p class="error" role="alert" data-email-signin-error>
        Sign-in links work once and expire after 10 minutes. Ask for a new one.
      </p>
      <button type="button" class="primary-action" data-email-signin-open on:click={openChoir}>
        Back to sign in
      </button>
    {/if}
  </div>
</main>

<style>
  .signin-page {
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

  .signin-card {
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
    animation: signin-spin 0.7s linear infinite;
  }

  @keyframes signin-spin {
    to { transform: rotate(360deg); }
  }
</style>
