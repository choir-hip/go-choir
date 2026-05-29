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

  const dispatch = createEventDispatcher();

  let loading = true;
  let error = '';
  let health = null;
  let selectedTheme = DEFAULT_THEME;
  let themeJSON = JSON.stringify(DEFAULT_THEME, null, 2);
  let themeError = '';
  let themeNotice = '';

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
    themeError = '';
    themeNotice = `${selectedTheme.name} applied`;
    applyTheme(selectedTheme);
  }

  function handleThemeJSONInput(event) {
    themeJSON = event.currentTarget.value;
    themeNotice = '';
    try {
      const parsed = JSON.parse(themeJSON);
      const validation = validateThemeConfig(parsed);
      if (!validation.ok) {
        themeError = validation.errors.join(', ');
        return;
      }
      selectedTheme = normalizeThemeConfig(parsed);
      themeError = '';
      applyTheme(selectedTheme);
    } catch (_err) {
      themeError = 'theme JSON is invalid';
    }
  }

  let removeLiveListener = () => {};

  onMount(async () => {
    selectedTheme = await loadStoredTheme();
    themeJSON = JSON.stringify(selectedTheme, null, 2);
    void refreshStatus();
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
      if (kind === 'theme.updated') {
        void loadStoredTheme().then((theme) => {
          selectedTheme = theme;
          themeJSON = JSON.stringify(selectedTheme, null, 2);
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
        <p class="muted">Signed in as</p>
      </div>
      <strong class="account-email">{currentUser?.email || 'unknown'}</strong>
    </section>

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
      <label class="theme-editor-label" for="theme-editor">Theme JSON</label>
      <textarea
        id="theme-editor"
        class="theme-editor"
        data-theme-editor
        spellcheck="false"
        value={themeJSON}
        on:input={handleThemeJSONInput}
      ></textarea>
      {#if themeError}
        <p class="theme-error" data-theme-error>{themeError}</p>
      {:else if themeNotice}
        <p class="theme-notice" data-theme-notice>{themeNotice}</p>
      {/if}
    </section>

    <section class="settings-card" data-settings-desktop>
      <div>
        <h3>Desktop layout</h3>
        <p class="muted">Reset open windows and icon positions if old persisted geometry gets in the way.</p>
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
    color: var(--choir-fg, #f8fafc);
    background:
      radial-gradient(circle at top left, rgba(59, 130, 246, 0.12), transparent 32%),
      var(--choir-panel, #171827);
  }

  .settings-hero {
    max-width: 44rem;
    margin-bottom: 1.1rem;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: var(--choir-accent, #60a5fa);
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
    letter-spacing: -0.05em;
  }

  h3 {
    font-size: 1rem;
  }

  .settings-hero p,
  .muted {
    color: var(--choir-muted, #a8b3c7);
    line-height: 1.45;
  }

  .settings-grid {
    display: grid;
    gap: 0.85rem;
  }

  .settings-card {
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: var(--choir-radius-lg, 18px);
    background: rgba(15, 23, 42, 0.52);
    padding: 1rem;
    box-shadow: var(--choir-shadow-soft, 0 16px 42px rgba(0, 0, 0, 0.28));
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

  .theme-swatch {
    display: flex;
    gap: 0.45rem;
    margin-top: 0.8rem;
  }

  .theme-swatch span {
    width: 2.3rem;
    height: 2.3rem;
    border-radius: 999px;
    border: 1px solid rgba(255, 255, 255, 0.16);
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
    color: #bfdbfe;
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
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: var(--choir-radius-md, 12px);
    background: rgba(15, 23, 42, 0.56);
    color: var(--choir-fg, #e2e8f0);
    cursor: pointer;
    padding: 0.45rem 0.6rem;
    text-align: left;
  }

  .theme-preset:hover,
  .theme-preset.active {
    border-color: rgba(96, 165, 250, 0.48);
    background: rgba(96, 165, 250, 0.12);
  }

  .preset-dot {
    width: 0.78rem;
    height: 0.78rem;
    border: 1px solid rgba(255, 255, 255, 0.42);
    border-radius: 50%;
    flex-shrink: 0;
  }

  .theme-editor-label {
    display: block;
    margin-top: 0.9rem;
    color: var(--choir-muted, #94a3b8);
    font-size: 0.78rem;
    font-weight: 800;
  }

  .theme-editor {
    width: 100%;
    min-height: 12rem;
    margin-top: 0.4rem;
    border: 1px solid rgba(148, 163, 184, 0.2);
    border-radius: var(--choir-radius-md, 12px);
    background: rgba(2, 6, 23, 0.58);
    color: var(--choir-fg, #e2e8f0);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.78rem;
    line-height: 1.45;
    padding: 0.75rem;
    resize: vertical;
  }

  .theme-error,
  .theme-notice {
    margin-top: 0.55rem;
    font-size: 0.78rem;
    font-weight: 760;
  }

  .theme-error {
    color: #fecaca;
  }

  .theme-notice {
    color: #bbf7d0;
  }

  .secondary-action {
    margin-top: 0.85rem;
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 999px;
    background: rgba(15, 23, 42, 0.8);
    color: #e0ecff;
    cursor: pointer;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .secondary-action:hover:enabled {
    border-color: rgba(147, 197, 253, 0.5);
    background: rgba(30, 41, 59, 0.92);
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
    color: var(--choir-muted, #94a3b8);
    font-size: 0.78rem;
  }

  dd {
    margin: 0;
    color: #e2e8f0;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  .settings-error {
    border: 1px solid rgba(248, 113, 113, 0.34);
    border-radius: 14px;
    background: rgba(127, 29, 29, 0.42);
    color: #fecaca;
    padding: 0.75rem;
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
