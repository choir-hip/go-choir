<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import { BUILD_INFO } from './build-info.js';
  import { fetchThemePreference } from './preferences.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import {
    DEFAULT_THEME,
    THEME_PRESETS,
    normalizeThemeConfig,
    themeCSSVariables,
    validateThemeConfig,
  } from './theme';

  export let currentUser = null;
  export let currentTheme = DEFAULT_THEME;

  const dispatch = createEventDispatcher();

  let loading = true;
  let error = '';
  let health = null;
  let selectedTheme = normalizeThemeConfig(currentTheme);
  let themeJSON = JSON.stringify(DEFAULT_THEME, null, 2);
  let themeError = '';
  let themeNotice = '';
  let lastAppliedThemeJSON = JSON.stringify(DEFAULT_THEME, null, 2);
  let themeDraftJSON = '';

  // API key management state
  let apiKeys = [];
  let apiKeyLoading = false;
  let apiKeyError = '';
  let apiKeyNotice = '';
  let newKeyLabel = '';
  let newKeyScopes = ['read:texture', 'read:base', 'read:runtime'];
  let newKeySecret = '';
  let showNewKeySecret = false;
  let copiedKey = false;

  const allScopes = [
    { value: 'read:texture', label: 'Read Texture' },
    { value: 'write:texture', label: 'Write Texture' },
    { value: 'read:base', label: 'Read Base' },
    { value: 'write:base', label: 'Write Base' },
    { value: 'read:runtime', label: 'Read Runtime' },
    { value: 'write:runtime', label: 'Write Runtime' },
    { value: 'acceptance:read', label: 'Read Acceptance Evidence' },
    { value: 'manage:keys', label: 'Manage API Keys' },
    { value: 'admin', label: 'Admin' },
  ];

  async function loadAPIKeys() {
    apiKeyLoading = true;
    apiKeyError = '';
    try {
      const res = await fetchWithRenewal('/auth/api-keys', { method: 'GET' });
      if (!res.ok) throw new Error(`Failed to load API keys (${res.status})`);
      const data = await res.json();
      apiKeys = Array.isArray(data) ? data : (data.keys || []);
    } catch (err) {
      if (err instanceof AuthRequiredError) { dispatch('authexpired'); return; }
      apiKeyError = err.message || 'Failed to load API keys';
    } finally {
      apiKeyLoading = false;
    }
  }

  async function createAPIKey() {
    apiKeyError = '';
    apiKeyNotice = '';
    const label = newKeyLabel.trim() || 'CLI key';
    try {
      const res = await fetchWithRenewal('/auth/api-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label, scopes: newKeyScopes }),
      });
      if (!res.ok) {
        const body = await res.text();
        throw new Error(`Failed to create API key (${res.status}): ${body}`);
      }
      const data = await res.json();
      newKeySecret = data.secret || '';
      showNewKeySecret = true;
      newKeyLabel = '';
      apiKeyNotice = 'API key created. Copy the secret now — it won\'t be shown again.';
      void loadAPIKeys();
    } catch (err) {
      if (err instanceof AuthRequiredError) { dispatch('authexpired'); return; }
      apiKeyError = err.message || 'Failed to create API key';
    }
  }

  async function revokeAPIKey(keyId) {
    apiKeyError = '';
    apiKeyNotice = '';
    try {
      const res = await fetchWithRenewal(`/auth/api-keys/${keyId}`, { method: 'DELETE' });
      if (!res.ok) throw new Error(`Failed to revoke API key (${res.status})`);
      apiKeyNotice = 'API key revoked.';
      void loadAPIKeys();
    } catch (err) {
      if (err instanceof AuthRequiredError) { dispatch('authexpired'); return; }
      apiKeyError = err.message || 'Failed to revoke API key';
    }
  }

  function toggleScope(scope) {
    if (newKeyScopes.includes(scope)) {
      newKeyScopes = newKeyScopes.filter(s => s !== scope);
    } else {
      newKeyScopes = [...newKeyScopes, scope];
    }
  }

  function copySecret() {
    if (newKeySecret) {
      navigator.clipboard.writeText(newKeySecret).then(() => {
        copiedKey = true;
        setTimeout(() => { copiedKey = false; }, 2000);
      });
    }
  }

  function dismissNewKeySecret() {
    showNewKeySecret = false;
    newKeySecret = '';
    apiKeyNotice = '';
  }

  function formatDate(dateStr) {
    if (!dateStr) return '—';
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
    } catch (_e) {
      return dateStr;
    }
  }

  $: themeValidation = validateThemeConfig(selectedTheme);
  $: themePreviewVars = themeCSSVariables(selectedTheme);

  async function refreshStatus() {
    loading = true;
    error = '';
    try {
      const res = await fetchWithRenewal('/health', { method: 'GET' });
      if (!res.ok) {
        throw new Error(`Runtime status failed (${res.status})`);
      }
      health = await res.json();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Runtime status unavailable';
    } finally {
      loading = false;
    }
  }

  function shortCommit(value) {
    if (!value) return 'unknown';
    return String(value).slice(0, 12);
  }

  function handleResetDesktop() {
    dispatch('resetdesktop');
  }

  function handleOpenComputeMonitor() {
    dispatch('opencomputemonitor');
  }

  $: if (!currentUser?.email && currentTheme?.id && currentTheme.id !== selectedTheme.id) {
    selectedTheme = normalizeThemeConfig(currentTheme);
    themeJSON = JSON.stringify(selectedTheme, null, 2);
  }

  async function loadStoredTheme() {
    try {
      const stored = await fetchThemePreference();
      const theme = stored && Object.keys(stored).length > 0 ? stored : DEFAULT_THEME;
      const validation = validateThemeConfig(theme);
      return validation.ok ? normalizeThemeConfig(theme) : DEFAULT_THEME;
    } catch (_err) {
      return DEFAULT_THEME;
    }
  }

  function applyTheme(theme) {
    window.dispatchEvent(new CustomEvent('choir-theme-change', { detail: { theme } }));
  }

  function selectTheme(theme) {
    selectedTheme = normalizeThemeConfig(theme);
    themeJSON = JSON.stringify(selectedTheme, null, 2);
    lastAppliedThemeJSON = themeJSON;
    themeDraftJSON = '';
    themeError = '';
    themeNotice = `${selectedTheme.name} applied`;
    applyTheme(selectedTheme);
  }

  function setPromptSurfacePlacement(placement) {
    const nextTheme = normalizeThemeConfig({
      ...selectedTheme,
      layout: {
        ...selectedTheme.layout,
        promptSurfacePlacement: placement,
      },
    });
    selectedTheme = nextTheme;
    themeJSON = JSON.stringify(nextTheme, null, 2);
    lastAppliedThemeJSON = themeJSON;
    themeDraftJSON = '';
    themeError = '';
    themeNotice = `Prompt surface moved to the ${placement}`;
    applyTheme(nextTheme);
  }

  function handleThemeJSONInput(event) {
    themeJSON = event.currentTarget.value;
    themeDraftJSON = themeJSON;
    themeNotice = '';
    themeError = '';
    try {
      const parsed = JSON.parse(themeJSON);
      const validation = validateThemeConfig(parsed);
      if (!validation.ok) {
        themeError = validation.errors.join(', ');
        return;
      }
      themeError = '';
    } catch (_err) {
      themeError = 'theme JSON is invalid';
    }
  }

  function applyThemeJSON() {
    try {
      const parsed = JSON.parse(themeJSON);
      const validation = validateThemeConfig(parsed);
      if (!validation.ok) {
        themeError = validation.errors.join(', ');
        return;
      }
      selectedTheme = normalizeThemeConfig(parsed);
      lastAppliedThemeJSON = JSON.stringify(selectedTheme, null, 2);
      themeJSON = lastAppliedThemeJSON;
      themeDraftJSON = '';
      themeError = '';
      themeNotice = `${selectedTheme.name} applied`;
      applyTheme(selectedTheme);
    } catch (_err) {
      themeError = 'theme JSON is invalid';
    }
  }

  function revertThemeJSON() {
    themeJSON = lastAppliedThemeJSON;
    themeDraftJSON = '';
    themeError = '';
    themeNotice = 'Draft reverted';
  }

  $: hasThemeDraft = themeDraftJSON.length > 0 && themeDraftJSON !== lastAppliedThemeJSON;

  let removeLiveListener = () => {};

  onMount(async () => {
    selectedTheme = currentUser?.email ? await loadStoredTheme() : normalizeThemeConfig(currentTheme);
    themeJSON = JSON.stringify(selectedTheme, null, 2);
    lastAppliedThemeJSON = themeJSON;
    void refreshStatus();
    if (currentUser?.email) void loadAPIKeys();
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
      if (kind === 'theme.updated') {
        void loadStoredTheme().then((theme) => {
          selectedTheme = theme;
          themeJSON = JSON.stringify(selectedTheme, null, 2);
          lastAppliedThemeJSON = themeJSON;
          themeDraftJSON = '';
        });
      }
      if (kind === 'runtime.health' || kind === 'runtime.degraded') {
        void refreshStatus();
      }
    });
  });

  onDestroy(() => {
    removeLiveListener();
  });
