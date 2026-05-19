<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import { BUILD_INFO } from './build-info.js';
  import {
    DEFAULT_THEME,
    THEME_PRESETS,
    THEME_STORAGE_KEY,
    normalizeThemeConfig,
    themeCSSVariables,
    validateThemeConfig,
  } from './theme.js';

  export let currentUser = null;

  const dispatch = createEventDispatcher();

  let loading = true;
  let error = '';
  let health = null;
  let selectedTheme = DEFAULT_THEME;
  let themeJSON = JSON.stringify(DEFAULT_THEME, null, 2);
  let themeError = '';
  let themeNotice = '';
  let promotionLoading = true;
  let promotionError = '';
  let promotionActionError = '';
  let promotionActingId = '';
  let promotions = [];

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

  async function refreshPromotions() {
    promotionLoading = true;
    promotionError = '';
    try {
      const res = await fetchWithRenewal('/api/promotions', { method: 'GET' });
      if (!res.ok) {
        throw new Error(`Promotion queue failed (${res.status})`);
      }
      const body = await res.json();
      promotions = Array.isArray(body?.candidates) ? body.candidates : [];
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      promotionError = err.message || 'Promotion queue unavailable';
      promotions = [];
    } finally {
      promotionLoading = false;
    }
  }

  function promotionReport(candidate) {
    return candidate?.report_json && typeof candidate.report_json === 'object' ? candidate.report_json : {};
  }

  function promotionApproved(candidate) {
    return promotionReport(candidate).promotion_approved === true;
  }

  function canApprovePromotion(candidate) {
    return candidate?.status === 'verified' && !promotionApproved(candidate);
  }

  function canPromotePromotion(candidate) {
    return candidate?.status === 'verified' && promotionApproved(candidate);
  }

  function canVerifyPromotion(candidate) {
    return candidate && ['queued', 'integrated', 'verification_failed'].includes(candidate.status);
  }

  function canRejectPromotion(candidate) {
    return candidate && !['promoted', 'rejected'].includes(candidate.status);
  }

  async function reviewPromotion(candidate, action) {
    if (!candidate?.candidate_id) return;
    promotionActionError = '';
    promotionActingId = `${candidate.candidate_id}:${action}`;
    try {
      const res = await fetchWithRenewal(`/api/promotions/${encodeURIComponent(candidate.candidate_id)}/${action}`, {
        method: 'POST',
        body: ['verify', 'promote'].includes(action) ? '{}' : undefined,
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body?.error || `Promotion action failed (${res.status})`);
      }
      await refreshPromotions();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      promotionActionError = err.message || 'Promotion action failed';
    } finally {
      promotionActingId = '';
    }
  }

  function shortCommit(value) {
    if (!value) return 'unknown';
    return String(value).slice(0, 12);
  }

  function handleResetDesktop() {
    dispatch('resetdesktop');
  }

  function handleOpenSystemMonitor() {
    dispatch('opensystemmonitor');
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

  onMount(() => {
    selectedTheme = loadStoredTheme();
    themeJSON = JSON.stringify(selectedTheme, null, 2);
    void refreshStatus();
    void refreshPromotions();
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
        <button class="secondary-action" data-settings-open-system-monitor on:click={handleOpenSystemMonitor}>
          Open System Monitor
        </button>
        <button class="secondary-action" data-settings-reset-desktop on:click={handleResetDesktop}>
          Reset layout
        </button>
      </div>
    </section>

    <section class="settings-card promotion-card" data-settings-promotions>
      <div class="status-header">
        <div>
          <h3>Promotion queue</h3>
          <p class="muted">Review candidate-world patchsets before canonical promotion.</p>
        </div>
        <button class="secondary-action" data-settings-promotions-refresh on:click={refreshPromotions} disabled={promotionLoading}>
          {promotionLoading ? 'Checking…' : 'Refresh'}
        </button>
      </div>

      {#if promotionError}
        <div class="settings-error" data-settings-promotions-error role="alert">{promotionError}</div>
      {:else}
        {#if promotionActionError}
          <div class="settings-error" data-settings-promotions-action-error role="alert">{promotionActionError}</div>
        {/if}
        {#if promotionLoading}
          <p class="promotion-empty" data-settings-promotions-loading>Loading promotion candidates…</p>
        {:else if promotions.length === 0}
          <p class="promotion-empty" data-settings-promotions-empty>No candidate patchsets queued.</p>
        {:else}
          <div class="promotion-list" data-settings-promotions-list>
            {#each promotions as candidate}
              <article class="promotion-item" data-settings-promotion-candidate data-settings-promotion-id={candidate.candidate_id}>
                <div>
                  <strong>{candidate.summary || candidate.candidate_id}</strong>
                  <span>{candidate.vm_id || 'no-vm'} · {candidate.integration_branch || 'not integrated'}</span>
                </div>
                <span class="promotion-status" data-settings-promotion-status>{candidate.status}</span>
                {#if candidate.error}
                  <p>{candidate.error}</p>
                {/if}
                {#if promotionApproved(candidate)}
                  <p class="promotion-approved" data-settings-promotion-approved>Owner approved</p>
                {/if}
                {#if canVerifyPromotion(candidate) || canApprovePromotion(candidate) || canPromotePromotion(candidate) || canRejectPromotion(candidate)}
                  <div class="promotion-actions" data-settings-promotion-actions>
                    {#if canVerifyPromotion(candidate)}
                      <button
                        class="promotion-action verify"
                        data-settings-promotion-verify
                        on:click={() => reviewPromotion(candidate, 'verify')}
                        disabled={promotionActingId === `${candidate.candidate_id}:verify`}
                      >
                        {promotionActingId === `${candidate.candidate_id}:verify` ? 'Verifying…' : 'Verify'}
                      </button>
                    {/if}
                    {#if canApprovePromotion(candidate)}
                      <button
                        class="promotion-action approve"
                        data-settings-promotion-approve
                        on:click={() => reviewPromotion(candidate, 'approve')}
                        disabled={promotionActingId === `${candidate.candidate_id}:approve`}
                      >
                        {promotionActingId === `${candidate.candidate_id}:approve` ? 'Approving…' : 'Approve'}
                      </button>
                    {/if}
                    {#if canPromotePromotion(candidate)}
                      <button
                        class="promotion-action approve"
                        data-settings-promotion-promote
                        on:click={() => reviewPromotion(candidate, 'promote')}
                        disabled={promotionActingId === `${candidate.candidate_id}:promote`}
                      >
                        {promotionActingId === `${candidate.candidate_id}:promote` ? 'Promoting…' : 'Promote'}
                      </button>
                    {/if}
                    {#if canRejectPromotion(candidate)}
                      <button
                        class="promotion-action reject"
                        data-settings-promotion-reject
                        on:click={() => reviewPromotion(candidate, 'reject')}
                        disabled={promotionActingId === `${candidate.candidate_id}:reject`}
                      >
                        {promotionActingId === `${candidate.candidate_id}:reject` ? 'Rejecting…' : 'Reject'}
                      </button>
                    {/if}
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        {/if}
      {/if}
    </section>

    <section class="settings-card status-card" data-settings-runtime-status>
      <div class="status-header">
        <div>
          <h3>Runtime status</h3>
          <p class="muted">Read-only deploy and backend identity.</p>
        </div>
        <button class="secondary-action" on:click={refreshStatus} disabled={loading}>
          {loading ? 'Checking…' : 'Refresh'}
        </button>
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

  .promotion-card {
    display: grid;
    gap: 0.85rem;
  }

  .status-header {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    align-items: flex-start;
  }

  .status-header .secondary-action {
    margin-top: 0;
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

  .promotion-empty {
    color: var(--choir-muted, #94a3b8);
    font-size: 0.86rem;
  }

  .promotion-list {
    display: grid;
    gap: 0.55rem;
  }

  .promotion-item {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.6rem;
    align-items: start;
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: var(--choir-radius-md, 12px);
    background: rgba(2, 6, 23, 0.35);
    padding: 0.7rem;
  }

  .promotion-item strong,
  .promotion-item span,
  .promotion-item p {
    overflow-wrap: anywhere;
  }

  .promotion-item div {
    display: grid;
    gap: 0.18rem;
    min-width: 0;
  }

  .promotion-item div span,
  .promotion-item p {
    color: var(--choir-muted, #94a3b8);
    font-size: 0.78rem;
  }

  .promotion-status {
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 999px;
    color: #bfdbfe;
    font-size: 0.72rem;
    font-weight: 800;
    padding: 0.2rem 0.5rem;
    white-space: nowrap;
  }

  .promotion-approved {
    color: #bbf7d0;
    font-weight: 800;
  }

  .promotion-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    margin-top: 0.28rem;
  }

  .promotion-action {
    min-height: 2rem;
    border: 1px solid rgba(148, 163, 184, 0.22);
    border-radius: var(--choir-radius-sm, 8px);
    color: var(--choir-fg, #e2e8f0);
    cursor: pointer;
    padding: 0.42rem 0.7rem;
    font-size: 0.76rem;
    font-weight: 820;
  }

  .promotion-action.verify {
    border-color: rgba(96, 165, 250, 0.42);
    background: rgba(30, 64, 175, 0.34);
  }

  .promotion-action.approve {
    border-color: rgba(74, 222, 128, 0.42);
    background: rgba(22, 101, 52, 0.38);
  }

  .promotion-action.reject {
    border-color: rgba(248, 113, 113, 0.36);
    background: rgba(127, 29, 29, 0.32);
  }

  .promotion-action:disabled {
    opacity: 0.58;
    cursor: not-allowed;
  }

  @media (min-width: 860px) {
    .settings-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .status-card,
    .promotion-card {
      grid-column: 1 / -1;
    }
  }
</style>
