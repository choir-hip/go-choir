<!--
  DeviceApproveRoute — approve a `choir login` from another device
  (/device?code=XXXX-XXXX; docs/design/cli-agent-onboarding-2026-11-03.md).

  The page shows which device is asking (the label the CLI sent, the tool and
  network it asked from, when) before the owner decides. Approving mints a
  full-access key that only the CLI holding the request's device code can
  collect. Only a passkey session may decide; an email session is sent to
  sign in with a passkey.

  Data attributes for test targeting:
    data-device-route            — root, with data-device-state={state}
    data-device-signin-email     — email input when signed out
    data-device-signin-btn       — "Sign in with passkey"
    data-device-code-input       — code input when no code was in the link
    data-device-lookup-btn       — "Continue"
    data-device-code             — the code under review
    data-device-label            — the requesting device's label
    data-device-from             — the tool and network it asked from
    data-device-approve          — "Approve"
    data-device-deny             — "Deny"
    data-device-error            — error message
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { getSession, loginPasskey, fetchWithRenewal, AuthRequiredError, passkeyErrorMessage } from './auth.js';

  type RouteState = 'checking' | 'signin' | 'enter' | 'loading' | 'review' | 'working' | 'approved' | 'denied' | 'gone' | 'passkey';

  let state: RouteState = 'checking';
  let code = '';
  let email = '';
  let error = '';
  let request = null;

  onMount(async () => {
    code = new URLSearchParams(window.location.search).get('code') || '';
    try {
      const session = await getSession();
      if (!session?.authenticated) {
        state = 'signin';
        return;
      }
    } catch (_err) {
      state = 'signin';
      return;
    }
    await afterSignIn();
  });

  async function afterSignIn() {
    if (code.trim()) {
      await lookup();
    } else {
      state = 'enter';
    }
  }

  async function signIn() {
    error = '';
    if (!email.trim()) {
      error = 'Enter the email of your Choir account.';
      return;
    }
    state = 'working';
    try {
      await loginPasskey(email.trim());
      await afterSignIn();
    } catch (err) {
      error = passkeyErrorMessage(err, 'login');
      state = 'signin';
    }
  }

  async function lookup() {
    error = '';
    state = 'loading';
    try {
      const res = await fetchWithRenewal(`/auth/device/lookup?user_code=${encodeURIComponent(code.trim())}`, { method: 'GET' });
      const body = await res.json().catch(() => ({}));
      if (res.status === 403) {
        state = 'passkey';
        return;
      }
      if (res.status === 404) {
        state = 'gone';
        return;
      }
      if (!res.ok) {
        error = body.error || `Lookup failed (${res.status})`;
        state = 'enter';
        return;
      }
      request = body;
      state = body.status === 'pending' ? 'review' : 'gone';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        state = 'signin';
        return;
      }
      error = 'Choir could not reach sign-in. Check your connection and try again.';
      state = 'enter';
    }
  }

  async function decide(decision) {
    error = '';
    state = 'working';
    try {
      const res = await fetchWithRenewal('/auth/device/approve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_code: code.trim(), decision }),
      });
      const body = await res.json().catch(() => ({}));
      if (res.status === 403) {
        state = 'passkey';
        return;
      }
      if (res.status === 404) {
        state = 'gone';
        return;
      }
      if (!res.ok) {
        error = body.error || `Request failed (${res.status})`;
        state = 'review';
        return;
      }
      state = decision === 'approve' ? 'approved' : 'denied';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        state = 'signin';
        return;
      }
      error = 'Choir could not reach sign-in. Check your connection and try again.';
      state = 'review';
    }
  }

  function formatTime(iso) {
    try {
      return new Date(iso).toLocaleTimeString();
    } catch (_err) {
      return iso;
    }
  }
</script>

