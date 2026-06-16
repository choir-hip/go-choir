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
  import { registerPasskey, loginPasskey, passkeyErrorMessage, prewarmAuthenticatedComputer, getSession } from './lib/auth.js';
  import { DEFAULT_THEME, applyThemeToElement, normalizeThemeConfig, validateThemeConfig } from './lib/theme';
  import { fetchThemePreference, saveThemePreference } from './lib/preferences.js';
  import { addLiveEventListener, isOwnLiveEvent, liveEventPayload } from './lib/live-events.js';

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
  let universalWirePublicToken = '';
  let universalWirePublicLink = null;
  let universalWirePublicStatus = '';
  let universalWirePublicError = '';
  const THEME_BOOT_CACHE_KEY = 'choir.theme.boot.v2';

  $: isAuthenticated = authState === 'signed_in';
  $: authIntentMessage = getAuthIntentMessage(pendingAuthIntent);
  $: isUniversalWirePublicReader = !!universalWirePublicToken;

  function normalizeTextureAuthIntentKind(kind) {
    return String(kind || '');
  }

  function isTextureAuthIntent(intent, ...kinds) {
    return kinds.includes(normalizeTextureAuthIntentKind(intent?.kind));
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
        startAuthenticatedPrewarm();
        void loadServerTheme();
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
    if (!intent) return 'Choose when to make this preview durable.';
    if (intent.kind === 'prompt') {
      return `This prompt will run on your computer: ${intent.text}`;
    }
    if (intent.kind === 'session_expired') {
      return 'Your session ended. Use your passkey to continue.';
    }
    if (intent.kind === 'app_launch') {
      return `Open ${intent.appName || 'this app'} with your private computer state.`;
    }
    if (isTextureAuthIntent(intent, 'save_texture')) return 'Save this Texture revision to your computer.';
    if (isTextureAuthIntent(intent, 'publish_texture')) return 'Publish this Texture as owner-scoped work.';
    if (intent.kind === 'file_upload') return 'Upload files into your private computer.';
    if (intent.kind === 'file_mutation') return 'Change files on your private computer.';
    if (String(intent.kind || '').startsWith('email')) return 'Use your mailbox, drafts, and send approval.';
    if (String(intent.kind || '').startsWith('podcast')) return 'Subscribe, import, search providers, or sync playback.';
    if (isTextureAuthIntent(intent, 'published_texture_edit')) {
      return `Edit your version of ${intent.title || 'this published Texture'}.`;
    }
    if (isTextureAuthIntent(intent, 'private_texture_document')) {
      return `Open ${intent.title || 'this Texture document'} from your private computer.`;
    }
    return 'Continue with private computer state.';
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
    if (isTextureAuthIntent(intent, 'published_texture_edit')) {
      appReplay = {
        id: `app-replay-${++appReplayCounter}`,
        appId: 'texture',
        appName: 'Texture',
        icon: '📝',
        appContext: {
          publishedRoutePath: intent.routePath || publicRoutePath || window.location.pathname,
          windowTitle: intent.title || 'Published Texture',
          startPublishedDerivative: true,
          allowMultiple: true,
        },
      };
      return;
    }
    if (isTextureAuthIntent(intent, 'private_texture_document') && intent.docId) {
      appReplay = {
        id: `app-replay-${++appReplayCounter}`,
        appId: 'texture',
        appName: 'Texture',
        icon: '📝',
        appContext: {
          docId: intent.docId,
          windowTitle: intent.title || 'Texture',
          createInitialVersion: false,
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

  function initialAppIntentFromURL() {
    if (typeof window === 'undefined') return null;
    const params = new URLSearchParams(window.location.search || '');
    const appId = (params.get('app') || '').trim().toLowerCase();
    if (appId === 'texture' || appId === 'texture') {
      const docId = (params.get('doc') || params.get('doc_id') || '').trim();
      if (!docId) {
        clearConsumedAppIntentFromURL();
        return null;
      }
      return {
        kind: 'private_texture_document',
        source: 'url',
        docId,
        title: (params.get('title') || '').trim() || 'Texture',
      };
    }
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
        appName: 'Email',
        icon: '✉️',
        appContext: {
          draftId,
          approvalToken,
          windowTitle: 'Email',
        },
      };
    }
    if (appId) {
      clearConsumedAppIntentFromURL();
    }
    return null;
  }

  function universalWirePublicTokenFromPath(pathname) {
    const prefix = '/universal-wire/publications/';
    if (!pathname.startsWith(prefix)) return '';
    return decodeURIComponent(pathname.slice(prefix.length).split('/')[0] || '').trim();
  }

  async function loadUniversalWirePublicLink(token) {
    if (!token) return;
    universalWirePublicStatus = 'error';
    universalWirePublicError = 'Legacy publication links were removed. Universal Wire will publish through platformd after auto-publish lands.';
    universalWirePublicLink = null;
  }

  function clearConsumedAppIntentFromURL(intent = null) {
    if (typeof window === 'undefined') return;
    if (intent && intent.source !== 'url') return;
    const url = new URL(window.location.href);
    if (!url.searchParams.has('app')) return;
    url.searchParams.delete('app');
    url.searchParams.delete('draft');
    url.searchParams.delete('approval');
    url.searchParams.delete('doc');
    url.searchParams.delete('doc_id');
    url.searchParams.delete('title');
    const next = `${url.pathname}${url.search}${url.hash}`;
    window.history.replaceState(window.history.state, '', next || '/');
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
        clearConsumedAppIntentFromURL(pendingAuthIntent);
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
    saveThemeBootCache(normalized);
    if (persist && isAuthenticated) {
      saveThemePreference(normalized).catch(() => {});
    }
    return validation;
  }

  function loadThemeBootCache() {
    try {
      const raw = window.localStorage?.getItem(THEME_BOOT_CACHE_KEY);
      if (!raw) return DEFAULT_THEME;
      const parsed = JSON.parse(raw);
      const validation = validateThemeConfig(parsed);
      return validation.ok ? normalizeThemeConfig(parsed) : DEFAULT_THEME;
    } catch (_err) {
      return DEFAULT_THEME;
    }
  }

  function saveThemeBootCache(theme) {
    try {
      window.localStorage?.setItem(THEME_BOOT_CACHE_KEY, JSON.stringify(normalizeThemeConfig(theme)));
    } catch (_err) {
      // The server preference remains authoritative; the boot cache is only a first-paint hint.
    }
  }

  async function loadServerTheme() {
    try {
      const stored = await fetchThemePreference();
      const theme = stored && Object.keys(stored).length > 0 ? stored : DEFAULT_THEME;
      applyTheme(theme, false);
    } catch (_err) {
      // Theme sync should not block desktop recovery.
    }
  }

  import { onMount } from 'svelte';
  function isPublicTextureRoutePath(pathname) {
    return pathname.startsWith('/pub/texture/');
  }

  onMount(() => {
    publicRoutePath = isPublicTextureRoutePath(window.location.pathname) ? window.location.pathname : '';
    universalWirePublicToken = universalWirePublicTokenFromPath(window.location.pathname);
    applyTheme(loadThemeBootCache(), false);
    if (universalWirePublicToken) {
      void loadUniversalWirePublicLink(universalWirePublicToken);
    }
    const initialIntent = initialAppIntentFromURL();
    checkSession().then((session) => {
      if (!initialIntent) return;
      if (session?.authenticated) {
        maybeReplayPendingIntent(initialIntent);
        clearConsumedAppIntentFromURL(initialIntent);
      } else {
        pendingAuthIntent = initialIntent;
        authOverlayOpen = true;
      }
    });

    function handleThemeChange(event) {
      applyTheme(event.detail?.theme || DEFAULT_THEME);
    }
    window.addEventListener('choir-theme-change', handleThemeChange);
    const removeLiveListener = addLiveEventListener((message) => {
      if (message.kind !== 'theme.updated' || isOwnLiveEvent(message)) return;
      applyTheme(liveEventPayload(message).theme || DEFAULT_THEME, false);
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

    return () => {
      window.removeEventListener('choir-theme-change', handleThemeChange);
      removeLiveListener();
      window.removeEventListener('pageshow', handlePageShow);
      window.removeEventListener('focus', handleFocus);
    };
  });
</script>

<div class="app-root" data-theme-id={currentTheme.id} data-auth-state={authState}>
  {#if isUniversalWirePublicReader}
    <main class="universal-wire-public-reader" data-universal-wire-public-reader>
      <header>
        <a class="reader-brand" href="/">Choir Universal Wire</a>
        <button
          type="button"
          on:click={() => {
            pendingAuthIntent = {
              kind: 'app_launch',
              appId: 'universal-wire',
              appName: 'Universal Wire',
              appContext: { windowTitle: 'Universal Wire' },
            };
            authOverlayOpen = true;
          }}
          data-universal-wire-public-sign-in
        >
          Sign in
        </button>
      </header>
      {#if universalWirePublicStatus === 'loading' || !universalWirePublicStatus}
        <section class="universal-wire-public-panel">
          <p data-universal-wire-public-status>Loading publication...</p>
        </section>
      {:else if universalWirePublicError}
        <section class="universal-wire-public-panel">
          <p data-universal-wire-public-error>{universalWirePublicError}</p>
        </section>
      {:else if universalWirePublicLink}
        <article class="universal-wire-public-panel" data-universal-wire-public-publication>
          <div class="reader-kicker">
            <span>{universalWirePublicLink.status}</span>
            <span>{universalWirePublicLink.route_path}</span>
            {#if universalWirePublicLink.feed_path}
              <a href={universalWirePublicLink.feed_path} data-universal-wire-public-feed>RSS</a>
            {/if}
          </div>
          <h1>{universalWirePublicLink.title}</h1>
          <pre>{universalWirePublicLink.export_body}</pre>
          <div class="reader-provenance" data-universal-wire-public-provenance>
            <strong>Provenance</strong>
            <span>citations: {universalWirePublicLink.citation_count}</span>
            <span>rollback refs: {universalWirePublicLink.rollback_count}</span>
            <span>export: {universalWirePublicLink.export_id}</span>
            <span>delivery: {universalWirePublicLink.delivery_id}</span>
          </div>
          <div class="reader-refs">
            <section data-universal-wire-public-citations>
              <h2>Citations</h2>
              <p>{(universalWirePublicLink.citation_refs || []).join(' · ')}</p>
            </section>
            <section data-universal-wire-public-rollback>
              <h2>Rollback Refs</h2>
              <p>{(universalWirePublicLink.rollback_refs || []).join(' · ')}</p>
            </section>
          </div>
        </article>
      {/if}
    </main>
    {#if authOverlayOpen && !isAuthenticated}
      <div class="auth-overlay" data-auth-overlay data-auth-intent-kind={pendingAuthIntent?.kind || ''}>
        <div class="auth-overlay-panel" role="dialog" aria-modal="true" aria-label="Use a passkey to continue">
          <button
            class="auth-overlay-close"
            data-auth-overlay-close
            type="button"
            on:click={clearAuthOverlay}
            aria-label="Close passkey sign in"
          >
            x
          </button>
          <AuthEntry
            {passkeyError}
            {ceremonyInProgress}
            intentMessage={authIntentMessage}
            on:authbegin={handleAuthBegin}
            on:clearpasskeyerror={handleClearPasskeyError}
          />
        </div>
      </div>
    {/if}
  {:else if authState === 'checking'}
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
      theme={currentTheme}
      on:logout={handleLogout}
      on:authexpired={handleAuthExpired}
      on:authrequired={handleAuthRequired}
    />
    {#if authOverlayOpen && !isAuthenticated}
      <div class="auth-overlay" data-auth-overlay data-auth-intent-kind={pendingAuthIntent?.kind || ''}>
        <div class="auth-overlay-panel" role="dialog" aria-modal="true" aria-label="Use a passkey to continue">
          <button
            class="auth-overlay-close"
            data-auth-overlay-close
            type="button"
            on:click={clearAuthOverlay}
            aria-label="Close passkey sign in"
          >
            x
          </button>
          <AuthEntry
            {passkeyError}
            {ceremonyInProgress}
            intentMessage={authIntentMessage}
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

  .universal-wire-public-reader {
    width: 100%;
    min-height: 100%;
    overflow: auto;
    background: var(--choir-bg);
    color: var(--choir-fg);
  }

  .universal-wire-public-reader header {
    position: sticky;
    top: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem clamp(1rem, 4vw, 2.5rem);
    border-bottom: 1px solid var(--choir-border);
    background: color-mix(in srgb, var(--choir-bg) 92%, transparent);
    backdrop-filter: blur(12px);
  }

  .reader-brand {
    color: var(--choir-fg);
    font-size: 0.85rem;
    font-weight: 780;
    text-decoration: none;
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }

  .universal-wire-public-reader button {
    min-height: 2rem;
    padding: 0.35rem 0.65rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    font-size: 0.78rem;
    font-weight: 720;
    cursor: pointer;
  }

  .universal-wire-public-panel {
    width: min(920px, calc(100% - 2rem));
    margin: 1rem auto 2rem;
    padding: clamp(1rem, 3vw, 2rem);
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
    box-shadow: var(--choir-window-shadow);
  }

  .reader-kicker,
  .reader-provenance {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem 0.8rem;
    color: var(--choir-text-secondary);
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }

  .universal-wire-public-panel h1 {
    margin: 0.75rem 0 1rem;
    color: var(--choir-text-primary);
    font-size: clamp(1.65rem, 3vw, 2.6rem);
    line-height: 1.08;
    overflow-wrap: anywhere;
  }

  .universal-wire-public-panel pre {
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    color: var(--choir-text-primary);
    font: inherit;
    font-size: 1rem;
    line-height: 1.55;
  }

  .reader-provenance {
    margin-top: 1.25rem;
    padding-top: 1rem;
    border-top: 1px solid var(--choir-border);
  }

  .reader-refs {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.8rem;
    margin-top: 1rem;
  }

  .reader-refs section {
    min-width: 0;
    padding: 0.65rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card);
  }

  .reader-refs h2 {
    margin: 0 0 0.35rem;
    font-size: 0.8rem;
    color: var(--choir-text-primary);
  }

  .reader-refs p {
    color: var(--choir-text-secondary);
    font-size: 0.78rem;
    line-height: 1.35;
    overflow-wrap: anywhere;
  }

  @media (max-width: 720px) {
    .reader-refs {
      grid-template-columns: 1fr;
    }

    .universal-wire-public-reader header {
      align-items: flex-start;
      flex-direction: column;
    }
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
    color: var(--choir-text-muted);
  }

  .auth-overlay {
    position: fixed;
    inset: 0;
    z-index: 20000;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
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
