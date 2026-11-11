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
<script lang="ts">
  import AuthEntry from './lib/AuthEntry.svelte';
  import Desktop from './lib/Desktop.svelte';
  import LegalDocument from './lib/LegalDocument.svelte';
  import RecoverRoute from './lib/RecoverRoute.svelte';
  import EmailSignInRoute from './lib/EmailSignInRoute.svelte';
  import DeviceApproveRoute from './lib/DeviceApproveRoute.svelte';
  import { legacyAutopaperRedirect } from './lib/autopaper';
  import { registerPasskey, loginPasskey, passkeyErrorMessage, prewarmAuthenticatedComputer, getSession, TransientAuthError } from './lib/auth.js';
  import { markReturning, onAccountSwitched, scopedKey, setLastUserId, switchAccount } from './lib/accounts.js';
  import { prepareAccountSwitch } from './lib/shell-services';
  import { handoffToComputerSurfaceIfStale } from './lib/computer-surface-handoff.js';
  import { applyThemeToElement, isKnownThemeId, normalizeThemeConfig, systemDefaultTheme, validateThemeConfig } from './lib/theme';
  import { fetchThemePreference, saveThemePreference } from './lib/preferences.js';
  import { links } from './lib/services/links';
  import { contributionsSettled } from './lib/contributions';
  import { bindThemeHost, publishTheme, rememberWantedTheme } from './lib/services/theme';
  import { addLiveEventListener, isOwnLiveEvent, liveEventPayload } from './lib/live-events.js';

  /** @type {'checking' | 'signed_out' | 'signed_in' | 'reconnecting'} */
  let authState = 'checking';

  /** Current authenticated user, if any. */
  let currentUser = null;

  /** Passkey ceremony error message displayed in the AuthEntry. */
  let passkeyError = '';

  /** Whether a passkey ceremony is in progress. */
  let ceremonyInProgress = false;

  // An owner who has not chosen a theme sees the default for the system colour
  // scheme (light: Chrome Pastel, dark: Liquid Metal). That default is never
  // written to the boot cache or the server: only a chosen theme is.
  let currentTheme = systemDefaultTheme();
  let usingDefaultTheme = true;
  let authOverlayOpen = false;
  let pendingAuthIntent = null;
  let promptReplay = null;
  let promptReplayCounter = 0;
  let appReplay = null;
  let appReplayCounter = 0;
  let sessionCheckSeq = 0;
  let lastPrewarmStartedAt = 0;
  let publicRoutePath = '';
  let legalDocumentKind: 'privacy' | 'terms' = 'privacy';
  let isLegalDocumentRoute = false;
  /** /recover: landing page for an account-recovery email link. */
  let isRecoverRoute = false;
  /** /signin/email: landing page for an email sign-in link. */
  let isEmailSignInRoute = false;
  /** /device: approve a `choir login` from another device. */
  let isDeviceApproveRoute = false;
  const THEME_BOOT_CACHE_KEY = 'choir.theme.boot.v2';

  $: isAuthenticated = authState === 'signed_in';
  $: authIntentMessage = getAuthIntentMessage(pendingAuthIntent);

  function summarizeIntentText(value, maxLength = 120) {
    const text = String(value || '').replace(/\s+/g, ' ').trim();
    if (text.length <= maxLength) return text;
    return `${text.slice(0, maxLength - 1).trimEnd()}…`;
  }

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
        // The per-user pointer comes first: the theme cache and the other per-user keys follow it.
        setLastUserId(data.user.id);
        markReturning(data.user.id);
        startAuthenticatedPrewarm();
        void loadServerTheme();
        return { authenticated: true, user: data.user };
      } else {
        authState = 'signed_out';
        currentUser = null;
        return { authenticated: false };
      }
    } catch (err) {
      if (!isCurrentCheck()) return { authenticated: false, stale: true };
      // Transient error (5xx, network) — auth service may be restarting
      // during a deploy. Don't log the user out; transition to reconnecting
      // and retry after a delay. Only a definitive "not authenticated"
      // response from the server should cause logout.
      if (err instanceof TransientAuthError) {
        // Transient error — don't log out. If we were signed in, keep
        // showing the Desktop while we retry. If we were still checking
        // (initial load), show a reconnecting state.
        if (authState !== 'signed_in') {
          authState = 'reconnecting';
        }
        // A 429 carries the wait (Retry-After); other transient errors use 3 s.
        const retryMs = Math.min(Math.max(3000, err.retryAfterMs || 0), 120000);
        setTimeout(() => {
          if (seq === sessionCheckSeq) checkSession();
        }, retryMs);
        return { authenticated: authState === 'signed_in', transient: true, retryAfterMs: retryMs };
      }
      // Permanent error (401/403) — server says not authenticated.
      authState = 'signed_out';
      currentUser = null;
      return { authenticated: false };
    }
  }

  function getAuthIntentMessage(intent) {
    if (!intent) return 'Open your private computer and keep working.';
    if (intent.kind === 'sign_in') return 'Open your private computer and keep working.';
    if (intent.kind === 'add_account') return 'Sign in to another account. Your current account stays signed in; switch between them from the Desk menu.';
    if (intent.kind === 'prompt') {
      return `Run your prompt after sign-in: “${summarizeIntentText(intent.text)}”`;
    }
    if (intent.kind === 'session_expired') {
      return 'Reconnect to your private computer and resume where you stopped.';
    }
    if (intent.kind === 'app_launch') {
      return `Open ${intent.appName || 'this app'} with your saved work.`;
    }
    if (intent.kind === 'save_texture') return 'Save this Texture as a new revision.';
    if (intent.kind === 'publish_texture') return 'Publish this Texture from your account.';
    if (intent.kind === 'file_upload') return 'Upload these files to your private computer.';
    if (intent.kind === 'file_mutation') return 'Make this change to your saved files.';
    if (String(intent.kind || '').startsWith('email')) return 'Open your mailbox and continue this email action.';
    if (String(intent.kind || '').startsWith('podcast')) return 'Continue this podcast action with your saved subscriptions.';
    return 'Open your private computer and continue this action.';
  }

  function clearAuthOverlay() {
    authOverlayOpen = false;
    pendingAuthIntent = null;
    passkeyError = '';
    tick().then(() => requestAnimationFrame(() => {
      const returnTarget = document.querySelector('[data-prompt-input]');
      if (returnTarget instanceof HTMLElement) returnTarget.focus();
    }));
  }

  function handleAuthOverlayKeydown(event: KeyboardEvent) {
    if (!authOverlayOpen || event.key !== 'Escape' || ceremonyInProgress) return;
    event.preventDefault();
    clearAuthOverlay();
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
        focus: intent.focus || null,
        windowId: intent.windowId || '',
        aim: intent.aim || null,
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

  function initialAppIntentFromURL() {
    if (typeof window === 'undefined') return null;
    const params = new URLSearchParams(window.location.search || '');
    const appId = (params.get('app') || '').trim().toLowerCase();
    if (appId === 'email') {
      const draftId = (params.get('draft') || '').trim();
      const approvalToken = (params.get('approval') || '').trim();
      if (!draftId && !approvalToken) {
        clearConsumedAppIntentFromURL();
        return null;
      }
      return {
        kind: 'app_launch',
        source: 'url',
        appId: 'email',
        appName: 'Mail',
        icon: '📬',
        appContext: {
          draftId,
          approvalToken,
          windowTitle: 'Mail',
        },
      };
    }
    // An app a package declares a link for (contributes.links): its parameters make the launch.
    const linked = links.resolve(window.location.search || '');
    if (linked) return linked;
    if (appId) {
      clearConsumedAppIntentFromURL();
    }
    return null;
  }

  function clearConsumedAppIntentFromURL(intent = null) {
    if (typeof window === 'undefined') return;
    if (intent && intent.source !== 'url') return;
    const url = new URL(window.location.href);
    if (!url.searchParams.has('app')) return;
    url.searchParams.delete('app');
    url.searchParams.delete('draft');
    url.searchParams.delete('approval');
    url.searchParams.delete('intake');
    url.searchParams.delete('intake_id');
    for (const name of links.consumedParams()) url.searchParams.delete(name);
    const next = `${url.pathname}${url.search}${url.hash}`;
    window.history.replaceState(window.history.state, '', next || '/');
  }

  /**
   * After a ceremony succeeded, the session read can be refused (429) or slow
   * (a restart). Wait out the stated delay and read again, so the sign-in
   * still reaches the computer-surface handoff; the sheet says why it waits.
   */
  async function settleSessionAfterSignIn(session) {
    let current = session;
    for (let attempt = 0; current?.transient && attempt < 3; attempt++) {
      passkeyError = 'Signed in. Choir is confirming your session; this will finish in a moment.';
      await new Promise((resolve) => setTimeout(resolve, Math.min(current.retryAfterMs || 3000, 65000) + 250));
      current = await checkSession();
    }
    if (!current?.transient) passkeyError = '';
    return current;
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
      const session = await settleSessionAfterSignIn(await checkSession());
      if (session?.authenticated) {
        markReturning(session.user?.id);
        authOverlayOpen = false;
        pendingAuthIntent = null;

        // Await prewarm so the user's computer microVM is awake and serving
        await prewarmAuthenticatedComputer().catch(() => {});

        // Under C15/I25, the UI that renders a user computer is that computer's surface.
        // The unauthenticated document was loaded from the host platform shell.
        // Handoff cleanly to the authenticated computer's surface so the browser
        // executes the computer's exact asset graph instead of mixing host and guest chunk hashes.
        window.location.replace(window.location.href);
        return;
      }
      if (session?.transient) {
        passkeyError = 'Signed in, but Choir could not confirm your session yet. Reload in a minute.';
      }
    } catch (err) {
      // Ceremony failed or was cancelled — stay in signed-out
      // state and display a retryable error message.
      // (An add-account ceremony leaves the signed-in account as it is.)
      if (!isAuthenticated) authState = 'signed_out';
      passkeyError = passkeyErrorMessage(err, type);
    } finally {
      ceremonyInProgress = false;
    }
  }

  /** The email code was accepted: the session cookies are set. Same landing as a passkey sign-in. */
  async function handleEmailSignedIn() {
    passkeyError = '';
    startAuthenticatedPrewarm();
    const session = await settleSessionAfterSignIn(await checkSession());
    if (session?.authenticated) {
      markReturning(session.user?.id);
      authOverlayOpen = false;
      pendingAuthIntent = null;
      await prewarmAuthenticatedComputer().catch(() => {});
      window.location.replace(window.location.href);
    } else if (session?.transient) {
      passkeyError = 'Signed in, but Choir could not confirm your session yet. Reload in a minute.';
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
      const res = await fetch('/auth/logout', {
        method: 'POST',
        credentials: 'include',
      });
      // Another account of this browser is still signed in: hand over to it.
      const body = await res.json().catch(() => ({}));
      if (body?.next_user_id) {
        try {
          await switchAccount(body.next_user_id);
          return;
        } catch (_err) {
          // The switch failed: stay signed out; the account stays in the menu.
        }
      }
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

  // No theme at all (null, undefined, an empty object) means the owner has not
  // chosen one: the system default shows and nothing is stored. A theme whose
  // id is not selectable yet (a pack still loading) shows the default the same
  // way; the owner's stored choice is applied when the pack registers.
  function isNoChoice(theme) {
    return !theme || typeof theme !== 'object' || Object.keys(theme).length === 0 || !isKnownThemeId(theme.id);
  }

  function applyTheme(theme, persist = true) {
    const noChoice = isNoChoice(theme);
    const normalized = noChoice ? systemDefaultTheme() : normalizeThemeConfig(theme);
    const validation = validateThemeConfig(normalized);
    if (!validation.ok) {
      return validation;
    }
    usingDefaultTheme = noChoice;
    currentTheme = normalized;
    publishTheme(normalized);
    applyThemeToElement(document.documentElement, normalized);
    if (noChoice) {
      clearThemeBootCache();
    } else {
      saveThemeBootCache(normalized);
      if (persist && isAuthenticated) {
        saveThemePreference(normalized).catch(() => {});
      }
    }
    return validation;
  }

  function clearThemeBootCache() {
    try {
      window.localStorage?.removeItem(scopedKey(THEME_BOOT_CACHE_KEY));
    } catch (_err) {
      // The cache is only a first-paint hint.
    }
  }

  function loadThemeBootCache() {
    try {
      const raw = window.localStorage?.getItem(scopedKey(THEME_BOOT_CACHE_KEY));
      if (!raw) return null;
      const parsed = JSON.parse(raw);
      const validation = validateThemeConfig(parsed);
      return validation.ok ? normalizeThemeConfig(parsed) : null;
    } catch (_err) {
      return null;
    }
  }

  function saveThemeBootCache(theme) {
    try {
      window.localStorage?.setItem(scopedKey(THEME_BOOT_CACHE_KEY), JSON.stringify(normalizeThemeConfig(theme)));
    } catch (_err) {
      // The server preference remains authoritative; the boot cache is only a first-paint hint.
    }
  }

  async function loadServerTheme() {
    try {
      const stored = await fetchThemePreference();
      const theme = stored && Object.keys(stored).length > 0 ? stored : null;
      rememberWantedTheme(theme);
      applyTheme(theme, false);
    } catch (_err) {
      // Theme sync should not block desktop recovery.
    }
  }

  import { onMount, tick } from 'svelte';
  function isRecoverPath(pathname) {
    return pathname === '/recover' || pathname === '/recover/';
  }

  function isEmailSignInPath(pathname) {
    return pathname === '/signin/email' || pathname === '/signin/email/';
  }

  function isDeviceApprovePath(pathname) {
    return pathname === '/device' || pathname === '/device/';
  }

  function legalDocumentKindFromPath(pathname): '' | 'privacy' | 'terms' {
    if (pathname === '/privacy' || pathname === '/privacy/') return 'privacy';
    if (pathname === '/terms' || pathname === '/terms/') return 'terms';
    return '';
  }

  onMount(() => {
    const legalKind = legalDocumentKindFromPath(window.location.pathname);
    legalDocumentKind = legalKind || 'privacy';
    isLegalDocumentRoute = !!legalKind;
    isRecoverRoute = isRecoverPath(window.location.pathname);
    isEmailSignInRoute = isEmailSignInPath(window.location.pathname);
    isDeviceApproveRoute = isDeviceApprovePath(window.location.pathname);
    // The standalone /autopaper pages are gone: an old link replaces the
    // address with the story's public Texture route (the desktop opens it
    // in a Texture window) or the desktop home.
    const legacy = legacyAutopaperRedirect(window.location.pathname);
    if (legacy !== null) {
      window.history.replaceState(window.history.state, '', `${legacy}${window.location.search}${window.location.hash}`);
    }
    applyTheme(loadThemeBootCache(), false);
    // The recovery page stands alone: no session check, prewarm or
    // computer-surface handoff (a handoff reload would drop the in-memory
    // recovery token). Deep links and public routes are the packages'
    // contributes.links, read once the in-tree contributions have applied.
    if (!isRecoverRoute && !isEmailSignInRoute && !isDeviceApproveRoute) contributionsSettled().then(() => {
      // A package's public route (/pub/<package>/...) opens its app on the desktop.
      publicRoutePath = links.resolveRoute(window.location.pathname) ? window.location.pathname : '';
      return initialAppIntentFromURL();
    }).then((initialIntent) => checkSession().then(async (session) => {
      if (session?.authenticated) {
        await prewarmAuthenticatedComputer().catch(() => {});
        if (await handoffToComputerSurfaceIfStale()) return;
      }
      if (!initialIntent) return;
      if (session?.authenticated) {
        maybeReplayPendingIntent(initialIntent);
        clearConsumedAppIntentFromURL(initialIntent);
      } else {
        pendingAuthIntent = initialIntent;
        authOverlayOpen = true;
      }
    }));

    // setTheme in the SDK (services/theme.ts) applies and persists through this host.
    const unbindThemeHost = bindThemeHost({ apply: (theme, persist) => applyTheme(theme, persist) });
    const removeLiveListener = addLiveEventListener((message) => {
      if (message.kind !== 'theme.updated' || isOwnLiveEvent(message)) return;
      const liveTheme = liveEventPayload(message).theme || null;
      rememberWantedTheme(liveTheme);
      applyTheme(liveTheme, false);
    });

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

    // Another tab switched or signed out accounts: this tab flushes and
    // reloads, so none keeps account A's in-memory state while its calls run
    // under B. (The account_switched session result is the backstop.)
    const offAccountSwitched = onAccountSwitched(async () => {
      sessionCheckSeq++;
      try { await prepareAccountSwitch(); } catch (_err) { /* the reload follows */ }
      window.location.reload();
    });

    // The system default follows the system colour scheme while the owner has not chosen a theme.
    const schemeQuery = window.matchMedia?.('(prefers-color-scheme: dark)');
    const handleSchemeChange = () => { if (usingDefaultTheme) applyTheme(null, false); };
    schemeQuery?.addEventListener?.('change', handleSchemeChange);

    return () => {
      offAccountSwitched();
      schemeQuery?.removeEventListener?.('change', handleSchemeChange);
      unbindThemeHost();
      removeLiveListener();
      window.removeEventListener('pageshow', handlePageShow);
      window.removeEventListener('focus', handleFocus);
    };
  });
</script>

<svelte:window on:keydown={handleAuthOverlayKeydown} />

<div class="app-root" data-theme-id={currentTheme.id} data-auth-state={authState}>
  {#if isLegalDocumentRoute}
    <LegalDocument kind={legalDocumentKind} />
  {:else if isRecoverRoute}
    <RecoverRoute />
  {:else if isEmailSignInRoute}
    <EmailSignInRoute />
  {:else if isDeviceApproveRoute}
    <DeviceApproveRoute />
  {:else if authState === 'checking' || authState === 'reconnecting'}
    <div class="loading" role="status" aria-live="polite">
      <span class="loading-orbit" aria-hidden="true"></span>
      <p>{authState === 'reconnecting' ? 'Reconnecting…' : 'Loading…'}</p>
    </div>
  {:else}
    <Desktop
      {currentUser}
      authenticated={isAuthenticated}
      {promptReplay}
      {appReplay}
      {publicRoutePath}
      theme={currentTheme}
      on:logout={handleLogout}
      on:authexpired={handleAuthExpired}
      on:authrequired={handleAuthRequired}
    />
    {#if authOverlayOpen && (!isAuthenticated || pendingAuthIntent?.kind === 'add_account')}
      <div class="auth-overlay" data-auth-overlay data-auth-intent-kind={pendingAuthIntent?.kind || ''}>
        <div class="auth-overlay-panel" role="dialog" aria-modal="true" aria-label="Sign in to Choir">
          <button
            class="auth-overlay-close"
            data-auth-overlay-close
            type="button"
            on:click={clearAuthOverlay}
            aria-label="Close sign-in and return to Choir"
          >
            ×
          </button>
          <AuthEntry
            {passkeyError}
            {ceremonyInProgress}
            intentMessage={authIntentMessage}
            initialEmail={pendingAuthIntent?.email || ''}
            on:authbegin={handleAuthBegin}
            on:emailsignedin={handleEmailSignedIn}
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
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.1rem;
    min-height: 100dvh;
    color: var(--choir-text-muted);
  }

  .loading p {
    font-size: 0.9rem;
    letter-spacing: 0.02em;
  }

  .loading-orbit {
    width: 2.4rem;
    height: 2.4rem;
    border-radius: 50%;
    border: 2px solid color-mix(in srgb, var(--choir-accent) 22%, transparent);
    border-top-color: var(--choir-accent);
    box-shadow: 0 0 24px color-mix(in srgb, var(--choir-accent) 24%, transparent);
    animation: choir-loading-orbit 0.85s linear infinite;
  }

  @keyframes choir-loading-orbit {
    to { transform: rotate(360deg); }
  }

  @media (prefers-reduced-motion: reduce) {
    .loading-orbit { animation-duration: 2.4s; }
  }

  .auth-overlay {
    position: fixed;
    inset: 0;
    z-index: 20000;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
    overflow: auto;
    overscroll-behavior: contain;
    background: color-mix(in srgb, var(--choir-bg) 58%, transparent);
    backdrop-filter: blur(10px);
  }

  .auth-overlay-panel {
    position: relative;
    width: min(100%, 480px);
  }

  .auth-overlay-close {
    position: absolute;
    top: 0.85rem;
    right: 0.85rem;
    z-index: 2;
    width: 2.15rem;
    height: 2.15rem;
    border: 0;
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    cursor: pointer;
    font-size: 0.95rem;
    line-height: 1;
    box-shadow: var(--choir-control-shadow);
  }

  .auth-overlay-close:hover {
    background: var(--choir-state-selected);
  }

  .auth-overlay-close:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 3px;
  }

  .auth-overlay :global(.auth-entry) {
    min-height: auto;
  }

  .auth-overlay :global(.auth-card) {
    max-width: 480px;
    box-shadow: var(--choir-shadow-floating);
  }

  :global(:root[data-theme-id='london-salmon']) .auth-overlay-close {
    font-family: var(--choir-font-ui, Georgia, serif);
    font-style: italic;
  }
</style>
