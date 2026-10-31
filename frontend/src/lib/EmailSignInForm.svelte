<!--
  EmailSignInForm — "Sign in with email": address, then the 8-digit code.

  Posts the address to /auth/email/begin (the server answers the same way for
  every address, so this screen never says whether an account exists), then the
  code to /auth/email/verify. The attempt cookie that binds the code to this
  browser is HttpOnly and travels on its own; only the attempt id is held here.

  Data attributes for test targeting:
    data-email-signin-form    — the address form
    data-email-signin-email   — address input
    data-email-signin-submit  — send-code button
    data-email-signin-code-form — the code form
    data-email-signin-code    — code input
    data-email-signin-verify  — verify button
    data-email-signin-error   — error message
    data-email-signin-back    — back to passkey sign-in
-->
<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import { beginEmailSignIn, verifyEmailCode } from './auth.js';

  export let initialEmail = '';

  const dispatch = createEventDispatcher();

  let email = initialEmail;
  let code = '';
  let attemptId = '';
  let state: 'email' | 'sending' | 'code' | 'verifying' = 'email';
  let error = '';
  let emailEl: HTMLInputElement | null = null;
  let codeEl: HTMLInputElement | null = null;

  function isValidEmail(value) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  }

  onMount(() => {
    tick().then(() => emailEl?.focus());
  });

  async function sendCode() {
    error = '';
    const value = email.trim();
    if (!value) {
      error = 'Please enter an email address.';
      return;
    }
    if (!isValidEmail(value)) {
      error = 'Please enter a valid email address.';
      return;
    }
    state = 'sending';
    try {
      const res = await beginEmailSignIn(value);
      attemptId = res?.attempt_id || '';
      code = '';
      state = 'code';
      tick().then(() => codeEl?.focus());
    } catch (err) {
      state = 'email';
      const status = err?.status || 0;
      if (status === 429) {
        error = 'Too many sign-in emails were requested. Wait an hour, then try again.';
      } else if (status === 400) {
        error = 'Please enter a valid email address.';
      } else {
        error = 'Choir could not send the email right now. Check your connection and try again.';
      }
    }
  }

  async function verify() {
    error = '';
    const value = code.replace(/\s+/g, '');
    if (!/^\d{8}$/.test(value)) {
      error = 'Enter the 8-digit code from the email.';
      return;
    }
    state = 'verifying';
    try {
      await verifyEmailCode(attemptId, value);
      dispatch('signedin');
    } catch (err) {
      state = 'code';
      const status = err?.status || 0;
      if (status === 429) {
        error = 'Too many attempts. Wait an hour, then try again.';
      } else if (status === 403) {
        error = 'Email sign-in is not available. Use your passkey.';
      } else if (status === 400) {
        error = 'That code did not work. Check the email, or ask for a new code.';
      } else {
        error = 'Choir could not check the code right now. Check your connection and try again.';
      }
      tick().then(() => codeEl?.select());
    }
  }

  function askAgain() {
    state = 'email';
    error = '';
    attemptId = '';
    code = '';
    tick().then(() => emailEl?.focus());
  }
</script>

