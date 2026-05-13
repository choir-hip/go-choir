<!--
  App — root Svelte component.

  Checks auth state on mount via GET /auth/session. Renders the guest
  auth entry UI when signed out and the placeholder desktop shell when
  signed in.

  Rehydration and renewal behaviour (VAL-CROSS-004 / VAL-CROSS-005):
    - On mount (including hard reload / new tab), checkSession() calls
      GET /auth/session which automatically does refresh rotation if the
      access JWT is expired but the refresh cookie is valid.
    - If the session is valid, the shell is rendered and bootstraps its
      protected routes (bootstrap + WS) using cookie-backed auth.
    - If both access and refresh state are invalid, the app falls back
      to the guest auth UI (VAL-CROSS-008).

  Does NOT eagerly call protected routes (/api/shell/bootstrap, /api/ws)
  while the user is signed out.

  Passkey ceremony errors (cancel/failure) keep the user in a retryable
  guest auth state and never reveal the authenticated shell.
-->
<script>
  import AuthEntry from './lib/AuthEntry.svelte';
  import Desktop from './lib/Desktop.svelte';
  import { registerPasskey, loginPasskey, passkeyErrorMessage } from './lib/auth.js';
  import { DEFAULT_THEME, THEME_STORAGE_KEY, applyThemeToElement, normalizeThemeConfig, validateThemeConfig } from './lib/theme.js';

  /** @type {'checking' | 'signed_out' | 'signed_in'} */
  let authState = 'checking';

  /** Current authenticated user, if any. */
  let currentUser = null;

  /** Passkey ceremony error message displayed in the AuthEntry. */
  let passkeyError = '';

  /** Whether a passkey ceremony is in progress. */
  let ceremonyInProgress = false;

  let currentTheme = DEFAULT_THEME;

  async function checkSession() {
    try {
      const res = await fetch('/auth/session', {
        method: 'GET',
        credentials: 'include',
      });
      if (!res.ok) {
        authState = 'signed_out';
        return;
      }
      const data = await res.json();
      if (data.authenticated && data.user) {
        authState = 'signed_in';
        currentUser = data.user;
      } else {
        authState = 'signed_out';
      }
    } catch (_err) {
      // Network error or unreachable — stay signed out.
      authState = 'signed_out';
    }
  }

  async function handleAuthBegin(event) {
    const { email, type } = event.detail;
    passkeyError = '';
    ceremonyInProgress = true;

    try {
      if (type === 'register') {
        await registerPasskey(email);
      } else {
        await loginPasskey(email);
      }

      // Ceremony succeeded — re-check session to transition to
      // the authenticated state.
      await checkSession();
    } catch (err) {
      // Ceremony failed or was cancelled — stay in signed-out
      // state and display a retryable error message.
      authState = 'signed_out';
      passkeyError = passkeyErrorMessage(err);
    } finally {
      ceremonyInProgress = false;
    }
  }

  function handleClearPasskeyError() {
    passkeyError = '';
  }

  async function handleLogout() {
    passkeyError = '';
    try {
      await fetch('/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    } catch (_err) {
      // Logout request failed — still transition to signed-out
      // state locally so the user is not stuck in the shell.
    }
    authState = 'signed_out';
    currentUser = null;
  }

  /**
   * Handles the authexpired event from the Shell component.
   * When a protected request fails with 401 and renewal cannot restore
   * the session, the Shell dispatches this event. The app transitions
   * cleanly to the guest auth state (VAL-CROSS-008).
   */
  function handleAuthExpired() {
    authState = 'signed_out';
    currentUser = null;
    passkeyError = '';
  }

  function applyTheme(theme, persist = true) {
    const normalized = normalizeThemeConfig(theme);
    const validation = validateThemeConfig(normalized);
    if (!validation.ok) {
      return validation;
    }
    currentTheme = normalized;
    applyThemeToElement(document.documentElement, normalized);
    if (persist) {
      try {
        window.localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify(normalized));
      } catch (_err) {
        // Theme application should not depend on storage availability.
      }
    }
    return validation;
  }

  function loadStoredTheme() {
    try {
      const raw = window.localStorage.getItem(THEME_STORAGE_KEY);
      if (!raw) return DEFAULT_THEME;
      const parsed = JSON.parse(raw);
      const validation = validateThemeConfig(parsed);
      return validation.ok ? normalizeThemeConfig(parsed) : DEFAULT_THEME;
    } catch (_err) {
      return DEFAULT_THEME;
    }
  }

  import { onMount } from 'svelte';
  onMount(() => {
    applyTheme(loadStoredTheme(), false);
    checkSession();

    function handleThemeChange(event) {
      applyTheme(event.detail?.theme || DEFAULT_THEME);
    }
    window.addEventListener('choir-theme-change', handleThemeChange);

    // Prevent bfcache from resurrecting an authenticated shell after
    // logout. When the page is restored from back/forward cache, the
    // old JavaScript state may still show the shell even though the
    // server-side session has been invalidated. Re-check the session
    // on pageshow to catch this case (VAL-CROSS-006).
    function handlePageShow(event) {
      if (event.persisted) {
        // Page was restored from bfcache — re-verify auth state.
        checkSession();
      }
    }
    window.addEventListener('pageshow', handlePageShow);

    // Also listen for focus events as a secondary guard: if the user
    // switches back to this tab after logging out in another tab or
    // context, we re-check the session.
    function handleFocus() {
      if (authState === 'signed_in') {
        // Only re-check if we think we're signed in — avoids
        // unnecessary session checks while already signed out.
        checkSession();
      }
    }
    window.addEventListener('focus', handleFocus);

    return () => {
      window.removeEventListener('choir-theme-change', handleThemeChange);
      window.removeEventListener('pageshow', handlePageShow);
      window.removeEventListener('focus', handleFocus);
    };
  });
</script>

<div class="app-root" data-theme-id={currentTheme.id}>
  {#if authState === 'checking'}
    <div class="loading">
      <p>Loading…</p>
    </div>
  {:else if authState === 'signed_out'}
    <AuthEntry
      {passkeyError}
      {ceremonyInProgress}
      on:authbegin={handleAuthBegin}
      on:clearpasskeyerror={handleClearPasskeyError}
    />
  {:else if authState === 'signed_in'}
    <Desktop {currentUser} on:logout={handleLogout} on:authexpired={handleAuthExpired} />
  {/if}
</div>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(html),
  :global(body),
  :global(#app) {
    width: 100%;
    height: 100%;
    min-height: 100%;
    overflow: hidden;
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen,
      Ubuntu, Cantarell, 'Fira Sans', 'Droid Sans', 'Helvetica Neue', sans-serif;
    background: var(--choir-bg);
    color: var(--choir-fg);
    overscroll-behavior: none;
  }

  .app-root {
    width: 100%;
    height: 100%;
    min-height: 100%;
    background: var(--choir-bg);
    color: var(--choir-fg);
  }

  :global(input),
  :global(textarea),
  :global([contenteditable="true"]) {
    font-size: max(16px, 1rem);
  }

  @supports (height: 100dvh) {
    :global(html),
    :global(body),
    :global(#app) {
      height: 100dvh;
      min-height: 100dvh;
    }
  }

  .loading {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100dvh;
    color: #888;
  }
</style>
