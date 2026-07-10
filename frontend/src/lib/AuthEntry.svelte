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
  import PretextInlineDisclosure from './PretextInlineDisclosure.svelte';

  export let passkeyError = '';
  export let intentMessage = 'Open your private computer and keep working.';

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
  const passkeyTooltipCopy = {
    register: 'Passkeys use Face ID, Touch ID, Windows Hello, a device PIN, or a security key. They are phishing-resistant. If your passkeys sync, you can use the same account on your other devices.',
    login: 'Your passkey uses your device lock or security key to confirm it is you. Choir never receives your fingerprint, face scan, or device PIN.',
  };

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
    <p class="auth-kicker">Continue in Choir</p>
    <h1>Sign in. Pick up where you left off.</h1>
    <p class="tagline">Your preview stays open. A passkey unlocks your saved work without a password.</p>
    <div class="auth-intent" data-auth-intent>
      <span>After sign-in</span>
      <p>{intentMessage}</p>
    </div>

    <div class="view-tabs">
      <button
        class="tab"
        class:active={view === 'register'}
        data-register-toggle
        on:click={() => switchView('register')}
        disabled={ceremonyInProgress}
      >
        Create account
      </button>
      <button
        class="tab"
        class:active={view === 'login'}
        data-login-toggle
        on:click={() => switchView('login')}
        disabled={ceremonyInProgress}
      >
        Sign in
      </button>
    </div>

    {#if view === 'register'}
      <div class="auth-view" data-register-view>
        <PretextInlineDisclosure
          prefix="Create a "
          subject="passkey"
          collapsedDetail="Use your device lock once. There is no password to create or remember."
          disclosure={passkeyTooltipCopy.register}
          ariaLabel="What is a passkey?"
        />

        <form on:submit|preventDefault={handleRegister}>
          <label for="register-email">Email address</label>
          <input
            id="register-email"
            name="email"
            type="email"
            bind:value={email}
            placeholder="you@example.com"
            autocomplete="email"
            inputmode="email"
            autocapitalize="none"
            spellcheck="false"
            disabled={ceremonyInProgress}
            required
          />
          <button type="submit" class="primary-action" disabled={ceremonyInProgress} data-auth-submit>
            {#if ceremonyInProgress}
              Waiting for your device…
            {:else}
              Create Account with Passkey
            {/if}
          </button>
        </form>
        <p class="fine-print">
          Choir stores a public credential, never your fingerprint, face scan, or device PIN.
          <a href="/privacy">Privacy</a>
          <a href="/terms">Terms</a>
        </p>
      </div>
    {:else}
      <div class="auth-view" data-login-view>
        <PretextInlineDisclosure
          prefix="Sign in with your "
          subject="passkey"
          collapsedDetail="Return to your saved documents, mailbox, and computer."
          disclosure={passkeyTooltipCopy.login}
          ariaLabel="What is a passkey?"
        />

        <form on:submit|preventDefault={handleLogin}>
          <label for="login-email">Email address</label>
          <input
            id="login-email"
            name="email"
            type="email"
            bind:value={email}
            placeholder="you@example.com"
            autocomplete="username webauthn"
            inputmode="email"
            autocapitalize="none"
            spellcheck="false"
            disabled={ceremonyInProgress}
            required
          />
          <button type="submit" class="primary-action" disabled={ceremonyInProgress} data-auth-submit>
            {#if ceremonyInProgress}
              Waiting for your device…
            {:else}
              Continue with Passkey
            {/if}
          </button>
        </form>
        <p class="fine-print">
          Your browser will ask for the device, password manager, or security key that holds your passkey.
          <a href="/privacy">Privacy</a>
          <a href="/terms">Terms</a>
        </p>
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
    background:
      linear-gradient(145deg, color-mix(in srgb, var(--choir-accent) 8%, transparent), transparent 42%),
      var(--choir-sheet-bg, var(--choir-state-selected));
    border: 0;
    border-radius: var(--choir-radius-panel, 22px);
    padding: 1.55rem;
    width: 100%;
    max-width: 480px;
    text-align: left;
    box-shadow: var(--choir-shadow-floating);
    color: var(--choir-text-primary);
  }

  .auth-kicker {
    margin: 0 0 0.45rem;
    color: var(--choir-accent);
    font-size: 0.72rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  h1 {
    font-family: var(--choir-font-display, inherit);
    max-width: 18rem;
    font-size: clamp(1.75rem, 3.5vw, 2.35rem);
    font-weight: 780;
    line-height: 0.98;
    letter-spacing: 0;
    color: var(--choir-text-primary);
    margin: 0 0 0.65rem;
  }

  .tagline {
    max-width: 34rem;
    font-size: 0.96rem;
    line-height: 1.45;
    color: var(--choir-text-muted);
    margin: 0 0 1.1rem;
  }

  .auth-intent {
    margin: 0 0 1rem;
    padding: 0.75rem 0.9rem;
    border-radius: var(--choir-radius-control, 16px);
    background: color-mix(in srgb, var(--choir-surface-card) 82%, transparent);
    color: var(--choir-text-primary);
    font-size: 0.92rem;
    line-height: 1.35;
    overflow-wrap: anywhere;
    box-shadow: inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 6%, transparent);
  }

  .auth-intent span {
    display: block;
    margin-bottom: 0.22rem;
    color: var(--choir-accent);
    font-size: 0.68rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .auth-intent p {
    margin: 0;
  }

  .view-tabs {
    display: flex;
    gap: 0.45rem;
    margin-bottom: 1.2rem;
    padding: 0.28rem;
    border-radius: var(--choir-radius-control, 16px);
    background: color-mix(in srgb, var(--choir-surface-card) 78%, transparent);
    box-shadow: inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 7%, transparent);
  }

  .tab {
    flex: 1;
    min-height: 2.45rem;
    padding: 0.65rem 0.85rem;
    font-size: 0.88rem;
    font-weight: 760;
    background: transparent;
    color: var(--choir-text-muted);
    border: none;
    border-radius: var(--choir-radius-control-sm, 12px);
    cursor: pointer;
    transition: background 0.2s, color 0.2s, box-shadow 0.2s;
  }

  .tab:hover {
    background: color-mix(in srgb, var(--choir-surface-control) 72%, transparent);
    color: var(--choir-text-primary);
  }

  .tab.active {
    background: var(--choir-state-selected);
    color: var(--choir-text-primary);
    box-shadow: var(--choir-control-shadow);
  }

  .auth-view {
    text-align: left;
  }

  form {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    text-align: left;
  }

  label {
    font-size: 0.8rem;
    font-weight: 720;
    color: var(--choir-text-muted);
  }

  input[type="email"] {
    min-height: 3rem;
    padding: 0.78rem 0.95rem;
    font-size: 1rem;
    background: var(--choir-surface-input);
    border: 0;
    border-radius: var(--choir-radius-control, 16px);
    color: var(--choir-text-primary);
    outline: none;
    box-shadow:
      inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 8%, transparent),
      0 10px 28px color-mix(in srgb, var(--choir-shadow-color) 12%, transparent);
    transition: box-shadow 0.2s;
  }

  input[type="email"]:focus {
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--choir-accent) 28%, transparent),
      0 14px 30px color-mix(in srgb, var(--choir-shadow-color) 16%, transparent);
  }

  input[type="email"]::placeholder {
    color: var(--choir-text-subtle);
  }

  .primary-action {
    min-height: 3.1rem;
    margin-top: 0.35rem;
    padding: 0.85rem 1rem;
    font-size: 1rem;
    font-weight: 820;
    background: var(--choir-accent);
    color: var(--choir-text-on-accent, var(--choir-text-primary));
    border: none;
    border-radius: var(--choir-radius-control, 16px);
    cursor: pointer;
    box-shadow: var(--choir-control-shadow);
    transition: filter 0.2s, transform 0.2s;
  }

  .primary-action:hover {
    filter: brightness(1.08);
    transform: translateY(-1px);
  }

  .primary-action:disabled {
    background: var(--choir-surface-control);
    color: var(--choir-text-subtle);
    cursor: not-allowed;
  }

  .error {
    margin-top: 1rem;
    color: var(--choir-status-danger);
    font-size: 0.9rem;
    line-height: 1.35;
  }

  .fine-print {
    margin: 0.85rem 0 0;
    color: var(--choir-text-muted);
    font-size: 0.78rem;
    line-height: 1.42;
  }

  .fine-print a {
    margin-left: 0.45rem;
    color: var(--choir-text-accent);
    font-weight: 700;
    text-decoration: none;
  }

  .fine-print a:hover,
  .fine-print a:focus-visible {
    text-decoration: underline;
  }

  :global(:root[data-theme-id='london-salmon']) .auth-card,
  :global(:root[data-theme-id='london-salmon']) .auth-view,
  :global(:root[data-theme-id='london-salmon']) .auth-intent,
  :global(:root[data-theme-id='london-salmon']) .tab,
  :global(:root[data-theme-id='london-salmon']) label,
  :global(:root[data-theme-id='london-salmon']) input[type="email"],
  :global(:root[data-theme-id='london-salmon']) .primary-action {
    font-family: var(--choir-font-ui, Georgia, serif);
  }

  :global(:root[data-theme-id='london-salmon']) .tab,
  :global(:root[data-theme-id='london-salmon']) .primary-action {
    font-style: italic;
  }

  :global(:root[data-theme-id='london-salmon']) .auth-kicker {
    color: var(--choir-text-muted);
  }

  :global(:root[data-theme-id='london-salmon']) .auth-intent p {
    font-style: italic;
  }
</style>
