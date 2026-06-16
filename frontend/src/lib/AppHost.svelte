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
      <div class="app-load-state" role="alert">
        <p>Could not open {app.name}</p>
        <small>{loadError}</small>
      </div>
    {:else}
      <div class="app-load-state" role="status">Opening {app.name}...</div>
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
</style>
