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
  import { createEventDispatcher, onMount, tick } from 'svelte';
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
  let emailInputEl: HTMLInputElement | null = null;

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
    error = '';
    // Clear passkeyError when switching views so the user gets a clean retry state.
    dispatch('clearpasskeyerror');
    tick().then(() => {
      emailInputEl?.focus();
      emailInputEl?.select();
    });
  }

  onMount(() => {
    if (window.localStorage?.getItem('choir.auth.returning') === 'true') {
      view = 'login';
    }
    tick().then(() => emailInputEl?.focus());
  });

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

<div class="auth-entry" data-auth-entry aria-busy={ceremonyInProgress ? 'true' : 'false'}>
  <div class="auth-card">
    <p class="auth-brand" aria-hidden="true">Choir</p>
    <p class="auth-kicker">Continue</p>
    <h1>Sign in. Pick up where you left off.</h1>
    <p class="tagline">Your preview stays open. A passkey unlocks your saved work without a password.</p>
    <div class="auth-intent" data-auth-intent>
      <span>After sign-in</span>
      <p>{intentMessage}</p>
    </div>

    <div class="view-tabs" role="tablist" aria-label="Choose sign-in path">
      <button
        class="tab"
        class:active={view === 'register'}
        data-register-toggle
        type="button"
        role="tab"
        id="register-tab"
        aria-controls="register-panel"
        aria-selected={view === 'register'}
        on:click={() => switchView('register')}
        disabled={ceremonyInProgress}
      >
        Create account
      </button>
      <button
        class="tab"
        class:active={view === 'login'}
        data-login-toggle
        type="button"
        role="tab"
        id="login-tab"
        aria-controls="login-panel"
        aria-selected={view === 'login'}
        on:click={() => switchView('login')}
        disabled={ceremonyInProgress}
      >
        Sign in
      </button>
    </div>

    {#if view === 'register'}
      <div class="auth-view" id="register-panel" role="tabpanel" aria-labelledby="register-tab" data-register-view>
        <PretextInlineDisclosure
          prefix="Create a "
          subject="passkey"
          collapsedDetail="Use your device lock once. There is no password to create or remember."
          disclosure={passkeyTooltipCopy.register}
          ariaLabel="What is a passkey?"
        />

        <form novalidate on:submit|preventDefault={handleRegister}>
          <label for="register-email">Email address</label>
          <input
            id="register-email"
            name="email"
            type="email"
            bind:this={emailInputEl}
            bind:value={email}
            placeholder="you@example.com"
            autocomplete="email"
            inputmode="email"
            autocapitalize="none"
            spellcheck="false"
            disabled={ceremonyInProgress}
            aria-invalid={error ? 'true' : 'false'}
            aria-errormessage={error ? 'auth-error' : undefined}
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
      <div class="auth-view" id="login-panel" role="tabpanel" aria-labelledby="login-tab" data-login-view>
        <PretextInlineDisclosure
          prefix="Sign in with your "
          subject="passkey"
          collapsedDetail="Return to your saved documents, mailbox, and computer."
          disclosure={passkeyTooltipCopy.login}
          ariaLabel="What is a passkey?"
        />

        <form novalidate on:submit|preventDefault={handleLogin}>
          <label for="login-email">Email address</label>
          <input
            id="login-email"
            name="email"
            type="email"
            bind:this={emailInputEl}
            bind:value={email}
            placeholder="you@example.com"
            autocomplete="username webauthn"
            inputmode="email"
            autocapitalize="none"
            spellcheck="false"
            disabled={ceremonyInProgress}
            aria-invalid={error ? 'true' : 'false'}
            aria-errormessage={error ? 'auth-error' : undefined}
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
      <p class="error" id="auth-error" role="alert" data-passkey-error>{displayError}</p>
    {/if}

    <p class="next-step"><strong>Next:</strong> Your browser asks you to unlock the passkey. Then Choir returns you to the action above.</p>
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
    position: relative;
    background:
      linear-gradient(155deg, color-mix(in srgb, var(--choir-accent) 14%, transparent), transparent 38%),
      linear-gradient(320deg, color-mix(in srgb, var(--choir-accent-2) 8%, transparent), transparent 46%),
      var(--choir-sheet-bg, var(--choir-state-selected));
    border: 1px solid color-mix(in srgb, var(--choir-border-strong) 55%, transparent);
    border-radius: var(--choir-radius-panel, 22px);
    padding: 1.7rem 1.55rem 1.45rem;
    width: 100%;
    max-width: 480px;
    max-height: calc(100dvh - 2rem);
    overflow: auto;
    overscroll-behavior: contain;
    text-align: left;
    box-shadow:
      var(--choir-shadow-floating),
      0 0 0 1px color-mix(in srgb, var(--choir-text-primary) 4%, transparent) inset,
      0 0 60px color-mix(in srgb, var(--choir-accent) 12%, transparent);
    color: var(--choir-text-primary);
    animation: auth-card-enter var(--choir-motion-sheet, 260ms cubic-bezier(0.2, 0.8, 0.2, 1)) both;
  }

  .auth-brand {
    margin: 0 0 0.55rem;
    font-family: var(--choir-font-display, inherit);
    font-size: clamp(2.35rem, 6vw, 3.15rem);
    font-weight: 780;
    letter-spacing: -0.04em;
    line-height: 0.92;
    color: var(--choir-text-primary);
    text-shadow: 0 0 42px color-mix(in srgb, var(--choir-accent) 28%, transparent);
  }

  .auth-kicker {
    margin: 0 0 0.4rem;
    color: var(--choir-accent);
    font-size: 0.72rem;
    font-weight: 820;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }

  h1 {
    font-family: var(--choir-font-display, inherit);
    max-width: 20rem;
    font-size: clamp(1.35rem, 2.8vw, 1.7rem);
    font-weight: 720;
    line-height: 1.12;
    letter-spacing: -0.01em;
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
    box-shadow:
      inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 6%, transparent),
      0 0 0 1px color-mix(in srgb, var(--choir-border) 70%, transparent);
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
    transition: background var(--choir-motion-fast, 120ms ease), color var(--choir-motion-fast, 120ms ease), box-shadow var(--choir-motion-fast, 120ms ease), transform var(--choir-motion-fast, 120ms ease);
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
    transition: box-shadow var(--choir-motion-fast, 120ms ease);
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
    box-shadow: var(--choir-control-shadow), 0 0 28px color-mix(in srgb, var(--choir-accent) 22%, transparent);
    transition: filter var(--choir-motion-fast, 120ms ease), transform var(--choir-motion-fast, 120ms ease), box-shadow var(--choir-motion-fast, 120ms ease);
  }

  .primary-action:hover:enabled {
    filter: brightness(1.08);
    transform: translateY(-1px);
    box-shadow: var(--choir-control-shadow), 0 0 36px color-mix(in srgb, var(--choir-accent) 32%, transparent);
  }

  .primary-action:disabled {
    background: var(--choir-surface-control);
    color: var(--choir-text-subtle);
    cursor: not-allowed;
    box-shadow: none;
  }

  .error {
    margin-top: 1rem;
    padding: 0.7rem 0.8rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
    font-size: 0.9rem;
    line-height: 1.35;
  }

  .next-step {
    margin: 0.9rem 0 0;
    padding-top: 0.8rem;
    border-top: 1px solid var(--choir-border);
    color: var(--choir-text-muted);
    font-size: 0.82rem;
    line-height: 1.45;
  }

  .next-step strong {
    color: var(--choir-text-primary);
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

  @keyframes auth-card-enter {
    from {
      opacity: 0;
      transform: translateY(10px) scale(0.985);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .auth-card {
      animation: none;
    }

    .tab,
    .primary-action,
    input[type="email"] {
      transition: none;
    }
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

  :global(:root[data-theme-id='london-salmon']) .auth-brand {
    font-family: var(--choir-font-display, Georgia, serif);
    text-shadow: none;
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