<main class="device-page" data-device-route data-device-state={state}>
  <div class="device-card">
    <p class="kicker">Choir login</p>

    {#if state === 'checking' || state === 'loading'}
      <h1>Checking the request…</h1>
    {:else if state === 'signin' || (state === 'working' && !request)}
      <h1>Sign in to approve</h1>
      <p class="body">A device asked to sign in to your Choir account. Sign in with your passkey to see the request.</p>
      <input
        type="email"
        class="text-input"
        data-device-signin-email
        placeholder="you@example.com"
        autocomplete="username webauthn"
        bind:value={email}
      />
      {#if error}<p class="error" role="alert" data-device-error>{error}</p>{/if}
      <button type="button" class="primary-action" data-device-signin-btn disabled={state === 'working'} on:click={signIn}>
        Sign in with passkey
      </button>
    {:else if state === 'enter'}
      <h1>Enter the code</h1>
      <p class="body">Type the code your terminal shows.</p>
      <input
        type="text"
        class="text-input code-input"
        data-device-code-input
        placeholder="XXXX-XXXX"
        autocomplete="off"
        bind:value={code}
      />
      {#if error}<p class="error" role="alert" data-device-error>{error}</p>{/if}
      <button type="button" class="primary-action" data-device-lookup-btn on:click={lookup}>Continue</button>
    {:else if state === 'review' || state === 'working'}
      <h1>Approve this login?</h1>
      <p class="code" data-device-code>{code.toUpperCase()}</p>
      <dl class="facts">
        <dt>Device</dt>
        <dd data-device-label>{request?.label}</dd>
        <dt>Asked from</dt>
        <dd data-device-from>{request?.requested_from}</dd>
        <dt>Asked at</dt>
        <dd>{formatTime(request?.requested_at)}</dd>
      </dl>
      <p class="warning">
        Approving gives that device {request?.grants || 'a full-access API key'}. Approve only a
        login you started yourself, and only if the code matches your terminal. Never approve a
        code someone sent you.
      </p>
      {#if error}<p class="error" role="alert" data-device-error>{error}</p>{/if}
      <button type="button" class="primary-action" data-device-approve disabled={state === 'working'} on:click={() => decide('approve')}>
        Approve
      </button>
      <button type="button" class="secondary-action" data-device-deny disabled={state === 'working'} on:click={() => decide('deny')}>
        Deny
      </button>
    {:else if state === 'approved'}
      <h1>Approved</h1>
      <p class="body">Return to your terminal; it finishes signing in on its own. You can revoke the key any time in Settings, API keys.</p>
    {:else if state === 'denied'}
      <h1>Denied</h1>
      <p class="body">The device gets nothing. You can close this page.</p>
    {:else if state === 'passkey'}
      <h1>Passkey needed</h1>
      <p class="error" role="alert" data-device-error>You are signed in by email. Sign in with a passkey to approve a device: approving creates an API key.</p>
    {:else}
      <h1>This code no longer works</h1>
      <p class="error" role="alert" data-device-error>
        Codes work once and expire after 10 minutes. Run <code>choir login</code> again for a new one.
      </p>
    {/if}
  </div>
</main>

<style>
  .device-page {
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

  .device-card {
    width: 100%;
    max-width: 460px;
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

  .body {
    margin: 0;
    font-size: 0.95rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .code {
    margin: 0;
    font-family: var(--choir-font-mono, ui-monospace, monospace);
    font-size: 1.6rem;
    font-weight: 760;
    letter-spacing: 0.12em;
  }

  .facts {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.3rem 0.8rem;
    margin: 0;
    font-size: 0.9rem;
  }

  .facts dt {
    color: var(--choir-text-muted);
  }

  .facts dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .warning {
    margin: 0;
    padding: 0.7rem 0.8rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-status-warning-soft, var(--choir-surface-control));
    font-size: 0.88rem;
    line-height: 1.4;
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

  .code-input {
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .primary-action,
  .secondary-action {
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

  .primary-action:disabled,
  .secondary-action:disabled {
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
