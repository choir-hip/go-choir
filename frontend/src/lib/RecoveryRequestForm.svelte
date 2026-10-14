<!--
  RecoveryRequestForm — "Lost your passkey?" email form.

  Posts the address to /auth/recovery/request and then shows "check your
  email". The server answers the same way for every address, so this
  screen never says whether an account exists.

  Data attributes for test targeting:
    data-recovery-form      — the email form
    data-recovery-email     — email input
    data-recovery-submit    — send button
    data-recovery-sent      — "check your email" confirmation
    data-recovery-error     — error message
    data-recovery-retry     — use a different email
-->
<script lang="ts">
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import { requestRecoveryEmail } from './auth.js';

  export let initialEmail = '';
  /** Optional lead copy above the form (e.g. after an expired link). */
  export let lead = 'Enter the email address on your Choir account. We will send a link that lets you add a new passkey.';

  const dispatch = createEventDispatcher();

  let email = initialEmail;
  let state: 'form' | 'sending' | 'sent' = 'form';
  let error = '';
  let sentTo = '';
  let inputEl: HTMLInputElement | null = null;

  function isValidEmail(value) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  }

  onMount(() => {
    tick().then(() => inputEl?.focus());
  });

  async function submit() {
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
      await requestRecoveryEmail(value);
      sentTo = value;
      state = 'sent';
      dispatch('sent', { email: value });
    } catch (err) {
      state = 'form';
      const status = err?.status || 0;
      if (status === 429) {
        error = 'Too many recovery emails were requested. Wait an hour, then try again.';
      } else if (status === 400) {
        error = 'Please enter a valid email address.';
      } else {
        error = 'Choir could not send the email right now. Check your connection and try again.';
      }
    }
  }

  function useDifferentEmail() {
    state = 'form';
    error = '';
    tick().then(() => {
      inputEl?.focus();
      inputEl?.select();
    });
  }
</script>

{#if state === 'sent'}
  <div class="recovery-sent" data-recovery-sent role="status" aria-live="polite">
    <h2>Check your email</h2>
    <p>
      If <strong>{sentTo}</strong> belongs to a Choir account, a recovery link is on its way.
      Open it on this device. It works once and expires in 15 minutes.
    </p>
    <p class="muted">No email after a few minutes? Check spam, or try again.</p>
    <div class="actions">
      <button type="button" class="secondary-action" data-recovery-retry on:click={useDifferentEmail}>
        Send again or use a different email
      </button>
      <button type="button" class="link-action" data-recovery-back on:click={() => dispatch('back')}>
        Back to sign in
      </button>
    </div>
  </div>
{:else}
  <form class="recovery-form" data-recovery-form novalidate on:submit|preventDefault={submit}>
    <p class="lead">{lead}</p>
    <label for="recovery-email">Email address</label>
    <input
      id="recovery-email"
      name="email"
      type="email"
      bind:this={inputEl}
      bind:value={email}
      placeholder="you@example.com"
      autocomplete="email"
      inputmode="email"
      autocapitalize="none"
      spellcheck="false"
      disabled={state === 'sending'}
      aria-invalid={error ? 'true' : 'false'}
      aria-errormessage={error ? 'recovery-error' : undefined}
      required
      data-recovery-email
    />
    <button type="submit" class="primary-action" disabled={state === 'sending'} data-recovery-submit>
      {#if state === 'sending'}
        <span class="spinner" aria-hidden="true"></span>
        Sending…
      {:else}
        Email me a recovery link
      {/if}
    </button>
    {#if error}
      <p class="error" id="recovery-error" role="alert" data-recovery-error>{error}</p>
    {/if}
    <button type="button" class="link-action" data-recovery-back on:click={() => dispatch('back')}>
      Back to sign in
    </button>
  </form>
{/if}

<style>
  .recovery-form,
  .recovery-sent {
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

  .lead,
  .muted {
    color: var(--choir-text-muted);
  }

  label {
    font-size: 0.8rem;
    font-weight: 720;
    color: var(--choir-text-muted);
  }

  input[type="email"] {
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

  input[type="email"]:focus {
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--choir-accent) 28%, transparent);
  }

  .actions {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-top: 0.35rem;
  }

  .primary-action,
  .secondary-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.55rem;
    min-height: 3rem;
    padding: 0.8rem 1rem;
    font-size: 1rem;
    font-weight: 800;
    border: none;
    border-radius: var(--choir-radius-control, 16px);
    cursor: pointer;
  }

  .primary-action {
    margin-top: 0.35rem;
    background: var(--choir-accent);
    color: var(--choir-text-on-accent, var(--choir-text-primary));
    box-shadow: var(--choir-control-shadow);
  }

  .primary-action:disabled {
    background: var(--choir-surface-control);
    color: var(--choir-text-subtle);
    cursor: not-allowed;
  }

  .secondary-action {
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
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
    animation: recovery-spin 0.7s linear infinite;
  }

  @keyframes recovery-spin {
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
