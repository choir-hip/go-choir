<script lang="ts">
  import { sessionExpired as shellSessionExpired } from '../../lib/app-sdk';
  import { onDestroy, onMount } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from '../../lib/app-sdk';
  import { buildInfo, copyText, openApp, resetDesktop } from '../../lib/app-sdk';
  import { getTheme, onThemeChange, setTheme } from '../../lib/app-sdk';
  import { addLiveEventListener, liveEventKind } from '../../lib/app-sdk';
  import { settingSections, getSetting, setSetting, onSetting, pickFiles, uploadFiles, currentDeviceId, chooseDesktopBackground, setDesktopBackground } from '../../lib/app-sdk';
  import PackageSettings from './PackageSettings.svelte';
  import WhoProvides from './WhoProvides.svelte';
  import {
    DEFAULT_THEME,
    listThemes,
    onThemeRegistryChange,
    normalizeThemeConfig,
    themeCSSVariables,
    validateThemeConfig,
  } from '../../lib/app-sdk';

  export let currentUser = null;
  // The window's launch context. `settingsTab` deep-links a tab (the desktop's
  // "Background and theme..." menu opens Settings on 'appearance'); the shell
  // merges a new context into the open singleton window, so `settingsTabSeq`
  // (a counter the opener bumps) makes a repeat request for the same tab land
  // again after the owner moved away. An unknown tab falls back to the first.
  export let appContext = {};

  const TABS = [
    { id: 'appearance', label: 'Appearance' },
    { id: 'account', label: 'Account & keys' },
    { id: 'packages', label: 'Packages' },
    { id: 'system', label: 'System' },
  ];
  let tab = TABS[0].id;
  let seenTabRequest = '';
  $: {
    const key = `${appContext?.settingsTab ?? ''}#${appContext?.settingsTabSeq ?? ''}`;
    if (key !== seenTabRequest) {
      seenTabRequest = key;
      if (appContext?.settingsTab !== undefined) {
        tab = TABS.some((t) => t.id === appContext.settingsTab) ? appContext.settingsTab : TABS[0].id;
      }
    }
  }
  function onTabKeydown(event, index) {
    const last = TABS.length - 1;
    let next = -1;
    if (event.key === 'ArrowRight') next = index === last ? 0 : index + 1;
    else if (event.key === 'ArrowLeft') next = index === 0 ? last : index - 1;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = last;
    if (next < 0) return;
    event.preventDefault();
    tab = TABS[next].id;
    event.currentTarget.parentElement?.querySelector(`[data-settings-tab="${TABS[next].id}"]`)?.focus();
  }

  // ---- Desktop background: its own setting, separate from the theme ----
  // (lib/desktop-background.ts holds the rules; a package may import only the SDK,
  // so the small image/mail filter is repeated here and the Desktop re-checks it.)
  const BG_EXT = ['png', 'jpg', 'jpeg', 'webp', 'gif'];
  const BG_FITS = ['cover', 'contain', 'center', 'tile'];
  const BG_FOLDER = 'Pictures/Backgrounds';
  let bgPath = '';
  let bgFit = 'cover';
  let bgError = '';
  let bgNotice = '';
  let bgBusy = false;
  const isBgImage = (name) => BG_EXT.includes(String(name).split('.').pop().toLowerCase());
  const filesURL = (parts) => '/api/files' + (parts.length ? '/' + parts.map(encodeURIComponent).join('/') : '');
  const removeBgListeners = [
    onSetting('background', (v) => { bgPath = typeof v === 'string' ? v : ''; }, 'settings'),
    onSetting('background_fit', (v) => { bgFit = BG_FITS.includes(v) ? v : 'cover'; }, 'settings'),
  ];
  onDestroy(() => removeBgListeners.forEach((off) => off()));
  async function loadBgSettings() {
    const p = await getSetting('background', 'settings');
    const f = await getSetting('background_fit', 'settings');
    bgPath = typeof p === 'string' ? p : '';
    bgFit = BG_FITS.includes(f) ? f : 'cover';
  }
  $: if ($settingSections) void loadBgSettings();
  async function saveBg(key, value) {
    bgError = '';
    const result = await setSetting(key, value, 'settings');
    if (!result.ok) bgError = result.error || 'Could not save the background';
    return result.ok;
  }
  async function useThemeColour() {
    if (await saveBg('background', '')) { bgPath = ''; bgNotice = 'Using the theme colour'; }
  }
  async function useBgPath(path) {
    const result = await setDesktopBackground(path);
    if (!result.ok) { bgError = result.error || 'Could not save the background'; return false; }
    bgPath = path;
    bgNotice = `Background set to ${path}`;
    return true;
  }
  async function chooseBgFit(fit) {
    if (await saveBg('background_fit', fit)) bgFit = fit;
  }
  // The Files picker in a modal. Cancelling changes nothing.
  async function chooseFromFiles() {
    bgError = '';
    const { path, result } = await chooseDesktopBackground();
    if (!path) return;
    if (!result?.ok) { bgError = result?.error || 'Could not save the background'; return; }
    bgPath = path;
    bgNotice = `Background set to ${path}`;
  }
  async function ensureBgFolder() {
    let parts = [];
    for (const part of BG_FOLDER.split('/')) {
      parts = [...parts, part];
      // 409 means it exists already.
      await fetchWithRenewal(filesURL(parts), { method: 'POST', headers: { 'X-Choir-Device': currentDeviceId() } }).catch(() => {});
    }
  }
  async function uploadBg() {
    bgError = '';
    const picked = await pickFiles({ accept: BG_EXT, multiple: false });
    if (!picked.length) return;
    if (!isBgImage(picked[0].name)) { bgError = 'Choose a png, jpg, webp or gif image'; return; }
    bgBusy = true;
    try {
      await ensureBgFolder();
      const [stored] = await uploadFiles(picked, { folder: BG_FOLDER });
      await useBgPath(stored);
    } catch (err) {
      bgError = err?.message || 'The upload failed';
    } finally {
      bgBusy = false;
    }
  }
  export let currentTheme = DEFAULT_THEME;

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
  let newKeyScopes = [];
  let apiKeyAdvanced = false;
  let newKeyComputerID = '';
  let newKeyExpiry = '';
  let computers = [];
  let computersLoading = false;
  let newKeySecret = '';
  let showNewKeySecret = false;
  let copiedKey = false;

  const selfDevScopes = [
    'computer:self_development:read',
    'computer:self_development:genesis',
    'computer:self_development:approve',
    'computer:self_development:rollback',
    'computer:self_development:mode',
  ];

  const allScopes = [
    { value: 'read:texture', label: 'Read Texture' },
    { value: 'write:texture', label: 'Write Texture' },
    { value: 'read:base', label: 'Read Base' },
    { value: 'write:base', label: 'Write Base' },
    { value: 'read:runtime', label: 'Read Runtime' },
    { value: 'write:runtime', label: 'Write Runtime' },
    { value: 'acceptance:read', label: 'Read Acceptance Evidence' },
    { value: 'precommit:read', label: 'Read Precommitments and Calibration' },
    { value: 'precommit:write', label: 'Write Precommitments (claims and settlements)' },
    { value: 'computer:lifecycle', label: 'Computer Lifecycle Control' },
    { value: 'selfdev-group', label: 'Computer Self-Development (all)' },
    { value: 'manage:keys', label: 'Manage API Keys' },
    { value: 'admin', label: 'Admin' },
    // Owner acts (arm, approve, restore, rematerialize, bootstrap-chain)
    // need this scope on a key; Admin does not imply it, and only this
    // signed-in session can grant it.
    { value: 'owner:acts', label: 'Owner Acts (approve, arm, restore)' },
  ];

  // The one-click key sends no scope list: the auth service gives it every
  // scope it issues, owner:acts and admin included (owner, 2026-11-03: "for
  // now, we should just grant all access"; the one list is auth.AllScopes;
  // docs/design/cli-agent-onboarding-2026-11-03.md). The advanced picker
  // starts with every scope selected, so narrowing is an explicit choice.
  newKeyScopes = allScopes.flatMap((s) => (s.value === 'selfdev-group' ? selfDevScopes : [s.value]));

  // Any scope other than manage:keys selects a computer at use time; those
  // keys are owner-wide when computer_id is empty or attenuated when bound.
  function scopesSelectComputer(scopes) {
    return scopes.some((s) => s !== 'manage:keys');
  }

  async function loadComputers() {
    computersLoading = true;
    try {
      const res = await fetchWithRenewal('/api/compute/status', { method: 'GET' });
      if (!res.ok) throw new Error(`Failed to load computers (${res.status})`);
      const data = await res.json();
      const list = Array.isArray(data.computers) ? data.computers : [];
      computers = list.filter((c) => c.computer_id);
      if (!computers.some((c) => c.computer_id === newKeyComputerID)) {
        // Recommended default is owner-wide (empty computer_id); a previous
        // explicit selection is preserved when it still exists.
        newKeyComputerID = '';
      }
    } catch (err) {
      if (!(err instanceof AuthRequiredError)) {
        apiKeyError = err.message || 'Failed to load computers';
      }
    } finally {
      computersLoading = false;
    }
  }

  async function loadAPIKeys() {
    apiKeyLoading = true;
    apiKeyError = '';
    try {
      const res = await fetchWithRenewal('/auth/api-keys', { method: 'GET' });
      if (!res.ok) throw new Error(`Failed to load API keys (${res.status})`);
      const data = await res.json();
      apiKeys = Array.isArray(data) ? data : (data.keys || []);
    } catch (err) {
      if (err instanceof AuthRequiredError) { shellSessionExpired(); return; }
      apiKeyError = err.message || 'Failed to load API keys';
    } finally {
      apiKeyLoading = false;
    }
  }

  async function createAPIKey() {
    apiKeyError = '';
    apiKeyNotice = '';
    const label = newKeyLabel.trim() || 'CLI key';
    // Simple mode: one click mints an owner-wide full-access key (scopes
    // omitted: every scope). Advanced mode: caller may narrow scopes, bind to
    // one computer, or set an expiry.
    let computerID = '';
    let expiry = '';
    const body = { label };
    if (apiKeyAdvanced) {
      body.scopes = newKeyScopes;
      computerID = newKeyComputerID;
      expiry = newKeyExpiry;
    }
    body.computer_id = computerID;
    if (expiry) {
      const parsed = new Date(expiry);
      if (!Number.isNaN(parsed.getTime())) {
        body.expires_at = parsed.toISOString();
      }
    }
    try {
      const res = await fetchWithRenewal('/auth/api-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
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
      if (err instanceof AuthRequiredError) { shellSessionExpired(); return; }
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
      if (err instanceof AuthRequiredError) { shellSessionExpired(); return; }
      apiKeyError = err.message || 'Failed to revoke API key';
    }
  }

  function toggleScope(scope) {
    if (scope === 'selfdev-group') {
      const allSelected = selfDevScopes.every((s) => newKeyScopes.includes(s));
      if (allSelected) {
        newKeyScopes = newKeyScopes.filter((s) => !selfDevScopes.includes(s));
      } else {
        const rest = newKeyScopes.filter((s) => !selfDevScopes.includes(s));
        newKeyScopes = [...rest, ...selfDevScopes];
      }
      return;
    }
    if (newKeyScopes.includes(scope)) {
      newKeyScopes = newKeyScopes.filter(s => s !== scope);
    } else {
      newKeyScopes = [...newKeyScopes, scope];
    }
  }

  function copySecret() {
    if (newKeySecret) {
      void copyText(newKeySecret, 'API key').then((copied) => {
        if (!copied) return;
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
  // The preview swatch's variables are set one property at a time, never
  // interpolated into a style string, so no value can end its declaration.
  function cssVars(node: HTMLElement, vars: Record<string, string>) {
    const set = (next: Record<string, string>) => {
      node.removeAttribute('style');
      for (const [key, value] of Object.entries(next || {})) node.style.setProperty(key, String(value));
    };
    set(vars);
    return { update: set };
  }

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
        shellSessionExpired();
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
    resetDesktop();
  }

  function handleOpenComputeMonitor() {
    openApp('compute-monitor', { appName: 'Compute Monitor', appContext: { windowTitle: 'Compute Monitor' } });
  }

  $: if (!currentUser?.email && currentTheme?.id && currentTheme.id !== selectedTheme.id) {
    selectedTheme = normalizeThemeConfig(currentTheme);
    themeJSON = JSON.stringify(selectedTheme, null, 2);
  }

  function applyTheme(theme) {
    const result = setTheme(theme);
    if (!result.ok) themeError = result.errors.join('; ');
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
  // The shell applied a different theme (another device's change, a pack's removal): show it.
  const removeThemeListener = onThemeChange((theme) => {
    selectedTheme = normalizeThemeConfig(theme);
    themeJSON = JSON.stringify(selectedTheme, null, 2);
    lastAppliedThemeJSON = themeJSON;
    themeDraftJSON = '';
  });
  let themeChoices = listThemes();
  const removeRegistryListener = onThemeRegistryChange(() => { themeChoices = listThemes(); });
  onDestroy(() => { removeThemeListener(); removeRegistryListener(); });

  onMount(async () => {
    selectedTheme = currentUser?.email ? getTheme() : normalizeThemeConfig(currentTheme);
    themeJSON = JSON.stringify(selectedTheme, null, 2);
    lastAppliedThemeJSON = themeJSON;
    void refreshStatus();
    if (currentUser?.email) {
      void loadAPIKeys();
      void loadComputers();
    }
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
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

  <div class="settings-tabs" role="tablist" aria-label="Settings sections" data-settings-tabs>
    {#each TABS as t, i}
      <button
        type="button"
        role="tab"
        id={`settings-tab-${t.id}`}
        class="settings-tab"
        class:active={tab === t.id}
        aria-selected={tab === t.id}
        aria-controls="settings-panel"
        tabindex={tab === t.id ? 0 : -1}
        data-settings-tab={t.id}
        on:click={() => (tab = t.id)}
        on:keydown={(event) => onTabKeydown(event, i)}
      >{t.label}</button>
    {/each}
  </div>

  <div class="settings-grid" role="tabpanel" id="settings-panel" aria-labelledby={`settings-tab-${tab}`} data-settings-panel={tab}>
    {#if tab === 'appearance'}
    <section class="settings-card" data-settings-theme>
      <div>
        <h3>Theme</h3>
        <p class="muted">Validated theme config with editable presets.</p>
      </div>
      <div class="theme-swatch" use:cssVars={themePreviewVars} aria-label="Current theme preview">
        <span></span>
        <span></span>
        <span></span>
      </div>
      <p class="theme-status" data-settings-theme-validation>
        {selectedTheme.name}: {themeValidation.ok ? 'valid config' : themeValidation.errors.join(', ')}
      </p>
      <div class="theme-presets" data-theme-presets>
        {#each themeChoices as theme}
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

    <section class="settings-card" data-settings-background>
      <div>
        <h3>Desktop background</h3>
        <p class="muted">An image from Files. Themes never change it, and it never changes the theme.</p>
      </div>
      <p class="theme-status" data-settings-background-current>
        {bgPath ? `Image: ${bgPath}` : 'Theme colour'}
      </p>
      <div class="theme-presets">
        <button class="theme-preset" class:active={!bgPath} data-background-theme-colour on:click={useThemeColour}>Theme colour</button>
        <button class="theme-preset" data-background-choose on:click={chooseFromFiles}>Choose from Files…</button>
        <button class="theme-preset" data-background-upload disabled={bgBusy} on:click={uploadBg}>{bgBusy ? 'Uploading...' : 'Upload a new image'}</button>
      </div>
      <div class="theme-presets" data-background-fits>
        {#each BG_FITS as fit}
          <button class="theme-preset" class:active={bgFit === fit} data-background-fit={fit} on:click={() => chooseBgFit(fit)}>{fit}</button>
        {/each}
      </div>
      {#if bgNotice}<p class="theme-notice" data-background-notice>{bgNotice}</p>{/if}
      {#if bgError}<p class="settings-error" role="alert" data-background-error>{bgError}</p>{/if}
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
    {:else if tab === 'account'}
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
          <p class="muted">Easiest: run <code>choir login</code> in a terminal and approve it here; the CLI and <code>choir mcp</code> then work with no setup. Or create a key below. The secret is shown once — copy it immediately.</p>
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
            {#if apiKeyAdvanced}
              <div class="api-key-scopes" data-api-key-scopes>
                {#each allScopes as scope}
                  <label
                    class="scope-chip"
                    class:active={scope.value === 'selfdev-group'
                      ? selfDevScopes.every((s) => newKeyScopes.includes(s))
                      : newKeyScopes.includes(scope.value)}
                  >
                    <input
                      type="checkbox"
                      checked={scope.value === 'selfdev-group'
                        ? selfDevScopes.every((s) => newKeyScopes.includes(s))
                        : newKeyScopes.includes(scope.value)}
                      on:change={() => toggleScope(scope.value)}
                    />
                    <span>{scope.label}</span>
                  </label>
                {/each}
              </div>
              {#if scopesSelectComputer(newKeyScopes)}
                <div class="api-key-computer-picker" data-api-key-computer-picker>
                  {#if computersLoading}
                    <p class="muted">Loading computers…</p>
                  {:else}
                    <label class="api-key-computer-label" for="api-key-computer-select">
                      Computer reach
                    </label>
                    <select
                      id="api-key-computer-select"
                      class="api-key-computer-select"
                      data-api-key-computer-select
                      bind:value={newKeyComputerID}
                    >
                      <option value="">All computers I own (recommended)</option>
                      {#each computers as computer}
                        <option value={computer.computer_id}>
                          {computer.desktop_id}{computer.current ? ' (current)' : ''} — {computer.computer_id}
                        </option>
                      {/each}
                    </select>
                    <p class="muted">An unbound key controls every interactive computer you own; binding narrows it to one.</p>
                  {/if}
                </div>
              {/if}
              <label class="api-key-expiry-label" for="api-key-expiry-input">
                Optional expiry
                <input
                  id="api-key-expiry-input"
                  type="datetime-local"
                  class="api-key-expiry-input"
                  data-api-key-expiry-input
                  bind:value={newKeyExpiry}
                />
              </label>
            {/if}
            <button
              class="secondary-action"
              data-api-key-create-btn
              disabled={apiKeyAdvanced && newKeyScopes.length === 0}
              on:click={createAPIKey}
            >
              {apiKeyAdvanced ? 'Create API key with these options' : 'Create API key'}
            </button>
            {#if apiKeyAdvanced}
              <button
                class="api-key-advanced-toggle"
                data-api-key-advanced-toggle
                on:click={() => apiKeyAdvanced = false}
              >
                ← Back to simple options
              </button>
            {:else}
              <p class="muted" data-api-key-simple-hint>
                Full access: every scope, owner acts included, on every computer you own, until you revoke it here. Anyone holding the key can act as you through the API. The secret is shown once — copy it immediately.
              </p>
              <button
                class="api-key-advanced-toggle"
                data-api-key-advanced-toggle
                on:click={() => apiKeyAdvanced = true}
              >
                Advanced options (scopes · computer reach · expiry)
              </button>
            {/if}
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
    {:else if tab === 'packages'}
    <section class="settings-card" data-settings-providers>
      <div>
        <h3>Who provides this?</h3>
        <p class="muted">The package drawing each part of Choir's own screen, and your choice. Use default switches back without uninstalling anything.</p>
      </div>
      <WhoProvides />
    </section>

    {#if $settingSections.some((sec) => sec.pkg !== 'settings')}
      <section class="settings-card" data-settings-packages>
        <div>
          <h3>Packages</h3>
          <p class="muted">Settings the installed packages declare.</p>
        </div>
        <PackageSettings />
      </section>
    {/if}
    {:else if tab === 'system'}
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
            <dd data-settings-frontend-commit>{shortCommit(buildInfo().commit)}</dd>
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
            <dt>Autoputer commit</dt>
            <dd data-settings-autoputer-commit>{shortCommit(health?.upstream_build?.commit)}</dd>
          </div>
          <div>
            <dt>Deploy time</dt>
            <dd>{health?.build?.deployed_at || health?.build?.built_at || 'unknown'}</dd>
          </div>
        </dl>
      {/if}
    </section>
    {/if}
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

  .settings-tabs {
    display: flex;
    gap: 0.35rem;
    margin-bottom: 0.85rem;
    overflow-x: auto;
    scrollbar-width: none;
    -webkit-overflow-scrolling: touch;
  }

  .settings-tabs::-webkit-scrollbar {
    display: none;
  }

  .settings-tab {
    flex: 0 0 auto;
    min-height: 40px;
    padding: 0.5rem 1rem;
    border: 0;
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
  }

  .settings-tab:hover {
    background: var(--choir-state-hover);
  }

  .settings-tab.active {
    background: var(--choir-accent);
    color: var(--choir-on-accent, var(--choir-bg));
  }

  .settings-tab:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
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

  .api-key-computer-picker {
    display: grid;
    gap: 0.35rem;
  }

  .api-key-computer-label,
  .api-key-expiry-label {
    font-size: 0.85rem;
    color: var(--choir-fg-muted, inherit);
    display: grid;
    gap: 0.35rem;
  }

  .api-key-computer-select,
  .api-key-expiry-input {
    font: inherit;
    color: var(--choir-fg);
    background: var(--choir-bg);
    border: 1px solid var(--choir-border, color-mix(in srgb, var(--choir-fg) 25%, transparent));
    border-radius: 8px;
    padding: 0.4rem 0.55rem;
  }

  .api-key-advanced-toggle {
    justify-self: start;
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    font-size: 0.85rem;
    color: var(--choir-accent, var(--choir-fg));
    text-decoration: underline;
    cursor: pointer;
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
