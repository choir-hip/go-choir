<!--
  App — root Svelte component.

  Checks auth state on mount via GET /auth/session. Renders the desktop
  shell for signed-out and signed-in visitors, then overlays AuthEntry
  when a signed-out visitor chooses a mutable action.

  Rehydration and renewal behaviour (VAL-CROSS-004 / VAL-CROSS-005):
    - On mount (including hard reload / new tab), checkSession() calls
      GET /auth/session which automatically does refresh rotation if the
      access JWT is expired but the refresh cookie is valid.
    - If the session is valid, the shell is rendered and bootstraps its
      protected routes (bootstrap + WS) using cookie-backed auth.
    - If both access and refresh state are invalid, the app falls back
      to the public desktop with auth available as an overlay (VAL-CROSS-008).

  Does NOT eagerly call protected routes (/api/shell/bootstrap, /api/ws)
  while the user is signed out.

  Passkey ceremony errors (cancel/failure) keep the user in a retryable
  guest auth state and never reveal the authenticated shell.
-->
<script>
  import AuthEntry from './lib/AuthEntry.svelte';
  import Desktop from './lib/Desktop.svelte';
  import { registerPasskey, loginPasskey, passkeyErrorMessage, prewarmAuthenticatedComputer, getSession } from './lib/auth.js';
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
  let authOverlayOpen = false;
  let pendingAuthIntent = null;
  let promptReplay = null;
  let promptReplayCounter = 0;
  let appReplay = null;
  let appReplayCounter = 0;
  let sessionCheckSeq = 0;
  let lastPrewarmStartedAt = 0;
  let publicRoutePath = '';

  $: isAuthenticated = authState === 'signed_in';
  $: authIntentMessage = getAuthIntentMessage(pendingAuthIntent);

  function startAuthenticatedPrewarm() {
    const now = Date.now();
    if (now - lastPrewarmStartedAt < 5000) return;
    lastPrewarmStartedAt = now;
    prewarmAuthenticatedComputer().catch(() => {});
  }

  async function checkSession() {
    const seq = ++sessionCheckSeq;
    const isCurrentCheck = () => seq === sessionCheckSeq;
    try {
      const data = await getSession();
      if (!isCurrentCheck()) return { authenticated: false, stale: true };
      if (data.authenticated && data.user) {
        authState = 'signed_in';
        currentUser = data.user;
        startAuthenticatedPrewarm();
        return { authenticated: true, user: data.user };
      } else {
        authState = 'signed_out';
        currentUser = null;
        return { authenticated: false };
      }
    } catch (_err) {
      if (!isCurrentCheck()) return { authenticated: false, stale: true };
      // Network error or unreachable — stay signed out.
      authState = 'signed_out';
      currentUser = null;
      return { authenticated: false };
    }
  }

  function getAuthIntentMessage(intent) {
    if (!intent) return 'Sign in to continue.';
    if (intent.kind === 'prompt') {
      return `Sign in to run: ${intent.text}`;
    }
    if (intent.kind === 'session_expired') {
      return 'Your session expired. Sign in to continue.';
    }
    if (intent.kind === 'app_launch') {
      return `Sign in to open ${intent.appName || 'this app'}.`;
    }
    if (intent.kind === 'published_vtext_edit') {
      return `Sign in to edit your version of ${intent.title || 'this published VText'}.`;
    }
    return 'Sign in to continue.';
  }

  function clearAuthOverlay() {
    authOverlayOpen = false;
    pendingAuthIntent = null;
    passkeyError = '';
  }

  function handleAuthRequired(event) {
    pendingAuthIntent = event.detail || { kind: 'sign_in' };
    authOverlayOpen = true;
    passkeyError = '';
  }

  function maybeReplayPendingIntent(intent) {
    if (intent?.kind === 'prompt' && intent.text) {
      promptReplay = {
        id: `prompt-replay-${++promptReplayCounter}`,
        text: intent.text,
      };
      return;
    }
    if (intent?.kind === 'published_vtext_edit') {
      appReplay = {
        id: `app-replay-${++appReplayCounter}`,
        appId: 'vtext',
        appName: 'VText',
        icon: '📝',
        appContext: {
          publishedRoutePath: intent.routePath || publicRoutePath || window.location.pathname,
          windowTitle: intent.title || 'Published VText',
          startPublishedDerivative: true,
          allowMultiple: true,
        },
      };
      return;
    }
    if (intent?.kind === 'app_launch' && intent.appId) {
      appReplay = {
        id: `app-replay-${++appReplayCounter}`,
        appId: intent.appId,
        appName: intent.appName || intent.appId,
        icon: intent.icon || '',
        appContext: intent.appContext || {},
      };
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

      startAuthenticatedPrewarm();

      // Ceremony succeeded — re-check session to transition to
      // the authenticated state.
      const session = await checkSession();
      if (session?.authenticated) {
        maybeReplayPendingIntent(pendingAuthIntent);
        authOverlayOpen = false;
        pendingAuthIntent = null;
      }
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
    sessionCheckSeq++;
    authState = 'signed_out';
    currentUser = null;
    authOverlayOpen = false;
    pendingAuthIntent = null;
    try {
      await fetch('/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
    } catch (_err) {
      // Logout request failed — still transition to signed-out
      // state locally so the user is not stuck in the shell.
    }
  }

  /**
   * Handles the authexpired event from the Shell component.
   * When a protected request fails with 401 and renewal cannot restore
   * the session, the Shell dispatches this event. The app transitions
   * cleanly to the guest auth state (VAL-CROSS-008).
   */
  function handleAuthExpired() {
    sessionCheckSeq++;
    authState = 'signed_out';
    currentUser = null;
    passkeyError = '';
    pendingAuthIntent = { kind: 'session_expired' };
    authOverlayOpen = true;
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
    publicRoutePath = window.location.pathname.startsWith('/pub/vtext/') ? window.location.pathname : '';
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

<div class="app-root" data-theme-id={currentTheme.id} data-auth-state={authState}>
  {#if authState === 'checking'}
    <div class="loading">
      <p>Loading…</p>
    </div>
  {:else}
    <Desktop
      {currentUser}
      authenticated={isAuthenticated}
      {promptReplay}
      {appReplay}
      {publicRoutePath}
      on:logout={handleLogout}
      on:authexpired={handleAuthExpired}
      on:authrequired={handleAuthRequired}
    />
    {#if authOverlayOpen && !isAuthenticated}
      <div class="auth-overlay" data-auth-overlay>
        <div class="auth-overlay-panel" role="dialog" aria-modal="true" aria-label="Sign in to continue">
          <button
            class="auth-overlay-close"
            data-auth-overlay-close
            type="button"
            on:click={clearAuthOverlay}
            aria-label="Close sign in"
          >
            x
          </button>
          <p class="auth-intent" data-auth-intent>{authIntentMessage}</p>
          <AuthEntry
            {passkeyError}
            {ceremonyInProgress}
            on:authbegin={handleAuthBegin}
            on:clearpasskeyerror={handleClearPasskeyError}
          />
        </div>
      </div>
    {/if}
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

  .auth-overlay {
    position: fixed;
    inset: 0;
    z-index: 1000;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    background: rgba(3, 7, 18, 0.62);
    backdrop-filter: blur(10px);
  }

  .auth-overlay-panel {
    position: relative;
    width: min(100%, 430px);
  }

  .auth-overlay-close {
    position: absolute;
    top: 0.75rem;
    right: 0.75rem;
    z-index: 2;
    width: 2rem;
    height: 2rem;
    border: 1px solid rgba(148, 163, 184, 0.22);
    border-radius: 999px;
    background: rgba(15, 23, 42, 0.86);
    color: #e2e8f0;
    cursor: pointer;
    font-size: 1rem;
    line-height: 1;
  }

  .auth-overlay-close:hover {
    background: rgba(30, 41, 59, 0.95);
  }

  .auth-intent {
    margin: 0 0 0.65rem;
    color: #dbeafe;
    font-size: 0.86rem;
    line-height: 1.4;
    overflow-wrap: anywhere;
    text-align: center;
  }

  .auth-overlay :global(.auth-entry) {
    min-height: auto;
  }

  .auth-overlay :global(.auth-card) {
    box-shadow: 0 24px 70px rgba(0, 0, 0, 0.46);
  }
</style>
