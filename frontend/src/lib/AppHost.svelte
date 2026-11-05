<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import AppSurface from './apps/AppSurface.svelte';
  import { getAppDefinition, registryRevision, type ChoirAppDefinition } from './apps/registry';
  import type { ComponentType } from 'svelte';
  import { handoffToComputerSurfaceIfStale, isDynamicImportError } from './computer-surface-handoff.js';
  import { writable } from 'svelte/store';
  import { provideLaunchContext } from './launch-file';
  import legacyEvents from './sdk-surface/legacy-events.json';

  export let win: any;
  export let currentUser: any = null;
  export let authenticated = false;
  export let theme: any = null;

  const dispatch = createEventDispatcher();

  let loadedApp: ChoirAppDefinition | null = null;
  let Component: ComponentType | null = null;
  let loadError = '';
  let loadToken = 0;

  // registryRevision: an installed package joins or leaves the registry at run time.
  $: app = appFor($registryRevision, win?.appId || '');

  function appFor(_revision: number, appId: string): ChoirAppDefinition | null {
    return getAppDefinition(appId);
  }
  $: appContext = { ...(win?.appContext || {}), appId: win?.appId || '' };

  // getLaunchFile() / launchFileStore() in the app read the window's context.
  const launchContext = writable<any>({});
  provideLaunchContext(launchContext);
  $: launchContext.set(appContext);
  $: shellAttrs = app?.theme.shellDataAttr ? { [app.theme.shellDataAttr]: '' } : {};
  $: if (app && app !== loadedApp) {
    void loadComponent(app);
  }

  async function loadComponent(definition: ChoirAppDefinition) {
    const token = ++loadToken;
    loadedApp = definition;
    Component = null;
    loadError = '';
    try {
      const module = await definition.component();
      if (token === loadToken) {
        Component = module.default;
      }
    } catch (err) {
      if (token === loadToken) {
        if (isDynamicImportError(err) && await handoffToComputerSurfaceIfStale()) {
          return;
        }
        loadError = err instanceof Error ? err.message : 'Could not load app';
      }
    }
  }

  function reloadApp() {
    window.location.reload();
  }

  // The events an app dispatches to the shell (SDK 1.1.0 functions replace them;
  // 2.0.0 deletes the list). One source: sdk-surface/legacy-events.json, the
  // same list the lock records and the installed-package shim carries.
  let instance: any = null;
  let offs: Array<() => void> = [];
  $: bindForwards(instance);

  function bindForwards(component: any) {
    offs.forEach((off) => off());
    offs = component && typeof component.$on === 'function'
      ? legacyEvents.events.map((type: string) => component.$on(type, (event: CustomEvent) => forward(type, event)))
      : [];
  }
  onDestroy(() => offs.forEach((off) => off()));

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
        bind:this={instance}
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