{#if state === 'code' || state === 'verifying'}
  <form class="email-form" data-email-signin-code-form novalidate on:submit|preventDefault={verify}>
    <h2>Check your email</h2>
    <p class="lead">
      If <strong>{email.trim()}</strong> can sign in by email, an 8-digit code is on its way. It
      works once, in this browser, and expires in 10 minutes. Never share it with anyone.
    </p>
    <label for="email-signin-code">8-digit code</label>
    <input
      id="email-signin-code"
      name="code"
      type="text"
      bind:this={codeEl}
      bind:value={code}
      placeholder="12345678"
      autocomplete="one-time-code"
      inputmode="numeric"
      pattern="[0-9]*"
      maxlength="12"
      spellcheck="false"
      disabled={state === 'verifying'}
      aria-invalid={error ? 'true' : 'false'}
      required
      data-email-signin-code
    />
    <button type="submit" class="primary-action" disabled={state === 'verifying'} data-email-signin-verify>
      {#if state === 'verifying'}
        <span class="spinner" aria-hidden="true"></span>
        Checking…
      {:else}
        Sign in
      {/if}
    </button>
    {#if error}
      <p class="error" role="alert" data-email-signin-error>{error}</p>
    {/if}
    <button type="button" class="link-action" on:click={askAgain}>Send a new code or use a different email</button>
    <button type="button" class="link-action" data-email-signin-back on:click={() => dispatch('back')}>
      Back to sign in
    </button>
  </form>
{:else}
  <form class="email-form" data-email-signin-form novalidate on:submit|preventDefault={sendCode}>
    <p class="lead">
      Enter the email address on your Choir account. We will send a code. Passkeys are safer: use
      email only when your passkey is not available.
    </p>
    <label for="email-signin-email">Email address</label>
    <input
      id="email-signin-email"
      name="email"
      type="email"
      bind:this={emailEl}
      bind:value={email}
      placeholder="you@example.com"
      autocomplete="email"
      inputmode="email"
      autocapitalize="none"
      spellcheck="false"
      disabled={state === 'sending'}
      aria-invalid={error ? 'true' : 'false'}
      required
      data-email-signin-email
    />
    <button type="submit" class="primary-action" disabled={state === 'sending'} data-email-signin-submit>
      {#if state === 'sending'}
        <span class="spinner" aria-hidden="true"></span>
        Sending…
      {:else}
        Email me a code
      {/if}
    </button>
    {#if error}
      <p class="error" role="alert" data-email-signin-error>{error}</p>
    {/if}
    <button type="button" class="link-action" data-email-signin-back on:click={() => dispatch('back')}>
      Back to sign in
    </button>
  </form>
{/if}

<style>
  .email-form {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    text-align: left;
  }

  h2 {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 780;
    color: var(--choir-text-primary);
  }

  p {
    margin: 0;
    font-size: 0.94rem;
    line-height: 1.45;
    color: var(--choir-text-primary);
    overflow-wrap: anywhere;
  }

  .lead {
    color: var(--choir-text-muted);
  }

  label {
    font-size: 0.8rem;
    font-weight: 720;
    color: var(--choir-text-muted);
  }

  input {
    min-height: 3rem;
    padding: 0.78rem 0.95rem;
    font-size: max(16px, 1rem);
    background: var(--choir-surface-input);
    border: 0;
    border-radius: var(--choir-radius-control, 16px);
    color: var(--choir-text-primary);
    outline: none;
    box-shadow: inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 8%, transparent);
  }

  input[name='code'] {
    letter-spacing: 0.2em;
    font-variant-numeric: tabular-nums;
  }

  input:focus {
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--choir-accent) 28%, transparent);
  }

  .primary-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.55rem;
    min-height: 3rem;
    margin-top: 0.35rem;
    padding: 0.8rem 1rem;
    font-size: 1rem;
    font-weight: 800;
    border: none;
    border-radius: var(--choir-radius-control, 16px);
    background: var(--choir-accent);
    color: var(--choir-text-on-accent, var(--choir-text-primary));
    box-shadow: var(--choir-control-shadow);
    cursor: pointer;
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

  .link-action:hover,
  .link-action:focus-visible {
    text-decoration: underline;
  }

  .spinner {
    width: 1.05rem;
    height: 1.05rem;
    border-radius: 50%;
    border: 2px solid color-mix(in srgb, currentColor 35%, transparent);
    border-top-color: currentColor;
    animation: email-signin-spin 0.7s linear infinite;
  }

  @keyframes email-signin-spin {
    to { transform: rotate(360deg); }
  }

  .error {
    padding: 0.7rem 0.8rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
    font-size: 0.9rem;
  }
</style>
