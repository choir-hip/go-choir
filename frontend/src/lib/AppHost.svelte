<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import AppSurface from './apps/AppSurface.svelte';
  import { getAppDefinition, type ChoirAppDefinition } from './apps/registry';
  import type { ComponentType } from 'svelte';

  export let win: any;
  export let currentUser: any = null;
  export let authenticated = false;
  export let theme: any = null;

  const dispatch = createEventDispatcher();

  let loadedAppId = '';
  let Component: ComponentType | null = null;
  let loadError = '';
  let loadToken = 0;

  $: app = getAppDefinition(win?.appId || '');
  $: appContext = { ...(win?.appContext || {}), appId: win?.appId || '' };
  $: shellAttrs = app?.theme.shellDataAttr ? { [app.theme.shellDataAttr]: '' } : {};
  $: if (app && app.id !== loadedAppId) {
    void loadComponent(app);
  }

  async function loadComponent(definition: ChoirAppDefinition) {
    const token = ++loadToken;
    loadedAppId = definition.id;
    Component = null;
    loadError = '';
    try {
      const module = await definition.component();
      if (token === loadToken) Component = module.default;
    } catch (err) {
      if (token === loadToken) loadError = err instanceof Error ? err.message : 'Could not load app';
    }
  }

  function reloadApp() {
    window.location.reload();
  }

  function forward(type: string, event: CustomEvent) {
    dispatch(type, event.detail || {});
  }
</script>

{#if app}
  <AppSurface {app} {win} attrs={shellAttrs}>
    {#if Component}
      <svelte:component
        this={Component}
        windowId={win.windowId}
        {currentUser}
        {authenticated}
        {appContext}
        currentTheme={theme}
        on:authexpired={(event) => forward('authexpired', event)}
        on:authrequired={(event) => forward('authrequired', event)}
        on:opentextfile={(event) => forward('opentextfile', event)}
        on:openmediafile={(event) => forward('openmediafile', event)}
        on:opentexture={(event) => forward('opentexture', event)}
        on:launchapp={(event) => forward('launchapp', event)}
        on:opentrace={(event) => forward('opentrace', event)}
        on:clearsavedwindows={(event) => forward('clearsavedwindows', event)}
        on:keepwindowonly={(event) => forward('keepwindowonly', event)}
        on:resetdesktop={(event) => forward('resetdesktop', event)}
        on:opencomputemonitor={(event) => forward('opencomputemonitor', event)}
        on:contextchange={(event) => forward('contextchange', event)}
      />
    {:else if loadError}
      <div class="app-load-state app-load-error" role="alert">
        <p>Could not open {app.name}</p>
        <small>{loadError}</small>
        <button class="app-reload-btn" type="button" on:click={reloadApp}>Reload app</button>
      </div>
    {:else}
      <div class="app-load-state" role="status" aria-live="polite">
        <span class="app-loading-spinner" aria-hidden="true"></span>
        <p>Opening {app.name}…</p>
      </div>
    {/if}
  </AppSurface>
{:else}
  <div class="app-content" data-app-host data-app-id={win?.appId || ''}>
    <div class="app-load-state" role="alert">Unknown app: {win?.appId || 'app'}</div>
  </div>
{/if}

<style>
  .app-load-state {
    display: grid;
    gap: 0.3rem;
    place-content: center;
    min-height: 100%;
    color: var(--choir-muted);
    text-align: center;
  }

  .app-load-state p {
    margin: 0;
    color: var(--choir-fg);
    font-weight: 760;
  }

  .app-load-state small {
    color: var(--choir-danger);
  }

  .app-reload-btn {
    justify-self: center;
    min-height: 2.5rem;
    margin-top: 0.45rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    padding: 0.55rem 0.9rem;
    font: inherit;
    font-weight: 750;
    cursor: pointer;
  }

  .app-reload-btn:hover {
    background: var(--choir-state-hover);
  }

  .app-reload-btn:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 3px;
  }

  .app-loading-spinner {
    justify-self: center;
    width: 1.55rem;
    height: 1.55rem;
    border: 2px solid var(--choir-state-hover);
    border-top-color: var(--choir-accent);
    border-radius: 50%;
    animation: app-spinner-spin 700ms linear infinite;
  }

  @keyframes app-spinner-spin {
    to { transform: rotate(360deg); }
  }

  @media (prefers-reduced-motion: reduce) {
    .app-loading-spinner {
      animation: none;
      border-color: var(--choir-state-focus);
      border-top-color: var(--choir-accent);
    }
  }
</style>