</script>

<section class="settings-app" data-settings-app>
  <div class="settings-hero">
    <p class="eyebrow">Settings</p>
    <h2>Desktop preferences</h2>
    <p>Product settings only. Runtime prompt policy and agent internals are not exposed here.</p>
  </div>

  <div class="settings-grid">
    <section class="settings-card" data-settings-account>
      <div>
        <h3>Account</h3>
        <p class="muted">{currentUser?.email ? 'Signed in as' : 'Logged-out review'}</p>
      </div>
      <strong class="account-email">{currentUser?.email || 'Public preview'}</strong>
    </section>

    {#if currentUser?.email}
      <section class="settings-card api-keys-card" data-settings-api-keys>
        <div>
          <h3>API keys</h3>
          <p class="muted">Create keys for CLI access and coding agents. The secret is shown once — copy it immediately.</p>
        </div>

        {#if showNewKeySecret}
          <div class="api-key-secret-banner" data-api-key-secret-banner>
            <div class="api-key-secret-row">
              <code class="api-key-secret" data-api-key-secret>{newKeySecret}</code>
              <button class="secondary-action" on:click={copySecret}>
                {copiedKey ? 'Copied!' : 'Copy'}
              </button>
              <button class="secondary-action" on:click={dismissNewKeySecret}>Done</button>
            </div>
            <p class="theme-notice">{apiKeyNotice}</p>
          </div>
        {:else}
          <div class="api-key-create-form" data-api-key-create-form>
            <input
              type="text"
              class="api-key-label-input"
              data-api-key-label-input
              placeholder="Key label (e.g. Devin CLI)"
              value={newKeyLabel}
              on:input={(e) => newKeyLabel = e.currentTarget.value}
            />
            <div class="api-key-scopes" data-api-key-scopes>
              {#each allScopes as scope}
                <label class="scope-chip" class:active={newKeyScopes.includes(scope.value)}>
                  <input
                    type="checkbox"
                    checked={newKeyScopes.includes(scope.value)}
                    on:change={() => toggleScope(scope.value)}
                  />
                  <span>{scope.label}</span>
                </label>
              {/each}
            </div>
            <button class="secondary-action" data-api-key-create-btn on:click={createAPIKey}>
              Create API key
            </button>
          </div>
        {/if}

        {#if apiKeyError}
          <p class="theme-error" data-api-key-error>{apiKeyError}</p>
        {/if}
        {#if apiKeyNotice && !showNewKeySecret}
          <p class="theme-notice" data-api-key-notice>{apiKeyNotice}</p>
        {/if}

        <div class="api-key-list" data-api-key-list>
          {#if apiKeyLoading}
            <p class="muted">Loading keys…</p>
          {:else if apiKeys.length === 0}
            <p class="muted">No API keys yet.</p>
          {:else}
            {#each apiKeys as key}
              <div class="api-key-row" class:revoked={key.revoked_at} data-api-key-row={key.id}>
                <div class="api-key-info">
                  <strong>{key.label}</strong>
                  <span class="api-key-meta">
                    {key.scopes?.join(', ') || 'no scopes'}
                    · created {formatDate(key.created_at)}
                    {#if key.last_used_at} · last used {formatDate(key.last_used_at)}{/if}
                    {#if key.revoked_at} · <strong>revoked</strong>{/if}
                  </span>
                </div>
                {#if !key.revoked_at}
                  <button class="secondary-action api-key-revoke-btn" on:click={() => revokeAPIKey(key.id)}>
                    Revoke
                  </button>
                {/if}
              </div>
            {/each}
          {/if}
        </div>
      </section>
    {/if}

    <section class="settings-card" data-settings-theme>
      <div>
        <h3>Theme</h3>
        <p class="muted">Validated theme config with editable presets.</p>
      </div>
      <div class="theme-swatch" style={Object.entries(themePreviewVars).map(([key, value]) => `${key}: ${value}`).join('; ')} aria-label="Current theme preview">
        <span></span>
        <span></span>
        <span></span>
      </div>
      <p class="theme-status" data-settings-theme-validation>
        {selectedTheme.name}: {themeValidation.ok ? 'valid config' : themeValidation.errors.join(', ')}
      </p>
      <div class="theme-presets" data-theme-presets>
        {#each THEME_PRESETS as theme}
          <button
            class:active={theme.id === selectedTheme.id}
            class="theme-preset"
            data-theme-preset={theme.id}
            on:click={() => selectTheme(theme)}
          >
            <span class="preset-dot" style={`background: ${theme.colors.accent}`}></span>
            <span>{theme.name}</span>
          </button>
        {/each}
      </div>
      <details class="theme-dev-panel">
        <summary>Advanced theme JSON</summary>
        <textarea
          id="theme-editor"
          class="theme-editor"
          data-theme-editor
          spellcheck="false"
          value={themeJSON}
          aria-invalid={themeError ? 'true' : 'false'}
          aria-describedby="theme-editor-help theme-editor-feedback"
          on:input={handleThemeJSONInput}
        ></textarea>
        <p id="theme-editor-help" class="theme-dev-help">Edit known theme values, then apply the draft. Unsafe or unknown CSS values are rejected.</p>
        <div class="theme-dev-actions">
          <button type="button" class="theme-apply-btn" data-theme-apply on:click={applyThemeJSON} disabled={!!themeError || !hasThemeDraft}>
            Apply draft
          </button>
          <button type="button" class="theme-revert-btn" data-theme-revert on:click={revertThemeJSON} disabled={!hasThemeDraft}>
            Revert
          </button>
        </div>
      </details>
      {#if themeError}
        <p id="theme-editor-feedback" class="theme-error" data-theme-error role="alert">{themeError}</p>
      {:else if themeNotice}
        <p id="theme-editor-feedback" class="theme-notice" data-theme-notice role="status">{themeNotice}</p>
      {/if}
    </section>

    <section class="settings-card" data-settings-desktop>
      <div>
        <h3>Desktop layout</h3>
        <p class="muted">Place the prompt surface for QA, or reset open windows and icon positions.</p>
      </div>
      <div class="layout-setting" data-settings-prompt-placement>
        <div>
          <strong>Prompt surface</strong>
          <span>{selectedTheme.layout.promptSurfacePlacement === 'top' ? 'Pinned to top' : 'Pinned to bottom'}</span>
        </div>
        <div class="segmented-control" role="group" aria-label="Prompt surface placement">
          <button
            type="button"
            class:active={selectedTheme.layout.promptSurfacePlacement !== 'top'}
            data-settings-prompt-placement-bottom
            aria-pressed={selectedTheme.layout.promptSurfacePlacement !== 'top'}
            on:click={() => setPromptSurfacePlacement('bottom')}
          >
            Bottom
          </button>
          <button
            type="button"
            class:active={selectedTheme.layout.promptSurfacePlacement === 'top'}
            data-settings-prompt-placement-top
            aria-pressed={selectedTheme.layout.promptSurfacePlacement === 'top'}
            on:click={() => setPromptSurfacePlacement('top')}
          >
            Top
          </button>
        </div>
      </div>
      <div class="settings-actions">
        <button class="secondary-action" data-settings-open-compute-monitor on:click={handleOpenComputeMonitor}>
          Open Compute Monitor
        </button>
        <button class="secondary-action" data-settings-reset-desktop on:click={handleResetDesktop}>
          Reset layout
        </button>
      </div>
    </section>

    <section class="settings-card status-card" data-settings-runtime-status>
      <div class="status-header">
        <div>
          <h3>Runtime status</h3>
          <p class="muted">Read-only deploy and backend identity.</p>
        </div>
      </div>

      {#if error}
        <div class="settings-error" role="alert">{error}</div>
      {:else}
        <dl class="status-list">
          <div>
            <dt>Frontend commit</dt>
            <dd data-settings-frontend-commit>{shortCommit(BUILD_INFO.commit)}</dd>
          </div>
          <div>
            <dt>Proxy status</dt>
            <dd>{health?.status || 'unknown'}</dd>
          </div>
          <div>
            <dt>Proxy commit</dt>
            <dd data-settings-proxy-commit>{shortCommit(health?.build?.commit)}</dd>
          </div>
          <div>
            <dt>Sandbox commit</dt>
            <dd data-settings-sandbox-commit>{shortCommit(health?.upstream_build?.commit)}</dd>
          </div>
          <div>
            <dt>Deploy time</dt>
            <dd>{health?.build?.deployed_at || health?.build?.built_at || 'unknown'}</dd>
          </div>
        </dl>
      {/if}
    </section>
  </div>
</section>

<style>
  .settings-app {
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: clamp(1rem, 2.3vw, 1.7rem);
    color: var(--choir-text-primary);
    background:
      radial-gradient(circle at top left, var(--choir-state-hover), transparent 32%),
      var(--choir-surface-app);
  }

  .settings-hero {
    max-width: 44rem;
    margin-bottom: 1.1rem;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-accent);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  h2,
  h3,
  p {
    margin: 0;
  }

  h2 {
    margin-bottom: 0.4rem;
    font-size: clamp(1.55rem, 4vw, 2.5rem);
    letter-spacing: 0;
  }

  h3 {
    font-size: 1rem;
  }

  .settings-hero p,
  .muted {
    color: var(--choir-text-muted);
    line-height: 1.45;
  }

  .settings-grid {
    display: grid;
    gap: 0.85rem;
  }

  .settings-card {
    border: 0;
    border-radius: var(--choir-radius-panel, 26px);
    background: var(--choir-surface-card);
    padding: 1rem;
    box-shadow: var(--choir-shadow-soft);
  }

  .account-email {
    display: block;
    margin-top: 0.5rem;
    overflow-wrap: anywhere;
  }

  .settings-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.55rem;
    margin-top: 0.8rem;
  }

  .layout-setting {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 0.75rem;
    align-items: center;
    margin-top: 0.85rem;
    border-radius: var(--choir-radius-control, 20px);
    background: color-mix(in srgb, var(--choir-surface-control) 76%, transparent);
    box-shadow: var(--choir-control-shadow);
    padding: 0.72rem;
  }

  .layout-setting strong,
  .layout-setting span {
    display: block;
  }

  .layout-setting span {
    margin-top: 0.16rem;
    color: var(--choir-text-muted);
    font-size: 0.78rem;
  }

  .segmented-control {
    display: flex;
    gap: 0.28rem;
    border-radius: var(--choir-radius-pill, 30px);
    background: color-mix(in srgb, var(--choir-bg) 42%, transparent);
    padding: 0.22rem;
  }

  .segmented-control button {
    min-width: 4.75rem;
    border: 0;
    border-radius: var(--choir-radius-pill, 30px);
    background: transparent;
    color: var(--choir-text-muted);
    cursor: pointer;
    padding: 0.5rem 0.68rem;
    font-weight: 760;
  }

  .segmented-control button.active {
    background: var(--choir-state-selected);
    color: var(--choir-text-primary);
    box-shadow: var(--choir-control-shadow);
  }

  .theme-swatch {
    display: flex;
    gap: 0.45rem;
    margin-top: 0.8rem;
  }

  .theme-swatch span {
    width: 2.3rem;
    height: 2.3rem;
    border-radius: 999px;
    box-shadow: var(--choir-control-shadow);
  }

  .theme-swatch span:nth-child(1) {
    background: var(--choir-bg);
  }

  .theme-swatch span:nth-child(2) {
    background: var(--choir-panel);
  }

  .theme-swatch span:nth-child(3) {
    background: var(--choir-accent);
  }

  .theme-status {
    margin-top: 0.65rem;
    color: var(--choir-text-accent);
    font-size: 0.78rem;
    font-weight: 700;
  }

  .theme-presets {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
    gap: 0.45rem;
    margin-top: 0.85rem;
  }

  .theme-preset {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    min-height: 2.25rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    box-shadow: var(--choir-control-shadow);
    cursor: pointer;
    padding: 0.45rem 0.6rem;
    text-align: left;
  }

  .theme-preset:hover,
  .theme-preset.active {
    background: var(--choir-state-selected);
    box-shadow: var(--choir-control-shadow), var(--choir-shadow-glow);
  }

  .preset-dot {
    width: 0.78rem;
    height: 0.78rem;
    border-radius: 50%;
    box-shadow: 0 0 14px currentColor;
    flex-shrink: 0;
  }

  .theme-dev-panel {
    margin-top: 0.9rem;
    border-radius: var(--choir-radius-control, 20px);
    background: color-mix(in srgb, var(--choir-bg) 46%, transparent);
    padding: 0.62rem;
  }

  .theme-dev-panel summary {
    color: var(--choir-text-muted);
    font-size: 0.78rem;
    font-weight: 800;
    cursor: pointer;
  }

  .theme-editor {
    width: 100%;
    min-height: 12rem;
    margin-top: 0.4rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: color-mix(in srgb, var(--choir-bg) 74%, transparent);
    color: var(--choir-text-primary);
    font-family: var(--choir-font-mono);
    font-size: 0.78rem;
    line-height: 1.45;
    padding: 0.75rem;
    resize: vertical;
  }

  .theme-editor[aria-invalid='true'] {
    outline: 2px solid var(--choir-status-danger);
    outline-offset: 2px;
  }

  .theme-dev-help {
    margin-top: 0.45rem;
    color: var(--choir-text-muted);
    font-size: 0.78rem;
    line-height: 1.4;
  }

  .theme-dev-actions {
    display: flex;
    gap: 0.45rem;
    margin-top: 0.6rem;
  }

  .theme-apply-btn,
  .theme-revert-btn {
    min-height: 2.5rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    padding: 0.5rem 0.8rem;
    font: inherit;
    font-weight: 750;
    cursor: pointer;
  }

  .theme-apply-btn {
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
  }

  .theme-revert-btn {
    background: var(--choir-surface-control);
    color: var(--choir-text-muted);
  }

  .theme-apply-btn:hover:enabled,
  .theme-revert-btn:hover:enabled {
    background: var(--choir-state-hover);
    color: var(--choir-text-primary);
  }

  .theme-apply-btn:focus-visible,
  .theme-revert-btn:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .theme-apply-btn:disabled,
  .theme-revert-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .theme-error,
  .theme-notice {
    margin-top: 0.55rem;
    font-size: 0.78rem;
    font-weight: 760;
  }

  .theme-error {
    color: var(--choir-status-danger);
  }

  .theme-notice {
    color: var(--choir-status-success);
  }

  .secondary-action {
    margin-top: 0.85rem;
    border: 0;
    border-radius: var(--choir-radius-pill, 30px);
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    box-shadow: var(--choir-control-shadow);
    cursor: pointer;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .secondary-action:hover:enabled {
    background: var(--choir-state-selected);
  }

  .secondary-action:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .status-card {
    display: grid;
    gap: 0.85rem;
  }

  .status-header {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    align-items: flex-start;
  }

  .status-list {
    display: grid;
    gap: 0.55rem;
    margin: 0;
  }

  .status-list div {
    display: grid;
    grid-template-columns: minmax(8rem, 0.5fr) minmax(0, 1fr);
    gap: 0.75rem;
    align-items: baseline;
  }

  dt {
    color: var(--choir-text-muted);
    font-size: 0.78rem;
  }

  dd {
    margin: 0;
    color: var(--choir-text-accent);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  .settings-error {
    border: 1px solid var(--choir-status-danger);
    border-radius: 14px;
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
    padding: 0.75rem;
  }

  /* API Keys */
  .api-keys-card {
    display: grid;
    gap: 0.85rem;
  }

  .api-key-create-form {
    display: grid;
    gap: 0.65rem;
  }

  .api-key-label-input {
    border: 1px solid var(--choir-state-hover);
    border-radius: var(--choir-radius-control, 20px);
    background: color-mix(in srgb, var(--choir-bg) 74%, transparent);
    color: var(--choir-text-primary);
    font-family: inherit;
    font-size: 0.85rem;
    padding: 0.55rem 0.8rem;
  }

  .api-key-scopes {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .scope-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    border-radius: var(--choir-radius-pill, 30px);
    background: color-mix(in srgb, var(--choir-bg) 42%, transparent);
    padding: 0.25rem 0.6rem;
    font-size: 0.78rem;
    cursor: pointer;
    user-select: none;
  }

  .scope-chip.active {
    background: var(--choir-state-selected);
  }

  .scope-chip input {
    margin: 0;
    accent-color: var(--choir-accent);
  }

  .api-key-secret-banner {
    border: 1px solid var(--choir-status-success);
    border-radius: 14px;
    background: var(--choir-status-success-soft, color-mix(in srgb, var(--choir-accent) 12%, transparent));
    padding: 0.75rem;
    display: grid;
    gap: 0.5rem;
  }

  .api-key-secret-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .api-key-secret {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.82rem;
    background: color-mix(in srgb, var(--choir-bg) 60%, transparent);
    padding: 0.4rem 0.6rem;
    border-radius: 8px;
  }

  .api-key-list {
    display: grid;
    gap: 0.5rem;
  }

  .api-key-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    border-radius: var(--choir-radius-control, 20px);
    background: color-mix(in srgb, var(--choir-surface-control) 76%, transparent);
    box-shadow: var(--choir-control-shadow);
    padding: 0.55rem 0.72rem;
  }

  .api-key-row.revoked {
    opacity: 0.55;
  }

  .api-key-info {
    display: grid;
    gap: 0.15rem;
    min-width: 0;
  }

  .api-key-meta {
    font-size: 0.75rem;
    color: var(--choir-text-muted);
    overflow-wrap: anywhere;
  }

  .api-key-revoke-btn {
    flex-shrink: 0;
    font-size: 0.78rem;
    padding: 0.4rem 0.7rem;
  }

  @media (min-width: 860px) {
    .settings-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .status-card {
      grid-column: 1 / -1;
    }
  }
</style>
