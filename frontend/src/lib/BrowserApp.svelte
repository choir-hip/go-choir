<!--
  BrowserApp — Web Lens / Web Import app for the ChoirOS desktop.

  Features:
    - URL input bar at the top of the content area
    - Basic navigation: back, forward, reload
    - Loading indicator while page loads
    - backend Web Lens snapshots when configured
    - iframe fallback when backend browser is unavailable
    - Graceful Web Lens fallback language when sites block iframe embedding
    - Works in floating window and mobile focus mode

  Data attributes for test targeting:
    data-browser-app        — root browser container
    data-browser-url-bar    — URL bar area
    data-browser-url-input  — URL text input
    data-browser-go-btn     — Go/submit button
    data-browser-nav-back   — back navigation button
    data-browser-nav-forward — forward navigation button
    data-browser-nav-reload — reload button
    data-browser-loading    — loading indicator
    data-browser-iframe     — iframe element
    data-browser-error      — error message display
-->
<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';

  export let appContext = {};
  export let authenticated = false;
  const dispatch = createEventDispatcher();

  // ---- State ----
  const initialTarget = appContext?.initialUrl || appContext?.sourceUrl || '';
  let urlInput = initialTarget || 'https://en.wikipedia.org';
  let currentUrl = initialTarget ? normalizeUrl(initialTarget) : '';
  let loading = false;
  let error = '';
  let iframeEl = null;
  let browserCapabilities = null;
  let capabilityError = '';
  let backendSession = null;
  let backendSnapshot = '';
  let backendHTML = '';
  let backendLinks = [];
  let backendScreenshotPNG = '';
  let controlSelector = '';
  let controlValue = '';
  let controlStatus = '';
  let backendNavigationSeq = 0;
  let mounted = false;
  let capabilityLoadStarted = false;

  // Navigation history
  let history = [];
  let historyIndex = -1;

  // Can go back/forward?
  $: canGoBack = historyIndex > 0;
  $: canGoForward = historyIndex < history.length - 1;

  // ---- Navigation ----

  function normalizeUrl(raw) {
    let url = raw.trim();
    if (!url) return '';
    if (url.match(/^[a-zA-Z][a-zA-Z0-9+.-]*:/)) {
      return url;
    }
    // Add https:// if no protocol specified
    if (!url.match(/^[a-zA-Z]+:\/\//)) {
      // Check if it looks like a domain (contains a dot)
      if (url.includes('.')) {
        url = 'https://' + url;
      } else {
        // Treat as a search query — use Wikipedia search
        url = 'https://en.wikipedia.org/wiki/Special:Search?search=' + encodeURIComponent(url);
      }
    }
    return url;
  }

  function navigateToUrl(url, addToHistory = true) {
    const normalized = normalizeUrl(url);
    if (!normalized) return;

    error = '';
    loading = true;
    urlInput = normalized;
    currentUrl = normalized;

    if (addToHistory) {
      // Trim forward history
      history = history.slice(0, historyIndex + 1);
      history.push(normalized);
      historyIndex = history.length - 1;
    }

    if (browserCapabilities?.available) {
      navigateBackend(normalized);
    }
  }

  function clearBackendSnapshots() {
    backendSnapshot = '';
    backendHTML = '';
    backendLinks = [];
    backendScreenshotPNG = '';
  }

  function handleGo() {
    navigateToUrl(urlInput);
  }

  function handleUrlKeydown(event) {
    if (event.key === 'Enter') {
      handleGo();
    }
  }

  function goBack() {
    if (canGoBack) {
      historyIndex--;
      const url = history[historyIndex];
      urlInput = url;
      currentUrl = url;
      loading = true;
      error = '';
    }
  }

  function goForward() {
    if (canGoForward) {
      historyIndex++;
      const url = history[historyIndex];
      urlInput = url;
      currentUrl = url;
      loading = true;
      error = '';
    }
  }

  function reload() {
    if (currentUrl) {
      loading = true;
      error = '';
      const url = currentUrl;
      if (browserCapabilities?.available) {
        navigateBackend(url);
      } else {
        // Force iframe reload by briefly clearing the src.
        currentUrl = '';
        requestAnimationFrame(() => {
          currentUrl = url;
        });
      }
    }
  }

  function handleIframeLoad() {
    loading = false;
    // Try to detect if the iframe loaded correctly
    // We can't read iframe content due to cross-origin policy,
    // but the load event fires regardless
  }

  function handleIframeError() {
    loading = false;
    error = 'This site may block embedding. Use Web Lens snapshots for text, links, source, and import when the backend is available.';
  }

  function handleAuthError(err) {
    if (err instanceof AuthRequiredError) {
      dispatch('authexpired');
      return true;
    }
    return false;
  }

  function enterGuestMode() {
    browserCapabilities = {
      available: false,
      mode: 'guest_iframe',
      substrate: 'iframe',
      supports: {},
    };
    capabilityError = '';
    backendSession = null;
    clearBackendSnapshots();
  }

  async function loadBrowserCapabilities() {
    if (!authenticated) {
      enterGuestMode();
      return;
    }
    capabilityLoadStarted = true;
    capabilityError = '';
    try {
      const res = await fetchWithRenewal('/api/browser/capabilities', { method: 'GET' });
      if (!res.ok) {
        capabilityError = `Web Lens capability check failed (${res.status})`;
        return;
      }
      browserCapabilities = await res.json();
      if (browserCapabilities?.available) {
        await ensureBackendSession();
        if (currentUrl) {
          navigateBackend(currentUrl);
        }
      }
    } catch (err) {
      if (handleAuthError(err)) return;
      capabilityError = 'Web Lens capability check failed';
    }
  }

  async function ensureBackendSession() {
    if (backendSession?.session_id && backendSession.state !== 'closed') return backendSession;
    const res = await fetchWithRenewal('/api/browser/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ initial_url: currentUrl || '' }),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body.error || `backend session failed (${res.status})`);
    }
    backendSession = body;
    return backendSession;
  }

  async function navigateBackend(targetUrl) {
    const seq = ++backendNavigationSeq;
    loading = true;
    error = '';
    try {
      const session = await ensureBackendSession();
      const res = await fetchWithRenewal(`/api/browser/sessions/${encodeURIComponent(session.session_id)}/navigate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: targetUrl }),
      });
      const body = await res.json().catch(() => ({}));
      if (seq !== backendNavigationSeq) return;
      if (!res.ok) {
        backendSession = body.session_id ? body : backendSession;
        clearBackendSnapshots();
        error = body.error || `Web Lens snapshot failed (${res.status})`;
        loading = false;
        return;
      }
      backendSession = body;
      backendSnapshot = body.text_snapshot || '';
      backendHTML = body.html_snapshot || '';
      backendLinks = Array.isArray(body.links) ? body.links : [];
      backendScreenshotPNG = body.screenshot_png_base64 || '';
      controlStatus = '';
      error = '';
      loading = false;
    } catch (err) {
      if (seq !== backendNavigationSeq) return;
      if (handleAuthError(err)) return;
      clearBackendSnapshots();
      error = err.message || 'Web Lens snapshot failed';
      loading = false;
    }
  }

  async function applyBackendControl(action) {
    if (!backendSession?.session_id || backendSession.state === 'closed') return;
    const selector = controlSelector.trim();
    if (!selector) {
      error = 'Backend control selector is required';
      return;
    }
    const seq = ++backendNavigationSeq;
    loading = true;
    error = '';
    controlStatus = '';
    try {
      const res = await fetchWithRenewal(`/api/browser/sessions/${encodeURIComponent(backendSession.session_id)}/control`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action, selector, value: controlValue }),
      });
      const body = await res.json().catch(() => ({}));
      if (seq !== backendNavigationSeq) return;
      const nextSession = body.session || body;
      if (nextSession?.session_id) {
        backendSession = nextSession;
        backendScreenshotPNG = nextSession.screenshot_png_base64 || backendScreenshotPNG;
      }
      const control = body.control || {};
      controlStatus = control.error || control.value || control.document_text || control.text || '';
      if (!res.ok || control.ok === false) {
        error = control.error || body.error || `Web Lens control failed (${res.status})`;
        loading = false;
        return;
      }
      loading = false;
    } catch (err) {
      if (seq !== backendNavigationSeq) return;
      if (handleAuthError(err)) return;
      error = err.message || 'Web Lens control failed';
      loading = false;
    }
  }

  async function closeBackendSession() {
    if (!backendSession?.session_id || backendSession.state === 'closed') return;
    const seq = ++backendNavigationSeq;
    loading = true;
    error = '';
    try {
      const res = await fetchWithRenewal(`/api/browser/sessions/${encodeURIComponent(backendSession.session_id)}/close`, {
        method: 'POST',
      });
      const body = await res.json().catch(() => ({}));
      if (seq !== backendNavigationSeq) return;
      if (!res.ok) {
        error = body.error || `Web Lens close failed (${res.status})`;
        loading = false;
        return;
      }
      backendSession = body;
      currentUrl = '';
      clearBackendSnapshots();
      loading = false;
    } catch (err) {
      if (seq !== backendNavigationSeq) return;
      if (handleAuthError(err)) return;
      error = err.message || 'Web Lens close failed';
      loading = false;
    }
  }

  function importSnapshotToVText() {
    if (!backendSession?.session_id || !backendSnapshot) return;
    const lines = [
      `# Web Lens import`,
      ``,
      `Source: ${currentUrl || backendSession.current_url || 'unknown'}`,
      `Session: ${backendSession.session_id}`,
      ``,
      `## Snapshot`,
      ``,
      backendSnapshot,
    ];
    if (backendLinks.length) {
      lines.push('', '## Links', '');
      for (const link of backendLinks.slice(0, 30)) {
        lines.push(`- ${link.text || link.url}: ${link.url}`);
      }
    }
    dispatch('openvtext', {
      title: backendSession.title || 'Web Lens Import',
      initialContent: lines.join('\n'),
      seedPrompt: `Import Web Lens snapshot for ${currentUrl || backendSession.current_url || 'web page'}`,
      sourceUrl: currentUrl || backendSession.current_url || '',
      sourceContentId: backendSession.session_id,
      appHint: 'web_lens',
      createdFrom: 'web_lens',
      toastMessage: 'Opened Web Lens snapshot in VText',
    });
  }

  // Monitor for iframe load timeout (sites that block may not fire error event)
  let loadTimeout = null;

  $: if (loading && currentUrl) {
    if (loadTimeout) clearTimeout(loadTimeout);
    loadTimeout = setTimeout(() => {
      // If still loading after 15 seconds, show a message
      if (loading) {
        error = 'This page is taking too long to embed. If the site blocks iframes, use Web Lens snapshots instead of treating this as a full browser.';
        loading = false;
      }
    }, 15000);
  }

  // ---- Lifecycle ----

  onMount(() => {
    mounted = true;
    if (authenticated) {
      loadBrowserCapabilities();
    } else {
      enterGuestMode();
    }
  });

  $: if (mounted && authenticated && !capabilityLoadStarted) {
    loadBrowserCapabilities();
  }

  $: if (mounted && !authenticated && browserCapabilities?.mode !== 'guest_iframe') {
    capabilityLoadStarted = false;
    enterGuestMode();
  }
</script>

<div class="browser-app" data-browser-app>
  <!-- URL bar -->
  <div class="url-bar" data-browser-url-bar>
    <button
      class="nav-btn"
      data-browser-nav-back
      on:click={goBack}
      disabled={!canGoBack}
      title="Back"
      aria-label="Go back"
    >
      ←
    </button>
    <button
      class="nav-btn"
      data-browser-nav-forward
      on:click={goForward}
      disabled={!canGoForward}
      title="Forward"
      aria-label="Go forward"
    >
      →
    </button>
    <button
      class="nav-btn"
      data-browser-nav-reload
      on:click={reload}
      disabled={!currentUrl}
      title="Reload"
      aria-label="Reload page"
    >
      ↻
    </button>
    <input
      type="text"
      class="url-input"
      data-browser-url-input
      bind:value={urlInput}
      on:keydown={handleUrlKeydown}
      placeholder="Enter URL..."
      aria-label="URL input"
    />
    <button
      class="go-btn"
      data-browser-go-btn
      on:click={handleGo}
      title="Go"
      aria-label="Navigate to URL"
    >
      Go
    </button>
  </div>

  <div
    class="backend-status"
    data-browser-backend-status
    data-browser-backend-mode={browserCapabilities?.mode || 'unknown'}
    data-browser-backend-substrate={browserCapabilities?.substrate || 'unknown'}
    data-browser-backend-available={browserCapabilities?.available ? 'true' : 'false'}
    data-browser-supports-text={browserCapabilities?.supports?.text ? 'true' : 'false'}
    data-browser-supports-html={browserCapabilities?.supports?.html ? 'true' : 'false'}
    data-browser-supports-links={browserCapabilities?.supports?.links ? 'true' : 'false'}
    data-browser-supports-screenshot={browserCapabilities?.supports?.screenshot ? 'true' : 'false'}
    data-browser-supports-cdp-screenshot={browserCapabilities?.supports?.cdp_screenshot ? 'true' : 'false'}
    data-browser-supports-bounded-input={browserCapabilities?.supports?.bounded_input ? 'true' : 'false'}
    data-browser-supports-input={browserCapabilities?.supports?.input ? 'true' : 'false'}
    data-browser-supports-cdp={browserCapabilities?.supports?.cdp ? 'true' : 'false'}
    data-browser-session-id={backendSession?.session_id || ''}
    data-browser-execution-scope={backendSession?.execution_scope || ''}
    data-browser-backend-session-id={backendSession?.backend_session_id || ''}
    data-browser-world-kind={backendSession?.world_kind || ''}
    data-browser-vm-id={backendSession?.vm_id || ''}
    data-browser-snapshot-id={backendSession?.snapshot_id || ''}
    data-browser-source-loop-id={backendSession?.source_loop_id || ''}
    data-browser-candidate-trace-id={backendSession?.candidate_trace_id || ''}
    data-browser-session-state={backendSession?.state || ''}
  >
    <span>
      {#if browserCapabilities?.available}
        {#if backendSession?.state === 'closed'}
          Web Lens snapshot closed: {browserCapabilities.provider}
        {:else}
          Web Lens snapshot ready: {browserCapabilities.provider}
        {/if}
      {:else if capabilityError}
        {capabilityError}
      {:else}
        Iframe preview mode - Web Lens backend not configured
      {/if}
    </span>
    {#if browserCapabilities?.available && backendSession?.session_id && backendSession.state !== 'closed'}
      <button
        class="backend-close"
        data-browser-close-session
        on:click={closeBackendSession}
        disabled={loading}
        title="Close Web Lens session"
        aria-label="Close Web Lens session"
      >
        Close
      </button>
    {/if}
  </div>

  {#if browserCapabilities?.supports?.bounded_input && backendSession?.session_id && backendSession.state !== 'closed'}
    <div
      class="backend-control"
      data-browser-backend-control
      data-browser-control-status={controlStatus}
    >
      <input
        class="control-selector"
        data-browser-control-selector
        bind:value={controlSelector}
        placeholder="CSS selector"
        aria-label="Backend control selector"
      />
      <input
        class="control-value"
        data-browser-control-value
        bind:value={controlValue}
        placeholder="Value"
        aria-label="Backend control value"
      />
      <button
        class="control-btn"
        data-browser-control-fill
        on:click={() => applyBackendControl('fill')}
        disabled={loading}
      >
        Fill
      </button>
      <button
        class="control-btn"
        data-browser-control-click
        on:click={() => applyBackendControl('click')}
        disabled={loading}
      >
        Click
      </button>
      {#if controlStatus}
        <span class="control-status" data-browser-control-status-text>{controlStatus}</span>
      {/if}
    </div>
  {/if}

  <!-- Loading indicator -->
  {#if loading}
    <div class="loading-bar" data-browser-loading>
      <div class="loading-progress"></div>
    </div>
  {/if}

  <!-- Error message -->
  {#if error}
    <div class="error-message" data-browser-error role="alert">
      <span class="error-icon">⚠️</span>
      <span class="error-text">{error}</span>
      <button
        class="error-dismiss"
        on:click={() => { error = ''; }}
        title="Dismiss"
        aria-label="Dismiss error"
      >
        ✕
      </button>
    </div>
  {/if}

  <!-- iframe -->
  {#if currentUrl}
    {#if browserCapabilities?.available}
      <div class="backend-snapshot" data-browser-backend-snapshot>
        {#if backendSnapshot}
          <div class="backend-snapshot-layout">
            <div class="backend-main">
              <div class="snapshot-actions">
                <span>Semantic snapshot</span>
                <button
                  class="import-btn"
                  data-browser-import-vtext
                  on:click={importSnapshotToVText}
                  disabled={!backendSnapshot}
                >
                  Open in VText
                </button>
              </div>
              {#if backendScreenshotPNG}
                <figure
                  class="backend-screenshot"
                  data-browser-backend-screenshot
                  data-browser-backend-screenshot-bytes={Math.floor((backendScreenshotPNG.length * 3) / 4)}
                >
                  <img
                    src={`data:image/png;base64,${backendScreenshotPNG}`}
                    alt="Web Lens visual proof"
                  />
                </figure>
              {/if}
              <pre>{backendSnapshot}</pre>
            </div>
            {#if backendLinks.length || backendHTML}
              <aside
                class="backend-links"
                data-browser-backend-links
                data-browser-backend-links-count={backendLinks.length}
              >
                {#if backendLinks.length}
                  <h3>Links</h3>
                  {#each backendLinks as link}
                    <a
                      href={link.url}
                      target="_blank"
                      rel="noreferrer"
                      data-browser-backend-link
                    >
                      <span>{link.text || link.url}</span>
                      <small>{link.url}</small>
                    </a>
                  {/each}
                {/if}
                {#if backendHTML}
                  <details class="backend-html" data-browser-backend-html>
                    <summary>HTML source</summary>
                    <pre>{backendHTML}</pre>
                  </details>
                {/if}
              </aside>
            {/if}
          </div>
        {:else}
          <span>Loading Web Lens snapshot...</span>
        {/if}
      </div>
    {:else}
      <div class="iframe-container">
      <!-- svelte-ignore a11y-missing-attribute -->
        <iframe
          class="browser-iframe"
          data-browser-iframe
          bind:this={iframeEl}
          src={currentUrl}
          on:load={handleIframeLoad}
          on:error={handleIframeError}
          title="Browser content"
          sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox"
          allow="accelerometer; camera; encrypted-media; geolocation; gyroscope; microphone"
        ></iframe>
      </div>
    {/if}
  {:else}
    <div class="empty-state">
      <span class="empty-icon">🌐</span>
      <span>Enter a URL to start browsing</span>
    </div>
  {/if}
</div>

<style>
  .browser-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    background: #1a1a2a;
    color: #c0c0d0;
  }

  /* ---- URL bar ---- */
  .url-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 8px;
    background: #181825;
    border-bottom: 1px solid #2a2a3a;
    flex-shrink: 0;
  }

  .nav-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid #333;
    border-radius: 4px;
    color: #c0c0d0;
    cursor: pointer;
    font-size: 1rem;
    transition: background 0.15s;
    flex-shrink: 0;
  }

  .nav-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.08);
  }

  .nav-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .url-input {
    flex: 1;
    padding: 6px 10px;
    background: #11111b;
    border: 1px solid #333;
    border-radius: 4px;
    color: #e0e0e0;
    font-size: 0.85rem;
    min-width: 0;
  }

  .url-input:focus {
    outline: none;
    border-color: #3b82f6;
  }

  .go-btn {
    padding: 6px 14px;
    background: rgba(59, 130, 246, 0.15);
    border: 1px solid rgba(59, 130, 246, 0.3);
    border-radius: 4px;
    color: #7eb8ff;
    cursor: pointer;
    font-size: 0.8rem;
    white-space: nowrap;
    transition: background 0.15s;
    flex-shrink: 0;
  }

  .go-btn:hover {
    background: rgba(59, 130, 246, 0.25);
  }

  .backend-status {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    flex-shrink: 0;
    border-bottom: 1px solid #252538;
    padding: 5px 10px;
    color: #93a4bd;
    background: #141421;
    font-size: 0.74rem;
  }

  .backend-close {
    border: 1px solid rgba(148, 163, 184, 0.22);
    border-radius: 4px;
    background: rgba(15, 23, 42, 0.62);
    color: #cbd5e1;
    cursor: pointer;
    font-size: 0.72rem;
    padding: 3px 8px;
  }

  .backend-close:hover:not(:disabled) {
    background: rgba(30, 41, 59, 0.84);
  }

  .backend-close:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .backend-control {
    display: grid;
    grid-template-columns: minmax(90px, 1fr) minmax(80px, 1fr) auto auto minmax(0, 1.2fr);
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
    border-bottom: 1px solid #252538;
    padding: 6px 8px;
    background: #111827;
  }

  .control-selector,
  .control-value {
    min-width: 0;
    border: 1px solid #334155;
    border-radius: 4px;
    background: #0f172a;
    color: #e2e8f0;
    font-size: 0.74rem;
    padding: 5px 7px;
  }

  .control-btn {
    border: 1px solid rgba(59, 130, 246, 0.34);
    border-radius: 4px;
    background: rgba(30, 64, 175, 0.24);
    color: #bfdbfe;
    cursor: pointer;
    font-size: 0.72rem;
    padding: 5px 8px;
  }

  .control-btn:hover:not(:disabled) {
    background: rgba(37, 99, 235, 0.34);
  }

  .control-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .control-status {
    min-width: 0;
    overflow: hidden;
    color: #a7f3d0;
    font-size: 0.72rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ---- Loading bar ---- */
  .loading-bar {
    height: 2px;
    background: #1a1a2a;
    flex-shrink: 0;
    overflow: hidden;
  }

  .loading-progress {
    height: 100%;
    width: 30%;
    background: #3b82f6;
    animation: loading-slide 1.5s ease-in-out infinite;
  }

  @keyframes loading-slide {
    0% { transform: translateX(-100%); }
    50% { transform: translateX(200%); }
    100% { transform: translateX(-100%); }
  }

  /* ---- Error message ---- */
  .error-message {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: rgba(239, 68, 68, 0.1);
    border-bottom: 1px solid rgba(239, 68, 68, 0.2);
    color: #fca5a5;
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .error-icon {
    font-size: 1rem;
    flex-shrink: 0;
  }

  .error-text {
    flex: 1;
    min-width: 0;
  }

  .error-dismiss {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: #f87171;
    cursor: pointer;
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .error-dismiss:hover {
    background: rgba(239, 68, 68, 0.2);
  }

  /* ---- iframe container ---- */
  .iframe-container {
    flex: 1;
    overflow: hidden;
    position: relative;
  }

  .browser-iframe {
    width: 100%;
    height: 100%;
    border: none;
    background: #fff;
    display: block;
  }

  .backend-snapshot {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 18px;
    background: #f6f7fb;
    color: #151821;
  }

  .backend-snapshot pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    font: 0.9rem/1.5 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .backend-snapshot-layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(180px, 240px);
    gap: 18px;
    min-height: 100%;
  }

  .backend-main {
    min-width: 0;
  }

  .snapshot-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 10px;
    color: #475569;
    font-size: 0.78rem;
    font-weight: 700;
  }

  .import-btn {
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    background: #ffffff;
    color: #1d4ed8;
    cursor: pointer;
    font-size: 0.74rem;
    padding: 5px 9px;
  }

  .import-btn:hover:not(:disabled) {
    border-color: #93b4f6;
    background: #f8fbff;
  }

  .import-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .backend-screenshot {
    margin: 0 0 14px;
    overflow: hidden;
    border: 1px solid #d9deea;
    border-radius: 4px;
    background: #ffffff;
  }

  .backend-screenshot img {
    display: block;
    width: 100%;
    height: auto;
  }

  .backend-links {
    display: flex;
    flex-direction: column;
    gap: 8px;
    border-left: 1px solid #d7dce8;
    padding-left: 14px;
  }

  .backend-links h3 {
    margin: 0 0 4px;
    color: #3d4657;
    font-size: 0.78rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .backend-links a {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 8px;
    border: 1px solid #d9deea;
    border-radius: 4px;
    color: #1d4ed8;
    background: #ffffff;
    text-decoration: none;
  }

  .backend-links a:hover {
    border-color: #93b4f6;
    background: #f8fbff;
  }

  .backend-links small {
    overflow-wrap: anywhere;
    color: #64748b;
    font-size: 0.7rem;
  }

  .backend-html {
    margin-top: 8px;
    border-top: 1px solid #d7dce8;
    padding-top: 10px;
  }

  .backend-html summary {
    cursor: pointer;
    color: #334155;
    font-size: 0.76rem;
    font-weight: 700;
  }

  .backend-html pre {
    margin-top: 8px;
    max-height: 260px;
    overflow: auto;
    border: 1px solid #d9deea;
    border-radius: 4px;
    background: #fff;
    padding: 8px;
    color: #1f2937;
    font: 0.72rem/1.45 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  /* ---- Empty state ---- */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    flex: 1;
    color: #666;
    font-size: 0.9rem;
  }

  .empty-icon {
    font-size: 3rem;
    opacity: 0.4;
  }

  /* ---- Mobile responsive ---- */
  @media (max-width: 768px) {
    .url-bar {
      padding: 6px 6px;
      gap: 3px;
    }

    .nav-btn {
      width: 36px;
      height: 36px;
      min-width: 36px;
    }

    .url-input {
      font-size: 16px; /* Prevent iOS zoom */
      padding: 8px 8px;
    }

    .go-btn {
      padding: 8px 10px;
      min-height: 36px;
    }

    .backend-snapshot-layout {
      grid-template-columns: 1fr;
    }

    .backend-links {
      border-left: none;
      border-top: 1px solid #d7dce8;
      padding-left: 0;
      padding-top: 12px;
    }
  }
</style>
