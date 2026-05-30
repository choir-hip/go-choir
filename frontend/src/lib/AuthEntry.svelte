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
  export let intentMessage = 'Choose when to make this preview durable.';

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
  let passkeyInfoOpen = false;

  /** Simple email format validation. */
  function isValidEmail(value) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  }

  function switchView(newView) {
    view = newView;
    email = '';
    error = '';
    passkeyInfoOpen = false;
    // Clear passkeyError when switching views so the user gets a clean retry state.
    dispatch('clearpasskeyerror');
  }

  function togglePasskeyInfo() {
    passkeyInfoOpen = !passkeyInfoOpen;
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
    <p class="auth-kicker">Choir private computer</p>
    <h1>Keep the preview. Protect the changes.</h1>
    <p class="tagline">Everything stays visible while logged out. A passkey is only needed when work becomes durable, private, shared, or spend-bearing.</p>
    <div class="auth-intent" data-auth-intent>
      <span>Private action</span>
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
        Create passkey
      </button>
      <button
        class="tab"
        class:active={view === 'login'}
        data-login-toggle
        on:click={() => switchView('login')}
        disabled={ceremonyInProgress}
      >
        Use passkey
      </button>
    </div>

    {#if view === 'register'}
      <div class="auth-view" data-register-view>
        <h2>Create a <span class="passkey-label">passkey<button
          type="button"
          class="passkey-info"
          aria-label="What is a passkey?"
          aria-expanded={passkeyInfoOpen}
          on:click={togglePasskeyInfo}
        >ⓘ</button></span></h2>
        <p class="view-desc">Use your device lock once. Next time, the same passkey can sign you in without a password.</p>
        <div class="passkey-tooltip" class:visible={passkeyInfoOpen} role="tooltip">
          Passkeys use Face ID, Touch ID, Windows Hello, a device PIN, or a security key. They are phishing-resistant and can work across your devices through your password manager or platform account.
        </div>

        <form on:submit|preventDefault={handleRegister}>
          <label for="register-email">Email for this computer</label>
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
              Creating passkey…
            {:else}
              Create Passkey
            {/if}
          </button>
        </form>
        <p class="fine-print">No password is created. Choir stores only the public credential needed to recognize your passkey.</p>
      </div>
    {:else}
      <div class="auth-view" data-login-view>
        <h2>Use your <span class="passkey-label">passkey<button
          type="button"
          class="passkey-info"
          aria-label="What is a passkey?"
          aria-expanded={passkeyInfoOpen}
          on:click={togglePasskeyInfo}
        >ⓘ</button></span></h2>
        <p class="view-desc">Return to your saved documents, mailbox, traces, and private computer state.</p>
        <div class="passkey-tooltip" class:visible={passkeyInfoOpen} role="tooltip">
          A passkey proves it is you with your device lock or security key. It is safer than a password and can be used from another device when your platform offers that option.
        </div>

        <form on:submit|preventDefault={handleLogin}>
          <label for="login-email">Email</label>
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
              Signing in…
            {:else}
              Use Passkey
            {/if}
          </button>
        </form>
        <p class="fine-print">The browser may offer this device, another device, or a security key depending on where your passkey lives.</p>
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

  .auth-view h2 {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-wrap: wrap;
    font-family: var(--choir-font-display, inherit);
    font-size: 1.28rem;
    font-weight: 760;
    line-height: 1.1;
    color: var(--choir-text-primary);
    margin: 0 0 0.45rem;
  }

  .passkey-label {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    white-space: nowrap;
  }

  .passkey-info {
    display: inline-grid;
    place-items: center;
    width: 1.35rem;
    height: 1.35rem;
    margin-left: 0.05rem;
    border: 0;
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-accent);
    cursor: pointer;
    font: inherit;
    font-size: 0.86rem;
    line-height: 1;
    box-shadow: var(--choir-control-shadow);
  }

  .view-desc {
    max-width: 32rem;
    font-size: 0.92rem;
    line-height: 1.4;
    color: var(--choir-text-muted);
    margin: 0 0 0.8rem;
  }

  .passkey-tooltip {
    display: none;
    margin: 0 0 0.95rem;
    padding: 0.85rem 0.95rem;
    border-radius: var(--choir-radius-control, 16px);
    background: var(--choir-surface-card);
    color: var(--choir-text-primary);
    box-shadow: var(--choir-card-shadow, 0 14px 34px color-mix(in srgb, var(--choir-shadow-color) 18%, transparent));
    font-size: 0.88rem;
    line-height: 1.42;
  }

  .auth-view h2:has(.passkey-info:hover) ~ .passkey-tooltip,
  .auth-view h2:has(.passkey-info:focus) ~ .passkey-tooltip,
  .passkey-tooltip.visible {
    display: block;
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
  :global(:root[data-theme-id='london-salmon']) .primary-action,
  :global(:root[data-theme-id='london-salmon']) .passkey-info {
    font-style: italic;
  }

  :global(:root[data-theme-id='london-salmon']) .auth-kicker {
    color: var(--choir-text-muted);
  }

  :global(:root[data-theme-id='london-salmon']) .auth-intent p {
    font-style: italic;
  }
</style>
