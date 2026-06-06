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
  import {
    renderInlineMarkdown,
    sourceEntitySnapshotWarnings,
    sourceEntitySnapshotText,
    sourceEntityTitle,
  } from './vtext-source-renderer.ts';

  export let appContext = {};
  export let authenticated = false;
  const dispatch = createEventDispatcher();

  // ---- State ----
  const initialTarget = appContext?.initialUrl || appContext?.sourceUrl || '';
  const sourceEntity = appContext?.sourceEntity || null;
  const initialSourceSnapshot = sourceEntitySnapshotText(sourceEntity);
  const initialSourceSnapshotWarnings = sourceEntitySnapshotWarnings(sourceEntity);
  let urlInput = initialTarget || 'https://en.wikipedia.org';
  let currentUrl = initialTarget ? normalizeUrl(initialTarget) : '';
  let loading = false;
  let error = '';
  let iframeEl = null;
  let browserCapabilities = null;
  let capabilityError = '';
  let backendSession = null;
  let backendSnapshot = initialSourceSnapshot;
  let snapshotMode = initialSourceSnapshot ? 'source_entity' : '';
  let backendHTML = '';
  let backendLinks = [];
  let backendScreenshotPNG = '';
  let backendWarnings = initialSourceSnapshotWarnings;
  let showingSnapshot = !!initialSourceSnapshot;
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
  $: readableSnapshotHTML = renderSnapshotMarkdown(backendSnapshot);

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
    clearBackendSnapshots();
    if (browserCapabilities?.available) {
      showingSnapshot = true;
      navigateBackend(normalized);
    } else {
      showingSnapshot = false;
    }

    if (addToHistory) {
      // Trim forward history
      history = history.slice(0, historyIndex + 1);
      history.push(normalized);
      historyIndex = history.length - 1;
    }

    window.clearTimeout(loadTimeout);
  }

  function clearBackendSnapshots() {
    backendSnapshot = '';
    snapshotMode = '';
    backendHTML = '';
    backendLinks = [];
    backendScreenshotPNG = '';
    backendWarnings = [];
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
      showingSnapshot = false;
      clearBackendSnapshots();
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
      showingSnapshot = false;
      clearBackendSnapshots();
    }
  }

  function reload() {
    if (currentUrl) {
      loading = true;
      error = '';
      const url = currentUrl;
      if (showingSnapshot && browserCapabilities?.available) {
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
    error = 'This site may block embedding. Try a Web Lens snapshot for readable text, links, source, and import.';
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
    if (initialSourceSnapshot) {
      backendSnapshot = initialSourceSnapshot;
      snapshotMode = 'source_entity';
      backendHTML = '';
      backendLinks = [];
      backendScreenshotPNG = '';
      backendWarnings = initialSourceSnapshotWarnings;
      showingSnapshot = true;
    } else {
      showingSnapshot = false;
      clearBackendSnapshots();
    }
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
        if (currentUrl && !backendSnapshot) {
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
    showingSnapshot = true;
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
      snapshotMode = 'backend';
      backendHTML = body.html_snapshot || '';
      backendLinks = Array.isArray(body.links) ? body.links : [];
      backendScreenshotPNG = body.screenshot_png_base64 || '';
      backendWarnings = Array.isArray(body.snapshot_warnings) ? body.snapshot_warnings.filter(Boolean) : [];
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
      showingSnapshot = false;
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
    if (!backendSnapshot) return;
    const sessionID = backendSession?.session_id || 'source-entity-snapshot';
    const lines = [
      `# ${snapshotMode === 'source_entity' ? 'Source reader import' : 'Web Lens import'}`,
      ``,
      `Source: ${currentUrl || backendSession?.current_url || 'unknown'}`,
      `Session: ${sessionID}`,
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
      title: backendSession?.title || sourceEntityTitle(sourceEntity) || 'Web Lens Import',
      initialContent: lines.join('\n'),
      seedPrompt: `Import Web Lens snapshot for ${currentUrl || backendSession?.current_url || 'web page'}`,
      sourceUrl: currentUrl || backendSession?.current_url || '',
      sourceContentId: sessionID,
      appHint: 'web_lens',
      createdFrom: 'web_lens',
      toastMessage: 'Opened Web Lens snapshot in VText',
    });
  }

  function showPagePreview() {
    showingSnapshot = false;
    error = '';
    loading = Boolean(currentUrl);
  }

  function showSnapshot() {
    if (!currentUrl) return;
    if (browserCapabilities?.available) {
      navigateBackend(currentUrl);
      return;
    }
    if (initialSourceSnapshot) {
      backendSnapshot = initialSourceSnapshot;
      snapshotMode = 'source_entity';
      backendWarnings = initialSourceSnapshotWarnings;
      showingSnapshot = true;
    }
  }

  function renderSnapshotMarkdown(value) {
    const raw = String(value || '').trim();
    if (!raw) return '';
    const lines = raw.split(/\r?\n/);
    const blocks = [];
    let paragraph = [];
    const flushParagraph = () => {
      const text = paragraph.join(' ').trim();
      if (text) blocks.push(`<p>${renderInlineMarkdown(text, [])}</p>`);
      paragraph = [];
    };
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed) {
        flushParagraph();
        continue;
      }
      const heading = /^(#{1,4})\s+(.+)$/.exec(trimmed);
      if (heading) {
        flushParagraph();
        const level = Math.min(4, heading[1].length + 1);
        blocks.push(`<h${level}>${renderInlineMarkdown(heading[2], [])}</h${level}>`);
        continue;
      }
      paragraph.push(trimmed);
    }
    flushParagraph();
    return blocks.join('\n');
  }

  // Monitor for iframe load timeout (sites that block may not fire error event)
  let loadTimeout = null;

  $: if (loading && currentUrl && !showingSnapshot) {
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
      {#if showingSnapshot && snapshotMode === 'source_entity' && backendSnapshot}
        Source reader snapshot
      {:else if browserCapabilities?.available}
        {#if backendSession?.state === 'closed'}
          Web Lens snapshot closed: {browserCapabilities.provider}
        {:else if showingSnapshot && loading}
          Web Lens snapshot loading: {browserCapabilities.provider}
        {:else if showingSnapshot && backendSnapshot && backendWarnings.length}
          Web Lens snapshot partial: {browserCapabilities.provider}
        {:else if showingSnapshot && backendSnapshot}
          Web Lens snapshot ready: {browserCapabilities.provider}
        {:else if showingSnapshot}
          Web Lens snapshot waiting: {browserCapabilities.provider}
        {:else}
          Page preview mode
        {/if}
      {:else if capabilityError}
        {capabilityError}
      {:else}
        Iframe preview mode - Web Lens backend not configured
      {/if}
    </span>
    {#if currentUrl && !showingSnapshot && (browserCapabilities?.available || initialSourceSnapshot)}
      <button
        class="backend-close"
        data-browser-open-snapshot
        on:click={showSnapshot}
        disabled={loading}
        title="Open readable Web Lens snapshot"
        aria-label="Open readable Web Lens snapshot"
      >
        Snapshot
      </button>
    {/if}
    {#if showingSnapshot}
      <button
        class="backend-close"
        data-browser-show-page-preview
        on:click={showPagePreview}
        disabled={loading}
        title="Return to page preview"
        aria-label="Return to page preview"
      >
        Page
      </button>
    {/if}
    {#if browserCapabilities?.available && showingSnapshot && backendSession?.session_id && backendSession.state !== 'closed'}
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
    {#if showingSnapshot && backendSnapshot}
      <div class="backend-snapshot" data-browser-backend-snapshot>
        {#if backendSnapshot}
          <div class="backend-snapshot-layout">
            <div class="backend-main">
              <div class="snapshot-actions">
                <span>{snapshotMode === 'source_entity' ? 'Source reader snapshot' : backendWarnings.length ? 'Semantic snapshot with warnings' : 'Semantic snapshot'}</span>
                <button
                  class="import-btn"
                  data-browser-import-vtext
                  on:click={importSnapshotToVText}
                  disabled={!backendSnapshot}
                >
                  Open in VText
                </button>
              </div>
              {#if backendWarnings.length}
                <div class="snapshot-warnings" data-browser-snapshot-warnings>
                  {#each backendWarnings as warning}
                    <span>{warning}</span>
                  {/each}
                </div>
              {/if}
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
              <article class="snapshot-reader" data-browser-reader-markdown>
                {@html readableSnapshotHTML}
              </article>
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
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
  }

  /* ---- URL bar ---- */
  .url-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 8px;
    background: var(--choir-state-selected);
    border-bottom: 1px solid var(--choir-border-strong);
    flex-shrink: 0;
  }

  .nav-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--choir-border);
    border-radius: 4px;
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 1rem;
    transition: background 0.15s;
    flex-shrink: 0;
  }

  .nav-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--choir-text-primary) 8%, transparent);
  }

  .nav-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .url-input {
    flex: 1;
    padding: 6px 10px;
    background: var(--choir-state-selected);
    border: 1px solid var(--choir-border);
    border-radius: 4px;
    color: var(--choir-text-primary);
    font-size: 0.85rem;
    min-width: 0;
  }

  .url-input:focus {
    outline: none;
    border-color: var(--choir-border-strong);
  }

  .go-btn {
    padding: 6px 14px;
    background: var(--choir-state-hover);
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.8rem;
    white-space: nowrap;
    transition: background 0.15s;
    flex-shrink: 0;
  }

  .go-btn:hover {
    background: var(--choir-state-selected);
  }

  .backend-status {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    flex-shrink: 0;
    border-bottom: 1px solid var(--choir-border-strong);
    padding: 5px 10px;
    color: var(--choir-text-accent);
    background: var(--choir-state-selected);
    font-size: 0.74rem;
  }

  .backend-close {
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.72rem;
    padding: 3px 8px;
  }

  .backend-close:hover:not(:disabled) {
    background: var(--choir-state-selected);
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
    border-bottom: 1px solid var(--choir-border-strong);
    padding: 6px 8px;
    background: var(--choir-state-selected);
  }

  .control-selector,
  .control-value {
    min-width: 0;
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    font-size: 0.74rem;
    padding: 5px 7px;
  }

  .control-btn {
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.72rem;
    padding: 5px 8px;
  }

  .control-btn:hover:not(:disabled) {
    background: var(--choir-state-selected);
  }

  .control-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .control-status {
    min-width: 0;
    overflow: hidden;
    color: var(--choir-status-success);
    font-size: 0.72rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ---- Loading bar ---- */
  .loading-bar {
    height: 2px;
    background: var(--choir-state-selected);
    flex-shrink: 0;
    overflow: hidden;
  }

  .loading-progress {
    height: 100%;
    width: 30%;
    background: var(--choir-state-selected);
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
    background: var(--choir-status-danger-soft);
    border-bottom: 1px solid var(--choir-status-danger);
    color: var(--choir-status-danger);
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
    color: var(--choir-status-danger);
    cursor: pointer;
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .error-dismiss:hover {
    background: var(--choir-status-danger-soft);
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
    background: var(--choir-surface-document);
    display: block;
  }

  .backend-snapshot {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 18px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
  }

  .backend-snapshot pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    font: 0.9rem/1.5 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  .snapshot-reader {
    max-width: 70ch;
    color: var(--choir-text-primary);
    font: 0.95rem/1.62 ui-serif, Georgia, Cambria, "Times New Roman", Times, serif;
  }

  .snapshot-reader :global(p) {
    margin: 0 0 0.95rem;
  }

  .snapshot-reader :global(h2),
  .snapshot-reader :global(h3),
  .snapshot-reader :global(h4),
  .snapshot-reader :global(h5) {
    margin: 1.15rem 0 0.55rem;
    color: var(--choir-text-accent);
    font-family: inherit;
    line-height: 1.2;
  }

  .snapshot-reader :global(a) {
    color: var(--choir-text-accent);
    text-decoration-thickness: 1px;
    text-underline-offset: 0.16em;
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
    color: var(--choir-text-accent);
    font-size: 0.78rem;
    font-weight: 700;
  }

  .snapshot-warnings {
    display: grid;
    gap: 4px;
    margin: 0 0 10px;
    border: 1px solid var(--choir-status-warning);
    border-radius: 4px;
    padding: 8px;
    background: color-mix(in srgb, var(--choir-status-warning) 12%, transparent);
    color: var(--choir-status-warning);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .import-btn {
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-surface-document);
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.74rem;
    padding: 5px 9px;
  }

  .import-btn:hover:not(:disabled) {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .import-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .backend-screenshot {
    margin: 0 0 14px;
    overflow: hidden;
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-surface-document);
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
    border-left: 1px solid var(--choir-border-strong);
    padding-left: 14px;
  }

  .backend-links h3 {
    margin: 0 0 4px;
    color: var(--choir-text-accent);
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
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    color: var(--choir-text-accent);
    background: var(--choir-surface-document);
    text-decoration: none;
  }

  .backend-links a:hover {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .backend-links small {
    overflow-wrap: anywhere;
    color: var(--choir-text-accent);
    font-size: 0.7rem;
  }

  .backend-html {
    margin-top: 8px;
    border-top: 1px solid var(--choir-border-strong);
    padding-top: 10px;
  }

  .backend-html summary {
    cursor: pointer;
    color: var(--choir-text-accent);
    font-size: 0.76rem;
    font-weight: 700;
  }

  .backend-html pre {
    margin-top: 8px;
    max-height: 260px;
    overflow: auto;
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-surface-document);
    padding: 8px;
    color: var(--choir-text-accent);
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
    color: var(--choir-text-subtle);
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
      border-top: 1px solid var(--choir-border-strong);
      padding-left: 0;
      padding-top: 12px;
    }
  }
</style>
